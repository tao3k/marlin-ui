// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderVertico(c *ui.Context, s *State, th theme.Theme, items []Candidate) {
	s.mu.Lock()
	sel := s.selected
	s.mu.Unlock()
	if sel >= len(items) && len(items) > 0 {
		sel = len(items) - 1
	}
	limit := min(len(items), 10)
	ui.Column(c.Key("vertico-list")).Background(th.Bar).Padding(6, 8).Gap(2).
		BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		for i := 0; i < limit; i++ {
			it := items[i]
			if theme.VerticoRow(c, "vrow-"+it.ID, it.Name, it.Desc, 240, i == sel, th).Clicked() {
				if it.Run != nil {
					it.Run()
				}
				s.Close()
			}
		}
	})
}

// FilterCandidates performs case-insensitive substring matching across name and description.
func FilterCandidates(all []Candidate, query string) []Candidate {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all
	}
	var out []Candidate
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.Name+" "+it.Desc), q) {
			out = append(out, it)
		}
	}
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
