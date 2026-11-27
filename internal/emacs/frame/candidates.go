// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"strings"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

// commandCandidates projects the declarative TOML command catalog
// (defaults/commands.toml + ~/.config/marlin/ui/commands.toml) merged with
// live marlin.ui.v1 gRPC commands — zero hardcoded Go command tables.
func (f *Frame) commandCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	declCmds := append([]declarativeCommand(nil), f.tomlCommands...)
	schemeCmds := append([]schema.UICommandSpec(nil), f.schemeCmds...)
	f.mu.Unlock()

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
			ID:   cmd.Name,
			Name: cmd.Name,
			Desc: cmd.Desc,
			Run:  func() { f.dispatchWidgetAction(cmd.Action) },
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
			ID:   "scheme-" + name,
			Name: name,
			Desc: desc,
			Run:  func() { f.dispatchWidgetAction(act) },
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
	wsName := "main"
	if f.wsMgr != nil {
		wsSnap := f.wsMgr.Snapshot()
		if wsSnap.Active.Name != "" {
			wsName = wsSnap.Active.Name
		}
		for _, b := range wsSnap.Active.Buffers {
			wsBufs[b] = true
		}
	}

	var inWS, popups, other []minibuffer.Candidate
	for _, b := range cat {
		bid := b.ID
		desc := b.Description
		if wsBufs[bid] {
			inWS = append(inWS, minibuffer.Candidate{
				ID:   string(bid),
				Name: string(bid),
				Desc: "[" + wsName + "] " + desc,
				Run:  func() { f.SelectBuffer(bid) },
			})
			continue
		}
		if f.wsMgr != nil {
			if rule, ok := f.wsMgr.MatchPopupRule(bid); ok {
				popups = append(popups, minibuffer.Candidate{
					ID:   string(bid),
					Name: string(bid),
					Desc: "[popup:" + string(rule.Side) + "] " + desc,
					Run:  func() { f.SelectBuffer(bid) },
				})
				continue
			}
		}
		other = append(other, minibuffer.Candidate{
			ID:   string(bid),
			Name: string(bid),
			Desc: "[all] " + desc,
			Run:  func() { f.SelectBuffer(bid) },
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

	all := make([]minibuffer.Candidate, 0, len(cat)+len(extCands))
	all = append(all, inWS...)
	all = append(all, extCands...)
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
