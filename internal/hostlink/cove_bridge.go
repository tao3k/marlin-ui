// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"crypto/subtle"
	"strings"
	"sync"

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
