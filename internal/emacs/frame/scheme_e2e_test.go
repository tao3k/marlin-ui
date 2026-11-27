// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/shellmanifest"
)

func TestLiveSchemeKeymapSlotsAndUserTOMLOverlay(t *testing.T) {
	var invoked []string
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	sh.Desktop.OnCommandInvoke = func(name string) {
		invoked = append(invoked, name)
	}

	// Downstream user TOML overlay adds a single personal key (`SPC w k -> cmd:win-only`)
	// without clobbering Scheme base or live Scheme gRPC bindings.
	userOverlay := `
[[heads]]
prefix = "SPC w"
key = "k"
label = "User Only Window"
column = "User"
exit = true
action = "cmd:win-only"
`
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-scheme-live"},
		ThemeBundle: appconfig.UIThemeBundle{
			KeymapsTOML: userOverlay,
		},
	})
	defer fr.Close()

	// 1. Modular component registry assembled Desktop + Shared modules with fail-closed trust.
	surf := fr.AssembledSurface()
	if !surf.HasModule(shellmanifest.ModWindowTree) || !surf.HasModule(shellmanifest.ModDoomModeline) || !surf.HasModule(shellmanifest.ModAgentRoom) {
		t.Fatalf("Desktop assembled surface missing expected modules: %+v", surf)
	}
	if surf.AdmitSlot("approval.gate", false) || !surf.AdmitSlot("approval.gate", true) {
		t.Fatalf("Desktop surface must enforce fail-closed approval.gate trust invariant")
	}

	// 2. Push a live marlin.ui.v1 Scheme snapshot containing a brand-new Scheme keybinding
	// (`SPC w z -> custom-scheme-zoom`), a Scheme transient (`scheme-git`), and live Slots.
	fr.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation:   42,
		ModelLabel:   "deepseek/deepseek-flash",
		SessionLabel: "sess-scheme-live",
		Keymap: []schema.UIKeyBindingSpec{
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "z"}, CommandName: "custom-scheme-zoom"},
		},
		Transients: []schema.UITransient{
			{
				ID: "scheme-git",
				Heads: []schema.UITransientHead{
					{Key: "s", CommandName: "scheme-git-status", Label: "Git status", Exit: true},
				},
			},
		},
		Slots: []schema.SlotDeclaration{
			{SlotID: "approval.gate", Cardinality: "single", FallbackRenderer: "host.gate", AllowUntrustedPlugin: true},
			{SlotID: "workbench.inspector", Cardinality: "single", FallbackRenderer: "host.inspector", AllowUntrustedPlugin: true},
		},
	})

	// Re-assembled surface admits workbench.inspector while STILL rejecting untrusted approval.gate!
	surfAfter := fr.AssembledSurface()
	if surfAfter.AdmitSlot("approval.gate", false) {
		t.Fatalf("live snapshot MUST NOT weaken fail-closed approval.gate trust invariant")
	}
	if !surfAfter.AdmitSlot("workbench.inspector", false) {
		t.Fatalf("expected live workbench.inspector slot to admit untrusted plugin")
	}

	// 3. Pressing SPC -> w -> z dispatches the brand-new Scheme command with ZERO Go code!
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyW)
	tt.Frame()
	if fr.KeypadPrefix() != "SPC w" {
		t.Fatalf("expected SPC w keypad, got %q", fr.KeypadPrefix())
	}
	if !tt.HasText("custom-scheme-zoom") || !tt.HasText("User Only Window") {
		t.Fatalf("expected BOTH live Scheme head (custom-scheme-zoom) and user TOML overlay (User Only Window) in SPC w, got %q", strings.Join(tt.Texts(), " | "))
	}
	tt.Key(0, ui.KeyZ)
	tt.Frame()
	if len(invoked) != 1 || invoked[0] != "custom-scheme-zoom" {
		t.Fatalf("expected SPC w z to dispatch live Scheme command custom-scheme-zoom, got %v", invoked)
	}

	// 4. Opening a live Scheme UITransient (`scheme-git`) and pressing `s` dispatches `scheme-git-status`.
	fr.OpenHydra("scheme-git")
	tt.Frame()
	if !tt.HasText("Git status") {
		t.Fatalf("expected live Scheme transient scheme-git rendered, got %q", strings.Join(tt.Texts(), " | "))
	}
	tt.Key(0, ui.KeyS)
	tt.Frame()
	if len(invoked) != 2 || invoked[1] != "scheme-git-status" {
		t.Fatalf("expected Scheme transient head s to dispatch scheme-git-status, got %v", invoked)
	}
}

func TestSchemeUIFrameMultiWindowSyncAndInteractiveExtensionBuffer(t *testing.T) {
	var invoked []string
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	sh.Desktop.OnCommandInvoke = func(name string) {
		invoked = append(invoked, name)
	}
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-scheme-win"},
	})
	defer fr.Close()

	// Stream a live Scheme UIWorkbenchSnapshot with:
	//  - 2-window UIFrame (*conversation* | *Git Status* Scheme extension)
	//  - Reactive ElemTemplate reading {{model}} & {{workspace}}
	//  - Interactive ElemButton invoking native Doom "window-split-below" + custom "git-stage-all"
	//  - 1 invalid empty-kind extension that MUST be rejected by MAR-UI-R03 ValidateBounds
	fr.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation:   99,
		ModelLabel:   "deepseek/deepseek-flash",
		SessionLabel: "sess-scheme-win",
		Frame: schema.UIFrame{
			SelectedWindow: "win-2",
			Windows: []schema.UIWindow{
				{ID: "win-1", BufferID: "*conversation*"},
				{ID: "win-2", BufferID: "git-status"},
			},
		},
		Extensions: []schema.UIExtensionRegion{
			{
				ID:    "git-status",
				Label: "*Git Status*",
				Content: schema.UIElement{
					Kind: schema.ElemRows,
					Children: []schema.UIElement{
						{
							Kind: schema.ElemTemplate,
							Spans: []schema.UISpan{
								{Text: "Branch main · {{model}} · {{workspace}}", Tone: schema.ToneInfo},
							},
						},
						{
							Kind:   schema.ElemButton,
							ID:     "btn-stage-all",
							Title:  "Stage All Changes",
							Action: "git-stage-all",
						},
						{
							Kind:   schema.ElemButton,
							ID:     "btn-split-below",
							Title:  "Split Pane Below",
							Action: "window-split-below",
						},
					},
				},
			},
			{
				ID:      "broken-ext",
				Label:   "*Broken*",
				Content: schema.UIElement{}, // must be rejected by ValidateBounds!
			},
		},
	})

	// 1. Both panes render side-by-side with per-window Doom modeline (1 & 2),
	//    reactive {{model}} / {{workspace}} bindings, and rejected invalid extension omitted.
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected Scheme UIFrame to sync 2 windows, got %d", fr.WorkspaceSnapshot().Active.WindowCount())
	}
	texts := strings.Join(tt.Texts(), " | ")
	if !tt.HasText("Branch main · deepseek/deepseek-flash · main") || !tt.HasText("*Git Status*") || !tt.HasText("1") || !tt.HasText("2") {
		t.Fatalf("expected side-by-side Scheme extension pane with reactive bindings & per-window modelines 1/2, got %q", texts)
	}
	if tt.HasText("*Broken*") {
		t.Fatalf("invalid empty-kind Scheme extension must be rejected by ValidateBounds")
	}

	// 2. Clicking Scheme ElemButton ("Stage All Changes") dispatches custom Scheme RPC cleanly!
	if err := tt.Click("Stage All Changes"); err != nil {
		t.Fatalf("expected Scheme ElemButton 'Stage All Changes' clickable: %v", err)
	}
	tt.Frame()
	if len(invoked) != 1 || invoked[0] != "git-stage-all" {
		t.Fatalf("expected clicking Scheme button to dispatch git-stage-all, got %v", invoked)
	}

	// 3. Clicking Scheme ElemButton ("Split Pane Below") resolves native Doom action -> 3 windows!
	if err := tt.Click("Split Pane Below"); err != nil {
		t.Fatalf("expected Scheme ElemButton 'Split Pane Below' clickable: %v", err)
	}
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 3 {
		t.Fatalf("expected Scheme button action window-split-below to split native window tree to 3 panes, got %d", fr.WorkspaceSnapshot().Active.WindowCount())
	}

	// 4. SPC b b lists live Scheme extension buffer *Git Status*!
	fr.OpenBufferSwitch()
	tt.Frame()
	if !tt.HasText("[scheme-ext]") {
		t.Fatalf("expected SPC b b buffer switcher to list [scheme-ext] live Scheme extension, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 5. Winner-undo restores back through Scheme-synced layouts!
	fr.WinnerUndo()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected winner-undo to restore 2-window Scheme layout, got %d", fr.WorkspaceSnapshot().Active.WindowCount())
	}
}

func TestDoomZenModeWriteroomAndFullscreenSubModes(t *testing.T) {
	var lastWriteroom, lastFullscreen bool
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	sh.Desktop.Transcript.AppendUser("Focus check")

	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-zen-modes"},
		OnZenModeChanged: func(writeroom, fullscreen bool) {
			lastWriteroom = writeroom
			lastFullscreen = fullscreen
		},
	})
	defer fr.Close()

	fr.SelectBuffer(buffer.Conversation)
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()

	// 1. SPC t z -> Doom Zen Writeroom (centered focus WITHOUT forcing OS fullscreen).
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyT)
	tt.Frame()
	tt.Key(0, ui.KeyZ)
	tt.Frame()
	if !fr.ZenMode() || fr.ZenFullscreen() || !lastWriteroom || lastFullscreen {
		t.Fatalf("expected SPC t z -> ZenWriteroom (writeroom=true, fullscreen=false), got zen=%v fs=%v cb=(%v,%v)",
			fr.ZenMode(), fr.ZenFullscreen(), lastWriteroom, lastFullscreen)
	}
	if !tt.HasText("Writeroom · SPC t z to exit") {
		t.Fatalf("expected centered Writeroom bar in *conversation*, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 2. SPC t Z (Shift+Z) -> Doom Zen Fullscreen (writeroom + OS fullscreen).
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyT)
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyZ)
	tt.Frame()
	if !fr.ZenMode() || !fr.ZenFullscreen() || !lastWriteroom || !lastFullscreen {
		t.Fatalf("expected SPC t Z -> ZenFullscreen (writeroom=true, fullscreen=true), got zen=%v fs=%v cb=(%v,%v)",
			fr.ZenMode(), fr.ZenFullscreen(), lastWriteroom, lastFullscreen)
	}

	// 3. Toggling SPC t Z again exits cleanly to ZenOff and restores OS fullscreen=false.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyT)
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyZ)
	tt.Frame()
	if fr.ZenMode() || fr.ZenFullscreen() || lastWriteroom || lastFullscreen {
		t.Fatalf("expected second SPC t Z to restore ZenOff, got zen=%v fs=%v cb=(%v,%v)",
			fr.ZenMode(), fr.ZenFullscreen(), lastWriteroom, lastFullscreen)
	}
}

func TestNativeEmacsKeybindingsWorkInInsertAndNormalModes(t *testing.T) {
	quitCalled := false
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-emacs-native"},
		OnQuit:    func() { quitCalled = true },
	})
	defer fr.Close()

	// Switch to Insert Mode explicitly (verifying native Emacs bindings work in Insert mode!).
	fr.EnterInsert()
	if fr.KeyMode() != meow.ModeInsert {
		t.Fatalf("expected Insert mode")
	}
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()

	// 1. C-x 3 splits window right in Insert mode (2 windows, focused on window 2).
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	if fr.KeypadPrefix() != "C-x" {
		t.Fatalf("expected C-x prefix active in Insert mode, got %q", fr.KeypadPrefix())
	}
	tt.Key(0, ui.Key3)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 2 || fr.WorkspaceSnapshot().Active.FocusedWindowNumber() != 2 {
		t.Fatalf("expected C-x 3 in Insert mode to split into 2 windows focused on 2, got count=%d focus=%d",
			fr.WorkspaceSnapshot().Active.WindowCount(), fr.WorkspaceSnapshot().Active.FocusedWindowNumber())
	}

	// 2. C-x o switches to other window (window 1) in Insert mode!
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	tt.Key(0, ui.KeyO)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.FocusedWindowNumber() != 1 {
		t.Fatalf("expected C-x o in Insert mode to focus window 1, got %d", fr.WorkspaceSnapshot().Active.FocusedWindowNumber())
	}

	// 3. M-2 jumps directly to window 2 in Insert mode!
	tt.Key(ui.Alt, ui.Key2)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.FocusedWindowNumber() != 2 {
		t.Fatalf("expected M-2 in Insert mode to jump to window 2, got %d (keypad=%q mini=%v mode=%v)",
			fr.WorkspaceSnapshot().Active.FocusedWindowNumber(), fr.KeypadPrefix(), fr.MiniMode(), fr.KeyMode())
	}

	// 4. C-x z toggles Zen Writeroom in Insert mode!
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	tt.Key(0, ui.KeyZ)
	tt.Frame()
	if !fr.ZenMode() || fr.ZenFullscreen() {
		t.Fatalf("expected C-x z in Insert mode to enter Zen Writeroom, got zen=%v fs=%v", fr.ZenMode(), fr.ZenFullscreen())
	}

	// 5. C-c u triggers winner-undo in Insert mode (restoring 1 window)!
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	tt.Key(0, ui.KeyU)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 1 {
		t.Fatalf("expected C-c u in Insert mode to undo split back to 1 window, got %d", fr.WorkspaceSnapshot().Active.WindowCount())
	}

	// 6. C-x C-c triggers OnQuit in Insert mode!
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyC)
	tt.Frame()
	if !quitCalled {
		t.Fatalf("expected C-x C-c in Insert mode to invoke OnQuit")
	}
}

func TestMarginaliaAnnotationsHydraCrossHintsAndCentaurBadges(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-marginalia"},
	})
	defer fr.Close()

	// Stream a Scheme transient with an explicit Hint so Hydra renders it.
	fr.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation: 12,
		Transients: []schema.UITransient{
			{
				ID: "scheme-review",
				Hint: schema.UIElement{
					Kind:  schema.ElemText,
					Spans: []schema.UISpan{{Text: "Review pending diff hunks"}},
				},
				Heads: []schema.UITransientHead{
					{Key: "a", CommandName: "review-approve", Label: "Approve hunk", Exit: true},
				},
			},
		},
	})

	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()

	// 1. Open SPC w Hydra: verify native Emacs cross-hints (C-x 3 / C-x o) render automatically.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyW)
	tt.Frame()
	if !tt.HasText("C-x 3") || !tt.HasText("C-x o") {
		t.Fatalf("expected SPC w Hydra to display native Emacs cross-hints (C-x 3, C-x o), got %q", strings.Join(tt.Texts(), " | "))
	}

	// 2. Execute split right (`v`) -> Centaur Tabs bar now renders winum badges 1 and 2.
	tt.Key(0, ui.KeyV)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected 2 windows after SPC w v")
	}
	if !tt.HasText("1") || !tt.HasText("2") {
		t.Fatalf("expected Centaur Tabs and pane modelines to display window badges 1 & 2, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 3. Open M-x and verify Marginalia shortcut annotations appear automatically.
	fr.OpenCommands()
	tt.Frame()
	if !tt.HasText("SPC w v · C-x 3") && !tt.HasText("SPC t z · C-x z") {
		// Filter directly by native Emacs chord "C-x 3" to prove Marginalia annotation matching!
		tt.Type("C-x 3")
		tt.Frame()
		if !tt.HasText("C-x 3") {
			t.Fatalf("expected M-x Marginalia annotations to include C-x 3, got %q", strings.Join(tt.Texts(), " | "))
		}
	}

	// 4. Open Scheme transient with Hint and verify Hydra renders the Hint header.
	fr.OpenHydra("scheme-review")
	tt.Frame()
	if !tt.HasText("· Review pending diff hunks") {
		t.Fatalf("expected Hydra popup header to render Scheme UITransient.Hint, got %q", strings.Join(tt.Texts(), " | "))
	}
}

func TestMarlinModelineDockHoverRevealAndAppIcons(t *testing.T) {
	// 1. Verify macOS Dock squircle app icon & Menu Bar Marlin silhouette PNGs.
	appPNG := theme.MarlinAppIconPNG()
	img, err := png.Decode(bytes.NewReader(appPNG))
	if err != nil {
		t.Fatalf("MarlinAppIconPNG decode failed: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("expected 256x256 squircle app icon, got %v", img.Bounds())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("expected transparent HIG outer corner on squircle app icon, got alpha=%d", a)
	}
	if _, _, _, a := img.At(128, 128).RGBA(); a == 0 {
		t.Fatalf("expected opaque center emblem on squircle app icon")
	}
	trayPNG := theme.MarlinTrayIconPNG()
	trayImg, err := png.Decode(bytes.NewReader(trayPNG))
	if err != nil {
		t.Fatalf("MarlinTrayIconPNG decode failed: %v", err)
	}
	if trayImg.Bounds().Dx() != 44 || trayImg.Bounds().Dy() != 44 {
		t.Fatalf("expected 44x44 Retina Marlin tray icon, got %v", trayImg.Bounds())
	}

	// 2. On *conversation*, formal ChatBox is hidden by default into the ModeLine Marlin dock.
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	sh.Desktop.Transcript.AppendUser("Keep chat clean")

	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-marlin-dock"},
	})
	defer fr.Close()
	fr.SelectBuffer(buffer.Conversation)

	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	if tt.HasText("Deep Think") {
		t.Fatalf("expected formal ChatBox collapsed into ModeLine Marlin dock by default, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 3. Hovering over the ModeLine-top Marlin crest surfaces 85-90% of the
	//    colorful Marlin with ~1.0s breach + splash animation without popping ChatBox.
	dockRect, ok := tt.Find("Marlin ChatBox")
	if !ok {
		t.Fatalf("expected ModeLine-top Marlin ChatBox crest button to be rendered")
	}
	if dockRect.W < 180 || dockRect.H < 48 {
		t.Fatalf("expected generous Marlin ocean crest above ModeLine (not squished inside 26px bar), got %.0fx%.0f", dockRect.W, dockRect.H)
	}
	tt.Move(dockRect.X+dockRect.W/2, dockRect.Y+dockRect.H/2)
	tt.Frame()
	if !fr.ComposerDockHovered() || fr.ComposerOpen() || tt.HasText("Deep Think") {
		t.Fatalf("expected hover to animate Marlin surfacing without auto-opening ChatBox, hovered=%v open=%v",
			fr.ComposerDockHovered(), fr.ComposerOpen())
	}

	// 4. Clicking the ModeLine Marlin fish triggers dash+splash feedback and opens formal ChatBox!
	if err := tt.Click("Marlin ChatBox"); err != nil {
		t.Fatalf("expected ModeLine Marlin dock clickable: %v", err)
	}
	tt.Frame()
	if !fr.ComposerOpen() || !tt.HasText("Deep Think") || !tt.HasText("Hide") {
		t.Fatalf("expected clicking ModeLine Marlin dock to open formal ChatBox, open=%v texts=%q",
			fr.ComposerOpen(), strings.Join(tt.Texts(), " | "))
	}
	if openDockRect, ok := tt.Find("Marlin ChatBox"); ok && openDockRect.H > 50 {
		t.Fatalf("expected Marlin to dive underwater into compact ModeLine strip when ChatBox is open, got height %.0f", openDockRect.H)
	}
	if err := tt.Click("Hide"); err != nil {
		t.Fatalf("expected Hide button inside formal ChatBox clickable: %v", err)
	}
	tt.Frame()
	if fr.ComposerOpen() || tt.HasText("Deep Think") {
		t.Fatalf("expected clicking Hide to fold formal ChatBox back into ModeLine Marlin")
	}
}

func TestSingleWorkspaceStartupFiltersScratchAndSplitsExactFiftyFifty(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-wm-5050"},
		Prefs: appconfig.UserPrefs{
			ActiveWorkspaceID: "ws-4",
			Workspaces: []appconfig.SavedWorkspace{
				{ID: "main", Name: "main", ActiveBuffer: string(buffer.Dashboard), Buffers: []string{string(buffer.Dashboard), string(buffer.Conversation)}},
				{ID: "ws-2", Name: "ws-2", ActiveBuffer: string(buffer.Dashboard)},
				{ID: "ws-3", Name: "ws-3", ActiveBuffer: string(buffer.Conversation)},
				{ID: "ws-4", Name: "ws-4", ActiveBuffer: string(buffer.Conversation)},
			},
		},
	})
	defer fr.Close()

	// 1. Ephemeral scratch ws-2..ws-4 are pruned on startup -> clean single 1:main!
	wsSnap := fr.WorkspaceSnapshot()
	if len(wsSnap.Workspaces) != 1 || wsSnap.ActiveID != "main" {
		t.Fatalf("expected clean single main workspace on startup, got %+v", wsSnap.Workspaces)
	}

	// 2. Focus *conversation* and split right -> splits current *conversation* 50/50!
	fr.SelectBuffer(buffer.Conversation)
	fr.SplitVertical("")
	wsAfter := fr.WorkspaceSnapshot()
	wins := wsAfter.Active.Windows()
	if len(wins) != 2 || wins[0].Buffer != buffer.Conversation || wins[1].Buffer != buffer.Conversation {
		t.Fatalf("expected SplitVertical on *conversation* to split current window 50/50, got %+v", wins)
	}
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	if !tt.HasText("1") || !tt.HasText("2") {
		t.Fatalf("expected both split panes rendered with winum badges 1 & 2, got %q", strings.Join(tt.Texts(), " | "))
	}
}
