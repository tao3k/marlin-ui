// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestSingleLineColumnsClipWithoutSiblingOverlap(t *testing.T) {
	th := theme.NordBento()
	longCol1 := "SPDX-FileCopyrightText: 2026 tao3k team and Contributors · README.org · extra-long-overflow-check"
	col2 := "workspace.file · #blake3-256"

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c.Key("root")).Width(640).Gap(4).AlignItems(ui.Stretch).Children(func() {
			theme.TabulatedRow(c, "trow-long", []theme.TableColumn{
				{Label: "ID", Width: 60, Mono: true},
				{Label: longCol1, Width: 220, Mono: true},
				{Label: col2, Width: 0, Mono: true},
			}, false, th)
			theme.VerticoRow(c, "vrow-short", "Justfile", "short.sub", "", 240, false, th)
			theme.VerticoRow(c, "vrow-long", longCol1, "long.sub", "", 240, false, th)
			theme.VerticoRow(c, "vrow-match", longCol1, "match.sub", "SPDX", 240, true, th)
		})
	}, 640, 240)
	tt.Frame()

	// 1. TabulatedRow column 1 (not the outer ButtonBase Label) must never extend past column 2's X.
	r1, ok1 := tt.Find(longCol1)
	r2, ok2 := tt.Find(col2)
	if !ok1 || !ok2 {
		t.Fatalf("expected TabulatedRow columns rendered")
	}
	if r1.W > 220.5 || r1.X+r1.W > r2.X {
		t.Fatalf("TabulatedRow column overflowed into next sibling: col1=[%.1f..%.1f] (w=%.1f) col2.X=%.1f",
			r1.X, r1.X+r1.W, r1.W, r2.X)
	}

	// 2. VerticoRow subtitle column must stay aligned across short, long, and highlighted rows.
	rs, okS := tt.Find("short.sub")
	rl, okL := tt.Find("long.sub")
	rm, okM := tt.Find("match.sub")
	if !okS || !okL || !okM {
		t.Fatalf("expected all VerticoRow subtitles rendered")
	}
	if rl.X != rs.X || rm.X != rs.X {
		t.Fatalf("VerticoRow subtitle column drifted: short=%.1f long=%.1f match=%.1f", rs.X, rl.X, rm.X)
	}
	if rl.X+rl.W > 640.5 || rm.X+rm.W > 640.5 {
		t.Fatalf("VerticoRow overflowed container width 640: long=[%.1f..%.1f] match=[%.1f..%.1f]",
			rl.X, rl.X+rl.W, rm.X, rm.X+rm.W)
	}
}
