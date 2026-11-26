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
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modeline"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/emacs/window"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

// Config configures Marlin UI's Doom Emacs Frame.
type Config struct {
	Adaptive        *mygoshell.AdaptiveShell
	Link            *hostlink.Link
	AppConfig       appconfig.Config
	Prefs           appconfig.UserPrefs
	FeatureReg      *features.Registry
	FeatureLedger   *features.GovernanceLedger
	FeatureHost     *features.Host
	InitialThemeID  string
	OnPrefsChanged  func(appconfig.UserPrefs)
	OnApplyFeatures func(spec string)
	Invalidate      func()
}

// Frame owns the top-level Emacs Frame state and GPU ui.View entry point.
type Frame struct {
	mu sync.Mutex

	Adaptive      *mygoshell.AdaptiveShell
	Link          *hostlink.Link
	FeatureReg    *features.Registry
	FeatureLedger *features.GovernanceLedger
	FeatureHost   *features.Host
	AppCfg        appconfig.Config
	Prefs         appconfig.UserPrefs

	Theme        theme.Theme
	themeUserSet bool
	syncedWBGen  uint64
	modelLabel   string
	sessionLabel string
	schemeExts   []schema.UIExtensionRegion
	schemeCmds   []schema.UICommandSpec

	activeBuffer    buffer.ID
	activeFeatureID string
	activeExtID     string
	openTabs        []buffer.ID
	tableCursor     int
	messagesLog     []string

	meowState *meow.State
	miniState *minibuffer.State
	customize *buffer.CustomizeState

	onPrefsChanged  func(appconfig.UserPrefs)
	onApplyFeatures func(spec string)
	invalidate      func()
}

// New initializes Marlin UI's Doom Emacs Frame.
func New(cfg Config) *Frame {
	th := theme.ByID(cfg.InitialThemeID)
	inv := cfg.Invalidate
	if inv == nil {
		inv = func() {}
	}
	f := &Frame{
		Adaptive:      cfg.Adaptive,
		Link:          cfg.Link,
		FeatureReg:    cfg.FeatureReg,
		FeatureLedger: cfg.FeatureLedger,
		FeatureHost:   cfg.FeatureHost,
		AppCfg:        cfg.AppConfig,
		Prefs:         cfg.Prefs,
		Theme:         th,
		activeBuffer:  buffer.Conversation,
		openTabs:      []buffer.ID{buffer.Conversation, buffer.Sessions, buffer.Targets},
		meowState:     meow.New(),
		miniState:     minibuffer.New(),
		customize:     buffer.NewCustomizeState(cfg.AppConfig, cfg.Prefs),
		messagesLog: []string{
			fmt.Sprintf("[%s] marlin-ui started (theme=%s, session=%s)", time.Now().Format("15:04:05"), th.Name, cfg.AppConfig.SessionID),
		},
		onPrefsChanged:  cfg.OnPrefsChanged,
		onApplyFeatures: cfg.OnApplyFeatures,
		invalidate:      inv,
	}
	if cfg.Link != nil {
		f.logMessage(fmt.Sprintf("hostlink mode=%s endpoint=%s", cfg.Link.Mode, cfg.Link.Endpoint))
	}
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
	f.mu.Unlock()
	f.invalidate()
}

// SelectBuffer focuses an Emacs buffer (adding it to sort-tab if not open)
// and closes any open Minibuffer completion list.
func (f *Frame) SelectBuffer(id buffer.ID) {
	f.miniState.Close()
	f.meowState.ClearKeypad()
	f.mu.Lock()
	f.activeBuffer = id
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.tableCursor = 0
	found := false
	for _, t := range f.openTabs {
		if t == id {
			found = true
			break
		}
	}
	if !found {
		f.openTabs = append(f.openTabs, id)
	}
	f.mu.Unlock()
	f.invalidate()
}

// SetTheme switches Marlin UI's active Doom Emacs theme profile and persists it.
func (f *Frame) SetTheme(id string) {
	th := theme.ByID(id)
	f.mu.Lock()
	f.Theme = th
	f.themeUserSet = true
	f.Prefs.ThemeID = th.ID
	prefs := f.Prefs
	f.mu.Unlock()
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
	f.Echo("Theme: " + th.Name)
}

// CycleTheme advances deterministically to the next Doom Emacs theme profile.
func (f *Frame) CycleTheme() {
	f.mu.Lock()
	cur := f.Theme.ID
	f.mu.Unlock()
	f.SetTheme(theme.Next(cur).ID)
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

func (f *Frame) syncFromGRPCWorkbench() {
	if f.Adaptive == nil || f.Adaptive.Desktop == nil || f.Adaptive.Desktop.Workbench == nil {
		return
	}
	snap, ok := f.Adaptive.Desktop.Workbench.Snapshot()
	if !ok {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if snap.Generation == f.syncedWBGen {
		return
	}
	f.syncedWBGen = snap.Generation
	if strings.TrimSpace(snap.ModelLabel) != "" {
		f.modelLabel = strings.TrimSpace(snap.ModelLabel)
	}
	if strings.TrimSpace(snap.SessionLabel) != "" {
		f.sessionLabel = strings.TrimSpace(snap.SessionLabel)
	}
	if len(snap.Extensions) > 0 {
		f.schemeExts = append([]schema.UIExtensionRegion(nil), snap.Extensions...)
	}
	if len(snap.Commands) > 0 {
		f.schemeCmds = append([]schema.UICommandSpec(nil), snap.Commands...)
	}
	if !f.themeUserSet && snap.Theme.Canvas != snap.Theme.Ink && snap.ProfileName != "" && !strings.HasPrefix(snap.ProfileName, "adaptive-") {
		f.Theme = theme.FromUITheme(snap.ProfileName, snap.ProfileName, snap.Theme)
		f.logMessage("synced marlin.ui.v1 workbench profile=" + snap.ProfileName)
	}
}

// View is the single ui.View callback passed to mygo.NewWindow for Desktop.
func (f *Frame) View(c *ui.Context) {
	f.syncFromGRPCWorkbench()
	f.dispatchKeys(c)

	f.mu.Lock()
	th := f.Theme
	activeBuf := f.activeBuffer
	activeFeat := f.activeFeatureID
	activeExtID := f.activeExtID
	openTabs := append([]buffer.ID(nil), f.openTabs...)
	schemeExts := append([]schema.UIExtensionRegion(nil), f.schemeExts...)
	cursor := f.tableCursor
	modelLbl := f.modelLabel
	sessLbl := f.sessionLabel
	msgs := append([]string(nil), f.messagesLog...)
	f.mu.Unlock()

	theme.ApplyToMyGo(c, th)

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
	if f.miniState.Mode() == minibuffer.ModeGateReview && gateID != "" {
		if c.Shortcut(0, ui.KeyY) {
			f.resolveGate(pg, gateID, true)
			f.miniState.Close()
		} else if c.Shortcut(0, ui.KeyN) {
			f.resolveGate(pg, gateID, false)
			f.miniState.Close()
		}
	}

	bc := &buffer.Context{
		Adaptive:           f.Adaptive,
		Link:               f.Link,
		FeatureReg:         f.FeatureReg,
		FeatureLedger:      f.FeatureLedger,
		FeatureHost:        f.FeatureHost,
		ActiveModel:        modelLbl,
		TableCursor:        cursor,
		MessagesLog:        msgs,
		Customize:          f.customize,
		OnSelectBuffer:     f.SelectBuffer,
		OnSetTheme:         f.SetTheme,
		OnOpenCommands:     f.OpenCommands,
		OnOpenHelm:         f.OpenHelm,
		OnOpenBufferSwitch: f.OpenBufferSwitch,
		OnOpenThemeSwitch:  f.OpenThemeSwitch,
		OnOpenSettings:     f.OpenSettings,
		OnSavePrefs:        f.saveCustomizePrefs,
		OnEcho:             f.Echo,
	}

	mapCtx := mapper.Context{
		WidthCols: 120,
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
		b := f.hydraBodyFor(kp)
		activeHydra = &b
	}

	activeTabKey := activeFeat
	if activeExtID != "" {
		activeTabKey = "scheme-ext:" + activeExtID
	}

	// Pure 4-region Doom Emacs frame (zero extraneous bottom strips):
	// 1. sort-tab  2. Full-bleed Window  3. Doom Modeline  4. Hydra/Vertico/Helm/y-or-n-p Minibuffer & Echo
	ui.Column(c.Key("doom-frame")).Fill().Background(th.Canvas).AlignItems(ui.Stretch).Children(func() {
		tabs.Render(c, tabs.Props{
			OpenTabs:        openTabs,
			ActiveBuffer:    activeBuf,
			ActiveFeatureID: activeTabKey,
			FeatureBuffers:  featBufs,
			Theme:           th,
			OnSelectBuffer:  f.SelectBuffer,
			OnSelectFeature: func(fid string) {
				f.miniState.Close()
				f.meowState.ClearKeypad()
				f.mu.Lock()
				if strings.HasPrefix(fid, "scheme-ext:") {
					f.activeExtID = strings.TrimPrefix(fid, "scheme-ext:")
					f.activeFeatureID = ""
				} else {
					f.activeFeatureID = fid
					f.activeExtID = ""
				}
				f.mu.Unlock()
			},
			OnNewSession:   f.newSession,
			OnOpenHelm:     f.OpenHelm,
			OnOpenThemes:   f.OpenThemeSwitch,
			OnOpenSettings: f.OpenSettings,
		})

		window.RenderBody(c, th, activeBuf, activeFeat, activeExt, f.FeatureHost, bc, mapCtx)
		st := f.buildModelineStatus(th, activeBuf, modelLbl, sessLbl)
		st.PendingGates = pendingGateCount
		st.OnReviewGate = f.OpenGateReview
		modeline.Render(c, st)
		minibuffer.Render(c, minibuffer.RenderProps{
			State:             f.miniState,
			Theme:             th,
			KeyMode:           f.meowState.Mode(),
			Keypad:            kp,
			ActiveHydra:       activeHydra,
			Helm:              helm,
			GateTitle:         gateTitle,
			GatePrompt:        gatePrompt,
			CommandCandidates: f.commandCandidates,
			BufferCandidates:  f.bufferCandidates,
			ThemeCandidates:   f.themeCandidates,
			OnSubmitPrompt:    f.submitPrompt,
			OnEnterInsert:     f.EnterInsert,
			OnExitHydra:       func() { f.meowState.ClearKeypad(); f.invalidate() },
			OnResolveGate: func(approve bool) {
				if gateID != "" {
					f.resolveGate(pg, gateID, approve)
				}
			},
			OnEcho:       f.Echo,
			OnInvalidate: f.invalidate,
		})
	})
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

func (f *Frame) dispatchKeys(c *ui.Context) {
	meow.Dispatch(c, f.meowState, meow.Handlers{
		IsMinibufferActive: f.miniState.IsCompletionActive,
		OnCancel: func(notice string) {
			f.miniState.Close()
			f.miniState.Echo(notice, 2*time.Second)
		},
		OnCloseMinibuffer:  f.miniState.Close,
		OnOpenCommands:     f.OpenCommands,
		OnOpenHelm:         f.OpenHelm,
		OnOpenBufferSwitch: f.OpenBufferSwitch,
		OnOpenThemeSwitch:  f.OpenThemeSwitch,
		OnOpenHelp:         func() { f.SelectBuffer(buffer.Help) },
		OnOpenSettings:     f.OpenSettings,
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
			f.mu.Unlock()
		},
		OnFocusMinibuffer: f.miniState.RequestFocus,
		OnHydraKey: func(ctx *ui.Context, prefix string) bool {
			body := f.hydraBodyFor(prefix)
			head, ok := body.MatchKey(ctx)
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

func (f *Frame) hydraBodyFor(prefix string) hydra.Body {
	switch prefix {
	case "SPC b":
		return hydra.Body{
			ID:     "buffers",
			Prefix: "SPC b",
			Title:  "+buffers",
			Heads: []hydra.Head{
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch buffer (Vertico)", Column: "Buffers", Exit: true, Run: f.OpenBufferSwitch},
				{Key: "c", KeyCode: ui.KeyC, Label: "*conversation*", Column: "Buffers", Exit: true, Run: func() { f.SelectBuffer(buffer.Conversation) }},
				{Key: "s", KeyCode: ui.KeyS, Label: "*Sessions*", Column: "Buffers", Exit: true, Run: func() { f.SelectBuffer(buffer.Sessions) }},
				{Key: "t", KeyCode: ui.KeyT, Label: "*Targets*", Column: "Runtime", Exit: true, Run: func() { f.SelectBuffer(buffer.Targets) }},
				{Key: "p", KeyCode: ui.KeyP, Label: "*Processes*", Column: "Runtime", Exit: true, Run: func() { f.SelectBuffer(buffer.Processes) }},
				{Key: "m", KeyCode: ui.KeyM, Label: "*Models*", Column: "Runtime", Exit: true, Run: func() { f.SelectBuffer(buffer.Models) }},
				{Key: "l", KeyCode: ui.KeyL, Label: "*Messages*", Column: "Diagnostics", Exit: true, Run: func() { f.SelectBuffer(buffer.Messages) }},
				{Key: "h", KeyCode: ui.KeyH, Label: "*Help*", Column: "Diagnostics", Exit: true, Run: func() { f.SelectBuffer(buffer.Help) }},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Quit hydra", Column: "Diagnostics", Exit: true},
			},
		}
	case "SPC t":
		return hydra.Body{
			ID:     "themes",
			Prefix: "SPC t",
			Title:  "+themes & toggles",
			Heads: []hydra.Head{
				{Key: "t", KeyCode: ui.KeyT, Label: "Switch theme (Vertico)", Column: "Themes", Exit: true, Run: f.OpenThemeSwitch},
				{Key: "n", KeyCode: ui.KeyN, Label: "Cycle next theme (repeat)", Column: "Themes", Exit: false, Run: f.CycleTheme},
				{Key: "1", KeyCode: ui.Key1, Label: "doom-one", Column: "Presets", Exit: false, Run: func() { f.SetTheme("doom-one") }},
				{Key: "2", KeyCode: ui.Key2, Label: "pi", Column: "Presets", Exit: false, Run: func() { f.SetTheme("pi") }},
				{Key: "3", KeyCode: ui.Key3, Label: "doom-paper", Column: "Presets", Exit: false, Run: func() { f.SetTheme("doom-paper") }},
				{Key: "g", KeyCode: ui.KeyG, Label: "Open *Targets*", Column: "Workspace", Exit: true, Run: func() { f.SelectBuffer(buffer.Targets) }},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Done", Column: "Workspace", Exit: true},
			},
		}
	case "SPC m":
		return hydra.Body{
			ID:     "session",
			Prefix: "SPC m",
			Title:  "+marlin session",
			Heads: []hydra.Head{
				{Key: "n", KeyCode: ui.KeyN, Label: "New session", Column: "Session", Exit: true, Run: f.newSession},
				{Key: "i", KeyCode: ui.KeyI, Label: "Enter INSERT prompt", Column: "Session", Exit: true, Run: f.EnterInsert},
				{Key: "p", KeyCode: ui.KeyP, Label: "Probe federation targets", Column: "Federation", Exit: true, Run: f.probeTargets},
				{Key: ",", KeyCode: ui.KeyComma, Label: "Open *Settings*", Column: "Federation", Exit: true, Run: f.OpenSettings},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Quit hydra", Column: "Federation", Exit: true},
			},
		}
	default:
		return hydra.Body{
			ID:     "leader",
			Prefix: "SPC",
			Title:  "Doom Leader / Hydra",
			Heads: []hydra.Head{
				{Key: "SPC", KeyCode: ui.KeySpace, Label: "Execute command (M-x)", Column: "Commands", Exit: true, Run: f.OpenCommands},
				{Key: "x", KeyCode: ui.KeyX, Label: "M-x command palette", Column: "Commands", Exit: true, Run: f.OpenCommands},
				{Key: "s", KeyCode: ui.KeyS, Label: "Helm live search", Column: "Commands", Exit: true, Run: f.OpenHelm},
				{Key: "b", KeyCode: ui.KeyB, Label: "+buffers", Column: "Navigation", IsPrefix: true, Run: func() { f.meowState.SetKeypad("SPC b") }},
				{Key: "t", KeyCode: ui.KeyT, Label: "+themes & toggles", Column: "Navigation", IsPrefix: true, Run: func() { f.meowState.SetKeypad("SPC t") }},
				{Key: "m", KeyCode: ui.KeyM, Label: "+marlin session", Column: "Navigation", IsPrefix: true, Run: func() { f.meowState.SetKeypad("SPC m") }},
				{Key: "i", KeyCode: ui.KeyI, Label: "Compose message (Insert)", Column: "System", Exit: true, Run: f.EnterInsert},
				{Key: "h", KeyCode: ui.KeyH, Label: "Documentation (*Help*)", Column: "System", Exit: true, Run: func() { f.SelectBuffer(buffer.Help) }},
				{Key: ",", KeyCode: ui.KeyComma, Label: "Private config (*Settings*)", Column: "System", Exit: true, Run: f.OpenSettings},
			},
		}
	}
}

func (f *Frame) buildModelineStatus(th theme.Theme, activeBuf buffer.ID, modelLbl, sessLbl string) modeline.Status {
	actLabel := "Ready"
	actCol := th.Success
	remoteHost := ""
	if f.Link != nil {
		switch f.Link.Mode {
		case hostlink.ModeSidecar:
			actLabel = "Online"
			actCol = th.Success
		case hostlink.ModeRemoteGRPC:
			actLabel = "Remote"
			actCol = th.Info
			remoteHost = f.Link.Endpoint
		default:
			actLabel = "Local"
			actCol = th.Warning
		}
	}
	sessID := f.AppCfg.SessionID
	if sessLbl != "" {
		sessID = sessLbl
	}
	turns := 0
	helmTotal := 0
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		if f.Adaptive.Desktop.ActiveSessionID != "" {
			sessID = f.Adaptive.Desktop.ActiveSessionID
		}
		if f.Adaptive.Desktop.Transcript != nil {
			turns = f.Adaptive.Desktop.Transcript.Len()
			if f.Adaptive.Desktop.Transcript.InFlight() {
				actLabel = "Working"
				actCol = th.Info
			}
		}
		if f.Adaptive.Desktop.HelmSearch != nil && f.Adaptive.Desktop.HelmSearch.Engine != nil {
			helmTotal, _, _, _ = f.Adaptive.Desktop.HelmSearch.Engine.Stats()
		}
	}

	preset := modeline.PresetMain
	if f.miniState.IsCompletionActive() {
		preset = modeline.PresetHelm
	} else if activeBuf == buffer.Conversation && turns == 0 {
		preset = modeline.PresetDashboard
	}

	ver := "MARLIN " + appconfig.Version
	return modeline.Status{
		Preset:        preset,
		KeyMode:       f.meowState.Mode(),
		MiniMode:      f.miniState.Mode(),
		Keypad:        f.meowState.Keypad(),
		BufferName:    string(activeBuf),
		MajorMode:     majorModeName(activeBuf),
		WorkspacePath: "~/marlin/" + sessID + "/",
		RemoteHost:    remoteHost,
		ActivityLabel: actLabel,
		ActivityColor: actCol,
		ModelLabel:    modelLbl,
		SessionID:     sessID,
		RowCount:      turns,
		HelmTotal:     helmTotal,
		VersionLabel:  ver,
		Theme:         th,
	}
}

func majorModeName(id buffer.ID) string {
	switch id {
	case buffer.Sessions, buffer.Targets, buffer.Processes, buffer.Models:
		return "Tabulated-List"
	case buffer.Help:
		return "Help"
	case buffer.Messages:
		return "Messages"
	case buffer.Settings:
		return "Customize"
	default:
		return "Marlin-Chat"
	}
}

func (f *Frame) newSession() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnNewSession != nil {
		f.Adaptive.Desktop.OnNewSession()
		f.SelectBuffer(buffer.Conversation)
		f.Echo("Session: " + f.Adaptive.Desktop.ActiveSessionID)
	}
}

func (f *Frame) probeTargets() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.FederationUI != nil && f.Adaptive.Desktop.FederationUI.OnProbeAll != nil {
		f.Adaptive.Desktop.FederationUI.OnProbeAll()
		f.Echo("Probing federation targets...")
	}
}

func (f *Frame) submitPrompt(prompt string) {
	f.SelectBuffer(buffer.Conversation)
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSendPrompt != nil {
		f.Adaptive.Desktop.OnSendPrompt(prompt)
	}
}

func (f *Frame) saveCustomizePrefs(sessionID, remoteHost string, realSession bool) {
	f.mu.Lock()
	if strings.TrimSpace(sessionID) != "" {
		f.Prefs.LastSessionID = strings.TrimSpace(sessionID)
	}
	f.Prefs.DefaultRemoteHost = strings.TrimSpace(remoteHost)
	rs := realSession
	f.Prefs.RealSession = &rs
	prefs := f.Prefs
	f.mu.Unlock()
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSelectSession != nil && prefs.LastSessionID != "" {
		f.Adaptive.Desktop.OnSelectSession(prefs.LastSessionID)
	}
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
}

func (f *Frame) commandCandidates(query string) []minibuffer.Candidate {
	all := []minibuffer.Candidate{
		{ID: "helm", Name: "helm", Desc: "Open interactive 120Hz Helm search (SPC s)", Run: f.OpenHelm},
		{ID: "switch-buffer", Name: "switch-buffer", Desc: "Switch active Emacs buffer (SPC b b)", Run: f.OpenBufferSwitch},
		{ID: "switch-themes", Name: "switch-themes", Desc: "Switch Doom Emacs theme profile (SPC t t)", Run: f.OpenThemeSwitch},
		{ID: "sessions", Name: "sessions", Desc: "Open *Sessions* recorded conversations", Run: func() { f.SelectBuffer(buffer.Sessions) }},
		{ID: "targets", Name: "targets", Desc: "Open *Targets* federation gRPC hosts", Run: func() { f.SelectBuffer(buffer.Targets) }},
		{ID: "models", Name: "models", Desc: "Open *Models* configured model routes", Run: func() { f.SelectBuffer(buffer.Models) }},
		{ID: "list-processes", Name: "list-processes", Desc: "Open *Processes* managed runtime processes", Run: func() { f.SelectBuffer(buffer.Processes) }},
		{ID: "messages", Name: "messages", Desc: "Open *Messages* UI diagnostics log", Run: func() { f.SelectBuffer(buffer.Messages) }},
		{ID: "help", Name: "help", Desc: "Open *Help* Meow & Doom keybindings", Run: func() { f.SelectBuffer(buffer.Help) }},
		{ID: "settings", Name: "settings", Desc: "Open *Settings* customize buffer", Run: f.OpenSettings},
		{ID: "new-session", Name: "new-session", Desc: "Create and switch to a fresh session", Run: f.newSession},
		{ID: "probe-targets", Name: "probe-targets", Desc: "Probe all registered Rust federation targets", Run: f.probeTargets},
		{ID: "review-gate", Name: "review-gate", Desc: "Review pending approval gate in Minibuffer (y or n)", Run: f.OpenGateReview},
	}
	seen := make(map[string]struct{}, len(all))
	for _, it := range all {
		seen[it.Name] = struct{}{}
	}
	f.mu.Lock()
	schemeCmds := append([]schema.UICommandSpec(nil), f.schemeCmds...)
	f.mu.Unlock()
	for _, sc := range schemeCmds {
		name := strings.TrimSpace(sc.Name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		desc := sc.Description
		if desc == "" {
			desc = "Scheme marlin.ui.v1 command"
		}
		all = append(all, minibuffer.Candidate{
			ID:   "scheme-" + name,
			Name: name,
			Desc: desc,
			Run: func() {
				if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnCommandInvoke != nil {
					f.Adaptive.Desktop.OnCommandInvoke(name)
				}
				f.Echo("Executed M-x " + name)
			},
		})
	}
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		for _, cmd := range f.Adaptive.Desktop.Commands {
			name := strings.TrimSpace(cmd.Name)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			desc := cmd.Description
			if desc == "" {
				desc = "marlin.ui.v1 command"
			}
			all = append(all, minibuffer.Candidate{
				ID:   "grpc-" + name,
				Name: name,
				Desc: desc,
				Run: func() {
					if f.Adaptive.Desktop.OnCommandInvoke != nil {
						f.Adaptive.Desktop.OnCommandInvoke(name)
					}
					f.Echo("Executed M-x " + name)
				},
			})
		}
	}
	return minibuffer.FilterCandidates(all, query)
}

func (f *Frame) bufferCandidates(query string) []minibuffer.Candidate {
	cat := buffer.Catalog()
	all := make([]minibuffer.Candidate, 0, len(cat))
	for _, b := range cat {
		bid := b.ID
		all = append(all, minibuffer.Candidate{
			ID:   string(bid),
			Name: string(bid),
			Desc: b.Description,
			Run:  func() { f.SelectBuffer(bid) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

func (f *Frame) themeCandidates(query string) []minibuffer.Candidate {
	cat := theme.Catalog()
	all := make([]minibuffer.Candidate, 0, len(cat))
	for _, tp := range cat {
		p := tp
		desc := "Dark Doom Emacs profile"
		if p.IsLight {
			desc = "Light Doom Emacs profile"
		}
		all = append(all, minibuffer.Candidate{
			ID:   p.ID,
			Name: p.Name,
			Desc: desc,
			Run:  func() { f.SetTheme(p.ID) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}
