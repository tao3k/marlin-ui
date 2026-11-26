// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package meow

import "github.com/egoist/mygo/ui"

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
	OnReturnFocusToFrame func()
	OnOpenCommands       func()
	OnOpenHelm           func()
	OnOpenBufferSwitch   func()
	OnOpenThemeSwitch    func()
	OnOpenHelp           func()
	OnOpenSettings       func()
	OnStepCandidate      func(delta int)
	OnStepCursor         func(delta int)
	OnFocusMinibuffer    func()
	// OnHydraKey dispatches a keypress to the active Hydra body; returns true if handled.
	OnHydraKey func(c *ui.Context, scope ui.Element, prefix string) bool
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
		return
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
