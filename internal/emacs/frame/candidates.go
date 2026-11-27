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
	prefOrder := []string{"SPC", "SPC w", "SPC b", "SPC TAB", "SPC t", "SPC m", "SPC q", "C-x", "C-x t", "C-x w", "C-c", "C-h"}
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
	return minibuffer.FilterCandidates(all, query)
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

	all := make([]minibuffer.Candidate, 0, len(cat)+len(extCands)+len(createCands))
	all = append(all, inWS...)
	all = append(all, extCands...)
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
