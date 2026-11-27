// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"testing"
	"time"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestFrameLatchPreventsBlankFlashAndSwapsAtomically(t *testing.T) {
	type viewState struct {
		items   []string
		hasSync bool
	}
	latch := theme.NewFrameLatch[viewState](40 * time.Millisecond)

	clone := func(src, dst viewState) viewState {
		return viewState{
			items:   append(dst.items[:0], src.items...),
			hasSync: src.hasSync,
		}
	}
	hold := func(front, live viewState) bool {
		return len(live.items) == 0 || (front.hasSync && !live.hasSync)
	}
	settled := func(front, live viewState) bool {
		return !front.hasSync || live.hasSync
	}

	// 1. Initial commit of generation 1.
	g1 := viewState{items: []string{"alpha.rs", "beta.rs"}, hasSync: true}
	out, holding := latch.Resolve(g1, 1, true, hold, settled, clone)
	if holding || len(out.items) != 2 || out.items[0] != "alpha.rs" {
		t.Fatalf("expected gen 1 committed immediately, got holding=%v out=%+v", holding, out)
	}

	// 2. Begin transition to generation 2 while live buffer is wiped empty -> MUST hold gen 1.
	latch.BeginTransition(2)
	out, holding = latch.Resolve(viewState{}, 2, false, hold, settled, clone)
	if !holding || len(out.items) != 2 || out.items[0] != "alpha.rs" {
		t.Fatalf("expected FrameLatch to hold front-buffer during empty wipe, got holding=%v out=%+v", holding, out)
	}
	if front, ok, latched := latch.Front(); !ok || !latched || len(front.items) != 2 {
		t.Fatalf("expected Front() to report latched front-buffer for cross-region telemetry, got ok=%v latched=%v", ok, latched)
	}

	// 3. Partial local-only update (hasSync=false) while async source still pending -> MUST keep holding gen 1.
	out, holding = latch.Resolve(viewState{items: []string{"local-buf"}, hasSync: false}, 2, true, hold, settled, clone)
	if !holding || out.items[0] != "alpha.rs" {
		t.Fatalf("expected FrameLatch to suppress partial local-only jump, got holding=%v out=%+v", holding, out)
	}

	// 4. Async gen 2 completes -> atomic single-frame swap!
	g2 := viewState{items: []string{"gamma.rs"}, hasSync: true}
	out, holding = latch.Resolve(g2, 2, true, hold, settled, clone)
	if holding || len(out.items) != 1 || out.items[0] != "gamma.rs" {
		t.Fatalf("expected atomic swap to gen 2, got holding=%v out=%+v", holding, out)
	}

	// 5. Genuine 0-hit query settles cleanly once grace expires.
	latch.BeginTransition(3)
	_, holding = latch.Resolve(viewState{}, 3, false, hold, settled, clone)
	if !holding {
		t.Fatalf("expected first frame of gen 3 held")
	}
	time.Sleep(55 * time.Millisecond)
	out, holding = latch.Resolve(viewState{}, 3, false, hold, settled, clone)
	if holding || len(out.items) != 0 {
		t.Fatalf("expected clean 0-hit commit after grace window, got holding=%v out=%+v", holding, out)
	}
}
