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
	"SPC g":   {id: "git-magit", label: "+git / Magit", title: "+git / Magit (Doom Emacs · jj + git + Aitia)"},
	"SPC w":   {id: "windows", label: "+windows", title: "+windows (Doom / winum / winner-mode)"},
	"SPC TAB": {id: "workspaces", label: "+workspaces", title: "+workspaces (Doom persp-mode)"},
	"SPC t":   {id: "themes", label: "+themes", title: "+themes & toggles"},
	"SPC m":   {id: "session", label: "+marlin", title: "+marlin session"},
	"SPC q":   {id: "quit", label: "+quit", title: "+quit / session"},
	"C-x":     {id: "emacs-ctl-x", label: "C-x", title: "Emacs C-x (windows · buffers · Magit · tabs · zen)"},
	"C-x t":   {id: "emacs-ctl-x-t", label: "+tab-bar", title: "Emacs C-x t (+workspaces / tab-bar)"},
	"C-x w":   {id: "emacs-ctl-x-w", label: "+window-state", title: "Emacs 30 C-x w (window-state)"},
	"C-c":     {id: "emacs-ctl-c", label: "C-c", title: "Emacs C-c (winner-mode · Magit AI audit · toggles)"},
	"C-c t":   {id: "emacs-ctl-c-t", label: "+toggles", title: "Emacs C-c t (minor-mode toggles · task tabs)"},
	"C-h":     {id: "emacs-ctl-h", label: "C-h", title: "Emacs C-h (help & self-documentation)"},
	"SPC h":   {id: "help", label: "+help", title: "+help (Emacs C-h self-documentation)"},
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
	case "describe-mode":
		return "cmd:describe-mode", "Describe active Major/Minor/Polymode (C-h m)", false
	case "describe-key":
		return "cmd:describe-key", "Describe key or chord (C-h k)", false
	case "describe-function":
		return "cmd:describe-function", "Describe command or function (C-h f)", false
	case "describe-bindings":
		return "cmd:describe-bindings", "Describe all active keybindings (C-h b)", false
	case "toggle-dedicated-window-mode":
		return "cmd:toggle-dedicated-mode", "Toggle dedicated-window-mode", false
	case "toggle-aitia-audit-mode":
		return "cmd:toggle-aitia-mode", "Toggle aitia-audit-minor-mode", false
	case "toggle-read-only-mode":
		return "cmd:toggle-readonly-mode", "Toggle read-only-mode", false
	case "toggle-llm-assist-mode":
		return "cmd:toggle-llm-assist-mode", "Toggle llm-inline-assist-mode", false
	case "kill-buffer":
		return "cmd:kill-buffer", "Kill current buffer (C-x k)", false
	case "ibuffer":
		return "cmd:ibuffer", "Open *Ibuffer* grouped by Major Mode (C-c b)", false
	case "previous-buffer":
		return "cmd:prev-buffer", "Previous buffer", true
	case "next-buffer":
		return "cmd:next-buffer", "Next buffer", true
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
	case "magit-status":
		return "cmd:magit-status", "Magit status (*magit*)", false
	case "magit-dispatch":
		return "hydra:magit:?", "Magit dispatch (?)", false
	case "magit-file-dispatch":
		return "hydra:magit:d", "Magit diff & Aitia audit", false
	case "magit-refresh":
		return "cmd:magit-refresh", "Refresh & Aitia re-audit", false
	case "magit-stage":
		return "cmd:magit-stage", "Stage file or hunk", false
	case "magit-unstage":
		return "cmd:magit-unstage", "Unstage file or hunk", false
	case "magit-ai-audit":
		return "cmd:magit-ai-audit", "AI Code Audit (50/50 split)", false
	case "magit-ai-triage":
		return "cmd:magit-ai-triage", "AI Full Evidence-Chain Triage", false
	case "magit-ai-commit":
		return "cmd:magit-ai-commit", "AI Commit Draft (Aitia trailer)", false
	case "magit-ai-split":
		return "cmd:magit-ai-split", "AI Semantic Split Proposal", false
	case "magit-ai-resolve":
		return "cmd:magit-ai-resolve", "AI 3-Way Conflict Resolution", false
	case "magit-commit-prompt":
		return "cmd:magit-commit-prompt", "Commit staged", false
	case "magit-amend-prompt":
		return "cmd:magit-amend-prompt", "Amend HEAD", false
	case "magit-extend":
		return "cmd:magit-extend", "Extend HEAD (no edit)", false
	case "magit-describe-prompt":
		return "cmd:magit-describe-prompt", "Reword / jj describe (@)", false
	case "magit-jj-new":
		return "cmd:magit-jj-new", "jj new (next change)", false
	case "magit-jj-squash":
		return "cmd:magit-jj-squash", "jj squash into parent", false
	case "magit-jj-undo":
		return "cmd:magit-jj-undo", "jj undo (revert last op)", false
	case "magit-toggle-oplog":
		return "cmd:magit-toggle-oplog", "Toggle jj Operation Log", false
	case "magit-level-3":
		return "cmd:magit-level-3", "Hunks summary (Level 3)", false
	case "magit-level-4":
		return "cmd:magit-level-4", "Expand full diff (Level 4)", false
	case "magit-branch-prompt":
		return "cmd:magit-branch-prompt", "Checkout branch", false
	case "magit-branch-create-prompt":
		return "cmd:magit-branch-create-prompt", "Create & checkout branch", false
	case "magit-branch-delete-prompt":
		return "cmd:magit-branch-delete-prompt", "Delete branch", false
	case "magit-fetch-upstream":
		return "cmd:magit-fetch-upstream", "Fetch from upstream", false
	case "magit-fetch-all":
		return "cmd:magit-fetch-all", "Fetch all remotes", false
	case "magit-pull":
		return "cmd:magit-pull", "Pull from upstream", false
	case "magit-pull-rebase":
		return "cmd:magit-pull-rebase", "Pull --rebase", false
	case "magit-push":
		return "cmd:magit-push", "Push to upstream", false
	case "magit-stash-push":
		return "cmd:magit-stash-push", "Stash both (index + worktree)", false
	case "magit-stash-pop":
		return "cmd:magit-stash-pop", "Pop latest stash", false
	case "magit-stash-apply":
		return "cmd:magit-stash-apply", "Apply latest stash", false
	case "magit-stash-drop":
		return "cmd:magit-stash-drop", "Drop latest stash", false
	case "magit-reset-soft":
		return "cmd:magit-reset-soft", "Soft reset HEAD~1 (keep staged)", false
	case "magit-reset-mixed":
		return "cmd:magit-reset-mixed", "Mixed reset HEAD~1 (unstage)", false
	case "magit-cherry-pick-prompt":
		return "cmd:magit-cherry-pick-prompt", "Cherry-pick revision...", false
	case "magit-revert-prompt":
		return "cmd:magit-revert-prompt", "Revert revision...", false
	}
	if strings.HasPrefix(cmd, "hydra:") {
		return cmd, strings.TrimPrefix(cmd, "hydra:"), false
	}
	if strings.HasPrefix(cmd, "cmd:") {
		return cmd, strings.TrimPrefix(cmd, "cmd:"), false
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
		isPref := strings.HasPrefix(action, "hydra:")
		upsertHead(leafPref, MappedHead{
			Key:      leafKey,
			Label:    label,
			Column:   col,
			Exit:     !repeatable && !isPref,
			IsPrefix: isPref,
			Action:   action,
		})
	}

	// Project explicit Scheme Hydra / Magit transient bodies (`scheme/ui-hydra`, `scheme/ui-magit`).
	for _, tr := range transients {
		if strings.TrimSpace(tr.ID) == "" || len(tr.Heads) == 0 {
			continue
		}
		pref := "hydra:" + tr.ID
		if strings.Contains(tr.ID, ":") {
			pref = tr.ID
		}
		hintTxt := extractHintText(tr.Hint)
		title := fmt.Sprintf("Hydra · %s", tr.ID)
		if hintTxt != "" {
			title = hintTxt
		}
		h := MappedHydra{
			ID:     tr.ID,
			Prefix: pref,
			Title:  title,
			Hint:   hintTxt,
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
			isPref := strings.HasPrefix(action, "hydra:")
			h.Heads = append(h.Heads, MappedHead{
				Key:      th.Key,
				Label:    lbl,
				Column:   "Transient",
				Exit:     (th.Exit || action == "") && !isPref,
				IsPrefix: isPref,
				Action:   action,
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
