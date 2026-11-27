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
	// Group channel @mention replies nest cleanly inside the triggering message's thread.
	deadline := time.Now().Add(2 * time.Second)
	var userMsgID string
	for time.Now().Before(deadline) {
		snap := featurekit.AgentRoomSnapshot()
		for _, m := range snap.Messages {
			if strings.Contains(m.Body, "@reviewer verify desktop parity") && m.ThreadCount >= 1 {
				userMsgID = m.ID
				break
			}
		}
		if userMsgID != "" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if userMsgID == "" {
		t.Fatalf("expected @reviewer reply threaded under user message (ThreadCount >= 1)")
	}
	if err := featurekit.OpenAgentRoomThread(userMsgID); err != nil {
		t.Fatalf("OpenAgentRoomThread(%s) failed: %v", userMsgID, err)
	}
	tt.Frame()
	if !tt.HasText("Reviewer Audit") {
		t.Fatalf("expected @reviewer reply rendered in THREAD REPLIES drawer, got %q", strings.Join(tt.Texts(), " | "))
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

	// Verify thread-scoped @mention autocomplete + thread-only bot reply (never leaks to main timeline)
	// + full thread root context passed to the bot responder, including CJK punctuation after @mention.
	var gotThreadReq featurekit.RoomAgentRequest
	threadBotDone := make(chan struct{})
	featurekit.SetRoomAgentResponder(func(_ context.Context, req featurekit.RoomAgentRequest) (string, error) {
		gotThreadReq = req
		defer close(threadBotDone)
		return "Architect Thread-Only Reply for `" + req.ThreadRootID + "`.", nil
	})
	if err := tt.Click("Thread reply"); err != nil {
		t.Fatalf("Click('Thread reply') for @ mention popover failed: %v", err)
	}
	tt.Type("@arc")
	tt.Frame()
	if _, ok := tt.Find("Thread mention @architect"); !ok {
		t.Fatalf("expected thread @ mention autocomplete popover to show 'Thread mention @architect', got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Thread mention @architect"); err != nil {
		t.Fatalf("Click('Thread mention @architect') failed: %v", err)
	}
	tt.Frame()
	tt.Type("，请只在这个线程里回复")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	select {
	case <-threadBotDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for thread-scoped @architect reply")
	}
	if gotThreadReq.ThreadRootID != "msg-1" {
		t.Fatalf("expected ThreadRootID=msg-1, got %q", gotThreadReq.ThreadRootID)
	}
	if !strings.Contains(gotThreadReq.RecentContext, "Shared context synchronized with Rust AppServer") {
		t.Fatalf("expected bot RecentContext to include Thread Root message, got %q", gotThreadReq.RecentContext)
	}

	snapAfterThread := featurekit.AgentRoomSnapshot()
	foundInThread := false
	for _, tm := range snapAfterThread.ThreadMessages {
		if strings.Contains(tm.Body, "Architect Thread-Only Reply") {
			foundInThread = true
		}
	}
	if !foundInThread {
		t.Fatalf("expected @architect reply inside ThreadMessages, got %+v", snapAfterThread.ThreadMessages)
	}
	for _, mm := range snapAfterThread.Messages {
		if strings.Contains(mm.Body, "Architect Thread-Only Reply") {
			t.Fatalf("thread-scoped @architect reply leaked into main timeline Messages: %+v", mm)
		}
	}

	// Verify Main Composer '@' autocomplete & '/' slash command popovers work on Desktop (1:1 with Mobile).
	featurekit.CloseAgentRoomThread()
	tt.Frame()
	if err := tt.Click("Room message"); err != nil {
		t.Fatalf("Click('Room message') failed: %v", err)
	}
	tt.Type("@rev")
	tt.Frame()
	if _, ok := tt.Find("Mention @reviewer"); !ok {
		t.Fatalf("expected main composer @ mention autocomplete popover to show 'Mention @reviewer', got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Mention @reviewer"); err != nil {
		t.Fatalf("Click('Mention @reviewer') failed: %v", err)
	}
	tt.Frame()

	// Verify Doom Zen Writeroom hides the left CHANNELS/AGENTS rail and centers #workspace cleanly.
	featurekit.SetRoomWriteroom(true)
	defer featurekit.SetRoomWriteroom(false)
	tt.Frame()
	if tt.HasText("CHANNELS") || tt.HasText("AI BOTS · DIRECT") || !tt.HasText("Writeroom · Exit") {
		t.Fatalf("expected Room Writeroom to hide left rail and show 'Writeroom · Exit', got %q", strings.Join(tt.Texts(), " | "))
	}
	featurekit.SetRoomWriteroom(false)
	tt.Frame()
	if !tt.HasText("CHANNELS") || tt.HasText("Writeroom · Exit") {
		t.Fatalf("expected exiting Room Writeroom to restore CHANNELS rail, got %q", strings.Join(tt.Texts(), " | "))
	}
}

func TestExtractRoomMentionsHandlesASCIIAndCJKPunctuation(t *testing.T) {
	got := featurekit.ExtractRoomMentions("@Architect，帮我看一下 @reviewer：以及 @security请检查")
	if len(got) != 3 || strings.ToLower(got[0]) != "architect" || strings.ToLower(got[1]) != "reviewer" || strings.ToLower(got[2]) != "security" {
		t.Fatalf("expected [architect reviewer security], got %v", got)
	}
}
