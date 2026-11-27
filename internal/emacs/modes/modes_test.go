// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package modes

import "testing"

func TestMajorModeInheritancePolymodeAndEmacsKeyLookupPrecedence(t *testing.T) {
	reg := DefaultRegistry()

	// 1. Major Mode parent inheritance chain
	chain := reg.ParentChain("magit-status-mode")
	want := []string{"magit-status-mode", "magit-mode", "marlin-special-mode", "fundamental-mode"}
	if len(chain) != len(want) {
		t.Fatalf("unexpected parent chain: got %v want %v", chain, want)
	}
	for i := range want {
		if chain[i] != want[i] {
			t.Fatalf("parent chain[%d]: got %q want %q", i, chain[i], want[i])
		}
	}

	// 2. Multi-instance *chat:<id>* buffer routing
	if got := reg.ResolveBufferMode("*chat:refactor-auth*"); got != "marlin-chat-mode" {
		t.Fatalf("expected *chat:refactor-auth* -> marlin-chat-mode, got %q", got)
	}

	// 3. Polymode span switching + hook firing ONLY on transitions
	var hookEvents []HookEvent
	reg.AddHook(HookPolymodeSwitchSpan, func(ev HookEvent) {
		hookEvents = append(hookEvents, ev)
	})
	st := reg.MakeBufferState("*chat:audit*")
	st = reg.SwitchSpan(st, SpanDiffHunk)
	st = reg.SwitchSpan(st, SpanDiffHunk) // duplicate span: must NOT fire hook again
	if len(hookEvents) != 1 || hookEvents[0].InnerMode != "magit-diff-inner-mode" {
		t.Fatalf("expected 1 polymode switch hook -> magit-diff-inner-mode, got %+v", hookEvents)
	}
	eff := reg.EffectiveStack(st)
	if eff.DisplayTitle != "Marlin-Chat[Diff·Aitia]" {
		t.Fatalf("expected Polymode display title Marlin-Chat[Diff·Aitia], got %q", eff.DisplayTitle)
	}

	// 4. Keymap lookup precedence: minor > polymode inner > major > parent
	if b, ok := reg.LookupKey(st, "s"); !ok || b.Command != "cmd:magit-stage" {
		t.Fatalf("expected Polymode diff-hunk inner mode to bind 's' -> cmd:magit-stage, got %+v %v", b, ok)
	}
	if b, ok := reg.LookupKey(st, "C-c g"); !ok || b.Command != "cmd:magit-ai-audit" {
		t.Fatalf("expected aitia-audit-minor-mode to bind 'C-c g', got %+v %v", b, ok)
	}
	st = reg.SwitchSpan(st, SpanProse)
	if _, ok := reg.LookupKey(st, "s"); ok {
		t.Fatalf("expected 's' to be unbound in chat prose span after leaving diff-hunk")
	}
	if b, ok := reg.LookupKey(st, "i"); !ok || b.Command != "cmd:insert" {
		t.Fatalf("expected host marlin-chat-mode to bind 'i' -> cmd:insert, got %+v %v", b, ok)
	}

	// 5. Magit inherits 'q' from marlin-special-mode parent while overriding 'g' in magit-mode
	magitSt := reg.MakeBufferState("*magit*")
	if b, ok := reg.LookupKey(magitSt, "q"); !ok || b.Command != "cmd:win-delete" {
		t.Fatalf("expected *magit* to inherit 'q' -> cmd:win-delete from marlin-special-mode, got %+v %v", b, ok)
	}
	if b, ok := reg.LookupKey(magitSt, "g"); !ok || b.Command != "cmd:magit-refresh" {
		t.Fatalf("expected *magit* to override 'g' -> cmd:magit-refresh, got %+v %v", b, ok)
	}
}

func TestRegistryAppliesLiveSchemeGRPCModeRecords(t *testing.T) {
	reg := DefaultRegistry()
	n := reg.ApplyModeRecords([]ModeRecord{
		{
			RecordID: "ui.mode.major:custom-sql-mode",
			Kind:     "ui.mode.major",
			Title:    "SQL-REPL",
			Facets: map[string]string{
				"id":             "custom-sql-mode",
				"parent":         "marlin-special-mode",
				"read_only":      "false",
				"display_action": "split-below",
				"keymap":         "e\tcmd:sql-exec\tRun SQL\ttrue",
			},
		},
		{
			RecordID: "ui.buffer_rule:*sql:",
			Kind:     "ui.buffer_rule",
			Title:    "SQL REPL",
			Facets: map[string]string{
				"prefix":        "*sql:",
				"major_mode":    "custom-sql-mode",
				"creatable":     "true",
				"create_label":  "New SQL REPL (*sql:*)",
				"create_action": "cmd:new-sql",
			},
		},
	})
	if n != 2 {
		t.Fatalf("expected 2 records applied, got %d", n)
	}
	st := reg.MakeBufferState("*sql:prod*")
	if st.MajorMode != "custom-sql-mode" {
		t.Fatalf("expected *sql:prod* -> custom-sql-mode, got %q", st.MajorMode)
	}
	if b, ok := reg.LookupKey(st, "e"); !ok || b.Command != "cmd:sql-exec" {
		t.Fatalf("expected live Scheme mode binding e -> cmd:sql-exec, got %+v %v", b, ok)
	}
}
