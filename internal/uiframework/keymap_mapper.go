// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package uiframework

import (
	"fmt"
	"strings"

	"github.com/tao3k/marlin-mygo/pkg/schema"
)

// MappedHead is a host-agnostic Hydra head projected directly from live Scheme
// `marlin.ui.v1` `UIKeyBindingSpec` or `UITransientHead` declarations.
type MappedHead struct {
	Key      string
	Label    string
	Column   string
	Exit     bool
	IsPrefix bool
	Action   string
}

// MappedHydra is a prefix-grouped Hydra body projected directly from Scheme.
type MappedHydra struct {
	ID     string
	Prefix string
	Title  string
	Hint   string
	Heads  []MappedHead
}

var schemePrefixMeta = map[string]struct {
	id    string
	label string
	title string
}{
	"SPC":     {id: "leader", label: "Leader", title: "Doom Leader / Hydra"},
	"SPC b":   {id: "buffers", label: "+buffers", title: "+buffers"},
	"SPC w":   {id: "windows", label: "+windows", title: "+windows (Doom / winum / winner-mode)"},
	"SPC TAB": {id: "workspaces", label: "+workspaces", title: "+workspaces (Doom persp-mode)"},
	"SPC t":   {id: "themes", label: "+themes", title: "+themes & toggles"},
	"SPC m":   {id: "session", label: "+marlin", title: "+marlin session"},
	"SPC q":   {id: "quit", label: "+quit", title: "+quit / session"},
	"C-x":     {id: "emacs-ctl-x", label: "C-x", title: "Emacs C-x (windows · buffers · tabs · zen)"},
	"C-x t":   {id: "emacs-ctl-x-t", label: "+tab-bar", title: "Emacs C-x t (+workspaces / tab-bar)"},
	"C-x w":   {id: "emacs-ctl-x-w", label: "+window-state", title: "Emacs 30 C-x w (window-state)"},
	"C-c":     {id: "emacs-ctl-c", label: "C-c", title: "Emacs C-c (winner-mode · mode)"},
	"C-h":     {id: "emacs-ctl-h", label: "C-h", title: "Emacs C-h (help)"},
}

// ResolveSchemeAction maps canonical Scheme command symbols (declared in
// `scheme/ui-meow/key-bindings.ss` and `scheme/ui-components/command-catalog.ss`)
// onto native MyGo frontend actions, forwarding any unknown Scheme command as
// `scheme:<commandName>` so new upstream Scheme bindings execute with ZERO Go edits.
func ResolveSchemeAction(commandName string) (action string, defaultLabel string, repeatable bool) {
	cmd := strings.TrimSpace(commandName)
	if cmd == "" {
		return "", "", false
	}
	switch cmd {
	case "open-commands":
		return "cmd:mx", "Execute command (M-x)", false
	case "switch-buffer":
		return "cmd:switch-buffer", "Switch buffer (Vertico)", false
	case "helm":
		return "cmd:helm", "Helm live search", false
	case "help":
		return "buffer:*Help*", "Documentation (*Help*)", false
	case "switch-themes":
		return "cmd:switch-theme", "Switch theme", false
	case "window-split-right", "split-window-right":
		return "cmd:win-split-right", "Split window right", false
	case "window-split-below", "split-window-below":
		return "cmd:win-split-below", "Split window below", false
	case "window-other", "other-window":
		return "cmd:win-other", "Other window", true
	case "window-prev":
		return "cmd:win-prev", "Previous window", true
	case "window-swap", "swap-window-panes":
		return "cmd:win-swap", "Swap windows", true
	case "window-delete", "delete-window":
		return "cmd:win-delete", "Close window / workspace", false
	case "window-only", "delete-other-windows":
		return "cmd:win-only", "Delete other windows", false
	case "window-balance", "balance-windows":
		return "cmd:win-balance", "Balance windows", false
	case "window-widen":
		return "cmd:win-widen", "Widen window", true
	case "window-narrow":
		return "cmd:win-narrow", "Narrow window", true
	case "winner-undo":
		return "cmd:win-undo", "Winner undo layout", true
	case "winner-redo":
		return "cmd:win-redo", "Winner redo layout", true
	case "window-toggle-dedicated", "toggle-window-dedicated":
		return "cmd:win-dedicated", "Toggle dedicated pin", false
	case "popup-toggle":
		return "cmd:popup-toggle", "Toggle popup drawer", false
	case "popup-raise":
		return "cmd:popup-raise", "Raise popup to window", false
	case "workspace-new":
		return "cmd:ws-new", "New workspace", false
	case "workspace-new-named":
		return "cmd:ws-create-prompt", "New named workspace", false
	case "workspace-kill":
		return "cmd:ws-kill", "Kill workspace", false
	case "workspace-next":
		return "cmd:ws-next", "Next workspace", true
	case "workspace-prev":
		return "cmd:ws-prev", "Previous workspace", true
	case "workspace-switch-1":
		return "cmd:ws-1", "Switch to workspace #1", false
	case "workspace-switch-2":
		return "cmd:ws-2", "Switch to workspace #2", false
	case "workspace-switch-3":
		return "cmd:ws-3", "Switch to workspace #3", false
	case "workspace-switch-4":
		return "cmd:ws-4", "Switch to workspace #4", false
	case "workspace-switch-final":
		return "cmd:ws-final", "Switch to final workspace", false
	case "zen-mode":
		return "cmd:zen-mode", "Toggle Zen Writeroom", false
	case "zen-fullscreen":
		return "cmd:zen-fullscreen", "Toggle Zen Fullscreen", false
	case "quit", "save-buffers-kill-terminal":
		return "cmd:quit", "Exit Marlin", false
	}
	if strings.HasPrefix(cmd, "winum-select-window-") {
		n := strings.TrimPrefix(cmd, "winum-select-window-")
		if len(n) == 1 && n[0] >= '1' && n[0] <= '9' {
			return "cmd:win-" + n, "Select window " + n, false
		}
	}
	return "scheme:" + cmd, cmd, false
}

// ProjectKeymapAndTransients transforms live `marlin.ui.v1` Scheme keybindings
// (`UIWorkbenchSnapshot.Keymap`) and Scheme Hydra bodies (`UIWorkbenchSnapshot.Transients`)
// into prefix-indexed MappedHydra definitions ready for MyGo frame mounting.
func ProjectKeymapAndTransients(
	keymap []schema.UIKeyBindingSpec,
	transients []schema.UITransient,
) map[string]MappedHydra {
	out := make(map[string]MappedHydra)

	ensureHydra := func(prefix string) MappedHydra {
		if h, ok := out[prefix]; ok {
			return h
		}
		meta, hasMeta := schemePrefixMeta[prefix]
		id := strings.ToLower(strings.ReplaceAll(prefix, " ", "-"))
		title := prefix
		if hasMeta {
			id = meta.id
			title = meta.title
		}
		h := MappedHydra{ID: id, Prefix: prefix, Title: title}
		out[prefix] = h
		return h
	}

	upsertHead := func(prefix string, head MappedHead) {
		h := ensureHydra(prefix)
		for i := range h.Heads {
			if h.Heads[i].Key == head.Key {
				h.Heads[i] = head
				out[prefix] = h
				return
			}
		}
		h.Heads = append(h.Heads, head)
		out[prefix] = h
	}

	for _, kb := range keymap {
		if kb.Scope != schema.KeyScopeNormal && kb.Scope != schema.KeyScopeGlobal && kb.Scope != "" {
			continue
		}
		if len(kb.Sequence) < 2 {
			continue
		}
		root := strings.TrimSpace(kb.Sequence[0])
		if root != "SPC" && root != "C-x" && root != "C-c" && root != "C-h" {
			continue
		}
		// Skip internal Scheme keypad modifier expansions (SPC c ... / SPC m ...).
		if root == "SPC" && len(kb.Sequence) >= 3 && (kb.Sequence[1] == "c" || kb.Sequence[1] == "m") {
			continue
		}
		cmd := strings.TrimSpace(kb.CommandName)
		if cmd == "" || kb.ActionKind == "prefix_hint" {
			continue
		}
		action, label, repeatable := ResolveSchemeAction(cmd)
		if action == "" {
			continue
		}

		// Ensure parent prefix transitions exist (e.g. SPC -> SPC w -> v).
		for depth := 1; depth < len(kb.Sequence)-1; depth++ {
			parentPref := strings.Join(kb.Sequence[:depth], " ")
			key := kb.Sequence[depth]
			childPref := strings.Join(kb.Sequence[:depth+1], " ")
			lbl := "+" + key
			if meta, ok := schemePrefixMeta[childPref]; ok {
				lbl = meta.label
			}
			upsertHead(parentPref, MappedHead{
				Key:      key,
				Label:    lbl,
				Column:   "Navigation",
				IsPrefix: true,
				Action:   "hydra:" + childPref,
			})
		}

		leafPref := strings.Join(kb.Sequence[:len(kb.Sequence)-1], " ")
		leafKey := kb.Sequence[len(kb.Sequence)-1]
		col := inferColumn(leafPref, leafKey, cmd)
		upsertHead(leafPref, MappedHead{
			Key:    leafKey,
			Label:  label,
			Column: col,
			Exit:   !repeatable,
			Action: action,
		})
	}

	// Project explicit Scheme Hydra transient bodies (`scheme/ui-hydra`).
	for _, tr := range transients {
		if strings.TrimSpace(tr.ID) == "" || len(tr.Heads) == 0 {
			continue
		}
		pref := "hydra:" + tr.ID
		h := MappedHydra{
			ID:     tr.ID,
			Prefix: pref,
			Title:  fmt.Sprintf("Hydra · %s", tr.ID),
			Hint:   extractHintText(tr.Hint),
		}
		for _, th := range tr.Heads {
			action, defLbl, _ := ResolveSchemeAction(th.CommandName)
			lbl := strings.TrimSpace(th.Label)
			if lbl == "" {
				lbl = defLbl
			}
			if th.CommandName == "exit" || th.CommandName == "" {
				action = ""
				if lbl == "" {
					lbl = "Quit"
				}
			}
			h.Heads = append(h.Heads, MappedHead{
				Key:    th.Key,
				Label:  lbl,
				Column: "Hydra",
				Exit:   th.Exit || action == "",
				Action: action,
			})
		}
		out[pref] = h
	}

	return out
}

func extractHintText(el schema.UIElement) string {
	if t := strings.TrimSpace(el.Title); t != "" {
		return t
	}
	var sb strings.Builder
	for _, sp := range el.Spans {
		if t := strings.TrimSpace(sp.Text); t != "" {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(t)
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	for _, ch := range el.Children {
		if t := extractHintText(ch); t != "" {
			return t
		}
	}
	return ""
}

func inferColumn(prefix, key, cmd string) string {
	switch prefix {
	case "SPC":
		if key == "SPC" || key == "x" || key == "s" {
			return "Commands"
		}
		if key == "h" {
			return "System"
		}
		return "Navigation"
	case "SPC w":
		if strings.HasPrefix(cmd, "winum-select-window-") {
			return "Jump"
		}
		if strings.HasPrefix(cmd, "winner-") {
			return "History"
		}
		if strings.HasPrefix(cmd, "popup-") || cmd == "window-toggle-dedicated" {
			return "Popups"
		}
		return "Splits"
	case "SPC TAB", "C-x t":
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			return "Jump"
		}
		return "Manage"
	case "C-x":
		if key == "0" || key == "1" || key == "2" || key == "3" || key == "o" || key == "O" || key == "+" {
			return "Windows"
		}
		if key == "b" || key == "p" {
			return "Buffers"
		}
		return "Emacs"
	case "C-c", "C-h", "C-x w":
		return "Emacs"
	default:
		return "Scheme"
	}
}
