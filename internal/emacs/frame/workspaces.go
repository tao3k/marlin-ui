// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"

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

// SwitchWorkspaceIndex activates the 0-based perspective workspace (SPC l 1..4).
func (f *Frame) SwitchWorkspaceIndex(idx int) bool {
	if f.wsMgr == nil {
		return false
	}
	ws, ok := f.wsMgr.SwitchByIndex(idx)
	if !ok {
		f.Echo(fmt.Sprintf("No workspace #%d", idx+1))
		return false
	}
	f.syncFromWorkspaceLocked(ws)
	f.Echo(fmt.Sprintf("Workspace %d: %s", idx+1, ws.Name))
	return true
}

// StepWorkspace cycles to the next (+1) or previous (-1) perspective workspace.
func (f *Frame) StepWorkspace(delta int) bool {
	if f.wsMgr == nil {
		return false
	}
	ws, ok := f.wsMgr.StepWorkspace(delta)
	if !ok {
		return false
	}
	f.syncFromWorkspaceLocked(ws)
	f.Echo("Workspace: " + ws.Name)
	return true
}

// KillActiveWorkspace closes the current perspective workspace (SPC l d) unless
// it is the last remaining workspace.
func (f *Frame) KillActiveWorkspace() bool {
	if f.wsMgr == nil {
		return false
	}
	killed, snap := f.wsMgr.KillWorkspace("")
	if !killed {
		f.Echo("Cannot kill last remaining workspace")
		return false
	}
	f.syncFromWorkspaceLocked(snap.Active)
	f.Echo("Killed workspace -> " + snap.Active.Name)
	return true
}

// SplitVertical splits the active workspace left | right (Doom SPC w v).
func (f *Frame) SplitVertical(secondary buffer.ID) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.SplitVertical(secondary)
	f.syncSplitStateLocked(snap.Active)
	f.Echo("Split right: " + string(snap.Active.SecondaryBuffer))
}

// SplitHorizontal splits the active workspace top / bottom (Doom SPC w s).
func (f *Frame) SplitHorizontal(secondary buffer.ID) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.SplitHorizontal(secondary)
	f.syncSplitStateLocked(snap.Active)
	f.Echo("Split below: " + string(snap.Active.SecondaryBuffer))
}

// DeleteOtherWindows collapses any split to a single window (Doom SPC w o).
func (f *Frame) DeleteOtherWindows() {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.DeleteOtherWindows()
	f.syncSplitStateLocked(snap.Active)
	f.Echo("Single window: " + string(snap.Active.ActiveBuffer))
}

// CloseFocusedPane closes the active split pane (Doom SPC w d).
func (f *Frame) CloseFocusedPane() {
	if f.wsMgr == nil {
		return
	}
	closed, snap := f.wsMgr.CloseFocusedPane()
	if !closed {
		f.Echo("Only one window open")
		return
	}
	f.syncSplitStateLocked(snap.Active)
	f.Echo("Closed split pane -> " + string(snap.Active.ActiveBuffer))
}

// CycleFocusPane alternates focus between split panes (Doom SPC w w).
func (f *Frame) CycleFocusPane() {
	if f.wsMgr == nil {
		return
	}
	pane, snap := f.wsMgr.CycleFocusPane()
	f.syncSplitStateLocked(snap.Active)
	f.Echo(fmt.Sprintf("Focused window [%d]", pane+1))
}

// FocusPane explicitly focuses pane 0 (primary) or pane 1 (secondary).
func (f *Frame) FocusPane(pane int) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.FocusPane(pane)
	f.syncSplitStateLocked(snap.Active)
}

func (f *Frame) syncSplitStateLocked(ws workspace.Perspective) {
	f.mu.Lock()
	f.activeBuffer = ws.ActiveBuffer
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.activePluginID = ""
	f.openTabs = append([]buffer.ID(nil), ws.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.notifyWebIslandVisibility("")
	f.requestFrame()
}

// SelectModel switches the active LLM route alias live (e.g. clicking a row in
// *Models*), updates the modeline & transcript header immediately, and
// dispatches `model.select <alias>` over marlin.ui.v1 gRPC to the Rust authority.
func (f *Frame) SelectModel(alias, displayLabel string) {
	cleanAlias := strings.TrimSpace(alias)
	if cleanAlias == "" {
		return
	}
	lbl := strings.TrimSpace(displayLabel)
	if lbl == "" {
		lbl = cleanAlias
	}
	f.mu.Lock()
	f.modelLabel = lbl
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		if f.Adaptive.Desktop.Transcript != nil {
			f.Adaptive.Desktop.Transcript.SetModelLabel(lbl)
		}
		if f.Adaptive.Desktop.OnCommandInvoke != nil {
			f.Adaptive.Desktop.OnCommandInvoke("model.select " + cleanAlias)
		}
	}
	f.Echo("Active model: " + lbl)
	f.requestFrame()
}

func (f *Frame) buildSplitProps(wsSnap workspace.Snapshot) window.SplitProps {
	act := wsSnap.Active
	if act.SplitMode == "" || act.SplitMode == workspace.SplitSingle || act.SecondaryBuffer == "" {
		return window.SplitProps{Mode: workspace.SplitSingle}
	}
	return window.SplitProps{
		Mode:            act.SplitMode,
		SecondaryBuffer: act.SecondaryBuffer,
		FocusedPane:     act.FocusedPane,
		OnFocusPane:     f.FocusPane,
		OnCloseSplit:    f.CloseFocusedPane,
	}
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
