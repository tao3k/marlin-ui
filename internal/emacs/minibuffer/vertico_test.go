// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"fmt"
	"testing"
)

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

func TestHistoryRing(t *testing.T) {
	s := New()
	s.RecordHistory(ModeCommand, "describe-mode")
	s.RecordHistory(ModeCommand, "ibuffer")
	s.Open(ModeCommand)

	v1, ok := s.StepHistory(-1)
	if !ok || v1 != "ibuffer" {
		t.Fatalf("expected first M-p -> ibuffer, got %q (%v)", v1, ok)
	}
	v2, ok := s.StepHistory(-1)
	if !ok || v2 != "describe-mode" {
		t.Fatalf("expected second M-p -> describe-mode, got %q (%v)", v2, ok)
	}
	v3, ok := s.StepHistory(1)
	if !ok || v3 != "ibuffer" {
		t.Fatalf("expected M-n -> ibuffer, got %q (%v)", v3, ok)
	}
	v4, ok := s.StepHistory(1)
	if !ok || v4 != "" {
		t.Fatalf("expected M-n past end -> empty input, got %q (%v)", v4, ok)
	}
}

func makeBenchProjectCandidates(n int) []Candidate {
	items := make([]Candidate, n)
	dirs := []string{
		"crates/marlin-agent-app-server/src/ui_grpc/",
		"crates/marlin-gerbil-scheme/src/",
		"scheme/ui-meow/",
		"scheme/ui-magit/",
		"scheme/ui-modes/",
		".data/marlin-ui/internal/emacs/frame/",
		".data/marlin-ui/internal/emacs/minibuffer/",
		".data/marlin-ui/internal/hostlink/",
	}
	exts := []string{".rs", ".ss", ".go", ".toml", ".md"}
	for i := 0; i < n; i++ {
		dir := dirs[i%len(dirs)]
		ext := exts[i%len(exts)]
		rel := fmt.Sprintf("%smodule_%04d_handler%s", dir, i, ext)
		items[i] = Candidate{
			ID:         "pf-" + rel,
			Name:       rel,
			Desc:       "/Users/dev/marlin-agent-core/" + rel,
			Annotation: "4.2 KiB",
		}
	}
	return items
}

func BenchmarkFilterCandidates_4000Files_SingleToken(b *testing.B) {
	items := makeBenchProjectCandidates(4000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FilterCandidates(items, "magit")
	}
}

func BenchmarkFilterCandidates_4000Files_MultiTokenOrderless(b *testing.B) {
	items := makeBenchProjectCandidates(4000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FilterCandidates(items, "handler 0042 scheme !toml")
	}
}

func BenchmarkFilterCandidates_10000Files_Orderless(b *testing.B) {
	items := makeBenchProjectCandidates(10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FilterCandidates(items, "frame 0128 go")
	}
}
