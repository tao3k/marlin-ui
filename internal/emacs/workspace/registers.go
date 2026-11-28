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

// ZoomWindowToggle implements Doom Emacs `doom/window-maximize-buffer`
// (`SPC w m` / `C-x w m`): pressing once temporarily zooms the focused window
// to fill the workspace while saving the exact split tree; pressing again
// restores the saved split tree cleanly.
func (m *Manager) ZoomWindowToggle() (zoomed bool, changed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if ws.zoomSaved != nil {
		recordWinnerSnapshot(ws)
		ws.WindowTree = cloneWindowTree(ws.zoomSaved.tree)
		ws.FocusedWindowID = ws.zoomSaved.focusedWindowID
		ws.zoomSaved = nil
		syncLegacySplitFields(ws)
		return false, true, m.snapshotLocked()
	}
	if len(ws.Windows()) <= 1 {
		return false, false, m.snapshotLocked()
	}
	recordWinnerSnapshot(ws)
	ws.zoomSaved = &windowLayoutSnap{
		tree:            cloneWindowTree(ws.WindowTree),
		focusedWindowID: ws.FocusedWindowID,
	}
	focus := findWindowLeaf(ws.WindowTree, ws.FocusedWindowID)
	buf := ws.FocusedBuffer()
	ded := false
	fid := ws.FocusedWindowID
	if focus != nil {
		buf = focus.Buffer
		ded = focus.Dedicated
		fid = focus.ID
	}
	ws.WindowTree = &WindowNode{ID: fid, Buffer: buf, Dedicated: ded}
	ws.FocusedWindowID = fid
	syncLegacySplitFields(ws)
	return true, true, m.snapshotLocked()
}

// TransposeWindowLayout implements Emacs `transpose-frame` (`SPC w T` / `C-x w t`):
// flips every internal split container in the active workspace's window tree
// between Vertical (`left | right`) and Horizontal (`top / bottom`).
func (m *Manager) TransposeWindowLayout() (transposed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if len(ws.Windows()) <= 1 {
		return false, m.snapshotLocked()
	}
	recordWinnerSnapshot(ws)
	ws.WindowTree = flipSplitDirections(ws.WindowTree)
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

func flipSplitDirections(n *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	cp := *n
	if n.IsSplit() {
		if cp.Direction == SplitVertical {
			cp.Direction = SplitHorizontal
		} else {
			cp.Direction = SplitVertical
		}
		cp.First = flipSplitDirections(n.First)
		cp.Second = flipSplitDirections(n.Second)
	}
	return &cp
}

// RenameBuffer renames oldID -> newID across the active workspace's buffer list
// and every leaf in its recursive window tree (Emacs `rename-buffer` / `C-x x r`).
func (m *Manager) RenameBuffer(oldID, newID buffer.ID) (ok bool, snap Snapshot) {
	if oldID == "" || newID == "" || oldID == newID {
		return false, m.Snapshot()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if containsBuf(ws.Buffers, newID) {
		return false, m.snapshotLocked()
	}
	found := false
	for i, b := range ws.Buffers {
		if b == oldID {
			ws.Buffers[i] = newID
			found = true
		}
	}
	if !found {
		return false, m.snapshotLocked()
	}
	ws.WindowTree = renameWindowBuffer(ws.WindowTree, oldID, newID)
	if ws.PopupBuffer == oldID {
		ws.PopupBuffer = newID
	}
	if ws.LastPopup == oldID {
		ws.LastPopup = newID
	}
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

func renameWindowBuffer(n *WindowNode, oldID, newID buffer.ID) *WindowNode {
	if n == nil {
		return nil
	}
	cp := *n
	if n.IsLeaf() {
		if cp.Buffer == oldID {
			cp.Buffer = newID
		}
		return &cp
	}
	cp.First = renameWindowBuffer(n.First, oldID, newID)
	cp.Second = renameWindowBuffer(n.Second, oldID, newID)
	return &cp
}

// BuryBuffer implements Emacs `bury-buffer` (`SPC b z` / `C-c z`): moves the
// focused buffer to the tail of the workspace's buffer list and switches the
// focused window to the next buffer at the head of the ring.
func (m *Manager) BuryBuffer() (nextBuf buffer.ID, ok bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return "", false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if len(ws.Buffers) <= 1 {
		return ws.FocusedBuffer(), false, m.snapshotLocked()
	}
	cur := ws.FocusedBuffer()
	pos := -1
	for i, b := range ws.Buffers {
		if b == cur {
			pos = i
			break
		}
	}
	if pos >= 0 {
		ws.Buffers = append(ws.Buffers[:pos], ws.Buffers[pos+1:]...)
		ws.Buffers = append(ws.Buffers, cur)
	}
	nextBuf = ws.Buffers[0]
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = nextBuf
		return &cp
	})
	syncLegacySplitFields(ws)
	return nextBuf, true, m.snapshotLocked()
}
