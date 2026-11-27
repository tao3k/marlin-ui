// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal

package featurekit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/features/chatroom"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

func TestDesktopAgentRoomAppleGeistParityAndLiveBotRouting(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	_, _, host := featurekit.Mount("all", sh, nil, func() {})
	if host == nil {
		t.Fatalf("expected non-nil feature host")
	}

	th := theme.DoomOne()
	mygoTheme := th.ToMyGoTheme()

	renderFn, ok := host.Renderer(chatroom.BufferIDChatRoom)
	if !ok || renderFn == nil {
		t.Fatalf("expected chatroom buffer renderer registered on Desktop feature host")
	}

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(mygoTheme)
		renderFn(c, mygoTheme)
	}, 1180, 760)
	tt.SetVibrancy(true)
	tt.Frame()

	// Verify dual-tier CHANNELS + AI BOTS · DIRECT rail + HUMANS / AI AGENTS grouping.
	for _, want := range []string{"CHANNELS", "AI BOTS · DIRECT", "HUMANS · 1", "AI AGENTS · 3", "APPROVER", "To Task"} {
		if !tt.HasText(want) {
			t.Fatalf("expected Desktop Agent Room to render %q, got %q", want, strings.Join(tt.Texts(), " | "))
		}
	}

	// Verify live @reviewer routing from Desktop room works identically to Mobile.
	done := make(chan struct{})
	featurekit.SetRoomAgentResponder(func(_ context.Context, req featurekit.RoomAgentRequest) (string, error) {
		defer close(done)
		return "**Reviewer Audit**: Desktop & Mobile room parity confirmed for `" + req.ChannelName + "`.", nil
	})
	defer featurekit.SetRoomAgentResponder(nil)

	if err := featurekit.PostAgentRoomMessage("ch-workspace", "@reviewer verify desktop parity", ""); err != nil {
		t.Fatalf("PostAgentRoomMessage failed: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for @reviewer live reply")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("Reviewer Audit") {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if !tt.HasText("Reviewer Audit") {
		t.Fatalf("expected @reviewer reply rendered in Desktop Agent Room, got %q", strings.Join(tt.Texts(), " | "))
	}

	// Verify opening a thread shows both root message and seeded reply, and replying
	// via Enter or "Send thread reply" button updates THREAD REPLIES immediately.
	if err := featurekit.OpenAgentRoomThread("msg-1"); err != nil {
		t.Fatalf("OpenAgentRoomThread failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("THREAD REPLIES") || !tt.HasText("Verified zero cyclic imports") {
		t.Fatalf("expected THREAD REPLIES drawer with seeded reply, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Thread reply"); err != nil {
		t.Fatalf("Click('Thread reply') failed: %v", err)
	}
	tt.Type("Hello inside thread!")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Hello inside thread!") {
		t.Fatalf("expected thread reply submitted via Enter to render in THREAD REPLIES, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Thread reply"); err != nil {
		t.Fatalf("Click('Thread reply') for button submit failed: %v", err)
	}
	tt.Type("Second reply via button!")
	tt.Frame()
	if err := tt.Click("Send thread reply"); err != nil {
		t.Fatalf("Click('Send thread reply') failed: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Second reply via button!") {
		t.Fatalf("expected thread reply submitted via Send button to render in THREAD REPLIES, got %q", strings.Join(tt.Texts(), " | "))
	}
}
