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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modeline"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/emacs/window"
	"github.com/tao3k/marlin-ui/internal/emacs/workspace"
	"github.com/tao3k/marlin-ui/internal/hostlink"
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
	RoomStats           func() (done, total, onlineAgents, totalAgents int)
	RoomTasks           func() []widgets.TaskItem
	OnAddTask           func(title string) error
	OnCycleTask         func(id string) error
	OnPrefsChanged      func(appconfig.UserPrefs)
	OnApplyFeatures     func(spec string)
	Invalidate          func()
}

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
	dashboardNode   *widgets.Node
	customSidebar   *widgets.SidebarSpec
	tomlCommands    []declarativeCommand
	tomlHydras      map[string]declarativeHydraDef
	uiModules       widgets.Modules
	zenMode         bool
	railPinned      bool
	dashPromptDraft string
	dashTaskDraft   string
	localTasks      []widgets.TaskItem
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

	meowState      *meow.State
	miniState      *minibuffer.State
	customize      *buffer.CustomizeState
	wsMgr          *workspace.Manager
	thinkingMu     sync.RWMutex
	thinkingOpened map[string]bool
	activeArtifact *buffer.ArtifactSelection

	// Deterministic RCU render snapshot + coalesced off-lock reload lane + 120Hz frame pacer.
	snapStore      *async.SnapshotStore[frameSnapshot]
	reloadLane     *async.FencedLane[struct{}, appconfig.UIThemeBundle]
	wbSyncLane     *async.FencedLane[schema.UIWorkbenchSnapshot, schema.UIWorkbenchSnapshot]
	pacer          *async.FramePacer
	pkgStore       *marketplace.PackageStore
	pkgRefreshLane *async.FencedLane[struct{}, marketplace.PackageStoreSnapshot]
	packageFilter  string
	mountedPlugins []marketplace.ManagedPackage
	activePluginID string

	onReloadThemeBundle func() appconfig.UIThemeBundle
	onGlassChanged      func(enabled bool)
	onThemeChanged      func(isLight bool)
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
		Theme:               theme.NordBento(),
		themeCatalog:        theme.Catalog(),
		dashboardNode:       widgets.DefaultDashboard(nil),
		uiModules:           widgets.DefaultModules(),
		startedAt:           time.Now(),
		activeBuffer:        buffer.Dashboard,
		openTabs:            []buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets},
		meowState:           meow.New(),
		miniState:           minibuffer.New(),
		customize:           buffer.NewCustomizeState(cfg.AppConfig, cfg.Prefs),
		wsMgr:               workspace.NewManager([]buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets}),
		thinkingOpened:      make(map[string]bool),
		onReloadThemeBundle: cfg.OnReloadThemeBundle,
		onGlassChanged:      cfg.OnGlassChanged,
		onThemeChanged:      cfg.OnThemeChanged,
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
	f.initPackageStore(cfg.PackageStore)
	f.initAsyncRuntime()
	return f
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
func (f *Frame) SelectFeature(fid string) {
	if f.selectMountedPlugin(fid) {
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

// ToggleZenMode switches immersive Zen Mode (hides sort-tab, doom-modeline, and
// resting echo bar so the Bento widget desktop fills the entire window).
func (f *Frame) ToggleZenMode() {
	f.mu.Lock()
	f.zenMode = !f.zenMode
	on := f.zenMode
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if on {
		f.Echo("Zen Mode enabled (SPC t z to restore chrome)")
	} else {
		f.Echo("Zen Mode disabled")
	}
}

// ZenMode reports whether immersive Zen Mode is currently active.
func (f *Frame) ZenMode() bool {
	return f.loadSnapshot().ZenMode
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
	f.invalidate()
}

// EnterInsert transitions Meow to INSERT mode and focuses the Minibuffer prompt.
func (f *Frame) EnterInsert() {
	f.meowState.EnterInsert()
	f.miniState.RequestFocus()
	f.invalidate()
}

// OpenMinibuffer activates the bottom Emacs Minibuffer in the given mode.
func (f *Frame) OpenMinibuffer(mode minibuffer.Mode) {
	f.meowState.ClearKeypad()
	f.miniState.Open(mode)
	if mode == minibuffer.ModeHelm && f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.HelmSearch != nil {
		f.Adaptive.Desktop.HelmSearch.SetQuery("")
	}
	f.invalidate()
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

// View is the single ui.View callback passed to mygo.NewWindow for Desktop.
// Reads Frame state via wait-free atomic.Pointer RCU snapshot (zero f.mu locks!).
func (f *Frame) View(c *ui.Context) {
	f.inView.Store(true)
	defer f.inView.Store(false)

	var frameStart time.Time
	if f.pacer != nil {
		frameStart = f.pacer.MarkFrameStart()
		f.pacer.SetWindowFocused(c.WindowActive())
		defer func() { f.pacer.RecordFrame(time.Since(frameStart)) }()
	}
	wbOverlay, hasWBOverlay := f.syncFromGRPCWorkbench()

	frameEl := ui.Column(c.Key("doom-frame")).Fill().TabStop(false).FocusRing(false).
		AutoFocus(ui.FocusOptions{PreventScroll: true}).AlignItems(ui.Stretch)

	f.dispatchKeys(c, frameEl)
	if f.consumeRefocusFrame() {
		frameEl.Focus(ui.FocusOptions{PreventScroll: true})
	}

	// Single wait-free RCU snapshot load (zero mutex acquisitions, zero slice copies!).
	snapVer := f.snapVersion()
	snap := f.loadSnapshot()
	if hasWBOverlay {
		if strings.TrimSpace(wbOverlay.ModelLabel) != "" {
			snap.ModelLabel = strings.TrimSpace(wbOverlay.ModelLabel)
		}
		if strings.TrimSpace(wbOverlay.SessionLabel) != "" {
			snap.SessionLabel = strings.TrimSpace(wbOverlay.SessionLabel)
		}
		if len(wbOverlay.StatusItems) > 0 {
			snap.StatusItems = wbOverlay.StatusItems
		}
		if len(wbOverlay.Extensions) > 0 {
			snap.SchemeExts = wbOverlay.Extensions
		}
	}
	th := snap.Theme
	availThemes := snap.ThemeCatalog
	dashNode := snap.DashboardNode
	zen := snap.ZenMode
	activeBuf := snap.ActiveBuffer
	activeFeat := snap.ActiveFeatureID
	activeExtID := snap.ActiveExtID
	openTabs := snap.OpenTabs
	schemeExts := snap.SchemeExts
	cursor := snap.TableCursor
	modelLbl := snap.ModelLabel
	if modelLbl == "" && f.AppCfg.ModelCfg.Loaded {
		modelLbl = f.AppCfg.ModelCfg.SummaryLabel()
	}
	sessLbl := snap.SessionLabel
	msgs := snap.MessagesLog
	mods := snap.UIModules
	mods.Zen = zen
	if f.FeatureReg != nil {
		for _, st := range f.FeatureReg.Snapshot() {
			switch st.Meta.ID {
			case features.FeatureID("ui.glass"):
				mods.Glass = st.Enabled
			case features.FeatureID("ui.motion"):
				mods.Motion = st.Enabled
			}
		}
	}

	if snap.MyGoTheme != nil {
		c.SetTheme(snap.MyGoTheme)
	} else {
		theme.ApplyToMyGo(c, th)
	}

	var featBufs []tabs.FeatureTab
	if f.FeatureHost != nil {
		for _, fb := range f.FeatureHost.Buffers() {
			featBufs = append(featBufs, tabs.FeatureTab{ID: fb.BufferID, Title: fb.Title})
		}
	}
	var activeExt *schema.UIExtensionRegion
	for i := range schemeExts {
		ext := schemeExts[i]
		lbl := strings.Trim(ext.Label, "*")
		if lbl == "" {
			lbl = ext.ID
		}
		featBufs = append(featBufs, tabs.FeatureTab{ID: "scheme-ext:" + ext.ID, Title: lbl})
		if activeExtID == ext.ID {
			cp := ext
			activeExt = &cp
		}
	}
	var activePlugin *marketplace.ManagedPackage
	featBufs, activePlugin = f.appendMountedPluginTabs(snap, featBufs)

	var pg *mygoshell.PromptGateController
	var helm *mygoshell.HelmMinibufferController
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		pg = f.Adaptive.Desktop.PromptGate
		helm = f.Adaptive.Desktop.HelmSearch
	}

	pendingGateCount := 0
	gateTitle, gatePrompt, gateID := "", "", ""
	if pg != nil && pg.Queue != nil {
		if req, _, total, ok := pg.Queue.Active(); ok {
			pendingGateCount = total
			gateTitle = req.Title
			gatePrompt = req.Prompt
			gateID = req.ID
		}
	}
	if f.miniState.Mode() == minibuffer.ModeHelm && helm != nil && helm.Engine != nil {
		if c.Shortcut(ui.Ctrl, ui.KeyO) {
			helm.Engine.JumpSource(1)
		} else if c.Shortcut(0, ui.KeyTab) {
			helm.Engine.ToggleMarkCurrent()
			helm.Engine.MoveSelection(1)
		} else if c.Shortcut(ui.Cmd, ui.KeyEnter) {
			if targets := helm.Engine.ResolveActionTargets(nil); len(targets) > 0 && helm.OnExecuteAction != nil {
				helm.OnExecuteAction(search.ActionMentionInComposer, targets)
				f.miniState.Close()
				f.Echo(fmt.Sprintf("Attached %d item(s)", len(targets)))
			}
		}
	}
	if f.miniState.Mode() == minibuffer.ModeGateReview && gateID != "" {
		if c.Shortcut(0, ui.KeyY) {
			f.resolveGate(pg, gateID, true)
			f.miniState.Close()
		} else if c.Shortcut(0, ui.KeyN) {
			f.resolveGate(pg, gateID, false)
			f.miniState.Close()
		}
	}

	isDashboard := activeBuf == buffer.Dashboard && activeFeat == "" && activeExt == nil && activePlugin == nil
	if f.pacer != nil {
		f.pacer.SetLiveClockVisible(isDashboard && mods.Motion)
	}
	glassOp, chromeOp, viewportOp, _ := effectiveSurfaceFromSnap(snap)
	var sig *widgets.Signals
	var wrt *widgets.Runtime
	if isDashboard || (snap.CustomSidebar != nil && snap.CustomSidebar.Body != nil) {
		sig = f.buildLiveSignals(th, modelLbl, sessLbl, pendingGateCount)
		wrt = f.buildWidgetRuntime(snap, mods, pendingGateCount, gateTitle, gateID, pg)
		wrt.Modules.GlassOpacity = glassOp
	}

	bc := &buffer.Context{
		Adaptive:            f.Adaptive,
		Link:                f.Link,
		FeatureReg:          f.FeatureReg,
		FeatureLedger:       f.FeatureLedger,
		FeatureHost:         f.FeatureHost,
		WebHost:             f.WebHost,
		ModelCfg:            f.AppCfg.ModelCfg,
		ActiveModel:         modelLbl,
		ActiveArtifact:      f.currentArtifact(),
		TableCursor:         cursor,
		MessagesLog:         msgs,
		AvailableThemes:     availThemes,
		DashboardNode:       dashNode,
		Signals:             sig,
		WidgetRuntime:       wrt,
		Modules:             mods,
		GlassOpacity:        glassOp,
		ZenMode:             zen,
		Customize:           f.customize,
		PackageSnap:         f.packageSnapshot(),
		PackageFilter:       snap.PackageFilter,
		OnSelectBuffer:      f.SelectBuffer,
		OnSetTheme:          f.SetTheme,
		OnReloadThemes:      f.ReloadThemesAsync,
		OnToggleGlass:       f.ToggleGlass,
		OnCycleGlassOpacity: f.CycleGlassOpacity,
		OnToggleMotion:      f.ToggleMotion,
		OnToggleZenMode:    f.ToggleZenMode,
		OnWidgetAction:     f.dispatchWidgetAction,
		OnOpenCommands:     f.OpenCommands,
		OnOpenHelm:         f.OpenHelm,
		OnOpenBufferSwitch: f.OpenBufferSwitch,
		OnOpenThemeSwitch:  f.OpenThemeSwitch,
		OnOpenSettings:     f.OpenSettings,
		OnPackageFilter:    f.SetPackageFilter,
		OnPackageRefresh:   f.RefreshPackagesAsync,
		OnPackageInstall:   f.InstallPackage,
		OnPackageUpdate:    f.UpdatePackage,
		OnPackageRemove:    f.RemovePackage,
		OnPackageApply:     f.ApplyPackage,
		OnSavePrefs:        f.saveCustomizePrefs,
		OnEcho:             f.Echo,
		IsThinkingOpen: func(key string) bool {
			f.thinkingMu.RLock()
			v := f.thinkingOpened[key]
			f.thinkingMu.RUnlock()
			return v
		},
		ToggleThinking: func(key string) {
			f.thinkingMu.Lock()
			f.thinkingOpened[key] = !f.thinkingOpened[key]
			f.thinkingMu.Unlock()
			f.invalidate()
		},
		OnOpenArtifact:  f.openArtifactStage,
		OnCloseArtifact: f.closeArtifactStage,
	}

	mapCtx := mapper.Context{
		WidthCols: 120,
		Sheet:     th.Sheet,
		Bindings: map[string]string{
			"model":   modelLbl,
			"session": sessLbl,
			"theme":   th.Name,
		},
		OnCommand: func(name string) {
			if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnCommandInvoke != nil {
				f.Adaptive.Desktop.OnCommandInvoke(name)
			}
			f.Echo("Executed " + name)
		},
	}

	kp := f.meowState.Keypad()
	var activeHydra *hydra.Body
	if kp != "" {
		b := f.hydraBodyFor(snap, kp)
		activeHydra = &b
	}

	activeTabKey := activeFeat
	if activeExtID != "" {
		activeTabKey = "scheme-ext:" + activeExtID
	} else if activePlugin != nil {
		activeTabKey = "pkg:" + activePlugin.ID
	}

	// 4-region Doom Emacs frame (or 100% borderless Zen Mode hiding tabs/modeline/resting echo).
	// When ui.glass is active, apply full-window Ghostty-style OS frosted glass
	// transparency across the ENTIRE window (titlebar, tabs, viewport, modeline, echo bar).
	showChrome := !zen
	showMini := !zen || activeHydra != nil || f.miniState.IsCompletionActive() || f.meowState.Mode() == meow.ModeInsert
	frameBg := th.Canvas
	if mods.Glass {
		kit.ApplyWindowTransparency(c, true)
		if isDashboard {
			frameBg = ui.Transparent
		} else if c.Vibrancy() {
			frameBg = kit.WithAlpha(th.Canvas, viewportOp)
		}
	}
	frameEl.Background(frameBg).Children(func() {
		if showChrome {
			popLbl := string(snap.Workspaces.Active.PopupBuffer)
			if popLbl == "" {
				popLbl = string(snap.Workspaces.Active.LastPopup)
			}
			tabs.Render(c, tabs.Props{
				Workspaces:        f.buildWorkspaceTabs(snap.Workspaces),
				OpenTabs:          openTabs,
				ActiveBuffer:      activeBuf,
				ActiveFeatureID:   activeTabKey,
				FeatureBuffers:    featBufs,
				PopupLabel:        popLbl,
				PopupVisible:      snap.Workspaces.Active.PopupVisible,
				Theme:             th,
				Glass:             mods.Glass,
				GlassOpacity:      chromeOp,
				OnSelectWorkspace: func(id string) { f.SwitchWorkspace(id) },
				OnNewWorkspace:    func() { f.CreateWorkspace("", "") },
				OnTogglePopup:     f.TogglePopup,
				OnSelectBuffer:    f.SelectBuffer,
				OnSelectFeature:   f.SelectFeature,
				OnNewSession:      f.newSession,
				OnOpenHelm:        f.OpenHelm,
				OnOpenThemes:      f.OpenThemeSwitch,
				OnOpenSettings:    f.OpenSettings,
			})
		} else if tb := c.TitleBar(); tb.Height > 0 {
			ui.Row(c.Key("zen-titlebar-drag")).Height(tb.Height).Background(ui.Transparent).DragWindow()
		}

		// Zero-stale-frame tab switch: if clicking a sort-tab updated the RCU
		// snapshot during Pass 0, reload immediately (<5ns) before painting the
		// viewport and modeline so Pass 0 paints the target tab with 0 wasted work.
		if f.snapVersion() != snapVer {
			snap = f.loadSnapshot()
			activeBuf = snap.ActiveBuffer
			activeFeat = snap.ActiveFeatureID
			activeExtID = snap.ActiveExtID
			activeExt = nil
			for i := range snap.SchemeExts {
				if snap.SchemeExts[i].ID == activeExtID {
					cp := snap.SchemeExts[i]
					activeExt = &cp
					break
				}
			}
			_, activePlugin = f.appendMountedPluginTabs(snap, nil)
			isDashboard = activeBuf == buffer.Dashboard && activeFeat == "" && activeExt == nil && activePlugin == nil
			if f.pacer != nil {
				f.pacer.SetLiveClockVisible(isDashboard && mods.Motion)
			}
			if isDashboard && sig == nil {
				sig = f.buildLiveSignals(th, modelLbl, sessLbl, pendingGateCount)
				wrt = f.buildWidgetRuntime(snap, mods, pendingGateCount, gateTitle, gateID, pg)
				wrt.Modules.GlassOpacity = glassOp
				bc.Signals = sig
				bc.WidgetRuntime = wrt
			}
		}

		ui.Column(c.Key("doom-viewport-host")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
			if !renderMountedPluginViewport(c, th, activePlugin, bc) {
				window.RenderBody(c, th, activeBuf, activeFeat, activeExt, f.FeatureHost, bc, mapCtx, f.buildPopupProps(snap.Workspaces))
			}
			f.renderEdgeSidebar(c, snap, mods, glassOp, featBufs, sig, wrt)
		})
		if showChrome {
			st := f.buildModelineStatus(snap, modelLbl, sessLbl)
			st.WorkspaceName = snap.Workspaces.Active.Name
			st.Glass = mods.Glass
			st.GlassOpacity = chromeOp
			st.PendingGates = pendingGateCount
			st.OnReviewGate = f.OpenGateReview
			modeline.Render(c, st)
		}
		if showMini {
			minibuffer.Render(c, minibuffer.RenderProps{
				State:             f.miniState,
				Theme:             th,
				Glass:             mods.Glass,
				GlassOpacity:      chromeOp,
				KeyMode:           f.meowState.Mode(),
				Keypad:            kp,
				ActiveHydra:       activeHydra,
				Helm:              helm,
				RustConnected:     f.Link != nil && f.Link.Mode != hostlink.ModeStandalone,
				GateTitle:         gateTitle,
				GatePrompt:        gatePrompt,
				CommandCandidates: f.commandCandidates,
				BufferCandidates:  f.bufferCandidates,
				ThemeCandidates:   f.themeCandidates,
				OnSubmitPrompt:    f.submitPrompt,
				OnEnterInsert:     f.EnterInsert,
				OnExitHydra:       func() { f.meowState.ClearKeypad(); f.requestRefocusFrame(); f.invalidate() },
				OnResolveGate: func(approve bool) {
					if gateID != "" {
						f.resolveGate(pg, gateID, approve)
					}
				},
				OnEcho:       f.Echo,
				OnInvalidate: f.invalidate,
			})
		}
	})
	if f.consumeRefocusFrame() {
		frameEl.Focus(ui.FocusOptions{PreventScroll: true})
	}
}

func (f *Frame) requestRefocusFrame() {
	f.refocusFrame.Store(true)
}

func (f *Frame) consumeRefocusFrame() bool {
	return f.refocusFrame.Swap(false)
}

func (f *Frame) resolveGate(pg *mygoshell.PromptGateController, gateID string, approve bool) {
	if pg == nil || pg.Queue == nil {
		return
	}
	act := promptgate.ActionReject
	if approve {
		act = promptgate.ActionApproveOnce
	}
	if req, resp, err := pg.Queue.Resolve(gateID, promptgate.PromptResponse{
		PromptID:       gateID,
		Action:         act,
		IdempotencyKey: fmt.Sprintf("doom-gate-%s-%v", gateID, approve),
	}); err == nil && pg.OnResolve != nil {
		pg.OnResolve(req, resp)
	}
	f.Echo(fmt.Sprintf("Gate %s -> %s", gateID, act))
}

func (f *Frame) dispatchKeys(c *ui.Context, scope ui.Element) {
	meow.Dispatch(c, f.meowState, meow.Handlers{
		Scope:              scope,
		IsMinibufferActive: f.miniState.IsCompletionActive,
		OnCancel: func(notice string) {
			f.miniState.Close()
			f.miniState.Echo(notice, 2*time.Second)
		},
		OnCloseMinibuffer:    f.miniState.Close,
		OnReturnFocusToFrame: f.requestRefocusFrame,
		OnOpenCommands:       f.OpenCommands,
		OnOpenHelm:           f.OpenHelm,
		OnOpenBufferSwitch:   f.OpenBufferSwitch,
		OnOpenThemeSwitch:    f.OpenThemeSwitch,
		OnOpenHelp:           func() { f.SelectBuffer(buffer.Help) },
		OnOpenSettings:       f.OpenSettings,
		OnStepCandidate: func(delta int) {
			if f.miniState.Mode() == minibuffer.ModeHelm && f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.HelmSearch != nil {
				f.Adaptive.Desktop.HelmSearch.Engine.MoveSelection(delta)
				return
			}
			f.miniState.StepSelection(delta)
		},
		OnStepCursor: func(delta int) {
			f.mu.Lock()
			f.tableCursor += delta
			if f.tableCursor < 0 {
				f.tableCursor = 0
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
		},
		OnFocusMinibuffer: f.miniState.RequestFocus,
		OnHydraKey: func(ctx *ui.Context, keyScope ui.Element, prefix string) bool {
			body := f.hydraBodyFor(f.loadSnapshot(), prefix)
			head, ok := body.MatchKeyInScope(ctx, keyScope)
			if !ok {
				return false
			}
			if head.Run != nil {
				head.Run()
			}
			if head.Exit && !head.IsPrefix {
				f.meowState.ClearKeypad()
			}
			return true
		},
	})
}
