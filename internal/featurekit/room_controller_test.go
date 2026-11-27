// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package featurekit_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/featurekit"
)

type stubRemoteRoomController struct {
	snap featurekit.RoomFullSnapshot
}

func (s *stubRemoteRoomController) Snapshot() featurekit.RoomFullSnapshot        { return s.snap }
func (s *stubRemoteRoomController) Stats() featurekit.RoomStats                  { return featurekit.RoomStats{AgentsOnline: len(s.snap.Participants)} }
func (s *stubRemoteRoomController) Tasks() []featurekit.RoomTask                 { return s.snap.Tasks }
func (s *stubRemoteRoomController) SelectChannel(string) error                   { return nil }
func (s *stubRemoteRoomController) PostMessage(string, string, string) error     { return nil }
func (s *stubRemoteRoomController) PostGateEmbed(string, string, string, string) error { return nil }
func (s *stubRemoteRoomController) ToggleReaction(string, string) error          { return nil }
func (s *stubRemoteRoomController) OpenThread(string) error                      { return nil }
func (s *stubRemoteRoomController) CloseThread()                                 {}
func (s *stubRemoteRoomController) AddTask(string) error                         { return nil }
func (s *stubRemoteRoomController) CycleTask(string) error                       { return nil }

func TestRoomControllerZeroBlankScreenFallbackAndSeamlessPromotion(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	_, _, _ = featurekit.Mount("all", sh, nil, nil)

	var notifications atomic.Int32
	unsub := featurekit.SubscribeRoomChanged(func() {
		notifications.Add(1)
	})
	defer unsub()

	// 1. Baseline local core room has real channels and online AI agents.
	localSnap := featurekit.AgentRoomSnapshot()
	if !localSnap.Available || len(localSnap.Channels) == 0 || len(localSnap.Participants) == 0 {
		t.Fatalf("expected hydrated local room, got available=%v channels=%d participants=%d",
			localSnap.Available, len(localSnap.Channels), len(localSnap.Participants))
	}
	if localSnap.SyncMode != "local" {
		t.Fatalf("expected SyncMode=local, got %q", localSnap.SyncMode)
	}

	// 2. Registering an unhydrated remote Cove controller MUST NOT blank out the room.
	remote := &stubRemoteRoomController{
		snap: featurekit.RoomFullSnapshot{
			Available: false,
			SyncMode:  "cove-connecting",
			SyncLabel: "Cove Connecting…",
		},
	}
	featurekit.SetRemoteRoomController(remote)
	defer featurekit.ClearRemoteRoomController(remote)

	fallbackSnap := featurekit.AgentRoomSnapshot()
	if !fallbackSnap.Available || len(fallbackSnap.Channels) == 0 || len(fallbackSnap.Participants) == 0 {
		t.Fatalf("zero blank-screen invariant violated: unhydrated remote hid local channels/agents: %+v", fallbackSnap)
	}
	if fallbackSnap.SyncMode != "cove-connecting" || fallbackSnap.SyncLabel != "Cove Connecting…" {
		t.Fatalf("expected non-destructive connecting badge while falling back to local, got mode=%q label=%q",
			fallbackSnap.SyncMode, fallbackSnap.SyncLabel)
	}

	// 3. Once remote hydrates with channels & participants, it seamlessly promotes.
	remote.snap = featurekit.RoomFullSnapshot{
		Available:       true,
		SyncMode:        "cove-grpc",
		SyncLabel:       "Cove Live · gRPC",
		ActiveChannelID: "general",
		Channels: []featurekit.RoomChannelView{
			{ID: "general", Name: "general"},
		},
		Participants: []featurekit.RoomParticipantView{
			{ID: "agent:architect", Handle: "architect", Name: "Architect", IsAgent: true, Presence: "online"},
		},
	}
	featurekit.NotifyRoomChanged()

	liveSnap := featurekit.AgentRoomSnapshot()
	if liveSnap.SyncMode != "cove-grpc" || liveSnap.ActiveChannelID != "general" || len(liveSnap.Participants) != 1 {
		t.Fatalf("expected promoted Cove Live snapshot, got mode=%q ch=%q participants=%d",
			liveSnap.SyncMode, liveSnap.ActiveChannelID, len(liveSnap.Participants))
	}

	// 4. Clearing remote returns cleanly to local controller and triggers change notifications.
	featurekit.ClearRemoteRoomController(remote)
	restored := featurekit.AgentRoomSnapshot()
	if restored.SyncMode != "local" || len(restored.Participants) < 3 {
		t.Fatalf("expected clean fallback to local room after ClearRemoteRoomController, got mode=%q participants=%d",
			restored.SyncMode, len(restored.Participants))
	}
	if notifications.Load() < 3 {
		t.Fatalf("expected SubscribeRoomChanged notifications on lifecycle transitions, got %d", notifications.Load())
	}
}

func TestChannelMentionRepliesThreadUnderUserMessage(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	_, _, _ = featurekit.Mount("chatroom", sh, nil, nil)
	featurekit.ClearRemoteRoomController(nil)

	done := make(chan string, 1)
	featurekit.SetRoomAgentResponder(func(_ context.Context, req featurekit.RoomAgentRequest) (string, error) {
		done <- req.ThreadRootID
		return "Audit ready in thread.", nil
	})
	defer featurekit.SetRoomAgentResponder(nil)

	before := featurekit.AgentRoomSnapshot()
	if err := featurekit.PostAgentRoomMessage(before.ActiveChannelID, "@reviewer hello", ""); err != nil {
		t.Fatalf("PostAgentRoomMessage failed: %v", err)
	}

	var gotThreadRoot string
	select {
	case gotThreadRoot = <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for @reviewer responder")
	}
	if strings.TrimSpace(gotThreadRoot) == "" {
		t.Fatalf("expected @reviewer channel mention to set non-empty ThreadRootID on bot reply")
	}

	// Wait briefly for appendBotReply to commit and verify the reply is nested
	// under the user's message thread (ThreadCount == 1) rather than top-level.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := featurekit.AgentRoomSnapshot()
		for _, m := range snap.Messages {
			if m.ID == gotThreadRoot && m.ThreadCount >= 1 {
				if err := featurekit.OpenAgentRoomThread(m.ID); err != nil {
					t.Fatalf("OpenAgentRoomThread failed: %v", err)
				}
				tsnap := featurekit.AgentRoomSnapshot()
				foundReply := false
				for _, tm := range tsnap.ThreadMessages {
					if strings.Contains(tm.Body, "Audit ready in thread.") {
						foundReply = true
						break
					}
				}
				if !foundReply {
					t.Fatalf("expected Reviewer reply inside ThreadMessages, got %+v", tsnap.ThreadMessages)
				}
				for _, top := range tsnap.Messages {
					if strings.Contains(top.Body, "Audit ready in thread.") {
						t.Fatalf("Reviewer reply must NOT appear as top-level channel message")
					}
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected user message %q to receive threaded reply (ThreadCount >= 1)", gotThreadRoot)
}
