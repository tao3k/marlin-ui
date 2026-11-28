// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import "github.com/tao3k/marlin-mygo/pkg/schema"

// SampleSchemeKeymapBindings returns realistic Scheme `marlin.ui.v1` bindings
// mirroring `scheme/ui-meow/key-bindings.ss` for offline unit tests.
func SampleSchemeKeymapBindings() []schema.UIKeyBindingSpec {
	return []schema.UIKeyBindingSpec{
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "SPC"}, CommandName: "open-commands"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "x"}, CommandName: "open-commands"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "s"}, CommandName: "helm"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "f", "f"}, CommandName: "find-file"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "f", "p"}, CommandName: "project-find-file"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "f", "r"}, CommandName: "recentf-open-files"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "p", "f"}, CommandName: "project-find-file"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "b"}, CommandName: "switch-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "B"}, CommandName: "switch-to-buffer-other-window"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "d"}, CommandName: "kill-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "K"}, CommandName: "kill-buffer-and-window"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "i"}, CommandName: "ibuffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "["}, CommandName: "previous-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "]"}, CommandName: "next-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "R"}, CommandName: "rename-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "b", "z"}, CommandName: "bury-buffer"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "v"}, CommandName: "window-split-right"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "s"}, CommandName: "window-split-below"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "w"}, CommandName: "window-other"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "W"}, CommandName: "window-prev"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "x"}, CommandName: "window-swap"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "l"}, CommandName: "window-widen"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "h"}, CommandName: "window-narrow"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "="}, CommandName: "window-balance"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "d"}, CommandName: "window-delete"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "o"}, CommandName: "window-only"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "1"}, CommandName: "winum-select-window-1"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "2"}, CommandName: "winum-select-window-2"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "3"}, CommandName: "winum-select-window-3"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "u"}, CommandName: "winner-undo"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "U"}, CommandName: "winner-redo"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "t"}, CommandName: "window-toggle-dedicated"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "p"}, CommandName: "popup-toggle"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "r"}, CommandName: "popup-raise"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "m"}, CommandName: "window-zoom-toggle"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "T"}, CommandName: "window-transpose-layout"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "R"}, CommandName: "window-configuration-to-register"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "J"}, CommandName: "jump-to-register"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "w", "L"}, CommandName: "list-registers"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "n"}, CommandName: "workspace-new"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "N"}, CommandName: "workspace-new-named"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "d"}, CommandName: "workspace-kill"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "]"}, CommandName: "workspace-next"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "["}, CommandName: "workspace-prev"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "TAB", "1"}, CommandName: "workspace-switch-1"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "t", "t"}, CommandName: "switch-themes"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "t", "z"}, CommandName: "zen-mode"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "t", "Z"}, CommandName: "zen-fullscreen"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "m"}, CommandName: "describe-mode"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "k"}, CommandName: "describe-key"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "f"}, CommandName: "describe-function"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "b"}, CommandName: "describe-bindings"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "l"}, CommandName: "view-lossage"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "h", "r"}, CommandName: "help"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "."}, CommandName: "embark-act"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "~"}, CommandName: "popup-toggle"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "q", "q"}, CommandName: "quit"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "C-f"}, CommandName: "find-file"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "C-r"}, CommandName: "recentf-open-files"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "b"}, CommandName: "switch-buffer"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "k"}, CommandName: "kill-buffer"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "0"}, CommandName: "window-delete"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "1"}, CommandName: "window-only"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "2"}, CommandName: "window-split-below"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "3"}, CommandName: "window-split-right"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "o"}, CommandName: "window-other"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "O"}, CommandName: "window-prev"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "+"}, CommandName: "window-balance"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "p"}, CommandName: "popup-toggle"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "z"}, CommandName: "zen-mode"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "Z"}, CommandName: "zen-fullscreen"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "C-c"}, CommandName: "quit"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "w", "d"}, CommandName: "window-toggle-dedicated"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "w", "m"}, CommandName: "window-zoom-toggle"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "w", "t"}, CommandName: "window-transpose-layout"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "t", "2"}, CommandName: "workspace-new"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "t", "0"}, CommandName: "workspace-kill"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "t", "o"}, CommandName: "workspace-next"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "t", "O"}, CommandName: "workspace-prev"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "u"}, CommandName: "winner-undo"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "U"}, CommandName: "winner-redo"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "p", "f"}, CommandName: "project-find-file"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-h", "m"}, CommandName: "describe-mode"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-h", "r"}, CommandName: "help"},
	}
}

// ApplyTestSchemeKeymap streams `SampleSchemeKeymapBindings()` through the real
// production `ApplyWorkbenchSnapshot` gRPC projection pipeline for offline tests.
func ApplyTestSchemeKeymap(f *Frame) {
	f.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation: f.syncedWBGen.Load() + 1,
		Keymap:     SampleSchemeKeymapBindings(),
	})
}
