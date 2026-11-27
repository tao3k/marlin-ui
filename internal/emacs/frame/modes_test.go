// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"strings"
	"testing"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
)

func TestWorkspaceIsolationMultiChatBuffersAndEmacsWindowClone(t *testing.T) {
	f := New(Config{AppConfig: appconfig.Config{SessionID: "modes-ws-test"}})

	// 1. Creating an unnamed workspace picks lowest unused ws-2 (never jumping to ws-5)
	// and isolates its own *chat:ws-2* buffer without hijacking main's *conversation*.
	ws := f.CreateWorkspace("", "")
	if ws.Name != "ws-2" || ws.ActiveBuffer != buffer.Dashboard {
		t.Fatalf("expected clean ws-2 starting on *Dashboard*, got %+v", ws)
	}
	hasOwnChat := false
	for _, b := range ws.Buffers {
		if b == buffer.ID("*chat:ws-2*") {
			hasOwnChat = true
		}
		if b == buffer.Conversation {
			t.Fatalf("new workspace must not hijack main's singleton *conversation*: %+v", ws.Buffers)
		}
	}
	if !hasOwnChat {
		t.Fatalf("expected new workspace to receive its own *chat:ws-2* buffer: %+v", ws.Buffers)
	}

	// 2. NewChatBuffer spawns a multi-instance *chat:<name>* governed by marlin-chat-mode.
	chatID := f.NewChatBuffer("refactor-auth")
	if chatID != buffer.ID("*chat:refactor-auth*") || f.ActiveBuffer() != chatID {
		t.Fatalf("expected active buffer *chat:refactor-auth*, got %s (active=%s)", chatID, f.ActiveBuffer())
	}
	eff := f.EffectiveModeStack(chatID)
	if eff.HostMode != "marlin-chat-mode" || !eff.HasSegment["deep-think"] || !eff.HasSegment["composer-dock"] {
		t.Fatalf("expected *chat:refactor-auth* to inherit marlin-chat-mode segments, got %+v", eff)
	}

	// 3. CloneCurrentWindow creates an authentic Emacs sibling window viewing the SAME buffer.
	f.CloneCurrentWindow()
	wins := f.WorkspaceSnapshot().Active.Windows()
	if len(wins) != 2 || wins[0].Buffer != chatID || wins[1].Buffer != chatID {
		t.Fatalf("expected CloneCurrentWindow to create linked 2-pane view of %s, got %+v", chatID, wins)
	}

	// 4. Buffer picker (Tab Bar "+" & SPC b b) surfaces dynamic *chat:* buffers & mode creators.
	cands := f.bufferCandidates("")
	foundNewChat := false
	foundClone := false
	foundInstance := false
	for _, c := range cands {
		if c.ID == "action:new-chat-buffer" {
			foundNewChat = true
		}
		if c.ID == "action:clone-window" {
			foundClone = true
		}
		if c.ID == string(chatID) {
			foundInstance = true
		}
	}
	if !foundNewChat || !foundClone || !foundInstance {
		t.Fatalf("expected buffer picker to include new-chat, clone-window, and %s (got %d candidates)", chatID, len(cands))
	}
}

func TestPolymodeSpanSwitchingInChatAndMagitBuffers(t *testing.T) {
	f := New(Config{AppConfig: appconfig.Config{SessionID: "polymode-test"}})
	chatID := f.NewChatBuffer("code-review")

	// Switch Polymode span inside *chat:code-review* into an AI diff hunk ->
	// Modeline switches to Marlin-Chat[Diff·Aitia] and 's' binds to cmd:magit-stage!
	var hookFired int
	f.ModeRegistry().AddHook(modes.HookPolymodeSwitchSpan, func(ev modes.HookEvent) {
		if ev.BufferID == string(chatID) && ev.InnerMode == "magit-diff-inner-mode" {
			hookFired++
		}
	})
	f.FocusPolymodeSpan(chatID, modes.SpanDiffHunk)
	if hookFired != 1 {
		t.Fatalf("expected polymode-switch-span hook to fire once, got %d", hookFired)
	}
	ml := f.buildModelineStatusForBuffer(f.loadSnapshot(), chatID, "gpt-5", "s1")
	if ml.MajorMode != "Marlin-Chat[Diff·Aitia]" {
		t.Fatalf("expected Modeline MajorMode Marlin-Chat[Diff·Aitia], got %q", ml.MajorMode)
	}
	if b, ok := f.LookupModeKey(chatID, "s"); !ok || b.Command != "cmd:magit-stage" {
		t.Fatalf("expected Polymode diff-hunk key 's' -> cmd:magit-stage, got %+v %v", b, ok)
	}

	// Navigating *magit* onto a hunk row automatically activates Magit-Status[Hunk·Aitia]!
	f.OpenMagit()
	view := f.loadSnapshot().Magit
	hunkIdx := -1
	for i, row := range view.Rows {
		if row.Kind == buffer.MagitRowHunk {
			hunkIdx = i
			break
		}
	}
	if hunkIdx < 0 {
		t.Fatalf("expected at least one hunk row in *magit* preview")
	}
	f.stepMagitCursor(hunkIdx - view.Cursor)
	magitEff := f.EffectiveModeStack(buffer.Magit)
	if !strings.Contains(magitEff.DisplayTitle, "Hunk·Aitia") {
		t.Fatalf("expected *magit* hunk cursor to activate Polymode Hunk·Aitia, got %q", magitEff.DisplayTitle)
	}
}
