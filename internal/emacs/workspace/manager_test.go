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
}
