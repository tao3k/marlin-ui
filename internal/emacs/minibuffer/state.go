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

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
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
	ModeWorkspaceCreate
	ModeMagitPrompt
	ModeDescribeKey
	ModeDescribeFunc
)

// Candidate represents one Vertico / Marginalia completion candidate.
type Candidate struct {
	ID         string
	Name       string
	Desc       string
	Annotation string // optional Marginalia right-aligned shortcut or metadata badge
	Run        func()
}

// helmVisualFrame holds the last committed non-empty Helm render frame so
// typing a new character never flashes the list, modeline counter, or preview
// pane blank during the 5-35ms async Rust gRPC round-trip (Double-Buffered
// Stale-While-Revalidate MVCC front-buffer).
type helmVisualFrame struct {
	valid          bool
	hadPrimaryRust bool // rust-graph or rust-dsl hits present
	hadAnyRust     bool // rust-graph, rust-dsl, or rust-commands hits present
	gen            uint64
	rows           []search.VisibleRow
	srcs           []search.SourceSummary
	totalHits      int
	selCand        search.Candidate
	hasSel         bool
	prevTitle      string
	prevBody       string
}

// State holds thread-safe Minibuffer, Echo Area, and Helm row scratch state.
// Async candidate presentation delegates to Marlin UI's Frame-wide double-buffered
// MVCC theme.FrameLatch so Helm and every future async surface share identical
// zero-flicker guarantees.
type State struct {
	mu          sync.Mutex
	mode        Mode
	input       string
	selected    int
	focusNext   bool
	echoText    string
	echoUntil   time.Time
	helmRowsBuf []search.VisibleRow
	helmLatch   *theme.FrameLatch[helmVisualFrame]
}

// New initializes a clean Emacs Minibuffer in prompt mode.
func New() *State {
	return &State{
		mode:        ModePrompt,
		focusNext:   true,
		helmRowsBuf: make([]search.VisibleRow, 0, 12),
		helmLatch:   theme.NewFrameLatch[helmVisualFrame](theme.DefaultLatchGrace),
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
	latch := s.helmLatch
	s.mu.Unlock()
	if latch != nil {
		latch.Reset()
	}
}

// Close returns the Minibuffer cleanly to idle prompt mode.
func (s *State) Close() {
	s.mu.Lock()
	s.mode = ModePrompt
	s.input = ""
	s.selected = 0
	latch := s.helmLatch
	s.mu.Unlock()
	if latch != nil {
		latch.Reset()
	}
}

// MarkHelmPending records that a new Helm query generation was dispatched so
// the Frame-wide theme.FrameLatch holds the previous committed frame steady
// until new hits arrive or the settle grace window expires (zero blank flash).
func (s *State) MarkHelmPending(gen uint64) {
	if s.helmLatch != nil {
		s.helmLatch.BeginTransition(gen)
	}
}

// ClearHelmPending marks the current query as settled (used when user toggles
// a source filter explicitly and wants immediate empty-state feedback).
func (s *State) ClearHelmPending() {
	if s.helmLatch != nil {
		s.helmLatch.Reset()
	}
}

// DisplayHelmTotal returns the FrameLatch-stabilized Helm candidate count so
// the Doom Modeline stays in lockstep with the latched Helm front-buffer.
func (s *State) DisplayHelmTotal(liveTotal int) int {
	if s.helmLatch != nil {
		if front, ok, latched := s.helmLatch.Front(); ok && latched {
			return front.totalHits
		}
	}
	return liveTotal
}

func hasPrimaryRustSource(srcs []search.SourceSummary) bool {
	for _, sm := range srcs {
		if (sm.ID == "rust-graph" || sm.ID == "rust-dsl") && sm.Count > 0 {
			return true
		}
	}
	return false
}

func hasAnyRustSource(srcs []search.SourceSummary) bool {
	for _, sm := range srcs {
		if (sm.ID == "rust-graph" || sm.ID == "rust-dsl" || sm.ID == "rust-commands") && sm.Count > 0 {
			return true
		}
	}
	return false
}

func (s *State) resolveHelmFrame(
	rustConnected bool,
	liveRows []search.VisibleRow,
	liveSrcs []search.SourceSummary,
	liveTotal int,
	liveGen uint64,
	liveSel search.Candidate,
	liveHasSel bool,
	livePrevTitle, livePrevBody string,
) (rows []search.VisibleRow, srcs []search.SourceSummary, total int, gen uint64, sel search.Candidate, hasSel bool, prevTitle, prevBody string, updating bool) {
	live := helmVisualFrame{
		valid:          len(liveRows) > 0,
		hadPrimaryRust: hasPrimaryRustSource(liveSrcs),
		hadAnyRust:     hasAnyRustSource(liveSrcs),
		gen:            liveGen,
		rows:           liveRows,
		srcs:           liveSrcs,
		totalHits:      liveTotal,
		selCand:        liveSel,
		hasSel:         liveHasSel,
		prevTitle:      livePrevTitle,
		prevBody:       livePrevBody,
	}
	if s.helmLatch == nil {
		return live.rows, live.srcs, live.totalHits, live.gen, live.selCand, live.hasSel, live.prevTitle, live.prevBody, false
	}
	resolved, holding := s.helmLatch.Resolve(
		live,
		liveGen,
		len(liveRows) > 0,
		func(front, cur helmVisualFrame) bool {
			return (rustConnected || front.hadAnyRust) &&
				((front.hadPrimaryRust && !cur.hadPrimaryRust) ||
					(!front.hadPrimaryRust && front.hadAnyRust && !cur.hadAnyRust) ||
					len(cur.rows) == 0)
		},
		func(front, cur helmVisualFrame) bool {
			return !rustConnected || cur.hadPrimaryRust || (!front.hadPrimaryRust && cur.hadAnyRust)
		},
		func(src, dst helmVisualFrame) helmVisualFrame {
			src.rows = append(dst.rows[:0], src.rows...)
			src.srcs = append(dst.srcs[:0], src.srcs...)
			return src
		},
	)
	return resolved.rows, resolved.srcs, resolved.totalHits, resolved.gen, resolved.selCand, resolved.hasSel, resolved.prevTitle, resolved.prevBody, holding
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
