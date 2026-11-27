// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestDesktopDashboardProfileUsesUnifiedMarlinEmblem(t *testing.T) {
	sig := widgets.NewSignals()
	sig.Set("agent.session", "sess-demo")
	sig.Set("agent.model", "deepseek/deepseek-chat")
	sig.Set("agent.host", "127.0.0.1:9000")
	sig.Set("agent.status", "ONLINE")

	var clickedAction string
	bc := &buffer.Context{
		DashboardNode: widgets.DefaultDashboard(nil),
		Signals:       sig,
		OnWidgetAction: func(act string) {
			clickedAction = act
		},
	}

	th := theme.DoomOne()
	tt := ui.NewTester(func(c *ui.Context) {
		buffer.Render(c, buffer.Dashboard, th, bc)
	}, 1280, 820)
	tt.Frame()

	for _, want := range []string{"WORKSPACE", "Marlin AI", "@sess-demo", "deepseek/deepseek-chat", "127.0.0.1:9000", "ONLINE"} {
		if !tt.HasText(want) {
			t.Fatalf("expected Desktop *Dashboard* profile card text %q, got %q", want, strings.Join(tt.Texts(), " | "))
		}
	}

	// Clicking the Marlin AI profile card must still route to *conversation*.
	if err := tt.Click("Marlin AI"); err != nil {
		t.Fatalf("click Marlin AI profile card: %v", err)
	}
	tt.Frame()
	if clickedAction != "buffer:*conversation*" {
		t.Fatalf("expected clicking Marlin AI profile card to trigger buffer:*conversation*, got %q", clickedAction)
	}
}
