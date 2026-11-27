// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package hydra implements Marlin UI's column-grouped Doom Emacs Hydra &
// Which-Key Minibuffer popup, aligned with marlin-agent-core/scheme/ui-hydra.
// Pressing SPC in Meow NORMAL mode opens the multi-column leader Hydra popup
// directly above the Echo Area; prefix heads (+buffers, +themes, +session)
// transition smoothly into sub-hydras, while repeatable heads (Exit=false)
// keep the Hydra active until an exit head or C-g/ESC is pressed.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A09 (emacsHydra).
package hydra

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// ForeignPolicy defines what happens when an unbound key is pressed inside a Hydra.
type ForeignPolicy uint8

const (
	ForeignExit ForeignPolicy = iota
	ForeignWarn
)

// Head defines one keybinding entry inside a Hydra body (mirrors scheme/ui-hydra).
type Head struct {
	Key      string       // display keycap: "SPC", "TAB", "w", "W", "1", "~", "q"
	KeyCode  ui.Key       // matching MyGo keyboard key
	KeyMods  ui.Modifiers // optional modifiers (e.g. ui.Shift for "W", "U", "N", "~")
	Label    string       // short description (e.g. "+buffers", "Helm search")
	Column   string       // visual column heading (e.g. "Commands", "Buffers", "Theme")
	Exit     bool         // true = close hydra after running; false = repeatable red head
	IsPrefix bool         // true = transitions to a nested Hydra prefix
	Run      func()
}

// Body defines a complete named Hydra menu (max 16 heads, column-grouped).
type Body struct {
	ID      string
	Prefix  string // e.g. "SPC", "SPC b", "SPC t", "SPC m"
	Title   string // e.g. "Doom Leader", "+buffers", "+themes"
	Foreign ForeignPolicy
	Heads   []Head
}

// ColumnGroup preserves deterministic column ordering for rendering.
type ColumnGroup struct {
	Name  string
	Heads []Head
}

// GroupColumns groups heads into ordered columns matching scheme/ui-hydra ui-key-hint-table.
func (b Body) GroupColumns() []ColumnGroup {
	var order []string
	byCol := make(map[string][]Head)
	for _, h := range b.Heads {
		col := h.Column
		if col == "" {
			col = "Actions"
		}
		if _, exists := byCol[col]; !exists {
			order = append(order, col)
		}
		byCol[col] = append(byCol[col], h)
	}
	out := make([]ColumnGroup, 0, len(order))
	for _, name := range order {
		out = append(out, ColumnGroup{Name: name, Heads: byCol[name]})
	}
	return out
}

// MatchKey checks whether any head in the Hydra body matches the current frame's keypress.
func (b Body) MatchKey(c *ui.Context) (Head, bool) {
	return b.MatchKeyInScope(c, ui.Element{})
}

// MatchKeyInScope checks whether any head in the Hydra body matches the current
// frame's keypress, claiming keys on scope so no focused descendant button can
// hijack Space/keys while a Hydra menu is active.
func (b Body) MatchKeyInScope(c *ui.Context, scope ui.Element) (Head, bool) {
	if c == nil {
		return Head{}, false
	}
	// Native Emacs Ctrl prefixes (C-x, C-c, C-h) intentionally remain active
	// even in Insert mode while a text input has focus.
	allowDuringText := strings.HasPrefix(b.Prefix, "C-")
	if !allowDuringText && c.FocusTakesText() {
		return Head{}, false
	}
	// Check modified heads (e.g. Shift+W, Shift+U, Shift+N, ~) before unmodified heads
	// so uppercase Doom bindings never get shadowed by their lowercase sibling.
	for _, wantMod := range []bool{true, false} {
		for _, h := range b.Heads {
			if h.KeyCode == 0 || (h.KeyMods != 0) != wantMod {
				continue
			}
			scoped := false
			if scope != (ui.Element{}) {
				scoped = scope.Shortcut(h.KeyMods, h.KeyCode)
			}
			global := c.Shortcut(h.KeyMods, h.KeyCode)
			if scoped || global {
				return h, true
			}
		}
	}
	return Head{}, false
}
