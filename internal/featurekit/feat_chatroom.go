// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && (marlin_feat_chatroom || marlin_full || (!marlin_feat_compare && !marlin_feat_canvas))

package featurekit

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/input"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/resilience"
	"github.com/tao3k/marlin-mygo/engine/shell"
	"github.com/tao3k/marlin-mygo/features/chatroom"
)

func init() {
	mod := &bentoChatRoomModule{
		inner:     chatroom.NewModule(nil),
		activeTab: chatroom.TabTimeline,
	}
	SetRoomStatsProvider(mod.liveStats)
	SetRoomTaskBridge(mod.listTasks, mod.addTask, mod.cycleTask)
	RegisterCompiled(mod)
}

type bentoChatRoomModule struct {
	inner       *chatroom.Module
	activeTab   chatroom.ViewTab
	draft       string
	threadDraft string
	itemDraft   string
}

func (m *bentoChatRoomModule) Meta() features.FeatureMeta { return m.inner.Meta() }

func (m *bentoChatRoomModule) SetGovernanceLedger(l *features.GovernanceLedger) {
	m.inner.SetGovernanceLedger(l)
}

func (m *bentoChatRoomModule) SetResilience(r *resilience.SessionResilience, fn func(string, string)) {
	m.inner.SetResilience(r, fn)
}

func (m *bentoChatRoomModule) SetPromptGate(q *promptgate.Queue, fn func(string)) {
	m.inner.SetPromptGate(q, fn)
}

func (m *bentoChatRoomModule) ApplyGateDecision(req promptgate.PromptRequest, resp promptgate.PromptResponse) {
	m.inner.ApplyGateDecision(req, resp)
}

func (m *bentoChatRoomModule) Register(host *features.Host) error {
	host.AddBuffer(shell.PaneBuffer{
		BufferID: chatroom.BufferIDChatRoom,
		Title:    "Agent Room",
		Kind:     "native-gpu",
	}, func(surface any, th any) {
		c, ok1 := surface.(*ui.Context)
		t, ok2 := th.(*ui.Theme)
		if ok1 && ok2 {
			m.renderBentoAgentRoom(c, t)
		}
	})
	host.AddCommand(input.CommandEntry{
		Name:        chatroom.CommandOpenChatRoom,
		Description: "Open Multi-Role Agent Room (Bento matte channels, matters, roles)",
	})
	return nil
}

func (m *bentoChatRoomModule) liveStats() RoomStats {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return RoomStats{}
	}
	snap := m.inner.View.Ctrl.Snapshot()
	var st RoomStats
	st.TasksTotal = len(snap.Items)
	for _, it := range snap.Items {
		if it.Status == chatroom.ItemDone {
			st.TasksDone++
		}
	}
	for _, p := range snap.Participants {
		if p.Kind == chatroom.ParticipantAgent {
			st.AgentsTotal++
			if p.Presence != chatroom.PresenceOffline {
				st.AgentsOnline++
			}
		}
	}
	return st
}

func (m *bentoChatRoomModule) listTasks() []RoomTask {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	snap := m.inner.View.Ctrl.Snapshot()
	owners := make(map[string]string, len(snap.Participants))
	for _, p := range snap.Participants {
		owners[p.ID] = p.Handle
	}
	out := make([]RoomTask, 0, len(snap.Items))
	for _, it := range snap.Items {
		own := owners[it.OwnerID]
		if own == "" {
			own = it.OwnerID
		}
		out = append(out, RoomTask{
			ID:     it.ID,
			Title:  it.Title,
			Status: string(it.Status),
			Owner:  own,
		})
	}
	return out
}

func (m *bentoChatRoomModule) addTask(title string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	_, err := ctrl.CreateItem(snap.ActiveChannelID, snap.CurrentActorID, title, snap.CurrentActorID)
	return err
}

func (m *bentoChatRoomModule) cycleTask(id string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	for _, it := range snap.Items {
		if it.ID != id {
			continue
		}
		next := chatroom.ItemInProgress
		switch it.Status {
		case chatroom.ItemInProgress:
			next = chatroom.ItemDone
		case chatroom.ItemDone:
			next = chatroom.ItemOpen
		case chatroom.ItemBlocked:
			next = chatroom.ItemInProgress
		}
		return ctrl.TransitionItem(id, snap.CurrentActorID, next)
	}
	return nil
}
