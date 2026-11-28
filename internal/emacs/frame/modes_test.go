// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
	"github.com/tao3k/marlin-ui/internal/hostlink"
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
	f.SelectBuffer(buffer.Magit)
	f.commitMagitSnapshot(hostlink.DemoMagitSnapshot(), "")
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
	ApplyTestSchemeKeymap(f)

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

func TestWindowRegistersOtherWindowReadOnlyGuardHistoryAndKeyboardIbuffer(t *testing.T) {
	f := New(Config{InitialThemeID: "doom-one"})
	defer f.Close()

	// 1. C-x 4 b (switch-to-buffer-other-window) auto-splits single window without overwriting original.
	f.SelectBuffer(buffer.Conversation)
	f.SelectBufferOtherWindow(buffer.Magit)
	ws := f.WorkspaceSnapshot().Active
	if ws.WindowCount() != 2 {
		t.Fatalf("expected C-x 4 b to create 2 windows, got %d", ws.WindowCount())
	}
	if ws.FocusedBuffer() != buffer.Magit {
		t.Fatalf("expected C-x 4 b to focus *magit*, got %s", ws.FocusedBuffer())
	}

	// 2. C-x r w / C-x r j (Window Registers) save & restore split tree and record winner-mode undo.
	if !f.SaveWindowRegister("audit") {
		t.Fatalf("expected SaveWindowRegister('audit') to succeed")
	}
	f.DeleteOtherWindows()
	if f.WorkspaceSnapshot().Active.WindowCount() != 1 {
		t.Fatalf("expected 1 window after DeleteOtherWindows")
	}
	if !f.JumpWindowRegister("audit") {
		t.Fatalf("expected JumpWindowRegister('audit') to succeed")
	}
	if f.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected 2 windows restored by register 'audit', got %d", f.WorkspaceSnapshot().Active.WindowCount())
	}
	f.ListWindowRegisters()
	if hd := f.currentHelpIntrospection(buffer.Magit); hd == nil || hd.KeyDoc == nil || hd.KeyDoc.Tier != "REGISTERS" {
		t.Fatalf("expected C-x r l to populate REGISTERS card in *Help*, got %+v", hd)
	}

	// 3. C-x 4 0 (kill-buffer-and-window) kills active buffer AND closes its split window.
	chatTmp := f.NewChatBuffer("ephemeral")
	f.SelectBufferOtherWindow(chatTmp)
	if !f.KillBufferAndWindow() {
		t.Fatalf("expected KillBufferAndWindow to succeed")
	}
	if f.WorkspaceSnapshot().Active.WindowCount() != 1 {
		t.Fatalf("expected KillBufferAndWindow to collapse 2 windows -> 1, got %d", f.WorkspaceSnapshot().Active.WindowCount())
	}

	// 4. read-only-mode blocks EnterInsert on chat buffers until disabled.
	f.SelectBuffer(buffer.Conversation)
	f.ToggleMinorModeOnActive("read-only-mode")
	f.EnterInsert()
	if f.KeyMode() != meow.ModeNormal {
		t.Fatalf("expected read-only-mode to block EnterInsert on *conversation*, got %v", f.KeyMode())
	}
	f.ToggleMinorModeOnActive("read-only-mode")
	f.EnterInsert()
	if f.KeyMode() != meow.ModeInsert {
		t.Fatalf("expected EnterInsert to succeed once read-only-mode is disabled, got %v", f.KeyMode())
	}

	// 5. M-x savehist MRU floats recently executed commands to top of Vertico.
	f.recordRecentCommand("list-registers")
	cands := f.commandCandidates("")
	if len(cands) == 0 || cands[0].Name != "list-registers" {
		t.Fatalf("expected MRU command 'list-registers' at index 0 of M-x, got %+v", cands[:min(3, len(cands))])
	}

	// 6. Keyboard-first *Ibuffer*: d marks cursor row & advances, u unmarks, RET selects.
	_ = f.NewChatBuffer("kbd-ibuf")
	f.OpenIbuffer()
	if !f.ibufferMarkAtCursor(true) {
		t.Fatalf("expected d (ibufferMarkAtCursor) to mark cursor row in *Ibuffer*")
	}
	if f.IbufferMarkedCount() != 1 {
		t.Fatalf("expected 1 marked buffer after d, got %d", f.IbufferMarkedCount())
	}
	if !f.ibufferSelectAtCursor() {
		t.Fatalf("expected RET (ibufferSelectAtCursor) to switch to cursor row")
	}
}

func TestEmbarkLossageRepeatUniversalArgZoomTransposeRenameAndBury(t *testing.T) {
	f := New(Config{InitialThemeID: "doom-one"})
	defer f.Close()

	// 1. Window Zoom Toggle (SPC w m / C-x w m) & Transpose Orientation (SPC w T / C-x w t).
	f.SelectBuffer(buffer.Conversation)
	f.SelectBufferOtherWindow(buffer.Magit)
	if f.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected 2 windows before zoom")
	}
	if !f.TransposeWindowLayout() {
		t.Fatalf("expected TransposeWindowLayout to succeed on split")
	}
	if f.WorkspaceSnapshot().Active.SplitMode != "horizontal" {
		t.Fatalf("expected vertical split transposed to horizontal, got %s", f.WorkspaceSnapshot().Active.SplitMode)
	}
	if !f.ZoomWindowToggle() {
		t.Fatalf("expected ZoomWindowToggle (zoom in) to succeed")
	}
	if f.WorkspaceSnapshot().Active.WindowCount() != 1 {
		t.Fatalf("expected 1 window while zoomed, got %d", f.WorkspaceSnapshot().Active.WindowCount())
	}
	if !f.ZoomWindowToggle() {
		t.Fatalf("expected ZoomWindowToggle (restore) to succeed")
	}
	if f.WorkspaceSnapshot().Active.WindowCount() != 2 {
		t.Fatalf("expected 2 windows restored after second ZoomWindowToggle, got %d", f.WorkspaceSnapshot().Active.WindowCount())
	}

	// 2. Live rename-buffer (C-x x r) preserves chat session binding & updates tabs/windows.
	cOrig := f.NewChatBuffer("scratch-1")
	origSess := f.ChatSessionFor(cOrig)
	renamed, ok := f.RenameActiveBuffer("auth-refactor")
	if !ok || renamed != buffer.ID("*chat:auth-refactor*") {
		t.Fatalf("expected RenameActiveBuffer -> *chat:auth-refactor*, got %s (%v)", renamed, ok)
	}
	if gotSess := f.ChatSessionFor(renamed); gotSess != origSess {
		t.Fatalf("expected renamed buffer to preserve session %q, got %q", origSess, gotSess)
	}

	// 3. bury-buffer (SPC b z / C-c z) rotates active buffer to tail of ring.
	if !f.BuryActiveBuffer() {
		t.Fatalf("expected BuryActiveBuffer to succeed with multiple buffers open")
	}
	bufs := f.WorkspaceSnapshot().Active.Buffers
	if bufs[len(bufs)-1] != renamed {
		t.Fatalf("expected buried buffer %s at tail of workspace buffers, got %v", renamed, bufs)
	}

	// 4. Embark contextual actions adapt to target at point (Magit vs Polymode span vs buffer).
	f.SelectBuffer(buffer.Magit)
	magitCands := f.embarkCandidates("")
	if len(magitCands) == 0 || magitCands[0].ID != "emb-stage" {
		t.Fatalf("expected Embark in *magit* to surface stage action first, got %+v", magitCands)
	}
	f.SelectBuffer(renamed)
	f.HandlePolymodeSpanFocus("code-block", "rust", "fn main() {}")
	spanCands := f.embarkCandidates("")
	if len(spanCands) == 0 || spanCands[0].ID != "emb-copy" {
		t.Fatalf("expected Embark on Polymode code span to surface copy action first, got %+v", spanCands)
	}

	// 5. view-lossage (C-h l), C-u universal-argument (4 -> 16), and repeat-last-command.
	if n := f.TriggerUniversalArg(); n != 4 {
		t.Fatalf("expected first C-u -> 4, got %d", n)
	}
	if n := f.TriggerUniversalArg(); n != 16 {
		t.Fatalf("expected second C-u -> 16, got %d", n)
	}
	f.dispatchModeAction("cmd:next-buffer")
	if f.UniversalArg() != 0 {
		t.Fatalf("expected C-u consumed after repeatable command, got %d", f.UniversalArg())
	}
	if !f.RepeatLastCommand() {
		t.Fatalf("expected RepeatLastCommand to repeat cmd:next-buffer")
	}
	f.ViewLossage()
	if hd := f.currentHelpIntrospection(renamed); hd == nil || hd.KeyDoc == nil || hd.KeyDoc.Tier != "LOSSAGE" || len(hd.KeyDoc.Aliases) == 0 {
		t.Fatalf("expected C-h l view-lossage to record and display lossage ring, got %+v", hd)
	}
}

func TestVerticoFindFileProjectFindFileRecentfAndHonestLiveMagit(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	targetFile := filepath.Join(tmp, "pkg", "hello.go")
	if err := os.WriteFile(targetFile, []byte("package pkg\n\nfunc Hello() string { return \"world\" }\n"), 0o644); err != nil {
		t.Fatalf("write hello.go: %v", err)
	}

	f := New(Config{InitialThemeID: "doom-one"})
	defer f.Close()

	// 1. Honest Magit: when disconnected from marlin-app-server, NEVER fabricate DemoMode files!
	f.SelectBuffer(buffer.Magit)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mv := f.loadSnapshot().Magit
		if mv != nil && !mv.Loading && mv.Error != "" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	mv := f.loadSnapshot().Magit
	if mv == nil || mv.Snapshot.DemoMode {
		t.Fatalf("expected *magit* to NEVER show fabricated DemoMode snapshot, got %+v", mv)
	}
	if mv.Error == "" {
		t.Fatalf("expected honest offline connection error in *magit*, got empty error")
	}

	// 2. Vertico find-file (C-x C-f / SPC f f): real directory listing + directory drill-down + open file!
	f.FindFileEnterDir(tmp)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.findFileCandidates("pkg")) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	dirCands := f.findFileCandidates("pkg")
	if len(dirCands) == 0 || !dirCands[0].KeepOpen || dirCands[0].Name != "pkg/" {
		t.Fatalf("expected pkg/ directory candidate with KeepOpen=true, got %+v", dirCands)
	}
	// Descend into pkg/ (vertico-directory-enter) and open hello.go.
	dirCands[0].Run()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.findFileCandidates("hello")) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	fileCands := f.findFileCandidates("hello")
	if len(fileCands) == 0 || fileCands[0].KeepOpen || fileCands[0].Name != "hello.go" {
		t.Fatalf("expected hello.go file candidate, got %+v", fileCands)
	}
	fileCands[0].Run()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fp := f.currentFilePreview(); fp != nil && strings.Contains(fp.Content, "func Hello()") {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	fp := f.currentFilePreview()
	if fp == nil || !strings.Contains(fp.Content, "func Hello()") {
		t.Fatalf("expected opened real disk file content in *Preview*, got %+v", fp)
	}

	// 3. recentf (C-x C-r / SPC f r) recorded the opened real file!
	recentCands := f.recentFileCandidates("hello")
	if len(recentCands) == 0 || recentCands[0].Name != "hello.go" {
		t.Fatalf("expected hello.go in recentf candidates, got %+v", recentCands)
	}

	// 4. project-find-file (C-c p f / SPC p f) indexes real repo files under workspace ProjectRoot!
	f.CreateWorkspace("tmp-repo", tmp)
	f.OpenProjectFindFile()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.projectFileCandidates("pkg hello")) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	projCands := f.projectFileCandidates("pkg hello")
	if len(projCands) == 0 || projCands[0].Name != "pkg/hello.go" {
		t.Fatalf("expected Orderless project-find-file 'pkg hello' -> pkg/hello.go, got %+v", projCands)
	}

	// 5. Unified consult-buffer (SPC , / C-x b) surfaces recentf virtual files alongside open buffers.
	cbCands := f.bufferCandidates("hello.go")
	if len(cbCands) == 0 || cbCands[0].Annotation != "recentf" {
		t.Fatalf("expected consult-buffer virtual recentf candidate for hello.go, got %+v", cbCands)
	}

	// 6. consult-line (SPC l / C-c l) finds and jumps to exact line inside opened file preview.
	f.OpenConsultLine()
	lineCands := f.consultLineCandidates("func Hello")
	if len(lineCands) == 0 || lineCands[0].Annotation != "L3" {
		t.Fatalf("expected consult-line match on L3 for 'func Hello', got %+v", lineCands)
	}
	lineCands[0].Run()
	f.mu.Lock()
	jumpedLine := f.filePreview.Line
	f.mu.Unlock()
	if jumpedLine != 3 {
		t.Fatalf("expected consult-line jump to set preview line=3, got %d", jumpedLine)
	}

	// 7. project-switch-project (SPC p p) rebinds active workspace ProjectRoot cleanly.
	altRepo := t.TempDir()
	f.SwitchActiveProjectRoot(altRepo)
	if got := f.activeWorkspaceProjectRoot(); got != altRepo {
		t.Fatalf("expected SwitchActiveProjectRoot -> %q, got %q", altRepo, got)
	}

	// 8. Emacs bookmarks (C-x r m / C-x r b) save and jump to file:line anchors.
	f.SaveBookmark("hello-fn")
	bmCands := f.bookmarkJumpCandidates("hello")
	if len(bmCands) == 0 || bmCands[0].Name != "hello-fn" || bmCands[0].Annotation != "bookmark · L3" {
		t.Fatalf("expected saved bookmark hello-fn at L3, got %+v", bmCands)
	}
}
