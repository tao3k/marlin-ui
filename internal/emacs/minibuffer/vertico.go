// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

const verticoDrawerHeight float32 = 304

func renderVertico(c *ui.Context, s *State, th theme.Theme, query string, items []Candidate, glass bool) {
	s.mu.Lock()
	sel := s.selected
	s.mu.Unlock()
	if sel >= len(items) && len(items) > 0 {
		sel = len(items) - 1
	}
	limit := min(len(items), 10)
	bg := th.Bar
	if glass && c.Vibrancy() {
		bg = theme.GlassTone(c, th.Bar, th, 0.32, 0.56)
	}
	// Fixed height (Doom Emacs vertico-resize = nil) so filtering M-x / SPC b b / SPC t t
	// never causes vertical viewport bounce as candidate count changes.
	ui.Column(c.Key("vertico-list")).Height(verticoDrawerHeight).Shrink(0).Clip().
		Background(bg).Padding(6, 8).Gap(2).
		BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.40, 0.50)).AlignItems(ui.Stretch).Children(func() {
		if limit == 0 {
			ui.Row(c).Padding(12, 14).Children(func() {
				ui.Text(c, "No matching candidates").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted)
			})
			return
		}
		for i := 0; i < limit; i++ {
			it := items[i]
			if theme.VerticoRowAnnotated(c, "vrow-"+it.ID, it.Name, it.Desc, it.Annotation, query, 240, i == sel, th).Clicked() {
				if it.Run != nil {
					it.Run()
				}
				s.Close()
			}
		}
	})
}

// FilterCandidates performs Emacs Orderless multi-token matching across name,
// description, and Marginalia keybinding annotations:
//   - Whitespace splits independent tokens that may appear in any order.
//   - Tokens prefixed with "!" negate (exclude matching candidates).
//   - Matches are ranked: exact/prefix Name matches first, then substring Name
//     matches, then Description/Annotation-only matches (preserving source order
//     within each tier).
func FilterCandidates(all []Candidate, query string) []Candidate {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all
	}
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return all
	}
	var tier0, tier1, tier2 []Candidate
	for _, it := range all {
		nameL := strings.ToLower(it.Name)
		haystack := nameL + " " + strings.ToLower(it.Desc) + " " + strings.ToLower(it.Annotation)
		matched := true
		allInName := true
		hasPrefix := false
		for _, tok := range tokens {
			if strings.HasPrefix(tok, "!") && len(tok) > 1 {
				neg := tok[1:]
				if strings.Contains(haystack, neg) {
					matched = false
					break
				}
				continue
			}
			if !strings.Contains(haystack, tok) {
				matched = false
				break
			}
			if strings.HasPrefix(nameL, tok) {
				hasPrefix = true
			}
			if !strings.Contains(nameL, tok) {
				allInName = false
			}
		}
		if !matched {
			continue
		}
		switch {
		case nameL == q || (allInName && hasPrefix):
			tier0 = append(tier0, it)
		case allInName:
			tier1 = append(tier1, it)
		default:
			tier2 = append(tier2, it)
		}
	}
	out := make([]Candidate, 0, len(tier0)+len(tier1)+len(tier2))
	out = append(out, tier0...)
	out = append(out, tier1...)
	out = append(out, tier2...)
	return out
}

func clampSel(sel, total int) int {
	if total == 0 {
		return 0
	}
	if sel < 0 {
		return 1
	}
	if sel >= total {
		return total
	}
	return sel + 1
}
