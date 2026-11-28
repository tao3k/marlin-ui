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
				if it.KeepOpen {
					s.SetInput("")
					s.RequestFocus()
				} else {
					s.Close()
				}
			}
		}
	})
}

type parsedToken struct {
	text    string
	negated bool
	ascii   bool
}

func isAllASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func asciiLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func hasPrefixFoldASCII(s, lowerASCII string) bool {
	if len(s) < len(lowerASCII) {
		return false
	}
	for i := 0; i < len(lowerASCII); i++ {
		if asciiLower(s[i]) != lowerASCII[i] {
			return false
		}
	}
	return true
}

func containsFoldASCII(s, lowerASCII string) bool {
	n := len(lowerASCII)
	if n == 0 {
		return true
	}
	if len(s) < n {
		return false
	}
	c0 := lowerASCII[0]
	c0Upper := c0
	if c0 >= 'a' && c0 <= 'z' {
		c0Upper = c0 - ('a' - 'A')
	}
	// Fast SIMD path when the leading byte is case-invariant (digits, '/', '.', '-', '_').
	if c0 == c0Upper {
		for off := 0; off <= len(s)-n; {
			idx := strings.IndexByte(s[off:], c0)
			if idx < 0 {
				return false
			}
			pos := off + idx
			if pos > len(s)-n {
				return false
			}
			j := 1
			for j < n && asciiLower(s[pos+j]) == lowerASCII[j] {
				j++
			}
			if j == n {
				return true
			}
			off = pos + 1
		}
		return false
	}
	limit := len(s) - n
	for i := 0; i <= limit; i++ {
		b := s[i]
		if b != c0 && b != c0Upper {
			continue
		}
		j := 1
		for j < n && asciiLower(s[i+j]) == lowerASCII[j] {
			j++
		}
		if j == n {
			return true
		}
	}
	return false
}

func fieldContains(field string, tok parsedToken) bool {
	if field == "" {
		return false
	}
	if tok.ascii {
		return containsFoldASCII(field, tok.text)
	}
	return strings.Contains(strings.ToLower(field), tok.text)
}

func fieldHasPrefix(field string, tok parsedToken) bool {
	if field == "" {
		return false
	}
	if tok.ascii {
		return hasPrefixFoldASCII(field, tok.text)
	}
	return strings.HasPrefix(strings.ToLower(field), tok.text)
}

// FilterCandidates performs high-speed, zero-allocation (on ASCII queries) Emacs
// Orderless multi-token matching across name, description, and Marginalia annotations:
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
	rawToks := strings.Fields(q)
	if len(rawToks) == 0 {
		return all
	}
	var tokBuf [8]parsedToken
	tokens := tokBuf[:0]
	for _, raw := range rawToks {
		if strings.HasPrefix(raw, "!") && len(raw) > 1 {
			sub := raw[1:]
			tokens = append(tokens, parsedToken{text: sub, negated: true, ascii: isAllASCII(sub)})
		} else {
			tokens = append(tokens, parsedToken{text: raw, negated: false, ascii: isAllASCII(raw)})
		}
	}
	qASCII := isAllASCII(q)

	var tier0, tier1, tier2 []Candidate
	for i := range all {
		it := &all[i]
		matched := true
		allInName := true
		hasPrefix := false
		for _, tok := range tokens {
			if tok.negated {
				if fieldContains(it.Name, tok) || fieldContains(it.Desc, tok) || fieldContains(it.Annotation, tok) {
					matched = false
					break
				}
				continue
			}
			if fieldContains(it.Name, tok) {
				if fieldHasPrefix(it.Name, tok) {
					hasPrefix = true
				}
				continue
			}
			if fieldContains(it.Desc, tok) || fieldContains(it.Annotation, tok) {
				allInName = false
				continue
			}
			matched = false
			break
		}
		if !matched {
			continue
		}
		exactName := false
		if len(it.Name) == len(q) {
			if qASCII {
				exactName = hasPrefixFoldASCII(it.Name, q)
			} else {
				exactName = strings.ToLower(it.Name) == q
			}
		}
		switch {
		case exactName || (allInName && hasPrefix):
			tier0 = append(tier0, *it)
		case allInName:
			tier1 = append(tier1, *it)
		default:
			tier2 = append(tier2, *it)
		}
	}
	if len(tier1) == 0 && len(tier2) == 0 {
		return tier0
	}
	if len(tier0) == 0 && len(tier2) == 0 {
		return tier1
	}
	if len(tier0) == 0 && len(tier1) == 0 {
		return tier2
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
