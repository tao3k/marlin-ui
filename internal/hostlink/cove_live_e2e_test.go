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
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tao3k/marlin-mygo/runtime/async"
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

// TestLiveCrossProcessCoveRelayEndToEnd verifies the complete live chain across
// real TCP sockets:
//
//	marlin-ui (Go CoveClient + SSE WatchEvents)
//	  <-> real Rust `marlin-cove serve` binary (TOML config + auth + webhooks)
//	  <-> live Marlin Gateway HTTP adapter (`POST /message` with RoomGovernanceEnvelope)
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

	// 1. Start live Marlin Gateway HTTP adapter verifying Bearer auth and RoomGovernanceEnvelope.
	var (
		gwMu          sync.Mutex
		capturedEnvs  []CoveGovernanceEnvelope
	)
	gatewaySrv := httptest.NewServer(NewMarlinGatewayHTTPHandler(gatewayToken, func(_ context.Context, req MarlinGatewayTurnRequest) (string, error) {
		gwMu.Lock()
		capturedEnvs = append(capturedEnvs, req.Governance)
		gwMu.Unlock()
		return fmt.Sprintf("Marlin Gateway reply for @%s (actor=%s tainted=%v)", req.TargetAgentHandle, req.Governance.ActorID, req.Governance.TaintedExternal), nil
	}))
	defer gatewaySrv.Close()

	// 2. Write real TOML configuration for `marlin-cove serve`.
	coveAddr := reserveLocalAddr(t)
	cfgPath := filepath.Join(t.TempDir(), "cove.toml")
	tomlCfg := fmt.Sprintf(`
[relay]
state = "enabled"
bind_addr = %q
auth_token = %q
allowed_origins = []
default_room_id = "cove-main"
default_room_title = "Marlin Cove E2E"
default_channel_id = "general"
max_bot_chain_depth = 2

[relay.limits]
event_bus_capacity = 128
max_rooms = 16
max_invites = 32
max_messages_per_room = 128
max_gates_per_room = 32
max_bridge_dedup_entries = 128
max_body_bytes = 65536
max_message_bytes = 16384

[marlin_gateway]
base_url = %q
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
`, coveAddr, coveToken, gatewaySrv.URL, gatewayToken, tgSecret, dcSecret)
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

	baseURL := "http://" + coveAddr
	client := NewCoveClient(baseURL, coveToken)
	unauthClient := NewCoveClient(baseURL, "")

	// Wait until /v1/health is live.
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
		t.Fatalf("marlin-cove failed to become healthy: %v (stderr: %s)", healthErr, stderrBuf.String())
	}

	// 4. Verify protected read fails without bearer token (401).
	if _, err := unauthClient.FetchSnapshot(ctx, "cove-main"); err == nil {
		t.Fatalf("expected unauthenticated FetchSnapshot to fail with 401")
	}

	// 5. Subscribe to real SSE stream using StructuredScope.
	sseEvents := make(chan CoveRoomEvent, 32)
	scope := async.NewStructuredScope(ctx)
	defer scope.CloseAndDrain()
	scope.Go(func(runCtx context.Context) {
		_ = client.WatchEvents(runCtx, "cove-main", func(evt CoveRoomEvent) {
			select {
			case sseEvents <- evt:
			default:
			}
		})
	})
	time.Sleep(80 * time.Millisecond)

	// 6. Scoped invite + Quint-verified anti-hijack protection.
	invite, err := client.CreateInvite(ctx, "cove-main", "human:owner", "external_guest", []string{"public"}, 2)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}
	if _, err := unauthClient.RedeemInvite(ctx, invite.Token, "human:owner", "Hijack", "hijack", "human"); err == nil {
		t.Fatalf("expected hijacking existing human:owner via invite to fail with 409")
	}
	if _, err := unauthClient.RedeemInvite(ctx, invite.Token, "human:guest-e2e", "Guest E2E", "guest", "human"); err != nil {
		t.Fatalf("RedeemInvite for new guest failed: %v", err)
	}

	// 7. Fire real Telegram Webhook & Discord Ingress into live marlin-cove.
	tgReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/bridges/telegram/webhook", strings.NewReader(`{
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

	dcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/bridges/discord/ingress", strings.NewReader(`{
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

	// 8. Dispatch live turn from marlin-ui -> marlin-cove -> Marlin Gateway -> marlin-cove room!
	dispatched, err := client.DispatchTurn(ctx, "cove-main", "architecture", "human:owner", "architect", "@architect inspect live bridge")
	if err != nil {
		t.Fatalf("DispatchTurn failed: %v", err)
	}
	if dispatched.AgentReply.Origin != "marlin_gateway" || !strings.Contains(dispatched.AgentReply.Body, "Marlin Gateway reply for @architect") {
		t.Fatalf("unexpected live Marlin Gateway reply: %+v", dispatched.AgentReply)
	}
	gwMu.Lock()
	gwCount := len(capturedEnvs)
	gwMu.Unlock()
	if gwCount != 1 {
		t.Fatalf("expected Marlin Gateway to receive 1 verified RoomGovernanceEnvelope, got %d", gwCount)
	}

	// 9. Tier-0 Gate: Agent denied (403), Human Owner approves (200), replay denied (409).
	gate, err := client.RequestGate(ctx, "cove-main", "security", "agent:architect", "Deploy prod", "shell:deploy")
	if err != nil {
		t.Fatalf("RequestGate failed: %v", err)
	}
	if _, err := client.ResolveGate(ctx, "cove-main", gate.ID, "agent:architect", true, "self"); err == nil {
		t.Fatalf("expected agent Tier-0 gate resolution to fail with 403")
	}
	receipt, err := client.ResolveGate(ctx, "cove-main", gate.ID, "human:owner", true, "human approved")
	if err != nil || receipt.Decision != "approved" {
		t.Fatalf("expected human owner ResolveGate to succeed: err=%v receipt=%+v", err, receipt)
	}
	if _, err := client.ResolveGate(ctx, "cove-main", gate.ID, "human:owner", false, "replay"); err == nil {
		t.Fatalf("expected replaying resolved gate to fail with 409")
	}

	// 10. Confirm live SSE stream received Telegram, Discord, Marlin Gateway, and GateResolved events!
	var sawTG, sawDC, sawGW, sawGateResolved bool
	deadline := time.After(3 * time.Second)
	for !(sawTG && sawDC && sawGW && sawGateResolved) {
		select {
		case evt := <-sseEvents:
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
			t.Fatalf("timed out waiting for live SSE events: tg=%v dc=%v gw=%v gate=%v", sawTG, sawDC, sawGW, sawGateResolved)
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
}
