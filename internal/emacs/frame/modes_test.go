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

func TestDisplayBufferAlistDescribeModePerBufferSessionsAndIbufferLifecycle(t *testing.T) {
	f := New(Config{AppConfig: appconfig.Config{SessionID: "main-sess"}})

	// 1. Per-buffer SessionID isolation across multiple *chat:<name>* buffers.
	chatA := f.NewChatBuffer("alpha")
	chatB := f.NewChatBuffer("beta")
	if f.ChatSessionFor(chatA) == f.ChatSessionFor(chatB) {
		t.Fatalf("expected distinct sessions for %s (%q) and %s (%q)", chatA, f.ChatSessionFor(chatA), chatB, f.ChatSessionFor(chatB))
	}

	// 2. Interactive Polymode span focus + Emacs describe-mode (C-h m) reflection.
	f.SelectBuffer(chatA)
	f.HandlePolymodeSpanFocus("diff-hunk", "diff", "@@ -1 +1 @@\n-old\n+new")
	f.DescribeCurrentMode()
	hi := f.currentHelpIntrospection(buffer.Help)
	if hi == nil || hi.BufferName != string(chatA) || hi.InnerModeID != "magit-diff-inner-mode" {
		t.Fatalf("expected describe-mode to reflect %s with magit-diff-inner-mode, got %+v", chatA, hi)
	}
	if len(hi.LocalBindings) == 0 || len(hi.PolymodeRules) < 3 {
		t.Fatalf("expected describe-mode to include live local bindings and Polymode rules, got %+v", hi)
	}

	// 3. *Ibuffer* groups open buffers by Major Mode, and KillBuffer / StepBuffer work cleanly.
	entries := f.BuildIbufferEntries(f.loadSnapshot())
	if len(entries) < 3 {
		t.Fatalf("expected >= 3 buffers in *Ibuffer*, got %d", len(entries))
	}
	if !f.StepBuffer(1) {
		t.Fatalf("expected StepBuffer(1) to cycle buffers in workspace")
	}
	if !f.KillBuffer(chatB) {
		t.Fatalf("expected KillBuffer(%s) to succeed", chatB)
	}
	for _, b := range f.WorkspaceSnapshot().Active.Buffers {
		if b == chatB {
			t.Fatalf("expected %s to be removed from workspace after KillBuffer", chatB)
		}
	}

	// 4. display-buffer-alist: *Preview* (DisplaySplitRight) auto-splits right cleanly.
	f.DeleteOtherWindows()
	f.SelectBuffer(chatA)
	// Ensure *Preview* is not intercepted by popup rule by testing OpenBufferWithAction directly.
	_, splitSnap := f.wsMgr.OpenBufferWithAction(buffer.ID("*magit-diff*"), "split-right")
	if splitSnap.Active.WindowCount() != 2 {
		t.Fatalf("expected display-buffer-alist split-right to open 2nd window, got %d windows", splitSnap.Active.WindowCount())
	}
}

func TestEmacsSelfDocMinorModeHooksOrderlessMxAndIbufferBatchKill(t *testing.T) {
	f := New(Config{InitialThemeID: "doom-one"})
	defer f.Close()

	// 1. C-h k (describe-key) resolves local Magit bindings ahead of global bindings.
	f.SelectBuffer(buffer.Magit)
	f.DescribeKey("s")
	hd := f.currentHelpIntrospection(buffer.Magit)
	if hd == nil || hd.KeyDoc == nil || hd.KeyDoc.Tier != "MAJOR" || hd.KeyDoc.Command != "cmd:magit-stage" {
		t.Fatalf("expected C-h k 's' in *magit* to resolve [MAJOR] cmd:magit-stage, got %+v", hd)
	}

	// C-h k also resolves global chords with equivalent Doom/Emacs aliases.
	f.DescribeKey("C-x 3")
	hd = f.currentHelpIntrospection(buffer.Magit)
	if hd == nil || hd.KeyDoc == nil || hd.KeyDoc.Tier != "GLOBAL" || hd.KeyDoc.Command != "cmd:win-split-right" {
		t.Fatalf("expected C-h k 'C-x 3' to resolve [GLOBAL] cmd:win-split-right, got %+v", hd)
	}
	if len(hd.KeyDoc.Aliases) < 2 {
		t.Fatalf("expected paired Doom/Emacs aliases for C-x 3, got %v", hd.KeyDoc.Aliases)
	}

	// 2. C-h f (describe-function) & C-h b (describe-bindings) populate *Help* cards.
	f.DescribeFunction("dedicated-window-mode")
	hd = f.currentHelpIntrospection(buffer.Magit)
	if hd == nil || hd.KeyDoc == nil || hd.KeyDoc.Tier != "MINOR-MODE" {
		t.Fatalf("expected C-h f dedicated-window-mode -> MINOR-MODE doc, got %+v", hd)
	}
	f.DescribeBindings()
	hd = f.currentHelpIntrospection(buffer.Magit)
	if hd == nil || len(hd.GlobalBindings) < 10 {
		t.Fatalf("expected C-h b to list live global bindings, got %d", len(hd.GlobalBindings))
	}

	// 3. Live Minor Mode Hooks: toggling dedicated-window-mode pins/unpins focused window,
	//    and toggling read-only-mode updates EffectiveStack.ReadOnly on *conversation*.
	f.SelectBuffer(buffer.Conversation)
	if f.EffectiveModeStack(buffer.Conversation).ReadOnly {
		t.Fatalf("expected *conversation* to start writable")
	}
	f.ToggleMinorModeOnActive("read-only-mode")
	if !f.EffectiveModeStack(buffer.Conversation).ReadOnly {
		t.Fatalf("expected read-only-mode toggle to mark *conversation* ReadOnly")
	}
	f.ToggleMinorModeOnActive("dedicated-window-mode")
	if !f.WorkspaceSnapshot().Active.FocusedLeaf().Dedicated {
		t.Fatalf("expected enter-minor-mode hook for dedicated-window-mode to pin active window")
	}
	f.ToggleMinorModeOnActive("dedicated-window-mode")
	if f.WorkspaceSnapshot().Active.FocusedLeaf().Dedicated {
		t.Fatalf("expected exit-minor-mode hook for dedicated-window-mode to unpin active window")
	}

	// 4. Orderless M-x surfaces live Minor Modes and mode-local commands.
	cands := f.commandCandidates("dedicated mode")
	if len(cands) == 0 {
		t.Fatalf("expected Orderless M-x query 'dedicated mode' to match dedicated-window-mode")
	}

	// 5. *Ibuffer* mark, filter, and batch kill marked buffers.
	c1 := f.NewChatBuffer("batch-1")
	c2 := f.NewChatBuffer("batch-2")
	f.SetIbufferFilter("Marlin-Chat")
	if f.IbufferFilter() != "Marlin-Chat" {
		t.Fatalf("expected IbufferFilter = Marlin-Chat, got %q", f.IbufferFilter())
	}
	f.ToggleIbufferMark(c1)
	f.ToggleIbufferMark(c2)
	if f.IbufferMarkedCount() != 2 {
		t.Fatalf("expected 2 marked buffers in *Ibuffer*, got %d", f.IbufferMarkedCount())
	}
	killed := f.KillMarkedIbuffer()
	if killed != 2 {
		t.Fatalf("expected KillMarkedIbuffer to kill 2 buffers, got %d", killed)
	}
	if f.IbufferMarkedCount() != 0 {
		t.Fatalf("expected marks cleared after KillMarkedIbuffer, got %d", f.IbufferMarkedCount())
	}
}
