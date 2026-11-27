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
	return m.OpenBufferWithAction(buf, "")
}

// OpenBufferWithAction executes Emacs `display-buffer-alist` placement policy
// (`reuse-window`, `split-right`, `split-below`, `popup-bottom`, `popup-right`)
// alongside Doom popup rules and `dedicated-window-mode` protection.
func (m *Manager) OpenBufferWithAction(buf buffer.ID, displayAction string) (openedAsPopup bool, snap Snapshot) {
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
	if !alreadyMainTab && (displayAction == "popup-bottom" || displayAction == "popup-right") {
		side := SideBottom
		if displayAction == "popup-right" {
			side = SideRight
		}
		rc := PopupRule{Matcher: buf, Side: side, Size: 0.36, Select: true, Quit: QuitCurrent}
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
	// Honor `split-right` / `split-below` display-buffer-alist rules when only
	// one window is open and buf differs from the focused window's buffer.
	if (displayAction == "split-right" || displayAction == "split-below") &&
		focusLeaf != nil && focusLeaf.Buffer != buf && len(ws.Windows()) == 1 {
		recordWinnerSnapshot(ws)
		ws.NextWindowSeq++
		newID := fmt.Sprintf("w%d", ws.NextWindowSeq)
		dir := SplitVertical
		if displayAction == "split-below" {
			dir = SplitHorizontal
		}
		ws.WindowTree = mapWindowLeaf(ws.WindowTree, focusLeaf.ID, func(leaf *WindowNode) *WindowNode {
			cp := *leaf
			return &WindowNode{
				Direction: dir,
				Ratio:     0.50,
				First:     &cp,
				Second:    &WindowNode{ID: newID, Buffer: buf},
			}
		})
		ws.FocusedWindowID = newID
		syncLegacySplitFields(ws)
		return false, m.snapshotLocked()
	}
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

// KillBuffer removes buf from the active workspace's buffer list (Emacs C-x k / SPC b d)
// and replaces any window showing it with the nearest remaining buffer, never killing
// the last remaining buffer in the workspace.
func (m *Manager) KillBuffer(buf buffer.ID) (killed bool, fallback buffer.ID, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, "", Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if len(ws.Buffers) <= 1 {
		return false, ws.FocusedBuffer(), m.snapshotLocked()
	}
	pos := -1
	for i, b := range ws.Buffers {
		if b == buf {
			pos = i
			break
		}
	}
	if pos < 0 {
		return false, ws.FocusedBuffer(), m.snapshotLocked()
	}
	ws.Buffers = append(ws.Buffers[:pos], ws.Buffers[pos+1:]...)
	if pos >= len(ws.Buffers) {
		pos = len(ws.Buffers) - 1
	}
	fallback = ws.Buffers[pos]
	// Replace every window leaf showing the killed buffer with fallback.
	for _, lf := range ws.Windows() {
		if lf.Buffer == buf {
			ws.WindowTree = mapWindowLeaf(ws.WindowTree, lf.ID, func(leaf *WindowNode) *WindowNode {
				cp := *leaf
				cp.Buffer = fallback
				return &cp
			})
		}
	}
	if ws.PopupBuffer == buf {
		ws.PopupVisible = false
		ws.PopupBuffer = ""
		ws.PopupRule = nil
	}
	syncLegacySplitFields(ws)
	return true, fallback, m.snapshotLocked()
}

// StepBuffer cycles the focused window to the previous (-1) or next (+1) buffer
// in the active workspace's buffer list (Emacs C-x LEFT/RIGHT · Doom SPC b [ / ]).
func (m *Manager) StepBuffer(delta int) (nextBuf buffer.ID, ok bool, snap Snapshot) {
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
	curIdx := 0
	for i, b := range ws.Buffers {
		if b == cur {
			curIdx = i
			break
		}
	}
	nextIdx := (curIdx + delta) % len(ws.Buffers)
	if nextIdx < 0 {
		nextIdx += len(ws.Buffers)
	}
	nextBuf = ws.Buffers[nextIdx]
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = nextBuf
		return &cp
	})
	syncLegacySplitFields(ws)
	return nextBuf, true, m.snapshotLocked()
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
