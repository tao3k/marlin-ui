// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package meow implements Marlin UI's Meow modal editing state machine
// (NORMAL default on Doom Dashboard, INSERT for typing, and SPC Keypad/Hydra
// prefix tree) plus universal Emacs keybindings.
//
// Declared in dev/sysml/emacs-workbench.sysml (MUI-EMACS-A03) and
// dev/sysml/emacs-lifecycle.sysml (MUI-EMACS-L01).
package meow

import "sync"

// Mode tracks the active Meow modal editing state.
type Mode uint8

const (
	ModeInsert Mode = iota
	ModeNormal
)

// String returns the Doom Modeline label for the Meow mode.
func (m Mode) String() string {
	if m == ModeNormal {
		return "NORMAL"
	}
	return "INSERT"
}

// State holds thread-safe Meow modal and SPC Hydra leader state.
type State struct {
	mu     sync.Mutex
	mode   Mode
	keypad string // "", "SPC", "SPC b", "SPC t", "SPC m"
}

// New initializes Meow state in NORMAL mode (authentic Doom Emacs startup so
// pressing SPC immediately pops up the Doom Leader Hydra Minibuffer).
func New() *State {
	return &State{mode: ModeNormal}
}

// Mode returns the current modal state.
func (s *State) Mode() Mode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

// Keypad returns the active SPC leader prefix (empty when not in Hydra/keypad).
func (s *State) Keypad() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keypad
}

// EnterNormal transitions to NORMAL mode and clears any pending keypad.
func (s *State) EnterNormal() {
	s.mu.Lock()
	s.mode = ModeNormal
	s.keypad = ""
	s.mu.Unlock()
}

// EnterInsert transitions to INSERT mode and clears any pending keypad.
func (s *State) EnterInsert() {
	s.mu.Lock()
	s.mode = ModeInsert
	s.keypad = ""
	s.mu.Unlock()
}

// SetKeypad updates the active SPC Hydra prefix (e.g. "SPC", "SPC b", "SPC t", "SPC m").
func (s *State) SetKeypad(prefix string) {
	s.mu.Lock()
	s.keypad = prefix
	s.mu.Unlock()
}

// ClearKeypad resets the active keypad prefix back to resting mode.
func (s *State) ClearKeypad() {
	s.mu.Lock()
	s.keypad = ""
	s.mu.Unlock()
}

// KeypadHints returns a concise fallback legend for the active keypad prefix.
func KeypadHints(prefix string) string {
	switch prefix {
	case "SPC b":
		return "b switch-buffer · c *conversation* · s *Sessions* · t *Targets* · C-g cancel"
	case "SPC t":
		return "t switch-themes · 1 doom-one · 2 pi · 3 doom-paper · n cycle · C-g cancel"
	case "SPC m":
		return "n new-session · p probe-targets · , settings · i insert · C-g cancel"
	default:
		return "SPC M-x · b +buffers · s helm · t +themes · m +session · h help · , settings · C-g cancel"
	}
}
