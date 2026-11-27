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
				{Key: "q", KeyCode: ui.KeyQ, Label: "+quit", Column: "System", IsPrefix: true, Action: "hydra:SPC q"},
				{Key: "h", KeyCode: ui.KeyH, Label: "+help", Column: "System", IsPrefix: true, Action: "hydra:SPC h"},
			},
		},
		"SPC h": {
			ID:     "help",
			Prefix: "SPC h",
			Title:  "+help (Emacs C-h self-documentation)",
			Heads: []declarativeHeadDef{
				{Key: "m", KeyCode: ui.KeyM, Label: "Describe active mode & Polymode (C-h m)", Column: "Help", Exit: true, Action: "cmd:describe-mode"},
				{Key: "r", KeyCode: ui.KeyR, Label: "Open *Help* keybindings (C-h r)", Column: "Help", Exit: true, Action: "buffer:*Help*"},
			},
		},
		"SPC q": {
			ID:     "quit",
			Prefix: "SPC q",
			Title:  "+quit / session",
			Heads: []declarativeHeadDef{
				{Key: "q", KeyCode: ui.KeyQ, Label: "Exit Marlin (C-x C-c)", Column: "System", Exit: true, Action: "cmd:quit"},
			},
		},
		"SPC b": {
			ID:     "buffers",
			Prefix: "SPC b",
			Title:  "+buffers",
			Heads: []declarativeHeadDef{
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch / Create buffer (Vertico)", Column: "Buffers", Exit: true, Action: "cmd:switch-buffer"},
				{Key: "i", KeyCode: ui.KeyI, Label: "Open *Ibuffer* by Major Mode (C-c b)", Column: "Buffers", Exit: true, Action: "cmd:ibuffer"},
				{Key: "d", KeyCode: ui.KeyD, Label: "Kill active buffer (C-x k)", Column: "Buffers", Exit: true, Action: "cmd:kill-buffer"},
				{Key: "[", KeyCode: ui.KeyBracketLeft, Label: "Previous buffer", Column: "Buffers", Exit: false, Action: "cmd:prev-buffer"},
				{Key: "]", KeyCode: ui.KeyBracketRight, Label: "Next buffer", Column: "Buffers", Exit: false, Action: "cmd:next-buffer"},
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
		// Canonical native Emacs ctl-x-map / C-c / C-h mirrored 1:1 from
		// scheme/ui-meow/key-bindings.ss (active in BOTH Insert and Normal modes!).
		"C-x": {
			ID:     "emacs-ctl-x",
			Prefix: "C-x",
			Title:  "Emacs C-x (windows · buffers · tabs · zen)",
			Heads: []declarativeHeadDef{
				{Key: "0", KeyCode: ui.Key0, Label: "Delete window / workspace", Column: "Windows", Exit: true, Action: "cmd:win-delete"},
				{Key: "1", KeyCode: ui.Key1, Label: "Delete other windows", Column: "Windows", Exit: true, Action: "cmd:win-only"},
				{Key: "2", KeyCode: ui.Key2, Label: "Split window below", Column: "Windows", Exit: true, Action: "cmd:win-split-below"},
				{Key: "3", KeyCode: ui.Key3, Label: "Split window right", Column: "Windows", Exit: true, Action: "cmd:win-split-right"},
				{Key: "o", KeyCode: ui.KeyO, Label: "Other window", Column: "Windows", Exit: true, Action: "cmd:win-other"},
				{Key: "O", KeyCode: ui.KeyO, KeyMods: ui.Shift, Label: "Previous window", Column: "Windows", Exit: true, Action: "cmd:win-prev"},
				{Key: "+", KeyCode: ui.KeyEqual, KeyMods: ui.Shift, Label: "Balance windows", Column: "Windows", Exit: true, Action: "cmd:win-balance"},
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch buffer", Column: "Buffers", Exit: true, Action: "cmd:switch-buffer"},
				{Key: "k", KeyCode: ui.KeyK, Label: "Kill active buffer", Column: "Buffers", Exit: true, Action: "cmd:kill-buffer"},
				{Key: "p", KeyCode: ui.KeyP, Label: "Toggle popup drawer", Column: "Buffers", Exit: true, Action: "cmd:popup-toggle"},
				{Key: "g", KeyCode: ui.KeyG, Label: "Magit status (*magit*)", Column: "Magit", Exit: true, Action: "cmd:magit-status"},
				{Key: "M-g", KeyCode: ui.KeyG, KeyMods: ui.Alt, Label: "Magit dispatch (?)", Column: "Magit", IsPrefix: true, Action: "hydra:magit:?"},
				{Key: "w", KeyCode: ui.KeyW, Label: "+window-state", Column: "Emacs", IsPrefix: true, Action: "hydra:C-x w"},
				{Key: "t", KeyCode: ui.KeyT, Label: "+tab-bar workspaces", Column: "Emacs", IsPrefix: true, Action: "hydra:C-x t"},
				{Key: "z", KeyCode: ui.KeyZ, Label: "Toggle Zen Writeroom", Column: "Emacs", Exit: true, Action: "cmd:zen-mode"},
				{Key: "Z", KeyCode: ui.KeyZ, KeyMods: ui.Shift, Label: "Toggle Zen Fullscreen", Column: "Emacs", Exit: true, Action: "cmd:zen-fullscreen"},
				{Key: "C-c", KeyCode: ui.KeyC, KeyMods: ui.Ctrl, Label: "Exit Marlin", Column: "Emacs", Exit: true, Action: "cmd:quit"},
			},
		},
		"C-x t": {
			ID:     "emacs-ctl-x-t",
			Prefix: "C-x t",
			Title:  "Emacs C-x t (+workspaces / tab-bar)",
			Heads: []declarativeHeadDef{
				{Key: "2", KeyCode: ui.Key2, Label: "New workspace", Column: "Manage", Exit: true, Action: "cmd:ws-new"},
				{Key: "0", KeyCode: ui.Key0, Label: "Kill workspace", Column: "Manage", Exit: true, Action: "cmd:ws-kill"},
				{Key: "o", KeyCode: ui.KeyO, Label: "Next workspace", Column: "Manage", Exit: true, Action: "cmd:ws-next"},
				{Key: "O", KeyCode: ui.KeyO, KeyMods: ui.Shift, Label: "Previous workspace", Column: "Manage", Exit: true, Action: "cmd:ws-prev"},
			},
		},
		"C-x w": {
			ID:     "emacs-ctl-x-w",
			Prefix: "C-x w",
			Title:  "Emacs 30 C-x w (window-state)",
			Heads: []declarativeHeadDef{
				{Key: "d", KeyCode: ui.KeyD, Label: "Toggle window dedicated", Column: "Emacs", Exit: true, Action: "cmd:win-dedicated"},
			},
		},
		"C-c": {
			ID:     "emacs-ctl-c",
			Prefix: "C-c",
			Title:  "Emacs C-c (winner-mode · Magit AI audit)",
			Heads: []declarativeHeadDef{
				{Key: "u", KeyCode: ui.KeyU, Label: "Winner undo layout", Column: "Emacs", Exit: true, Action: "cmd:win-undo"},
				{Key: "U", KeyCode: ui.KeyU, KeyMods: ui.Shift, Label: "Winner redo layout", Column: "Emacs", Exit: true, Action: "cmd:win-redo"},
				{Key: "g", KeyCode: ui.KeyG, Label: "Magit AI Code Audit (50/50 split)", Column: "Magit", Exit: true, Action: "cmd:magit-ai-audit"},
				{Key: "M-g", KeyCode: ui.KeyG, KeyMods: ui.Alt, Label: "Magit diff & Aitia audit", Column: "Magit", IsPrefix: true, Action: "hydra:magit:d"},
				{Key: "b", KeyCode: ui.KeyB, Label: "Open *Ibuffer* by Major Mode", Column: "Buffers", Exit: true, Action: "cmd:ibuffer"},
				{Key: "[", KeyCode: ui.KeyBracketLeft, Label: "Previous buffer", Column: "Buffers", Exit: true, Action: "cmd:prev-buffer"},
				{Key: "]", KeyCode: ui.KeyBracketRight, Label: "Next buffer", Column: "Buffers", Exit: true, Action: "cmd:next-buffer"},
			},
		},
		"C-h": {
			ID:     "emacs-ctl-h",
			Prefix: "C-h",
			Title:  "Emacs C-h (help & describe-mode)",
			Heads: []declarativeHeadDef{
				{Key: "m", KeyCode: ui.KeyM, Label: "Describe active Major/Minor/Polymode", Column: "Emacs", Exit: true, Action: "cmd:describe-mode"},
				{Key: "r", KeyCode: ui.KeyR, Label: "Open Marlin help", Column: "Emacs", Exit: true, Action: "buffer:*Help*"},
				{Key: "?", KeyCode: ui.KeySlash, KeyMods: ui.Shift, Label: "Open Marlin help", Column: "Emacs", Exit: true, Action: "buffer:*Help*"},
			},
		},
	}
}
