// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"testing"

	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
)

func TestDecodeMagitGraphRecords(t *testing.T) {
	path := "crates/auth.rs"
	start := uint32(10)
	end := uint32(18)
	records := []*projectionv1.GraphRecordView{
		{
			RecordId:   "magit-head:r4",
			Kind:       "magit.head",
			Title:      "marlin-agent-core",
			Summary:    "feat(magit): live jj bridge",
			SourcePath: &path,
			Facets: map[string]string{
				"revision":          "4",
				"backend":           "jj-colocated",
				"branch":            "main",
				"head_sha":          "deadbeef",
				"change_id":         "kntqzsqt",
				"ahead":             "2",
				"behind":            "1",
				"aitia_evidence_id": "aitia:magit:test:kntqzsqt:r4",
				"high_risk_hunks":   "1",
				"total_hunks":       "1",
			},
		},
		{
			RecordId: "magit-receipt:aitia:magit:test",
			Kind:     "magit.receipt",
			Title:    "stage",
			Summary:  "Staged crates/auth.rs",
			Facets: map[string]string{
				"ok":                "true",
				"aitia_evidence_id": "aitia:magit:test:r4",
			},
		},
		{
			RecordId:   "magit-file:unstaged:crates/auth.rs",
			Kind:       "magit.file",
			Title:      "crates/auth.rs",
			Summary:    "Touches security/auth/exec boundary (+5/-1)",
			SourcePath: &path,
			Facets: map[string]string{
				"section":     "unstaged",
				"status_code": "M",
				"added":       "5",
				"removed":     "1",
				"risk":        "high",
				"concern":     "security-boundary",
			},
		},
		{
			RecordId:   "unstaged:crates/auth.rs:h1:L10",
			Kind:       "magit.hunk",
			Title:      "@@ -10,2 +10,6 @@ fn check",
			Summary:    "+validate_token(req)?;",
			SourcePath: &path,
			StartLine:  &start,
			EndLine:    &end,
			Facets: map[string]string{
				"section":     "unstaged",
				"added":       "5",
				"removed":     "1",
				"risk":        "high",
				"concern":     "security-boundary",
				"risk_reason": "Touches security/auth/exec boundary (+5/-1)",
			},
		},
		{
			RecordId: "magit-log:0:deadbeef",
			Kind:     "magit.log",
			Title:    "feat(magit): live jj bridge",
			Facets: map[string]string{
				"commit_sha":      "deadbeef",
				"change_id":       "kntqzsqt",
				"bookmarks":       "main,origin/main",
				"author":          "marlin",
				"relative_time":   "2m ago",
				"is_working_copy": "true",
			},
		},
		{
			RecordId: "magit-op:0:op1234",
			Kind:     "magit.oplog",
			Title:    "describe commit",
			Summary:  "1m ago",
			Facets: map[string]string{
				"op_id":      "op1234",
				"is_current": "true",
			},
		},
	}

	snap := DecodeMagitGraphRecords(records)
	if snap.Backend != "jj-colocated" || snap.ChangeID != "kntqzsqt" || snap.Branch != "main" {
		t.Fatalf("unexpected head decode: %+v", snap)
	}
	if snap.AitiaEvidenceID != "aitia:magit:test:kntqzsqt:r4" {
		t.Fatalf("missing Aitia evidence ID: %q", snap.AitiaEvidenceID)
	}
	if snap.LastReceipt == nil || !snap.LastReceipt.OK || snap.LastReceipt.Action != "stage" {
		t.Fatalf("unexpected receipt decode: %+v", snap.LastReceipt)
	}
	if len(snap.Unstaged) != 1 || len(snap.Unstaged[0].Hunks) != 1 {
		t.Fatalf("expected 1 unstaged file with 1 hunk, got %+v", snap.Unstaged)
	}
	if snap.Unstaged[0].Hunks[0].Risk != "high" {
		t.Fatalf("expected high risk hunk, got %+v", snap.Unstaged[0].Hunks[0])
	}
	if len(snap.RecentLog) != 1 || len(snap.RecentLog[0].Bookmarks) != 2 {
		t.Fatalf("unexpected log decode: %+v", snap.RecentLog)
	}
	if len(snap.OpLog) != 1 || !snap.OpLog[0].IsCurrent {
		t.Fatalf("unexpected oplog decode: %+v", snap.OpLog)
	}
}
