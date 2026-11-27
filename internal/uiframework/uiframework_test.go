// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package uiframework_test

import (
	"testing"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/shellmanifest"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

func TestModularComponentRegistryAssemblesAllSurfacesAndEnforcesSlotTrust(t *testing.T) {
	reg := uiframework.DefaultRegistry()

	for _, target := range []shellmanifest.TargetKind{
		shellmanifest.TargetDesktopMacOS,
		shellmanifest.TargetMobileIOS,
		shellmanifest.TargetMobileAndroid,
	} {
		surf, err := reg.AssembleSurface(target, []schema.SlotDeclaration{
			{SlotID: "approval.gate", Cardinality: "single", FallbackRenderer: "host.gate", AllowUntrustedPlugin: true}, // must be forced fail-closed!
		})
		if err != nil {
			t.Fatalf("AssembleSurface(%s) failed: %v", target, err)
		}
		if !surf.HasModule(shellmanifest.ModAgentRoom) || !surf.HasModule(shellmanifest.ModTranscript) {
			t.Fatalf("surface %s missing shared core modules: %+v", target, surf)
		}
		// Restricted approval.gate MUST NEVER admit untrusted plugins even if a live payload attempts it.
		if surf.AdmitSlot("approval.gate", false) {
			t.Fatalf("surface %s violated approval.gate fail-closed trust invariant", target)
		}
		if !surf.AdmitSlot("approval.gate", true) {
			t.Fatalf("surface %s should admit host-trusted approval.gate renderer", target)
		}
	}

	desk, err := reg.AssembleSurface(shellmanifest.TargetDesktopMacOS, nil)
	if err != nil {
		t.Fatalf("AssembleSurface(desktop-macos): %v", err)
	}
	if !desk.HasModule(shellmanifest.ModWindowTree) || !desk.HasModule(shellmanifest.ModDoomModeline) || desk.SchemeDrivenCount < 10 {
		t.Fatalf("expected desktop surface to mount Scheme-driven Doom workbench modules, got %+v", desk)
	}
}

func TestProjectKeymapAndTransientsMapsCanonicalAndCustomSchemeBindings(t *testing.T) {
	keymap := []schema.UIKeyBindingSpec{
		{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "v"}, CommandName: "window-split-right"},
		{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "1"}, CommandName: "winum-select-window-1"},
		{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "z"}, CommandName: "custom-scheme-zoom"},
		{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "n"}, CommandName: "workspace-new"},
		{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "~"}, CommandName: "popup-toggle"},
		{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "o"}, CommandName: "window-other"},
		{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "C-c"}, CommandName: "quit"},
		{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "t", "2"}, CommandName: "workspace-new"},
		{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "u"}, CommandName: "winner-undo"},
	}
	transients := []schema.UITransient{
		{
			ID: "zoom",
			Heads: []schema.UITransientHead{
				{Key: "+", CommandName: "zoom-in", Label: "Zoom in", Exit: false},
				{Key: "q", CommandName: "exit", Label: "Quit", Exit: true},
			},
		},
	}

	mapped := uiframework.ProjectKeymapAndTransients(keymap, transients)

	winHydra, ok := mapped["SPC w"]
	if !ok {
		t.Fatalf("expected SPC w hydra projected from Scheme keymap")
	}
	foundCustom := false
	foundSplit := false
	for _, h := range winHydra.Heads {
		if h.Key == "v" && h.Action == "cmd:win-split-right" {
			foundSplit = true
		}
		if h.Key == "z" && h.Action == "scheme:custom-scheme-zoom" {
			foundCustom = true
		}
	}
	if !foundSplit || !foundCustom {
		t.Fatalf("expected both canonical SPC w v and dynamic custom SPC w z -> scheme:custom-scheme-zoom, got %+v", winHydra.Heads)
	}
	if _, ok := mapped["SPC TAB"]; !ok {
		t.Fatalf("expected SPC TAB workspace hydra projected from Scheme keymap")
	}
	if _, ok := mapped["C-x"]; !ok {
		t.Fatalf("expected native Emacs global C-x keymap projected from Scheme")
	}
	if _, ok := mapped["C-x t"]; !ok {
		t.Fatalf("expected native Emacs global C-x t tab-bar keymap projected from Scheme")
	}
	if _, ok := mapped["C-c"]; !ok {
		t.Fatalf("expected native Emacs global C-c keymap projected from Scheme")
	}
	if _, ok := mapped["hydra:zoom"]; !ok {
		t.Fatalf("expected Scheme transient body hydra:zoom projected")
	}
}

func TestProjectWorkbenchValidatesExtensionBoundsResolvesFrameWindowsAndAdaptsMobileWidth(t *testing.T) {
	reg := uiframework.DefaultRegistry()
	snap := schema.UIWorkbenchSnapshot{
		Generation: 7,
		Frame: schema.UIFrame{
			SelectedWindow: "win-2",
			Windows: []schema.UIWindow{
				{ID: "win-1", BufferID: "conversation"},
				{ID: "win-2", BufferID: "*Git Status*"},
			},
		},
		Extensions: []schema.UIExtensionRegion{
			{
				ID:    "git-status",
				Label: "*Git Status*",
				Content: schema.UIElement{
					Kind:  schema.ElemText,
					Spans: []schema.UISpan{{Text: "clean working tree", Tone: schema.ToneSuccess}},
				},
			},
			{
				ID:      "invalid-empty-kind",
				Label:   "*Broken*",
				Content: schema.UIElement{}, // empty Kind must be rejected by MAR-UI-R03 ValidateBounds
			},
		},
		StatusItems: []schema.UIStatusItem{
			{ID: "mode", Label: "MARLIN", KeepPriority: 100},
			{ID: "wide-only", Label: "LONG TELEMETRY PANEL", MinWidth: 100, KeepPriority: 50},
		},
		Commands: []schema.UICommandSpec{
			{Name: "window-split-right", Description: "Split right"},
			{Name: "git-stage-all", Description: "Stage all changes"},
		},
	}

	desk, err := uiframework.ProjectWorkbench(reg, shellmanifest.TargetDesktopMacOS, snap)
	if err != nil {
		t.Fatalf("ProjectWorkbench(desktop): %v", err)
	}
	if len(desk.Extensions) != 1 || desk.Extensions[0].ID != "git-status" || desk.RejectedExtensions != 1 {
		t.Fatalf("expected 1 valid extension and 1 rejected invalid extension, got valid=%d rejected=%d",
			len(desk.Extensions), desk.RejectedExtensions)
	}
	if len(desk.Windows) != 2 || desk.Windows[0].BufferID != "*conversation*" || desk.Windows[1].BufferID != "scheme-ext:git-status" || !desk.Windows[1].IsExtension {
		t.Fatalf("unexpected projected Scheme frame windows: %+v", desk.Windows)
	}
	if len(desk.StatusItems) != 2 {
		t.Fatalf("expected Desktop (120 cols) to keep both status items, got %+v", desk.StatusItems)
	}
	if len(desk.Commands) != 2 || desk.Commands[0].Action != "cmd:win-split-right" || desk.Commands[1].Action != "scheme:git-stage-all" {
		t.Fatalf("unexpected mapped Scheme commands: %+v", desk.Commands)
	}

	mob, err := uiframework.ProjectWorkbench(reg, shellmanifest.TargetMobileIOS, snap)
	if err != nil {
		t.Fatalf("ProjectWorkbench(ios): %v", err)
	}
	if len(mob.StatusItems) != 1 || mob.StatusItems[0].ID != "mode" {
		t.Fatalf("expected Mobile (42 cols) to drop MinWidth=100 item, got %+v", mob.StatusItems)
	}
}
