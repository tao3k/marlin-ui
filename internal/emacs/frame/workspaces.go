// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
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
	f.persistWorkspaces()
	f.Echo(fmt.Sprintf("Workspace created: %s", ws.Name))
	return ws
}

// OpenWorkspaceCreate prompts in the Minibuffer for a workspace name and
// optional `@/project/root` path (Doom SPC l c).
func (f *Frame) OpenWorkspaceCreate() {
	f.OpenMinibuffer(minibuffer.ModeWorkspaceCreate)
}

// CreateWorkspaceFromSpec parses `"name"` or `"name @/path/to/project"` from
// the Minibuffer and activates the new isolated workspace.
func (f *Frame) CreateWorkspaceFromSpec(spec string) {
	raw := strings.TrimSpace(spec)
	name := raw
	root := ""
	if idx := strings.Index(raw, "@"); idx >= 0 {
		name = strings.TrimSpace(raw[:idx])
		root = strings.TrimSpace(raw[idx+1:])
	}
	f.CreateWorkspace(name, root)
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
	f.persistWorkspaces()
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

// helmInitialQuery seeds workspace-scoped `@prefix:<root> ` when the active
// Doom workspace is bound to a relative project directory (projectile parity).
func (f *Frame) helmInitialQuery() string {
	if f.wsMgr == nil {
		return ""
	}
	root := strings.TrimSpace(f.wsMgr.Snapshot().Active.ProjectRoot)
	if root == "" || strings.HasPrefix(root, "/") {
		return ""
	}
	return "@prefix:" + root + " "
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
	f.persistWorkspaces()
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
	f.persistWorkspaces()
	f.Echo("Split right: " + string(snap.Active.SecondaryBuffer))
}

// SplitHorizontal splits the active workspace top / bottom (Doom SPC w s).
func (f *Frame) SplitHorizontal(secondary buffer.ID) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.SplitHorizontal(secondary)
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo("Split below: " + string(snap.Active.SecondaryBuffer))
}

// DeleteOtherWindows collapses any split to a single window (Doom SPC w o).
func (f *Frame) DeleteOtherWindows() {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.DeleteOtherWindows()
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo("Single window: " + string(snap.Active.ActiveBuffer))
}

// CloseFocusedPane implements Doom `+workspace/close-window-or-workspace` (SPC w d).
func (f *Frame) CloseFocusedPane() {
	if f.wsMgr == nil {
		return
	}
	action, snap := f.wsMgr.CloseWindowOrWorkspace()
	switch action {
	case "closed-window":
		f.syncSplitStateLocked(snap.Active)
		f.persistWorkspaces()
		f.Echo("Closed window -> " + string(snap.Active.FocusedBuffer()))
	case "killed-workspace":
		f.syncFromWorkspaceLocked(snap.Active)
		f.persistWorkspaces()
		f.Echo("Closed workspace -> " + snap.Active.Name)
	default:
		f.Echo("Only one window open")
	}
}

// ResizeSplit widens (+delta) or narrows (-delta) the enclosing split pane.
func (f *Frame) ResizeSplit(delta float32) {
	if f.wsMgr == nil {
		return
	}
	ratio, snap := f.wsMgr.ResizeSplit(delta)
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo(fmt.Sprintf("Split ratio: %d%% / %d%%", int(ratio*100+0.5), 100-int(ratio*100+0.5)))
}

// BalanceSplit resets all splits in the active workspace to 50/50 (Doom SPC w e).
func (f *Frame) BalanceSplit() {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.BalanceSplit()
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo("Balanced windows (50% / 50%)")
}

// SwapSplitPanes swaps the focused window's buffer with the next window (Doom SPC w x).
func (f *Frame) SwapSplitPanes() {
	if f.wsMgr == nil {
		return
	}
	swapped, snap := f.wsMgr.SwapSplitPanes()
	if !swapped {
		f.Echo("No split active to swap")
		return
	}
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo("Swapped windows")
}

// CycleFocusPane advances focus to the next window in Doom winum order (SPC w w).
func (f *Frame) CycleFocusPane() {
	f.CycleFocusWindow(1)
}

// CycleFocusWindow steps window focus by delta (+1 next, -1 previous).
func (f *Frame) CycleFocusWindow(delta int) {
	if f.wsMgr == nil {
		return
	}
	leaf, snap := f.wsMgr.CycleFocusWindow(delta)
	f.syncSplitStateLocked(snap.Active)
	f.Echo(fmt.Sprintf("Window %d: %s", leaf.Number, leaf.Buffer))
}

// SelectWindowNumber jumps directly to Doom winum window 1..9 (SPC w 1..9).
func (f *Frame) SelectWindowNumber(num int) {
	if f.wsMgr == nil {
		return
	}
	leaf, ok, snap := f.wsMgr.SelectWindowNumber(num)
	if !ok {
		f.Echo(fmt.Sprintf("No window %d", num))
		return
	}
	f.syncSplitStateLocked(snap.Active)
	f.Echo(fmt.Sprintf("Window %d: %s", leaf.Number, leaf.Buffer))
}

// FocusWindowID focuses a specific split window leaf by ID (e.g. clicking its modeline).
func (f *Frame) FocusWindowID(id string) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.FocusWindowID(id)
	f.syncSplitStateLocked(snap.Active)
}

// FocusPane explicitly focuses a 0-based pane index in reading order.
func (f *Frame) FocusPane(pane int) {
	if f.wsMgr == nil {
		return
	}
	snap := f.wsMgr.FocusPane(pane)
	f.syncSplitStateLocked(snap.Active)
}

// ToggleWindowDedicated toggles Emacs 30 `toggle-window-dedicated` on the
// focused window (Doom SPC w t).
func (f *Frame) ToggleWindowDedicated() {
	if f.wsMgr == nil {
		return
	}
	dedicated, snap := f.wsMgr.ToggleWindowDedicated()
	f.syncSplitStateLocked(snap.Active)
	if dedicated {
		f.Echo("Window dedicated to " + string(snap.Active.FocusedBuffer()))
	} else {
		f.Echo("Window undedicated")
	}
}

// WinnerUndo restores the previous window tree in this workspace (Doom SPC w u).
func (f *Frame) WinnerUndo() {
	if f.wsMgr == nil {
		return
	}
	ok, snap := f.wsMgr.WinnerUndo()
	if !ok {
		f.Echo("No earlier window layout in this workspace")
		return
	}
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo(fmt.Sprintf("Winner undo (%d window(s))", snap.Active.WindowCount()))
}

// WinnerRedo re-applies an undone window tree in this workspace (Doom SPC w U).
func (f *Frame) WinnerRedo() {
	if f.wsMgr == nil {
		return
	}
	ok, snap := f.wsMgr.WinnerRedo()
	if !ok {
		f.Echo("No later window layout in this workspace")
		return
	}
	f.syncSplitStateLocked(snap.Active)
	f.persistWorkspaces()
	f.Echo(fmt.Sprintf("Winner redo (%d window(s))", snap.Active.WindowCount()))
}

func (f *Frame) syncSplitStateLocked(ws workspace.Perspective) {
	f.mu.Lock()
	f.activeBuffer = ws.FocusedBuffer()
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

func (f *Frame) buildSplitProps(
	wsSnap workspace.Snapshot,
	showChrome bool,
	schemeExts []schema.UIExtensionRegion,
	mapCtx mapper.Context,
	featHost *features.Host,
	renderPaneModeline func(c *ui.Context, leaf workspace.WindowLeafView),
) window.SplitProps {
	act := wsSnap.Active
	winCount := act.WindowCount()
	if winCount <= 1 || act.WindowTree == nil {
		return window.SplitProps{
			Mode:        workspace.SplitSingle,
			WindowCount: 1,
			SchemeExts:  schemeExts,
			MapCtx:      mapCtx,
			FeatHost:    featHost,
		}
	}
	return window.SplitProps{
		Mode:               act.SplitMode,
		SecondaryBuffer:    act.SecondaryBuffer,
		FocusedPane:        act.FocusedPane,
		Ratio:              act.SplitRatio,
		Tree:               act.WindowTree,
		FocusedWindowID:    act.FocusedWindowID,
		WindowCount:        winCount,
		ShowChrome:         showChrome,
		SchemeExts:         schemeExts,
		MapCtx:             mapCtx,
		FeatHost:           featHost,
		RenderPaneModeline: renderPaneModeline,
		OnFocusWindow:      f.FocusWindowID,
		OnFocusPane:        f.FocusPane,
		OnSwapPanes:        f.SwapSplitPanes,
		OnCloseSplit:       f.CloseFocusedPane,
	}
}

// ToggleDeepThink flips Deep Think reasoning locally and forwards
// `thinking.set <on|off>` over marlin.ui.v1 gRPC to the Rust authority host.
func (f *Frame) ToggleDeepThink() {
	if f.Adaptive == nil || f.Adaptive.Desktop == nil || f.Adaptive.Desktop.Transcript == nil {
		return
	}
	tr := f.Adaptive.Desktop.Transcript
	next := !tr.DeepThinkEnabled()
	tr.SetDeepThink(next)
	state := "off"
	if next {
		state = "on"
	}
	if f.Adaptive.Desktop.OnCommandInvoke != nil {
		f.Adaptive.Desktop.OnCommandInvoke("thinking.set " + state)
	}
	f.Echo("Deep Think: " + state)
	f.requestFrame()
}

// CiteInComposer inserts an `@cite:<ref>` token directly into the inline
// *conversation* frosted-glass Composer draft and transcript composer state.
func (f *Frame) CiteInComposer(ref string) {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return
	}
	token := "@cite:" + clean
	f.mu.Lock()
	cur := strings.TrimSpace(f.convComposerDraft)
	if cur != "" {
		f.convComposerDraft = cur + " " + token + " "
	} else {
		f.convComposerDraft = token + " "
	}
	draft := f.convComposerDraft
	f.mu.Unlock()
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.Transcript != nil {
		f.Adaptive.Desktop.Transcript.SetComposerText(draft)
	}
	f.Echo("Cited " + clean)
	f.requestFrame()
}

// OpenFilePreview loads a local file (bounded to 16KB) or Helm snippet into the
// governed *Preview* right-hand Doom Popup Drawer without disrupting the main layout.
func (f *Frame) OpenFilePreview(title, rawPath, snippet string, line int) {
	path := strings.TrimSpace(rawPath)
	if line <= 0 && path != "" {
		if base, linePart, ok := strings.Cut(path, ":"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(linePart)); err == nil && n > 0 {
				path = strings.TrimSpace(base)
				line = n
			}
		}
	}
	content := strings.TrimSpace(snippet)
	if path != "" {
		if body, ok := appconfig.ReadPreviewSnippet(path); ok {
			content = body
		}
	}
	if content == "" {
		content = fmt.Sprintf("// Preview unavailable locally for %s\n// Open via connected Rust workspace host or Helm Follow Preview.", path)
	}
	f.thinkingMu.Lock()
	f.filePreview = &buffer.FilePreviewState{
		Title:   strings.TrimSpace(title),
		Path:    path,
		Line:    line,
		Content: content,
	}
	f.thinkingMu.Unlock()
	f.SelectBuffer(buffer.Preview)
}

func (f *Frame) currentFilePreview() *buffer.FilePreviewState {
	f.thinkingMu.RLock()
	fp := f.filePreview
	f.thinkingMu.RUnlock()
	return fp
}

func (f *Frame) restoreSavedWorkspaces() {
	if f.wsMgr == nil || len(f.Prefs.Workspaces) == 0 {
		return
	}
	persps := make([]workspace.Perspective, 0, len(f.Prefs.Workspaces))
	for _, sw := range f.Prefs.Workspaces {
		bufs := make([]buffer.ID, 0, len(sw.Buffers))
		for _, b := range sw.Buffers {
			if strings.TrimSpace(b) != "" {
				bufs = append(bufs, buffer.ID(b))
			}
		}
		persps = append(persps, workspace.Perspective{
			ID:              sw.ID,
			Name:            sw.Name,
			ProjectRoot:     sw.ProjectRoot,
			ActiveBuffer:    buffer.ID(sw.ActiveBuffer),
			SplitMode:       workspace.SplitMode(sw.SplitMode),
			SecondaryBuffer: buffer.ID(sw.SecondaryBuffer),
			SplitRatio:      sw.SplitRatio,
			Buffers:         bufs,
		})
	}
	snap := f.wsMgr.RestorePerspectives(f.Prefs.ActiveWorkspaceID, persps)
	f.activeBuffer = snap.Active.ActiveBuffer
	f.openTabs = append([]buffer.ID(nil), snap.Active.Buffers...)
}

func (f *Frame) persistWorkspaces() {
	if f.wsMgr == nil {
		return
	}
	wsSnap := f.wsMgr.Snapshot()
	saved := make([]appconfig.SavedWorkspace, 0, len(wsSnap.Workspaces))
	for _, w := range wsSnap.Workspaces {
		bufs := make([]string, 0, len(w.Buffers))
		for _, b := range w.Buffers {
			bufs = append(bufs, string(b))
		}
		saved = append(saved, appconfig.SavedWorkspace{
			ID:              w.ID,
			Name:            w.Name,
			ProjectRoot:     w.ProjectRoot,
			ActiveBuffer:    string(w.ActiveBuffer),
			SplitMode:       string(w.SplitMode),
			SecondaryBuffer: string(w.SecondaryBuffer),
			SplitRatio:      w.SplitRatio,
			Buffers:         bufs,
		})
	}
	f.mu.Lock()
	f.Prefs.ActiveWorkspaceID = wsSnap.ActiveID
	f.Prefs.Workspaces = saved
	prefs := f.Prefs
	f.mu.Unlock()
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
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
