// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"
	"time"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
)

// LossageEntry records one recent command/keystroke event for Emacs `view-lossage` (`C-h l`).
type LossageEntry struct {
	At     time.Time
	Buffer buffer.ID
	Action string
}

// RecordLossage appends an executed action into the bounded 60-item Emacs lossage ring
// and remembers the latest non-repeat command for `C-c .` (`repeat-last-command`).
func (f *Frame) RecordLossage(action string) {
	act := strings.TrimSpace(action)
	if act == "" {
		return
	}
	buf := f.ActiveBuffer()
	f.mu.Lock()
	f.lossage = append(f.lossage, LossageEntry{
		At:     time.Now(),
		Buffer: buf,
		Action: act,
	})
	if len(f.lossage) > 60 {
		f.lossage = f.lossage[len(f.lossage)-60:]
	}
	if act != "cmd:repeat-last" && act != "cmd:view-lossage" {
		f.lastAction = act
	}
	f.mu.Unlock()
}

// ViewLossage implements Emacs `C-h l` / `SPC h l` (`view-lossage`): opens *Help*
// displaying the chronological ring of the last 60 dispatched commands/keystrokes.
func (f *Frame) ViewLossage() {
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	f.mu.Lock()
	entries := append([]LossageEntry(nil), f.lossage...)
	f.mu.Unlock()
	lines := make([]string, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		lines = append(lines, fmt.Sprintf("%s  [%s]  %s", e.At.Format("15:04:05"), e.Buffer, e.Action))
	}
	desc := "No commands recorded yet in this session."
	if len(lines) > 0 {
		desc = fmt.Sprintf("Most recent %d dispatched commands/keystrokes (newest first).", len(lines))
	}
	data.KeyDoc = &buffer.HelpKeyDoc{
		Heading:     "C-h l · view-lossage",
		Subject:     "Recent Keystroke & Command Ring",
		Tier:        "LOSSAGE",
		Command:     fmt.Sprintf("%d event(s)", len(lines)),
		Description: desc,
		Aliases:     lines,
	}
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("view-lossage: %d recorded event(s)", len(lines)))
}

// TriggerUniversalArg implements Emacs `C-u` (`universal-argument`): multiplies
// the numeric prefix argument (0 -> 4 -> 16 -> 64) for the next repeatable command.
func (f *Frame) TriggerUniversalArg() int {
	f.mu.Lock()
	if f.universalArg <= 0 {
		f.universalArg = 4
	} else if f.universalArg < 64 {
		f.universalArg *= 4
	}
	n := f.universalArg
	f.mu.Unlock()
	f.Echo(fmt.Sprintf("C-u %d-", n))
	return n
}

// UniversalArg returns the currently active Emacs `C-u` prefix count (0 when inactive).
func (f *Frame) UniversalArg() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.universalArg
}

func (f *Frame) clearUniversalArg() {
	f.mu.Lock()
	f.universalArg = 0
	f.mu.Unlock()
}

func (f *Frame) consumeUniversalArg() int {
	f.mu.Lock()
	n := f.universalArg
	f.universalArg = 0
	f.mu.Unlock()
	if n <= 0 {
		return 1
	}
	return n
}

// RepeatLastCommand implements Emacs `repeat` (`C-c .` / `SPC m .`), honoring
// any active `C-u` universal prefix multiplier.
func (f *Frame) RepeatLastCommand() bool {
	f.mu.Lock()
	act := f.lastAction
	f.mu.Unlock()
	if act == "" {
		f.Echo("No previous command to repeat")
		return false
	}
	count := f.consumeUniversalArg()
	for i := 0; i < count; i++ {
		f.dispatchWidgetAction(act)
	}
	f.Echo(fmt.Sprintf("Repeated %s (%dx)", act, count))
	return true
}

// EmbarkAct implements modern Emacs `embark-act` (`C-.` / `SPC .` / `C-c e`):
// inspects the active target at point (Magit hunk, Polymode code/diff span,
// *Ibuffer* cursor buffer, or active buffer) and opens contextual actions in Vertico.
func (f *Frame) EmbarkAct() {
	f.OpenMinibuffer(minibuffer.ModeEmbark)
}

func (f *Frame) embarkTarget() (kind, label string, targetBuf buffer.ID) {
	snap := f.loadSnapshot()
	active := snap.ActiveBuffer
	if active == buffer.Ibuffer {
		rows := f.orderedIbufferRows(snap)
		if len(rows) > 0 {
			idx := snap.TableCursor
			if idx < 0 {
				idx = 0
			}
			if idx >= len(rows) {
				idx = len(rows) - 1
			}
			b := rows[idx].BufferID
			return "ibuffer-row", string(b), b
		}
	}
	if active == buffer.Magit {
		return "magit", "*magit* hunk/repo", buffer.Magit
	}
	f.mu.Lock()
	spanCode, spanLang := f.activeSpanCode, f.activeSpanLang
	f.mu.Unlock()
	if buffer.IsChatBuffer(active) && strings.TrimSpace(spanCode) != "" {
		lbl := "code span"
		if spanLang != "" {
			lbl = spanLang + " span"
		}
		return "polymode-span", lbl, active
	}
	return "buffer", string(active), active
}

func (f *Frame) currentEmbarkTargetLabel() string {
	_, lbl, _ := f.embarkTarget()
	return lbl
}

func (f *Frame) embarkCandidates(query string) []minibuffer.Candidate {
	kind, _, tgt := f.embarkTarget()
	var cands []minibuffer.Candidate
	switch kind {
	case "magit":
		cands = []minibuffer.Candidate{
			{ID: "emb-stage", Name: "stage-hunk-or-file", Desc: "Stage selected file/hunk in *magit*", Annotation: "s", Run: func() { f.dispatchWidgetAction("cmd:magit-stage") }},
			{ID: "emb-unstage", Name: "unstage-hunk-or-file", Desc: "Unstage selected file/hunk in *magit*", Annotation: "u", Run: func() { f.dispatchWidgetAction("cmd:magit-unstage") }},
			{ID: "emb-audit", Name: "ai-code-audit", Desc: "Open 50/50 AI + Lambda Aitia code audit split", Annotation: "a · C-c g", Run: func() { f.dispatchWidgetAction("cmd:magit-ai-audit") }},
			{ID: "emb-refresh", Name: "refresh-aitia", Desc: "Refresh jj/git status & re-run Aitia gRPC audit", Annotation: "g", Run: func() { f.dispatchWidgetAction("cmd:magit-refresh") }},
			{ID: "emb-desc", Name: "describe-mode", Desc: "Inspect Magit-Status mode & Polymode bindings", Annotation: "C-h m", Run: f.DescribeCurrentMode},
		}
	case "polymode-span":
		cands = []minibuffer.Candidate{
			{ID: "emb-copy", Name: "copy-code-span", Desc: "Copy active Polymode code/diff span to clipboard", Annotation: "y", Run: func() { f.dispatchWidgetAction("cmd:span-copy-code") }},
			{ID: "emb-prev", Name: "open-span-in-preview", Desc: "Open focused Polymode span in *Preview*", Annotation: "p", Run: func() { f.dispatchWidgetAction("cmd:span-open-preview") }},
			{ID: "emb-explain", Name: "ai-explain-span", Desc: "Ask LLM to explain & audit this Polymode span", Annotation: "a", Run: func() { f.dispatchWidgetAction("cmd:span-ai-explain") }},
			{ID: "emb-rename", Name: "rename-buffer", Desc: "Rename " + string(tgt), Annotation: "C-x x r", Run: f.OpenRenameBuffer},
			{ID: "emb-ro", Name: "toggle-read-only-mode", Desc: "Toggle read-only protection on " + string(tgt), Annotation: "C-c t r", Run: func() { f.ToggleMinorModeOnActive("read-only-mode") }},
			{ID: "emb-desc", Name: "describe-mode", Desc: "Inspect active Polymode Host + Inner Mode stack", Annotation: "C-h m", Run: f.DescribeCurrentMode},
		}
	default:
		b := tgt
		cands = []minibuffer.Candidate{
			{ID: "emb-other", Name: "open-in-other-window", Desc: "Open " + string(b) + " in sibling window (C-x 4 b)", Annotation: "o · C-x 4 b", Run: func() { f.SelectBufferOtherWindow(b) }},
			{ID: "emb-clone", Name: "clone-window", Desc: "Split right with linked view of " + string(b), Annotation: "c · C-x 3", Run: f.CloneCurrentWindow},
			{ID: "emb-zoom", Name: "window-zoom-toggle", Desc: "Zoom / restore focused window (SPC w m)", Annotation: "m · C-x w m", Run: func() { f.ZoomWindowToggle() }},
			{ID: "emb-rename", Name: "rename-buffer", Desc: "Rename buffer " + string(b), Annotation: "r · C-x x r", Run: f.OpenRenameBuffer},
			{ID: "emb-bury", Name: "bury-buffer", Desc: "Move " + string(b) + " to end of workspace buffer ring", Annotation: "z · C-c z", Run: func() { f.BuryActiveBuffer() }},
			{ID: "emb-ded", Name: "toggle-dedicated-window-mode", Desc: "Pin / unpin focused window to " + string(b), Annotation: "d · C-x w d", Run: func() { f.ToggleMinorModeOnActive("dedicated-window-mode") }},
			{ID: "emb-ro", Name: "toggle-read-only-mode", Desc: "Toggle read-only protection on " + string(b), Annotation: "C-c t r", Run: func() { f.ToggleMinorModeOnActive("read-only-mode") }},
			{ID: "emb-kill", Name: "kill-buffer", Desc: "Kill buffer " + string(b), Annotation: "k · C-x k", Run: func() { f.KillBuffer(b) }},
			{ID: "emb-desc", Name: "describe-mode", Desc: "Describe Major/Minor/Polymode of " + string(b), Annotation: "C-h m", Run: f.DescribeCurrentMode},
		}
	}
	return minibuffer.FilterCandidates(cands, query)
}

// ZoomWindowToggle implements Doom `SPC w m` / Emacs `C-x w m`
// (`doom/window-maximize-buffer`): toggles full-frame zoom for the focused window.
func (f *Frame) ZoomWindowToggle() bool {
	if f.wsMgr == nil {
		return false
	}
	zoomed, changed, wsSnap := f.wsMgr.ZoomWindowToggle()
	if !changed {
		f.Echo("Window zoom requires a split layout")
		return false
	}
	f.syncSplitStateLocked(wsSnap.Active)
	if zoomed {
		f.Echo("Zoomed window: " + string(wsSnap.Active.FocusedBuffer()) + " (SPC w m to restore)")
	} else {
		f.Echo(fmt.Sprintf("Restored split layout (%d windows)", wsSnap.Active.WindowCount()))
	}
	return true
}

// TransposeWindowLayout implements Emacs `transpose-frame` (`SPC w T` / `C-x w t`):
// flips split orientation between vertical and horizontal.
func (f *Frame) TransposeWindowLayout() bool {
	if f.wsMgr == nil {
		return false
	}
	ok, wsSnap := f.wsMgr.TransposeWindowLayout()
	if !ok {
		f.Echo("Transpose requires a split layout")
		return false
	}
	f.syncSplitStateLocked(wsSnap.Active)
	f.Echo("Transposed split orientation (V <-> H)")
	return true
}

// OpenRenameBuffer opens the Emacs `C-x x r` / `SPC b R` (`rename-buffer`) prompt.
func (f *Frame) OpenRenameBuffer() {
	f.OpenMinibuffer(minibuffer.ModeRenameBuffer)
}

// RenameActiveBuffer renames the active buffer across workspace tabs, window tree,
// mode state, and per-buffer chat session bindings (`C-x x r` / `SPC b R`).
func (f *Frame) RenameActiveBuffer(newName string) (buffer.ID, bool) {
	if f.wsMgr == nil {
		return "", false
	}
	oldID := f.ActiveBuffer()
	clean := strings.TrimSpace(newName)
	if clean == "" {
		return "", false
	}
	var newID buffer.ID
	if buffer.IsChatBuffer(oldID) && !strings.HasPrefix(clean, "*") {
		newID = buffer.NewChatBufferID(clean)
	} else if !strings.HasPrefix(clean, "*") {
		newID = buffer.ID("*" + clean + "*")
	} else {
		newID = buffer.ID(clean)
	}
	ok, wsSnap := f.wsMgr.RenameBuffer(oldID, newID)
	if !ok {
		f.Echo(fmt.Sprintf("Cannot rename %s -> %s (already exists or unchanged)", oldID, newID))
		return "", false
	}
	f.mu.Lock()
	if st, exists := f.bufModes[oldID]; exists {
		st.BufferID = string(newID)
		delete(f.bufModes, oldID)
		f.bufModes[newID] = st
	}
	if sess, exists := f.chatSessions[oldID]; exists {
		delete(f.chatSessions, oldID)
		f.chatSessions[newID] = sess
	}
	if f.ibufferMarked[oldID] {
		delete(f.ibufferMarked, oldID)
		f.ibufferMarked[newID] = true
	}
	f.activeBuffer = wsSnap.Active.FocusedBuffer()
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.persistWorkspaces()
	f.requestFrame()
	f.Echo(fmt.Sprintf("Renamed %s -> %s", oldID, newID))
	return newID, true
}

// BuryActiveBuffer implements Emacs `bury-buffer` (`SPC b z` / `C-c z`): moves
// the focused buffer to the tail of the workspace ring and switches to the next.
func (f *Frame) BuryActiveBuffer() bool {
	if f.wsMgr == nil {
		return false
	}
	nextBuf, ok, wsSnap := f.wsMgr.BuryBuffer()
	if !ok {
		f.Echo("Only one buffer open in workspace")
		return false
	}
	f.syncChatSessionForBuffer(nextBuf)
	f.mu.Lock()
	f.activeBuffer = nextBuf
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	f.Echo("Buried buffer -> " + string(nextBuf))
	return true
}
