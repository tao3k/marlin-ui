// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// frameSnapshot is an immutable, defensively cloned RCU view of Frame state
// published via async.SnapshotStore[frameSnapshot]. Frame.View(c) reads it via
// a single wait-free atomic.Pointer Load (< 5ns, zero mutex acquisitions).
type frameSnapshot struct {
	Theme           theme.Theme
	ThemeCatalog    []theme.Theme
	SurfaceCfg      appconfig.SurfaceConfig
	ThemeSurfaces   map[string]appconfig.SurfaceConfig
	DashboardNode   *widgets.Node
	CustomSidebar   *widgets.SidebarSpec
	UIModules       widgets.Modules
	ZenMode         bool
	RailPinned      bool
	ActiveBuffer    buffer.ID
	ActiveFeatureID string
	ActiveExtID     string
	OpenTabs        []buffer.ID
	TableCursor     int
	ModelLabel      string
	SessionLabel    string
	StatusItems     []schema.UIStatusItem
	SchemeExts      []schema.UIExtensionRegion
	MessagesLog     []string
}

func (f *Frame) initAsyncRuntime() {
	f.pacer = async.NewFramePacer(f.invalidate, 8333*time.Microsecond)
	f.snapStore = async.NewSnapshotStore(f.captureSnapshotLocked())
	f.reloadLane = async.NewFencedLane[struct{}, appconfig.UIThemeBundle](
		func(ctx context.Context, _ struct{}) (appconfig.UIThemeBundle, error) {
			if f.onReloadThemeBundle == nil {
				return appconfig.UIThemeBundle{}, fmt.Errorf("reload unavailable")
			}
			if ctx.Err() != nil {
				return appconfig.UIThemeBundle{}, ctx.Err()
			}
			// Disk I/O runs completely off-lock outside Frame.mu!
			return f.onReloadThemeBundle(), nil
		},
		func(_ uint64, bundle appconfig.UIThemeBundle) {
			f.commitReloadedBundle(bundle)
		},
	)
}

func (f *Frame) captureSnapshotLocked() frameSnapshot {
	surfs := make(map[string]appconfig.SurfaceConfig, len(f.themeSurfaces))
	for k, v := range f.themeSurfaces {
		surfs[k] = v
	}
	return frameSnapshot{
		Theme:           f.Theme,
		ThemeCatalog:    append([]theme.Theme(nil), f.themeCatalog...),
		SurfaceCfg:      f.surfaceCfg,
		ThemeSurfaces:   surfs,
		DashboardNode:   f.dashboardNode,
		CustomSidebar:   f.customSidebar,
		UIModules:       f.uiModules,
		ZenMode:         f.zenMode,
		RailPinned:      f.railPinned,
		ActiveBuffer:    f.activeBuffer,
		ActiveFeatureID: f.activeFeatureID,
		ActiveExtID:     f.activeExtID,
		OpenTabs:        append([]buffer.ID(nil), f.openTabs...),
		TableCursor:     f.tableCursor,
		ModelLabel:      f.modelLabel,
		SessionLabel:    f.sessionLabel,
		StatusItems:     append([]schema.UIStatusItem(nil), f.statusItems...),
		SchemeExts:      append([]schema.UIExtensionRegion(nil), f.schemeExts...),
		MessagesLog:     append([]string(nil), f.messagesLog...),
	}
}

func (f *Frame) publishSnapshotLocked() {
	if f.snapStore == nil {
		return
	}
	snap := f.captureSnapshotLocked()
	f.snapStore.Commit(func(_ frameSnapshot) frameSnapshot { return snap })
}

// loadSnapshot performs a wait-free atomic.Pointer load on the render thread.
func (f *Frame) loadSnapshot() frameSnapshot {
	if f.snapStore != nil {
		if s := f.snapStore.Load(); s != nil {
			return s.Value
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.captureSnapshotLocked()
}

// ReloadThemesAsync schedules an off-lock, coalesced, epoch-fenced reload of
// ~/.config/marlin/ui/ (used by the live file watcher so disk I/O NEVER blocks
// the 120Hz GPU render thread).
func (f *Frame) ReloadThemesAsync() {
	if f.reloadLane != nil {
		f.reloadLane.Submit(struct{}{})
	}
}

func (f *Frame) commitReloadedBundle(bundle appconfig.UIThemeBundle) {
	f.mu.Lock()
	preferred := f.Theme.ID
	if strings.TrimSpace(bundle.Manifest.ActiveTheme) != "" {
		preferred = strings.TrimSpace(bundle.Manifest.ActiveTheme)
	}
	warnings := f.applyThemeBundleLocked(bundle, preferred, true)
	activeName := f.Theme.Name
	count := len(f.themeCatalog)
	for _, w := range warnings {
		f.logMessage("theme warning: " + w)
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()

	if f.onGlassChanged != nil {
		f.onGlassChanged(f.UIModules().Glass)
	}
	if len(warnings) > 0 {
		f.Echo("UI reload warning: " + warnings[0])
		return
	}
	f.Echo(fmt.Sprintf("Reloaded ~/.config/marlin/ui/ (%s · %d themes)", activeName, count))
}

// ShouldTickIdle reports whether a background 1Hz clock tick should trigger a
// redraw (false when unfocused or viewing non-Dashboard buffers, saving macOS
// transparent-window GPU power).
func (f *Frame) ShouldTickIdle() bool {
	if f.pacer == nil {
		return false
	}
	return f.pacer.ShouldTickIdle()
}

// FrameTelemetry returns (framesRendered, slowFramesOverBudget, maxFrameDuration).
func (f *Frame) FrameTelemetry() (uint64, uint64, time.Duration) {
	if f.pacer == nil {
		return 0, 0, 0
	}
	return f.pacer.Telemetry()
}

// Close cancels and drains all background async lanes cleanly (quiescence barrier).
func (f *Frame) Close() {
	if f.reloadLane != nil {
		f.reloadLane.Close()
	}
}

func effectiveSurfaceFromSnap(snap frameSnapshot) (glassOp, chromeOp, viewportOp float32, vibrancy string) {
	root := snap.SurfaceCfg
	pack := snap.ThemeSurfaces[snap.Theme.ID]
	runtimeOp := snap.UIModules.GlassOpacity

	glassOp = kit.ResolveGlassOpacity(kit.SurfaceOptions{}, snap.Theme.Sheet)
	if pack.GlassOpacity != nil {
		glassOp = *pack.GlassOpacity
	}
	if root.GlassOpacity != nil {
		glassOp = *root.GlassOpacity
	}
	if runtimeOp > 0 {
		glassOp = runtimeOp
	}

	chromeOp = 0.18
	if pack.ChromeOpacity != nil {
		chromeOp = *pack.ChromeOpacity
	}
	if root.ChromeOpacity != nil {
		chromeOp = *root.ChromeOpacity
	}

	viewportOp = max(glassOp*0.62, 0.16)
	if pack.ViewportOpacity != nil {
		viewportOp = *pack.ViewportOpacity
	}
	if root.ViewportOpacity != nil {
		viewportOp = *root.ViewportOpacity
	}

	vibrancy = "auto"
	if pack.Vibrancy != "" {
		vibrancy = pack.Vibrancy
	}
	if root.Vibrancy != "" {
		vibrancy = root.Vibrancy
	}
	return glassOp, chromeOp, viewportOp, vibrancy
}
