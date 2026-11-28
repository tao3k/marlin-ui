// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import "github.com/egoist/mygo/ui"

// loadStandaloneBootstrapKeymaps provides ONLY the minimal emergency bootstrap
// chords (M-x, find-file, switch-buffer, quit) when running completely offline
// with no live Rust/Scheme gRPC host connected.
//
// Whenever a live `marlin.ui.v1` `UiWorkbenchSnapshot` arrives from Gerbil
// Scheme over gRPC (`f.liveSchemeHydras`), it replaces this bootstrap directly
// as the authoritative Single Source of Truth (zero duplicated Scheme keymap
// tables in production Go code).
func loadStandaloneBootstrapKeymaps() map[string]declarativeHydraDef {
	return map[string]declarativeHydraDef{
		"SPC": {
			ID:     "leader",
			Prefix: "SPC",
			Title:  "Doom Leader (offline bootstrap — connect Scheme host for full keymap)",
			Heads: []declarativeHeadDef{
				{Key: "SPC", KeyCode: ui.KeySpace, Label: "Execute command (M-x)", Column: "Commands", Exit: true, Action: "cmd:mx"},
				{Key: "f", KeyCode: ui.KeyF, Label: "+files", Column: "Navigation", IsPrefix: true, Action: "hydra:SPC f"},
				{Key: "b", KeyCode: ui.KeyB, Label: "+buffers", Column: "Navigation", IsPrefix: true, Action: "hydra:SPC b"},
				{Key: "q", KeyCode: ui.KeyQ, Label: "+quit", Column: "System", IsPrefix: true, Action: "hydra:SPC q"},
			},
		},
		"SPC f": {
			ID:     "files",
			Prefix: "SPC f",
			Title:  "+files",
			Heads: []declarativeHeadDef{
				{Key: "f", KeyCode: ui.KeyF, Label: "Find file (C-x C-f)", Column: "Files", Exit: true, Action: "cmd:find-file"},
				{Key: "p", KeyCode: ui.KeyP, Label: "Find file in project", Column: "Files", Exit: true, Action: "cmd:project-find-file"},
				{Key: "r", KeyCode: ui.KeyR, Label: "Open recent file", Column: "Files", Exit: true, Action: "cmd:recentf"},
			},
		},
		"SPC b": {
			ID:     "buffers",
			Prefix: "SPC b",
			Title:  "+buffers",
			Heads: []declarativeHeadDef{
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch buffer (C-x b)", Column: "Buffers", Exit: true, Action: "cmd:switch-buffer"},
				{Key: "i", KeyCode: ui.KeyI, Label: "Open *Ibuffer*", Column: "Buffers", Exit: true, Action: "cmd:ibuffer"},
			},
		},
		"SPC q": {
			ID:     "quit",
			Prefix: "SPC q",
			Title:  "+quit",
			Heads: []declarativeHeadDef{
				{Key: "q", KeyCode: ui.KeyQ, Label: "Exit Marlin (C-x C-c)", Column: "System", Exit: true, Action: "cmd:quit"},
			},
		},
		"C-x": {
			ID:     "emacs-ctl-x",
			Prefix: "C-x",
			Title:  "Emacs C-x (offline bootstrap)",
			Heads: []declarativeHeadDef{
				{Key: "C-f", KeyCode: ui.KeyF, KeyMods: ui.Ctrl, Label: "Find file", Column: "Files", Exit: true, Action: "cmd:find-file"},
				{Key: "b", KeyCode: ui.KeyB, Label: "Switch buffer", Column: "Buffers", Exit: true, Action: "cmd:switch-buffer"},
				{Key: "C-c", KeyCode: ui.KeyC, KeyMods: ui.Ctrl, Label: "Exit Marlin", Column: "Emacs", Exit: true, Action: "cmd:quit"},
			},
		},
	}
}
