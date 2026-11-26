// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"testing"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestMatchRunsSubstringAndFuzzyHighlight(t *testing.T) {
	// Exact case-insensitive substring match: "ta" in "Targets" -> [Ta][rgets].
	runs := theme.MatchRuns("Targets", "ta")
	if len(runs) != 2 || !runs[0].Matched || runs[0].Text != "Ta" || runs[1].Matched || runs[1].Text != "rgets" {
		t.Fatalf("unexpected substring runs: %+v", runs)
	}

	// Fuzzy subsequence match: "ta" in "AI Transcript" -> highlights 'T' and 'a'.
	fuzzy := theme.MatchRuns("AI Transcript", "ta")
	matchedChars := ""
	for _, r := range fuzzy {
		if r.Matched {
			matchedChars += r.Text
		}
	}
	if matchedChars != "Ta" {
		t.Fatalf("expected fuzzy matched chars 'Ta', got %q (runs=%+v)", matchedChars, fuzzy)
	}

	// Orderless multi-token match: "sw th" in "switch-themes" -> highlights "sw" and "th".
	ord := theme.MatchRuns("switch-themes", "sw th")
	matchedOrd := ""
	for _, r := range ord {
		if r.Matched {
			matchedOrd += r.Text
		}
	}
	if matchedOrd != "swth" {
		t.Fatalf("expected orderless matched chars 'swth', got %q (runs=%+v)", matchedOrd, ord)
	}
}
