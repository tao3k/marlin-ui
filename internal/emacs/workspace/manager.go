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
	}
}

// SplitMode selects single-pane or split-pane window layout within a workspace.
type SplitMode string

const (
	SplitSingle     SplitMode = "single"
	SplitVertical   SplitMode = "vertical"   // left | right (SPC w v)
	SplitHorizontal SplitMode = "horizontal" // top / bottom (SPC w s)
)

// Perspective holds one workspace's isolated window state, split layout,
// open tabs, and workspace-local popup drawer state (matching Doom persp-mode).
type Perspective struct {
	ID              string
	Name            string
	ProjectRoot     string
	ActiveBuffer    buffer.ID
	SplitMode       SplitMode
	SecondaryBuffer buffer.ID
	FocusedPane     int // 0 = primary pane, 1 = secondary pane
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
	nextSeq    int
}

// NewManager initializes the primary "main" workspace with default tabs and
// Doom popup rules.
func NewManager(initialTabs []buffer.ID) *Manager {
	tabs := append([]buffer.ID(nil), initialTabs...)
	if len(tabs) == 0 {
		tabs = []buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets}
	}
	return &Manager{
		activeID: "main",
		workspaces: []Perspective{
			{
				ID:           "main",
				Name:         "main",
				ActiveBuffer: tabs[0],
				SplitMode:    SplitSingle,
				Buffers:      tabs,
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
// own independent buffer list and clean popup state.
func (m *Manager) CreateWorkspace(name, projectRoot string) Perspective {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextSeq++
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = fmt.Sprintf("ws-%d", m.nextSeq)
	}
	id := strings.ToLower(strings.ReplaceAll(cleanName, " ", "-"))
	for _, w := range m.workspaces {
		if w.ID == id {
			id = fmt.Sprintf("%s-%d", id, m.nextSeq)
			break
		}
	}
	ws := Perspective{
		ID:           id,
		Name:         cleanName,
		ProjectRoot:  strings.TrimSpace(projectRoot),
		ActiveBuffer: buffer.Conversation,
		SplitMode:    SplitSingle,
		Buffers:      []buffer.ID{buffer.Dashboard, buffer.Conversation},
	}
	m.workspaces = append(m.workspaces, ws)
	m.activeID = ws.ID
	return clonePerspective(ws)
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

// SplitVertical splits the active workspace left | right (Doom SPC w v).
func (m *Manager) SplitVertical(secondary buffer.ID) Snapshot {
	return m.applySplit(SplitVertical, secondary)
}

// SplitHorizontal splits the active workspace top / bottom (Doom SPC w s).
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
	if secondary == "" {
		secondary = defaultSecondaryBuffer(ws.ActiveBuffer)
	}
	ws.SplitMode = mode
	ws.SecondaryBuffer = secondary
	ws.FocusedPane = 1
	if !containsBuf(ws.Buffers, secondary) {
		ws.Buffers = append(ws.Buffers, secondary)
	}
	return m.snapshotLocked()
}

// DeleteOtherWindows collapses any active split into a single window keeping
// the focused pane's buffer active (Doom SPC w o / C-x 1).
func (m *Manager) DeleteOtherWindows() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.SplitMode != SplitSingle && ws.FocusedPane == 1 && ws.SecondaryBuffer != "" {
		ws.ActiveBuffer = ws.SecondaryBuffer
	}
	ws.SplitMode = SplitSingle
	ws.SecondaryBuffer = ""
	ws.FocusedPane = 0
	return m.snapshotLocked()
}

// CloseFocusedPane closes the currently focused pane when split (Doom SPC w d).
func (m *Manager) CloseFocusedPane() (closed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.SplitMode == SplitSingle {
		return false, m.snapshotLocked()
	}
	if ws.FocusedPane == 0 && ws.SecondaryBuffer != "" {
		ws.ActiveBuffer = ws.SecondaryBuffer
	}
	ws.SplitMode = SplitSingle
	ws.SecondaryBuffer = ""
	ws.FocusedPane = 0
	return true, m.snapshotLocked()
}

// CycleFocusPane alternates focus between pane 0 and pane 1 (Doom SPC w w).
func (m *Manager) CycleFocusPane() (focused int, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return 0, Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.SplitMode == SplitSingle || ws.SecondaryBuffer == "" {
		ws.FocusedPane = 0
		return 0, m.snapshotLocked()
	}
	if ws.FocusedPane == 0 {
		ws.FocusedPane = 1
	} else {
		ws.FocusedPane = 0
	}
	return ws.FocusedPane, m.snapshotLocked()
}

// FocusPane explicitly focuses pane 0 (primary) or pane 1 (secondary).
func (m *Manager) FocusPane(pane int) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.SplitMode == SplitSingle || ws.SecondaryBuffer == "" || pane <= 0 {
		ws.FocusedPane = 0
	} else {
		ws.FocusedPane = 1
	}
	return m.snapshotLocked()
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

// OpenBuffer routes buf through Doom popup rules:
//   - If buf matches a popup rule AND is not already promoted into this workspace's
//     main buffer tabs, it opens non-destructively in the Popup Drawer (returning openedAsPopup=true).
//   - Otherwise it focuses buf in the active pane (primary or secondary when split)
//     and records it strictly in this workspace's isolated buffer list.
func (m *Manager) OpenBuffer(buf buffer.ID) (openedAsPopup bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	alreadyMainTab := containsBuf(ws.Buffers, buf)
	if rule, ok := m.matchRuleLocked(buf); ok && !alreadyMainTab {
		rc := rule
		ws.PopupBuffer = buf
		ws.PopupRule = &rc
		ws.PopupVisible = true
		ws.LastPopup = buf
		return true, m.snapshotLocked()
	}
	if ws.SplitMode != SplitSingle && ws.FocusedPane == 1 {
		ws.SecondaryBuffer = buf
	} else {
		ws.ActiveBuffer = buf
	}
	if !alreadyMainTab {
		ws.Buffers = append(ws.Buffers, buf)
	}
	return false, m.snapshotLocked()
}

// DismissPopup closes the active workspace's popup drawer without touching the
// main buffer or tab layout.
func (m *Manager) DismissPopup() (dismissed bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 || !m.workspaces[idx].PopupVisible {
		return false, m.snapshotLocked()
	}
	ws := &m.workspaces[idx]
	ws.PopupVisible = false
	ws.PopupBuffer = ""
	ws.PopupRule = nil
	return true, m.snapshotLocked()
}

// TogglePopup toggles the active workspace's last popup drawer (Doom C-x p).
func (m *Manager) TogglePopup() (visible bool, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 {
		return false, Snapshot{}
	}
	ws := &m.workspaces[idx]
	if ws.PopupVisible {
		ws.PopupVisible = false
		ws.PopupBuffer = ""
		ws.PopupRule = nil
		return false, m.snapshotLocked()
	}
	if ws.LastPopup != "" {
		if rule, ok := m.matchRuleLocked(ws.LastPopup); ok {
			rc := rule
			ws.PopupBuffer = ws.LastPopup
			ws.PopupRule = &rc
			ws.PopupVisible = true
			return true, m.snapshotLocked()
		}
	}
	return false, m.snapshotLocked()
}

// RaisePopup promotes the active popup buffer into a first-class main window
// buffer inside the active workspace (Doom +popup/raise).
func (m *Manager) RaisePopup() (promoted buffer.ID, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.activeIndexLocked()
	if idx < 0 || !m.workspaces[idx].PopupVisible || m.workspaces[idx].PopupBuffer == "" {
		return "", m.snapshotLocked()
	}
	ws := &m.workspaces[idx]
	promoted = ws.PopupBuffer
	ws.PopupVisible = false
	ws.PopupBuffer = ""
	ws.PopupRule = nil
	ws.LastPopup = ""
	ws.ActiveBuffer = promoted
	if !containsBuf(ws.Buffers, promoted) {
		ws.Buffers = append(ws.Buffers, promoted)
	}
	return promoted, m.snapshotLocked()
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
