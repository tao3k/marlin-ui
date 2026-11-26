// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package meow

import "github.com/egoist/mygo/ui"

// Handlers decouples Meow/Emacs key dispatch from concrete Frame & Hydra actions.
type Handlers struct {
	IsMinibufferActive func() bool
	OnCancel           func(notice string)
	OnCloseMinibuffer  func()
	OnOpenCommands     func()
	OnOpenHelm         func()
	OnOpenBufferSwitch func()
	OnOpenThemeSwitch  func()
	OnOpenHelp         func()
	OnOpenSettings     func()
	OnStepCandidate    func(delta int)
	OnStepCursor       func(delta int)
	OnFocusMinibuffer  func()
	// OnHydraKey dispatches a keypress to the active Hydra body; returns true if handled.
	OnHydraKey func(c *ui.Context, prefix string) bool
}

// Dispatch processes one frame's Meow modal, Hydra leader, and universal Emacs chords.
func Dispatch(c *ui.Context, s *State, h Handlers) {
	if c.Shortcut(ui.Ctrl, ui.KeyG) {
		s.ClearKeypad()
		if h.OnCancel != nil {
			h.OnCancel("Quit")
		}
		return
	}
	if c.Shortcut(ui.Alt, ui.KeyX) || c.Shortcut(ui.Cmd, ui.KeyK) {
		s.ClearKeypad()
		if h.OnOpenCommands != nil {
			h.OnOpenCommands()
		}
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		s.ClearKeypad()
		if h.OnOpenHelm != nil {
			h.OnOpenHelm()
		}
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		s.ClearKeypad()
		if h.OnOpenBufferSwitch != nil {
			h.OnOpenBufferSwitch()
		}
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		s.ClearKeypad()
		if h.OnOpenSettings != nil {
			h.OnOpenSettings()
		}
		return
	}

	miniActive := h.IsMinibufferActive != nil && h.IsMinibufferActive()
	if c.Shortcut(0, ui.KeyEscape) {
		wasKeypad := s.Keypad() != ""
		s.ClearKeypad()
		if miniActive {
			if h.OnCloseMinibuffer != nil {
				h.OnCloseMinibuffer()
			}
			return
		}
		if !wasKeypad {
			s.EnterNormal()
		}
		return
	}

	// Vertico / Helm candidate navigation (C-n / C-p / Up / Down).
	if miniActive {
		if c.Shortcut(ui.Ctrl, ui.KeyN) || c.Shortcut(0, ui.KeyDown) {
			if h.OnStepCandidate != nil {
				h.OnStepCandidate(1)
			}
			return
		}
		if c.Shortcut(ui.Ctrl, ui.KeyP) || c.Shortcut(0, ui.KeyUp) {
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

	// Active Hydra / SPC Which-Key leader menu takes priority when open.
	if kp := s.Keypad(); kp != "" {
		if h.OnHydraKey != nil && h.OnHydraKey(c, kp) {
			return
		}
		if c.Shortcut(0, ui.KeyQ) {
			s.ClearKeypad()
		}
		return
	}

	switch {
	case c.Shortcut(0, ui.KeySpace):
		s.SetKeypad("SPC")
	case c.Shortcut(0, ui.KeyI), c.Shortcut(0, ui.KeyA):
		s.EnterInsert()
		if h.OnFocusMinibuffer != nil {
			h.OnFocusMinibuffer()
		}
	case c.Shortcut(0, ui.KeyJ), c.Shortcut(0, ui.KeyDown):
		if h.OnStepCursor != nil {
			h.OnStepCursor(1)
		}
	case c.Shortcut(0, ui.KeyK), c.Shortcut(0, ui.KeyUp):
		if h.OnStepCursor != nil {
			h.OnStepCursor(-1)
		}
	case c.Shortcut(0, ui.KeySlash):
		if h.OnOpenHelm != nil {
			h.OnOpenHelm()
		}
	}
}
