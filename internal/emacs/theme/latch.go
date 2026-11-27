// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"sync"
	"time"
)

// DefaultLatchGrace is the bounded Stale-While-Revalidate settle ceiling before
// an in-flight async transition commits an empty or partial back-buffer.
const DefaultLatchGrace = 260 * time.Millisecond

// FrameLatch is Marlin UI's generic Frame-wide Double-Buffered MVCC presentation
// latch (front-buffer / back-buffer swap) for any Emacs Window, Buffer, Popup,
// Split Pane, or Minibuffer backed by asynchronous or multi-source data streams.
//
// By reading through FrameLatch.Resolve instead of exposing a live mutable
// write buffer directly to the 120Hz render pass, every surface built on the
// Emacs Frame hierarchy automatically achieves:
//   - Zero blank-flash when an upstream query/refresh resets live rows
//   - Zero partial-commit row jumps while concurrent backend streams settle
//   - Single-frame atomic swap once the new generation is ready
//   - Consistent cross-region telemetry (Front() keeps Modeline/Header in lockstep)
type FrameLatch[T any] struct {
	mu            sync.Mutex
	grace         time.Duration
	hasFront      bool
	front         T
	frontGen      uint64
	pendingGen    uint64
	pendingSince  time.Time
	pendingFrames int
}

// NewFrameLatch constructs a double-buffered presentation latch with the given
// settle grace window (defaults to DefaultLatchGrace when <= 0).
func NewFrameLatch[T any](grace time.Duration) *FrameLatch[T] {
	if grace <= 0 {
		grace = DefaultLatchGrace
	}
	return &FrameLatch[T]{grace: grace}
}

// BeginTransition marks generation gen as in-flight while keeping the previous
// committed front-buffer frozen and readable until Resolve commits gen.
func (l *FrameLatch[T]) BeginTransition(gen uint64) {
	l.mu.Lock()
	l.pendingGen = gen
	l.pendingSince = time.Now()
	l.pendingFrames = 0
	l.mu.Unlock()
}

// Reset discards both the front-buffer and any pending transition (used when
// closing a surface or switching an explicit filter that should reflect empty
// state immediately).
func (l *FrameLatch[T]) Reset() {
	l.mu.Lock()
	var zero T
	l.hasFront = false
	l.front = zero
	l.frontGen = 0
	l.pendingGen = 0
	l.pendingSince = time.Time{}
	l.pendingFrames = 0
	l.mu.Unlock()
}

// Front returns the currently committed front-buffer and reports whether an
// async transition is actively latched within its grace window.
func (l *FrameLatch[T]) Front() (val T, ok bool, latched bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.hasFront {
		return val, false, false
	}
	latched = !l.pendingSince.IsZero() &&
		(l.pendingFrames == 0 || time.Since(l.pendingSince) < l.graceWindowLocked())
	return l.front, true, latched
}

// Resolve selects between the stable MVCC front-buffer and the live back-buffer:
//   - If a transition is in-flight within its grace window AND shouldHold(front, live)
//     reports that the back-buffer is still incomplete/wiped, Resolve returns the
//     committed front-buffer with holding=true (zero flicker).
//   - Otherwise, if hasData is true, Resolve atomically swaps front = clone(live)
//     (and marks the pending generation settled when settled(live) is true).
//   - If hasData is false after grace expires, Resolve commits the empty live
//     value cleanly once and clears the stale front-buffer.
func (l *FrameLatch[T]) Resolve(
	live T,
	liveGen uint64,
	hasData bool,
	shouldHold func(front, live T) bool,
	isSettled func(front, live T) bool,
	clone func(src, dst T) T,
) (out T, holding bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	pendingActive := l.hasFront && !l.pendingSince.IsZero()
	inGrace := pendingActive && (l.pendingFrames == 0 || time.Since(l.pendingSince) < l.graceWindowLocked())
	if inGrace && shouldHold != nil && shouldHold(l.front, live) {
		l.pendingFrames++
		return l.front, true
	}

	if hasData {
		next := live
		if clone != nil {
			next = clone(live, l.front)
		}
		l.front = next
		l.frontGen = liveGen
		l.hasFront = true
		if liveGen >= l.pendingGen && (isSettled == nil || isSettled(l.front, live)) {
			l.pendingSince = time.Time{}
		}
		return live, false
	}

	var zero T
	l.hasFront = false
	l.front = zero
	l.frontGen = liveGen
	l.pendingSince = time.Time{}
	return live, false
}

func (l *FrameLatch[T]) graceWindowLocked() time.Duration {
	if l.grace > 0 {
		return l.grace
	}
	return DefaultLatchGrace
}
