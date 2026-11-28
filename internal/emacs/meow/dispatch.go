// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package meow

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Handlers decouples Meow/Emacs key dispatch from concrete Frame & Hydra actions.
type Handlers struct {
	// Scope is the root modal Frame element. Claiming shortcuts on Scope
	// guarantees focused descendant buttons never hijack Space or modal keys
	// before Meow's leader keymap runs, while c.FocusTakesText() isolates
	// active text inputs automatically.
	Scope                ui.Element
	IsMinibufferActive   func() bool
	OnCancel             func(notice string)
	OnCloseMinibuffer    func()
	OnDismissPopup       func() bool
	OnReturnFocusToFrame func()
	OnOpenCommands       func()
	OnOpenHelm           func()
	OnOpenBufferSwitch   func()
	OnOpenThemeSwitch    func()
	OnOpenHelp           func()
	OnOpenSettings       func()
	OnSelectWindowNumber func(num int)
	OnStepCandidate      func(delta int)
	OnStepHistory        func(delta int)
	OnStepCursor         func(delta int)
	OnFocusMinibuffer    func()
	// OnHydraKey dispatches a keypress to the active Hydra body; returns true if handled.
	OnHydraKey func(c *ui.Context, scope ui.Element, prefix string) bool
	// OnBufferKey dispatches major-mode single-key bindings (e.g. *magit* s/u/x/c/d/TAB/1-4/?/g/q)
	// in Normal mode when no Hydra keypad or text input is active; returns true if handled.
	OnBufferKey func(c *ui.Context, scope ui.Element) bool
}

func modalShortcut(c *ui.Context, scope ui.Element, mods ui.Modifiers, key ui.Key) bool {
	if c == nil {
		return false
	}
	if mods == 0 && key != ui.KeyEscape && c.FocusTakesText() {
		return false
	}
	scoped := false
	if scope != (ui.Element{}) {
		scoped = scope.Shortcut(mods, key)
	}
	global := c.Shortcut(mods, key)
	return scoped || global
}

// Dispatch processes one frame's Meow modal, Hydra leader, and universal Emacs chords.
func Dispatch(c *ui.Context, s *State, h Handlers) {
	if modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyG) {
		s.ClearKeypad()
		if h.OnReturnFocusToFrame != nil {
			h.OnReturnFocusToFrame()
		}
		if h.OnCancel != nil {
			h.OnCancel("Quit")
		}
		return
	}
	if modalShortcut(c, h.Scope, ui.Alt, ui.KeyX) || modalShortcut(c, h.Scope, ui.Cmd, ui.KeyK) {
		s.ClearKeypad()
		if h.OnOpenCommands != nil {
			h.OnOpenCommands()
		}
		return
	}
	if modalShortcut(c, h.Scope, ui.Cmd, ui.KeyP) {
		s.ClearKeypad()
		if h.OnOpenHelm != nil {
			h.OnOpenHelm()
		}
		return
	}
	if modalShortcut(c, h.Scope, ui.Cmd, ui.KeyB) {
		s.ClearKeypad()
		if h.OnOpenBufferSwitch != nil {
			h.OnOpenBufferSwitch()
		}
		return
	}
	if modalShortcut(c, h.Scope, ui.Cmd, ui.KeyComma) {
		s.ClearKeypad()
		if h.OnOpenSettings != nil {
			h.OnOpenSettings()
		}
		return
	}

	miniActive := h.IsMinibufferActive != nil && h.IsMinibufferActive()
	if modalShortcut(c, h.Scope, 0, ui.KeyEscape) {
		wasKeypad := s.Keypad() != ""
		s.ClearKeypad()
		if miniActive {
			if h.OnCloseMinibuffer != nil {
				h.OnCloseMinibuffer()
			}
			if h.OnReturnFocusToFrame != nil {
				h.OnReturnFocusToFrame()
			}
			return
		}
		if !wasKeypad {
			if s.Mode() == ModeNormal && !c.FocusTakesText() && h.OnDismissPopup != nil && h.OnDismissPopup() {
				if h.OnReturnFocusToFrame != nil {
					h.OnReturnFocusToFrame()
				}
				return
			}
			s.EnterNormal()
		}
		if h.OnReturnFocusToFrame != nil {
			h.OnReturnFocusToFrame()
		}
		return
	}

	// Vertico / Helm candidate navigation (C-n / C-p / Up / Down).
	if miniActive {
		if modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyN) || c.Shortcut(0, ui.KeyDown) {
			if h.OnStepCandidate != nil {
				h.OnStepCandidate(1)
			}
			return
		}
		if modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyP) || c.Shortcut(0, ui.KeyUp) {
			if h.OnStepCandidate != nil {
				h.OnStepCandidate(-1)
			}
			return
		}
		if modalShortcut(c, h.Scope, ui.Alt, ui.KeyP) {
			if h.OnStepHistory != nil {
				h.OnStepHistory(-1)
			}
			return
		}
		if modalShortcut(c, h.Scope, ui.Alt, ui.KeyN) {
			if h.OnStepHistory != nil {
				h.OnStepHistory(1)
			}
			return
		}
		return
	}

	// Active native Emacs Ctrl prefix (C-x, C-x t, C-x w, C-c, C-h) works
	// identically in BOTH Insert mode AND Normal mode!
	if kp := s.Keypad(); strings.HasPrefix(kp, "C-") {
		if h.OnHydraKey != nil && h.OnHydraKey(c, h.Scope, kp) {
			return
		}
		if modalShortcut(c, h.Scope, 0, ui.KeyQ) {
			s.ClearKeypad()
		}
		return
	}

	// Universal native Emacs global bindings (Insert & Normal modes, mirrored
	// from scheme/ui-meow/key-bindings.ss meow-global):
	//   C-x -> ctl-x-map (C-x 0/1/2/3/o/O/+/b/p/z/Z/C-c/w/t)
	//   C-c -> mode/winner-mode prefix (C-c u / C-c U)
	//   C-h -> help prefix (C-h r / C-h ?)
	//   C-s -> Helm search
	//   M-1..M-9 -> direct winum window selection
	switch {
	case modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyX):
		s.SetKeypad("C-x")
		if h.OnReturnFocusToFrame != nil {
			h.OnReturnFocusToFrame()
		}
		return
	case modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyC):
		s.SetKeypad("C-c")
		if h.OnReturnFocusToFrame != nil {
			h.OnReturnFocusToFrame()
		}
		return
	case modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyH):
		s.SetKeypad("C-h")
		if h.OnReturnFocusToFrame != nil {
			h.OnReturnFocusToFrame()
		}
		return
	case modalShortcut(c, h.Scope, ui.Ctrl, ui.KeyS):
		s.ClearKeypad()
		if h.OnOpenHelm != nil {
			h.OnOpenHelm()
		}
		return
	}
	if h.OnSelectWindowNumber != nil {
		numKeys := [9]ui.Key{
			ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5,
			ui.Key6, ui.Key7, ui.Key8, ui.Key9,
		}
		for idx, k := range numKeys {
			if modalShortcut(c, h.Scope, ui.Alt, k) {
				s.ClearKeypad()
				h.OnSelectWindowNumber(idx + 1)
				return
			}
		}
	}

	if s.Mode() != ModeNormal {
		return
	}

	// Isolate text inputs (e.g. Dashboard Quick-Ask, Add-Task, Settings inputs):
	// never intercept unmodified single-key Meow commands while a text field has focus.
	if c.FocusTakesText() {
		return
	}

	// Active Hydra / SPC Which-Key leader menu takes priority when open.
	if kp := s.Keypad(); kp != "" {
		if h.OnHydraKey != nil && h.OnHydraKey(c, h.Scope, kp) {
			return
		}
		if modalShortcut(c, h.Scope, 0, ui.KeyQ) {
			s.ClearKeypad()
		}
		return
	}

	switch {
	case modalShortcut(c, h.Scope, 0, ui.KeySpace):
		s.SetKeypad("SPC")
		return
	}
	// Major-mode buffer-local keys (e.g. Magit s/S/u/U/x/TAB/1-4/c/d/b/l/r/z/m/?/g/q/v/!).
	if h.OnBufferKey != nil && h.OnBufferKey(c, h.Scope) {
		return
	}
	switch {
	case modalShortcut(c, h.Scope, 0, ui.KeyI), modalShortcut(c, h.Scope, 0, ui.KeyA):
		s.EnterInsert()
		if h.OnFocusMinibuffer != nil {
			h.OnFocusMinibuffer()
		}
	case modalShortcut(c, h.Scope, 0, ui.KeyJ), modalShortcut(c, h.Scope, 0, ui.KeyDown):
		if h.OnStepCursor != nil {
			h.OnStepCursor(1)
		}
	case modalShortcut(c, h.Scope, 0, ui.KeyK), modalShortcut(c, h.Scope, 0, ui.KeyUp):
		if h.OnStepCursor != nil {
			h.OnStepCursor(-1)
		}
	case modalShortcut(c, h.Scope, 0, ui.KeySlash):
		if h.OnOpenHelm != nil {
			h.OnOpenHelm()
		}
	}
}
