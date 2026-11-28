// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import "testing"

func TestFilterCandidatesOrderless(t *testing.T) {
	items := []Candidate{
		{ID: "1", Name: "magit-status", Desc: "Open Magit git status workbench", Annotation: "SPC g g · C-x g"},
		{ID: "2", Name: "magit-ai-audit", Desc: "Open 50/50 AI + Aitia code audit", Annotation: "SPC g a · C-c g"},
		{ID: "3", Name: "describe-mode", Desc: "Describe active Major/Minor/Polymode", Annotation: "SPC h m · C-h m"},
		{ID: "4", Name: "kill-buffer", Desc: "Kill current buffer in workspace", Annotation: "SPC b d · C-x k"},
	}

	// Out-of-order multi-token match across name + desc.
	got := FilterCandidates(items, "aitia magit")
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("expected orderless match for 'aitia magit' -> id 2, got %+v", got)
	}

	// Negation token (!audit) excludes matching candidates.
	gotNeg := FilterCandidates(items, "magit !audit")
	if len(gotNeg) != 1 || gotNeg[0].ID != "1" {
		t.Fatalf("expected negated orderless match 'magit !audit' -> id 1, got %+v", gotNeg)
	}

	// Ranking puts prefix Name match before Desc-only match.
	ranked := FilterCandidates(items, "kill")
	if len(ranked) != 1 || ranked[0].ID != "4" {
		t.Fatalf("expected 'kill' -> kill-buffer, got %+v", ranked)
	}
}
