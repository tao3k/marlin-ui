// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// MarlinGatewayTurnRequest mirrors marlin-cove's MarlinGatewayTurnRequest JSON
// envelope dispatched to Marlin Gateway (`marlin-agent-core`).
type MarlinGatewayTurnRequest struct {
	SessionID         string                 `json:"session_id"`
	Governance        CoveGovernanceEnvelope `json:"governance"`
	TargetAgentHandle string                 `json:"target_agent_handle,omitempty"`
	Message           string                 `json:"message"`
}

// MarlinGatewayTurnResponse mirrors marlin-cove's MarlinGatewayTurnResponse.
type MarlinGatewayTurnResponse struct {
	Response string `json:"response"`
}

// NewMarlinGatewayHTTPHandler builds an authenticated HTTP `/message` handler
// that bridges Marlin Cove's verified RoomGovernanceEnvelope directly to a
// Marlin Agent Core responder (e.g. gRPC TurnStreamController).
func NewMarlinGatewayHTTPHandler(
	bearerToken string,
	responder func(ctx context.Context, req MarlinGatewayTurnRequest) (string, error),
) http.Handler {
	mux := http.NewServeMux()
	expected := strings.TrimSpace(bearerToken)
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if expected != "" {
			got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		var req MarlinGatewayTurnRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
			http.Error(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		if responder == nil {
			http.Error(w, "gateway responder unavailable", http.StatusServiceUnavailable)
			return
		}
		reply, err := responder(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MarlinGatewayTurnResponse{Response: reply})
	})
	return mux
}

// CoveRelayBridge binds marlin-ui's Agent Room directly to a live Marlin Cove
// Cloud Relay using structured SSE streaming (`async.StructuredScope`).
type CoveRelayBridge struct {
	client *CoveClient
	roomID string
	actor  string
	scope  *async.StructuredScope

	mu              sync.RWMutex
	snapshot        CoveRoomSnapshot
	activeChannelID string
	activeThreadID  string
	onDirty         func()
}

// AttachCoveRelayBridge connects marlin-ui's Agent Room to a live marlin-cove
// server when cfg.CoveURL is configured, returning nil otherwise.
func AttachCoveRelayBridge(ctx context.Context, cfg appconfig.Config, onDirty func()) *CoveRelayBridge {
	client := NewCoveClientFromConfig(cfg)
	if client == nil {
		return nil
	}
	roomID := strings.TrimSpace(cfg.CoveRoomID)
	if roomID == "" {
		roomID = "cove-main"
	}
	bridge := &CoveRelayBridge{
		client:          client,
		roomID:          roomID,
		actor:           "human:owner",
		scope:           async.NewStructuredScope(ctx),
		activeChannelID: "general",
		onDirty:         onDirty,
	}

	if snap, err := client.FetchSnapshot(ctx, roomID); err == nil {
		bridge.setSnapshot(snap)
	}

	featurekit.SetRoomBridge(featurekit.RoomBridge{
		Snapshot:      bridge.projectedSnapshot,
		SelectChannel: bridge.selectChannel,
		PostMessage:   bridge.postMessage,
		OpenThread:    bridge.openThread,
		CloseThread:   bridge.closeThread,
	})

	bridge.scope.Go(func(runCtx context.Context) {
		_ = client.WatchEvents(runCtx, roomID, func(evt CoveRoomEvent) {
			bridge.applyEvent(evt)
		})
	})

	return bridge
}

// Close cancels and drains the structured SSE stream subscription cleanly.
func (b *CoveRelayBridge) Close() {
	if b == nil || b.scope == nil {
		return
	}
	b.scope.CloseAndDrain()
}

func (b *CoveRelayBridge) setSnapshot(snap CoveRoomSnapshot) {
	b.mu.Lock()
	b.snapshot = snap
	if b.activeChannelID == "" && len(snap.Channels) > 0 {
		b.activeChannelID = snap.Channels[0].ID
	}
	b.mu.Unlock()
	if b.onDirty != nil {
		b.onDirty()
	}
}

func (b *CoveRelayBridge) applyEvent(evt CoveRoomEvent) {
	b.mu.Lock()
	if evt.Seq > b.snapshot.LatestSeq {
		b.snapshot.LatestSeq = evt.Seq
	}
	switch evt.Kind.Type {
	case "message_posted":
		if evt.Kind.Message != nil {
			b.snapshot.Messages = append(b.snapshot.Messages, *evt.Kind.Message)
		}
	case "participant_joined":
		if evt.Kind.Participant != nil {
			b.snapshot.Participants = append(b.snapshot.Participants, *evt.Kind.Participant)
		}
	case "gate_requested":
		if evt.Kind.Gate != nil {
			b.snapshot.Gates = append(b.snapshot.Gates, *evt.Kind.Gate)
		}
	case "gate_resolved":
		if evt.Kind.Receipt != nil {
			for i := range b.snapshot.Gates {
				if b.snapshot.Gates[i].ID == evt.Kind.Receipt.GateID {
					b.snapshot.Gates[i].Decision = evt.Kind.Receipt.Decision
				}
			}
		}
	}
	b.mu.Unlock()
	if b.onDirty != nil {
		b.onDirty()
	}
}

func (b *CoveRelayBridge) projectedSnapshot() featurekit.RoomFullSnapshot {
	b.mu.RLock()
	snap := b.snapshot
	ch := b.activeChannelID
	threadID := b.activeThreadID
	actor := b.actor
	b.mu.RUnlock()
	out := snap.ToRoomFullSnapshot(ch, actor)
	out.ActiveThreadID = threadID
	return out
}

func (b *CoveRelayBridge) selectChannel(channelID string) error {
	b.mu.Lock()
	b.activeChannelID = channelID
	b.activeThreadID = ""
	b.mu.Unlock()
	if b.onDirty != nil {
		b.onDirty()
	}
	return nil
}

func (b *CoveRelayBridge) postMessage(channelID, body, threadRootID string) error {
	ch := strings.TrimSpace(channelID)
	if ch == "" {
		b.mu.RLock()
		ch = b.activeChannelID
		b.mu.RUnlock()
	}
	if handle := firstAgentMention(body); handle != "" && strings.TrimSpace(threadRootID) == "" {
		_, err := b.client.DispatchTurn(context.Background(), b.roomID, ch, b.actor, handle, body)
		return err
	}
	_, err := b.client.PostMessage(context.Background(), b.roomID, ch, b.actor, body, threadRootID)
	return err
}

func (b *CoveRelayBridge) openThread(msgID string) error {
	b.mu.Lock()
	b.activeThreadID = msgID
	b.mu.Unlock()
	if b.onDirty != nil {
		b.onDirty()
	}
	return nil
}

func (b *CoveRelayBridge) closeThread() {
	b.mu.Lock()
	b.activeThreadID = ""
	b.mu.Unlock()
	if b.onDirty != nil {
		b.onDirty()
	}
}

func firstAgentMention(body string) string {
	for _, field := range strings.Fields(body) {
		if !strings.HasPrefix(field, "@") {
			continue
		}
		handle := strings.ToLower(strings.TrimRight(strings.TrimPrefix(field, "@"), ".,!?:;"))
		switch handle {
		case "architect", "reviewer", "security":
			return handle
		}
	}
	return ""
}
