// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package meow_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/meow"
)

func TestMeowModalAndKeypadLifecycle(t *testing.T) {
	st := meow.New()
	if st.Mode() != meow.ModeNormal || st.Mode().String() != "NORMAL" {
		t.Fatalf("expected Doom Emacs NORMAL default mode, got %v", st.Mode())
	}
	st.SetKeypad("SPC")
	if st.Keypad() != "SPC" || meow.KeypadHints("SPC") == "" {
		t.Fatalf("expected SPC keypad state and hints")
	}
	st.SetKeypad("SPC b")
	if st.Keypad() != "SPC b" || meow.KeypadHints("SPC b") == "" {
		t.Fatalf("unexpected SPC b hints: %q", meow.KeypadHints("SPC b"))
	}
	st.EnterInsert()
	if st.Mode() != meow.ModeInsert || st.Mode().String() != "INSERT" || st.Keypad() != "" {
		t.Fatalf("expected EnterInsert to clear keypad and enter INSERT")
	}
	st.EnterNormal()
	if st.Mode() != meow.ModeNormal {
		t.Fatalf("expected EnterNormal to restore NORMAL")
	}
}

func TestMeowNativeEmacsGlobalChordsInInsertMode(t *testing.T) {
	st := meow.New()
	st.EnterInsert()
	gotWin := 0
	gotHelm := false
	refocus := false
	tt := ui.NewTester(func(c *ui.Context) {
		root := ui.Column(c.Key("root")).Fill().TabStop(false).AutoFocus()
		if refocus {
			refocus = false
			root.Focus()
		}
		meow.Dispatch(c, st, meow.Handlers{
			Scope:                root,
			OnReturnFocusToFrame: func() { refocus = true },
			OnOpenHelm:           func() { gotHelm = true },
			OnSelectWindowNumber: func(n int) { gotWin = n },
			OnHydraKey: func(ctx *ui.Context, scope ui.Element, prefix string) bool {
				if ctx.Shortcut(0, ui.KeyO) || scope.Shortcut(0, ui.KeyO) {
					st.ClearKeypad()
					return true
				}
				return false
			},
		})
		draft := ""
		root.Children(func() {
			ui.TextInputBase(c.Key("inp"), &draft)
		})
	}, 400, 300)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	tt.Key(0, ui.KeyO)
	tt.Frame()
	tt.Key(ui.Alt, ui.Key2)
	tt.Frame()
	if gotWin != 2 {
		t.Fatalf("expected M-2 in Insert mode with focused TextInput -> window 2, got %d (keypad=%q)", gotWin, st.Keypad())
	}
	tt.Key(ui.Ctrl, ui.KeyS)
	tt.Frame()
	if !gotHelm {
		t.Fatalf("expected C-s in Insert mode -> OnOpenHelm")
	}
	tt.Key(ui.Ctrl, ui.KeyX)
	tt.Frame()
	if st.Keypad() != "C-x" {
		t.Fatalf("expected C-x in Insert mode -> keypad 'C-x', got %q", st.Keypad())
	}
}
