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
	commonv1 "github.com/tao3k/marlin-mygo/gen/marlin/common/v1"
	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
	uiv1 "github.com/tao3k/marlin-mygo/gen/marlin/ui/v1"
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

	// Wire live multi-source Helm search: instant in-memory projections +
	// async, generation-fenced Rust backend RPCs (QueryProjectGraph + BrowseCommands).
	var (
		rustSearchMu     sync.Mutex
		rustSearchCancel context.CancelFunc
	)
	sr.addCloser(func() {
		rustSearchMu.Lock()
		if rustSearchCancel != nil {
			rustSearchCancel()
			rustSearchCancel = nil
		}
		rustSearchMu.Unlock()
	})
	mygoshell.BindDefaultHelmSources(app, func(gen uint64, query string) {
		if app.HelmSearch == nil || app.HelmSearch.Engine == nil {
			return
		}
		backendQ, localFilter := splitConsultStages(query)
		q := strings.ToLower(strings.TrimSpace(localFilter))

		// Stream durable sessions into Helm.
		sessHits := make([]search.Candidate, 0, len(app.Sessions))
		for _, s := range app.Sessions {
			hay := s.Title + " " + s.SessionID
			if !matchesOrderless(hay, q) {
				continue
			}
			score := int32(82)
			if s.SessionID == app.ActiveSessionID {
				score = 96
			}
			_, mask, _ := search.ScoreAndMask(q, s.Title)
			sessHits = append(sessHits, search.Candidate{
				ID:            "session:" + s.SessionID,
				SourceID:      "sessions",
				Title:         s.Title,
				Subtitle:      fmt.Sprintf("%s · %d turns", s.SessionID, s.TurnCount),
				Location:      s.SessionID,
				Snippet:       fmt.Sprintf("Switch to session %s (%d turns)", s.SessionID, s.TurnCount),
				Score:         score,
				MatchMask:     mask,
				PayloadKind:   "command",
				PayloadTarget: "session:" + s.SessionID,
			})
		}
		if len(sessHits) > 0 {
			app.HelmSearch.Engine.IngestBatch(gen, search.SourceMeta{
				ID:       "sessions",
				Label:    "Sessions",
				Priority: 12,
			}, sessHits)
		}

		// Stream Federation targets into Helm.
		if targets != nil {
			metas := targets.List()
			targetHits := make([]search.Candidate, 0, len(metas))
			for _, m := range metas {
				hay := m.DisplayName + " " + m.ID + " " + m.Address
				if !matchesOrderless(hay, q) {
					continue
				}
				_, mask, _ := search.ScoreAndMask(q, m.DisplayName)
				targetHits = append(targetHits, search.Candidate{
					ID:            "target:" + m.ID,
					SourceID:      "targets",
					Title:         m.DisplayName,
					Subtitle:      fmt.Sprintf("%s · %s", m.Mode, m.Address),
					Location:      m.Address,
					Snippet:       fmt.Sprintf("Switch active Marlin Core target to %s (%s)", m.DisplayName, m.Address),
					Score:         80,
					MatchMask:     mask,
					PayloadKind:   "command",
					PayloadTarget: "target:" + m.ID,
				})
			}
			if len(targetHits) > 0 {
				app.HelmSearch.Engine.IngestBatch(gen, search.SourceMeta{
					ID:       "targets",
					Label:    "Targets",
					Priority: 15,
				}, targetHits)
			}
		}

		// Stream live Rust backend search hits asynchronously (preempted on every keystroke).
		if switcher != nil {
			if tgt := switcher.CurrentTarget(); tgt != nil {
				rctx, rcancel := context.WithTimeout(linkCtx, 1500*time.Millisecond)
				rustSearchMu.Lock()
				if rustSearchCancel != nil {
					rustSearchCancel()
				}
				rustSearchCancel = rcancel
				rustSearchMu.Unlock()

				sessID := sr.currentSession()
				eng := app.HelmSearch.Engine
				go dispatchRustBackendSearch(rctx, rcancel, tgt, eng, gen, sessID, backendQ, q, markDirty)
			}
		}
		markDirty()
	})
	if app.HelmSearch != nil && app.HelmSearch.Engine != nil && !app.HelmSearch.Engine.PreviewOpen() {
		app.HelmSearch.Engine.TogglePreview() // Follow-Mode split preview ON by default for Desktop UX.
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

func splitConsultStages(raw string) (backendQ, localFilter string) {
	trimmed := strings.TrimSpace(raw)
	if left, right, ok := strings.Cut(trimmed, "#"); ok {
		return strings.TrimSpace(left), strings.TrimSpace(right)
	}
	return trimmed, trimmed
}

func matchesOrderless(haystack, query string) bool {
	if query == "" {
		return true
	}
	lh := strings.ToLower(haystack)
	for _, tok := range strings.Fields(query) {
		if strings.HasPrefix(tok, "!") {
			neg := strings.TrimPrefix(tok, "!")
			if neg != "" && strings.Contains(lh, neg) {
				return false
			}
			continue
		}
		if !strings.Contains(lh, tok) {
			if _, _, ok := search.ScoreAndMask(tok, haystack); !ok {
				return false
			}
		}
	}
	return true
}

func dispatchRustBackendSearch(
	ctx context.Context,
	cancel context.CancelFunc,
	tgt *grpcwire.Target,
	eng *search.HelmEngine,
	gen uint64,
	sessID string,
	backendQ string,
	localFilter string,
	markDirty func(),
) {
	defer cancel()
	var wg sync.WaitGroup
	if proj := tgt.Projection(); proj != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := proj.QueryProjectGraph(ctx, &projectionv1.QueryProjectGraphRequest{
				ProjectId: "default",
				SessionId: &sessID,
				Surfaces: []projectionv1.GraphVisibleSurface{
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_WORKSPACE,
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_MEMORY,
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_TOOLS,
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_SESSIONS,
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_CONTENT,
					projectionv1.GraphVisibleSurface_GRAPH_VISIBLE_SURFACE_TOPOLOGY,
				},
				SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_DENIED,
				Query:            backendQ,
				Page:             &commonv1.CursorPageRequest{MaxItems: 64},
			})
			if err != nil || ctx.Err() != nil || resp == nil {
				return
			}
			cands := make([]search.Candidate, 0, len(resp.GetRecords()))
			for _, rec := range resp.GetRecords() {
				title := strings.TrimSpace(rec.GetTitle())
				if title == "" {
					title = rec.GetRecordId()
				}
				loc := rec.GetSourcePath()
				if loc != "" && rec.GetStartLine() > 0 {
					loc = fmt.Sprintf("%s:L%d", loc, rec.GetStartLine())
				}
				if loc == "" {
					loc = rec.GetKind()
				}
				if !matchesOrderless(title+" "+rec.GetSummary()+" "+loc, localFilter) {
					continue
				}
				score, mask, _ := search.ScoreAndMask(localFilter, title)
				cands = append(cands, search.Candidate{
					ID:            "rust:" + rec.GetRecordId(),
					SourceID:      "rust-graph",
					Title:         title,
					Subtitle:      loc,
					Location:      loc,
					Snippet:       rec.GetSummary(),
					Score:         score + 40,
					MatchMask:     mask,
					PayloadKind:   "file",
					PayloadTarget: loc,
				})
			}
			if len(cands) > 0 {
				eng.IngestBatch(gen, search.SourceMeta{
					ID:       "rust-graph",
					Label:    "Rust Search",
					Priority: 2,
				}, cands)
				markDirty()
			}
		}()
	}
	if uiClient := tgt.UI(); uiClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := uiClient.BrowseCommands(ctx, &uiv1.BrowseCommandsRequest{
				SessionId: sessID,
				Query:     backendQ,
			})
			if err != nil || ctx.Err() != nil || resp == nil || resp.GetCandidates() == nil {
				return
			}
			names := resp.GetCandidates().GetNames()
			cands := make([]search.Candidate, 0, len(names))
			for _, name := range names {
				if !matchesOrderless(name, localFilter) {
					continue
				}
				score, mask, _ := search.ScoreAndMask(localFilter, name)
				cands = append(cands, search.Candidate{
					ID:            "rust-cmd:" + name,
					SourceID:      "rust-commands",
					Title:         name,
					Subtitle:      "Scheme / Rust command",
					Location:      "M-x",
					Snippet:       "Execute admitted Scheme command: " + name,
					Score:         score + 15,
					MatchMask:     mask,
					PayloadKind:   "command",
					PayloadTarget: name,
				})
			}
			if len(cands) > 0 {
				eng.IngestBatch(gen, search.SourceMeta{
					ID:       "rust-commands",
					Label:    "Scheme Commands",
					Priority: 6,
				}, cands)
				markDirty()
			}
		}()
	}
	wg.Wait()
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
