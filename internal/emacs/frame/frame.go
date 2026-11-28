// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package frame composes Marlin UI's top-level Doom Emacs Frame out of the
// hierarchical internal/emacs/* packages (theme, meow, hydra, tabs, window,
// buffer, modeline, minibuffer) and synchronizes live marlin.ui.v1 gRPC state.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A01 (emacsFrame).
package frame

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/emacs/workspace"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	"github.com/tao3k/marlin-ui/internal/shellmanifest"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

// Config configures Marlin UI's Doom Emacs Frame.
type Config struct {
	Adaptive            *mygoshell.AdaptiveShell
	Link                *hostlink.Link
	AppConfig           appconfig.Config
	Prefs               appconfig.UserPrefs
	FeatureReg          *features.Registry
	FeatureLedger       *features.GovernanceLedger
	FeatureHost         *features.Host
	PackageStore        *marketplace.PackageStore
	WebHost               kit.WebIslandHost
	OnWebIslandAction     func(islandID, event string) bool
	OnWebIslandVisibility func(activeIslandID string)
	OnPushArtifact        func(dataType, title, payload string)
	InitialThemeID        string
	ThemeBundle         appconfig.UIThemeBundle
	OnReloadThemeBundle func() appconfig.UIThemeBundle
	OnGlassChanged      func(enabled bool)
	OnThemeChanged      func(isLight bool)
	OnZenModeChanged    func(writeroom, fullscreen bool)
	OnQuit              func()
	RoomStats           func() (done, total, onlineAgents, totalAgents int)
	RoomTasks           func() []widgets.TaskItem
	OnAddTask           func(title string) error
	OnCycleTask         func(id string) error
	OnPrefsChanged      func(appconfig.UserPrefs)
	OnApplyFeatures     func(spec string)
	Invalidate          func()
}

// ZenStyle selects Doom Emacs `:ui zen` presentation mode:
//   - ZenOff: normal Emacs workbench chrome & sidebars
//   - ZenWriteroom (SPC t z): centered focus / writeroom-mode without OS fullscreen
//   - ZenFullscreen (SPC t Z): centered focus / writeroom-mode + native OS fullscreen
type ZenStyle uint8

const (
	ZenOff ZenStyle = iota
	ZenWriteroom
	ZenFullscreen
)

// Frame owns the top-level Emacs Frame state and GPU ui.View entry point.
type Frame struct {
	mu sync.Mutex

	Adaptive      *mygoshell.AdaptiveShell
	Link          *hostlink.Link
	FeatureReg    *features.Registry
	FeatureLedger *features.GovernanceLedger
	FeatureHost   *features.Host
	WebHost       kit.WebIslandHost
	AppCfg        appconfig.Config
	Prefs         appconfig.UserPrefs

	Theme           theme.Theme
	themeCatalog    []theme.Theme
	surfaceCfg      appconfig.SurfaceConfig
	themeSurfaces   map[string]appconfig.SurfaceConfig
	userTokensJSON  string
	userOverrideCSS string
	dashboardNode      *widgets.Node
	customSidebar      *widgets.SidebarSpec
	tomlCommands       []declarativeCommand
	tomlHydras         map[string]declarativeHydraDef
	desktopTOMLOverlay map[string]declarativeHydraDef
	userTOMLOverlay    map[string]declarativeHydraDef
	liveSchemeHydras   map[string]declarativeHydraDef
	assembledSurface   uiframework.AssembledSurface
	uiModules          widgets.Modules
	zenMode         bool
	zenStyle        ZenStyle
	railPinned      bool
	dashPromptDraft   string
	dashTaskDraft     string
	convComposerDraft string
	localTasks        []widgets.TaskItem
	startedAt       time.Time
	themeUserSet    bool
	syncedWBGen     atomic.Uint64
	modelLabel      string
	sessionLabel    string
	statusItems     []schema.UIStatusItem
	schemeExts      []schema.UIExtensionRegion
	schemeCmds      []schema.UICommandSpec

	activeBuffer    buffer.ID
	activeFeatureID string
	activeExtID     string
	openTabs        []buffer.ID
	tableCursor     int
	messagesLog     []string

	meowState          *meow.State
	miniState          *minibuffer.State
	modeReg            *modes.Registry
	modeHooksInstalled bool
	bufModes           map[buffer.ID]modes.BufferState
	chatSessions       map[buffer.ID]string
	activeSpanCode     string
	activeSpanLang     string
	helpData           *buffer.HelpIntrospectionData
	ibufferMarked      map[buffer.ID]bool
	ibufferFilter      string
	recentCommands     []string
	lossage            []LossageEntry
	lastAction         string
	universalArg       int
	chatSeq            int
	customize      *buffer.CustomizeState
	wsMgr          *workspace.Manager
	thinkingMu          sync.RWMutex
	thinkingOpened      map[string]bool
	activeArtifact      *buffer.ArtifactSelection
	filePreview         *buffer.FilePreviewState
	composerMu          sync.Mutex
	composerOpen        bool
	composerDockHovered bool
	composerHoverStart  time.Time
	composerClickStart  time.Time

	// Deterministic RCU render snapshot + coalesced off-lock reload lane + 120Hz frame pacer.
	snapStore      *async.SnapshotStore[frameSnapshot]
	reloadLane     *async.FencedLane[struct{}, appconfig.UIThemeBundle]
	wbSyncLane     *async.FencedLane[schema.UIWorkbenchSnapshot, schema.UIWorkbenchSnapshot]
	modesSyncLane  *async.FencedLane[struct{}, []modes.ModeRecord]
	pacer          *async.FramePacer
	pkgStore       *marketplace.PackageStore
	pkgRefreshLane *async.FencedLane[struct{}, marketplace.PackageStoreSnapshot]
	packageFilter  string
	mountedPlugins []marketplace.ManagedPackage
	activePluginID string

	magitSnap        hostlink.MagitSnapshot
	magitLoading     bool
	magitErr         string
	magitCursor      int
	magitVisLevel    int
	magitFolded      map[string]bool
	magitAuditMap    map[string]string
	magitPromptKind  string
	magitPromptLabel string
	magitRefreshLane *async.FencedLane[string, magitRefreshResult]
	magitExecLane    *async.FencedLane[string, string]

	findFileDir       string
	findFileParent    string
	findFileHome      string
	findFileEntries   []hostlink.FileEntry
	findFileCandCache []minibuffer.Candidate
	findFileLoading   bool
	findFileErr       string
	projFilesRoot     string
	projFilesEntries  []hostlink.FileEntry
	projFileCandCache []minibuffer.Candidate
	projFilesLoading  bool
	projFilesErr      string
	recentFiles       []string
	dirListLane      *async.FencedLane[string, dirLoadResult]
	projFilesLane    *async.FencedLane[string, projectFilesResult]
	fileReadLane     *async.FencedLane[string, fileReadResult]

	onReloadThemeBundle func() appconfig.UIThemeBundle
	onGlassChanged      func(enabled bool)
	onThemeChanged      func(isLight bool)
	onZenModeChanged    func(writeroom, fullscreen bool)
	onQuit              func()
	refocusFrame        atomic.Bool
	inView              atomic.Bool
	roomStatsFn         func() (done, total, onlineAgents, totalAgents int)
	roomTasksFn         func() []widgets.TaskItem
	onAddTaskFn         func(title string) error
	onCycleTaskFn       func(id string) error
	onPrefsChanged      func(appconfig.UserPrefs)
	onApplyFeatures       func(spec string)
	onWebIslandAction     func(islandID, event string) bool
	onWebIslandVisibility func(activeIslandID string)
	onPushArtifact        func(dataType, title, payload string)
	invalidate            func()
}

// New initializes Marlin UI's Doom Emacs Frame.
func New(cfg Config) *Frame {
	inv := cfg.Invalidate
	if inv == nil {
		inv = func() {}
	}
	f := &Frame{
		Adaptive:            cfg.Adaptive,
		Link:                cfg.Link,
		FeatureReg:          cfg.FeatureReg,
		FeatureLedger:       cfg.FeatureLedger,
		FeatureHost:         cfg.FeatureHost,
		WebHost:             cfg.WebHost,
		AppCfg:              cfg.AppConfig,
		Prefs:               cfg.Prefs,
		Theme:               theme.DoomOne(),
		themeCatalog:        theme.Catalog(),
		dashboardNode:       widgets.DefaultDashboard(nil),
		uiModules:           widgets.DefaultModules(),
		startedAt:           time.Now(),
		activeBuffer:        buffer.Dashboard,
		openTabs:            []buffer.ID{buffer.Dashboard, buffer.Conversation},
		meowState:           meow.New(),
		miniState:           minibuffer.New(),
		customize:           buffer.NewCustomizeState(cfg.AppConfig, cfg.Prefs),
		wsMgr:               workspace.NewManager([]buffer.ID{buffer.Dashboard, buffer.Conversation}),
		thinkingOpened:      make(map[string]bool),
		onReloadThemeBundle: cfg.OnReloadThemeBundle,
		onGlassChanged:      cfg.OnGlassChanged,
		onThemeChanged:      cfg.OnThemeChanged,
		onZenModeChanged:    cfg.OnZenModeChanged,
		onQuit:              cfg.OnQuit,
		roomStatsFn:         cfg.RoomStats,
		roomTasksFn:         cfg.RoomTasks,
		onAddTaskFn:         cfg.OnAddTask,
		onCycleTaskFn:       cfg.OnCycleTask,
		onPrefsChanged:      cfg.OnPrefsChanged,
		onApplyFeatures:       cfg.OnApplyFeatures,
		onWebIslandAction:     cfg.OnWebIslandAction,
		onWebIslandVisibility: cfg.OnWebIslandVisibility,
		onPushArtifact:        cfg.OnPushArtifact,
		invalidate:            inv,
	}
	preferred := strings.TrimSpace(cfg.InitialThemeID)
	if preferred == "" {
		preferred = strings.TrimSpace(cfg.ThemeBundle.Manifest.ActiveTheme)
	}
	warnings := f.applyThemeBundleLocked(cfg.ThemeBundle, preferred, false)
	f.messagesLog = []string{
		fmt.Sprintf("[%s] marlin-ui started (theme=%s, themes=%d, session=%s)", time.Now().Format("15:04:05"), f.Theme.Name, len(f.themeCatalog), cfg.AppConfig.SessionID),
	}
	for _, w := range warnings {
		f.logMessage("theme warning: " + w)
	}
	if cfg.Link != nil {
		f.logMessage(fmt.Sprintf("hostlink mode=%s endpoint=%s", cfg.Link.Mode, cfg.Link.Endpoint))
	}
	f.restoreSavedWorkspaces()
	f.initPackageStore(cfg.PackageStore)
	if assembled, err := uiframework.DefaultRegistry().AssembleSurface(desktopTargetKind(), nil); err == nil {
		f.assembledSurface = assembled
	}
	f.initAsyncRuntime()
	return f
}

func desktopTargetKind() shellmanifest.TargetKind {
	switch runtime.GOOS {
	case "darwin":
		return shellmanifest.TargetDesktopMacOS
	case "windows":
		return shellmanifest.TargetDesktopWindows
	default:
		return shellmanifest.TargetDesktopLinux
	}
}

// AssembledSurface returns Desktop's validated modular UI Component surface
// (shared core modules + Desktop Emacs modules + live Scheme slot policies).
func (f *Frame) AssembledSurface() uiframework.AssembledSurface {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.assembledSurface
}

// ApplyWorkbenchSnapshot admits a live `marlin.ui.v1` Scheme/Rust workbench
// snapshot directly into Frame state (updating live Scheme keymaps, Hydra
// transients, status items, commands, extensions, and slot trust policies).
func (f *Frame) ApplyWorkbenchSnapshot(snap schema.UIWorkbenchSnapshot) {
	f.commitGRPCWorkbenchSnapshot(snap)
}

// rebuildEffectiveHydrasLocked composes the 3-tier Doom/Emacs keymap hierarchy:
//  1. Authoritative Scheme gRPC (`marlin.ui.v1` `snap.Keymap` + `snap.Transients`
//     from `scheme/ui-meow` & `scheme/ui-magit`), falling back to `loadSchemeBaseKeymaps()`
//     only when running standalone without a live Rust authority host.
//  2. Desktop-Special Extension (`defaults/keymaps.toml`) — strictly Desktop-only
//     GUI toggles (Liquid Glass, Motion, Settings, Packages), never duplicating Scheme.
//  3. Downstream User Override (`~/.config/marlin/ui/keymaps.toml`) — overrides or
//     extends any Scheme or Desktop key, keypad, or transient.
func (f *Frame) rebuildEffectiveHydrasLocked() {
	var eff map[string]declarativeHydraDef
	if len(f.liveSchemeHydras) > 0 {
		eff = mergeHydraOverlay(nil, f.liveSchemeHydras)
	} else {
		eff = loadStandaloneBootstrapKeymaps()
	}
	if len(f.desktopTOMLOverlay) > 0 {
		eff = mergeHydraOverlay(eff, f.desktopTOMLOverlay)
	}
	if len(f.userTOMLOverlay) > 0 {
		eff = mergeHydraOverlay(eff, f.userTOMLOverlay)
	}
	f.tomlHydras = eff
}

func (f *Frame) logMessage(line string) {
	ts := time.Now().Format("15:04:05")
	f.messagesLog = append(f.messagesLog, fmt.Sprintf("[%s] %s", ts, line))
	if len(f.messagesLog) > 256 {
		f.messagesLog = f.messagesLog[len(f.messagesLog)-256:]
	}
}

// Echo writes a concise message to the Emacs Echo Area and *Messages* buffer.
func (f *Frame) Echo(msg string) {
	f.miniState.Echo(msg, 4*time.Second)
	f.mu.Lock()
	f.logMessage(msg)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

func (f *Frame) requestFrame() {
	// Skip redundant async win.Invalidate() while already inside Frame.View(c):
	// MyGo is actively building & painting this exact frame right now.
	if f.inView.Load() {
		return
	}
	if f.pacer != nil {
		f.pacer.RequestFrame()
		return
	}
	if f.invalidate != nil {
		f.invalidate()
	}
}

// IsLightTheme reports whether the currently active theme pack is a light palette.
func (f *Frame) IsLightTheme() bool {
	return f.loadSnapshot().Theme.IsLight
}

// SetTheme switches Marlin UI's active Doom Emacs CSS theme pack (cascading any
// admitted ~/.config/marlin/ui/theme.css override) and persists the selection.
func (f *Frame) SetTheme(id string) {
	f.mu.Lock()
	th := f.findThemeLocked(id)
	if strings.TrimSpace(f.userTokensJSON) != "" {
		if cascaded, err := th.WithDesignTokensJSON(f.userTokensJSON); err == nil {
			th = cascaded
		}
	}
	if strings.TrimSpace(f.userOverrideCSS) != "" {
		if cascaded, err := th.WithOverrideCSS(f.userOverrideCSS); err == nil {
			th = cascaded
		}
	}
	f.Theme = th
	f.themeUserSet = true
	f.Prefs.ThemeID = th.ID
	prefs := f.Prefs
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if f.onThemeChanged != nil {
		f.onThemeChanged(th.IsLight)
	}
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
	f.Echo("Theme: " + th.Name)
}

// CycleTheme advances deterministically to the next theme pack in the catalog.
func (f *Frame) CycleTheme() {
	snap := f.loadSnapshot()
	nextID := theme.DefaultThemeID
	if len(snap.ThemeCatalog) > 0 {
		nextID = snap.ThemeCatalog[0].ID
		for i, th := range snap.ThemeCatalog {
			if th.ID == snap.Theme.ID {
				nextID = snap.ThemeCatalog[(i+1)%len(snap.ThemeCatalog)].ID
				break
			}
		}
	}
	f.SetTheme(nextID)
}

// SelectFeature focuses an opted-in feature buffer (e.g. "feature:chatroom") or Scheme extension.
// When multi-window split mode is active, Scheme extensions (`scheme-ext:<id>`)
// open as first-class buffers inside the focused split pane so the user's
// multi-window layout stays intact.
func (f *Frame) SelectFeature(fid string) {
	if f.selectMountedPlugin(fid) {
		return
	}
	if strings.HasPrefix(fid, uiframework.SchemeExtBufferPrefix) && f.wsMgr != nil && f.wsMgr.Snapshot().Active.WindowCount() > 1 {
		f.SelectBuffer(buffer.ID(fid))
		return
	}
	wasMini := f.miniState.IsCompletionActive()
	f.miniState.Close()
	f.meowState.ClearKeypad()
	f.mu.Lock()
	wasWeb := f.activeFeatureID == "feature:webisland" || f.activePluginID != ""
	f.activePluginID = ""
	if strings.HasPrefix(fid, "scheme-ext:") {
		f.activeExtID = strings.TrimPrefix(fid, "scheme-ext:")
		f.activeFeatureID = ""
	} else {
		f.activeFeatureID = fid
		f.activeExtID = ""
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if fid == "feature:webisland" {
		f.notifyWebIslandVisibility("workflow-graph")
	} else {
		f.notifyWebIslandVisibility("")
		if wasWeb || wasMini {
			f.refocusFrame.Store(true)
		}
	}
	f.requestFrame()
}

// ToggleZenMode toggles Doom Emacs `:ui zen` Writeroom Mode (SPC t z):
// centered focus column (hiding tabs, modeline, and peripheral chat rails)
// without forcing OS fullscreen. Exiting from ZenFullscreen restores normal mode.
func (f *Frame) ToggleZenMode() {
	f.mu.Lock()
	if f.zenStyle == ZenOff {
		f.zenStyle = ZenWriteroom
	} else {
		f.zenStyle = ZenOff
	}
	f.zenMode = f.zenStyle != ZenOff
	style := f.zenStyle
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if f.onZenModeChanged != nil {
		f.onZenModeChanged(style != ZenOff, style == ZenFullscreen)
	}
	f.refocusFrame.Store(true)
	if style == ZenWriteroom {
		f.Echo("Zen Writeroom enabled (centered focus · SPC t z to exit, SPC t Z fullscreen)")
	} else {
		f.Echo("Zen Mode disabled")
	}
}

// ToggleZenFullscreen toggles Doom Emacs `+zen/toggle-fullscreen` (SPC t Z):
// Writeroom centered focus combined with native OS fullscreen.
func (f *Frame) ToggleZenFullscreen() {
	f.mu.Lock()
	if f.zenStyle == ZenFullscreen {
		f.zenStyle = ZenOff
	} else {
		f.zenStyle = ZenFullscreen
	}
	f.zenMode = f.zenStyle != ZenOff
	style := f.zenStyle
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if f.onZenModeChanged != nil {
		f.onZenModeChanged(style != ZenOff, style == ZenFullscreen)
	}
	f.refocusFrame.Store(true)
	if style == ZenFullscreen {
		f.Echo("Zen Fullscreen enabled (writeroom + fullscreen · SPC t Z to exit)")
	} else {
		f.Echo("Zen Fullscreen disabled")
	}
}

// ZenMode reports whether immersive Zen Mode (Writeroom or Fullscreen) is active.
func (f *Frame) ZenMode() bool {
	return f.loadSnapshot().ZenMode
}

// ZenFullscreen reports whether Zen Fullscreen mode is currently active.
func (f *Frame) ZenFullscreen() bool {
	return f.loadSnapshot().ZenStyle == ZenFullscreen
}

// CurrentZenStyle returns the active ZenStyle (ZenOff, ZenWriteroom, or ZenFullscreen).
func (f *Frame) CurrentZenStyle() ZenStyle {
	return f.loadSnapshot().ZenStyle
}

// ToggleGlass hot-swaps the optional GPU Liquid Glass surface module via
// marlin-mygo/engine/features.Registry (audited via GovernanceLedger).
func (f *Frame) ToggleGlass() {
	next := !f.UIModules().Glass
	if f.FeatureReg != nil {
		if err := f.FeatureReg.SetEnabled(features.FeatureID("ui.glass"), next, f.FeatureLedger, f.FeatureHost); err == nil {
			f.mu.Lock()
			f.uiModules.Glass = next
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.refocusFrame.Store(true)
			if f.onGlassChanged != nil {
				f.onGlassChanged(next)
			}
			if next {
				f.Echo("Hot-plugged module: ui.glass (Liquid Glass)")
			} else {
				f.Echo("Unplugged module: ui.glass (Matte)")
			}
			return
		}
	}
	f.mu.Lock()
	f.uiModules.Glass = next
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if f.onGlassChanged != nil {
		f.onGlassChanged(next)
	}
	if next {
		f.Echo("Module enabled: Liquid Glass surface")
	} else {
		f.Echo("Module disabled: Matte surface")
	}
}

// ToggleMotion hot-swaps the optional 120Hz Spring Motion module via
// marlin-mygo/engine/features.Registry (audited via GovernanceLedger).
func (f *Frame) ToggleMotion() {
	next := !f.UIModules().Motion
	if f.FeatureReg != nil {
		if err := f.FeatureReg.SetEnabled(features.FeatureID("ui.motion"), next, f.FeatureLedger, f.FeatureHost); err == nil {
			f.mu.Lock()
			f.uiModules.Motion = next
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.refocusFrame.Store(true)
			if next {
				f.Echo("Hot-plugged module: ui.motion (120Hz Spring)")
			} else {
				f.Echo("Unplugged module: ui.motion (Static)")
			}
			return
		}
	}
	f.mu.Lock()
	f.uiModules.Motion = next
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if next {
		f.Echo("Module enabled: 120Hz Spring Motion")
	} else {
		f.Echo("Module disabled: Static low-power mode")
	}
}

// UIModules returns a live snapshot of hot-pluggable UI modules backed by
// marlin-mygo/engine/features.Registry when mounted.
func (f *Frame) UIModules() widgets.Modules {
	snap := f.loadSnapshot()
	m := snap.UIModules
	m.Zen = snap.ZenMode
	if f.FeatureReg != nil {
		for _, st := range f.FeatureReg.Snapshot() {
			switch st.Meta.ID {
			case features.FeatureID("ui.glass"):
				m.Glass = st.Enabled
			case features.FeatureID("ui.motion"):
				m.Motion = st.Enabled
			}
		}
	}
	return m
}

// ReloadThemes hot-reloads ~/.config/marlin/ui/ themes, CSS overrides, and
// declarative widgets fail-closed (disk I/O runs completely off-lock).
func (f *Frame) ReloadThemes() {
	if f.onReloadThemeBundle == nil {
		f.Echo("Theme reload unavailable in this host mode")
		return
	}
	bundle := f.onReloadThemeBundle()
	f.commitReloadedBundle(bundle)
}

// OpenHydra pops up the Doom Leader / Hydra Which-Key drawer for prefix (e.g. "SPC", "SPC b", "SPC t", "SPC m").
func (f *Frame) OpenHydra(prefix string) {
	f.miniState.Close()
	f.meowState.EnterNormal()
	f.meowState.SetKeypad(prefix)
	f.requestFrame()
}

// EnterInsert transitions Meow to INSERT mode and focuses the Minibuffer prompt,
// enforcing Emacs `read-only-mode` when the active chat buffer is locked read-only.
func (f *Frame) EnterInsert() {
	active := f.ActiveBuffer()
	if buffer.IsChatBuffer(active) && f.EffectiveModeStack(active).ReadOnly {
		f.Echo("Buffer " + string(active) + " is read-only (toggle with SPC t r / C-c t r)")
		return
	}
	f.meowState.EnterInsert()
	f.miniState.RequestFocus()
	f.requestFrame()
}

// OpenMinibuffer activates the bottom Emacs Minibuffer in the given mode.
func (f *Frame) OpenMinibuffer(mode minibuffer.Mode) {
	f.meowState.ClearKeypad()
	f.miniState.Open(mode)
	if mode == minibuffer.ModeHelm && f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.HelmSearch != nil {
		seed := f.helmInitialQuery()
		if seed != "" {
			f.miniState.SetInput(seed)
		}
		gen := f.Adaptive.Desktop.HelmSearch.SetQuery(seed)
		f.miniState.MarkHelmPending(gen)
	}
	f.requestFrame()
}

// OpenHelm opens 120Hz Helm live search inside the Minibuffer (SPC s / Cmd+P).
func (f *Frame) OpenHelm() { f.OpenMinibuffer(minibuffer.ModeHelm) }

// OpenCommands opens Vertico M-x commands inside the Minibuffer (SPC SPC / M-x / Cmd+K).
func (f *Frame) OpenCommands() { f.OpenMinibuffer(minibuffer.ModeCommand) }

// OpenBufferSwitch opens Vertico buffer selection (SPC b b / Cmd+B).
func (f *Frame) OpenBufferSwitch() { f.OpenMinibuffer(minibuffer.ModeBufferSwitch) }

// OpenThemeSwitch opens Vertico theme profile selection (SPC t t).
func (f *Frame) OpenThemeSwitch() { f.OpenMinibuffer(minibuffer.ModeThemeSwitch) }

// OpenGateReview opens the authentic Emacs y-or-n-p approval prompt in the Minibuffer.
func (f *Frame) OpenGateReview() { f.OpenMinibuffer(minibuffer.ModeGateReview) }

// OpenSettings focuses the *Settings* Emacs customize buffer (Cmd+,).
func (f *Frame) OpenSettings() { f.SelectBuffer(buffer.Settings) }

// ActiveBuffer returns the currently focused Emacs buffer ID.
func (f *Frame) ActiveBuffer() buffer.ID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activeBuffer
}

// KeyMode returns the current Meow modal state (Normal or Insert).
func (f *Frame) KeyMode() meow.Mode { return f.meowState.Mode() }

// KeypadPrefix returns the currently active SPC Hydra prefix (empty if none).
func (f *Frame) KeypadPrefix() string { return f.meowState.Keypad() }

// MiniMode returns the active Minibuffer mode.
func (f *Frame) MiniMode() minibuffer.Mode { return f.miniState.Mode() }

// SyncSavedRemote persists a newly added remote gRPC target to UserPrefs.
func (f *Frame) SyncSavedRemote(r appconfig.SavedRemote) {
	f.mu.Lock()
	f.Prefs.UpsertRemote(r)
	prefs := f.Prefs
	f.mu.Unlock()
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
}

func (f *Frame) syncFromGRPCWorkbench() (schema.UIWorkbenchSnapshot, bool) {
	if f.Adaptive == nil || f.Adaptive.Desktop == nil || f.Adaptive.Desktop.Workbench == nil {
		return schema.UIWorkbenchSnapshot{}, false
	}
	snap, ok := f.Adaptive.Desktop.Workbench.Snapshot()
	if !ok || snap == nil || snap.Generation == f.syncedWBGen.Load() {
		return schema.UIWorkbenchSnapshot{}, false
	}
	val := *snap
	if f.wbSyncLane != nil {
		f.wbSyncLane.Submit(val)
	}
	return val, true
}
