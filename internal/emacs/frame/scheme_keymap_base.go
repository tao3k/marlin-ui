// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import "github.com/egoist/mygo/ui"

// loadSchemeBaseKeymaps projects the authoritative Doom Emacs muscle-memory
// keymap declared upstream in `marlin-agent-core/scheme/ui-meow/key-bindings.ss`
// (`SPC`, `SPC b`, `SPC w`, `SPC TAB`, `SPC ~`).
//
// Desktop-specific extensions (`defaults/keymaps.toml`) and downstream user
// overrides (`~/.config/marlin/ui/keymaps.toml`) layer cleanly on top via
// `mergeHydraOverlay` without duplicating upstream Doom bindings.
func loadSchemeBaseKeymaps() map[string]declarativeHydraDef {
	return map[string]declarativeHydraDef{
		"SPC": {
			ID:     "leader",
			Prefix: "SPC",
			Title:  "Doom Leader / Hydra",
			Heads: []declarativeHeadDef{
				{Key: "SPC", KeyCode: ui.KeySpace, Label: "Execute command (M-x)", Column: "Commands", Exit: true, Action: "cmd:mx"},
				{Key: "x", KeyCode: ui.KeyX, Label: "M-x command palette", Column: "Commands", Exit: true, Action: "cmd:mx"},
				{Key: "s", KeyCode: ui.KeyS, Label: "Helm live search", Column: "Commands", Exit: true, Action: "cmd:helm"},
				{Key: "b", KeyCode: ui.KeyB, Label: "+buffers", Column: "Navigation", IsPrefix: true, Action: "hydra:SPC b"},
				{Key: "w", KeyCode: ui.KeyW, Label: "+windows", Column: "Navigation", IsPrefix: true, Action: "hydra:SPC w"},
				{Key: "TAB", KeyCode: ui.KeyTab, Label: "+workspaces", Column: "Navigation", IsPrefix: true, Action: "hydra:SPC TAB"},
				{Key: "~", KeyCode: ui.KeyBackquote, KeyMods: ui.Shift, Label: "Toggle last popup", Column: "Navigation", Exit: true, Action: "cmd:popup-toggle"},
				{Key: "h", KeyCode: ui.KeyH, Label: "Documentation (*Help*)", Column: "System", Exit: true, Action: "buffer:*Help*"},
			},
		},
		"SPC b": {
			ID:     "buffers",
			Prefix: "SPC b",
			Title:  "+buffers",
			Heads: []declarativeHeadDef{
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch buffer (Vertico)", Column: "Buffers", Exit: true, Action: "cmd:switch-buffer"},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Quit hydra", Column: "Diagnostics", Exit: true},
			},
		},
		// Exact Doom Emacs `evil-window-map` + `winum` + `winner-mode` + `toggle-window-dedicated`
		// mirrored 1:1 from scheme/ui-meow/key-bindings.ss.
		"SPC w": {
			ID:     "windows",
			Prefix: "SPC w",
			Title:  "+windows (Doom / winum / winner-mode)",
			Heads: []declarativeHeadDef{
				{Key: "v", KeyCode: ui.KeyV, Label: "Split window right", Column: "Splits", Exit: true, Action: "cmd:win-split-right"},
				{Key: "s", KeyCode: ui.KeyS, Label: "Split window below", Column: "Splits", Exit: true, Action: "cmd:win-split-below"},
				{Key: "w", KeyCode: ui.KeyW, Label: "Other window", Column: "Splits", Exit: false, Action: "cmd:win-other"},
				{Key: "W", KeyCode: ui.KeyW, KeyMods: ui.Shift, Label: "Previous window", Column: "Splits", Exit: false, Action: "cmd:win-prev"},
				{Key: "x", KeyCode: ui.KeyX, Label: "Swap windows", Column: "Splits", Exit: false, Action: "cmd:win-swap"},
				{Key: "l", KeyCode: ui.KeyL, Label: "Widen window", Column: "Splits", Exit: false, Action: "cmd:win-widen"},
				{Key: "h", KeyCode: ui.KeyH, Label: "Narrow window", Column: "Splits", Exit: false, Action: "cmd:win-narrow"},
				{Key: "=", KeyCode: ui.KeyEqual, Label: "Balance windows", Column: "Splits", Exit: true, Action: "cmd:win-balance"},
				{Key: "e", KeyCode: ui.KeyE, Label: "Balance windows", Column: "Splits", Exit: true, Action: "cmd:win-balance"},
				{Key: "d", KeyCode: ui.KeyD, Label: "Close window / workspace", Column: "Splits", Exit: true, Action: "cmd:win-delete"},
				{Key: "c", KeyCode: ui.KeyC, Label: "Close window / workspace", Column: "Splits", Exit: true, Action: "cmd:win-delete"},
				{Key: "o", KeyCode: ui.KeyO, Label: "Delete other windows", Column: "Splits", Exit: true, Action: "cmd:win-only"},
				{Key: "1", KeyCode: ui.Key1, Label: "Select window 1", Column: "Jump", Exit: true, Action: "cmd:win-1"},
				{Key: "2", KeyCode: ui.Key2, Label: "Select window 2", Column: "Jump", Exit: true, Action: "cmd:win-2"},
				{Key: "3", KeyCode: ui.Key3, Label: "Select window 3", Column: "Jump", Exit: true, Action: "cmd:win-3"},
				{Key: "4", KeyCode: ui.Key4, Label: "Select window 4", Column: "Jump", Exit: true, Action: "cmd:win-4"},
				{Key: "5", KeyCode: ui.Key5, Label: "Select window 5", Column: "Jump", Exit: true, Action: "cmd:win-5"},
				{Key: "6", KeyCode: ui.Key6, Label: "Select window 6", Column: "Jump", Exit: true, Action: "cmd:win-6"},
				{Key: "7", KeyCode: ui.Key7, Label: "Select window 7", Column: "Jump", Exit: true, Action: "cmd:win-7"},
				{Key: "8", KeyCode: ui.Key8, Label: "Select window 8", Column: "Jump", Exit: true, Action: "cmd:win-8"},
				{Key: "9", KeyCode: ui.Key9, Label: "Select window 9", Column: "Jump", Exit: true, Action: "cmd:win-9"},
				{Key: "u", KeyCode: ui.KeyU, Label: "Winner undo layout", Column: "History", Exit: false, Action: "cmd:win-undo"},
				{Key: "U", KeyCode: ui.KeyU, KeyMods: ui.Shift, Label: "Winner redo layout", Column: "History", Exit: false, Action: "cmd:win-redo"},
				{Key: "t", KeyCode: ui.KeyT, Label: "Toggle dedicated pin", Column: "Popups", Exit: true, Action: "cmd:win-dedicated"},
				{Key: "p", KeyCode: ui.KeyP, Label: "Toggle popup drawer", Column: "Popups", Exit: true, Action: "cmd:popup-toggle"},
				{Key: "r", KeyCode: ui.KeyR, Label: "Raise popup to window", Column: "Popups", Exit: true, Action: "cmd:popup-raise"},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Quit hydra", Column: "Popups", Exit: true},
			},
		},
		// Exact Doom Emacs `SPC TAB` (+workspaces / persp-mode) mirrored 1:1 from scheme/ui-meow/key-bindings.ss.
		"SPC TAB": {
			ID:     "workspaces",
			Prefix: "SPC TAB",
			Title:  "+workspaces (Doom persp-mode)",
			Heads: []declarativeHeadDef{
				{Key: "n", KeyCode: ui.KeyN, Label: "New workspace", Column: "Manage", Exit: true, Action: "cmd:ws-new"},
				{Key: "N", KeyCode: ui.KeyN, KeyMods: ui.Shift, Label: "New named workspace", Column: "Manage", Exit: true, Action: "cmd:ws-create-prompt"},
				{Key: "d", KeyCode: ui.KeyD, Label: "Kill this workspace", Column: "Manage", Exit: true, Action: "cmd:ws-kill"},
				{Key: "]", KeyCode: ui.KeyBracketRight, Label: "Next workspace", Column: "Manage", Exit: false, Action: "cmd:ws-next"},
				{Key: "[", KeyCode: ui.KeyBracketLeft, Label: "Previous workspace", Column: "Manage", Exit: false, Action: "cmd:ws-prev"},
				{Key: "1", KeyCode: ui.Key1, Label: "Switch to workspace #1", Column: "Jump", Exit: true, Action: "cmd:ws-1"},
				{Key: "2", KeyCode: ui.Key2, Label: "Switch to workspace #2", Column: "Jump", Exit: true, Action: "cmd:ws-2"},
				{Key: "3", KeyCode: ui.Key3, Label: "Switch to workspace #3", Column: "Jump", Exit: true, Action: "cmd:ws-3"},
				{Key: "4", KeyCode: ui.Key4, Label: "Switch to workspace #4", Column: "Jump", Exit: true, Action: "cmd:ws-4"},
				{Key: "0", KeyCode: ui.Key0, Label: "Switch to final workspace", Column: "Jump", Exit: true, Action: "cmd:ws-final"},
				{Key: "q", KeyCode: ui.KeyQ, Label: "Quit hydra", Column: "Jump", Exit: true},
			},
		},
	}
}
