// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"testing"

	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
)

func TestDecodeModeRecords(t *testing.T) {
	in := []*projectionv1.GraphRecordView{
		nil,
		{
			RecordId: "mode.major:marlin-llm-chat-mode",
			Kind:     "ui.mode.major",
			Title:    "LLM Chat",
			Facets: map[string]string{
				"id":             "marlin-llm-chat-mode",
				"parent":         "marlin-special-mode",
				"display_action": "reuse-window",
			},
		},
		{
			RecordId: "mode.poly:poly-llm-chat-mode",
			Kind:     "ui.polymode",
			Title:    "Poly LLM Chat",
			Facets: map[string]string{
				"id":        "poly-llm-chat-mode",
				"host_mode": "marlin-llm-chat-mode",
			},
		},
	}

	got := DecodeModeRecords(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 decoded mode records, got %d", len(got))
	}
	if got[0].Kind != "ui.mode.major" || got[0].Facets["display_action"] != "reuse-window" {
		t.Fatalf("unexpected major mode record: %+v", got[0])
	}
	if got[1].Kind != "ui.polymode" || got[1].Facets["host_mode"] != "marlin-llm-chat-mode" {
		t.Fatalf("unexpected polymode record: %+v", got[1])
	}
}
