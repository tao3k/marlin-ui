// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package featurekit_test

import (
	"sync/atomic"
	"testing"

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
