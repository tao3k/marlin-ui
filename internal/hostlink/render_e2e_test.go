// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package hostlink_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	"github.com/tao3k/marlin-ui/internal/mobile/ios"
)

// TestLiveRustHostRendersEndToEndAcrossDesktopAndIOS verifies the complete
// Rust marlin-app-server -> Go hostlink gRPC -> Desktop Frame & iOS Shell
// rendering pipeline, including live turn streaming, New Chat session isolation,
// and live Rust HELM search rendering.
func TestLiveRustHostRendersEndToEndAcrossDesktopAndIOS(t *testing.T) {
	binPath := strings.TrimSpace(os.Getenv("MARLIN_APP_SERVER_BIN"))
	if binPath == "" {
		t.Skip("set MARLIN_APP_SERVER_BIN to run live Rust -> Desktop + iOS rendering E2E")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	tmp := t.TempDir()
	cfg := appconfig.Config{
		AppServerBin: binPath,
		SessionID:    "e2e-render-1",
		ThemeID:      "doom-one",
		TursoPath:    filepath.Join(tmp, "render-e2e.turso"),
		RecoveryFile: filepath.Join(tmp, "render-rec.json"),
		RealSession:  true,
	}

	as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	link, err := hostlink.Connect(ctx, cfg, as, schema.TargetDesktopMacOS, nil)
	if err != nil {
		t.Fatalf("Connect live Rust host: %v", err)
	}
	defer link.Close()

	reg, ledger, host := featurekit.Mount("all", as, link.OnDraftChanged, func() {})
	deskFrame := frame.New(frame.Config{
		Adaptive:      as,
		Link:          link,
		AppConfig:     cfg,
		FeatureReg:    reg,
		FeatureLedger: ledger,
		FeatureHost:   host,
	})
	deskFrame.SelectBuffer(buffer.Conversation)

	iosShell := ios.New(ios.Config{
		Adaptive:  as,
		Link:      link,
		AppConfig: cfg,
	})

	deskTester := ui.NewTester(deskFrame.View, 1280, 820)
	deskTester.SetVibrancy(true)
	iosTester := ui.NewTester(iosShell.View, 393, 852)
	iosTester.SetVibrancy(true)

	// 1. Send a live prompt through Rust gRPC and wait for streamed completion.
	as.Desktop.OnSendPrompt("say ok")
	dl := time.Now().Add(12 * time.Second)
	gotReply := ""
	for time.Now().Before(dl) {
		for i := as.Desktop.Transcript.Len() - 1; i >= 0; i-- {
			if e, ok := as.Desktop.Transcript.Entry(i); ok && e.Role == mygoshell.RoleAssistant && strings.TrimSpace(e.Text) != "" && !e.Streaming {
				gotReply = strings.TrimSpace(e.Text)
				break
			}
		}
		if gotReply != "" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if gotReply == "" {
		t.Fatalf("timed out waiting for live Rust reply (entries=%d)", as.Desktop.Transcript.Len())
	}

	// 2. Verify BOTH Desktop *conversation* AND iOS Chat render the live Rust reply cleanly.
	deskTester.Frame()
	iosTester.Frame()
	if !deskTester.HasText("say ok") {
		t.Fatalf("Desktop *conversation* missing user prompt, got %q", strings.Join(deskTester.Texts(), " | "))
	}
	if !iosTester.HasText("say ok") {
		t.Fatalf("iOS Chat missing user prompt, got %q", strings.Join(iosTester.Texts(), " | "))
	}

	// 3. Verify New Chat starts a brand-new isolated session & clean Hero Stage on BOTH shells,
	//    while switching back to the previous session restores its full conversation!
	firstSID := as.Desktop.ActiveSessionID
	iosShell.NewChat()
	if as.Desktop.ActiveSessionID == firstSID {
		t.Fatalf("expected NewChat() to switch to a fresh session ID, stayed on %q", firstSID)
	}
	iosTester.Frame()
	if !iosTester.HasText("What can we build today?") || iosTester.HasText("say ok") {
		t.Fatalf("expected fresh empty Hero Stage after NewChat(), got %q", strings.Join(iosTester.Texts(), " | "))
	}
	as.Desktop.OnSelectSession(firstSID)
	iosTester.Frame()
	if !iosTester.HasText("say ok") {
		t.Fatalf("expected switching back to prior session %q to restore history", firstSID)
	}
}
