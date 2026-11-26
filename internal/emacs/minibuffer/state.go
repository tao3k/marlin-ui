// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package minibuffer implements Marlin UI's single-line Emacs Minibuffer,
// Echo Area, inline Vertico candidate list (M-x with live marlin.ui.v1
// commands, switch-buffer, switch-themes), 120Hz Helm candidate list, and
// Meow Keypad Which-Key echo bar.
//
// Declared in dev/sysml/emacs-workbench.sysml (MUI-EMACS-A08) and
// dev/sysml/emacs-lifecycle.sysml (MUI-EMACS-L02).
package minibuffer

import (
	"sync"
	"time"

	"github.com/tao3k/marlin-mygo/engine/search"
)

// Mode selects the active Emacs Minibuffer interaction state.
type Mode uint8

const (
	ModePrompt Mode = iota
	ModeCommand
	ModeHelm
	ModeBufferSwitch
	ModeThemeSwitch
	ModeGateReview
)

// Candidate represents one Vertico completion candidate.
type Candidate struct {
	ID   string
	Name string
	Desc string
	Run  func()
}

// State holds thread-safe Minibuffer, Echo Area, and Helm row scratch state.
type State struct {
	mu          sync.Mutex
	mode        Mode
	input       string
	selected    int
	focusNext   bool
	echoText    string
	echoUntil   time.Time
	helmRowsBuf []search.VisibleRow
}

// New initializes a clean Emacs Minibuffer in prompt mode.
func New() *State {
	return &State{
		mode:        ModePrompt,
		focusNext:   true,
		helmRowsBuf: make([]search.VisibleRow, 0, 12),
	}
}

// Mode returns the currently active Minibuffer mode.
func (s *State) Mode() Mode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

// IsCompletionActive reports whether Vertico or Helm completion is open.
func (s *State) IsCompletionActive() bool {
	return s.Mode() != ModePrompt
}

// Open activates the Minibuffer in the given completion mode and focuses input.
func (s *State) Open(m Mode) {
	s.mu.Lock()
	s.mode = m
	s.input = ""
	s.selected = 0
	s.focusNext = true
	s.mu.Unlock()
}

// Close returns the Minibuffer cleanly to idle prompt mode.
func (s *State) Close() {
	s.mu.Lock()
	s.mode = ModePrompt
	s.input = ""
	s.selected = 0
	s.mu.Unlock()
}

// RequestFocus asks the next frame to focus the Minibuffer text input.
func (s *State) RequestFocus() {
	s.mu.Lock()
	s.focusNext = true
	s.mu.Unlock()
}

// Input returns the current Minibuffer input string.
func (s *State) Input() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.input
}

// SetInput updates the Minibuffer input string and resets candidate selection.
func (s *State) SetInput(val string) {
	s.mu.Lock()
	s.input = val
	s.selected = 0
	s.mu.Unlock()
}

// StepSelection moves the Vertico candidate highlight by delta.
func (s *State) StepSelection(delta int) {
	s.mu.Lock()
	s.selected += delta
	if s.selected < 0 {
		s.selected = 0
	}
	s.mu.Unlock()
}

// Echo sets a transient right-aligned message in the Emacs Echo Area.
func (s *State) Echo(msg string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 4 * time.Second
	}
	s.mu.Lock()
	s.echoText = msg
	s.echoUntil = time.Now().Add(ttl)
	s.mu.Unlock()
}

func (s *State) snapshot() (mode Mode, input string, selected int, focusNow bool, echo string) {
	s.mu.Lock()
	mode = s.mode
	input = s.input
	selected = s.selected
	focusNow = s.focusNext
	s.focusNext = false
	if s.echoText != "" && time.Now().Before(s.echoUntil) {
		echo = s.echoText
	}
	s.mu.Unlock()
	return
}
