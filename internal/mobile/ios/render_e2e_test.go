// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package ios_test

import (
	"context"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/mobile/ios"
)

func TestIOSNewChatIsolationAgentRoomsMarkdownAndGatesRendering(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorIOS,
	})
	featurekit.Mount("all", sh, nil, func() {})

	shell := ios.New(ios.Config{
		Adaptive: sh,
		AppConfig: appconfig.Config{
			SessionID: "ios-e2e-1",
		},
	})
	tt := ui.NewTester(shell.View, 393, 852)
	tt.SetVibrancy(true)
	settleMotion := func() {
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(20 * time.Millisecond)
		tt.Advance(800 * time.Millisecond)
		tt.SetPreferences(ui.Preferences{})
		tt.Frame()
	}
	settleMotion()

	// 1. Render rich Markdown + folded <think> + Vercel AI SDK data-metrics part in Chat.
	sh.Desktop.Transcript.AppendUser("Inspect workspace metrics")
	sh.Desktop.Transcript.BeginAssistantTurn(1)
	sh.Desktop.Transcript.ApplyCheckpoint(1, "ev-1", "<think>Checking build health</think>**All systems nominal.**\n\n```vercel-ui:metrics\n[{\"label\":\"FPS\",\"value\":120}]\n```")
	sh.Desktop.Transcript.CompleteTurn(1, "SESSION_TURN_OUTCOME_COMPLETED")
	tt.Frame()
	if !tt.HasText("Inspect workspace metrics") || !tt.HasText("AI SDK · data-metrics") {
		t.Fatalf("expected user prompt + Vercel AI SDK data-metrics card in iOS Chat, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 2. Click New Chat -> MUST open a fresh clean Hero Stage (never showing prior session turns).
	shell.NewChat()
	tt.Frame()
	if !tt.HasText("What can we build today?") {
		t.Fatalf("expected NewChat() to mount clean Hero Stage, got %q", strings.Join(tt.Texts(), " | "))
	}
	if tt.HasText("Inspect workspace metrics") {
		t.Fatalf("NewChat() leaked prior session message into fresh chat view")
	}

	// 3. Switch to Agent Rooms tab & verify immersive no-tab-bar mode, grouped feed,
	// Channel Details (HUMANS vs AI AGENTS), Direct Bot 1:1 channel, and dynamic @ popover.
	shell.SelectTab(mobile.TabRooms)
	settleMotion()
	// Verify bottom tab bar is hidden in immersive Rooms (Search orb & bottom tab bar absent).
	if _, foundSearchOrb := tt.Find("Search"); foundSearchOrb {
		t.Fatalf("expected bottom tab bar and Search orb hidden inside immersive Agent Rooms")
	}
	snap := featurekit.AgentRoomSnapshot()
	if snap.Available {
		// Top nav bar shows #workspace, Live Bot Avatar Stack (AI count), and Thread Preview Card for seeded msg-1.
		if !tt.HasText("#workspace") || !tt.HasText("AI") || !tt.HasText("1 reply") {
			t.Fatalf("expected #workspace header, AI stack, and seeded 1 reply thread preview card, got %q", strings.Join(tt.Texts(), " | "))
		}

		// Consecutive message grouping: post two messages from the same human author; both bodies render cleanly.
		if err := featurekit.PostAgentRoomMessage(snap.ActiveChannelID, "Hello @architect from iOS E2E", ""); err != nil {
			t.Fatalf("PostAgentRoomMessage 1 failed: %v", err)
		}
		if err := featurekit.PostAgentRoomMessage(snap.ActiveChannelID, "Follow-up grouped line with **Markdown**", ""); err != nil {
			t.Fatalf("PostAgentRoomMessage 2 failed: %v", err)
		}
		tt.Frame()
		if !tt.HasText("Hello @architect from iOS E2E") || !tt.HasText("Follow-up grouped line with") {
			t.Fatalf("expected both grouped Agent Room messages rendered, got %q", strings.Join(tt.Texts(), " | "))
		}

		// Context-aware @ mention popover: hidden by default, appears when typing "@ar", hides when cleared.
		if tt.HasText("AI AGENTS") {
			t.Fatalf("mention popover should remain hidden before typing @")
		}
		shell.SetRoomDraft("check @ar")
		tt.Frame()
		if !tt.HasText("AI AGENTS") || !tt.HasText("@architect") {
			t.Fatalf("expected @ mention popover grouped with AI AGENTS + @architect, got %q", strings.Join(tt.Texts(), " | "))
		}
		shell.SetRoomDraft("")
		tt.Frame()
		if dumpDir := os.Getenv("MARLIN_DUMP_DIR"); dumpDir != "" {
			saveTab := func(name string, tab mobile.Tab, prep func()) {
				shell.SelectTab(tab)
				if prep != nil {
					prep()
				}
				settleMotion()
				tt.SetScale(2)
				tt.Frame()
				if img := tt.Image(); img != nil {
					if f, err := os.Create(dumpDir + "/" + name + ".png"); err == nil {
						_ = png.Encode(f, img)
						_ = f.Close()
					}
				}
				tt.SetScale(1)
			}
			saveTab("rooms", mobile.TabRooms, func() { shell.SetRoomDraft("Review feature boundary") })
			saveTab("bento", mobile.TabHome, nil)
			saveTab("search", mobile.TabSearch, nil)
			saveTab("style", mobile.TabSettings, nil)
			shell.SetRoomDraft("")
			shell.SelectTab(mobile.TabRooms)
			settleMotion()
		}

		settleSheet := settleMotion

		// Open Channel Details sheet via Members pill -> verifies strict HUMANS vs AI AGENTS grouping.
		if err := tt.Click("Members"); err != nil {
			t.Fatalf("click Members pill failed: %v", err)
		}
		settleSheet()
		if !tt.HasText("HUMANS · 1") || !tt.HasText("AI AGENTS · 3") || !tt.HasText("APPROVER") {
			t.Fatalf("expected HUMANS · 1 + AI AGENTS · 3 + APPROVER in Channel Details Members sheet, got %q", strings.Join(tt.Texts(), " | "))
		}
		// Switch Segmented control to Roles tab -> verifies Tier-0 RBAC matrix.
		if err := tt.Click("Roles"); err != nil {
			t.Fatalf("click Roles segment failed: %v", err)
		}
		tt.Frame()
		if !tt.HasText("HUMAN APPROVER") || !tt.HasText("Gate Required") {
			t.Fatalf("expected Tier-0 RBAC Roles view, got %q", strings.Join(tt.Texts(), " | "))
		}
		if err := tt.Click("Close details"); err != nil {
			t.Fatalf("close details sheet failed: %v", err)
		}
		settleSheet()

		// Open Channels drawer -> verify CHANNELS vs AI BOTS · DIRECT and switch to 1:1 @architect DM.
		if err := tt.Click("Channels"); err != nil {
			t.Fatalf("click Channels drawer button failed: %v", err)
		}
		settleSheet()
		if !tt.HasText("CHANNELS") || !tt.HasText("AI BOTS · DIRECT") {
			t.Fatalf("expected CHANNELS and AI BOTS · DIRECT sections in drawer, got %q", strings.Join(tt.Texts(), " | "))
		}
		if err := tt.Click("Architect"); err != nil {
			t.Fatalf("click Architect 1:1 DM row failed: %v", err)
		}
		settleSheet()
		if !tt.HasText("@Architect") {
			t.Fatalf("expected 1:1 Direct Bot header @Architect, got %q", strings.Join(tt.Texts(), " | "))
		}
		_ = featurekit.SelectAgentRoomChannel("ch-workspace")
	}

	// 4. Enqueue a PromptGate, verify it renders in TabGates, and verify Approve resolves it cleanly.
	if err := sh.Desktop.PromptGate.Queue.Enqueue(promptgate.PromptRequest{
		ID:     "gate-ios-e2e-1",
		Title:  "Run cargo test",
		Prompt: "Approve running unit tests on workspace?",
	}); err != nil {
		t.Fatalf("enqueue gate: %v", err)
	}
	shell.SelectTab(mobile.TabGates)
	tt.Frame()
	if !tt.HasText("Run cargo test") || !tt.HasText("Approve") {
		t.Fatalf("expected pending gate card with Approve button, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Approve"); err != nil {
		t.Fatalf("click Approve failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("All Clear") {
		t.Fatalf("expected gate cleared to All Clear after Approve, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 5. MVP-2: Verify native mobile.PageView horizontal touch swipe advances
	// strictly 1 page (Chat -> immersive Rooms, hiding the bottom tab bar) with
	// smooth native spring physics and zero teleportation.
	shell.SelectTab(mobile.TabChat)
	settleMotion()
	tt.FingerDown(340, 420)
	tt.FingerMove(180, 420)
	tt.FingerMove(40, 420)
	tt.FingerUp(40, 420)
	settleMotion()
	if _, foundSearch := tt.Find("Search"); foundSearch {
		t.Fatalf("expected horizontal touch swipe from Chat to land strictly on immersive Rooms")
	}
	if !tt.HasText("#workspace") {
		t.Fatalf("expected horizontal touch swipe to advance strictly 1 page to Rooms (#workspace), got %q", strings.Join(tt.Texts(), " | "))
	}

	// 5a. Slash command popover (/ta) + executing /task creates a real shared task.
	shell.SetRoomDraft("/ta")
	tt.Frame()
	if !tt.HasText("ROOM COMMANDS") || !tt.HasText("/task") {
		t.Fatalf("expected / slash command popover with /task, got %q", strings.Join(tt.Texts(), " | "))
	}
	shell.SubmitRoomDraft("ch-workspace", "/task Ship MVP-2 live multi-agent rooms")
	tt.Frame()
	foundTask := false
	for _, tk := range featurekit.AgentRoomTasks() {
		if strings.Contains(tk.Title, "Ship MVP-2 live multi-agent rooms") {
			foundTask = true
			break
		}
	}
	if !foundTask {
		t.Fatalf("expected /task slash command to create real channel task")
	}

	// 5b. Live @architect mention triggers Thinking indicator -> real bot reply in channel.
	gateCh := make(chan struct{})
	doneCh := make(chan struct{})
	featurekit.SetRoomAgentResponder(func(ctx context.Context, req featurekit.RoomAgentRequest) (string, error) {
		<-gateCh
		defer close(doneCh)
		return "**Architect Review**: Boundary invariants verified for `" + req.BotHandle + "`.", nil
	})
	defer featurekit.SetRoomAgentResponder(nil)

	if err := featurekit.PostAgentRoomMessage("ch-workspace", "@architect audit boundary", ""); err != nil {
		t.Fatalf("PostAgentRoomMessage @architect failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Architect is thinking…") {
		t.Fatalf("expected live D3 typing indicator 'Architect is thinking…', got %q", strings.Join(tt.Texts(), " | "))
	}
	close(gateCh)
	<-doneCh
	// Wait briefly for goroutine to post threaded reply under the user's @architect message.
	deadline := time.Now().Add(2 * time.Second)
	var threadedMsgID string
	for time.Now().Before(deadline) {
		snap := featurekit.AgentRoomSnapshot()
		for _, m := range snap.Messages {
			if strings.Contains(m.Body, "@architect audit boundary") && m.ThreadCount >= 1 {
				threadedMsgID = m.ID
				break
			}
		}
		if threadedMsgID != "" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if threadedMsgID == "" {
		t.Fatalf("expected Architect live reply threaded under @architect audit boundary")
	}
	if err := featurekit.OpenAgentRoomThread(threadedMsgID); err != nil {
		t.Fatalf("OpenAgentRoomThread(%s) failed: %v", threadedMsgID, err)
	}
	tsnap := featurekit.AgentRoomSnapshot()
	foundReply := false
	for _, tm := range tsnap.ThreadMessages {
		if strings.Contains(tm.Body, "Architect Review") {
			foundReply = true
			break
		}
	}
	featurekit.CloseAgentRoomThread()
	if !foundReply {
		t.Fatalf("expected Architect live reply inside thread messages, got %+v", tsnap.ThreadMessages)
	}

	// 5c. Inline Tier-0 Bot Gate Embed directly inside Rooms feed -> 1-tap Approve resolves gate.
	if err := sh.Desktop.PromptGate.Queue.Enqueue(promptgate.PromptRequest{
		ID:     "gate-room-inline-1",
		Title:  "Deploy schema migration",
		Prompt: "Allow @security to apply verified schema change?",
	}); err != nil {
		t.Fatalf("enqueue inline room gate: %v", err)
	}
	if err := featurekit.PostAgentRoomGateEmbed("ch-workspace", "agent-security", "Requesting human Tier-0 sign-off:", "gate-room-inline-1"); err != nil {
		t.Fatalf("PostAgentRoomGateEmbed failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("APPROVAL REQUIRED") || !tt.HasText("Deploy schema migration") {
		t.Fatalf("expected inline Bot Gate Embed in Rooms feed, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Approve"); err != nil {
		t.Fatalf("click inline Gate Embed Approve failed: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Gate Resolved · Tier-0 Audited") {
		t.Fatalf("expected inline Gate Embed to transition to Gate Resolved receipt, got %q", strings.Join(tt.Texts(), " | "))
	}
}
