// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package workspace

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
)

// WindowRegister stores an authentic Emacs `window-configuration-to-register`
// (`C-x r w`) snapshot of a workspace's recursive window tree & focus.
type WindowRegister struct {
	Name            string
	WorkspaceID     string
	WindowTree      *WindowNode
	FocusedWindowID string
	ActiveBuffer    buffer.ID
	WindowCount     int
	Summary         string
	SavedAt         time.Time
}

// SaveWindowRegister snapshots the active workspace's complete recursive window
// layout into named register regName (Emacs `C-x r w` / Doom `SPC w R`).
func (m *Manager) SaveWindowRegister(regName string) (WindowRegister, bool) {
	name := strings.TrimSpace(regName)
	if name == "" {
		return WindowRegister{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return WindowRegister{}, false
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if m.registers == nil {
		m.registers = make(map[string]WindowRegister, 8)
	}
	leaves := ws.Windows()
	parts := make([]string, 0, len(leaves))
	for _, lf := range leaves {
		parts = append(parts, fmt.Sprintf("#%d:%s", lf.Number, lf.Buffer))
	}
	reg := WindowRegister{
		Name:            name,
		WorkspaceID:     ws.ID,
		WindowTree:      cloneWindowTree(ws.WindowTree),
		FocusedWindowID: ws.FocusedWindowID,
		ActiveBuffer:    ws.FocusedBuffer(),
		WindowCount:     len(leaves),
		Summary:         strings.Join(parts, " | "),
		SavedAt:         time.Now(),
	}
	m.registers[name] = reg
	return reg, true
}

// JumpWindowRegister restores a saved window configuration register onto the
// active workspace (Emacs `C-x r j` / Doom `SPC w J`), recording the previous
// layout in `winner-mode` undo history so `C-c u` / `SPC w u` can revert cleanly.
func (m *Manager) JumpWindowRegister(regName string) (WindowRegister, bool, Snapshot) {
	name := strings.TrimSpace(regName)
	if name == "" {
		return WindowRegister{}, false, Snapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	reg, ok := m.registers[name]
	if !ok || reg.WindowTree == nil {
		return WindowRegister{}, false, m.snapshotLocked()
	}
	idx := m.activeIndexLocked()
	if idx < 0 {
		return WindowRegister{}, false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	recordWinnerSnapshot(ws)
	ws.WindowTree = cloneWindowTree(reg.WindowTree)
	ws.FocusedWindowID = reg.FocusedWindowID
	if !containsWindowID(ws.WindowTree, ws.FocusedWindowID) {
		if first := firstWindowLeaf(ws.WindowTree); first != nil {
			ws.FocusedWindowID = first.ID
		}
	}
	for _, lf := range ws.Windows() {
		if lf.Buffer != "" && !containsBuf(ws.Buffers, lf.Buffer) {
			ws.Buffers = append(ws.Buffers, lf.Buffer)
		}
	}
	syncLegacySplitFields(ws)
	return reg, true, m.snapshotLocked()
}

// ListWindowRegisters returns all saved Emacs window registers sorted by name.
func (m *Manager) ListWindowRegisters() []WindowRegister {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]WindowRegister, 0, len(m.registers))
	for _, r := range m.registers {
		cp := r
		cp.WindowTree = cloneWindowTree(r.WindowTree)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
