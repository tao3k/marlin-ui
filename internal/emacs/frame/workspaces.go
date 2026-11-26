// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/window"
	"github.com/tao3k/marlin-ui/internal/emacs/workspace"
)

// SelectBuffer focuses an Emacs buffer via Doom's popup rule & workspace
// isolation engine: auxiliary buffers (*Help*, *Messages*, *Processes*, *Models*)
// open non-destructively in their governed frosted-glass Popup Drawer without
// disturbing the main window, while primary buffers isolate per workspace.
func (f *Frame) SelectBuffer(id buffer.ID) {
	wasMini := f.miniState.IsCompletionActive()
	f.miniState.Close()
	f.meowState.ClearKeypad()
	if f.wsMgr != nil {
		asPopup, wsSnap := f.wsMgr.OpenBuffer(id)
		f.mu.Lock()
		wasWeb := f.activeFeatureID == "feature:webisland" || f.activePluginID != ""
		if !asPopup {
			f.activeBuffer = wsSnap.Active.ActiveBuffer
			f.activeFeatureID = ""
			f.activeExtID = ""
			f.activePluginID = ""
			f.tableCursor = 0
			f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
		}
		f.publishSnapshotLocked()
		f.mu.Unlock()
		if !asPopup {
			f.notifyWebIslandVisibility("")
		}
		if wasWeb || wasMini {
			f.refocusFrame.Store(true)
		}
		f.requestFrame()
		return
	}
	f.mu.Lock()
	f.activeBuffer = id
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

// CreateWorkspace spawns and activates a fresh isolated perspective workspace
// (Doom persp-mode / :ui workspaces) with its own clean buffer list.
func (f *Frame) CreateWorkspace(name, projectRoot string) workspace.Perspective {
	if f.wsMgr == nil {
		return workspace.Perspective{}
	}
	ws := f.wsMgr.CreateWorkspace(name, projectRoot)
	f.syncFromWorkspaceLocked(ws)
	f.Echo(fmt.Sprintf("Workspace created: %s", ws.Name))
	return ws
}

// SwitchWorkspace switches to an isolated perspective workspace and restores
// its exact active buffer, open tabs, and popup drawer state.
func (f *Frame) SwitchWorkspace(id string) bool {
	if f.wsMgr == nil {
		return false
	}
	ws, ok := f.wsMgr.Switch(id)
	if !ok {
		return false
	}
	f.syncFromWorkspaceLocked(ws)
	f.Echo("Workspace: " + ws.Name)
	return true
}

// DismissPopup closes the active workspace's Doom popup drawer without touching
// the main editing/conversation buffer.
func (f *Frame) DismissPopup() bool {
	if f.wsMgr == nil {
		return false
	}
	dismissed, _ := f.wsMgr.DismissPopup()
	if !dismissed {
		return false
	}
	f.mu.Lock()
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return true
}

// TogglePopup toggles the active workspace's last popup drawer (Doom C-x p).
func (f *Frame) TogglePopup() {
	if f.wsMgr == nil {
		return
	}
	visible, _ := f.wsMgr.TogglePopup()
	f.mu.Lock()
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if visible {
		f.Echo("Popup restored")
	} else {
		f.Echo("Popup hidden")
	}
	f.requestFrame()
}

// RaisePopup promotes the active popup buffer into a first-class main window
// tab inside the active workspace (Doom +popup/raise).
func (f *Frame) RaisePopup() {
	if f.wsMgr == nil {
		return
	}
	promoted, snap := f.wsMgr.RaisePopup()
	if promoted == "" {
		return
	}
	f.mu.Lock()
	f.activeBuffer = snap.Active.ActiveBuffer
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.activePluginID = ""
	f.openTabs = append([]buffer.ID(nil), snap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.Echo("Raised popup to main window: " + string(promoted))
	f.requestFrame()
}

// WorkspaceSnapshot returns an immutable snapshot of all workspaces and the
// active workspace's isolated buffers and popup drawer.
func (f *Frame) WorkspaceSnapshot() workspace.Snapshot {
	if f.wsMgr == nil {
		return workspace.Snapshot{}
	}
	return f.wsMgr.Snapshot()
}

func (f *Frame) syncFromWorkspaceLocked(ws workspace.Perspective) {
	f.miniState.Close()
	f.meowState.ClearKeypad()
	f.mu.Lock()
	f.activeBuffer = ws.ActiveBuffer
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.activePluginID = ""
	f.openTabs = append([]buffer.ID(nil), ws.Buffers...)
	f.tableCursor = 0
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.notifyWebIslandVisibility("")
	f.requestFrame()
}

func (f *Frame) buildWorkspaceTabs(wsSnap workspace.Snapshot) []tabs.WorkspaceTab {
	if len(wsSnap.Workspaces) == 0 {
		return nil
	}
	out := make([]tabs.WorkspaceTab, 0, len(wsSnap.Workspaces))
	for _, w := range wsSnap.Workspaces {
		out = append(out, tabs.WorkspaceTab{
			ID:     w.ID,
			Name:   w.Name,
			Active: w.ID == wsSnap.ActiveID,
		})
	}
	return out
}

func (f *Frame) buildPopupProps(wsSnap workspace.Snapshot) window.PopupProps {
	act := wsSnap.Active
	if !act.PopupVisible || act.PopupBuffer == "" {
		return window.PopupProps{}
	}
	side := workspace.SideBottom
	size := float32(0.35)
	if act.PopupRule != nil {
		side = act.PopupRule.Side
		size = act.PopupRule.Size
	}
	return window.PopupProps{
		Visible:   true,
		Buffer:    act.PopupBuffer,
		Side:      side,
		Size:      size,
		OnRaise:   f.RaisePopup,
		OnDismiss: func() { f.DismissPopup() },
	}
}
