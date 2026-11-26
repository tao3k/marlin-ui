// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"strings"
	"unicode"
)

// TextRun represents a contiguous slice of text that either matched the active
// search query (Orderless substring or fuzzy subsequence) or did not.
type TextRun struct {
	Text    string
	Matched bool
}

// MatchRuns splits text into minimal contiguous matched/unmatched runs against
// query (supporting Orderless space-separated tokens, substring preference,
// and fuzzy subsequence fallback matching Helm/Vertico).
func MatchRuns(text, query string) []TextRun {
	if text == "" {
		return nil
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []TextRun{{Text: text, Matched: false}}
	}

	runes := []rune(text)
	lower := make([]rune, len(runes))
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}
	hits := make([]bool, len(runes))
	anyHit := false

	for _, rawTok := range strings.Fields(trimmed) {
		if strings.HasPrefix(rawTok, "!") {
			continue // Orderless negation token does not highlight positive hits.
		}
		tok := []rune(strings.ToLower(rawTok))
		if len(tok) == 0 {
			continue
		}
		if markSubstring(lower, tok, hits) {
			anyHit = true
			continue
		}
		if markFuzzySubsequence(lower, tok, hits) {
			anyHit = true
		}
	}

	if !anyHit {
		return []TextRun{{Text: text, Matched: false}}
	}

	runs := make([]TextRun, 0, 8)
	start := 0
	cur := hits[0]
	for i := 1; i < len(runes); i++ {
		if hits[i] != cur {
			runs = append(runs, TextRun{Text: string(runes[start:i]), Matched: cur})
			start = i
			cur = hits[i]
		}
	}
	runs = append(runs, TextRun{Text: string(runes[start:]), Matched: cur})
	return runs
}

func markSubstring(lower, tok []rune, hits []bool) bool {
	if len(tok) > len(lower) {
		return false
	}
	matched := false
	for i := 0; i <= len(lower)-len(tok); i++ {
		ok := true
		for j := 0; j < len(tok); j++ {
			if lower[i+j] != tok[j] {
				ok = false
				break
			}
		}
		if ok {
			for j := 0; j < len(tok); j++ {
				hits[i+j] = true
			}
			matched = true
			break // Highlight first strongest substring occurrence cleanly.
		}
	}
	return matched
}

func markFuzzySubsequence(lower, tok []rune, hits []bool) bool {
	pos := make([]int, 0, len(tok))
	ti := 0
	for i := 0; i < len(lower) && ti < len(tok); i++ {
		if lower[i] == tok[ti] {
			pos = append(pos, i)
			ti++
		}
	}
	if ti != len(tok) {
		return false
	}
	for _, idx := range pos {
		hits[idx] = true
	}
	return true
}
