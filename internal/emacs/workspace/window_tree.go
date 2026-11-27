// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package workspace

import "github.com/tao3k/marlin-ui/internal/emacs/buffer"

// WindowNode represents one node in a workspace's recursive Emacs window tree
// (mirroring scheme/ui-workspaces WindowLeaf. and WindowSplit.).
//
// A leaf node has ID != "" and First == nil && Second == nil.
// An internal split node has Direction (SplitVertical | SplitHorizontal),
// Ratio in [0.20, 0.80], and non-nil First & Second children.
type WindowNode struct {
	ID        string
	Buffer    buffer.ID
	Dedicated bool
	Direction SplitMode
	Ratio     float32
	First     *WindowNode
	Second    *WindowNode
}

// WindowLeafView is a reading-order (left-to-right, top-to-bottom) projection
// of one live window leaf with its deterministic 1-based Doom `winum` number.
type WindowLeafView struct {
	ID        string
	Number    int // 1-based winum index (1..9+)
	Buffer    buffer.ID
	Dedicated bool
	Focused   bool
}

type windowLayoutSnap struct {
	tree            *WindowNode
	focusedWindowID string
}

// IsLeaf reports whether n is a terminal window displaying a buffer.
func (n *WindowNode) IsLeaf() bool {
	return n != nil && n.First == nil && n.Second == nil
}

// IsSplit reports whether n is an internal split container.
func (n *WindowNode) IsSplit() bool {
	return n != nil && n.First != nil && n.Second != nil
}

func cloneWindowTree(n *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	cp := *n
	cp.First = cloneWindowTree(n.First)
	cp.Second = cloneWindowTree(n.Second)
	return &cp
}

func collectWindowLeaves(n *WindowNode, focusedID string, out *[]WindowLeafView) {
	if n == nil {
		return
	}
	if n.IsLeaf() {
		*out = append(*out, WindowLeafView{
			ID:        n.ID,
			Number:    len(*out) + 1,
			Buffer:    n.Buffer,
			Dedicated: n.Dedicated,
			Focused:   n.ID == focusedID,
		})
		return
	}
	collectWindowLeaves(n.First, focusedID, out)
	collectWindowLeaves(n.Second, focusedID, out)
}

func findWindowLeaf(n *WindowNode, id string) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		if n.ID == id {
			return n
		}
		return nil
	}
	if hit := findWindowLeaf(n.First, id); hit != nil {
		return hit
	}
	return findWindowLeaf(n.Second, id)
}

func firstWindowLeaf(n *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		return n
	}
	if l := firstWindowLeaf(n.First); l != nil {
		return l
	}
	return firstWindowLeaf(n.Second)
}

func firstUndedicatedLeaf(n *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		if !n.Dedicated {
			return n
		}
		return nil
	}
	if l := firstUndedicatedLeaf(n.First); l != nil {
		return l
	}
	return firstUndedicatedLeaf(n.Second)
}

func containsWindowID(n *WindowNode, id string) bool {
	return findWindowLeaf(n, id) != nil
}

func mapWindowLeaf(n *WindowNode, id string, fn func(*WindowNode) *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		if n.ID == id {
			return fn(n)
		}
		cp := *n
		return &cp
	}
	return &WindowNode{
		Direction: n.Direction,
		Ratio:     n.Ratio,
		First:     mapWindowLeaf(n.First, id, fn),
		Second:    mapWindowLeaf(n.Second, id, fn),
	}
}

func deleteWindowLeaf(n *WindowNode, id string) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		if n.ID == id {
			return nil
		}
		cp := *n
		return &cp
	}
	nl := deleteWindowLeaf(n.First, id)
	nr := deleteWindowLeaf(n.Second, id)
	if nl == nil && nr == nil {
		return nil
	}
	if nl == nil {
		return nr
	}
	if nr == nil {
		return nl
	}
	return &WindowNode{
		Direction: n.Direction,
		Ratio:     n.Ratio,
		First:     nl,
		Second:    nr,
	}
}

func balanceWindowTree(n *WindowNode) *WindowNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		cp := *n
		return &cp
	}
	return &WindowNode{
		Direction: n.Direction,
		Ratio:     0.50,
		First:     balanceWindowTree(n.First),
		Second:    balanceWindowTree(n.Second),
	}
}

func clampSplitRatio(r float32) float32 {
	if r < 0.25 {
		return 0.25
	}
	if r > 0.75 {
		return 0.75
	}
	return r
}

// resizeEnclosingSplit adjusts the innermost split enclosing targetID by delta.
func resizeEnclosingSplit(n *WindowNode, targetID string, delta float32) (*WindowNode, float32, bool) {
	if n == nil || n.IsLeaf() {
		return n, 0.50, false
	}
	inLeft := containsWindowID(n.First, targetID)
	inRight := containsWindowID(n.Second, targetID)
	if !inLeft && !inRight {
		return n, n.Ratio, false
	}
	if inLeft {
		nl, ratio, handled := resizeEnclosingSplit(n.First, targetID, delta)
		if handled {
			return &WindowNode{Direction: n.Direction, Ratio: n.Ratio, First: nl, Second: cloneWindowTree(n.Second)}, ratio, true
		}
		nextRatio := clampSplitRatio(n.Ratio + delta)
		return &WindowNode{Direction: n.Direction, Ratio: nextRatio, First: cloneWindowTree(n.First), Second: cloneWindowTree(n.Second)}, nextRatio, true
	}
	nr, ratio, handled := resizeEnclosingSplit(n.Second, targetID, delta)
	if handled {
		return &WindowNode{Direction: n.Direction, Ratio: n.Ratio, First: cloneWindowTree(n.First), Second: nr}, ratio, true
	}
	// When focused on the second child (or root-level 2-pane split), preserve
	// intuitive primary-ratio widening (+delta widens primary pane).
	nextRatio := clampSplitRatio(n.Ratio + delta)
	return &WindowNode{Direction: n.Direction, Ratio: nextRatio, First: cloneWindowTree(n.First), Second: cloneWindowTree(n.Second)}, nextRatio, true
}

// Windows returns all live windows in deterministic Doom `winum` reading order.
func (p Perspective) Windows() []WindowLeafView {
	if p.WindowTree == nil {
		if p.ActiveBuffer == "" {
			return nil
		}
		return []WindowLeafView{{
			ID:      "w1",
			Number:  1,
			Buffer:  p.ActiveBuffer,
			Focused: true,
		}}
	}
	out := make([]WindowLeafView, 0, 4)
	collectWindowLeaves(p.WindowTree, p.FocusedWindowID, &out)
	return out
}

// WindowCount returns the number of live windows in this workspace.
func (p Perspective) WindowCount() int {
	return len(p.Windows())
}

// FocusedLeaf returns the focused window leaf projection.
func (p Perspective) FocusedLeaf() WindowLeafView {
	wins := p.Windows()
	for _, w := range wins {
		if w.Focused {
			return w
		}
	}
	if len(wins) > 0 {
		return wins[0]
	}
	return WindowLeafView{ID: "w1", Number: 1, Buffer: p.ActiveBuffer, Focused: true}
}

// FocusedWindowNumber returns the 1-based Doom `winum` number of the focused window.
func (p Perspective) FocusedWindowNumber() int {
	return p.FocusedLeaf().Number
}

// FocusedBuffer returns the buffer displayed in the currently focused window.
func (p Perspective) FocusedBuffer() buffer.ID {
	return p.FocusedLeaf().Buffer
}
