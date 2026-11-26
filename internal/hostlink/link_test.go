// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func TestHostLinkStandaloneContinuityAndDraftRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	recFile := filepath.Join(t.TempDir(), "marlin-ui-recovery.json")
	cfg := appconfig.Config{
		SessionID:    "ui-standalone-1",
		ThemeID:      "midnight",
		RecoveryFile: recFile,
		Standalone:   true,
	}

	// Session 1: write unsent draft -> Close() flushes fsync atomic snapshot.
	as1 := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{BrandTitle: "Marlin UI"})
	link1, err := hostlink.Connect(ctx, cfg, as1, schema.TargetDesktopMacOS, nil)
	if err != nil {
		t.Fatalf("Connect 1: %v", err)
	}
	if link1.Mode != hostlink.ModeStandalone {
		t.Fatalf("expected ModeStandalone, got %s", link1.Mode)
	}
	as1.Desktop.Transcript.SetComposerText("Recover me across marlin-ui cold restart")
	link1.Close()

	// Session 2: cold restart -> draft automatically restored!
	as2 := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{BrandTitle: "Marlin UI"})
	link2, err := hostlink.Connect(ctx, cfg, as2, schema.TargetDesktopMacOS, nil)
	if err != nil {
		t.Fatalf("Connect 2: %v", err)
	}
	defer link2.Close()

	if got := as2.Desktop.Transcript.ComposerText(); got != "Recover me across marlin-ui cold restart" {
		t.Fatalf("expected restored draft, got %q", got)
	}
	as2.Desktop.OnSendPrompt("Hello Marlin UI")
	if as2.Desktop.Transcript.Len() < 2 {
		t.Fatalf("expected user + preview turn appended, len=%d", as2.Desktop.Transcript.Len())
	}
}

func TestHostLinkMultiSessionHelmAndMultimodalSubmit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := appconfig.Config{
		SessionID:    "marlin-alpha",
		ThemeID:      "midnight",
		RecoveryFile: filepath.Join(t.TempDir(), "multi-session-recovery.json"),
		Standalone:   true,
	}
	as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{BrandTitle: "Marlin UI"})
	var switched []string
	link, err := hostlink.Connect(
		ctx,
		cfg,
		as,
		schema.TargetDesktopMacOS,
		nil,
		hostlink.WithOnSessionChanged(func(sid string) { switched = append(switched, sid) }),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer link.Close()

	// 1. Draft isolation across session creation & switching.
	as.Desktop.Transcript.SetComposerText("Draft in alpha session")
	if as.Desktop.OnNewSession == nil {
		t.Fatalf("expected OnNewSession wired")
	}
	as.Desktop.OnNewSession()
	betaID := as.Desktop.ActiveSessionID
	if betaID == "" || betaID == "marlin-alpha" {
		t.Fatalf("expected new session ID, got %q", betaID)
	}
	as.Desktop.Transcript.SetComposerText("Draft in beta session")
	as.Desktop.OnSelectSession("marlin-alpha")
	if got := as.Desktop.Transcript.ComposerText(); got != "Draft in alpha session" {
		t.Fatalf("expected restored alpha draft, got %q", got)
	}
	as.Desktop.OnSelectSession(betaID)
	if got := as.Desktop.Transcript.ComposerText(); got != "Draft in beta session" {
		t.Fatalf("expected restored beta draft, got %q", got)
	}
	if len(switched) < 3 {
		t.Fatalf("expected session-change callbacks, got %v", switched)
	}

	// 2. Live multi-source Helm search streams sessions.
	if as.Desktop.HelmSearch == nil || as.Desktop.HelmSearch.Engine == nil {
		t.Fatalf("expected HelmSearch initialized")
	}
	as.Desktop.HelmSearch.SetQuery("alpha")
	totalHits, _, _, _ := as.Desktop.HelmSearch.Engine.Stats()
	if totalHits == 0 {
		t.Fatalf("expected HelmSearch to admit live session candidate for query 'alpha'")
	}

	// 3. Multimodal composer submission preserves attachments.
	before := as.Desktop.Transcript.Len()
	as.Desktop.Transcript.OnSubmitTurn(composer.TurnSubmission{
		Prompt: "Inspect attached log",
		Attachments: []composer.StagedAttachment{
			{
				Ref: composer.AttachmentRef{
					ID:       "att-1",
					Name:     "trace.log",
					MIMEType: "text/plain",
					ByteSize: 128,
				},
			},
		},
	})
	if as.Desktop.Transcript.Len() < before+2 {
		t.Fatalf("expected user+assistant rows after multimodal submit, got %d", as.Desktop.Transcript.Len())
	}
	if entry, ok := as.Desktop.Transcript.Entry(before); !ok || len(entry.Attachments) != 1 || entry.Attachments[0].Name != "trace.log" {
		t.Fatalf("expected attachment preserved on transcript entry, got %+v", entry.Attachments)
	}
}

func TestHostLinkLiveRustAppServerGRPCConnection(t *testing.T) {
	binPath := strings.TrimSpace(os.Getenv("MARLIN_APP_SERVER_BIN"))
	if binPath == "" {
		t.Skip("set MARLIN_APP_SERVER_BIN (or run `just dev-test-rust`) to verify live Rust gRPC connection")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tmp := t.TempDir()
	cfg := appconfig.Config{
		AppServerBin: binPath,
		SessionID:    "marlin-ui-live-rust",
		ThemeID:      "midnight",
		TursoPath:    filepath.Join(tmp, "ui-live.turso"),
		RecoveryFile: filepath.Join(tmp, "ui-recovery.json"),
		RealSession:  true,
	}
	as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{BrandTitle: "Marlin UI"})
	link, err := hostlink.Connect(ctx, cfg, as, schema.TargetDesktopMacOS, nil)
	if err != nil {
		t.Fatalf("Connect live Rust sidecar: %v", err)
	}
	defer link.Close()

	if link.Mode != hostlink.ModeSidecar {
		t.Fatalf("expected ModeSidecar against %s, got %s", binPath, link.Mode)
	}

	as.Desktop.OnSendPrompt("Verify live marlin-ui -> Rust marlin-mygo-host gRPC turn")
	deadline := time.Now().Add(10 * time.Second)
	gotReply := false
	for time.Now().Before(deadline) {
		for i := 0; i < as.Desktop.Transcript.Len(); i++ {
			if e, ok := as.Desktop.Transcript.Entry(i); ok && e.Role == mygoshell.RoleAssistant && strings.TrimSpace(e.Text) != "" && !e.Streaming {
				gotReply = true
				break
			}
		}
		if gotReply {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !gotReply {
		t.Fatalf("timed out waiting for live Rust assistant reply over gRPC (entries=%d)", as.Desktop.Transcript.Len())
	}
}
