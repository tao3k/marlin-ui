// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package workspace

import (
	"fmt"
	"strings"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
)

// OpenBuffer routes buf through Doom popup rules and dedicated-window protection:
//   - If buf matches a popup rule AND is not already promoted into this workspace's
//     main buffer tabs, it opens non-destructively in the Popup Drawer.
//   - If the focused window is dedicated to another buffer, opens buf in an
//     undedicated sibling window (or splits vertically if all windows are dedicated).
//   - Otherwise focuses buf in the active window and records it in this
//     workspace's isolated buffer list.
func (m *Manager) OpenBuffer(buf buffer.ID) (openedAsPopup bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	alreadyMainTab := containsBuf(ws.Buffers, buf)
	if rule, ok := m.matchRuleLocked(buf); ok && !alreadyMainTab {
		rc := rule
		ws.PopupBuffer = buf
		ws.PopupRule = &rc
		ws.PopupVisible = true
		ws.LastPopup = buf
		return true, m.snapshotLocked()
	}
	if !alreadyMainTab {
		ws.Buffers = append(ws.Buffers, buf)
	}
	focusLeaf := findWindowLeaf(ws.WindowTree, ws.FocusedWindowID)
	if focusLeaf != nil && focusLeaf.Dedicated && focusLeaf.Buffer != buf {
		if other := firstUndedicatedLeaf(ws.WindowTree); other != nil {
			ws.WindowTree = mapWindowLeaf(ws.WindowTree, other.ID, func(leaf *WindowNode) *WindowNode {
				cp := *leaf
				cp.Buffer = buf
				return &cp
			})
			ws.FocusedWindowID = other.ID
			syncLegacySplitFields(ws)
			return false, m.snapshotLocked()
		}
		recordWinnerSnapshot(ws)
		ws.NextWindowSeq++
		newID := fmt.Sprintf("w%d", ws.NextWindowSeq)
		ws.WindowTree = mapWindowLeaf(ws.WindowTree, focusLeaf.ID, func(leaf *WindowNode) *WindowNode {
			cp := *leaf
			return &WindowNode{
				Direction: SplitVertical,
				Ratio:     0.50,
				First:     &cp,
				Second:    &WindowNode{ID: newID, Buffer: buf},
			}
		})
		ws.FocusedWindowID = newID
		syncLegacySplitFields(ws)
		return false, m.snapshotLocked()
	}
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = buf
		return &cp
	})
	syncLegacySplitFields(ws)
	return false, m.snapshotLocked()
}

// DismissPopup closes the active workspace's popup drawer without touching the
// main buffer or tab layout.
func (m *Manager) DismissPopup() (dismissed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 || !m.workspaces[idx].PopupVisible {
		return false, m.snapshotLocked()
	}
	ws := &m.workspaces[idx]
	ws.PopupVisible = false
	ws.PopupBuffer = ""
	ws.PopupRule = nil
	return true, m.snapshotLocked()
}

// TogglePopup toggles the active workspace's last popup drawer (Doom C-x p).
func (m *Manager) TogglePopup() (visible bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.PopupVisible {
		ws.PopupVisible = false
		ws.PopupBuffer = ""
		ws.PopupRule = nil
		return false, m.snapshotLocked()
	}
	if ws.LastPopup != "" {
		if rule, ok := m.matchRuleLocked(ws.LastPopup); ok {
			rc := rule
			ws.PopupBuffer = ws.LastPopup
			ws.PopupRule = &rc
			ws.PopupVisible = true
			return true, m.snapshotLocked()
		}
	}
	return false, m.snapshotLocked()
}

// RaisePopup promotes the active popup buffer into a first-class main window
// buffer inside the active workspace (Doom +popup/raise).
func (m *Manager) RaisePopup() (promoted buffer.ID, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 || !m.workspaces[idx].PopupVisible || m.workspaces[idx].PopupBuffer == "" {
		return "", m.snapshotLocked()
	}
	ws := &m.workspaces[idx]
	promoted = ws.PopupBuffer
	ws.PopupVisible = false
	ws.PopupBuffer = ""
	ws.PopupRule = nil
	ws.LastPopup = ""
	ensureWindowTree(ws)
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = promoted
		cp.Dedicated = false
		return &cp
	})
	syncLegacySplitFields(ws)
	if !containsBuf(ws.Buffers, promoted) {
		ws.Buffers = append(ws.Buffers, promoted)
	}
	return promoted, m.snapshotLocked()
}

// SchemeWindowSpec describes one window leaf streamed from Scheme's
// `marlin.ui.v1.UiFrame.Windows` topology.
type SchemeWindowSpec struct {
	ID     string
	Buffer buffer.ID
}

// SyncSchemeWindows applies a Scheme-authored `UIFrame.Windows` layout onto
// the active workspace's recursive window tree, preserving previous layouts in
// `winner-mode` undo history so `SPC w u` seamlessly restores user splits.
func (m *Manager) SyncSchemeWindows(windows []SchemeWindowSpec, selectedID string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 || len(windows) == 0 {
		return m.snapshotLocked()
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)

	valid := make([]SchemeWindowSpec, 0, len(windows))
	for i, w := range windows {
		if strings.TrimSpace(string(w.Buffer)) == "" {
			continue
		}
		id := strings.TrimSpace(w.ID)
		if id == "" {
			id = fmt.Sprintf("scheme-w%d", i+1)
		}
		valid = append(valid, SchemeWindowSpec{ID: id, Buffer: w.Buffer})
	}
	if len(valid) == 0 {
		return m.snapshotLocked()
	}

	// Skip no-op re-sync when the exact same window topology & selection are already live.
	curLeaves := ws.Windows()
	same := len(curLeaves) == len(valid)
	if same {
		for i := range valid {
			if curLeaves[i].ID != valid[i].ID || curLeaves[i].Buffer != valid[i].Buffer {
				same = false
				break
			}
		}
		if same && (selectedID == "" || ws.FocusedWindowID == selectedID) {
			return m.snapshotLocked()
		}
	}

	recordWinnerSnapshot(ws)
	ws.WindowTree = buildBalancedSchemeTree(valid, SplitVertical)
	targetFocus := strings.TrimSpace(selectedID)
	if targetFocus == "" || findWindowLeaf(ws.WindowTree, targetFocus) == nil {
		targetFocus = valid[0].ID
	}
	ws.FocusedWindowID = targetFocus
	for _, w := range valid {
		if !strings.HasPrefix(string(w.Buffer), "scheme-ext:") && !containsBuf(ws.Buffers, w.Buffer) {
			ws.Buffers = append(ws.Buffers, w.Buffer)
		}
	}
	syncLegacySplitFields(ws)
	return m.snapshotLocked()
}

func buildBalancedSchemeTree(specs []SchemeWindowSpec, dir SplitMode) *WindowNode {
	if len(specs) == 1 {
		return &WindowNode{
			ID:     specs[0].ID,
			Buffer: specs[0].Buffer,
		}
	}
	mid := len(specs) / 2
	nextDir := SplitHorizontal
	if dir == SplitHorizontal {
		nextDir = SplitVertical
	}
	return &WindowNode{
		Direction: dir,
		Ratio:     0.50,
		First:     buildBalancedSchemeTree(specs[:mid], nextDir),
		Second:    buildBalancedSchemeTree(specs[mid:], nextDir),
	}
}
