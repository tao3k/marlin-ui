// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

func reserveLocalAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve ephemeral TCP port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func locateMarlinCoveBinary() string {
	candidates := []string{
		os.Getenv("MARLIN_COVE_BIN"),
		filepath.Join("..", "..", "..", "marlin-cove", "target", "debug", "marlin-cove"),
	}
	for _, p := range candidates {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

// TestLiveCrossProcessCoveRelayEndToEnd verifies the complete gRPC over HTTP/2
// cross-process chain over real TCP sockets:
//
//	marlin-ui (Go gRPC HTTP/2 CoveClient + WatchRoomEvents stream)
//	  <-> real Rust `marlin-cove serve` binary (tonic gRPC HTTP/2 + external webhooks)
//	  <-> native `marlin.app.v1.AppServer` (`StreamTurn` + `x-marlin-cove-*` governance metadata)
func TestLiveCrossProcessCoveRelayEndToEnd(t *testing.T) {
	bin := locateMarlinCoveBinary()
	if bin == "" {
		t.Skip("marlin-cove debug binary not built; build .data/marlin-cove to run live cross-process E2E")
	}

	const (
		coveToken    = "e2e-cove-bearer-secret"
		gatewayToken = "e2e-marlin-gateway-secret"
		tgSecret     = "e2e-telegram-secret"
		dcSecret     = "e2e-discord-secret"
	)

	// 1. Start live gRPC HTTP/2 Marlin Gateway server verifying Bearer metadata and RoomGovernanceEnvelope.
	var (
		gwMu         sync.Mutex
		capturedEnvs []CoveGovernanceEnvelope
	)
	gwListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen marlin gateway gRPC: %v", err)
	}
	gwServer := grpc.NewServer()
	RegisterMarlinGatewayGRPCServer(gwServer, gatewayToken, func(_ context.Context, req MarlinGatewayTurnRequest) (string, error) {
		gwMu.Lock()
		capturedEnvs = append(capturedEnvs, req.Governance)
		gwMu.Unlock()
		return fmt.Sprintf("Marlin Gateway gRPC reply for @%s (actor=%s tainted=%v)", req.TargetAgentHandle, req.Governance.ActorID, req.Governance.TaintedExternal), nil
	})
	go func() { _ = gwServer.Serve(gwListener) }()
	defer gwServer.Stop()

	// 2. Write real TOML configuration for `marlin-cove serve`.
	grpcAddr := reserveLocalAddr(t)
	webhookAddr := reserveLocalAddr(t)
	cfgPath := filepath.Join(t.TempDir(), "cove.toml")
	tomlCfg := fmt.Sprintf(`
[relay]
state = "enabled"
grpc_bind_addr = %q
webhook_bind_addr = %q
auth_token = %q
default_room_id = "cove-main"
default_room_title = "Marlin Cove gRPC E2E"
default_channel_id = "general"
max_bot_chain_depth = 2

[relay.limits]
event_bus_capacity = 128
max_rooms = 16
max_invites = 32
max_messages_per_room = 128
max_gates_per_room = 32
max_bridge_dedup_entries = 128
max_grpc_message_bytes = 65536
max_message_bytes = 16384

[marlin_gateway]
grpc_endpoint = %q
bearer_token = %q

[telegram]
state = "enabled"
bot_token = "e2e-tg-bot-token"
webhook_secret = %q

[discord]
state = "enabled"
bot_token = "e2e-dc-bot-token"
bot_user_id = "bot-e2e"
ingress_secret = %q
mention_policy = "required"
`, grpcAddr, webhookAddr, coveToken, "http://"+gwListener.Addr().String(), gatewayToken, tgSecret, dcSecret)
	if err := os.WriteFile(cfgPath, []byte(tomlCfg), 0o600); err != nil {
		t.Fatalf("write cove.toml: %v", err)
	}

	// 3. Launch real Rust `marlin-cove serve --config <cfgPath>` process.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "serve", "--config", cfgPath)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start marlin-cove serve: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := NewCoveClient(grpcAddr, coveToken)
	defer func() { _ = client.Close() }()
	unauthClient := NewCoveClient(grpcAddr, "")
	defer func() { _ = unauthClient.Close() }()

	// Wait until gRPC HTTP/2 CoveRelayService/Health is live.
	var health CoveHealth
	var healthErr error
	for range 40 {
		health, healthErr = client.Health(ctx)
		if healthErr == nil && health.Status == "ok" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if healthErr != nil {
		t.Fatalf("marlin-cove gRPC failed to become healthy: %v (stderr: %s)", healthErr, stderrBuf.String())
	}

	// 4. Verify protected RPC fails without bearer metadata (Unauthenticated).
	if _, err := unauthClient.FetchSnapshot(ctx, "cove-main"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated FetchSnapshot to fail with Unauthenticated, got: %v", err)
	}

	// 5. Subscribe to real gRPC HTTP/2 server-streaming WatchRoomEvents inside StructuredScope.
	streamEvents := make(chan CoveRoomEvent, 32)
	scope := async.NewStructuredScope(ctx)
	defer scope.CloseAndDrain()
	scope.Go(func(runCtx context.Context) {
		_ = client.WatchEvents(runCtx, "cove-main", func(evt CoveRoomEvent) {
			select {
			case streamEvents <- evt:
			default:
			}
		})
	})
	time.Sleep(80 * time.Millisecond)

	// 6. Scoped invite + Quint-verified anti-hijack protection (AlreadyExists).
	invite, err := client.CreateInvite(ctx, "cove-main", "human:owner", "external_guest", []string{"public"}, 2)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}
	if _, err := unauthClient.RedeemInvite(ctx, invite.Token, "human:owner", "Hijack", "hijack", "human"); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected hijacking existing human:owner via invite to fail with AlreadyExists, got: %v", err)
	}
	if _, err := unauthClient.RedeemInvite(ctx, invite.Token, "human:guest-e2e", "Guest E2E", "guest", "human"); err != nil {
		t.Fatalf("RedeemInvite for new guest failed: %v", err)
	}

	// 7. Fire real external third-party Telegram & Discord HTTP webhooks into webhook_bind_addr.
	webhookBaseURL := "http://" + webhookAddr
	tgReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, webhookBaseURL+"/v1/bridges/telegram/webhook", strings.NewReader(`{
		"update_id": 777,
		"message": {
			"message_id": 901,
			"from": { "id": 501, "username": "tg_user", "first_name": "TG" },
			"chat": { "id": -1001, "type": "supergroup", "title": "Marlin" },
			"date": 1700000900,
			"text": "hello from live telegram webhook"
		}
	}`))
	tgReq.Header.Set("Content-Type", "application/json")
	tgReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", tgSecret)
	tgResp, err := http.DefaultClient.Do(tgReq)
	if err != nil || tgResp.StatusCode != http.StatusAccepted {
		t.Fatalf("telegram webhook failed: err=%v resp=%v", err, tgResp)
	}
	_ = tgResp.Body.Close()

	dcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, webhookBaseURL+"/v1/bridges/discord/ingress", strings.NewReader(`{
		"message_id": "dc-e2e-1",
		"guild_id": "g1",
		"channel_id": "c1",
		"author_id": "u9",
		"author_name": "dc_user",
		"author_is_bot": false,
		"content": "<@bot-e2e> hello from live discord",
		"mentioned_user_ids": ["bot-e2e"],
		"timestamp_unix": 1700000901
	}`))
	dcReq.Header.Set("Content-Type", "application/json")
	dcReq.Header.Set("X-Marlin-Cove-Ingress-Secret", dcSecret)
	dcResp, err := http.DefaultClient.Do(dcReq)
	if err != nil || dcResp.StatusCode != http.StatusAccepted {
		t.Fatalf("discord ingress failed: err=%v resp=%v", err, dcResp)
	}
	_ = dcResp.Body.Close()

	// 8. Dispatch live turn over gRPC HTTP/2: marlin-ui -> marlin-cove -> MarlinGatewayService -> marlin-cove!
	dispatched, err := client.DispatchTurn(ctx, "cove-main", "architecture", "human:owner", "architect", "@architect inspect live gRPC bridge")
	if err != nil {
		t.Fatalf("DispatchTurn failed: %v", err)
	}
	if dispatched.AgentReply.Origin != "marlin_gateway" || !strings.Contains(dispatched.AgentReply.Body, "Marlin Gateway gRPC reply for @architect") {
		t.Fatalf("unexpected live Marlin Gateway gRPC reply: %+v", dispatched.AgentReply)
	}
	gwMu.Lock()
	gwCount := len(capturedEnvs)
	var capturedEnv CoveGovernanceEnvelope
	if gwCount > 0 {
		capturedEnv = capturedEnvs[0]
	}
	gwMu.Unlock()
	if gwCount != 1 || capturedEnv.RoomID != "cove-main" || capturedEnv.ActorID != "human:owner" || capturedEnv.ActorRole != "owner" || capturedEnv.TaintedExternal {
		t.Fatalf("expected marlin.app.v1.AppServer to receive 1 verified RoomGovernanceEnvelope via metadata, got count=%d env=%+v", gwCount, capturedEnv)
	}

	// 9. Tier-0 Gate: Agent denied (PermissionDenied), Human Owner approves, replay denied (AlreadyExists).
	gate, err := client.RequestGate(ctx, "cove-main", "security", "agent:architect", "Deploy prod", "shell:deploy")
	if err != nil {
		t.Fatalf("RequestGate failed: %v", err)
	}
	if _, err := client.ResolveGate(ctx, "cove-main", gate.ID, "agent:architect", true, "self"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected agent Tier-0 gate resolution to fail with PermissionDenied, got: %v", err)
	}
	receipt, err := client.ResolveGate(ctx, "cove-main", gate.ID, "human:owner", true, "human approved")
	if err != nil || receipt.Decision != "approved" {
		t.Fatalf("expected human owner ResolveGate to succeed: err=%v receipt=%+v", err, receipt)
	}
	if _, err := client.ResolveGate(ctx, "cove-main", gate.ID, "human:owner", false, "replay"); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected replaying resolved gate to fail with AlreadyExists, got: %v", err)
	}

	// 10. Confirm live gRPC HTTP/2 stream received Telegram, Discord, Marlin Gateway, and GateResolved events!
	var sawTG, sawDC, sawGW, sawGateResolved bool
	deadline := time.After(3 * time.Second)
	for !(sawTG && sawDC && sawGW && sawGateResolved) {
		select {
		case evt := <-streamEvents:
			if evt.Kind.Type == "message_posted" && evt.Kind.Message != nil {
				switch evt.Kind.Message.Origin {
				case "telegram":
					sawTG = true
				case "discord":
					sawDC = true
				case "marlin_gateway":
					sawGW = true
				}
			}
			if evt.Kind.Type == "gate_resolved" && evt.Kind.Receipt != nil && evt.Kind.Receipt.GateID == gate.ID {
				sawGateResolved = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for live gRPC HTTP/2 stream events: tg=%v dc=%v gw=%v gate=%v", sawTG, sawDC, sawGW, sawGateResolved)
		}
	}

	// 11. Confirm snapshot projects external bridge messages with TaintedExternal=true.
	snap, err := client.FetchSnapshot(ctx, "cove-main")
	if err != nil {
		t.Fatalf("FetchSnapshot failed: %v", err)
	}
	projected := snap.ToRoomFullSnapshot("general", "human:owner")
	taintedCount := 0
	for _, m := range projected.Messages {
		if m.TaintedExternal {
			taintedCount++
		}
	}
	if taintedCount != 2 {
		t.Fatalf("expected 2 tainted external bridge messages (Telegram + Discord) in #general, got %d", taintedCount)
	}

	// 12. Verify CoveRelayBridge binds featurekit.RoomController for Desktop, iOS & Android
	// including 1:1 Direct Bot DM channels (`dm:agent:architect`) and async non-blocking DispatchTurn.
	var dirtyCalls int
	var dirtyMu sync.Mutex
	bridge := AttachCoveRelayBridge(ctx, appconfig.Config{
		CoveURL:    grpcAddr,
		CoveToken:  coveToken,
		CoveRoomID: "cove-main",
	}, func() {
		dirtyMu.Lock()
		dirtyCalls++
		dirtyMu.Unlock()
	})
	if bridge == nil {
		t.Fatalf("expected AttachCoveRelayBridge to return live bridge")
	}
	defer bridge.Close()

	uiSnap := featurekit.AgentRoomSnapshot()
	if !uiSnap.Available || len(uiSnap.Channels) < 6 {
		t.Fatalf("expected live Cove snapshot with shared + 3 direct bot channels, got %+v", uiSnap.Channels)
	}
	if err := featurekit.SelectAgentRoomChannel("dm:agent:architect"); err != nil {
		t.Fatalf("SelectAgentRoomChannel dm:agent:architect failed: %v", err)
	}
	if err := featurekit.PostAgentRoomMessage("dm:agent:architect", "Direct DM check over gRPC HTTP/2", ""); err != nil {
		t.Fatalf("PostAgentRoomMessage in DM channel failed: %v", err)
	}
	dmDeadline := time.Now().Add(3 * time.Second)
	sawDMReply := false
	for time.Now().Before(dmDeadline) {
		cur := featurekit.AgentRoomSnapshot()
		for _, m := range cur.Messages {
			if m.IsAgent && strings.Contains(m.Body, "Marlin Gateway gRPC reply for @architect") {
				sawDMReply = true
				break
			}
		}
		if sawDMReply {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !sawDMReply {
		t.Fatalf("timed out waiting for async 1:1 Direct Bot DM reply in CoveRelayBridge")
	}
}

// TestLiveRealAppServerCoveRoomLLM exercises the real 3-process production
// chain when MARLIN_COVE_LIVE_LLM=1 is set:
//
//	marlin-ui (CoveRelayBridge) <-> real Rust marlin-cove (gRPC HTTP/2)
//	  <-> real Rust .bin/marlin-app-server (marlin.app.v1.AppServer/StreamTurn -> live LLM)
func TestLiveRealAppServerCoveRoomLLM(t *testing.T) {
	if os.Getenv("MARLIN_COVE_LIVE_LLM") != "1" {
		t.Skip("set MARLIN_COVE_LIVE_LLM=1 to run live 3-process LLM Room E2E")
	}
	coveBin := locateMarlinCoveBinary()
	if coveBin == "" {
		t.Fatalf("marlin-cove binary not found")
	}
	appBin, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".bin", "marlin-app-server"))
	if err != nil {
		t.Fatalf("resolve marlin-app-server: %v", err)
	}
	if _, err := os.Stat(appBin); err != nil {
		t.Fatalf("marlin-app-server binary missing at %s: %v", appBin, err)
	}
	cfgFile, err := filepath.Abs(filepath.Join("..", "..", "example.toml"))
	if err != nil {
		t.Fatalf("resolve example.toml: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// 1. Start real Rust marlin-app-server with example.toml + inherited .env LLM keys.
	const bridgeToken = "e2e-live-bridge-token"
	appCmd := exec.CommandContext(ctx, appBin)
	appCmd.Env = append(os.Environ(),
		"MARLIN_BRIDGE_REAL_SESSION=1",
		"MARLIN_BRIDGE_TOKEN="+bridgeToken,
		"MARLIN_CONFIG="+cfgFile,
		"MARLIN_BRIDGE_CONFIG="+cfgFile,
	)
	var appOut bytes.Buffer
	var appMu sync.Mutex
	appCmd.Stdout = &syncWriter{mu: &appMu, buf: &appOut}
	appCmd.Stderr = &syncWriter{mu: &appMu, buf: &appOut}
	if err := appCmd.Start(); err != nil {
		t.Fatalf("start marlin-app-server: %v", err)
	}
	defer func() {
		_ = appCmd.Process.Kill()
		_ = appCmd.Wait()
	}()

	var appAddr string
	for range 80 {
		appMu.Lock()
		text := appOut.String()
		appMu.Unlock()
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "READY ") {
				appAddr = strings.TrimSpace(strings.TrimPrefix(line, "READY "))
				break
			}
		}
		if appAddr != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if appAddr == "" {
		appMu.Lock()
		logs := appOut.String()
		appMu.Unlock()
		t.Fatalf("marlin-app-server never printed READY; logs:\n%s", logs)
	}
	t.Logf("live marlin-app-server READY at %s", appAddr)

	// 2. Start real Rust marlin-cove targeting marlin-app-server's native marlin.app.v1.AppServer.
	coveAddr := reserveLocalAddr(t)
	coveCmd := exec.CommandContext(ctx, coveBin, "serve")
	coveCmd.Env = append(os.Environ(),
		"MARLIN_COVE_RELAY_GRPC_BIND="+coveAddr,
		"MARLIN_COVE_MARLIN_GATEWAY_ENDPOINT=http://"+appAddr,
		"MARLIN_COVE_MARLIN_GATEWAY_TOKEN="+bridgeToken,
	)
	var coveErr bytes.Buffer
	coveCmd.Stderr = &coveErr
	if err := coveCmd.Start(); err != nil {
		t.Fatalf("start marlin-cove serve: %v", err)
	}
	defer func() {
		_ = coveCmd.Process.Kill()
		_ = coveCmd.Wait()
	}()

	client := NewCoveClient(coveAddr, "")
	defer func() { _ = client.Close() }()
	var healthErr error
	for range 40 {
		if _, healthErr = client.Health(ctx); healthErr == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if healthErr != nil {
		t.Fatalf("marlin-cove unhealthy: %v (stderr: %s)", healthErr, coveErr.String())
	}
	t.Logf("live marlin-cove READY at %s -> app-server %s", coveAddr, appAddr)

	// 3. Dispatch a real @architect turn through Cove -> real marlin-app-server -> real LLM!
	resp, err := client.DispatchTurn(
		ctx,
		"cove-main",
		"architecture",
		"human:owner",
		"architect",
		"@architect Reply in one short sentence confirming you received this Marlin Cove Room gRPC turn.",
	)
	if err != nil {
		appMu.Lock()
		logs := appOut.String()
		appMu.Unlock()
		t.Fatalf("live DispatchTurn failed: %v\napp-server logs:\n%s\ncove stderr:\n%s", err, logs, coveErr.String())
	}
	if strings.TrimSpace(resp.AgentReply.Body) == "" || strings.Contains(resp.AgentReply.Body, "marlin-bridge received") {
		t.Fatalf("expected real live LLM reply (not echo stub), got: %q", resp.AgentReply.Body)
	}
	t.Logf("LIVE LLM ROOM REPLY (@architect in #architecture): %s", resp.AgentReply.Body)
}

type syncWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}
