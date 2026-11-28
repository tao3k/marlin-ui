// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package workspace implements Marlin UI's Doom Emacs-inspired Perspective
// Workspace manager (persp-mode / :ui workspaces) and declarative Popup Window
// Rule engine (set-popup-rule!), mirrored 1:1 with scheme/ui-workspaces/.
package workspace

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
)

// PopupSide selects where a governed popup drawer docks relative to the main window.
type PopupSide string

const (
	SideBottom PopupSide = "bottom"
	SideRight  PopupSide = "right"
)

// PopupQuit specifies how ESC / quit interacts with an open popup drawer.
type PopupQuit string

const (
	QuitCurrent PopupQuit = "current"
	QuitOther   PopupQuit = "other"
	QuitNever   PopupQuit = "never"
)

// PopupRule mirrors Doom Emacs `set-popup-rule!` and scheme/ui-workspaces PopupRule.
type PopupRule struct {
	Matcher  buffer.ID
	Side     PopupSide
	Size     float32 // fractional ratio in [0.20, 0.60]
	Select   bool
	Quit     PopupQuit
	Modeline bool
}

// DefaultPopupRules returns Marlin's curated Doom popup rules keeping auxiliary
// buffers (*Help*, *Messages*, *Processes*, *Models*) out of the primary layout.
// *Sessions* and *Targets* remain main workspace tabs by default so direct tab
// clicks preserve 1:1 workbench navigation, while *Help*/*Messages*/*Processes*/*Models*
// glide into sleek non-destructive frosted glass drawers.
func DefaultPopupRules() []PopupRule {
	return []PopupRule{
		{Matcher: buffer.Help, Side: SideBottom, Size: 0.36, Select: true, Quit: QuitCurrent},
		{Matcher: buffer.Messages, Side: SideBottom, Size: 0.30, Select: false, Quit: QuitCurrent},
		{Matcher: buffer.Processes, Side: SideBottom, Size: 0.34, Select: true, Quit: QuitCurrent},
		{Matcher: buffer.Models, Side: SideRight, Size: 0.38, Select: true, Quit: QuitCurrent},
		{Matcher: buffer.Preview, Side: SideRight, Size: 0.44, Select: true, Quit: QuitCurrent},
	}
}

// SplitMode selects single-pane or split-pane window layout within a workspace.
type SplitMode string

const (
	SplitSingle     SplitMode = "single"
	SplitVertical   SplitMode = "vertical"   // left | right (SPC w v)
	SplitHorizontal SplitMode = "horizontal" // top / bottom (SPC w s)
)

// Perspective holds one workspace's isolated window tree, split layout,
// open tabs, per-workspace winner-mode history, and workspace-local popup
// drawer state (matching Doom persp-mode + winum + winner-mode).
type Perspective struct {
	ID              string
	Name            string
	ProjectRoot     string
	SessionID       string
	ActiveBuffer    buffer.ID
	SplitMode       SplitMode
	SecondaryBuffer buffer.ID
	SplitRatio      float32 // primary pane ratio in [0.25, 0.75], default 0.50
	FocusedPane     int     // 0-based index of focused leaf in winum reading order
	WindowTree      *WindowNode
	FocusedWindowID string
	NextWindowSeq   int
	winnerUndo      []windowLayoutSnap
	winnerRedo      []windowLayoutSnap
	zoomSaved       *windowLayoutSnap
	Buffers         []buffer.ID
	PopupBuffer     buffer.ID
	PopupRule       *PopupRule
	PopupVisible    bool
	LastPopup       buffer.ID
}

// Snapshot is an immutable read-only view of all workspaces and the active
// workspace's isolated buffers and popup drawer for lock-free 120Hz rendering.
type Snapshot struct {
	ActiveID   string
	Active     Perspective
	Workspaces []Perspective
}

// Manager coordinates perspective workspaces, per-workspace buffer isolation,
// and declarative Doom popup window rules.
type Manager struct {
	mu         sync.RWMutex
	activeID   string
	workspaces []Perspective
	rules      []PopupRule
	registers  map[string]WindowRegister
	nextSeq    int
}

// NewManager initializes the single primary "main" workspace with a clean
// minimal tab set (*Dashboard* + *conversation*) and Doom popup rules.
func NewManager(initialTabs []buffer.ID) *Manager {
	tabs := append([]buffer.ID(nil), initialTabs...)
	if len(tabs) == 0 {
		tabs = []buffer.ID{buffer.Dashboard, buffer.Conversation}
	}
	return &Manager{
		activeID: "main",
		workspaces: []Perspective{
			{
				ID:              "main",
				Name:            "main",
				ActiveBuffer:    tabs[0],
				SplitMode:       SplitSingle,
				SplitRatio:      0.50,
				WindowTree:      &WindowNode{ID: "w1", Buffer: tabs[0]},
				FocusedWindowID: "w1",
				NextWindowSeq:   1,
				Buffers:         tabs,
			},
		},
		rules:   DefaultPopupRules(),
		nextSeq: 1,
	}
}

// Snapshot returns a deep-copied immutable snapshot for lock-free UI frames.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wsCopy := make([]Perspective, len(m.workspaces))
	var active Perspective
	for i, w := range m.workspaces {
		cp := clonePerspective(w)
		wsCopy[i] = cp
		if cp.ID == m.activeID {
			active = cp
		}
	}
	return Snapshot{
		ActiveID:   m.activeID,
		Active:     active,
		Workspaces: wsCopy,
	}
}

// SetPopupRule installs or overrides a declarative popup rule (most recent wins).
func (m *Manager) SetPopupRule(rule PopupRule) {
	if rule.Size < 0.20 {
		rule.Size = 0.35
	} else if rule.Size > 0.60 {
		rule.Size = 0.60
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]PopupRule, 0, len(m.rules)+1)
	filtered = append(filtered, rule)
	for _, r := range m.rules {
		if r.Matcher != rule.Matcher {
			filtered = append(filtered, r)
		}
	}
	m.rules = filtered
}

// MatchPopupRule resolves whether buf is governed by a popup rule.
func (m *Manager) MatchPopupRule(buf buffer.ID) (PopupRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.matchRuleLocked(buf)
}

func (m *Manager) matchRuleLocked(buf buffer.ID) (PopupRule, bool) {
	for _, r := range m.rules {
		if r.Matcher == buf {
			return r, true
		}
	}
	return PopupRule{}, false
}

// CreateWorkspace creates and switches to a fresh isolated workspace with its
// own independent buffer list, its own dedicated *chat:<id>* buffer instance,
// and clean popup state (never hijacking another workspace's *conversation*).
func (m *Manager) CreateWorkspace(name, projectRoot string) Perspective {
	m.mu.Lock()
	defer m.mu.Unlock()
	cleanRoot := strings.TrimSpace(projectRoot)
	cleanName := strings.TrimSpace(name)
	if cleanName == "" && cleanRoot != "" {
		trimmed := strings.TrimRight(cleanRoot, "/")
		if idx := strings.LastIndex(trimmed, "/"); idx >= 0 && idx+1 < len(trimmed) {
			cleanName = trimmed[idx+1:]
		}
	}
	if cleanName == "" {
		// Pick the lowest unused workspace slot (ws-2, ws-3, ...) instead of
		// jumping to stale persisted sequence numbers like ws-5.
		for n := 2; ; n++ {
			cand := fmt.Sprintf("ws-%d", n)
			taken := false
			for _, w := range m.workspaces {
				if w.ID == cand || w.Name == cand {
					taken = true
					break
				}
			}
			if !taken {
				cleanName = cand
				break
			}
		}
	}
	m.nextSeq++
	id := strings.ToLower(strings.ReplaceAll(cleanName, " ", "-"))
	for _, w := range m.workspaces {
		if w.ID == id {
			id = fmt.Sprintf("%s-%d", id, m.nextSeq)
			break
		}
	}
	wsChatBuf := buffer.NewChatBufferID(id)
	ws := Perspective{
		ID:              id,
		Name:            cleanName,
		ProjectRoot:     cleanRoot,
		SessionID:       "marlin-" + id,
		ActiveBuffer:    buffer.Dashboard,
		SplitMode:       SplitSingle,
		SplitRatio:      0.50,
		WindowTree:      &WindowNode{ID: "w1", Buffer: buffer.Dashboard},
		FocusedWindowID: "w1",
		NextWindowSeq:   1,
		Buffers:         []buffer.ID{buffer.Dashboard, wsChatBuf},
	}
	m.workspaces = append(m.workspaces, ws)
	m.activeID = ws.ID
	return clonePerspective(ws)
}

// SetActiveProjectRoot updates the active workspace's bound Git/jj project root
// (used by Doom Emacs `project-switch-project` / `SPC p p`).
func (m *Manager) SetActiveProjectRoot(root string) Perspective {
	m.mu.Lock()
	defer m.mu.Unlock()
	clean := strings.TrimSpace(root)
	for i := range m.workspaces {
		if m.workspaces[i].ID == m.activeID {
			m.workspaces[i].ProjectRoot = clean
			return clonePerspective(m.workspaces[i])
		}
	}
	return Perspective{}
}

// Switch activates the target workspace and restores its isolated buffer set,
// split layout, and popup drawer state.
func (m *Manager) Switch(id string) (Perspective, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range m.workspaces {
		if w.ID == id {
			m.activeID = id
			return clonePerspective(w), true
		}
	}
	return Perspective{}, false
}

// SwitchByIndex activates the 0-based workspace index (SPC l 1..4).
func (m *Manager) SwitchByIndex(idx int) (Perspective, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx < 0 || idx >= len(m.workspaces) {
		return Perspective{}, false
	}
	m.activeID = m.workspaces[idx].ID
	return clonePerspective(m.workspaces[idx]), true
}

// StepWorkspace cycles to the next (+1) or previous (-1) workspace.
func (m *Manager) StepWorkspace(delta int) (Perspective, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.workspaces)
	if n <= 1 {
		return Perspective{}, false
	}
	cur := m.activeIndexLocked()
	if cur < 0 {
		cur = 0
	}
	next := (cur + delta) % n
	if next < 0 {
		next += n
	}
	m.activeID = m.workspaces[next].ID
	return clonePerspective(m.workspaces[next]), true
}

// KillWorkspace removes a workspace unless it is the last remaining perspective.
// If id is empty, kills the currently active workspace.
func (m *Manager) KillWorkspace(id string) (killed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.workspaces) <= 1 {
		return false, m.snapshotLocked()
	}
	target := strings.TrimSpace(id)
	if target == "" {
		target = m.activeID
	}
	remaining := make([]Perspective, 0, len(m.workspaces))
	for _, w := range m.workspaces {
		if w.ID == target {
			killed = true
			continue
		}
		remaining = append(remaining, w)
	}
	if !killed || len(remaining) == 0 {
		return false, m.snapshotLocked()
	}
	m.workspaces = remaining
	if m.activeID == target {
		m.activeID = remaining[0].ID
	}
	return true, m.snapshotLocked()
}

func ensureWindowTree(ws *Perspective) {
	if ws.WindowTree == nil {
		baseBuf := ws.ActiveBuffer
		if baseBuf == "" {
			baseBuf = buffer.Dashboard
		}
		if (ws.SplitMode == SplitVertical || ws.SplitMode == SplitHorizontal) && ws.SecondaryBuffer != "" {
			ratio := ws.SplitRatio
			if ratio < 0.25 || ratio > 0.75 {
				ratio = 0.50
			}
			ws.WindowTree = &WindowNode{
				Direction: ws.SplitMode,
				Ratio:     ratio,
				First:     &WindowNode{ID: "w1", Buffer: baseBuf},
				Second:    &WindowNode{ID: "w2", Buffer: ws.SecondaryBuffer},
			}
			ws.NextWindowSeq = 2
			if ws.FocusedPane == 1 {
				ws.FocusedWindowID = "w2"
			} else {
				ws.FocusedWindowID = "w1"
			}
		} else {
			ws.WindowTree = &WindowNode{ID: "w1", Buffer: baseBuf}
			ws.FocusedWindowID = "w1"
			ws.NextWindowSeq = 1
		}
	}
	if ws.NextWindowSeq <= 0 {
		ws.NextWindowSeq = len(ws.Windows())
		if ws.NextWindowSeq <= 0 {
			ws.NextWindowSeq = 1
		}
	}
	if findWindowLeaf(ws.WindowTree, ws.FocusedWindowID) == nil {
		if first := firstWindowLeaf(ws.WindowTree); first != nil {
			ws.FocusedWindowID = first.ID
		}
	}
	syncLegacySplitFields(ws)
}

func syncLegacySplitFields(ws *Perspective) {
	leaves := ws.Windows()
	if len(leaves) == 0 {
		return
	}
	if findWindowLeaf(ws.WindowTree, ws.FocusedWindowID) == nil {
		ws.FocusedWindowID = leaves[0].ID
		leaves = ws.Windows()
	}
	ws.ActiveBuffer = leaves[0].Buffer
	ws.FocusedPane = 0
	for i, l := range leaves {
		if l.ID == ws.FocusedWindowID {
			ws.FocusedPane = i
			break
		}
	}
	if ws.WindowTree != nil && ws.WindowTree.IsSplit() {
		ws.SplitMode = ws.WindowTree.Direction
		ws.SplitRatio = clampSplitRatio(ws.WindowTree.Ratio)
		if len(leaves) >= 2 {
			ws.SecondaryBuffer = leaves[1].Buffer
		}
	} else {
		ws.SplitMode = SplitSingle
		ws.SplitRatio = 0.50
		ws.SecondaryBuffer = ""
	}
}

func recordWinnerSnapshot(ws *Perspective) {
	const maxWinnerHistory = 24
	if ws.WindowTree == nil {
		return
	}
	ws.winnerUndo = append(ws.winnerUndo, windowLayoutSnap{
		tree:            cloneWindowTree(ws.WindowTree),
		focusedWindowID: ws.FocusedWindowID,
	})
	if len(ws.winnerUndo) > maxWinnerHistory {
		ws.winnerUndo = ws.winnerUndo[len(ws.winnerUndo)-maxWinnerHistory:]
	}
	ws.winnerRedo = nil
}

// SplitVertical splits the focused window left | right (Doom SPC w v).
func (m *Manager) SplitVertical(secondary buffer.ID) Snapshot {
	return m.applySplit(SplitVertical, secondary)
}

// SplitHorizontal splits the focused window top / bottom (Doom SPC w s).
func (m *Manager) SplitHorizontal(secondary buffer.ID) Snapshot {
	return m.applySplit(SplitHorizontal, secondary)
}

func (m *Manager) applySplit(mode SplitMode, secondary buffer.ID) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	recordWinnerSnapshot(ws)

	focusLeaf := findWindowLeaf(ws.WindowTree, ws.FocusedWindowID)
	if focusLeaf == nil {
		focusLeaf = firstWindowLeaf(ws.WindowTree)
	}
	if secondary == "" {
		// Split based on the currently focused window's buffer (Emacs / Ghostty / tmux).
		secondary = focusLeaf.Buffer
	}
	ws.NextWindowSeq++
	newID := fmt.Sprintf("w%d", ws.NextWindowSeq)
	newLeaf := &WindowNode{ID: newID, Buffer: secondary}
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, focusLeaf.ID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		return &WindowNode{
			Direction: mode,
			Ratio:     0.50,
			First:     &cp,
			Second:    newLeaf,
		}
	})
	// Auto-balance split tree across the split axis (50/50 for 2 panes, 33.3% each for 3 panes).
	ws.WindowTree = balanceWindowTree(ws.WindowTree)
	ws.FocusedWindowID = newID
	if !strings.HasPrefix(string(secondary), "scheme-ext:") && !strings.HasPrefix(string(secondary), "feature:") && !containsBuf(ws.Buffers, secondary) {
		ws.Buffers = append(ws.Buffers, secondary)
	}
	syncLegacySplitFields(ws)
	return m.snapshotLocked()
}

// ResizeSplit adjusts the enclosing split ratio of the focused window by delta,
// clamped to [0.25, 0.75] (Doom SPC w l / SPC w h).
func (m *Manager) ResizeSplit(delta float32) (ratio float32, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return 0.50, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	nextTree, nextRatio, handled := resizeEnclosingSplit(ws.WindowTree, ws.FocusedWindowID, delta)
	if handled {
		ws.WindowTree = nextTree
		syncLegacySplitFields(ws)
		return nextRatio, m.snapshotLocked()
	}
	return ws.SplitRatio, m.snapshotLocked()
}

// BalanceSplit resets all split ratios in the active workspace to 50/50
// (Doom SPC w e / C-x +).
func (m *Manager) BalanceSplit() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if ws.WindowCount() > 1 {
		recordWinnerSnapshot(ws)
		ws.WindowTree = balanceWindowTree(ws.WindowTree)
		syncLegacySplitFields(ws)
	}
	return m.snapshotLocked()
}

// SwapSplitPanes swaps the focused window's buffer with the next window in
// reading order (Doom SPC w x).
func (m *Manager) SwapSplitPanes() (swapped bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	leaves := ws.Windows()
	if len(leaves) < 2 {
		return false, m.snapshotLocked()
	}
	recordWinnerSnapshot(ws)
	curIdx := ws.FocusedPane
	if curIdx < 0 || curIdx >= len(leaves) {
		curIdx = 0
	}
	nextIdx := (curIdx + 1) % len(leaves)
	a, b := leaves[curIdx], leaves[nextIdx]
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, a.ID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = b.Buffer
		return &cp
	})
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, b.ID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Buffer = a.Buffer
		return &cp
	})
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

// RestorePerspectives hydrates saved workspaces from persisted UserPrefs while
// validating split ratios and guaranteeing at least one valid active workspace.
func (m *Manager) RestorePerspectives(activeID string, saved []Perspective) Snapshot {
	if len(saved) == 0 {
		return m.Snapshot()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	restored := make([]Perspective, 0, len(saved))
	for _, w := range saved {
		id := strings.TrimSpace(w.ID)
		name := strings.TrimSpace(w.Name)
		if id == "" {
			continue
		}
		if name == "" {
			name = id
		}
		bufs := append([]buffer.ID(nil), w.Buffers...)
		if len(bufs) == 0 {
			bufs = []buffer.ID{buffer.Dashboard, buffer.Conversation}
		}
		act := w.ActiveBuffer
		if act == "" {
			act = bufs[0]
		}
		mode := w.SplitMode
		if mode != SplitVertical && mode != SplitHorizontal {
			mode = SplitSingle
			w.SecondaryBuffer = ""
		}
		ratio := w.SplitRatio
		if ratio < 0.15 || ratio > 0.85 {
			ratio = 0.50
		}
		p := Perspective{
			ID:              id,
			Name:            name,
			ProjectRoot:     strings.TrimSpace(w.ProjectRoot),
			ActiveBuffer:    act,
			SplitMode:       mode,
			SecondaryBuffer: w.SecondaryBuffer,
			SplitRatio:      ratio,
			WindowTree:      cloneWindowTree(w.WindowTree),
			FocusedWindowID: w.FocusedWindowID,
			NextWindowSeq:   w.NextWindowSeq,
			Buffers:         bufs,
		}
		ensureWindowTree(&p)
		restored = append(restored, p)
	}
	if len(restored) == 0 {
		return m.snapshotLocked()
	}
	m.workspaces = restored
	m.nextSeq = len(restored)
	m.activeID = restored[0].ID
	for _, w := range restored {
		if w.ID == activeID {
			m.activeID = w.ID
			break
		}
	}
	return m.snapshotLocked()
}

// DeleteOtherWindows collapses any active splits into a single window keeping
// the focused window's buffer active (Doom SPC w o / C-x 1).
func (m *Manager) DeleteOtherWindows() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if ws.WindowCount() > 1 {
		recordWinnerSnapshot(ws)
		focusLeaf := ws.FocusedLeaf()
		ws.WindowTree = &WindowNode{
			ID:        focusLeaf.ID,
			Buffer:    focusLeaf.Buffer,
			Dedicated: focusLeaf.Dedicated,
		}
		ws.FocusedWindowID = focusLeaf.ID
		syncLegacySplitFields(ws)
	}
	return m.snapshotLocked()
}

// CloseFocusedPane closes the currently focused window when split (Doom SPC w d).
func (m *Manager) CloseFocusedPane() (closed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	leaves := ws.Windows()
	if len(leaves) <= 1 {
		return false, m.snapshotLocked()
	}
	recordWinnerSnapshot(ws)
	curIdx := ws.FocusedPane
	ws.WindowTree = balanceWindowTree(deleteWindowLeaf(ws.WindowTree, ws.FocusedWindowID))
	nextLeaves := ws.Windows()
	if len(nextLeaves) > 0 {
		if curIdx >= len(nextLeaves) {
			curIdx = len(nextLeaves) - 1
		}
		if curIdx < 0 {
			curIdx = 0
		}
		ws.FocusedWindowID = nextLeaves[curIdx].ID
	}
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

// CloseWindowOrWorkspace implements Doom Emacs `+workspace/close-window-or-workspace`:
// closes the focused window if > 1 exist, otherwise kills the current workspace
// if it is non-main and other workspaces remain.
func (m *Manager) CloseWindowOrWorkspace() (action string, snap Snapshot) {
	if closed, s := m.CloseFocusedPane(); closed {
		return "closed-window", s
	}
	m.mu.Lock()
	idx := m.activeIndexLocked()
	canKillWS := idx >= 0 && len(m.workspaces) > 1 && m.workspaces[idx].ID != "main"
	targetID := ""
	if canKillWS {
		targetID = m.workspaces[idx].ID
	}
	m.mu.Unlock()
	if canKillWS {
		if killed, s := m.KillWorkspace(targetID); killed {
			return "killed-workspace", s
		}
	}
	return "noop", m.Snapshot()
}

// CycleFocusPane cycles focus to the next window in winum reading order (Doom SPC w w).
func (m *Manager) CycleFocusPane() (focused int, snap Snapshot) {
	leaf, s := m.CycleFocusWindow(1)
	if leaf.Number > 0 {
		return leaf.Number - 1, s
	}
	return 0, s
}

// CycleFocusWindow cycles focus by delta (+1 next, -1 prev) in Doom winum order.
func (m *Manager) CycleFocusWindow(delta int) (focused WindowLeafView, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return WindowLeafView{}, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	leaves := ws.Windows()
	if len(leaves) <= 1 {
		return ws.FocusedLeaf(), m.snapshotLocked()
	}
	nextIdx := (ws.FocusedPane + delta) % len(leaves)
	if nextIdx < 0 {
		nextIdx += len(leaves)
	}
	ws.FocusedWindowID = leaves[nextIdx].ID
	syncLegacySplitFields(ws)
	return ws.FocusedLeaf(), m.snapshotLocked()
}

// SelectWindowNumber jumps directly to the 1-based Doom `winum` window (SPC w 1..9).
func (m *Manager) SelectWindowNumber(num int) (focused WindowLeafView, ok bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return WindowLeafView{}, false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	leaves := ws.Windows()
	if num < 1 || num > len(leaves) {
		return ws.FocusedLeaf(), false, m.snapshotLocked()
	}
	ws.FocusedWindowID = leaves[num-1].ID
	syncLegacySplitFields(ws)
	return ws.FocusedLeaf(), true, m.snapshotLocked()
}

// FocusWindowID explicitly focuses the window leaf matching id.
func (m *Manager) FocusWindowID(id string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if findWindowLeaf(ws.WindowTree, id) != nil {
		ws.FocusedWindowID = id
		syncLegacySplitFields(ws)
	}
	return m.snapshotLocked()
}

// FocusPane explicitly focuses the 0-based pane index in reading order.
func (m *Manager) FocusPane(pane int) Snapshot {
	if pane < 0 {
		pane = 0
	}
	_, _, snap := m.SelectWindowNumber(pane + 1)
	return snap
}

// ToggleWindowDedicated toggles Emacs 30 `toggle-window-dedicated` on the
// focused window (SPC w t), protecting its buffer from accidental replacement.
func (m *Manager) ToggleWindowDedicated() (dedicated bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Dedicated = !cp.Dedicated
		dedicated = cp.Dedicated
		return &cp
	})
	syncLegacySplitFields(ws)
	return dedicated, m.snapshotLocked()
}

// SetWindowDedicated explicitly sets Emacs `dedicated-window-mode` state on the
// focused window leaf, returning the updated Snapshot.
func (m *Manager) SetWindowDedicated(dedicated bool) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	ws.WindowTree = mapWindowLeaf(ws.WindowTree, ws.FocusedWindowID, func(leaf *WindowNode) *WindowNode {
		cp := *leaf
		cp.Dedicated = dedicated
		return &cp
	})
	syncLegacySplitFields(ws)
	return m.snapshotLocked()
}

// WinnerUndo restores the previous window configuration in the active workspace
// (Doom per-workspace `winner-mode` / SPC w u).
func (m *Manager) WinnerUndo() (ok bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if len(ws.winnerUndo) == 0 {
		return false, m.snapshotLocked()
	}
	prev := ws.winnerUndo[len(ws.winnerUndo)-1]
	ws.winnerUndo = ws.winnerUndo[:len(ws.winnerUndo)-1]
	ws.winnerRedo = append(ws.winnerRedo, windowLayoutSnap{
		tree:            cloneWindowTree(ws.WindowTree),
		focusedWindowID: ws.FocusedWindowID,
	})
	ws.WindowTree = cloneWindowTree(prev.tree)
	ws.FocusedWindowID = prev.focusedWindowID
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

// WinnerRedo re-applies an undone window configuration in the active workspace
// (Doom per-workspace `winner-mode` / SPC w U).
func (m *Manager) WinnerRedo() (ok bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	ensureWindowTree(ws)
	if len(ws.winnerRedo) == 0 {
		return false, m.snapshotLocked()
	}
	nxt := ws.winnerRedo[len(ws.winnerRedo)-1]
	ws.winnerRedo = ws.winnerRedo[:len(ws.winnerRedo)-1]
	ws.winnerUndo = append(ws.winnerUndo, windowLayoutSnap{
		tree:            cloneWindowTree(ws.WindowTree),
		focusedWindowID: ws.FocusedWindowID,
	})
	ws.WindowTree = cloneWindowTree(nxt.tree)
	ws.FocusedWindowID = nxt.focusedWindowID
	syncLegacySplitFields(ws)
	return true, m.snapshotLocked()
}

func defaultSecondaryBuffer(primary buffer.ID) buffer.ID {
	switch primary {
	case buffer.Conversation:
		return buffer.Sessions
	case buffer.Dashboard:
		return buffer.Conversation
	default:
		return buffer.Conversation
	}
}

func (m *Manager) activeIndexLocked() int {
	for i := range m.workspaces {
		if m.workspaces[i].ID == m.activeID {
			return i
		}
	}
	if len(m.workspaces) > 0 {
		return 0
	}
	return -1
}

func (m *Manager) snapshotLocked() Snapshot {
	wsCopy := make([]Perspective, len(m.workspaces))
	var active Perspective
	for i, w := range m.workspaces {
		cp := clonePerspective(w)
		wsCopy[i] = cp
		if cp.ID == m.activeID {
			active = cp
		}
	}
	return Snapshot{
		ActiveID:   m.activeID,
		Active:     active,
		Workspaces: wsCopy,
	}
}

func clonePerspective(p Perspective) Perspective {
	cp := p
	cp.Buffers = append([]buffer.ID(nil), p.Buffers...)
	cp.WindowTree = cloneWindowTree(p.WindowTree)
	cp.winnerUndo = nil
	cp.winnerRedo = nil
	if p.PopupRule != nil {
		r := *p.PopupRule
		cp.PopupRule = &r
	}
	return cp
}

func containsBuf(list []buffer.ID, id buffer.ID) bool {
	for _, b := range list {
		if b == id {
			return true
		}
	}
	return false
}
