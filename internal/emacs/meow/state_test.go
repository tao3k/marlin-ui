// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package meow_test

import (
	"testing"

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
