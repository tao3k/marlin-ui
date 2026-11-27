// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package tabs_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestCentaurTabsWindowBadgesBusyDraftAndGroupOverflow(t *testing.T) {
	var selectedBuf buffer.ID
	var selectedFeat string

	view := func(c *ui.Context) {
		tabs.Render(c, tabs.Props{
			Workspaces: []tabs.WorkspaceTab{
				{ID: "ws-1", Name: "main", Active: true},
				{ID: "ws-2", Name: "audit", Active: false},
			},
			OpenTabs:        []buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions},
			ActiveBuffer:    buffer.Conversation,
			ActiveFeatureID: "",
			FeatureBuffers: []tabs.FeatureTab{
				{ID: "chatroom", Title: "Agent Room", Group: "feature"},
				{ID: "scheme-ext:git-status", Title: "Git Status", Group: "scheme-ext"},
				{ID: "pkg:demo", Title: "Demo Plugin", Group: "plugin"},
			},
			WindowBadges: map[buffer.ID]int{
				buffer.Conversation:               1,
				buffer.ID("scheme-ext:git-status"): 2,
			},
			BusyBuffers: map[buffer.ID]bool{
				buffer.Conversation: true,
			},
			DraftBuffers: map[buffer.ID]bool{
				buffer.Sessions: true,
			},
			PopupLabel:      "*Messages*",
			PopupVisible:    false,
			Theme:           theme.DoomOne(),
			OnSelectBuffer:  func(id buffer.ID) { selectedBuf = id },
			OnSelectFeature: func(id string) { selectedFeat = id },
			OnTogglePopup:   func() {},
			OnNewSession:    func() {},
		})
	}

	tt := ui.NewTester(view, 1280, 60)
	tt.Frame()

	joined := strings.Join(tt.Texts(), " | ")
	for _, want := range []string{"1:main", "2:audit", "*Dashboard*", "*conversation*", "*Agent Room*", "*Git Status*", "*Demo Plugin*", "1", "2", "Popup: *Messages*"} {
		if !tt.HasText(want) {
			t.Fatalf("expected Centaur tab strip to render %q, got %q", want, joined)
		}
	}

	if err := tt.Click("*Dashboard*"); err != nil {
		t.Fatalf("expected *Dashboard* tab clickable: %v", err)
	}
	tt.Frame()
	if selectedBuf != buffer.Dashboard {
		t.Fatalf("expected clicking *Dashboard* to select buffer.Dashboard, got %q", selectedBuf)
	}

	if err := tt.Click("*Git Status*"); err != nil {
		t.Fatalf("expected *Git Status* Scheme extension tab clickable: %v", err)
	}
	tt.Frame()
	if selectedFeat != "scheme-ext:git-status" {
		t.Fatalf("expected clicking *Git Status* to select scheme-ext:git-status, got %q", selectedFeat)
	}
}
