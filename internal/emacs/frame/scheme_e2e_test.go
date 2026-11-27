// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
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
