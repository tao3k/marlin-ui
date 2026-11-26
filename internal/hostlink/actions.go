// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tao3k/marlin-mygo/control/transport"
	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/notify"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
)

// wireInteractiveActions binds user-facing interactive callbacks across
// Helm action dispatch, Transcript slash commands / @ mentions / search /
// retry / fork / clipboard copy, Projection Inspector -> Composer bridging,
// Federation ProbeAll, and Mobile QuickCapture / Gate forwarding.
func wireInteractiveActions(
	linkCtx context.Context,
	adaptive *mygoshell.AdaptiveShell,
	targets *transport.TargetRegistry,
	federation *transport.FederationSupervisor,
	copyText func(string),
	markDirty func(),
) {
	if adaptive == nil || adaptive.Desktop == nil {
		return
	}
	app := adaptive.Desktop
	tr := app.Transcript

	toast := func(title, msg string) {
		if app.Notifications != nil && app.Notifications.Center != nil {
			_, _ = app.Notifications.Center.Publish(notify.NotificationSpec{
				Severity: notify.SeverityInfo,
				Source:   notify.SourceHost,
				Title:    title,
				Message:  msg,
			})
		}
		markDirty()
	}

	doCopy := func(text string) {
		if text == "" {
			return
		}
		if copyText != nil {
			copyText(text)
		}
		toast("Copied to Clipboard", fmt.Sprintf("%d characters copied", len(text)))
	}

	// 1. Helm action dispatch: route session:* and target:* candidates natively,
	// delegating all other candidates to BindDefaultHelmSources' built-in handler.
	if app.HelmSearch != nil {
		prevExec := app.HelmSearch.OnExecuteAction
		app.HelmSearch.OnExecuteAction = func(action search.TargetAction, cands []search.Candidate) {
			var remaining []search.Candidate
			for _, cand := range cands {
				switch {
				case strings.HasPrefix(cand.PayloadTarget, "session:"):
					sid := strings.TrimPrefix(cand.PayloadTarget, "session:")
					if app.OnSelectSession != nil && sid != "" {
						app.OnSelectSession(sid)
					}
				case strings.HasPrefix(cand.PayloadTarget, "target:"):
					tid := strings.TrimPrefix(cand.PayloadTarget, "target:")
					if app.FederationUI != nil && app.FederationUI.OnSelectTarget != nil && tid != "" {
						app.FederationUI.OnSelectTarget(tid)
					}
				default:
					remaining = append(remaining, cand)
				}
			}
			if len(remaining) > 0 && prevExec != nil {
				prevExec(action, remaining)
			}
			markDirty()
		}
	}

	// 2. Transcript interactive callbacks: copy, slash commands, @ mentions, search, retry, fork.
	if tr != nil {
		tr.OnCopyCode = doCopy

		tr.OnSlashCommand = func(cmd composer.SlashCommand) {
			name := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cmd.Name), "/"))
			switch name {
			case "new", "new-session":
				if app.OnNewSession != nil {
					app.OnNewSession()
				}
			case "settings", "prefs":
				app.OpenSettings()
			case "auth", "login", "account":
				app.OpenSettingsTab(mygoshell.SettingsTabAuth)
			case "search", "helm":
				app.OpenHelmSearch()
			case "federation", "targets":
				app.OpenFederation()
			case "governance", "gov", "features":
				app.OpenGovernance()
			case "keys", "keymap":
				app.OpenKeymapInspector()
			case "theme":
				adaptive.CycleTheme()
			case "sidebar":
				app.ToggleSidebar()
			case "clear":
				tr.SetComposerText("")
			default:
				if app.OnSendPrompt != nil {
					app.OnSendPrompt("/" + name)
				}
			}
			markDirty()
		}

		tr.OnQueryMentions = func(query string) []composer.ContextMention {
			q := strings.ToLower(strings.TrimSpace(query))
			var out []composer.ContextMention
			add := func(m composer.ContextMention) {
				if q != "" && !strings.Contains(strings.ToLower(m.Kind+" "+m.Target+" "+m.Label), q) {
					return
				}
				out = append(out, m)
			}
			if app.Projection != nil {
				snap := app.Projection.Snapshot()
				for _, df := range snap.DiffFiles {
					add(composer.ContextMention{
						Kind:   "file",
						Target: df.Path,
						Label:  fmt.Sprintf("%s (+%d/-%d)", df.Path, df.Additions, df.Deletions),
					})
				}
			}
			for _, s := range app.Sessions {
				add(composer.ContextMention{
					Kind:   "session",
					Target: s.SessionID,
					Label:  s.Title,
				})
			}
			if targets != nil {
				for _, tm := range targets.List() {
					add(composer.ContextMention{
						Kind:   "target",
						Target: tm.ID,
						Label:  fmt.Sprintf("%s (%s)", tm.DisplayName, tm.Address),
					})
				}
			}
			for _, buf := range app.WindowBuffers {
				add(composer.ContextMention{
					Kind:   "buffer",
					Target: buf.BufferID,
					Label:  buf.Title,
				})
			}
			return out
		}

		tr.OnSearchSession = func(query string) []mygoshell.SessionSearchHit {
			q := strings.ToLower(strings.TrimSpace(query))
			if q == "" {
				return nil
			}
			n := tr.Len()
			var hits []mygoshell.SessionSearchHit
			for i := 0; i < n; i++ {
				e, ok := tr.Entry(i)
				if !ok {
					continue
				}
				low := strings.ToLower(e.Text)
				idx := strings.Index(low, q)
				if idx < 0 {
					continue
				}
				snippet := strings.TrimSpace(e.Text)
				if len(snippet) > 120 {
					start := idx - 24
					if start < 0 {
						start = 0
					}
					end := start + 120
					if end > len(snippet) {
						end = len(snippet)
					}
					snippet = snippet[start:end]
				}
				score := float32(1.0)
				if idx == 0 {
					score = 1.5
				}
				hits = append(hits, mygoshell.SessionSearchHit{
					Row:     i,
					Snippet: snippet,
					Score:   score,
				})
			}
			return hits
		}

		findUserPromptForTurn := func(turnOrdinal uint64) string {
			n := tr.Len()
			var lastUser string
			for i := 0; i < n; i++ {
				e, ok := tr.Entry(i)
				if !ok {
					continue
				}
				if e.Role == mygoshell.RoleUser && strings.TrimSpace(e.Text) != "" {
					lastUser = e.Text
				}
				if e.Role == mygoshell.RoleAssistant && e.TurnOrdinal == turnOrdinal && lastUser != "" {
					return lastUser
				}
			}
			return lastUser
		}

		tr.OnRetryTurn = func(turnOrdinal uint64) {
			if p := findUserPromptForTurn(turnOrdinal); p != "" && app.OnSendPrompt != nil {
				app.OnSendPrompt(p)
			}
		}
		tr.OnForkTurn = func(turnOrdinal uint64) {
			p := findUserPromptForTurn(turnOrdinal)
			if app.OnNewSession != nil {
				app.OnNewSession()
			}
			if p != "" && app.OnSendPrompt != nil {
				app.OnSendPrompt(p)
			}
		}
		tr.OnModelChanged = func(m composer.ModelOption) {
			toast("Model Selected", m.Label)
		}
		tr.OnDeepThink = func(enabled bool) {
			state := "off"
			if enabled {
				state = "on"
			}
			toast("Deep Thinking", "Reasoning mode "+state)
		}
	}

	// 3. Projection Inspector -> Composer bridge.
	if app.Projection != nil {
		app.Projection.OnCopyText = doCopy
		app.Projection.OnMentionFile = func(path string) {
			if tr == nil || strings.TrimSpace(path) == "" {
				return
			}
			cur := tr.ComposerText()
			mention := "@file:" + strings.TrimSpace(path) + " "
			if cur != "" && !strings.HasSuffix(cur, " ") {
				cur += " "
			}
			tr.SetComposerText(cur + mention)
			toast("Mentioned File", path)
		}
		app.Projection.OnAttachDiff = func(file mygoshell.WorkspaceDiffFileView) {
			if tr == nil || strings.TrimSpace(file.Path) == "" {
				return
			}
			body := fmt.Sprintf("Diff %s (%s +%d/-%d)\n%s\n", file.Path, file.Status, file.Additions, file.Deletions, strings.Join(file.HunkLines, "\n"))
			_, _ = tr.AttachFile(composer.AttachmentInput{
				Filename: file.Path + ".diff",
				MIMEType: "text/x-diff",
				Data:     []byte(body),
			})
			toast("Attached Workspace Diff", file.Path)
		}
		app.Projection.OnAttachProcessLogs = func(proc mygoshell.ManagedProcessView) {
			if tr == nil || strings.TrimSpace(proc.ProcessID) == "" {
				return
			}
			body := fmt.Sprintf("Process %s (%s, state=%s, exit=%d)\n%s\n", proc.ProcessID, proc.Command, proc.State, proc.ExitCode, strings.Join(proc.TailLines, "\n"))
			_, _ = tr.AttachFile(composer.AttachmentInput{
				Filename: proc.ProcessID + ".log",
				MIMEType: "text/plain",
				Data:     []byte(body),
			})
			toast("Attached Process Logs", proc.ProcessID)
		}
	}

	// 4. Federation Cockpit ProbeAll.
	if app.FederationUI != nil && federation != nil {
		app.FederationUI.OnProbeAll = func() {
			go func() {
				federation.ProbeAll(linkCtx, 2*time.Second)
				markDirty()
			}()
		}
	}

	// 5. Unified Gate decision forwarding & Mobile QuickCapture / Cancel parity.
	decideGate := func(gateID string, approve bool) {
		if app.PromptGate == nil || app.PromptGate.Queue == nil {
			return
		}
		act := promptgate.ActionReject
		if approve {
			act = promptgate.ActionApproveOnce
		}
		if req, resp, err := app.PromptGate.Queue.Resolve(gateID, promptgate.PromptResponse{
			PromptID:       gateID,
			Action:         act,
			IdempotencyKey: fmt.Sprintf("ui-gate-%s-%v", gateID, approve),
		}); err == nil && app.PromptGate.OnResolve != nil {
			app.PromptGate.OnResolve(req, resp)
		}
		markDirty()
	}
	app.OnGateDecision = decideGate

	if adaptive.Mobile != nil {
		adaptive.Mobile.OnCancelTurn = func() {
			if app.OnCancelTurn != nil {
				app.OnCancelTurn()
			}
		}
		adaptive.Mobile.OnGateDecision = decideGate
		adaptive.Mobile.OnQuickCapture = func(channel, text string) {
			if strings.TrimSpace(text) == "" || app.OnSendPrompt == nil {
				return
			}
			app.OnSendPrompt(fmt.Sprintf("[capture:%s] %s", strings.TrimSpace(channel), strings.TrimSpace(text)))
		}
	}
}
