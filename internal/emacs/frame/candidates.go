// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

// buildActionShortcutIndex projects live Marginalia shortcut badges (e.g.
// "SPC w v · C-x 3") directly from the Frame's active Scheme + TOML keymaps.
func (f *Frame) buildActionShortcutIndex(hydras map[string]declarativeHydraDef) map[string]string {
	doomByAct := make(map[string]string, 64)
	emacsByAct := make(map[string]string, 32)
	// Direct global Emacs shortcuts.
	emacsByAct["cmd:mx"] = "M-x"
	emacsByAct["cmd:helm"] = "C-s"
	for i := 1; i <= 9; i++ {
		emacsByAct[fmt.Sprintf("cmd:win-%d", i)] = fmt.Sprintf("M-%d", i)
	}
	prefOrder := []string{"SPC", "SPC f", "SPC p", "SPC w", "SPC b", "SPC h", "SPC g", "SPC TAB", "SPC t", "SPC m", "SPC q", "C-x", "C-x 4", "C-x r", "C-x t", "C-x w", "C-x x", "C-c", "C-c p", "C-c t", "C-h"}
	seen := make(map[string]bool, len(prefOrder))
	visit := func(pref string, def declarativeHydraDef) {
		for _, hd := range def.Heads {
			if hd.Action == "" || hd.IsPrefix {
				continue
			}
			chord := pref + " " + hd.Key
			if strings.HasPrefix(pref, "SPC") {
				if _, ok := doomByAct[hd.Action]; !ok {
					doomByAct[hd.Action] = chord
				}
			} else if strings.HasPrefix(pref, "C-") {
				if _, ok := emacsByAct[hd.Action]; !ok {
					emacsByAct[hd.Action] = chord
				}
			}
		}
	}
	for _, p := range prefOrder {
		seen[p] = true
		if def, ok := hydras[p]; ok {
			visit(p, def)
		}
	}
	for p, def := range hydras {
		if !seen[p] {
			visit(p, def)
		}
	}
	out := make(map[string]string, len(doomByAct)+len(emacsByAct))
	for act, d := range doomByAct {
		if e, ok := emacsByAct[act]; ok && e != d {
			out[act] = d + " · " + e
		} else {
			out[act] = d
		}
	}
	for act, e := range emacsByAct {
		if _, ok := out[act]; !ok {
			out[act] = e
		}
	}
	return out
}

// commandCandidates projects the declarative TOML command catalog
// (defaults/commands.toml + ~/.config/marlin/ui/commands.toml) merged with
// live marlin.ui.v1 gRPC commands — zero hardcoded Go command tables —
// enriched with live Marginalia keybinding annotations.
func (f *Frame) commandCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	declCmds := append([]declarativeCommand(nil), f.tomlCommands...)
	schemeCmds := append([]schema.UICommandSpec(nil), f.schemeCmds...)
	hydras := f.tomlHydras
	f.mu.Unlock()
	shortcuts := f.buildActionShortcutIndex(hydras)

	all := make([]minibuffer.Candidate, 0, len(declCmds)+len(schemeCmds))
	seen := make(map[string]struct{}, len(declCmds)+len(schemeCmds))
	for _, dc := range declCmds {
		cmd := dc
		if cmd.Name == "" {
			continue
		}
		if _, dup := seen[cmd.Name]; dup {
			continue
		}
		seen[cmd.Name] = struct{}{}
		all = append(all, minibuffer.Candidate{
			ID:         cmd.Name,
			Name:       cmd.Name,
			Desc:       cmd.Desc,
			Annotation: shortcuts[cmd.Action],
			Run:        func() { f.dispatchWidgetAction(cmd.Action) },
		})
	}
	for _, sc := range schemeCmds {
		name := strings.TrimSpace(sc.Name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		desc := sc.Description
		if desc == "" {
			desc = "Scheme marlin.ui.v1 command"
		}
		act, _, _ := uiframework.ResolveSchemeAction(name)
		all = append(all, minibuffer.Candidate{
			ID:         "scheme-" + name,
			Name:       name,
			Desc:       desc,
			Annotation: shortcuts[act],
			Run:        func() { f.dispatchWidgetAction(act) },
		})
	}
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		for _, cmd := range f.Adaptive.Desktop.Commands {
			name := strings.TrimSpace(cmd.Name)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			desc := cmd.Description
			if desc == "" {
				desc = "marlin.ui.v1 command"
			}
			all = append(all, minibuffer.Candidate{
				ID:   "grpc-" + name,
				Name: name,
				Desc: desc,
				Run: func() {
					if f.Adaptive.Desktop.OnCommandInvoke != nil {
						f.Adaptive.Desktop.OnCommandInvoke(name)
					}
					f.Echo("Executed M-x " + name)
				},
			})
		}
	}

	// Project live Emacs self-doc, buffer-lifecycle, active Mode-local commands,
	// and Minor Mode toggle commands straight into M-x with live Marginalia badges.
	builtinEmacs := []struct {
		name, desc, act string
	}{
		{"find-file", "Open real file via Vertico directory browser (C-x C-f / SPC f f)", "cmd:find-file"},
		{"project-find-file", "Fuzzy-find any real file in current Git/jj project (C-c p f / SPC p f)", "cmd:project-find-file"},
		{"project-switch-project", "Switch active Git/jj repository (C-c p p / SPC p p)", "cmd:project-switch"},
		{"consult-ripgrep", "Live Rust ripgrep across current project (SPC / · C-c /)", "cmd:consult-ripgrep"},
		{"consult-line", "Orderless line search & jump in active buffer (SPC l · C-c l)", "cmd:consult-line"},
		{"recentf-open-files", "Open recently visited real file (C-x C-r / SPC f r)", "cmd:recentf"},
		{"describe-mode", "Describe active buffer Major/Minor/Polymode & local keymap", "cmd:describe-mode"},
		{"describe-key", "Describe key or chord across active Emacs keymap precedence", "cmd:describe-key"},
		{"describe-function", "Describe command documentation and all bound keybindings", "cmd:describe-function"},
		{"describe-bindings", "List all active local and global keybindings in *Help*", "cmd:describe-bindings"},
		{"view-lossage", "Show last 60 keystrokes and commands in *Help* (C-h l)", "cmd:view-lossage"},
		{"embark-act", "Open contextual Embark actions for target at point (C-. / SPC .)", "cmd:embark-act"},
		{"repeat-last-command", "Repeat the last executed command (C-c .)", "cmd:repeat-last"},
		{"window-zoom-toggle", "Temporarily zoom / restore focused window (SPC w m / C-x w m)", "cmd:win-zoom-toggle"},
		{"window-transpose-layout", "Flip split orientation Vertical <-> Horizontal (SPC w T / C-x w t)", "cmd:win-transpose"},
		{"rename-buffer", "Rename active buffer across workspace and windows (C-x x r)", "cmd:rename-buffer"},
		{"bury-buffer", "Bury current buffer to tail of workspace ring (SPC b z / C-c z)", "cmd:bury-buffer"},
		{"ibuffer", "Open *Ibuffer* interactive buffer manager grouped by Major Mode", "cmd:ibuffer"},
		{"kill-buffer", "Kill active buffer in the current workspace", "cmd:kill-buffer"},
		{"previous-buffer", "Switch focused window to previous buffer in workspace", "cmd:prev-buffer"},
		{"next-buffer", "Switch focused window to next buffer in workspace", "cmd:next-buffer"},
		{"new-chat-buffer", "Create a new multi-instance LLM conversation (*chat:*)", "cmd:new-chat-buffer"},
		{"clone-window", "Split window right viewing the same buffer (C-x 3)", "cmd:clone-window"},
		{"switch-to-buffer-other-window", "Switch buffer in other window (C-x 4 b / SPC b B)", "cmd:switch-buffer-other-window"},
		{"kill-buffer-and-window", "Kill active buffer and close its split window (C-x 4 0)", "cmd:kill-buffer-and-window"},
		{"window-configuration-to-register", "Save current split layout to a named register (C-x r w)", "cmd:register-save"},
		{"jump-to-register", "Restore a saved window configuration register (C-x r j)", "cmd:register-jump"},
		{"list-registers", "List all saved Emacs window registers in *Help* (C-x r l)", "cmd:register-list"},
	}
	for _, bc := range builtinEmacs {
		if _, dup := seen[bc.name]; dup {
			continue
		}
		seen[bc.name] = struct{}{}
		act := bc.act
		all = append(all, minibuffer.Candidate{
			ID:         "emacs-" + bc.name,
			Name:       bc.name,
			Desc:       bc.desc,
			Annotation: shortcuts[act],
			Run:        func() { f.dispatchWidgetAction(act) },
		})
	}
	activeBuf := f.ActiveBuffer()
	activeState := f.BufferModeState(activeBuf)
	activeMinorSet := make(map[string]bool, len(activeState.MinorModes))
	for _, mid := range activeState.MinorModes {
		activeMinorSet[mid] = true
	}
	for _, mm := range f.ModeRegistry().AllMinorModes() {
		if _, dup := seen[mm.ID]; dup {
			continue
		}
		seen[mm.ID] = struct{}{}
		mid := mm.ID
		stateBadge := "[off]"
		if activeMinorSet[mid] {
			stateBadge = "[ON]"
		}
		all = append(all, minibuffer.Candidate{
			ID:         "minor-" + mid,
			Name:       mid,
			Desc:       fmt.Sprintf("Toggle minor mode %s (%s) on %s", mm.Title, mm.Lighter, activeBuf),
			Annotation: stateBadge,
			Run:        func() { f.ToggleMinorModeOnActive(mid) },
		})
	}
	hi := f.BuildHelpIntrospection(activeBuf)
	for _, lb := range hi.LocalBindings {
		keyStr := lb[1]
		label := lb[2]
		cmdName := label
		if idx := strings.LastIndex(label, "("); idx >= 0 && strings.HasSuffix(label, ")") {
			cmdName = strings.TrimSpace(label[idx+1 : len(label)-1])
		}
		if cmdName == "" {
			continue
		}
		if _, dup := seen[cmdName]; dup {
			continue
		}
		seen[cmdName] = struct{}{}
		act := cmdName
		all = append(all, minibuffer.Candidate{
			ID:         "local-" + cmdName,
			Name:       cmdName,
			Desc:       fmt.Sprintf("[%s · %s] %s", lb[0], hi.MajorTitle, label),
			Annotation: keyStr,
			Run:        func() { f.dispatchWidgetAction(act) },
		})
	}
	// Wrap candidate Run hooks to maintain Emacs savehist-style MRU history
	// and float recently executed commands to the top of Vertico M-x.
	f.mu.Lock()
	recent := append([]string(nil), f.recentCommands...)
	f.mu.Unlock()
	recentRank := make(map[string]int, len(recent))
	for i, name := range recent {
		recentRank[name] = i + 1
	}
	var mru, rest []minibuffer.Candidate
	for i := range all {
		cand := all[i]
		origRun := cand.Run
		cmdName := cand.Name
		cand.Run = func() {
			f.recordRecentCommand(cmdName)
			if origRun != nil {
				origRun()
			}
		}
		if recentRank[cand.Name] > 0 {
			if cand.Annotation == "" {
				cand.Annotation = "★ recent"
			} else {
				cand.Annotation = "★ " + cand.Annotation
			}
			mru = append(mru, cand)
		} else {
			rest = append(rest, cand)
		}
	}
	if len(mru) > 1 {
		for i := 1; i < len(mru); i++ {
			for j := i; j > 0 && recentRank[mru[j].Name] < recentRank[mru[j-1].Name]; j-- {
				mru[j], mru[j-1] = mru[j-1], mru[j]
			}
		}
	}
	ordered := make([]minibuffer.Candidate, 0, len(all))
	ordered = append(ordered, mru...)
	ordered = append(ordered, rest...)
	return minibuffer.FilterCandidates(ordered, query)
}

// describeFunctionCandidates projects all M-x commands for Emacs `C-h f`
// (`describe-function`) so selecting any candidate opens its documentation card in *Help*.
func (f *Frame) describeFunctionCandidates(query string) []minibuffer.Candidate {
	base := f.commandCandidates(query)
	out := make([]minibuffer.Candidate, 0, len(base))
	for _, cand := range base {
		c := cand
		out = append(out, minibuffer.Candidate{
			ID:         "doc-" + c.ID,
			Name:       c.Name,
			Desc:       c.Desc,
			Annotation: c.Annotation,
			Run:        func() { f.DescribeFunction(c.Name) },
		})
	}
	return out
}

func (f *Frame) bufferCandidates(query string) []minibuffer.Candidate {
	cat := buffer.Catalog()
	wsBufs := make(map[buffer.ID]bool)
	winNums := make(map[buffer.ID]int)
	wsName := "main"
	if f.wsMgr != nil {
		wsSnap := f.wsMgr.Snapshot()
		if wsSnap.Active.Name != "" {
			wsName = wsSnap.Active.Name
		}
		for _, b := range wsSnap.Active.Buffers {
			wsBufs[b] = true
		}
		if leaves := wsSnap.Active.Windows(); len(leaves) > 1 {
			for _, lf := range leaves {
				if _, exists := winNums[lf.Buffer]; !exists {
					winNums[lf.Buffer] = lf.Number
				}
			}
		}
	}
	winBadge := func(id buffer.ID) string {
		if n := winNums[id]; n > 0 {
			return fmt.Sprintf("win #%d", n)
		}
		return ""
	}

	var inWS, popups, other []minibuffer.Candidate
	seenBuf := make(map[buffer.ID]bool, len(cat)+len(wsBufs))
	for _, b := range cat {
		bid := b.ID
		seenBuf[bid] = true
		desc := b.Description
		modeTitle := f.EffectiveModeStack(bid).DisplayTitle
		if wsBufs[bid] {
			inWS = append(inWS, minibuffer.Candidate{
				ID:         string(bid),
				Name:       string(bid),
				Desc:       "[" + wsName + " · " + modeTitle + "] " + desc,
				Annotation: winBadge(bid),
				Run:        func() { f.SelectBuffer(bid) },
			})
			continue
		}
		if f.wsMgr != nil {
			if rule, ok := f.wsMgr.MatchPopupRule(bid); ok {
				popups = append(popups, minibuffer.Candidate{
					ID:         string(bid),
					Name:       string(bid),
					Desc:       "[popup:" + string(rule.Side) + " · " + modeTitle + "] " + desc,
					Annotation: "C-x p",
					Run:        func() { f.SelectBuffer(bid) },
				})
				continue
			}
		}
		other = append(other, minibuffer.Candidate{
			ID:         string(bid),
			Name:       string(bid),
			Desc:       "[" + modeTitle + "] " + desc,
			Annotation: winBadge(bid),
			Run:        func() { f.SelectBuffer(bid) },
		})
	}
	// Include dynamic multi-instance buffers open in the active workspace (e.g. *chat:auth-fix*).
	for bid := range wsBufs {
		if seenBuf[bid] || strings.HasPrefix(string(bid), "feature:") || strings.HasPrefix(string(bid), uiframework.SchemeExtBufferPrefix) {
			continue
		}
		id := bid
		modeTitle := f.EffectiveModeStack(id).DisplayTitle
		inWS = append(inWS, minibuffer.Candidate{
			ID:         string(id),
			Name:       string(id),
			Desc:       "[" + wsName + " · " + modeTitle + "] Active buffer instance",
			Annotation: winBadge(id),
			Run:        func() { f.SelectBuffer(id) },
		})
	}

	// Mode-driven buffer & window creation actions (powers Tab Bar "+" & SPC b b).
	createCands := []minibuffer.Candidate{
		{
			ID:         "action:new-chat-buffer",
			Name:       "+ New LLM Chat (*chat:*)",
			Desc:       "[Marlin-Chat · Polymode] Open a fresh multi-instance LLM conversation buffer",
			Annotation: "C-c C-n",
			Run:        func() { f.NewChatBuffer("") },
		},
		{
			ID:         "action:clone-window",
			Name:       "+ Split / Clone Current Window",
			Desc:       "[Emacs Window] Create a linked sibling window viewing the current buffer",
			Annotation: "C-x 3 · SPC w v",
			Run:        func() { f.CloneCurrentWindow() },
		},
	}
	for _, rule := range f.ModeRegistry().CreatableBufferRules() {
		if rule.CreateAction == "" || rule.CreateAction == "cmd:new-chat-buffer" {
			continue
		}
		act := rule.CreateAction
		lbl := rule.CreateLabel
		if lbl == "" {
			lbl = rule.Prefix
		}
		createCands = append(createCands, minibuffer.Candidate{
			ID:   "create:" + rule.Prefix,
			Name: "+ " + lbl,
			Desc: "[" + rule.MajorMode + "] Open or switch in current window",
			Run:  func() { f.dispatchWidgetAction(act) },
		})
	}

	// Include live Scheme UIExtensionRegion buffers right after active workspace
	// buffers so SPC b b surfaces active Scheme extensions immediately without
	// scrolling past [all] or [popup] buffers.
	f.mu.Lock()
	schemeExts := append([]schema.UIExtensionRegion(nil), f.schemeExts...)
	recentPaths := append([]string(nil), f.recentFiles...)
	f.mu.Unlock()
	extCands := make([]minibuffer.Candidate, 0, len(schemeExts))
	for _, ext := range schemeExts {
		eid := ext.ID
		if strings.TrimSpace(eid) == "" {
			continue
		}
		lbl := strings.TrimSpace(ext.Label)
		if lbl == "" {
			lbl = "*" + eid + "*"
		}
		bufID := buffer.ID(uiframework.SchemeExtBufferPrefix + eid)
		extCands = append(extCands, minibuffer.Candidate{
			ID:   string(bufID),
			Name: lbl,
			Desc: "[scheme-ext] Live marlin.ui.v1 Scheme extension buffer",
			Run:  func() { f.SelectFeature(string(bufID)) },
		})
	}
	// Unified Doom Emacs `consult-buffer` virtual source (`SPC ,` / `C-x b`):
	// surface recently visited real files right alongside open buffers.
	recentCands := make([]minibuffer.Candidate, 0, min(len(recentPaths), 12))
	for i, p := range recentPaths {
		if i >= 12 {
			break
		}
		abs := p
		base := abs
		if idx := strings.LastIndex(abs, "/"); idx >= 0 && idx+1 < len(abs) {
			base = abs[idx+1:]
		}
		recentCands = append(recentCands, minibuffer.Candidate{
			ID:         "consult-rf-" + abs,
			Name:       base,
			Desc:       "[recentf] " + abs,
			Annotation: "recentf",
			Run:        func() { f.OpenDiskFile(abs) },
		})
	}

	all := make([]minibuffer.Candidate, 0, len(cat)+len(extCands)+len(createCands)+len(recentCands))
	all = append(all, inWS...)
	all = append(all, extCands...)
	all = append(all, recentCands...)
	all = append(all, createCands...)
	all = append(all, popups...)
	all = append(all, other...)
	return minibuffer.FilterCandidates(all, query)
}

func (f *Frame) themeCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	cat := append([]theme.Theme(nil), f.themeCatalog...)
	f.mu.Unlock()
	if len(cat) == 0 {
		cat = theme.Catalog()
	}
	all := make([]minibuffer.Candidate, 0, len(cat))
	for _, tp := range cat {
		p := tp
		desc := "Dark CSS theme pack"
		if p.IsLight {
			desc = "Light CSS theme pack"
		}
		all = append(all, minibuffer.Candidate{
			ID:   p.ID,
			Name: p.Name,
			Desc: desc,
			Run:  func() { f.SetTheme(p.ID) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}
