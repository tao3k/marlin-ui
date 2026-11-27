// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && !js

package featurekit

import (
	"context"
	"fmt"
	"strings"
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

// BuiltinWorkflowStudioIslandID is the ONLY island ID permitted Tier-0
// first-party host RPC (default-deny per HybridWebViewFocusLifecycle.qnt).
const BuiltinWorkflowStudioIslandID = "workflow-graph"

// BuiltinArtifactStageIslandID is Marlin's Layer-3 Tier-2 Untrusted sandboxed
// WebView stage for Vercel AI SDK `data-*` Generative UI artifacts.
const BuiltinArtifactStageIslandID = "artifact-stage"

var (
	webIslandMu       sync.Mutex
	webViewFactory    WebViewFactory
	webViewInvalidate func()
	webViewScope      = async.NewStructuredScope(context.Background())
	webViewPermits    = async.NewPermitPool(1, 2)
	webViewCache      = map[string]*WebViewInstance{}
	webViewPending    = map[string]bool{}
	webViewUseSeq     uint64
	userIslandHTMLs   = map[string]string{}
	lastArtifactScript string
	webIslandOpacity  = kit.DefaultGlassOpacity
	webIslandPalette  = kit.Palette{
		Canvas:  ui.Hex("#282c34"),
		Bar:     ui.Hex("#21242b"),
		Surface: ui.Hex("#2d333f"),
		Hover:   ui.Hex("#363d4c"),
		Border:  ui.Hex("#424959"),
		Ink:     ui.Hex("#d5dce6"),
		Muted:   ui.Hex("#8b94a6"),
		Accent:  ui.Hex("#51afef"),
		Info:    ui.Hex("#46d9ff"),
		Success: ui.Hex("#98be65"),
		Warning: ui.Hex("#ecbe7b"),
		Danger:  ui.Hex("#ff6c6b"),
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
	op := webIslandOpacity
	for k, inst := range webViewCache {
		if _, had := prev[k]; had {
			if _, still := userIslandHTMLs[k]; !still && inst.ReloadHTML != nil {
				jobs = append(jobs, reloadJob{fn: inst.ReloadHTML, html: workflowStudioThemedHTML(pal, op)})
			}
		}
	}
	webIslandMu.Unlock()

	for _, j := range jobs {
		job := j
		webViewScope.Go(func(ctx context.Context) {
			if ctx.Err() == nil {
				job.fn(job.html)
			}
		})
	}
}

// PrewarmWebIsland asynchronously pre-initializes islandID off the user's
// first-click critical path (e.g. shortly after desktop window startup) so
// switching to *Workflow Canvas* is instant with zero 300ms WKWebView boot stall.
func PrewarmWebIsland(islandID string) {
	if islandID == "" {
		return
	}
	_ = WebIslandHost().Resolve(islandID)
}

// NotifyWebIslandVisibility enforces HybridWebViewFocusLifecycle.qnt focus
// revocation: hidden islands invoke HideTab() (clearing shown & focused) while
// the active island invokes ShowTab() when ready. On visibility transitions,
// dispatches a single lightweight marlin:visibility event to pause/resume
// continuous SVG/CSS animations so hidden WebViews never contend with GPU tabs.
func NotifyWebIslandVisibility(activeIslandID string) {
	type visJob struct {
		lane    *async.FencedLane[string, struct{}]
		visible bool
	}
	var jobs []visJob

	webIslandMu.Lock()
	for id, inst := range webViewCache {
		if inst == nil || inst.lifecycle == nil {
			continue
		}
		_, wasShown, _, _ := inst.lifecycle.Snapshot()
		wantShown := id == activeIslandID
		if wantShown {
			inst.lifecycle.ShowTab()
		} else {
			inst.lifecycle.HideTab()
		}
		_, nowShown, _, _ := inst.lifecycle.Snapshot()
		if inst.evalLane != nil && nowShown != wasShown {
			jobs = append(jobs, visJob{lane: inst.evalLane, visible: nowShown})
		}
	}
	webIslandMu.Unlock()

	for _, j := range jobs {
		j.lane.Submit(kit.WebIslandVisibilityScript(j.visible))
	}
}

// CheckWorkflowBridgeRPC verifies that the first-party Tier-0 Workflow Studio
// island is mounted, Ready, and permitted host RPC (tier0-rpc-capability-gate).
func CheckWorkflowBridgeRPC() error {
	webIslandMu.Lock()
	inst := webViewCache[BuiltinWorkflowStudioIslandID]
	var lc *async.WebViewLifecycle
	if inst != nil {
		lc = inst.lifecycle
	}
	webIslandMu.Unlock()
	if lc == nil {
		return async.ErrUntrustedRPCForbidden
	}
	return lc.CheckHostRPC()
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

	script := webIslandViewportThemeScript(pal, glassOpacity)
	for _, lane := range lanes {
		lane.Submit(script)
	}
}

func rgbaCSS(c ui.Color, opacity float32) string {
	if opacity < 0 {
		opacity = 0
	} else if opacity > 1 {
		opacity = 1
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", c.R, c.G, c.B, opacity)
}

// workflowStudioThemedHTML injects the exact host Emacs viewport background
// tint (rgba(pal.Canvas, viewportOpacity)) into WorkflowStudioHTML so the
// native WKWebView matches *Dashboard*, *conversation*, and every other Window
// on its very first paint with zero full-transparent flash.
func workflowStudioThemedHTML(pal kit.Palette, viewportOpacity float32) string {
	if viewportOpacity <= 0 {
		viewportOpacity = kit.DefaultGlassOpacity
	}
	bg := rgbaCSS(pal.Canvas, viewportOpacity)
	override := fmt.Sprintf("<style id=\"marlin-viewport-theme\">:root{--canvas:%s;--viewport-opacity:%.3f;--viewport-bg:%s;}html,body{background:var(--viewport-bg) !important;}</style>\n</head>",
		hexRGB(pal.Canvas), viewportOpacity, bg)
	raw := kit.WorkflowStudioHTML(pal)
	if strings.Contains(raw, "</head>") {
		return strings.Replace(raw, "</head>", override, 1)
	}
	return override + raw
}

// webIslandViewportThemeScript wraps kit.WebIslandThemeScript and also updates
// --canvas, --marlin-canvas, --viewport-opacity, --viewport-bg, and the root
// html/body background so switching themes, toggling Liquid Glass, or cycling
// opacity keeps WebView islands visually identical to native GPU Windows.
func webIslandViewportThemeScript(pal kit.Palette, viewportOpacity float32) string {
	if viewportOpacity <= 0 {
		viewportOpacity = kit.DefaultGlassOpacity
	}
	bg := rgbaCSS(pal.Canvas, viewportOpacity)
	base := kit.WebIslandThemeScript(pal, viewportOpacity)
	return base + fmt.Sprintf(`(function(){
const bg=%q;
const canvas=%q;
const op=%.3f;
function applyViewportBg(){
  const r=document.documentElement.style;
  r.setProperty("--canvas",canvas);
  r.setProperty("--marlin-canvas",canvas);
  r.setProperty("--viewport-opacity",String(op));
  r.setProperty("--marlin-glass-opacity",String(op));
  r.setProperty("--viewport-bg",bg);
  r.setProperty("--marlin-viewport-bg",bg);
  r.background=bg;
  if(document.body){document.body.style.background=bg;}
}
applyViewportBg();
if(document.readyState==="loading"){window.addEventListener("DOMContentLoaded",applyViewportBg,{once:true});}
})();`, bg, hexRGB(pal.Canvas), viewportOpacity)
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

// PushArtifactStagePayload pushes a Vercel AI SDK `data-*` Generative UI
// artifact into the sandboxed Layer-3 Artifact Stage WebIsland via its
// coalescing FencedLane, lazily booting the island if not yet mounted.
func PushArtifactStagePayload(dataType, title, rawPayload string) {
	script := kit.ArtifactStageLoadScript(dataType, title, rawPayload)
	webIslandMu.Lock()
	lastArtifactScript = script
	inst := webViewCache[BuiltinArtifactStageIslandID]
	var lane *async.FencedLane[string, struct{}]
	if inst != nil {
		lane = inst.evalLane
	}
	webIslandMu.Unlock()
	if lane != nil {
		lane.Submit(script)
		return
	}
	_ = WebIslandHost().Resolve(BuiltinArtifactStageIslandID)
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

	// Default-DENY Tier-0 trust gate (HybridWebViewFocusLifecycle.qnt):
	// ONLY the explicit BuiltinWorkflowStudioIslandID without user override
	// receives TrustTier0FirstParty. Unregistered external island IDs fail closed.
	tier := async.TrustTier2Untrusted
	if hasUser {
		tier = async.TrustTier2Untrusted
	} else if islandID == BuiltinWorkflowStudioIslandID {
		tier = async.TrustTier0FirstParty
		html = workflowStudioThemedHTML(pal, op)
	} else if islandID == BuiltinArtifactStageIslandID {
		tier = async.TrustTier2Untrusted
		html = appleGeistArtifactStageHTML(pal, op)
	} else {
		webIslandMu.Lock()
		delete(webViewPending, islandID)
		webIslandMu.Unlock()
		return nil
	}
	permitID, permitErr := webViewPermits.Admit()
	if permitErr != nil {
		webIslandMu.Lock()
		delete(webViewPending, islandID)
		webIslandMu.Unlock()
		return nil
	}
	preload := webIslandViewportThemeScript(pal, op)
	lc := async.NewWebViewLifecycle(tier)
	lc.RequestAsyncMount()

	if !webViewScope.Go(func(ctx context.Context) {
		webViewPermits.Start(permitID)
		defer webViewPermits.Drain(permitID)
		if ctx.Err() != nil {
			webViewPermits.BeginCancelCleanup(permitID)
			webIslandMu.Lock()
			delete(webViewPending, islandID)
			webIslandMu.Unlock()
			return
		}
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
				e := evicted
				webViewScope.Go(func(_ context.Context) {
					if e.lifecycle != nil {
						e.lifecycle.HideTab()
					}
					if e.evalLane != nil {
						e.evalLane.Close()
					}
					if e.Destroy != nil {
						e.Destroy()
					}
				})
			}
		}
		latestPal := webIslandPalette
		latestOp := webIslandOpacity
		cached := webViewCache[islandID]
		artScript := ""
		if islandID == BuiltinArtifactStageIslandID {
			artScript = lastArtifactScript
		}
		webIslandMu.Unlock()

		webViewPermits.Complete(permitID)
		if err == nil && cached != nil {
			if cached.evalLane != nil {
				cached.evalLane.Submit(webIslandViewportThemeScript(latestPal, latestOp))
				if artScript != "" {
					cached.evalLane.Submit(artScript)
				}
			}
			if inv != nil {
				inv()
			}
		}
	}) {
		webViewPermits.Start(permitID)
		webViewPermits.BeginCancelCleanup(permitID)
		webViewPermits.Drain(permitID)
		webIslandMu.Lock()
		delete(webViewPending, islandID)
		webIslandMu.Unlock()
	}
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
