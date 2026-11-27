// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoveClientWireParityAndSnapshotProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/health":
			_ = json.NewEncoder(w).Encode(CoveHealth{
				Status:   "ok",
				Service:  "marlin-cove",
				Features: []string{"relay", "telegram", "discord"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rooms/cove-main/snapshot":
			_ = json.NewEncoder(w).Encode(CoveRoomSnapshot{
				RoomID:    "cove-main",
				Title:     "Marlin Cove Main",
				LatestSeq: 4,
				Channels: []CoveChannel{
					{ID: "general", Name: "general", Topic: "Shared room"},
					{ID: "architecture", Name: "architecture", Topic: "Design"},
				},
				Participants: []CoveParticipant{
					{ID: "human:owner", DisplayName: "Owner", Handle: "owner", Kind: "human", Role: "owner", Presence: "online"},
					{ID: "agent:architect", DisplayName: "Architect", Handle: "architect", Kind: "agent", Role: "role_agent", Presence: "thinking"},
					{ID: "tg:501", DisplayName: "@alice", Handle: "alice", Kind: "human", Role: "external_guest", Presence: "online"},
				},
				Messages: []CoveMessage{
					{
						ID:         "msg_1",
						RoomID:     "cove-main",
						ChannelID:  "general",
						AuthorID:   "tg:501",
						AuthorKind: "human",
						Body:       "@architect review Cove relay",
						Mentions:   []string{"architect"},
						Origin:     "telegram",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rooms/cove-main/governance/tg:501":
			_ = json.NewEncoder(w).Encode(CoveGovernanceEnvelope{
				RoomID:          "cove-main",
				ChannelID:       "general",
				ActorID:         "tg:501",
				ActorKind:       "human",
				ActorRole:       "external_guest",
				GrantedScopes:   []string{"public"},
				Origin:          "telegram",
				TaintedExternal: true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewCoveClient(srv.URL, "cove-test-token")
	ctx := context.Background()

	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if health.Service != "marlin-cove" || len(health.Features) != 3 {
		t.Fatalf("unexpected health response: %+v", health)
	}

	snap, err := client.FetchSnapshot(ctx, "cove-main")
	if err != nil {
		t.Fatalf("FetchSnapshot failed: %v", err)
	}
	projected := snap.ToRoomFullSnapshot("general", "human:owner")
	if !projected.Available || len(projected.Messages) != 1 {
		t.Fatalf("expected 1 projected message, got %+v", projected)
	}
	if !projected.Messages[0].TaintedExternal || projected.Messages[0].Delivery != "telegram" {
		t.Fatalf("expected telegram bridge message to carry TaintedExternal=true, got %+v", projected.Messages[0])
	}
	if len(projected.ThinkingBots) != 1 || projected.ThinkingBots[0] != "Architect" {
		t.Fatalf("expected Architect in ThinkingBots, got %+v", projected.ThinkingBots)
	}

	env, err := client.FetchGovernanceEnvelope(ctx, "cove-main", "tg:501")
	if err != nil {
		t.Fatalf("FetchGovernanceEnvelope failed: %v", err)
	}
	if !env.TaintedExternal || env.ActorRole != "external_guest" || env.Origin != "telegram" {
		t.Fatalf("unexpected governance envelope: %+v", env)
	}
}
