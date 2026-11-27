// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	appv1 "github.com/tao3k/marlin-mygo/gen/marlin/app/v1"
	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// MarlinGatewayTurnRequest projects an inbound `marlin.app.v1.AppServer/StreamTurn`
// call arriving from `marlin-cove`, including its verified `x-marlin-cove-*`
// governance metadata headers.
type MarlinGatewayTurnRequest struct {
	SessionID         string
	Governance        CoveGovernanceEnvelope
	TargetAgentHandle string
	Message           string
}

type marlinAppServerGRPCAdapter struct {
	appv1.UnimplementedAppServerServer
	expectedToken string
	responder     func(ctx context.Context, req MarlinGatewayTurnRequest) (string, error)
}

func (s *marlinAppServerGRPCAdapter) StreamTurn(
	req *appv1.StreamTurnRequest,
	stream grpc.ServerStreamingServer[appv1.TurnEvent],
) error {
	ctx := stream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	if s.expectedToken != "" {
		authVals := md.Get("authorization")
		if len(authVals) == 0 {
			return status.Error(codes.Unauthenticated, "missing bearer token")
		}
		got := strings.TrimSpace(strings.TrimPrefix(authVals[0], "Bearer "))
		if subtle.ConstantTimeCompare([]byte(s.expectedToken), []byte(got)) != 1 {
			return status.Error(codes.Unauthenticated, "invalid bearer token")
		}
	}
	if s.responder == nil {
		return status.Error(codes.Unavailable, "gateway responder unavailable")
	}

	var userMessage string
	for _, msg := range req.GetMessages() {
		if msg.GetRole() == appv1.MessageRole_MESSAGE_ROLE_USER {
			userMessage = msg.GetText()
		}
	}

	turnReq := MarlinGatewayTurnRequest{
		SessionID:         firstMeta(md, "x-marlin-cove-session-id"),
		TargetAgentHandle: firstMeta(md, "x-marlin-cove-target-agent"),
		Message:           userMessage,
		Governance:        governanceFromMetadata(md),
	}

	reply, err := s.responder(ctx, turnReq)
	if err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}

	if err := stream.Send(&appv1.TurnEvent{
		Event: &appv1.TurnEvent_Started{Started: &appv1.TurnStarted{}},
	}); err != nil {
		return err
	}
	if err := stream.Send(&appv1.TurnEvent{
		Event: &appv1.TurnEvent_TextDelta{
			TextDelta: &appv1.TextDelta{Sequence: 1, Text: reply},
		},
	}); err != nil {
		return err
	}
	return stream.Send(&appv1.TurnEvent{
		Event: &appv1.TurnEvent_Completed{
			Completed: &appv1.TurnCompleted{
				TextChunks: 1,
				TextBytes:  uint64(len(reply)),
			},
		},
	})
}

func firstMeta(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}

func governanceFromMetadata(md metadata.MD) CoveGovernanceEnvelope {
	var scopes []string
	if raw := firstMeta(md, "x-marlin-cove-granted-scopes"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
	}
	return CoveGovernanceEnvelope{
		RoomID:          firstMeta(md, "x-marlin-cove-room-id"),
		ChannelID:       firstMeta(md, "x-marlin-cove-channel-id"),
		ActorID:         firstMeta(md, "x-marlin-cove-actor-id"),
		ActorKind:       firstMeta(md, "x-marlin-cove-actor-kind"),
		ActorRole:       firstMeta(md, "x-marlin-cove-actor-role"),
		GrantedScopes:   scopes,
		Origin:          firstMeta(md, "x-marlin-cove-origin"),
		TaintedExternal: strings.EqualFold(firstMeta(md, "x-marlin-cove-tainted-external"), "true"),
	}
}

// RegisterMarlinGatewayGRPCServer mounts an authenticated native
// `marlin.app.v1.AppServer` (`StreamTurn`) gRPC HTTP/2 handler onto server,
// extracting Cove's verified `x-marlin-cove-*` governance metadata without
// requiring `marlin-agent-core` to depend on any `marlin-cove` protocol.
func RegisterMarlinGatewayGRPCServer(
	server *grpc.Server,
	bearerToken string,
	responder func(ctx context.Context, req MarlinGatewayTurnRequest) (string, error),
) {
	appv1.RegisterAppServerServer(server, &marlinAppServerGRPCAdapter{
		expectedToken: strings.TrimSpace(bearerToken),
		responder:     responder,
	})
}

// CoveRelayBridge binds marlin-ui's Agent Room directly to a live Marlin Cove
// Cloud Relay over gRPC HTTP/2 server-streaming (`async.StructuredScope`).
type CoveRelayBridge struct {
	client *CoveClient
	roomID string
	actor  string
	scope  *async.StructuredScope

	mu              sync.RWMutex
	snapshot        CoveRoomSnapshot
	activeChannelID string
	activeThreadID  string
	thinkingBots    map[string]string
	onDirty         func()
}

// AttachCoveRelayBridge connects marlin-ui's Agent Room to a live marlin-cove
// gRPC HTTP/2 server when cfg.CoveURL is configured, returning nil otherwise.
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
		thinkingBots:    make(map[string]string),
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
		for {
			if runCtx.Err() != nil {
				return
			}
			if snap, err := client.FetchSnapshot(runCtx, roomID); err == nil {
				bridge.setSnapshot(snap)
			}
			_ = client.WatchEvents(runCtx, roomID, func(evt CoveRoomEvent) {
				bridge.applyEvent(evt)
			})
			select {
			case <-runCtx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	})

	return bridge
}

// Close cancels and drains the gRPC HTTP/2 event stream and closes the client connection.
func (b *CoveRelayBridge) Close() {
	if b == nil {
		return
	}
	if b.scope != nil {
		b.scope.CloseAndDrain()
	}
	if b.client != nil {
		_ = b.client.Close()
	}
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

func (b *CoveRelayBridge) upsertMessageLocked(msg CoveMessage) {
	if msg.ID == "" {
		return
	}
	for i := range b.snapshot.Messages {
		if b.snapshot.Messages[i].ID == msg.ID {
			b.snapshot.Messages[i] = msg
			return
		}
	}
	b.snapshot.Messages = append(b.snapshot.Messages, msg)
}

func (b *CoveRelayBridge) applyEvent(evt CoveRoomEvent) {
	b.mu.Lock()
	if evt.Seq > b.snapshot.LatestSeq {
		b.snapshot.LatestSeq = evt.Seq
	}
	switch evt.Kind.Type {
	case "message_posted":
		if evt.Kind.Message != nil {
			b.upsertMessageLocked(*evt.Kind.Message)
		}
	case "participant_joined":
		if evt.Kind.Participant != nil {
			replaced := false
			for i := range b.snapshot.Participants {
				if b.snapshot.Participants[i].ID == evt.Kind.Participant.ID {
					b.snapshot.Participants[i] = *evt.Kind.Participant
					replaced = true
					break
				}
			}
			if !replaced {
				b.snapshot.Participants = append(b.snapshot.Participants, *evt.Kind.Participant)
			}
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
	thinkingCopy := make(map[string]string, len(b.thinkingBots))
	for k, v := range b.thinkingBots {
		thinkingCopy[k] = v
	}
	b.mu.RUnlock()
	return snap.ToRoomFullSnapshotWithThread(ch, threadID, actor, thinkingCopy)
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

func (b *CoveRelayBridge) resolveTargetAgent(channelID, body string) (botID string, handle string) {
	b.mu.RLock()
	participants := b.snapshot.Participants
	b.mu.RUnlock()

	if strings.HasPrefix(channelID, "dm:") {
		wantID := strings.TrimPrefix(channelID, "dm:")
		for _, p := range participants {
			if p.Kind == "agent" && p.ID == wantID {
				return p.ID, strings.TrimPrefix(strings.ToLower(p.Handle), "@")
			}
		}
	}
	if mentioned := firstAgentMention(body, participants); mentioned != "" {
		for _, p := range participants {
			if p.Kind == "agent" && strings.TrimPrefix(strings.ToLower(p.Handle), "@") == mentioned {
				return p.ID, mentioned
			}
		}
		return "agent:" + mentioned, mentioned
	}
	return "", ""
}

func (b *CoveRelayBridge) postMessage(channelID, body, threadRootID string) error {
	txt := strings.TrimSpace(body)
	if txt == "" {
		return nil
	}
	ch := strings.TrimSpace(channelID)
	if ch == "" {
		b.mu.RLock()
		ch = b.activeChannelID
		b.mu.RUnlock()
	}
	if ch == "" {
		ch = "general"
	}

	botID, handle := b.resolveTargetAgent(ch, txt)
	if handle != "" {
		b.mu.Lock()
		b.thinkingBots[botID] = "thinking"
		b.mu.Unlock()
		if b.onDirty != nil {
			b.onDirty()
		}
		b.scope.Go(func(runCtx context.Context) {
			rpcCtx, cancel := context.WithTimeout(runCtx, 120*time.Second)
			defer cancel()
			resp, err := b.client.DispatchTurnInThread(rpcCtx, b.roomID, ch, b.actor, handle, txt, threadRootID)
			b.mu.Lock()
			delete(b.thinkingBots, botID)
			if err == nil {
				b.upsertMessageLocked(resp.UserMessage)
				b.upsertMessageLocked(resp.AgentReply)
			} else {
				b.upsertMessageLocked(CoveMessage{
					ID:            fmt.Sprintf("local-err-%d", time.Now().UnixNano()),
					RoomID:        b.roomID,
					ChannelID:     ch,
					ThreadRootID:  threadRootID,
					AuthorID:      botID,
					AuthorKind:    "agent",
					Body:          fmt.Sprintf("_@%s could not complete turn:_ `%v`", handle, err),
					Origin:        "marlin_gateway",
					CreatedAtUnix: time.Now().Unix(),
				})
			}
			b.mu.Unlock()
			if b.onDirty != nil {
				b.onDirty()
			}
		})
		return nil
	}

	b.scope.Go(func(runCtx context.Context) {
		rpcCtx, cancel := context.WithTimeout(runCtx, 15*time.Second)
		defer cancel()
		if msg, err := b.client.PostMessage(rpcCtx, b.roomID, ch, b.actor, txt, threadRootID); err == nil {
			b.mu.Lock()
			b.upsertMessageLocked(msg)
			b.mu.Unlock()
			if b.onDirty != nil {
				b.onDirty()
			}
		}
	})
	return nil
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

func firstAgentMention(body string, participants []CoveParticipant) string {
	known := map[string]bool{
		"architect": true,
		"reviewer":  true,
		"security":  true,
	}
	for _, p := range participants {
		if p.Kind == "agent" {
			if h := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(p.Handle)), "@"); h != "" {
				known[h] = true
			}
		}
	}
	for _, field := range strings.Fields(body) {
		if !strings.HasPrefix(field, "@") {
			continue
		}
		handle := strings.ToLower(strings.TrimRight(strings.TrimPrefix(field, "@"), ".,!?:;"))
		if known[handle] {
			return handle
		}
	}
	return ""
}
