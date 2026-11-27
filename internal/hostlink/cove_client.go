// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	covev1 "github.com/tao3k/marlin-ui/gen/marlin/cove/v1"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// CoveHealth mirrors marlin.cove.v1.HealthResponse.
type CoveHealth struct {
	Status   string
	Service  string
	Features []string
}

// CoveChannel mirrors marlin.cove.v1.RoomChannel.
type CoveChannel struct {
	ID       string
	Name     string
	Topic    string
	IsDirect bool
}

// CoveParticipant mirrors marlin.cove.v1.RoomParticipant.
type CoveParticipant struct {
	ID          string
	DisplayName string
	Handle      string
	Kind        string // "human" | "agent"
	Role        string // "owner" | "maintainer" | "member" | "external_guest" | "role_agent"
	Scopes      []string
	Presence    string
}

// CoveMessage mirrors marlin.cove.v1.RoomMessage.
type CoveMessage struct {
	ID            string
	RoomID        string
	ChannelID     string
	ThreadRootID  string
	AuthorID      string
	AuthorKind    string
	Body          string
	Mentions      []string
	GateID        string
	Origin        string
	BotChainDepth uint8
	CreatedAtUnix int64
}

// CoveGate mirrors marlin.cove.v1.RoomGate.
type CoveGate struct {
	ID          string
	RoomID      string
	ChannelID   string
	RequestedBy string
	Summary     string
	Capability  string
	Decision    string
}

// CoveRoomSnapshot mirrors marlin.cove.v1.RoomSnapshot.
type CoveRoomSnapshot struct {
	RoomID       string
	Title        string
	LatestSeq    uint64
	Channels     []CoveChannel
	Participants []CoveParticipant
	Messages     []CoveMessage
	Gates        []CoveGate
}

// CoveInvite mirrors marlin.cove.v1.RoomInvite.
type CoveInvite struct {
	Token       string
	RoomID      string
	IssuedBy    string
	GrantRole   string
	GrantScopes []string
	MaxUses     uint32
	Uses        uint32
}

// CoveGovernanceEnvelope mirrors marlin.cove.v1.RoomGovernanceEnvelope
// forwarded to Marlin Gateway (`marlin-agent-core`) over gRPC HTTP/2.
type CoveGovernanceEnvelope struct {
	RoomID          string
	ChannelID       string
	ActorID         string
	ActorKind       string
	ActorRole       string
	GrantedScopes   []string
	Origin          string
	TaintedExternal bool
}

// CoveDispatchResponse mirrors marlin.cove.v1.DispatchTurnResponse.
type CoveDispatchResponse struct {
	UserMessage CoveMessage
	Governance  CoveGovernanceEnvelope
	AgentReply  CoveMessage
}

// CoveGateReceipt mirrors marlin.cove.v1.GateApprovalReceipt.
type CoveGateReceipt struct {
	GateID         string
	ResolvedBy     string
	Decision       string
	Reason         string
	ResolvedAtUnix int64
}

// CoveRoomEvent mirrors marlin.cove.v1.RoomEvent streamed over HTTP/2 gRPC.
type CoveRoomEvent struct {
	Seq    uint64
	RoomID string
	Kind   CoveRoomEventKind
}

// CoveRoomEventKind projects the RoomEvent oneof payload.
type CoveRoomEventKind struct {
	Type        string
	Message     *CoveMessage
	Participant *CoveParticipant
	Gate        *CoveGate
	Receipt     *CoveGateReceipt
}

// CoveClient speaks Protobuf v3 gRPC over HTTP/2 (`marlin.cove.v1.CoveRelayService`)
// to a Marlin Cove cloud relay server.
type CoveClient struct {
	target string
	token  string

	mu   sync.Mutex
	conn *grpc.ClientConn
	rpc  covev1.CoveRelayServiceClient
}

// NewCoveClient builds a Marlin Cove gRPC HTTP/2 client for the given target
// address (accepts `host:port`, `http://host:port`, or `grpc://host:port`).
func NewCoveClient(target, bearerToken string) *CoveClient {
	return &CoveClient{
		target: normalizeGRPCTarget(target),
		token:  strings.TrimSpace(bearerToken),
	}
}

// NewCoveClientFromConfig returns a configured CoveClient when MARLIN_COVE_URL
// is set, or nil when Marlin UI runs without an external Cove relay.
func NewCoveClientFromConfig(cfg appconfig.Config) *CoveClient {
	if strings.TrimSpace(cfg.CoveURL) == "" {
		return nil
	}
	return NewCoveClient(cfg.CoveURL, cfg.CoveToken)
}

// Close closes the underlying HTTP/2 gRPC client connection.
func (c *CoveClient) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.rpc = nil
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (c *CoveClient) client() (covev1.CoveRelayServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rpc != nil {
		return c.rpc, nil
	}
	if c.target == "" {
		return nil, fmt.Errorf("marlin-cove gRPC target is empty")
	}
	conn, err := grpc.NewClient(c.target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial marlin-cove gRPC %s: %w", c.target, err)
	}
	c.conn = conn
	c.rpc = covev1.NewCoveRelayServiceClient(conn)
	return c.rpc, nil
}

func (c *CoveClient) withAuth(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	header := c.token
	if !strings.HasPrefix(header, "Bearer ") {
		header = "Bearer " + header
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", header)
}

// Health invokes CoveRelayService/Health over HTTP/2.
func (c *CoveClient) Health(ctx context.Context) (CoveHealth, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveHealth{}, err
	}
	resp, err := rpc.Health(ctx, &covev1.HealthRequest{})
	if err != nil {
		return CoveHealth{}, err
	}
	status := "unknown"
	if resp.GetStatus() == covev1.ServiceHealthStatus_SERVICE_HEALTH_STATUS_OK {
		status = "ok"
	}
	return CoveHealth{
		Status:   status,
		Service:  resp.GetService(),
		Features: append([]string(nil), resp.GetFeatures()...),
	}, nil
}

// FetchSnapshot invokes CoveRelayService/GetRoomSnapshot over HTTP/2.
func (c *CoveClient) FetchSnapshot(ctx context.Context, roomID string) (CoveRoomSnapshot, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveRoomSnapshot{}, err
	}
	resp, err := rpc.GetRoomSnapshot(c.withAuth(ctx), &covev1.GetRoomSnapshotRequest{
		RoomId: roomID,
	})
	if err != nil {
		return CoveRoomSnapshot{}, err
	}
	return snapshotFromProto(resp), nil
}

// FetchGovernanceEnvelope invokes CoveRelayService/GetGovernanceEnvelope over HTTP/2.
func (c *CoveClient) FetchGovernanceEnvelope(ctx context.Context, roomID, principalID string) (CoveGovernanceEnvelope, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveGovernanceEnvelope{}, err
	}
	resp, err := rpc.GetGovernanceEnvelope(c.withAuth(ctx), &covev1.GetGovernanceEnvelopeRequest{
		RoomId:      roomID,
		PrincipalId: principalID,
	})
	if err != nil {
		return CoveGovernanceEnvelope{}, err
	}
	return governanceFromProto(resp), nil
}

// CreateInvite invokes CoveRelayService/CreateInvite over HTTP/2.
func (c *CoveClient) CreateInvite(ctx context.Context, roomID, issuedBy, grantRole string, scopes []string, maxUses uint32) (CoveInvite, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveInvite{}, err
	}
	resp, err := rpc.CreateInvite(c.withAuth(ctx), &covev1.CreateInviteRequest{
		RoomId:      roomID,
		IssuedBy:    issuedBy,
		GrantRole:   principalRoleToProto(grantRole),
		GrantScopes: scopes,
		MaxUses:     maxUses,
	})
	if err != nil {
		return CoveInvite{}, err
	}
	return inviteFromProto(resp), nil
}

// RedeemInvite invokes CoveRelayService/RedeemInvite over HTTP/2.
func (c *CoveClient) RedeemInvite(ctx context.Context, token, principalID, displayName, handle, kind string) (CoveParticipant, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveParticipant{}, err
	}
	resp, err := rpc.RedeemInvite(ctx, &covev1.RedeemInviteRequest{
		Token:       token,
		PrincipalId: principalID,
		DisplayName: displayName,
		Handle:      handle,
		Kind:        principalKindToProto(kind),
	})
	if err != nil {
		return CoveParticipant{}, err
	}
	return participantFromProto(resp), nil
}

// PostMessage invokes CoveRelayService/PostMessage over HTTP/2.
func (c *CoveClient) PostMessage(ctx context.Context, roomID, channelID, authorID, body, threadRootID string) (CoveMessage, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveMessage{}, err
	}
	resp, err := rpc.PostMessage(c.withAuth(ctx), &covev1.PostMessageRequest{
		RoomId:       roomID,
		ChannelId:    channelID,
		ThreadRootId: strings.TrimSpace(threadRootID),
		AuthorId:     authorID,
		Body:         body,
		Origin:       covev1.BridgeOrigin_BRIDGE_ORIGIN_COVE_NATIVE,
	})
	if err != nil {
		return CoveMessage{}, err
	}
	return messageFromProto(resp), nil
}

// DispatchTurn posts a user turn into Cove and dispatches it live over HTTP/2
// to marlin-agent-core (`marlin.app.v1.AppServer/StreamTurn`).
func (c *CoveClient) DispatchTurn(ctx context.Context, roomID, channelID, authorID, targetAgentHandle, body string) (CoveDispatchResponse, error) {
	return c.DispatchTurnInThread(ctx, roomID, channelID, authorID, targetAgentHandle, body, "")
}

// DispatchTurnInThread posts a user turn (optionally inside a thread) into Cove
// and dispatches it live over HTTP/2 to marlin-agent-core.
func (c *CoveClient) DispatchTurnInThread(ctx context.Context, roomID, channelID, authorID, targetAgentHandle, body, threadRootID string) (CoveDispatchResponse, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveDispatchResponse{}, err
	}
	resp, err := rpc.DispatchTurn(c.withAuth(ctx), &covev1.DispatchTurnRequest{
		RoomId:            roomID,
		ChannelId:         channelID,
		AuthorId:          authorID,
		TargetAgentHandle: targetAgentHandle,
		Body:              body,
		ThreadRootId:      strings.TrimSpace(threadRootID),
	})
	if err != nil {
		return CoveDispatchResponse{}, err
	}
	return CoveDispatchResponse{
		UserMessage: messageFromProto(resp.GetUserMessage()),
		Governance:  governanceFromProto(resp.GetGovernance()),
		AgentReply:  messageFromProto(resp.GetAgentReply()),
	}, nil
}

// RequestGate invokes CoveRelayService/RequestGate over HTTP/2.
func (c *CoveClient) RequestGate(ctx context.Context, roomID, channelID, requestedBy, summary, capability string) (CoveGate, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveGate{}, err
	}
	resp, err := rpc.RequestGate(c.withAuth(ctx), &covev1.RequestGateRequest{
		RoomId:      roomID,
		ChannelId:   channelID,
		RequestedBy: requestedBy,
		Summary:     summary,
		Capability:  capability,
	})
	if err != nil {
		return CoveGate{}, err
	}
	return gateFromProto(resp), nil
}

// ResolveGate invokes CoveRelayService/ResolveGate over HTTP/2 as a human Owner/Maintainer.
func (c *CoveClient) ResolveGate(ctx context.Context, roomID, gateID, actorID string, approved bool, reason string) (CoveGateReceipt, error) {
	rpc, err := c.client()
	if err != nil {
		return CoveGateReceipt{}, err
	}
	resp, err := rpc.ResolveGate(c.withAuth(ctx), &covev1.ResolveGateRequest{
		RoomId:   roomID,
		GateId:   gateID,
		ActorId:  actorID,
		Approved: approved,
		Reason:   reason,
	})
	if err != nil {
		return CoveGateReceipt{}, err
	}
	return receiptFromProto(resp), nil
}

// WatchEvents subscribes to CoveRelayService/WatchRoomEvents (native HTTP/2
// gRPC server-streaming) and invokes onEvent for every RoomEvent until ctx is
// canceled or the stream closes.
func (c *CoveClient) WatchEvents(ctx context.Context, roomID string, onEvent func(CoveRoomEvent)) error {
	rpc, err := c.client()
	if err != nil {
		return err
	}
	stream, err := rpc.WatchRoomEvents(c.withAuth(ctx), &covev1.WatchRoomEventsRequest{
		RoomId: roomID,
	})
	if err != nil {
		return err
	}
	for {
		evt, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if onEvent != nil {
			onEvent(eventFromProto(evt))
		}
	}
}

// ToRoomFullSnapshot projects a Cove relay snapshot into marlin-ui's native
// build-tag-free RoomFullSnapshot used across Desktop, iOS, and Android.
func (s CoveRoomSnapshot) ToRoomFullSnapshot(activeChannelID, currentActorID string) featurekit.RoomFullSnapshot {
	return s.ToRoomFullSnapshotWithThread(activeChannelID, "", currentActorID, nil)
}

// ToRoomFullSnapshotWithThread projects a Cove relay snapshot with active thread
// and live thinking-bot presence overrides into marlin-ui's RoomFullSnapshot.
func (s CoveRoomSnapshot) ToRoomFullSnapshotWithThread(
	activeChannelID, activeThreadID, currentActorID string,
	thinkingOverrides map[string]string,
) featurekit.RoomFullSnapshot {
	if activeChannelID == "" && len(s.Channels) > 0 {
		activeChannelID = s.Channels[0].ID
	}
	byID := make(map[string]CoveParticipant, len(s.Participants))
	participants := make([]featurekit.RoomParticipantView, 0, len(s.Participants))
	thinking := make([]string, 0)
	for _, p := range s.Participants {
		pres := p.Presence
		if ov, ok := thinkingOverrides[p.ID]; ok && ov != "" {
			pres = ov
		}
		p.Presence = pres
		byID[p.ID] = p
		isAgent := p.Kind == "agent"
		if isAgent && (pres == "thinking" || pres == "working") {
			thinking = append(thinking, p.DisplayName)
		}
		participants = append(participants, featurekit.RoomParticipantView{
			ID:              p.ID,
			Handle:          p.Handle,
			Name:            p.DisplayName,
			Role:            p.Role,
			IsAgent:         isAgent,
			HostNode:        "marlin-cove",
			Presence:        pres,
			CanPost:         true,
			CanManageItems:  !isAgent,
			CanInvokeTools:  isAgent,
			RequireGate:     isAgent,
			CanApproveGates: !isAgent && (p.Role == "owner" || p.Role == "maintainer"),
		})
	}

	channels := make([]featurekit.RoomChannelView, 0, len(s.Channels))
	for _, ch := range s.Channels {
		botID := ""
		if ch.IsDirect && strings.HasPrefix(ch.ID, "dm:") {
			botID = strings.TrimPrefix(ch.ID, "dm:")
		}
		channels = append(channels, featurekit.RoomChannelView{
			ID:               ch.ID,
			Name:             ch.Name,
			Topic:            ch.Topic,
			IsDM:             ch.IsDirect,
			BotParticipantID: botID,
		})
	}

	replyCounts := make(map[string]int)
	for _, m := range s.Messages {
		if m.ThreadRootID != "" {
			replyCounts[m.ThreadRootID]++
		}
	}

	toMsgView := func(m CoveMessage) featurekit.RoomMessageView {
		author := byID[m.AuthorID]
		authorName := author.DisplayName
		if authorName == "" {
			authorName = m.AuthorID
		}
		ts := ""
		if m.CreatedAtUnix > 0 {
			ts = time.Unix(m.CreatedAtUnix, 0).Local().Format("15:04")
		}
		return featurekit.RoomMessageView{
			ID:              m.ID,
			ChannelID:       m.ChannelID,
			AuthorID:        m.AuthorID,
			AuthorName:      authorName,
			AuthorHandle:    author.Handle,
			AuthorRole:      author.Role,
			AuthorPresence:  author.Presence,
			IsAgent:         m.AuthorKind == "agent",
			Body:            m.Body,
			Mentions:        m.Mentions,
			GateID:          m.GateID,
			TaintedExternal: m.Origin == "telegram" || m.Origin == "discord",
			Delivery:        m.Origin,
			Timestamp:       ts,
			CreatedUnix:     m.CreatedAtUnix,
			ThreadCount:     replyCounts[m.ID],
		}
	}

	messages := make([]featurekit.RoomMessageView, 0, len(s.Messages))
	threadMessages := make([]featurekit.RoomMessageView, 0)
	for _, m := range s.Messages {
		if activeChannelID != "" && m.ChannelID != activeChannelID {
			continue
		}
		view := toMsgView(m)
		if m.ThreadRootID == "" {
			messages = append(messages, view)
		} else if activeThreadID != "" && m.ThreadRootID == activeThreadID {
			threadMessages = append(threadMessages, view)
		}
	}

	return featurekit.RoomFullSnapshot{
		Available:       true,
		ActiveChannelID: activeChannelID,
		ActiveThreadID:  activeThreadID,
		CurrentActorID:  currentActorID,
		ThinkingBots:    thinking,
		Channels:        channels,
		Participants:    participants,
		Messages:        messages,
		ThreadMessages:  threadMessages,
		Tasks:           featurekit.AgentRoomTasks(),
	}
}

func normalizeGRPCTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	for _, prefix := range []string{"http://", "https://", "grpc://"} {
		trimmed = strings.TrimPrefix(trimmed, prefix)
	}
	return strings.TrimRight(trimmed, "/")
}

func principalKindToProto(kind string) covev1.PrincipalKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "human":
		return covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN
	case "agent":
		return covev1.PrincipalKind_PRINCIPAL_KIND_AGENT
	default:
		return covev1.PrincipalKind_PRINCIPAL_KIND_UNSPECIFIED
	}
}

func principalKindFromProto(kind covev1.PrincipalKind) string {
	switch kind {
	case covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN:
		return "human"
	case covev1.PrincipalKind_PRINCIPAL_KIND_AGENT:
		return "agent"
	default:
		return "unknown"
	}
}

func principalRoleToProto(role string) covev1.PrincipalRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner":
		return covev1.PrincipalRole_PRINCIPAL_ROLE_OWNER
	case "maintainer":
		return covev1.PrincipalRole_PRINCIPAL_ROLE_MAINTAINER
	case "member":
		return covev1.PrincipalRole_PRINCIPAL_ROLE_MEMBER
	case "external_guest":
		return covev1.PrincipalRole_PRINCIPAL_ROLE_EXTERNAL_GUEST
	case "role_agent":
		return covev1.PrincipalRole_PRINCIPAL_ROLE_ROLE_AGENT
	default:
		return covev1.PrincipalRole_PRINCIPAL_ROLE_UNSPECIFIED
	}
}

func principalRoleFromProto(role covev1.PrincipalRole) string {
	switch role {
	case covev1.PrincipalRole_PRINCIPAL_ROLE_OWNER:
		return "owner"
	case covev1.PrincipalRole_PRINCIPAL_ROLE_MAINTAINER:
		return "maintainer"
	case covev1.PrincipalRole_PRINCIPAL_ROLE_MEMBER:
		return "member"
	case covev1.PrincipalRole_PRINCIPAL_ROLE_EXTERNAL_GUEST:
		return "external_guest"
	case covev1.PrincipalRole_PRINCIPAL_ROLE_ROLE_AGENT:
		return "role_agent"
	default:
		return "unknown"
	}
}

func presenceFromProto(status covev1.PresenceStatus) string {
	switch status {
	case covev1.PresenceStatus_PRESENCE_STATUS_ONLINE:
		return "online"
	case covev1.PresenceStatus_PRESENCE_STATUS_THINKING:
		return "thinking"
	case covev1.PresenceStatus_PRESENCE_STATUS_WORKING:
		return "working"
	case covev1.PresenceStatus_PRESENCE_STATUS_IDLE:
		return "idle"
	case covev1.PresenceStatus_PRESENCE_STATUS_OFFLINE:
		return "offline"
	default:
		return "online"
	}
}

func bridgeOriginFromProto(origin covev1.BridgeOrigin) string {
	switch origin {
	case covev1.BridgeOrigin_BRIDGE_ORIGIN_COVE_NATIVE:
		return "cove_native"
	case covev1.BridgeOrigin_BRIDGE_ORIGIN_TELEGRAM:
		return "telegram"
	case covev1.BridgeOrigin_BRIDGE_ORIGIN_DISCORD:
		return "discord"
	case covev1.BridgeOrigin_BRIDGE_ORIGIN_MARLIN_GATEWAY:
		return "marlin_gateway"
	default:
		return "cove_native"
	}
}

func gateDecisionFromProto(decision covev1.GateDecision) string {
	switch decision {
	case covev1.GateDecision_GATE_DECISION_PENDING:
		return "pending"
	case covev1.GateDecision_GATE_DECISION_APPROVED:
		return "approved"
	case covev1.GateDecision_GATE_DECISION_DENIED:
		return "denied"
	default:
		return "pending"
	}
}

func governanceFromProto(pb *covev1.RoomGovernanceEnvelope) CoveGovernanceEnvelope {
	if pb == nil {
		return CoveGovernanceEnvelope{}
	}
	return CoveGovernanceEnvelope{
		RoomID:          pb.GetRoomId(),
		ChannelID:       pb.GetChannelId(),
		ActorID:         pb.GetActorId(),
		ActorKind:       principalKindFromProto(pb.GetActorKind()),
		ActorRole:       principalRoleFromProto(pb.GetActorRole()),
		GrantedScopes:   append([]string(nil), pb.GetGrantedScopes()...),
		Origin:          bridgeOriginFromProto(pb.GetOrigin()),
		TaintedExternal: pb.GetTaintedExternal(),
	}
}

func participantFromProto(pb *covev1.RoomParticipant) CoveParticipant {
	if pb == nil {
		return CoveParticipant{}
	}
	return CoveParticipant{
		ID:          pb.GetId(),
		DisplayName: pb.GetDisplayName(),
		Handle:      pb.GetHandle(),
		Kind:        principalKindFromProto(pb.GetKind()),
		Role:        principalRoleFromProto(pb.GetRole()),
		Scopes:      append([]string(nil), pb.GetScopes()...),
		Presence:    presenceFromProto(pb.GetPresence()),
	}
}

func inviteFromProto(pb *covev1.RoomInvite) CoveInvite {
	if pb == nil {
		return CoveInvite{}
	}
	return CoveInvite{
		Token:       pb.GetToken(),
		RoomID:      pb.GetRoomId(),
		IssuedBy:    pb.GetIssuedBy(),
		GrantRole:   principalRoleFromProto(pb.GetGrantRole()),
		GrantScopes: append([]string(nil), pb.GetGrantScopes()...),
		MaxUses:     pb.GetMaxUses(),
		Uses:        pb.GetUses(),
	}
}

func messageFromProto(pb *covev1.RoomMessage) CoveMessage {
	if pb == nil {
		return CoveMessage{}
	}
	return CoveMessage{
		ID:            pb.GetId(),
		RoomID:        pb.GetRoomId(),
		ChannelID:     pb.GetChannelId(),
		ThreadRootID:  pb.GetThreadRootId(),
		AuthorID:      pb.GetAuthorId(),
		AuthorKind:    principalKindFromProto(pb.GetAuthorKind()),
		Body:          pb.GetBody(),
		Mentions:      append([]string(nil), pb.GetMentions()...),
		GateID:        pb.GetGateId(),
		Origin:        bridgeOriginFromProto(pb.GetOrigin()),
		BotChainDepth: uint8(pb.GetBotChainDepth()),
		CreatedAtUnix: int64(pb.GetCreatedAtUnix()),
	}
}

func gateFromProto(pb *covev1.RoomGate) CoveGate {
	if pb == nil {
		return CoveGate{}
	}
	return CoveGate{
		ID:          pb.GetId(),
		RoomID:      pb.GetRoomId(),
		ChannelID:   pb.GetChannelId(),
		RequestedBy: pb.GetRequestedBy(),
		Summary:     pb.GetSummary(),
		Capability:  pb.GetCapability(),
		Decision:    gateDecisionFromProto(pb.GetDecision()),
	}
}

func receiptFromProto(pb *covev1.GateApprovalReceipt) CoveGateReceipt {
	if pb == nil {
		return CoveGateReceipt{}
	}
	return CoveGateReceipt{
		GateID:         pb.GetGateId(),
		ResolvedBy:     pb.GetResolvedBy(),
		Decision:       gateDecisionFromProto(pb.GetDecision()),
		Reason:         pb.GetReason(),
		ResolvedAtUnix: int64(pb.GetResolvedAtUnix()),
	}
}

func snapshotFromProto(pb *covev1.RoomSnapshot) CoveRoomSnapshot {
	if pb == nil {
		return CoveRoomSnapshot{}
	}
	channels := make([]CoveChannel, 0, len(pb.GetChannels()))
	for _, ch := range pb.GetChannels() {
		channels = append(channels, CoveChannel{
			ID:       ch.GetId(),
			Name:     ch.GetName(),
			Topic:    ch.GetTopic(),
			IsDirect: ch.GetIsDirect(),
		})
	}
	participants := make([]CoveParticipant, 0, len(pb.GetParticipants()))
	for _, p := range pb.GetParticipants() {
		participants = append(participants, participantFromProto(p))
	}
	messages := make([]CoveMessage, 0, len(pb.GetMessages()))
	for _, m := range pb.GetMessages() {
		messages = append(messages, messageFromProto(m))
	}
	gates := make([]CoveGate, 0, len(pb.GetGates()))
	for _, g := range pb.GetGates() {
		gates = append(gates, gateFromProto(g))
	}
	return CoveRoomSnapshot{
		RoomID:       pb.GetRoomId(),
		Title:        pb.GetTitle(),
		LatestSeq:    pb.GetLatestSeq(),
		Channels:     channels,
		Participants: participants,
		Messages:     messages,
		Gates:        gates,
	}
}

func eventFromProto(pb *covev1.RoomEvent) CoveRoomEvent {
	out := CoveRoomEvent{
		Seq:    pb.GetSeq(),
		RoomID: pb.GetRoomId(),
	}
	switch kind := pb.GetKind().(type) {
	case *covev1.RoomEvent_MessagePosted:
		msg := messageFromProto(kind.MessagePosted)
		out.Kind = CoveRoomEventKind{Type: "message_posted", Message: &msg}
	case *covev1.RoomEvent_ParticipantJoined:
		p := participantFromProto(kind.ParticipantJoined)
		out.Kind = CoveRoomEventKind{Type: "participant_joined", Participant: &p}
	case *covev1.RoomEvent_GateRequested:
		g := gateFromProto(kind.GateRequested)
		out.Kind = CoveRoomEventKind{Type: "gate_requested", Gate: &g}
	case *covev1.RoomEvent_GateResolved:
		r := receiptFromProto(kind.GateResolved)
		out.Kind = CoveRoomEventKind{Type: "gate_resolved", Receipt: &r}
	}
	return out
}
