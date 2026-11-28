// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
)

// OpenRegisterSave prompts for a register name and snapshots the active
// workspace's recursive window layout (Emacs `C-x r w` / Doom `SPC w R`).
func (f *Frame) OpenRegisterSave() {
	f.OpenMinibuffer(minibuffer.ModeRegisterSave)
}

// OpenRegisterJump opens Vertico listing all saved Emacs window registers
// (Emacs `C-x r j` / Doom `SPC w J`).
func (f *Frame) OpenRegisterJump() {
	f.OpenMinibuffer(minibuffer.ModeRegisterJump)
}

// SaveWindowRegister snapshots the active workspace's recursive window tree
// into register regName (Emacs `window-configuration-to-register`).
func (f *Frame) SaveWindowRegister(regName string) bool {
	if f.wsMgr == nil {
		return false
	}
	reg, ok := f.wsMgr.SaveWindowRegister(regName)
	if !ok {
		f.Echo("Register name cannot be empty")
		return false
	}
	f.Echo(fmt.Sprintf("Saved window layout to register [%s]: %s", reg.Name, reg.Summary))
	return true
}

// JumpWindowRegister restores saved window register regName (Emacs `jump-to-register`),
// pushing the previous layout onto `winner-mode` undo history.
func (f *Frame) JumpWindowRegister(regName string) bool {
	if f.wsMgr == nil {
		return false
	}
	reg, ok, wsSnap := f.wsMgr.JumpWindowRegister(regName)
	if !ok {
		f.Echo(fmt.Sprintf("Register [%s] not found", strings.TrimSpace(regName)))
		return false
	}
	f.syncChatSessionForBuffer(wsSnap.Active.FocusedBuffer())
	f.mu.Lock()
	f.activeBuffer = wsSnap.Active.FocusedBuffer()
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.persistWorkspaces()
	f.requestFrame()
	f.Echo(fmt.Sprintf("Jumped to register [%s] (%d window(s))", reg.Name, reg.WindowCount))
	return true
}

// ListWindowRegisters opens *Help* with a structured Emacs `list-registers`
// (`C-x r l` / `SPC w L`) summary of all saved window layout registers.
func (f *Frame) ListWindowRegisters() {
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	var aliases []string
	if f.wsMgr != nil {
		for _, r := range f.wsMgr.ListWindowRegisters() {
			aliases = append(aliases, fmt.Sprintf("[%s] (%dw) %s", r.Name, r.WindowCount, r.Summary))
		}
	}
	desc := "No window registers saved yet. Press C-x r w (or SPC w R) to save the current split layout."
	if len(aliases) > 0 {
		desc = fmt.Sprintf("%d saved window configuration register(s). Jump with C-x r j or SPC w J.", len(aliases))
	}
	data.KeyDoc = &buffer.HelpKeyDoc{
		Heading:     "C-x r l · list-registers",
		Subject:     "Window Configuration Registers",
		Tier:        "REGISTERS",
		Command:     fmt.Sprintf("%d register(s)", len(aliases)),
		Description: desc,
		Aliases:     aliases,
	}
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("list-registers: %d saved", len(aliases)))
}

func (f *Frame) registerJumpCandidates(query string) []minibuffer.Candidate {
	if f.wsMgr == nil {
		return nil
	}
	regs := f.wsMgr.ListWindowRegisters()
	all := make([]minibuffer.Candidate, 0, len(regs))
	for _, r := range regs {
		reg := r
		all = append(all, minibuffer.Candidate{
			ID:         "reg-" + reg.Name,
			Name:       reg.Name,
			Desc:       reg.Summary,
			Annotation: fmt.Sprintf("%d win · %s", reg.WindowCount, reg.SavedAt.Format("15:04:05")),
			Run:        func() { f.JumpWindowRegister(reg.Name) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

// OpenBufferOtherWindowPrompt opens Vertico for Emacs `C-x 4 b` / `SPC b B`
// (`switch-to-buffer-other-window`).
func (f *Frame) OpenBufferOtherWindowPrompt() {
	f.OpenMinibuffer(minibuffer.ModeBufferOtherWindow)
}

// SelectBufferOtherWindow opens buf in an undedicated sibling window (splitting
// vertically if only one window exists), preserving the original window untouched.
func (f *Frame) SelectBufferOtherWindow(id buffer.ID) {
	f.miniState.Close()
	f.meowState.ClearKeypad()
	_ = f.EffectiveModeStack(id)
	f.syncChatSessionForBuffer(id)
	if f.wsMgr == nil {
		f.SelectBuffer(id)
		return
	}
	wsSnap := f.wsMgr.OpenBufferOtherWindow(id)
	f.mu.Lock()
	f.activeBuffer = wsSnap.Active.FocusedBuffer()
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.activePluginID = ""
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.persistWorkspaces()
	f.requestFrame()
	f.Echo(fmt.Sprintf("Other window -> %s (%d windows)", id, wsSnap.Active.WindowCount()))
}

func (f *Frame) bufferOtherWindowCandidates(query string) []minibuffer.Candidate {
	base := f.bufferCandidates(query)
	out := make([]minibuffer.Candidate, 0, len(base))
	for _, cand := range base {
		c := cand
		if strings.HasPrefix(c.ID, "action:") || strings.HasPrefix(c.ID, "create:") || strings.HasPrefix(c.ID, "scheme-ext:") {
			continue
		}
		bid := buffer.ID(c.ID)
		c.Run = func() { f.SelectBufferOtherWindow(bid) }
		out = append(out, c)
	}
	return out
}

// KillBufferAndWindow implements Emacs `C-x 4 0` / `SPC b K`
// (`kill-buffer-and-window`): kills the focused buffer and closes its split
// window when multiple windows are open.
func (f *Frame) KillBufferAndWindow() bool {
	if f.wsMgr == nil {
		return false
	}
	target := f.ActiveBuffer()
	winCount := f.wsMgr.Snapshot().Active.WindowCount()
	killed := f.KillBuffer(target)
	if winCount > 1 {
		f.CloseFocusedPane()
	}
	return killed
}

// recordRecentCommand records an executed M-x command into Frame's savehist
// MRU list so recently used commands float to the top of Vertico completion.
func (f *Frame) recordRecentCommand(name string) {
	n := strings.TrimSpace(name)
	if n == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	next := make([]string, 0, len(f.recentCommands)+1)
	next = append(next, n)
	for _, old := range f.recentCommands {
		if old != n && len(next) < 32 {
			next = append(next, old)
		}
	}
	f.recentCommands = next
}

// orderedIbufferRows returns *Ibuffer* entries in the exact visual display order
// (grouped alphabetically by Major Mode, respecting the active Major-Mode filter)
// so keyboard cursor navigation (`j`/`k`/`d`/`u`/`RET`) aligns 1:1 with the UI.
func (f *Frame) orderedIbufferRows(snap frameSnapshot) []buffer.IbufferEntry {
	rows := f.BuildIbufferEntries(snap)
	flt := f.IbufferFilter()
	groups := make(map[string][]buffer.IbufferEntry)
	var names []string
	for _, r := range rows {
		grp := r.MajorMode
		if grp == "" {
			grp = "Fundamental"
		}
		if flt != "" && flt != grp {
			continue
		}
		if _, ok := groups[grp]; !ok {
			names = append(names, grp)
		}
		groups[grp] = append(groups[grp], r)
	}
	sort.Strings(names)
	out := make([]buffer.IbufferEntry, 0, len(rows))
	for _, g := range names {
		out = append(out, groups[g]...)
	}
	return out
}

func (f *Frame) ibufferMarkAtCursor(mark bool) bool {
	snap := f.loadSnapshot()
	if snap.ActiveBuffer != buffer.Ibuffer {
		return false
	}
	rows := f.orderedIbufferRows(snap)
	if len(rows) <= 1 {
		return false
	}
	idx := snap.TableCursor
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	target := rows[idx].BufferID
	f.mu.Lock()
	if f.ibufferMarked == nil {
		f.ibufferMarked = make(map[buffer.ID]bool, 8)
	}
	if mark {
		f.ibufferMarked[target] = true
	} else {
		delete(f.ibufferMarked, target)
	}
	if idx+1 < len(rows) {
		f.tableCursor = idx + 1
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	if mark {
		f.Echo(fmt.Sprintf("Ibuffer: marked %s [D]", target))
	} else {
		f.Echo(fmt.Sprintf("Ibuffer: unmarked %s", target))
	}
	return true
}

func (f *Frame) ibufferSelectAtCursor() bool {
	snap := f.loadSnapshot()
	if snap.ActiveBuffer != buffer.Ibuffer {
		return false
	}
	rows := f.orderedIbufferRows(snap)
	if len(rows) == 0 {
		return false
	}
	idx := snap.TableCursor
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	f.SelectBuffer(rows[idx].BufferID)
	return true
}
