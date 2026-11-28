// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

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
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

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
			featBufs = append(featBufs, tabs.FeatureTab{ID: fb.BufferID, Title: fb.Title, Group: "feature"})
		}
	}
	var activeExt *schema.UIExtensionRegion
	for i := range schemeExts {
		ext := schemeExts[i]
		lbl := strings.Trim(ext.Label, "*")
		if lbl == "" {
			lbl = ext.ID
		}
		featBufs = append(featBufs, tabs.FeatureTab{ID: "scheme-ext:" + ext.ID, Title: lbl, Group: "scheme-ext"})
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

	activeMode := f.EffectiveModeStack(activeBuf)
	isDashboard := activeMode.HostMode == "marlin-dashboard-mode" && activeFeat == "" && activeExt == nil && activePlugin == nil
	overlayActive := f.miniState.IsCompletionActive() || f.meowState.Keypad() != ""
	if f.pacer != nil {
		f.pacer.SetLiveClockVisible(isDashboard && mods.Motion && !overlayActive)
	}
	glassOp, chromeOp, viewportOp, _ := effectiveSurfaceFromSnap(snap)
	var sig *widgets.Signals
	var wrt *widgets.Runtime
	if isDashboard || (snap.CustomSidebar != nil && snap.CustomSidebar.Body != nil) {
		sig = f.buildLiveSignals(th, modelLbl, sessLbl, pendingGateCount)
		wrt = f.buildWidgetRuntime(snap, mods, pendingGateCount, gateTitle, gateID, pg)
		wrt.Modules.GlassOpacity = glassOp
		wrt.Modules.Motion = mods.Motion && !overlayActive
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
		ComposerExpanded:    f.ComposerOpen(),
		ComposerPinned:      f.ComposerOpen(),
		Customize:           f.customize,
		PackageSnap:         f.packageSnapshot(),
		PackageFilter:       snap.PackageFilter,
		Magit:               snap.Magit,
		HelpIntrospection:   f.currentHelpIntrospection(activeBuf),
		IbufferRows:         f.BuildIbufferEntries(snap),
		IbufferFilter:       f.IbufferFilter(),
		IbufferMarkedCount:  f.IbufferMarkedCount(),
		ActivePolymodeSpan:  string(f.BufferModeState(activeBuf).FocusedSpan),
		OnSelectBuffer:      f.SelectBuffer,
		OnKillBuffer:        func(id buffer.ID) { f.KillBuffer(id) },
		OnToggleIbufferMark: func(id buffer.ID) { f.ToggleIbufferMark(id) },
		OnUnmarkAllIbuffer:  f.UnmarkAllIbuffer,
		OnKillMarkedIbuffer: func() { f.KillMarkedIbuffer() },
		OnFilterIbuffer:     f.SetIbufferFilter,
		OnNewChatBuffer:     func() { f.NewChatBuffer("") },
		OnFocusPolymodeSpan: f.HandlePolymodeSpanFocus,
		OnSetTheme:          f.SetTheme,
		OnReloadThemes:      f.ReloadThemesAsync,
		OnToggleGlass:       f.ToggleGlass,
		OnCycleGlassOpacity: f.CycleGlassOpacity,
		OnToggleMotion:      f.ToggleMotion,
		OnToggleZenMode:     f.ToggleZenMode,
		OnWidgetAction:      f.dispatchWidgetAction,
		OnOpenCommands:      f.OpenCommands,
		OnOpenHelm:          f.OpenHelm,
		OnOpenBufferSwitch:  f.OpenBufferSwitch,
		OnOpenThemeSwitch:   f.OpenThemeSwitch,
		OnOpenSettings:      f.OpenSettings,
		OnPackageFilter:     f.SetPackageFilter,
		OnPackageRefresh:    f.RefreshPackagesAsync,
		OnPackageInstall:    f.InstallPackage,
		OnPackageUpdate:     f.UpdatePackage,
		OnPackageRemove:     f.RemovePackage,
		OnPackageApply:      f.ApplyPackage,
		OnSavePrefs:         f.saveCustomizePrefs,
		OnEcho:              f.Echo,
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
		OnOpenArtifact:    f.openArtifactStage,
		OnCloseArtifact:   f.closeArtifactStage,
		FilePreview:       f.currentFilePreview(),
		ComposerDraft:     &f.convComposerDraft,
		OnSubmitPrompt:    f.submitPrompt,
		OnSelectModel:     f.SelectModel,
		OnToggleDeepThink:   f.ToggleDeepThink,
		OnCiteInComposer:    f.CiteInComposer,
		OnOpenFilePreview:   f.OpenFilePreview,
		OnToggleComposerPin: f.ToggleComposerOpen,
		OnMagitAction:       f.handleMagitAction,
	}

	winCount := snap.Workspaces.Active.WindowCount()
	mapCtx := mapper.Context{
		WidthCols: 120,
		Sheet:     th.Sheet,
		Bindings: map[string]string{
			"model":         modelLbl,
			"session":       sessLbl,
			"theme":         th.Name,
			"workspace":     snap.Workspaces.Active.Name,
			"buffer":        string(activeBuf),
			"window.count":  fmt.Sprintf("%d", winCount),
			"gates.pending": fmt.Sprintf("%d", pendingGateCount),
		},
		Facts: map[string]bool{
			"split.active":      winCount > 1,
			"zen.active":        zen,
			"glass.enabled":     mods.Glass,
			"gates.has_pending": pendingGateCount > 0,
		},
		OnCommand: func(name string) {
			act, _, _ := uiframework.ResolveSchemeAction(name)
			if act != "" {
				f.dispatchWidgetAction(act)
				return
			}
			if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnCommandInvoke != nil {
				f.Adaptive.Desktop.OnCommandInvoke(name)
			}
			f.Echo("Executed " + name)
		},
		OnAction: func(action, _ string) {
			act := strings.TrimSpace(action)
			if act == "" {
				return
			}
			if strings.Contains(act, ":") {
				f.dispatchWidgetAction(act)
				return
			}
			resolved, _, _ := uiframework.ResolveSchemeAction(act)
			f.dispatchWidgetAction(resolved)
		},
		OnSelectRow: func(listID, rowID string) {
			if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnCommandInvoke != nil {
				f.Adaptive.Desktop.OnCommandInvoke(listID + ":" + rowID)
			}
			f.Echo("Selected " + rowID)
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
		activeTabKey = uiframework.SchemeExtBufferPrefix + activeExtID
	} else if strings.HasPrefix(string(activeBuf), uiframework.SchemeExtBufferPrefix) {
		activeTabKey = string(activeBuf)
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
		if c.Vibrancy() {
			frameBg = kit.WithAlpha(th.Canvas, viewportOp)
		}
	}
	frameEl.Background(frameBg).Children(func() {
		if showChrome {
			popLbl := string(snap.Workspaces.Active.PopupBuffer)
			if popLbl == "" {
				popLbl = string(snap.Workspaces.Active.LastPopup)
			}
			var winBadges map[buffer.ID]int
			if leaves := snap.Workspaces.Active.Windows(); len(leaves) > 1 {
				winBadges = make(map[buffer.ID]int, len(leaves))
				for _, lf := range leaves {
					if _, exists := winBadges[lf.Buffer]; !exists {
						winBadges[lf.Buffer] = lf.Number
					}
				}
			}
			var busyBufs map[buffer.ID]bool
			if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.Transcript != nil && f.Adaptive.Desktop.Transcript.InFlight() {
				chatKey := buffer.Conversation
				if buffer.IsChatBuffer(activeBuf) {
					chatKey = activeBuf
				}
				busyBufs = map[buffer.ID]bool{chatKey: true}
			}
			var draftBufs map[buffer.ID]bool
			if strings.TrimSpace(f.convComposerDraft) != "" {
				chatKey := buffer.Conversation
				if buffer.IsChatBuffer(activeBuf) {
					chatKey = activeBuf
				}
				draftBufs = map[buffer.ID]bool{chatKey: true}
			}
			tabs.Render(c, tabs.Props{
				Workspaces:         f.buildWorkspaceTabs(snap.Workspaces),
				OpenTabs:           openTabs,
				ActiveBuffer:       activeBuf,
				ActiveFeatureID:    activeTabKey,
				FeatureBuffers:     featBufs,
				WindowBadges:       winBadges,
				BusyBuffers:        busyBufs,
				DraftBuffers:       draftBufs,
				PopupLabel:         popLbl,
				PopupVisible:       snap.Workspaces.Active.PopupVisible,
				Theme:              th,
				Glass:              mods.Glass,
				GlassOpacity:       chromeOp,
				OnSelectWorkspace:  func(id string) { f.SwitchWorkspace(id) },
				OnKillWorkspace:    func(id string) { f.KillWorkspaceByID(id) },
				OnTogglePopup:      f.TogglePopup,
				OnSelectBuffer:     f.SelectBuffer,
				OnSelectFeature:    f.SelectFeature,
				OnOpenBufferPicker: f.OpenBufferSwitch,
				OnOpenHelm:         f.OpenHelm,
				OnOpenThemes:       f.OpenThemeSwitch,
				OnOpenSettings:     f.OpenSettings,
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
			activeMode = f.EffectiveModeStack(activeBuf)
			isDashboard = activeMode.HostMode == "marlin-dashboard-mode" && activeFeat == "" && activeExt == nil && activePlugin == nil
			if f.pacer != nil {
				f.pacer.SetLiveClockVisible(isDashboard && mods.Motion && !overlayActive)
			}
			if isDashboard && sig == nil {
				sig = f.buildLiveSignals(th, modelLbl, sessLbl, pendingGateCount)
				wrt = f.buildWidgetRuntime(snap, mods, pendingGateCount, gateTitle, gateID, pg)
				wrt.Modules.GlassOpacity = glassOp
				wrt.Modules.Motion = mods.Motion && !overlayActive
				bc.Signals = sig
				bc.WidgetRuntime = wrt
			}
		}

		multiPaneActive := showChrome && activeExt == nil && activeFeat == "" && activePlugin == nil && snap.Workspaces.Active.WindowCount() > 1
		renderPaneModeline := func(ctx *ui.Context, leaf workspace.WindowLeafView) {
			st := f.buildModelineStatusForBuffer(snap, leaf.Buffer, modelLbl, sessLbl)
			st.WorkspaceName = snap.Workspaces.Active.Name
			st.Glass = mods.Glass
			st.GlassOpacity = chromeOp
			st.PendingGates = pendingGateCount
			st.OnReviewGate = f.OpenGateReview
			st.OnToggleDedicated = f.ToggleWindowDedicated
			st.WindowNumber = leaf.Number
			st.WindowCount = snap.Workspaces.Active.WindowCount()
			if st.WindowCount >= 3 {
				st.WidthCols = 48
			} else {
				st.WidthCols = 72
			}
			st.Focused = leaf.Focused
			st.Dedicated = leaf.Dedicated
			st.KeyPrefix = leaf.ID
			if !leaf.Focused {
				id := leaf.ID
				st.OnFocus = func() { f.FocusWindowID(id) }
			}
			modeline.Render(ctx, st)
		}

		ui.Column(c.Key("doom-stage-with-modeline")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("doom-viewport-host")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
				if !renderMountedPluginViewport(c, th, activePlugin, bc) {
					splitProps := f.buildSplitProps(snap.Workspaces, showChrome, schemeExts, mapCtx, f.FeatureHost, renderPaneModeline)
					window.RenderBody(c, th, activeBuf, activeFeat, activeExt, f.FeatureHost, bc, mapCtx, splitProps, f.buildPopupProps(snap.Workspaces))
				}
				f.renderEdgeSidebar(c, snap, mods, glassOp, featBufs, sig, wrt)
			})
			if showChrome && !multiPaneActive {
				st := f.buildModelineStatus(snap, modelLbl, sessLbl)
				st.WorkspaceName = snap.Workspaces.Active.Name
				st.Glass = mods.Glass
				st.GlassOpacity = chromeOp
				st.PendingGates = pendingGateCount
				st.OnReviewGate = f.OpenGateReview
				st.OnToggleDedicated = f.ToggleWindowDedicated
				st.WindowCount = 1
				st.WidthCols = 120
				st.Focused = true
				st.Dedicated = snap.Workspaces.Active.FocusedLeaf().Dedicated
				modeline.Render(c, st)
			}
		})
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
				CommandCandidates:   f.commandCandidates,
				BufferCandidates:    f.bufferCandidates,
				ThemeCandidates:     f.themeCandidates,
				DescribeFuncCands:   f.describeFunctionCandidates,
				OnSubmitPrompt:      f.submitPrompt,
				OnCreateWorkspace:   f.CreateWorkspaceFromSpec,
				OnDescribeKey:       f.DescribeKey,
				MagitPromptLabel:    f.currentMagitPromptLabel(),
				OnSubmitMagitPrompt: f.submitMagitPrompt,
				OnHelmPreview:       f.OpenFilePreview,
				OnHelmCite:        f.CiteInComposer,
				OnEnterInsert:     f.EnterInsert,
				OnExitHydra:       func() { f.meowState.ClearKeypad(); f.requestRefocusFrame(); f.invalidate() },
				OnResolveGate: func(approve bool) {
					if gateID != "" {
						f.resolveGate(pg, gateID, approve)
					}
				},
				OnEcho:       f.Echo,
				OnInvalidate: f.requestFrame,
			})
		}
		// Transparent hanging-pendant Marlin water overlay straddling the entire
		// ModeLine bar and overhanging slightly below its bottom lip (hidden
		// whenever Helm / M-x / Hydra completion drawers are active).
		if showChrome && !overlayActive && activeMode.HasSegment["composer-dock"] && activeFeat == "" && activeExt == nil && activePlugin == nil {
			bottomOffset := float32(0)
			if showMini {
				bottomOffset = theme.MinibufferHeight - theme.MarlinOceanOverhang
			}
			f.renderMarlinOceanCrestOverlay(c, th, bottomOffset)
		}
	})
	if f.consumeRefocusFrame() {
		frameEl.Focus(ui.FocusOptions{PreventScroll: true})
	}
}

func (f *Frame) renderMarlinOceanCrestOverlay(c *ui.Context, th theme.Theme, bottomOffset float32) {
	open, hoverStart, clickStart := f.composerDockAnimTimes()
	prevHovered := f.ComposerDockHovered()

	// Straddles the ModeLine's top border line in the horizontal center:
	// bottom 26px overlays the ModeLine bar for wet water stains & hanging
	// droplets, while the upper 112px extends seamlessly into open air above.
	ui.Row(c.Key("dm-marlin-crest-wrap")).
		Absolute().Bottom(bottomOffset).Left(0).Right(0).
		Height(theme.MarlinOceanDockHeight).
		Justify(ui.Center).AlignItems(ui.End).
		PassThrough().
		Children(func() {
			ui.Box(c.Key("dm-marlin-crest-stage")).
				Width(theme.MarlinOceanDockWidth).
				Height(theme.MarlinOceanDockHeight).
				PassThrough().
				Children(func() {
					// Interactive hitbox stays compact over the ModeLine water strip when
					// ChatBox is open (Marlin dived underwater -> never blocks ChatBox!).
					// When ChatBox is closed, expands upward on hover to cover the full fish.
					hitH := float32(76)
					if open {
						hitH = theme.ModelineHeight + theme.MarlinOceanOverhang + 6
					} else if prevHovered {
						hitH = theme.MarlinOceanDockHeight - 10
					}
					hitBtn := kit.ActionControl(ui.ButtonBase(c.Key("dm-marlin-dock")).
						Label("Marlin ChatBox").
						Absolute().Bottom(0).Left(48).Right(48).Height(hitH).
						Background(ui.Transparent))
					hovered := hitBtn.Hovered()

					theme.MarlinOceanDock(c, theme.MarlinOceanDockProps{
						Key:        "dm-marlin-ocean",
						Hovered:    hovered,
						Open:       open,
						HoverStart: hoverStart,
						ClickStart: clickStart,
						Theme:      th,
					})

					f.setComposerDockHovered(hovered)
					if hitBtn.Clicked() {
						f.ToggleComposerOpen()
					}
				})
		})
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
		OnDismissPopup:       f.DismissPopup,
		OnReturnFocusToFrame: f.requestRefocusFrame,
		OnOpenCommands:       f.OpenCommands,
		OnOpenHelm:           f.OpenHelm,
		OnOpenBufferSwitch:   f.OpenBufferSwitch,
		OnOpenThemeSwitch:    f.OpenThemeSwitch,
		OnOpenHelp:           func() { f.SelectBuffer(buffer.Help) },
		OnOpenSettings:       f.OpenSettings,
		OnSelectWindowNumber: f.SelectWindowNumber,
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
		OnBufferKey: f.dispatchBufferModeKey,
	})
}
