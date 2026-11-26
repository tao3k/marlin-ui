// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tao3k/marlin-mygo/control/grpcwire"
	"github.com/tao3k/marlin-mygo/control/transport"
	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

type sessionRuntime struct {
	mu               sync.Mutex
	activeSessionID  string
	nextSeq          atomic.Uint64
	openedSessions   map[string]bool
	extraClosers     []func()
	authTokenSource  func(context.Context) (string, error)
	sendPromptRaw    func(displayPrompt, wirePrompt string, refs []composer.AttachmentRef)
	turnCtrl         *grpcwire.TurnStreamController
	composerBridge   *grpcwire.ComposerBridge
	rustSearch       *grpcwire.RustSearchBridge
	saveNow          func()
	scheduleSave     func()
	onSessionChanged func(sessionID string)
	onRemoteAdded    func(appconfig.SavedRemote)
}

func (sr *sessionRuntime) currentSession() string {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.activeSessionID
}

func (sr *sessionRuntime) setSession(id string) {
	sr.mu.Lock()
	sr.activeSessionID = id
	sr.mu.Unlock()
}

func (sr *sessionRuntime) addCloser(fn func()) {
	if fn == nil {
		return
	}
	sr.mu.Lock()
	sr.extraClosers = append(sr.extraClosers, fn)
	sr.mu.Unlock()
}

func (sr *sessionRuntime) closeExtras() {
	sr.mu.Lock()
	closers := sr.extraClosers
	sr.extraClosers = nil
	sr.mu.Unlock()
	for _, fn := range closers {
		fn()
	}
}

func wireSessionAndHelm(
	linkCtx context.Context,
	cfg appconfig.Config,
	adaptive *mygoshell.AdaptiveShell,
	targets *transport.TargetRegistry,
	federation *transport.FederationSupervisor,
	switcher *grpcwire.SwitchableTargetProvider,
	sr *sessionRuntime,
	markDirty func(),
) {
	app := adaptive.Desktop
	transcript := app.Transcript

	// Enable Emacs/Doom window & Helm chords (SPC w v/s/d/u, SPC s s, SPC b b).
	_ = app.EnableEmacsWindowKeymap()

	// Ensure the active session is present in the sidebar list.
	ensureSessionEntry(app, cfg.SessionID, "Primary Workspace")
	app.ActiveSessionID = cfg.SessionID

	// Wire authoritative Rust-backed Helm search via grpcwire.RustSearchBridge:
	// ZERO Go-side fabricated files/grep/backend search. All Workspace Search,
	// Ascent/Dusa DSL queries, and admitted Scheme/Rust commands stream strictly
	// from marlin-agent-core over gRPC, while pure UI buffers/sessions/targets
	// stay lightweight client-side navigation projections.
	projectID := "default"
	if p := strings.TrimSpace(cfg.ModelCfg.Session.Project); p != "" {
		projectID = p
	}
	if app.HelmSearch != nil && app.HelmSearch.Engine != nil {
		rustBridge := &grpcwire.RustSearchBridge{
			Provider:  switcher,
			Engine:    app.HelmSearch.Engine,
			ProjectID: projectID,
			SessionFn: sr.currentSession,
			OnUpdated: markDirty,
			Timeout:   1800 * time.Millisecond,
			PageLimit: 64,
		}
		sr.rustSearch = rustBridge
		sr.addCloser(rustBridge.Close)

		app.HelmSearch.OnQueryChanged = func(gen uint64, query string) {
			eng := app.HelmSearch.Engine
			cq := search.ParseConsultQuery(query)

			// 1. Authoritative Rust backend search (QueryProjectGraph + BrowseCommands).
			rustBridge.QueryAsync(linkCtx, gen, query)

			// 2. Pure UI-owned open Emacs buffers (never fabricated backend files).
			if !cq.IsDSL && len(app.WindowBuffers) > 0 {
				bufHits := make([]search.Candidate, 0, len(app.WindowBuffers))
				for _, b := range app.WindowBuffers {
					score, mask, ok := search.ScoreOrderlessAndMask(query, b.Title+" "+b.BufferID)
					if !ok {
						continue
					}
					bufHits = append(bufHits, search.Candidate{
						ID:            "buffer:" + b.BufferID,
						SourceID:      "buffers",
						Title:         b.Title,
						Subtitle:      "Open Emacs buffer",
						Location:      b.BufferID,
						Snippet:       "Focus buffer " + b.Title,
						Score:         score + 5,
						MatchMask:     mask,
						PayloadKind:   "buffer",
						PayloadTarget: "buffer:" + b.BufferID,
					})
				}
				if len(bufHits) > 0 {
					eng.IngestBatch(gen, search.SourceMeta{
						ID:       "buffers",
						Label:    "Buffers",
						Priority: 10,
					}, bufHits)
				}
			}

			// 3. UI-known sessions & federation targets (for quick switching).
			if !cq.IsDSL && len(app.Sessions) > 0 {
				sessHits := make([]search.Candidate, 0, len(app.Sessions))
				for _, s := range app.Sessions {
					score, mask, ok := search.ScoreOrderlessAndMask(query, s.Title+" "+s.SessionID)
					if !ok {
						continue
					}
					if s.SessionID == app.ActiveSessionID {
						score += 14
					}
					sessHits = append(sessHits, search.Candidate{
						ID:            "session:" + s.SessionID,
						SourceID:      "sessions",
						Title:         s.Title,
						Subtitle:      fmt.Sprintf("%s · %d turns", s.SessionID, s.TurnCount),
						Location:      s.SessionID,
						Snippet:       fmt.Sprintf("Switch to session %s (%d turns)", s.SessionID, s.TurnCount),
						Score:         score,
						MatchMask:     mask,
						PayloadKind:   "session",
						PayloadTarget: "session:" + s.SessionID,
					})
				}
				if len(sessHits) > 0 {
					eng.IngestBatch(gen, search.SourceMeta{
						ID:       "sessions",
						Label:    "Sessions",
						Priority: 12,
					}, sessHits)
				}
			}

			if !cq.IsDSL && targets != nil {
				metas := targets.List()
				targetHits := make([]search.Candidate, 0, len(metas))
				for _, m := range metas {
					score, mask, ok := search.ScoreOrderlessAndMask(query, m.DisplayName+" "+m.ID+" "+m.Address)
					if !ok {
						continue
					}
					targetHits = append(targetHits, search.Candidate{
						ID:            "target:" + m.ID,
						SourceID:      "targets",
						Title:         m.DisplayName,
						Subtitle:      fmt.Sprintf("%s · %s", m.Mode, m.Address),
						Location:      m.Address,
						Snippet:       fmt.Sprintf("Switch active Marlin Core target to %s (%s)", m.DisplayName, m.Address),
						Score:         score,
						MatchMask:     mask,
						PayloadKind:   "target",
						PayloadTarget: "target:" + m.ID,
					})
				}
				if len(targetHits) > 0 {
					eng.IngestBatch(gen, search.SourceMeta{
						ID:       "targets",
						Label:    "Targets",
						Priority: 15,
					}, targetHits)
				}
			}

			markDirty()
		}
		if !app.HelmSearch.Engine.PreviewOpen() {
			app.HelmSearch.Engine.TogglePreview() // Follow-Mode split preview ON by default.
		}
	}

	// Switch sessions cleanly while preserving per-session unsent drafts via fsync.
	switchSession := func(sessionID string) {
		sid := strings.TrimSpace(sessionID)
		if sid == "" {
			return
		}
		prev := sr.currentSession()
		if transcript != nil && transcript.Resilience != nil && prev != "" {
			transcript.Resilience.SetDraft("chat:"+prev, transcript.ComposerText())
		}
		sr.setSession(sid)
		app.ActiveSessionID = sid
		ensureSessionEntry(app, sid, sid)
		if transcript != nil && transcript.Resilience != nil {
			scope := "chat:" + sid
			transcript.ResilienceScope = scope
			restored := transcript.RestoreDraftFromResilience(scope)
			transcript.SetComposerText(restored)
			sr.scheduleSave()
		}
		if sr.onSessionChanged != nil {
			sr.onSessionChanged(sid)
		}
		if sr.composerBridge != nil {
			go func() {
				sctx, cancel := context.WithTimeout(linkCtx, 3*time.Second)
				defer cancel()
				if summary, err := sr.composerBridge.FetchSessionSummary(sctx, sid, sid); err == nil {
					ensureSessionEntry(app, summary.SessionID, summary.Title)
					for i := range app.Sessions {
						if app.Sessions[i].SessionID == summary.SessionID {
							app.Sessions[i].TurnCount = summary.TurnCount
							break
						}
					}
					markDirty()
				}
			}()
		}
		markDirty()
	}

	app.OnSelectSession = switchSession
	app.OnNewSession = func() {
		n := sr.nextSeq.Add(1)
		sid := fmt.Sprintf("marlin-%s-%d", time.Now().Format("150405"), n)
		title := fmt.Sprintf("Session %d", len(app.Sessions)+1)
		ensureSessionEntry(app, sid, title)
		switchSession(sid)
		if sr.turnCtrl != nil {
			go func() {
				sctx, cancel := context.WithTimeout(linkCtx, 4*time.Second)
				defer cancel()
				_, _ = sr.turnCtrl.OpenSession(sctx, sid, true)
				markDirty()
			}()
		}
	}

	// Dynamic "+ Remote" TLS 1.3 / mTLS gRPC target addition from Federation Cockpit.
	app.OnAddRemoteTarget = func(targetID, displayName, endpoint, bearerToken string) error {
		addr := strings.TrimSpace(endpoint)
		if addr == "" {
			return fmt.Errorf("remote gRPC endpoint is required")
		}
		id := strings.TrimSpace(targetID)
		if id == "" {
			id = fmt.Sprintf("remote-%d", time.Now().Unix()%10000)
		}
		label := strings.TrimSpace(displayName)
		if label == "" {
			label = id
		}
		var tlsCfg *grpcwire.TLSConfig
		if cfg.TLSCAFile != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
			tlsCfg = &grpcwire.TLSConfig{
				CAFile:   cfg.TLSCAFile,
				CertFile: cfg.TLSCertFile,
				KeyFile:  cfg.TLSKeyFile,
			}
		}
		auth := appconfig.Config{AuthToken: bearerToken}.BearerHeader()
		dialCfg := grpcwire.DialConfig{
			ID:          id,
			DisplayName: label,
			Mode:        transport.ModeRemoteGRPC,
			Address:     addr,
			AuthToken:   auth,
			TLS:         tlsCfg,
		}
		if dialCfg.AuthToken == "" && sr.authTokenSource != nil {
			dialCfg.AuthTokenSource = sr.authTokenSource
		}
		dialed, err := grpcwire.Dial(dialCfg)
		if err != nil {
			return fmt.Errorf("dial remote %s: %w", addr, err)
		}
		sr.addCloser(func() { _ = dialed.Close() })
		_ = targets.Register(dialed)
		_, _ = targets.Switch(id)
		_, _ = switcher.ResolveFromRegistry(targets, id)
		go func() {
			federation.ProbeAll(linkCtx, 2*time.Second)
			markDirty()
		}()
		if sr.onRemoteAdded != nil {
			sr.onRemoteAdded(appconfig.SavedRemote{
				ID:          id,
				DisplayName: label,
				Endpoint:    addr,
			})
		}
		markDirty()
		return nil
	}

	// Wire multimodal composer submission so file/image/voice attachments travel
	// through the unified turn pipeline instead of dropping attachment refs.
	if transcript != nil {
		transcript.OnSubmitTurn = func(sub composer.TurnSubmission) {
			refs := make([]composer.AttachmentRef, 0, len(sub.Attachments))
			for _, att := range sub.Attachments {
				refs = append(refs, att.Ref)
			}
			wirePrompt := grpcwire.FormatTurnSubmissionPrompt(sub, refs)
			if strings.TrimSpace(wirePrompt) == "" {
				return
			}
			display := strings.TrimSpace(sub.Prompt)
			if display == "" {
				display = wirePrompt
			}
			if sr.sendPromptRaw != nil {
				sr.sendPromptRaw(display, wirePrompt, refs)
			} else if app.OnSendPrompt != nil {
				app.OnSendPrompt(wirePrompt)
			}
		}
	}
}

func ensureSessionEntry(app *mygoshell.WorkbenchApp, sessionID, title string) {
	for i := range app.Sessions {
		if app.Sessions[i].SessionID == sessionID {
			if title != "" && app.Sessions[i].Title == "" {
				app.Sessions[i].Title = title
			}
			return
		}
	}
	if title == "" {
		title = sessionID
	}
	app.Sessions = append(app.Sessions, mygoshell.SessionSummaryView{
		SessionID: sessionID,
		Title:     title,
	})
}

func registerSavedRemotes(
	linkCtx context.Context,
	cfg appconfig.Config,
	remotes []appconfig.SavedRemote,
	targets *transport.TargetRegistry,
	federation *transport.FederationSupervisor,
	sr *sessionRuntime,
	markDirty func(),
) {
	if targets == nil || len(remotes) == 0 {
		return
	}
	existing := make(map[string]bool)
	for _, m := range targets.List() {
		existing[m.ID] = true
	}
	var tlsCfg *grpcwire.TLSConfig
	if cfg.TLSCAFile != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		tlsCfg = &grpcwire.TLSConfig{
			CAFile:   cfg.TLSCAFile,
			CertFile: cfg.TLSCertFile,
			KeyFile:  cfg.TLSKeyFile,
		}
	}
	added := false
	for _, rem := range remotes {
		id := strings.TrimSpace(rem.ID)
		addr := strings.TrimSpace(rem.Endpoint)
		if id == "" || addr == "" || existing[id] {
			continue
		}
		label := strings.TrimSpace(rem.DisplayName)
		if label == "" {
			label = id
		}
		dialCfg := grpcwire.DialConfig{
			ID:          id,
			DisplayName: label,
			Mode:        transport.ModeRemoteGRPC,
			Address:     addr,
			TLS:         tlsCfg,
		}
		if sr.authTokenSource != nil {
			dialCfg.AuthTokenSource = sr.authTokenSource
		}
		dialed, err := grpcwire.Dial(dialCfg)
		if err != nil {
			continue
		}
		sr.addCloser(func() { _ = dialed.Close() })
		_ = targets.Register(dialed)
		existing[id] = true
		added = true
	}
	if added && federation != nil {
		go func() {
			federation.ProbeAll(linkCtx, 2*time.Second)
			markDirty()
		}()
	}
}
