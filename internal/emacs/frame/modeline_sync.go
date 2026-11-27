// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"strings"
	"time"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modeline"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

// ComposerOpen reports whether the formal *conversation* ChatBox is open
// (toggled by clicking the ModeLine Marlin dock).
func (f *Frame) ComposerOpen() bool {
	f.composerMu.Lock()
	defer f.composerMu.Unlock()
	return f.composerOpen
}

// ComposerDockHovered reports whether the pointer is currently hovering over
// the center ModeLine Marlin dock.
func (f *Frame) ComposerDockHovered() bool {
	f.composerMu.Lock()
	defer f.composerMu.Unlock()
	return f.composerDockHovered
}

// ToggleComposerOpen toggles the formal *conversation* ChatBox open/closed and
// fires the click forward-dash + splash animation on the ModeLine Marlin.
func (f *Frame) ToggleComposerOpen() {
	f.composerMu.Lock()
	f.composerOpen = !f.composerOpen
	f.composerClickStart = time.Now()
	if f.composerOpen && f.composerHoverStart.IsZero() {
		f.composerHoverStart = time.Now()
	}
	f.composerMu.Unlock()
	f.requestFrame()
}

func (f *Frame) setComposerDockHovered(hovered bool) {
	f.composerMu.Lock()
	prev := f.composerDockHovered
	f.composerDockHovered = hovered
	if hovered && !prev {
		// Record entry timestamp so the ~1.0s surfacing + fin-flutter + splash
		// animation runs once per hover entry and then settles static!
		f.composerHoverStart = time.Now()
	} else if !hovered && prev && !f.composerOpen {
		f.composerHoverStart = time.Time{}
	}
	f.composerMu.Unlock()
	if prev != hovered {
		f.requestFrame()
	}
}

func (f *Frame) composerDockAnimTimes() (open bool, hoverStart, clickStart time.Time) {
	f.composerMu.Lock()
	defer f.composerMu.Unlock()
	return f.composerOpen, f.composerHoverStart, f.composerClickStart
}

func (f *Frame) buildModelineStatus(snap frameSnapshot, modelLbl, sessLbl string) modeline.Status {
	return f.buildModelineStatusForBuffer(snap, snap.ActiveBuffer, modelLbl, sessLbl)
}

func (f *Frame) buildModelineStatusForBuffer(snap frameSnapshot, activeBuf buffer.ID, modelLbl, sessLbl string) modeline.Status {
	th := snap.Theme
	actLabel := "Ready"
	actCol := th.Success
	remoteHost := ""
	if f.Link != nil {
		switch f.Link.Mode {
		case hostlink.ModeSidecar:
			actLabel = "Online"
			actCol = th.Success
		case hostlink.ModeRemoteGRPC:
			actLabel = "Remote"
			actCol = th.Info
			remoteHost = f.Link.Endpoint
		default:
			actLabel = "Local"
			actCol = th.Warning
		}
	}
	sessID := f.AppCfg.SessionID
	if sessLbl != "" {
		sessID = sessLbl
	}
	turns := 0
	helmTotal := 0
	agentWorking := false
	deepThink := false
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		if f.Adaptive.Desktop.ActiveSessionID != "" {
			sessID = f.Adaptive.Desktop.ActiveSessionID
		}
		if f.Adaptive.Desktop.Transcript != nil {
			turns = f.Adaptive.Desktop.Transcript.Len()
			deepThink = f.Adaptive.Desktop.Transcript.DeepThinkEnabled()
			if f.Adaptive.Desktop.Transcript.InFlight() {
				agentWorking = true
				actLabel = "Working"
				actCol = th.Info
			}
		}
		if f.Adaptive.Desktop.HelmSearch != nil && f.Adaptive.Desktop.HelmSearch.Engine != nil {
			helmTotal, _, _, _ = f.Adaptive.Desktop.HelmSearch.Engine.Stats()
			helmTotal = f.miniState.DisplayHelmTotal(helmTotal)
		}
	}

	eff := f.EffectiveModeStack(activeBuf)

	preset := modeline.PresetMain
	if f.miniState.IsCompletionActive() {
		preset = modeline.PresetHelm
	} else if eff.HasSegment["dashboard-preset"] || (eff.HostMode == "marlin-chat-mode" && turns == 0) {
		preset = modeline.PresetDashboard
	}

	ver := ""
	if preset == modeline.PresetDashboard {
		ver = "MARLIN"
	}
	wsPath := "~/marlin/" + sessID + "/"
	if pr := strings.TrimSpace(snap.Workspaces.Active.ProjectRoot); pr != "" {
		wsPath = pr
	}
	bufName := string(activeBuf)
	majorTitle := eff.DisplayTitle
	if strings.HasPrefix(bufName, "scheme-ext:") {
		extID := strings.TrimPrefix(bufName, "scheme-ext:")
		for _, ext := range snap.SchemeExts {
			if ext.ID == extID && strings.TrimSpace(ext.Label) != "" {
				bufName = strings.TrimSpace(ext.Label)
				break
			}
		}
		majorTitle = "Scheme-Ext"
	}
	return modeline.Status{
		Preset:            preset,
		KeyMode:           f.meowState.Mode(),
		MiniMode:          f.miniState.Mode(),
		Keypad:            f.meowState.Keypad(),
		BufferName:        bufName,
		MajorMode:         majorTitle,
		WorkspacePath:     wsPath,
		RemoteHost:        remoteHost,
		ActivityLabel:     actLabel,
		ActivityColor:     actCol,
		AgentWorking:      agentWorking,
		HasUnsavedDraft:   eff.HasSegment["composer-dock"] && strings.TrimSpace(f.convComposerDraft) != "",
		DeepThink:         deepThink,
		ShowDeepThink:     eff.HasSegment["deep-think"],
		ModelLabel:        modelLbl,
		StatusItems:       snap.StatusItems,
		SessionID:         sessID,
		RowCount:          turns,
		HelmTotal:         helmTotal,
		ShowGateBadge:     eff.HasSegment["gate-badge"],
		OnOpenModels:      func() { f.SelectBuffer(buffer.Models) },
		OnOpenWorkspaces:  func() { f.OpenHydra("SPC TAB") },
		OnToggleDeepThink: f.ToggleDeepThink,
		VersionLabel:      ver,
		Theme:             th,
	}
}

func majorModeName(id buffer.ID) string {
	reg := modes.DefaultRegistry()
	st := reg.MakeBufferState(string(id))
	if strings.HasPrefix(string(id), "scheme-ext:") {
		return "Scheme-Ext"
	}
	return reg.EffectiveStack(st).DisplayTitle
}

func (f *Frame) newSession() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnNewSession != nil {
		f.Adaptive.Desktop.OnNewSession()
		f.SelectBuffer(buffer.Conversation)
		f.Echo("Session: " + f.Adaptive.Desktop.ActiveSessionID)
	}
}

func (f *Frame) probeTargets() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.FederationUI != nil && f.Adaptive.Desktop.FederationUI.OnProbeAll != nil {
		f.Adaptive.Desktop.FederationUI.OnProbeAll()
		f.Echo("Probing federation targets...")
	}
}

func (f *Frame) submitPrompt(prompt string) {
	f.SelectBuffer(buffer.Conversation)
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSendPrompt != nil {
		f.Adaptive.Desktop.OnSendPrompt(prompt)
	}
}

func (f *Frame) saveCustomizePrefs(sessionID, remoteHost string, realSession bool) {
	f.mu.Lock()
	if strings.TrimSpace(sessionID) != "" {
		f.Prefs.LastSessionID = strings.TrimSpace(sessionID)
	}
	f.Prefs.DefaultRemoteHost = strings.TrimSpace(remoteHost)
	rs := realSession
	f.Prefs.RealSession = &rs
	prefs := f.Prefs
	f.mu.Unlock()
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSelectSession != nil && prefs.LastSessionID != "" {
		f.Adaptive.Desktop.OnSelectSession(prefs.LastSessionID)
	}
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
}
