// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package featurekit

import (
	"strings"
	"sync"
)

// RoomController is the SINGLE authoritative Agent Room interface consumed
// identically by Desktop, iOS, and Android.
//
// Both the local in-process backend (bentoChatRoomModule + marlin-app-server)
// and the remote Marlin Cove gRPC HTTP/2 backend (hostlink.CoveRelayBridge)
// implement this exact contract so UI shells never duplicate state machines or
// lose capabilities when switching between local and cloud rooms.
type RoomController interface {
	Snapshot() RoomFullSnapshot
	Stats() RoomStats
	Tasks() []RoomTask
	SelectChannel(channelID string) error
	PostMessage(channelID, body, threadRootID string) error
	PostGateEmbed(channelID, botID, body, gateID string) error
	ToggleReaction(msgID, tag string) error
	OpenThread(msgID string) error
	CloseThread()
	AddTask(title string) error
	CycleTask(id string) error
}

var (
	roomCtrlMu     sync.RWMutex
	localRoomCtrl  RoomController
	remoteRoomCtrl RoomController
	roomListeners  map[uint64]func()
	roomListenerID uint64
)

// SetLocalRoomController registers the built-in local RoomController (installed
// by feat_chatroom.go when chatroom is compiled in).
func SetLocalRoomController(ctrl RoomController) {
	roomCtrlMu.Lock()
	localRoomCtrl = ctrl
	roomCtrlMu.Unlock()
	NotifyRoomChanged()
}

// SetRemoteRoomController registers a remote cloud RoomController (e.g. Marlin
// Cove gRPC HTTP/2). Until the remote controller delivers a hydrated snapshot
// with channels and participants, reads and writes automatically fall back to
// the local controller so the UI never renders a blank room.
func SetRemoteRoomController(ctrl RoomController) {
	roomCtrlMu.Lock()
	remoteRoomCtrl = ctrl
	roomCtrlMu.Unlock()
	NotifyRoomChanged()
}

// ClearRemoteRoomController detaches the active remote RoomController and
// returns control seamlessly to the local controller.
func ClearRemoteRoomController(ctrl RoomController) {
	roomCtrlMu.Lock()
	if ctrl == nil || remoteRoomCtrl == ctrl {
		remoteRoomCtrl = nil
	}
	roomCtrlMu.Unlock()
	NotifyRoomChanged()
}

// LocalRoomController returns the registered local controller (used by remote
// bridges to share local task/matter CRUD until cloud tasks are wired).
func LocalRoomController() RoomController {
	roomCtrlMu.RLock()
	defer roomCtrlMu.RUnlock()
	return localRoomCtrl
}

// SubscribeRoomChanged registers a UI invalidation listener notified whenever
// any RoomController state changes across Desktop, iOS, or Android.
func SubscribeRoomChanged(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	roomCtrlMu.Lock()
	if roomListeners == nil {
		roomListeners = make(map[uint64]func())
	}
	roomListenerID++
	id := roomListenerID
	roomListeners[id] = fn
	roomCtrlMu.Unlock()
	return func() {
		roomCtrlMu.Lock()
		delete(roomListeners, id)
		roomCtrlMu.Unlock()
	}
}

// NotifyRoomChanged wakes all subscribed UI shells (Desktop, iOS, Android) so
// channel switches, thread replies, reactions, and live gRPC stream events
// paint immediately at 120Hz.
func NotifyRoomChanged() {
	roomCtrlMu.RLock()
	listeners := make([]func(), 0, len(roomListeners))
	for _, fn := range roomListeners {
		listeners = append(listeners, fn)
	}
	roomCtrlMu.RUnlock()

	notifyRoomInvalidated()
	for _, fn := range listeners {
		fn()
	}
}

func resolveActiveRoomController() (active RoomController, remoteSyncMode, remoteSyncLabel string) {
	roomCtrlMu.RLock()
	local := localRoomCtrl
	remote := remoteRoomCtrl
	roomCtrlMu.RUnlock()

	if remote != nil {
		snap := remote.Snapshot()
		if snap.Available && len(snap.Channels) > 0 && len(snap.Participants) > 0 {
			return remote, snap.SyncMode, snap.SyncLabel
		}
		// Remote Cove is configured but not yet hydrated: keep local room live
		// and surface a non-destructive connecting badge instead of a blank screen.
		mode := strings.TrimSpace(snap.SyncMode)
		if mode == "" {
			mode = "cove-connecting"
		}
		label := strings.TrimSpace(snap.SyncLabel)
		if label == "" {
			label = "Cove Connecting…"
		}
		if local != nil {
			return local, mode, label
		}
		return remote, mode, label
	}
	return local, "local", "Local Core"
}

// AgentRoomSnapshot returns the unified Agent Room snapshot for Desktop, iOS,
// and Android (Available=false only under -tags marlin_minimal).
func AgentRoomSnapshot() RoomFullSnapshot {
	ctrl, syncMode, syncLabel := resolveActiveRoomController()
	if ctrl == nil {
		return RoomFullSnapshot{}
	}
	snap := ctrl.Snapshot()
	if snap.SyncMode == "" {
		snap.SyncMode = syncMode
	}
	if snap.SyncLabel == "" {
		snap.SyncLabel = syncLabel
	}
	// If falling back to local while remote Cove is still connecting, preserve
	// the connecting badge so operators see live status without losing agents.
	if syncMode == "cove-connecting" {
		snap.SyncMode = syncMode
		snap.SyncLabel = syncLabel
	}
	return snap
}

// AgentRoomStats returns live Agent Room task and agent telemetry from the
// active unified RoomController.
func AgentRoomStats() RoomStats {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return RoomStats{}
	}
	return ctrl.Stats()
}

// AgentRoomTasks returns live collaborative tasks from the active RoomController.
func AgentRoomTasks() []RoomTask {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	return ctrl.Tasks()
}

// SelectAgentRoomChannel switches the active Agent Room channel and repaints.
func SelectAgentRoomChannel(channelID string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.SelectChannel(channelID)
	NotifyRoomChanged()
	return err
}

// PostAgentRoomMessage sends a channel message or thread reply and repaints.
func PostAgentRoomMessage(channelID, body, threadRootID string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.PostMessage(channelID, body, threadRootID)
	NotifyRoomChanged()
	return err
}

// PostAgentRoomGateEmbed posts a bot message linked to a Tier-0 PromptGate.
func PostAgentRoomGateEmbed(channelID, botID, body, gateID string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.PostGateEmbed(channelID, botID, body, gateID)
	NotifyRoomChanged()
	return err
}

// ToggleAgentRoomReaction toggles a reaction tag on an Agent Room message.
func ToggleAgentRoomReaction(msgID, tag string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.ToggleReaction(msgID, tag)
	NotifyRoomChanged()
	return err
}

// OpenAgentRoomThread opens the thread reply drawer/sheet for msgID and repaints.
func OpenAgentRoomThread(msgID string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.OpenThread(msgID)
	NotifyRoomChanged()
	return err
}

// CloseAgentRoomThread closes the active thread reply drawer/sheet and repaints.
func CloseAgentRoomThread() {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return
	}
	ctrl.CloseThread()
	NotifyRoomChanged()
}

// AddAgentRoomTask creates a real collaborative task in the active room.
func AddAgentRoomTask(title string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.AddTask(title)
	NotifyRoomChanged()
	return err
}

// CycleAgentRoomTask advances a task's status (OPEN -> IN_PROGRESS -> DONE -> OPEN).
func CycleAgentRoomTask(id string) error {
	ctrl, _, _ := resolveActiveRoomController()
	if ctrl == nil {
		return nil
	}
	err := ctrl.CycleTask(id)
	NotifyRoomChanged()
	return err
}
