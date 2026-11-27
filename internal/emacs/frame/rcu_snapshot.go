// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/emacs/workspace"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

// frameSnapshot is an immutable, defensively cloned RCU view of Frame state
// published via async.SnapshotStore[frameSnapshot]. Frame.View(c) reads it via
// a single wait-free atomic.Pointer Load (< 5ns, zero mutex acquisitions).
type frameSnapshot struct {
	Theme           theme.Theme
	MyGoTheme       *ui.Theme
	ThemeCatalog    []theme.Theme
	SurfaceCfg      appconfig.SurfaceConfig
	ThemeSurfaces   map[string]appconfig.SurfaceConfig
	DashboardNode   *widgets.Node
	CustomSidebar   *widgets.SidebarSpec
	UIModules       widgets.Modules
	ZenMode         bool
	ZenStyle        ZenStyle
	RailPinned      bool
	ActiveBuffer    buffer.ID
	ActiveFeatureID string
	ActiveExtID     string
	ActivePluginID  string
	MountedPlugins  []marketplace.ManagedPackage
	OpenTabs        []buffer.ID
	Workspaces      workspace.Snapshot
	TableCursor     int
	PackageFilter   string
	ModelLabel      string
	SessionLabel    string
	StatusItems     []schema.UIStatusItem
	SchemeExts      []schema.UIExtensionRegion
	MessagesLog     []string
	LocalTasks      []widgets.TaskItem
	TomlHydras      map[string]declarativeHydraDef
	Magit           *buffer.MagitViewState
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
	f.wbSyncLane = async.NewFencedLane[schema.UIWorkbenchSnapshot, schema.UIWorkbenchSnapshot](
		func(ctx context.Context, snap schema.UIWorkbenchSnapshot) (schema.UIWorkbenchSnapshot, error) {
			if ctx.Err() != nil {
				return schema.UIWorkbenchSnapshot{}, ctx.Err()
			}
			return snap, nil
		},
		func(_ uint64, snap schema.UIWorkbenchSnapshot) {
			f.commitGRPCWorkbenchSnapshot(snap)
		},
	)
	f.modesSyncLane = async.NewFencedLane[struct{}, []modes.ModeRecord](
		func(ctx context.Context, _ struct{}) ([]modes.ModeRecord, error) {
			if f.Link == nil || f.Link.Switcher == nil {
				return nil, fmt.Errorf("no gRPC link")
			}
			raw, err := f.Link.FetchSchemeModeRecords(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]modes.ModeRecord, 0, len(raw))
			for _, rec := range raw {
				out = append(out, modes.ModeRecord{
					RecordID: rec.RecordID,
					Kind:     rec.Kind,
					Title:    rec.Title,
					Facets:   rec.Facets,
				})
			}
			return out, nil
		},
		func(_ uint64, recs []modes.ModeRecord) {
			if n := f.ApplySchemeModeRecords(recs); n > 0 {
				f.mu.Lock()
				f.logMessage(fmt.Sprintf("synced %d live Scheme ui-modes record(s) over gRPC", n))
				f.publishSnapshotLocked()
				f.mu.Unlock()
			}
		},
	)
	if f.Link != nil && f.Link.Switcher != nil {
		f.modesSyncLane.Submit(struct{}{})
	}
	f.initMagitLane()
}

func (f *Frame) captureSnapshotLocked() frameSnapshot {
	surfs := make(map[string]appconfig.SurfaceConfig, len(f.themeSurfaces))
	for k, v := range f.themeSurfaces {
		surfs[k] = v
	}
	hydras := make(map[string]declarativeHydraDef, len(f.tomlHydras))
	for k, v := range f.tomlHydras {
		hydras[k] = v
	}
	var wsSnap workspace.Snapshot
	if f.wsMgr != nil {
		wsSnap = f.wsMgr.Snapshot()
	}
	return frameSnapshot{
		Theme:           f.Theme,
		MyGoTheme:       f.Theme.ToMyGoTheme(),
		ThemeCatalog:    append([]theme.Theme(nil), f.themeCatalog...),
		SurfaceCfg:      f.surfaceCfg,
		ThemeSurfaces:   surfs,
		DashboardNode:   f.dashboardNode,
		CustomSidebar:   f.customSidebar,
		UIModules:       f.uiModules,
		ZenMode:         f.zenMode,
		ZenStyle:        f.zenStyle,
		RailPinned:      f.railPinned,
		ActiveBuffer:    f.activeBuffer,
		ActiveFeatureID: f.activeFeatureID,
		ActiveExtID:     f.activeExtID,
		ActivePluginID:  f.activePluginID,
		MountedPlugins:  append([]marketplace.ManagedPackage(nil), f.mountedPlugins...),
		OpenTabs:        append([]buffer.ID(nil), f.openTabs...),
		Workspaces:      wsSnap,
		TableCursor:     f.tableCursor,
		PackageFilter:   f.packageFilter,
		ModelLabel:      f.modelLabel,
		SessionLabel:    f.sessionLabel,
		StatusItems:     append([]schema.UIStatusItem(nil), f.statusItems...),
		SchemeExts:      append([]schema.UIExtensionRegion(nil), f.schemeExts...),
		MessagesLog:     append([]string(nil), f.messagesLog...),
		LocalTasks:      append([]widgets.TaskItem(nil), f.localTasks...),
		TomlHydras:      hydras,
		Magit:           f.captureMagitViewLocked(),
	}
}

// hydraBodyFor resolves the active Hydra popup wait-free from an RCU snapshot
// (zero mutex acquisitions on the 120Hz render thread).
func (f *Frame) hydraBodyFor(snap frameSnapshot, prefix string) hydra.Body {
	def, ok := snap.TomlHydras[prefix]
	if !ok {
		for _, cand := range snap.TomlHydras {
			if cand.ID == prefix {
				def = cand
				ok = true
				break
			}
		}
	}
	if !ok {
		def = snap.TomlHydras["SPC"]
	}
	return f.materializeHydra(def)
}

func (f *Frame) notifyWebIslandVisibility(activeIslandID string) {
	if f.onWebIslandVisibility != nil {
		f.onWebIslandVisibility(activeIslandID)
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

// snapVersion returns the monotonic RCU snapshot generation in < 5ns (wait-free).
func (f *Frame) snapVersion() uint64 {
	if f.snapStore != nil {
		if s := f.snapStore.Load(); s != nil {
			return s.Gen
		}
	}
	return 0
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

// commitGRPCWorkbenchSnapshot applies a new marlin.ui.v1 workbench snapshot
// off the 120Hz render thread via wbSyncLane.
func (f *Frame) commitGRPCWorkbenchSnapshot(snap schema.UIWorkbenchSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if snap.Generation == f.syncedWBGen.Load() {
		return
	}
	f.syncedWBGen.Store(snap.Generation)
	if strings.TrimSpace(snap.ModelLabel) != "" {
		f.modelLabel = strings.TrimSpace(snap.ModelLabel)
	}
	if strings.TrimSpace(snap.SessionLabel) != "" {
		f.sessionLabel = strings.TrimSpace(snap.SessionLabel)
	}
	if len(snap.StatusItems) > 0 {
		f.statusItems = append([]schema.UIStatusItem(nil), snap.StatusItems...)
	}
	if len(snap.Commands) > 0 {
		f.schemeCmds = append([]schema.UICommandSpec(nil), snap.Commands...)
	}
	if proj, err := uiframework.ProjectWorkbench(uiframework.DefaultRegistry(), desktopTargetKind(), snap); err == nil {
		if len(snap.Slots) > 0 {
			f.assembledSurface = proj.Surface
		}
		if len(snap.Extensions) > 0 {
			f.schemeExts = proj.Extensions
			if proj.RejectedExtensions > 0 {
				f.logMessage(fmt.Sprintf("marlin.ui.v1 rejected %d unbounded/invalid extension(s)", proj.RejectedExtensions))
			}
		}
		if len(proj.Hydras) > 0 {
			f.liveSchemeHydras = projectMappedHydras(proj.Hydras)
			f.rebuildEffectiveHydrasLocked()
		}
		if len(proj.Windows) > 0 && f.wsMgr != nil {
			specs := make([]workspace.SchemeWindowSpec, 0, len(proj.Windows))
			for _, w := range proj.Windows {
				specs = append(specs, workspace.SchemeWindowSpec{
					ID:     w.ID,
					Buffer: buffer.ID(w.BufferID),
				})
			}
			wsSnap := f.wsMgr.SyncSchemeWindows(specs, proj.SelectedWindowID)
			f.activeBuffer = wsSnap.Active.ActiveBuffer
			f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
			if strings.HasPrefix(string(f.activeBuffer), uiframework.SchemeExtBufferPrefix) && wsSnap.Active.WindowCount() <= 1 {
				f.activeExtID = strings.TrimPrefix(string(f.activeBuffer), uiframework.SchemeExtBufferPrefix)
			} else {
				f.activeExtID = ""
			}
		}
	}
	if !f.themeUserSet && snap.Theme.Canvas != snap.Theme.Ink && snap.ProfileName != "" && !strings.HasPrefix(snap.ProfileName, "adaptive-") {
		f.Theme = theme.FromUITheme(snap.ProfileName, snap.ProfileName, snap.Theme)
		f.logMessage("synced marlin.ui.v1 workbench profile=" + snap.ProfileName)
	}
	f.publishSnapshotLocked()
	f.requestFrame()
}

// Close cancels and drains all background async lanes cleanly (quiescence barrier).
func (f *Frame) Close() {
	if f.reloadLane != nil {
		f.reloadLane.Close()
	}
	if f.wbSyncLane != nil {
		f.wbSyncLane.Close()
	}
	if f.modesSyncLane != nil {
		f.modesSyncLane.Close()
	}
	f.closePackageStore()
}

func effectiveSurfaceFromSnap(snap frameSnapshot) (glassOp, chromeOp, viewportOp float32, vibrancy string) {
	defaults := appconfig.DefaultSurfaceConfig()
	root := snap.SurfaceCfg
	pack := snap.ThemeSurfaces[snap.Theme.ID]
	runtimeOp := snap.UIModules.GlassOpacity

	glassOp = kit.ResolveGlassOpacity(kit.SurfaceOptions{}, snap.Theme.Sheet)
	if defaults.GlassOpacity != nil {
		glassOp = *defaults.GlassOpacity
	}
	if pack.GlassOpacity != nil {
		glassOp = *pack.GlassOpacity
	}
	if root.GlassOpacity != nil {
		glassOp = *root.GlassOpacity
	}
	if runtimeOp > 0 {
		glassOp = runtimeOp
	}

	if defaults.ChromeOpacity != nil {
		chromeOp = *defaults.ChromeOpacity
	}
	if pack.ChromeOpacity != nil {
		chromeOp = *pack.ChromeOpacity
	}
	if root.ChromeOpacity != nil {
		chromeOp = *root.ChromeOpacity
	}

	if defaults.ViewportOpacity != nil {
		viewportOp = *defaults.ViewportOpacity
	}
	if pack.ViewportOpacity != nil {
		viewportOp = *pack.ViewportOpacity
	}
	if root.ViewportOpacity != nil {
		viewportOp = *root.ViewportOpacity
	}
	if runtimeOp > 0 {
		viewportOp = runtimeOp
	}

	vibrancy = defaults.Vibrancy
	if pack.Vibrancy != "" {
		vibrancy = pack.Vibrancy
	}
	if root.Vibrancy != "" {
		vibrancy = root.Vibrancy
	}
	return glassOp, chromeOp, viewportOp, vibrancy
}
