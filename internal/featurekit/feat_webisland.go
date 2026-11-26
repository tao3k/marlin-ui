// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && !js

package featurekit

import (
	"context"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/input"
	"github.com/tao3k/marlin-mygo/engine/shell"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
)

// FeatureUIWebView registers Marlin's optional Hybrid GPU + Sandboxed WebView
// host (for built-in BPMN/Flowchart editors, Mermaid/ECharts canvases, and
// user-authored TypeScript/HTML islands). Lazy-loaded asynchronously off the
// GPU render thread so tab/window switching never stalls, and compile-time
// pruned under `-tags marlin_minimal` and Web WASM.
const FeatureUIWebView features.FeatureID = "ui.webview"

// MaxCachedWebIslands bounds live WebView instances via LRU eviction so memory
// and WebContent process count stay deterministic.
const MaxCachedWebIslands = 4

// WebViewInstance bundles a native transparent WebView with non-blocking JS
// evaluation, live HTML reload, teardown, and lifecycle state.
type WebViewInstance struct {
	View       ui.NativeWebView
	Eval       func(js string)
	ReloadHTML func(html string)
	Destroy    func()
	Tier       async.TrustTier
	lifecycle  *async.WebViewLifecycle
	evalLane   *async.FencedLane[string, struct{}]
	lastUseSeq uint64
}

// WebViewFactory creates a transparent sandboxed native WebView hosting html
// inside the active desktop window with explicit trust tier and preload JS.
type WebViewFactory func(islandID, html, preloadJS string, tier async.TrustTier) (WebViewInstance, error)

var (
	webIslandMu       sync.Mutex
	webViewFactory    WebViewFactory
	webViewInvalidate func()
	webViewScope      = async.NewStructuredScope(context.Background())
	webViewCache      = map[string]*WebViewInstance{}
	webViewPending    = map[string]bool{}
	webViewUseSeq     uint64
	userIslandHTMLs   = map[string]string{}
	webIslandOpacity  = kit.DefaultGlassOpacity
	webIslandPalette  = kit.Palette{
		Canvas:  ui.Hex("#1e222a"),
		Bar:     ui.Hex("#191d24"),
		Surface: ui.Hex("#2b303c"),
		Hover:   ui.Hex("#353c4b"),
		Border:  ui.Hex("#3d4556"),
		Ink:     ui.Hex("#e5e9f0"),
		Muted:   ui.Hex("#8c96a8"),
		Accent:  ui.Hex("#d48ab8"),
		Info:    ui.Hex("#88c0d0"),
		Success: ui.Hex("#a3be8c"),
		Warning: ui.Hex("#ebcb8b"),
		Danger:  ui.Hex("#e06c75"),
	}
)

// SetWebViewFactory binds the desktop window's lazy transparent WebView factory.
func SetWebViewFactory(fn WebViewFactory) {
	webIslandMu.Lock()
	webViewFactory = fn
	webIslandMu.Unlock()
}

// SetWebViewInvalidate registers the window invalidate callback invoked once
// an asynchronously initialized WebView island becomes ready.
func SetWebViewInvalidate(fn func()) {
	webIslandMu.Lock()
	webViewInvalidate = fn
	webIslandMu.Unlock()
}

// RegisterUserIslands registers governed HTML/TS island artifacts loaded from
// ~/.config/marlin/ui/islands/<id>/index.html and hot-reloads any active cached
// WebViews whose HTML content changed on disk.
func RegisterUserIslands(islands map[string]string) {
	type reloadJob struct {
		fn   func(string)
		html string
	}
	var jobs []reloadJob

	webIslandMu.Lock()
	prev := userIslandHTMLs
	userIslandHTMLs = make(map[string]string, len(islands))
	for k, v := range islands {
		userIslandHTMLs[k] = v
		if prev[k] != v {
			if inst, ok := webViewCache[k]; ok && inst.ReloadHTML != nil {
				jobs = append(jobs, reloadJob{fn: inst.ReloadHTML, html: v})
			}
		}
	}
	pal := webIslandPalette
	for k, inst := range webViewCache {
		if _, had := prev[k]; had {
			if _, still := userIslandHTMLs[k]; !still && inst.ReloadHTML != nil {
				jobs = append(jobs, reloadJob{fn: inst.ReloadHTML, html: kit.WorkflowStudioHTML(pal)})
			}
		}
	}
	webIslandMu.Unlock()

	for _, j := range jobs {
		job := j
		go job.fn(job.html)
	}
}

// SyncWebIslandTheme updates the theme palette and glass opacity for built-in
// and mounted WebView islands, pushing live CSS custom properties + a
// `marlin:theme` CustomEvent through each island's coalescing FencedLane
// (preventing main-loop Eval storms under rapid theme/opacity cycling).
func SyncWebIslandTheme(pal kit.Palette, glassOpacity float32) {
	if glassOpacity <= 0 {
		glassOpacity = kit.DefaultGlassOpacity
	}
	webIslandMu.Lock()
	changed := webIslandPalette != pal || webIslandOpacity != glassOpacity
	webIslandPalette = pal
	webIslandOpacity = glassOpacity
	if !changed || len(webViewCache) == 0 {
		webIslandMu.Unlock()
		return
	}
	lanes := make([]*async.FencedLane[string, struct{}], 0, len(webViewCache))
	for _, inst := range webViewCache {
		if inst != nil && inst.evalLane != nil {
			lanes = append(lanes, inst.evalLane)
		}
	}
	webIslandMu.Unlock()

	script := kit.WebIslandThemeScript(pal, glassOpacity)
	for _, lane := range lanes {
		lane.Submit(script)
	}
}

// DispatchWebIslandAction sends a structured `marlin:action` event into a
// mounted WebView island through its coalescing FencedLane (e.g. triggered by
// native GPU sidebar action URI `island:workflow-graph:step`).
func DispatchWebIslandAction(islandID, event string) bool {
	webIslandMu.Lock()
	inst := webViewCache[islandID]
	var lane *async.FencedLane[string, struct{}]
	if inst != nil {
		lane = inst.evalLane
	}
	webIslandMu.Unlock()
	if lane == nil {
		return false
	}
	lane.Submit(kit.WebIslandActionScript(event, "null"))
	return true
}

// RecoverWebIsland evicts a crashed WebContent process cleanly and schedules
// non-blocking recovery on the next frame without stalling GPU rendering.
func RecoverWebIsland(islandID string) {
	webIslandMu.Lock()
	inst := webViewCache[islandID]
	delete(webViewCache, islandID)
	delete(webViewPending, islandID)
	inv := webViewInvalidate
	webIslandMu.Unlock()
	if inst != nil {
		if inst.lifecycle != nil {
			inst.lifecycle.MarkCrashed()
		}
		if inst.evalLane != nil {
			inst.evalLane.Close()
		}
		if inst.Destroy != nil {
			inst.Destroy()
		}
	}
	if inv != nil {
		inv()
	}
}

// ShutdownWebIslands drains all active WebView async lanes cleanly on app quit.
func ShutdownWebIslands() {
	webIslandMu.Lock()
	lanes := make([]*async.FencedLane[string, struct{}], 0, len(webViewCache))
	for _, inst := range webViewCache {
		if inst != nil && inst.evalLane != nil {
			lanes = append(lanes, inst.evalLane)
		}
	}
	webIslandMu.Unlock()
	for _, lane := range lanes {
		lane.Close()
	}
	webViewScope.CloseAndDrain()
}

// WebIslandHost returns Marlin's non-blocking, async-resolving kit.WebIslandHost.
func WebIslandHost() kit.WebIslandHost {
	return desktopWebIslandHost{}
}

type desktopWebIslandHost struct{}

// Resolve returns a cached WebView in O(1) without blocking the GPU render
// thread. On first access, initialization runs asynchronously inside
// webViewScope (StructuredScope) with PreloadScript theme injection and
// invalidates the window upon completion so tab/window switching stays 0-stall.
func (desktopWebIslandHost) Resolve(islandID string) ui.NativeWebView {
	webIslandMu.Lock()
	if existing, ok := webViewCache[islandID]; ok && existing != nil && existing.View != nil {
		webViewUseSeq++
		existing.lastUseSeq = webViewUseSeq
		webIslandMu.Unlock()
		return existing.View
	}
	factory := webViewFactory
	if factory == nil || webViewPending[islandID] {
		webIslandMu.Unlock()
		return nil
	}
	webViewPending[islandID] = true
	html, hasUser := userIslandHTMLs[islandID]
	pal := webIslandPalette
	op := webIslandOpacity
	inv := webViewInvalidate
	webIslandMu.Unlock()

	tier := async.TrustTier0FirstParty
	if hasUser {
		tier = async.TrustTier2Untrusted
	} else {
		html = kit.WorkflowStudioHTML(pal)
	}
	preload := kit.WebIslandThemeScript(pal, op)
	lc := async.NewWebViewLifecycle(tier)
	lc.RequestAsyncMount()

	webViewScope.Go(func(_ context.Context) {
		inst, err := factory(islandID, html, preload, tier)
		webIslandMu.Lock()
		delete(webViewPending, islandID)
		if err == nil && inst.View != nil {
			lc.MarkReady()
			inst.Tier = tier
			inst.lifecycle = lc
			if inst.Eval != nil {
				evalFn := inst.Eval
				inst.evalLane = async.NewFencedLane[string, struct{}](
					func(ctx context.Context, js string) (struct{}, error) {
						if ctx.Err() != nil {
							return struct{}{}, ctx.Err()
						}
						evalFn(js)
						return struct{}{}, nil
					},
					nil,
				)
			}
			cp := inst
			webViewUseSeq++
			cp.lastUseSeq = webViewUseSeq
			webViewCache[islandID] = &cp
			var evicted *WebViewInstance
			if len(webViewCache) > MaxCachedWebIslands {
				var victimKey string
				var oldest uint64 = ^uint64(0)
				for k, v := range webViewCache {
					if k != islandID && v != nil && v.lastUseSeq < oldest {
						oldest = v.lastUseSeq
						victimKey = k
					}
				}
				if victimKey != "" {
					evicted = webViewCache[victimKey]
					delete(webViewCache, victimKey)
				}
			}
			if evicted != nil {
				go func(e *WebViewInstance) {
					if e.evalLane != nil {
						e.evalLane.Close()
					}
					if e.Destroy != nil {
						e.Destroy()
					}
				}(evicted)
			}
		}
		latestPal := webIslandPalette
		latestOp := webIslandOpacity
		cached := webViewCache[islandID]
		webIslandMu.Unlock()

		if err == nil && cached != nil {
			if cached.evalLane != nil {
				cached.evalLane.Submit(kit.WebIslandThemeScript(latestPal, latestOp))
			}
			if inv != nil {
				inv()
			}
		}
	})
	return nil
}

func init() {
	RegisterCompiled(&webIslandModule{})
}

type webIslandModule struct{}

func (m *webIslandModule) Meta() features.FeatureMeta {
	return features.FeatureMeta{
		ID:             FeatureUIWebView,
		Name:           "Hybrid WebView Studio",
		Description:    "Lazy-loaded transparent WebView islands for BPMN/Flowchart canvases & TS artifacts",
		Category:       "hybrid",
		TrustTier:      features.Tier0HostNative,
		DefaultEnabled: true,
	}
}

func (m *webIslandModule) Register(host *features.Host) error {
	if host == nil {
		return nil
	}
	host.AddCommand(input.CommandEntry{
		Name:        "open-workflow-canvas",
		Description: "Open Hybrid GPU + WebView BPMN / Workflow Studio (*Workflow Canvas*)",
	})
	host.AddBuffer(shell.PaneBuffer{
		BufferID: "feature:webisland",
		Title:    "Workflow Canvas",
		Kind:     "hybrid-webview",
	}, func(surface any, th any) {
		c, ok1 := surface.(*ui.Context)
		t, ok2 := th.(*ui.Theme)
		if !ok1 || !ok2 {
			return
		}
		pal := paletteFromUITheme(t)
		SyncWebIslandTheme(pal, webIslandOpacity)
		stageBg := pal.Canvas
		if c.Vibrancy() {
			stageBg = ui.Transparent
		}
		// Full-bleed immersive Emacs Window: zero redundant header, border, or
		// badge chrome — the sort-tab already displays *Workflow Canvas*.
		kit.SafeCol(c, "webisland-stage", 0).Fill().Background(stageBg).AlignItems(ui.Stretch).Children(func() {
			kit.WebIslandFrame(c, kit.WebIslandConfig{
				Key:        "workflow-studio-island",
				IslandID:   "workflow-graph",
				Borderless: true,
				Palette:    pal,
				Surface:    kit.SurfaceOptions{Glass: true, Motion: true},
				Host:       WebIslandHost(),
			})
		})
	})
	return nil
}

func paletteFromUITheme(t *ui.Theme) kit.Palette {
	if t == nil {
		return webIslandPalette
	}
	isLight := int(t.Background.R)+int(t.Background.G)+int(t.Background.B) > 380
	return kit.Palette{
		IsLight: isLight,
		Canvas:  t.Background,
		Bar:     t.Surface,
		Surface: t.SurfacePressed,
		Hover:   t.SurfaceHover,
		Border:  t.Border,
		Ink:     t.Text,
		Muted:   t.TextMuted,
		Accent:  t.Accent,
		Info:    t.AccentHover,
		Success: t.Success,
		Warning: t.Warning,
		Danger:  t.Danger,
	}
}
