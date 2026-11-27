// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package workspace

import (
	"testing"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
)

func TestWorkspaceIsolationAndDoomPopupLifecycle(t *testing.T) {
	mgr := NewManager([]buffer.ID{buffer.Dashboard, buffer.Conversation})

	// 1. Open *Help* -> routes into Bottom Popup Drawer without displacing *Dashboard*.
	asPopup, snap := mgr.OpenBuffer(buffer.Help)
	if !asPopup || !snap.Active.PopupVisible || snap.Active.PopupBuffer != buffer.Help {
		t.Fatalf("expected *Help* to open as popup drawer, got asPopup=%v active=%+v", asPopup, snap.Active)
	}
	if snap.Active.ActiveBuffer != buffer.Dashboard {
		t.Fatalf("expected main buffer to remain *Dashboard*, got %s", snap.Active.ActiveBuffer)
	}

	// 2. Dismiss + Toggle restores last popup cleanly.
	dismissed, snap := mgr.DismissPopup()
	if !dismissed || snap.Active.PopupVisible {
		t.Fatalf("expected popup dismissed cleanly")
	}
	visible, snap := mgr.TogglePopup()
	if !visible || snap.Active.PopupBuffer != buffer.Help {
		t.Fatalf("expected TogglePopup to restore *Help*, got visible=%v buf=%s", visible, snap.Active.PopupBuffer)
	}

	// 3. Raise promotes *Models* into main workspace tabs.
	_, _ = mgr.OpenBuffer(buffer.Models)
	promoted, snap := mgr.RaisePopup()
	if promoted != buffer.Models || snap.Active.ActiveBuffer != buffer.Models || snap.Active.PopupVisible {
		t.Fatalf("expected RaisePopup to promote *Models*, got promoted=%s active=%+v", promoted, snap.Active)
	}

	// 4. Create a second workspace "review" -> buffers are strictly isolated!
	review := mgr.CreateWorkspace("review", "/repo/marlin-ui")
	if review.ID != "review" {
		t.Fatalf("expected workspace review, got %+v", review)
	}
	_, _ = mgr.OpenBuffer(buffer.Packages)
	reviewSnap := mgr.Snapshot()
	if !containsBuf(reviewSnap.Active.Buffers, buffer.Packages) || containsBuf(reviewSnap.Active.Buffers, buffer.Models) {
		t.Fatalf("expected review workspace to isolate *Packages* from main's *Models*: %+v", reviewSnap.Active.Buffers)
	}

	// Switching back to main restores *Models* and hides *Packages*.
	mainWS, ok := mgr.Switch("main")
	if !ok || !containsBuf(mainWS.Buffers, buffer.Models) || containsBuf(mainWS.Buffers, buffer.Packages) {
		t.Fatalf("expected main workspace isolation on switch back: %+v", mainWS.Buffers)
	}

	// 5. Per-workspace multi-pane splits isolate and restore across workspace switches.
	splitSnap := mgr.SplitVertical(buffer.Conversation)
	if splitSnap.Active.SplitMode != SplitVertical || splitSnap.Active.SecondaryBuffer != buffer.Conversation || splitSnap.Active.FocusedPane != 1 {
		t.Fatalf("expected vertical split on main, got %+v", splitSnap.Active)
	}
	// Opening *Targets* while secondary pane is focused routes to SecondaryBuffer.
	_, splitSnap = mgr.OpenBuffer(buffer.Targets)
	if splitSnap.Active.SecondaryBuffer != buffer.Targets || splitSnap.Active.ActiveBuffer != buffer.Models {
		t.Fatalf("expected secondary pane to update to *Targets*, got primary=%s secondary=%s", splitSnap.Active.ActiveBuffer, splitSnap.Active.SecondaryBuffer)
	}
	// Switching to review workspace must still be single-pane!
	revWS, ok := mgr.SwitchByIndex(1)
	if !ok || revWS.SplitMode != SplitSingle {
		t.Fatalf("expected review workspace to remain SplitSingle, got %+v", revWS)
	}
	// Stepping back to main restores main's vertical split!
	backWS, ok := mgr.StepWorkspace(-1)
	if !ok || backWS.ID != "main" || backWS.SplitMode != SplitVertical || backWS.SecondaryBuffer != buffer.Targets {
		t.Fatalf("expected main split restored on StepWorkspace(-1), got %+v", backWS)
	}
	// CycleFocusPane & DeleteOtherWindows collapse cleanly to focused buffer.
	focused, _ := mgr.CycleFocusPane()
	if focused != 0 {
		t.Fatalf("expected CycleFocusPane -> 0, got %d", focused)
	}
	closed, afterClose := mgr.CloseFocusedPane()
	if !closed || afterClose.Active.SplitMode != SplitSingle || afterClose.Active.ActiveBuffer != buffer.Targets {
		t.Fatalf("expected closing pane 0 to promote secondary *Targets*, got closed=%v active=%+v", closed, afterClose.Active)
	}
	// Killing review workspace leaves main intact; killing last workspace is denied.
	killed, afterKill := mgr.KillWorkspace("review")
	if !killed || len(afterKill.Workspaces) != 1 {
		t.Fatalf("expected KillWorkspace(review) to succeed, got killed=%v count=%d", killed, len(afterKill.Workspaces))
	}
	killedLast, _ := mgr.KillWorkspace("main")
	if killedLast {
		t.Fatalf("expected KillWorkspace on last remaining workspace to fail closed")
	}

	// 6. Split resize clamping, balance, swap, and persisted workspace restoration.
	_ = mgr.SplitVertical(buffer.Conversation)
	r1, _ := mgr.ResizeSplit(0.15)
	if r1 < 0.64 || r1 > 0.66 {
		t.Fatalf("expected ResizeSplit(+0.15) ~0.65, got %v", r1)
	}
	rMax, _ := mgr.ResizeSplit(0.50)
	if rMax != 0.75 {
		t.Fatalf("expected ResizeSplit clamped to 0.75, got %v", rMax)
	}
	balSnap := mgr.BalanceSplit()
	if balSnap.Active.SplitRatio != 0.50 {
		t.Fatalf("expected BalanceSplit -> 0.50, got %v", balSnap.Active.SplitRatio)
	}
	beforePrimary := balSnap.Active.ActiveBuffer
	beforeSecondary := balSnap.Active.SecondaryBuffer
	swapped, swapSnap := mgr.SwapSplitPanes()
	if !swapped || swapSnap.Active.ActiveBuffer != beforeSecondary || swapSnap.Active.SecondaryBuffer != beforePrimary {
		t.Fatalf("expected SwapSplitPanes to swap %s <-> %s, got %+v", beforePrimary, beforeSecondary, swapSnap.Active)
	}

	// *Preview* routes into right-hand Doom Popup Drawer by default.
	asPrevPopup, prevSnap := mgr.OpenBuffer(buffer.Preview)
	if !asPrevPopup || prevSnap.Active.PopupBuffer != buffer.Preview || prevSnap.Active.PopupRule == nil || prevSnap.Active.PopupRule.Side != SideRight {
		t.Fatalf("expected *Preview* to open in right popup drawer, got %+v", prevSnap.Active)
	}

	// RestorePerspectives hydrates saved workspaces across launches.
	mgr2 := NewManager(nil)
	restored := mgr2.RestorePerspectives("lab", []Perspective{
		{ID: "main", Name: "main", ActiveBuffer: buffer.Dashboard, SplitMode: SplitSingle, Buffers: []buffer.ID{buffer.Dashboard}},
		{ID: "lab", Name: "Lab", ProjectRoot: "/repo/lab", ActiveBuffer: buffer.Conversation, SplitMode: SplitVertical, SecondaryBuffer: buffer.Sessions, SplitRatio: 0.62, Buffers: []buffer.ID{buffer.Conversation, buffer.Sessions}},
	})
	if restored.ActiveID != "lab" || restored.Active.ProjectRoot != "/repo/lab" || restored.Active.SplitRatio != 0.62 {
		t.Fatalf("expected RestorePerspectives to hydrate lab workspace, got %+v", restored)
	}
}

func TestDoomMultiWindowTreeWinumDedicatedAndWinnerMode(t *testing.T) {
	mgr := NewManager([]buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets})

	// 1. Nested splits build a 3-window tree with deterministic reading-order winum 1, 2, 3.
	_ = mgr.SplitVertical(buffer.Conversation)
	snap := mgr.SplitHorizontal(buffer.Sessions)
	if snap.Active.WindowCount() != 3 || snap.Active.FocusedWindowNumber() != 3 {
		t.Fatalf("expected 3-window split focused on window 3, got count=%d focus=%d",
			snap.Active.WindowCount(), snap.Active.FocusedWindowNumber())
	}

	// 2. Direct winum selection (SPC w 1) + Emacs 30 toggle-window-dedicated (SPC w t).
	leaf, ok, _ := mgr.SelectWindowNumber(1)
	if !ok || leaf.Buffer != buffer.Dashboard {
		t.Fatalf("expected SelectWindowNumber(1) -> *Dashboard*, got ok=%v leaf=%+v", ok, leaf)
	}
	ded, _ := mgr.ToggleWindowDedicated()
	if !ded {
		t.Fatalf("expected ToggleWindowDedicated -> true")
	}
	// Opening *Targets* while window 1 is dedicated MUST preserve *Dashboard* in window 1.
	_, afterOpen := mgr.OpenBuffer(buffer.Targets)
	wins := afterOpen.Active.Windows()
	if wins[0].Buffer != buffer.Dashboard || afterOpen.Active.FocusedBuffer() != buffer.Targets {
		t.Fatalf("expected dedicated window 1 preserved (*Dashboard*) and *Targets* routed to sibling, got %+v", wins)
	}

	// 3. DeleteOtherWindows collapses to 1 window; per-workspace winner-undo/redo restores 3 windows!
	collapsed := mgr.DeleteOtherWindows()
	if collapsed.Active.WindowCount() != 1 {
		t.Fatalf("expected DeleteOtherWindows -> 1 window, got %d", collapsed.Active.WindowCount())
	}
	undone, afterUndo := mgr.WinnerUndo()
	if !undone || afterUndo.Active.WindowCount() != 3 {
		t.Fatalf("expected WinnerUndo -> 3 windows, got undone=%v count=%d", undone, afterUndo.Active.WindowCount())
	}
	redone, afterRedo := mgr.WinnerRedo()
	if !redone || afterRedo.Active.WindowCount() != 1 {
		t.Fatalf("expected WinnerRedo -> 1 window, got redone=%v count=%d", redone, afterRedo.Active.WindowCount())
	}

	// 4. Doom +workspace/close-window-or-workspace kills a non-main workspace when closing its last window.
	_ = mgr.CreateWorkspace("scratch", "")
	act, afterClose := mgr.CloseWindowOrWorkspace()
	if act != "killed-workspace" || afterClose.ActiveID != "main" {
		t.Fatalf("expected CloseWindowOrWorkspace on single-window scratch workspace to kill it, got act=%q active=%q", act, afterClose.ActiveID)
	}
}

func TestSyncSchemeWindowsBuildsMultiWindowTreeAndPreservesWinnerUndo(t *testing.T) {
	mgr := NewManager([]buffer.ID{buffer.Dashboard, buffer.Conversation})

	synced := mgr.SyncSchemeWindows([]SchemeWindowSpec{
		{ID: "win-1", Buffer: buffer.Conversation},
		{ID: "win-2", Buffer: buffer.ID("scheme-ext:git-status")},
		{ID: "win-3", Buffer: buffer.Sessions},
	}, "win-2")

	wins := synced.Active.Windows()
	if len(wins) != 3 {
		t.Fatalf("expected 3 Scheme-synced windows, got %d", len(wins))
	}
	if wins[0].ID != "win-1" || wins[1].ID != "win-2" || wins[1].Buffer != "scheme-ext:git-status" || !wins[1].Focused {
		t.Fatalf("unexpected Scheme window layout or focus: %+v", wins)
	}

	// Winner-mode undo cleanly restores the pre-sync 1-window *Dashboard* layout!
	undone, restored := mgr.WinnerUndo()
	if !undone || restored.Active.WindowCount() != 1 || restored.Active.ActiveBuffer != buffer.Dashboard {
		t.Fatalf("expected WinnerUndo to restore pre-sync single *Dashboard* layout, got undone=%v count=%d buf=%q",
			undone, restored.Active.WindowCount(), restored.Active.ActiveBuffer)
	}
}
