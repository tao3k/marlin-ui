// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"

	covev1 "github.com/tao3k/marlin-ui/gen/marlin/cove/v1"
)

type stubCoveRelayGRPCServer struct {
	covev1.UnimplementedCoveRelayServiceServer
}

func (s *stubCoveRelayGRPCServer) Health(context.Context, *covev1.HealthRequest) (*covev1.HealthResponse, error) {
	return &covev1.HealthResponse{
		Status:   covev1.ServiceHealthStatus_SERVICE_HEALTH_STATUS_OK,
		Service:  "marlin-cove",
		Features: []string{"relay", "telegram", "discord"},
	}, nil
}

func (s *stubCoveRelayGRPCServer) GetRoomSnapshot(context.Context, *covev1.GetRoomSnapshotRequest) (*covev1.RoomSnapshot, error) {
	return &covev1.RoomSnapshot{
		RoomId:    "cove-main",
		Title:     "Marlin Cove Main",
		LatestSeq: 4,
		Channels: []*covev1.RoomChannel{
			{Id: "general", Name: "general", Topic: "Shared room"},
			{Id: "architecture", Name: "architecture", Topic: "Design"},
		},
		Participants: []*covev1.RoomParticipant{
			{Id: "human:owner", DisplayName: "Owner", Handle: "owner", Kind: covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN, Role: covev1.PrincipalRole_PRINCIPAL_ROLE_OWNER, Presence: covev1.PresenceStatus_PRESENCE_STATUS_ONLINE},
			{Id: "agent:architect", DisplayName: "Architect", Handle: "architect", Kind: covev1.PrincipalKind_PRINCIPAL_KIND_AGENT, Role: covev1.PrincipalRole_PRINCIPAL_ROLE_ROLE_AGENT, Presence: covev1.PresenceStatus_PRESENCE_STATUS_THINKING},
			{Id: "tg:501", DisplayName: "@alice", Handle: "alice", Kind: covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN, Role: covev1.PrincipalRole_PRINCIPAL_ROLE_EXTERNAL_GUEST, Presence: covev1.PresenceStatus_PRESENCE_STATUS_ONLINE},
		},
		Messages: []*covev1.RoomMessage{
			{
				Id:         "msg_1",
				RoomId:     "cove-main",
				ChannelId:  "general",
				AuthorId:   "tg:501",
				AuthorKind: covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN,
				Body:       "@architect review Cove relay",
				Mentions:   []string{"architect"},
				Origin:     covev1.BridgeOrigin_BRIDGE_ORIGIN_TELEGRAM,
			},
		},
	}, nil
}

func (s *stubCoveRelayGRPCServer) GetGovernanceEnvelope(context.Context, *covev1.GetGovernanceEnvelopeRequest) (*covev1.RoomGovernanceEnvelope, error) {
	return &covev1.RoomGovernanceEnvelope{
		RoomId:          "cove-main",
		ChannelId:       "general",
		ActorId:         "tg:501",
		ActorKind:       covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN,
		ActorRole:       covev1.PrincipalRole_PRINCIPAL_ROLE_EXTERNAL_GUEST,
		GrantedScopes:   []string{"public"},
		Origin:          covev1.BridgeOrigin_BRIDGE_ORIGIN_TELEGRAM,
		TaintedExternal: true,
	}, nil
}

func (s *stubCoveRelayGRPCServer) DispatchTurn(context.Context, *covev1.DispatchTurnRequest) (*covev1.DispatchTurnResponse, error) {
	return &covev1.DispatchTurnResponse{
		UserMessage: &covev1.RoomMessage{Id: "msg_u1", ChannelId: "architecture", AuthorId: "human:owner", AuthorKind: covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN, Body: "@architect check", Origin: covev1.BridgeOrigin_BRIDGE_ORIGIN_COVE_NATIVE},
		Governance:  &covev1.RoomGovernanceEnvelope{RoomId: "cove-main", ChannelId: "architecture", ActorId: "human:owner", ActorKind: covev1.PrincipalKind_PRINCIPAL_KIND_HUMAN, ActorRole: covev1.PrincipalRole_PRINCIPAL_ROLE_OWNER, Origin: covev1.BridgeOrigin_BRIDGE_ORIGIN_COVE_NATIVE, TaintedExternal: false},
		AgentReply:  &covev1.RoomMessage{Id: "msg_a1", ChannelId: "architecture", AuthorId: "agent:architect", AuthorKind: covev1.PrincipalKind_PRINCIPAL_KIND_AGENT, Body: "Marlin Gateway verified.", Origin: covev1.BridgeOrigin_BRIDGE_ORIGIN_MARLIN_GATEWAY, BotChainDepth: 1},
	}, nil
}

func (s *stubCoveRelayGRPCServer) ResolveGate(context.Context, *covev1.ResolveGateRequest) (*covev1.GateApprovalReceipt, error) {
	return &covev1.GateApprovalReceipt{
		GateId:     "gate_1",
		ResolvedBy: "human:owner",
		Decision:   covev1.GateDecision_GATE_DECISION_APPROVED,
		Reason:     "human reviewed",
	}, nil
}

func TestCoveClientGRPCWireParityAndSnapshotProjection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	covev1.RegisterCoveRelayServiceServer(grpcSrv, &stubCoveRelayGRPCServer{})
	go func() { _ = grpcSrv.Serve(ln) }()
	defer grpcSrv.Stop()

	client := NewCoveClient(ln.Addr().String(), "cove-test-token")
	defer func() { _ = client.Close() }()
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

	dispatched, err := client.DispatchTurn(ctx, "cove-main", "architecture", "human:owner", "architect", "@architect check")
	if err != nil {
		t.Fatalf("DispatchTurn failed: %v", err)
	}
	if dispatched.AgentReply.Origin != "marlin_gateway" || dispatched.Governance.TaintedExternal {
		t.Fatalf("unexpected dispatch response: %+v", dispatched)
	}

	receipt, err := client.ResolveGate(ctx, "cove-main", "gate_1", "human:owner", true, "human reviewed")
	if err != nil {
		t.Fatalf("ResolveGate failed: %v", err)
	}
	if receipt.Decision != "approved" || receipt.ResolvedBy != "human:owner" {
		t.Fatalf("unexpected gate receipt: %+v", receipt)
	}
}
