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

// Perspective holds one workspace's isolated window state, open tabs, and
// workspace-local popup drawer state (matching Doom persp-mode).
type Perspective struct {
	ID           string
	Name         string
	ProjectRoot  string
	ActiveBuffer buffer.ID
	Buffers      []buffer.ID
	PopupBuffer  buffer.ID
	PopupRule    *PopupRule
	PopupVisible bool
	LastPopup    buffer.ID
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
		Buffers:      []buffer.ID{buffer.Dashboard, buffer.Conversation},
	}
	m.workspaces = append(m.workspaces, ws)
	m.activeID = ws.ID
	return clonePerspective(ws)
}

// Switch activates the target workspace and restores its isolated buffer set
// and popup drawer state.
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

// OpenBuffer routes buf through Doom popup rules:
//   - If buf matches a popup rule AND is not already promoted into this workspace's
//     main buffer tabs, it opens non-destructively in the Popup Drawer (returning openedAsPopup=true).
//   - Otherwise it focuses buf in the main window and records it strictly in this
//     workspace's isolated buffer list.
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
	ws.ActiveBuffer = buf
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
