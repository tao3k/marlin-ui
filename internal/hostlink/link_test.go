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
