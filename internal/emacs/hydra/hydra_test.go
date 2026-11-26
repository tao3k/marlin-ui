// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package hydra_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestHydraColumnGroupingAndPopupClick(t *testing.T) {
	ran := false
	exited := false
	body := hydra.Body{
		ID:     "leader",
		Prefix: "SPC",
		Title:  "Doom Leader",
		Heads: []hydra.Head{
			{Key: "SPC", KeyCode: ui.KeySpace, Label: "M-x commands", Column: "Commands", Exit: true, Run: func() { ran = true }},
			{Key: "b", KeyCode: ui.KeyB, Label: "+buffers", Column: "Navigation", IsPrefix: true},
			{Key: "t", KeyCode: ui.KeyT, Label: "+themes", Column: "Appearance", IsPrefix: true},
		},
	}

	cols := body.GroupColumns()
	if len(cols) != 3 {
		t.Fatalf("expected 3 grouped columns, got %d", len(cols))
	}

	th := theme.DoomOne()
	tt := ui.NewTester(func(c *ui.Context) {
		hydra.RenderPopup(c, body, th, func() { exited = true })
	}, 960, 260)

	if !tt.HasText("SPC-") || !tt.HasText("Doom Leader") || !tt.HasText("+buffers") || !tt.HasText("M-x commands") {
		t.Fatalf("expected Hydra popup text, got %v", tt.Texts())
	}
	if err := tt.Click("M-x commands"); err != nil {
		t.Fatalf("click hydra head: %v", err)
	}
	tt.Frame()
	if !ran || !exited {
		t.Fatalf("expected exit head to run and close hydra, got ran=%v exited=%v", ran, exited)
	}
}
