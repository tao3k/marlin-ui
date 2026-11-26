// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

// Package hostlink connects Marlin UI (Desktop & Mobile) to a Rust
// marlin-agent-core gRPC host (auto-discovered supervised local sidecar or
// remote TLS 1.3/mTLS endpoint) and wires durable session streaming,
// Federation hot-swapping, Tier-0 PromptGate approvals, and fsync continuity.
package hostlink

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tao3k/marlin-mygo/control/grpcwire"
	"github.com/tao3k/marlin-mygo/control/transport"
	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/notify"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/resilience"
	"github.com/tao3k/marlin-mygo/engine/shell"
	appv1 "github.com/tao3k/marlin-mygo/gen/marlin/app/v1"
	gatev1 "github.com/tao3k/marlin-mygo/gen/marlin/gate/v1"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/device"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

// Mode describes how Marlin UI connected to the Rust backend host.
type Mode string

const (
	ModeRemoteGRPC Mode = "remote-grpc"
	ModeSidecar    Mode = "local-sidecar"
	ModeStandalone Mode = "standalone"
)

// Link owns the live transport, federation supervisor, stream pumps, and
// fsync power-loss continuity coordinator for one Marlin UI process.
type Link struct {
	Mode       Mode
	TargetID   string
	Endpoint   string
	Targets    *transport.TargetRegistry
	Federation *transport.FederationSupervisor
	Switcher   *grpcwire.SwitchableTargetProvider
	Driver     *grpcwire.SessionDriver
	TurnCtrl   *grpcwire.TurnStreamController
	Coord      *grpcwire.ContinuityCoordinator

	SaveNow        func()
	OnDraftChanged func(scopeID, text string)

	closeOnce sync.Once
	closeFn   func()
}

// Close flushes the final fsync recovery snapshot and shuts down all gRPC
// streams and supervised child processes cleanly.
func (l *Link) Close() {
	if l == nil {
		return
	}
	l.closeOnce.Do(func() {
		if l.closeFn != nil {
			l.closeFn()
		}
	})
}

// ConnectOption configures optional runtime hooks for Connect.
type ConnectOption func(*connectOptions)

type connectOptions struct {
	enableDesktopMyGoAdapters bool
	authTokenSource           func(context.Context) (string, error)
	savedRemotes              []appconfig.SavedRemote
	onRemoteAdded             func(appconfig.SavedRemote)
	onSessionChanged          func(sessionID string)
}

// WithDesktopMyGoAdapters registers MyGo's native OS clipboard and desktop
// notification adapters on the device bridge. Callers MUST only pass this
// option when the MyGo main event loop is active (e.g. inside
// mygo.App.WhenReady), because MyGo desktop capability probes dispatch onto
// the GUI main thread.
func WithDesktopMyGoAdapters() ConnectOption {
	return func(o *connectOptions) {
		o.enableDesktopMyGoAdapters = true
	}
}

// WithAuthTokenSource provides a dynamic per-RPC Authorization header resolver
// (typically backed by OS Keychain OAuth credentials via OptionalAuthTokenSource)
// for remote gRPC targets when no explicit static token override is given.
func WithAuthTokenSource(fn func(context.Context) (string, error)) ConnectOption {
	return func(o *connectOptions) {
		o.authTokenSource = fn
	}
}

// WithSavedRemotes pre-registers persisted non-secret remote endpoints into the
// Federation TargetRegistry on startup.
func WithSavedRemotes(remotes []appconfig.SavedRemote) ConnectOption {
	return func(o *connectOptions) {
		if len(remotes) > 0 {
			o.savedRemotes = append([]appconfig.SavedRemote(nil), remotes...)
		}
	}
}

// WithOnRemoteAdded notifies the caller when the user adds a new remote gRPC
// endpoint via the Federation Cockpit so non-secret endpoints can be persisted.
func WithOnRemoteAdded(fn func(appconfig.SavedRemote)) ConnectOption {
	return func(o *connectOptions) {
		o.onRemoteAdded = fn
	}
}

// WithOnSessionChanged notifies the caller whenever the active session changes.
func WithOnSessionChanged(fn func(sessionID string)) ConnectOption {
	return func(o *connectOptions) {
		o.onSessionChanged = fn
	}
}

// Connect establishes the live gRPC host connection (remote TLS 1.3 or
// auto-discovered local supervised marlin-app-server),
// restores crash-persisted drafts via fsync, and binds all reactive UI hooks
// onto adaptive.
func Connect(
	ctx context.Context,
	cfg appconfig.Config,
	adaptive *mygoshell.AdaptiveShell,
	targetKind schema.TargetKind,
	invalidate func(),
	opts ...ConnectOption,
) (*Link, error) {
	var copts connectOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&copts)
		}
	}
	if adaptive == nil || adaptive.Desktop == nil {
		return nil, fmt.Errorf("hostlink: AdaptiveShell is required")
	}
	if invalidate == nil {
		invalidate = func() {}
	}
	if strings.TrimSpace(cfg.SessionID) == "" {
		cfg.SessionID = "marlin-main"
	}

	app := adaptive.Desktop
	transcript := app.Transcript
	targets := app.Targets
	if targets == nil {
		targets = transport.NewTargetRegistry()
		app.Targets = targets
	}
	federation := transport.NewFederationSupervisor(targets)
	app.Federation = federation
	if app.FederationUI == nil {
		app.FederationUI = mygoshell.NewFederationController(federation, targets)
	}
	var notifyCenter *notify.Center
	if app.Notifications != nil {
		notifyCenter = app.Notifications.Center
	}

	linkCtx, cancelLink := context.WithCancel(ctx)
	coalescer := grpcwire.NewFrameCoalescer(grpcwire.DefaultFrameInterval, invalidate)
	markDirty := coalescer.MarkDirty

	var (
		mode        = ModeStandalone
		targetID    = "standalone-preview"
		endpoint    = "offline-preview"
		provider    grpcwire.TargetProvider
		closeTarget func()
	)

	// 1. Resolve live Rust gRPC backend: explicit remote -host -> local supervised sidecar -> standalone preview.
	if addr := strings.TrimSpace(cfg.HostAddr); addr != "" {
		var tlsCfg *grpcwire.TLSConfig
		if cfg.TLSCAFile != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
			tlsCfg = &grpcwire.TLSConfig{
				CAFile:   cfg.TLSCAFile,
				CertFile: cfg.TLSCertFile,
				KeyFile:  cfg.TLSKeyFile,
			}
		}
		dialCfg := grpcwire.DialConfig{
			ID:          "remote-marlin-core",
			DisplayName: "Remote Marlin Core",
			Mode:        transport.ModeRemoteGRPC,
			Address:     addr,
			AuthToken:   cfg.BearerHeader(),
			TLS:         tlsCfg,
		}
		if dialCfg.AuthToken == "" && copts.authTokenSource != nil {
			dialCfg.AuthTokenSource = copts.authTokenSource
		}
		dialed, err := grpcwire.Dial(dialCfg)
		if err != nil {
			cancelLink()
			coalescer.Close()
			return nil, fmt.Errorf("dial remote Rust host %s: %w", addr, err)
		}
		_ = targets.Register(dialed)
		federation.ProbeAll(linkCtx, 2*time.Second)
		mode = ModeRemoteGRPC
		targetID = dialed.Meta().ID
		endpoint = addr
		provider = dialed
		closeTarget = func() { _ = dialed.Close() }
	} else if !cfg.Standalone {
		if binPath, src := appconfig.DiscoverAppServerBinary(cfg.AppServerBin); binPath != "" {
			var extraEnv []string
			if cfg.RealSession {
				extraEnv = append(extraEnv, "MARLIN_BRIDGE_REAL_SESSION=1")
			}
			if strings.TrimSpace(cfg.TursoPath) != "" {
				_ = os.MkdirAll(filepath.Dir(cfg.TursoPath), 0o700)
				extraEnv = append(extraEnv, "MARLIN_BRIDGE_TURSO_PATH="+cfg.TursoPath)
			}
			var notifyBridge *grpcwire.NotifyBridge
			if notifyCenter != nil {
				notifyBridge = grpcwire.NewNotifyBridge(notifyCenter)
			}
			resilient, err := grpcwire.LaunchResilientSidecar(linkCtx, grpcwire.ResilientSidecarConfig{
				ID:          "local-marlin-core",
				DisplayName: "Local Marlin Core",
				Sidecar: transport.SidecarConfig{
					BinaryPath:   binPath,
					ExtraEnv:     extraEnv,
					StartTimeout: 8 * time.Second,
					Stderr:       os.Stderr,
				},
				OnCrash: func(attempt int, exitErr error) {
					log.Printf("marlin sidecar exited (attempt %d): %v", attempt, exitErr)
					if notifyBridge != nil {
						notifyBridge.OnSidecarCrash("Local Marlin Core", attempt, exitErr)
					}
					markDirty()
				},
				OnRecovered: func(rc context.Context, newTarget *grpcwire.Target, restarts int) {
					federation.ProbeAll(rc, 2*time.Second)
					if notifyBridge != nil {
						notifyBridge.OnSidecarRecovered("Local Marlin Core", newTarget.Meta().Address, restarts)
					}
					markDirty()
				},
			})
			if err != nil {
				log.Printf("auto-launch Rust sidecar %s (%s) failed, falling back to standalone preview: %v", binPath, src, err)
			} else {
				_ = targets.Register(resilient)
				federation.ProbeAll(linkCtx, 2*time.Second)
				mode = ModeSidecar
				targetID = "local-marlin-core"
				if cur := resilient.CurrentTarget(); cur != nil {
					endpoint = cur.Meta().Address
				}
				provider = resilient
				closeTarget = func() { _ = resilient.Close() }
				log.Printf("connected to local Rust Marlin Core (%s via %s) at %s", binPath, src, endpoint)
			}
		}
	}

	switcher := grpcwire.NewSwitchableTargetProvider(targetID, provider)
	link := &Link{
		Mode:       mode,
		TargetID:   targetID,
		Endpoint:   endpoint,
		Targets:    targets,
		Federation: federation,
		Switcher:   switcher,
	}

	// 2. Wire fsync power-loss continuity & exponential-backoff outbox coordinator.
	scope := "chat:" + cfg.SessionID
	var (
		saveMu    sync.Mutex
		saveTimer *time.Timer
	)
	saveNow := func() {
		if transcript == nil || transcript.Resilience == nil || cfg.RecoveryFile == "" {
			return
		}
		if err := transcript.Resilience.SaveFile(cfg.RecoveryFile); err != nil {
			log.Printf("recovery snapshot save failed: %v", err)
		}
	}
	scheduleSave := func() {
		saveMu.Lock()
		if saveTimer != nil {
			saveTimer.Reset(150 * time.Millisecond)
		} else {
			saveTimer = time.AfterFunc(150*time.Millisecond, saveNow)
		}
		saveMu.Unlock()
	}
	if transcript != nil && transcript.Resilience != nil {
		if cfg.RecoveryFile != "" {
			if err := transcript.Resilience.LoadFile(cfg.RecoveryFile); err == nil {
				_ = transcript.RestoreDraftFromResilience(scope)
				if adaptive.Mobile != nil {
					adaptive.Mobile.RestoreDraftsFromResilience()
				}
			} else {
				transcript.ResilienceScope = scope
			}
		} else {
			transcript.ResilienceScope = scope
		}
		transcript.OnDraftChanged = func(_, _ string) { scheduleSave() }
		transcript.OnDiscardOutbox = func(_ string) {
			saveNow()
			markDirty()
		}
	}
	link.SaveNow = saveNow
	link.OnDraftChanged = func(_, _ string) { scheduleSave() }

	// 3. Wire Federation Cockpit live target switching & reconnect.
	app.FederationUI.OnSelectTarget = func(id string) {
		_, _ = targets.Switch(id)
		if ok, _ := switcher.SwitchTo(id); !ok {
			_, _ = switcher.ResolveFromRegistry(targets, id)
		}
		go func() {
			federation.ProbeAll(linkCtx, 2*time.Second)
			markDirty()
		}()
		markDirty()
	}
	app.FederationUI.OnReconnectTarget = func(id string) {
		if switcher.ActiveID() == id {
			switcher.ReconnectActive()
		} else {
			_, _ = switcher.ResolveFromRegistry(targets, id)
		}
		go func() {
			federation.ProbeAll(linkCtx, 2*time.Second)
			markDirty()
		}()
		markDirty()
	}

	// 4. Wire live gRPC SessionDriver + ProjectionStreamBridge + TurnStreamController OR standalone demo.
	var (
		stopDriver func()
		stopProj   func()
		usedTokens atomic.Int64
	)
	sr := &sessionRuntime{
		activeSessionID:  cfg.SessionID,
		openedSessions:   make(map[string]bool),
		authTokenSource:  copts.authTokenSource,
		saveNow:          saveNow,
		scheduleSave:     scheduleSave,
		onSessionChanged: copts.onSessionChanged,
		onRemoteAdded:    copts.onRemoteAdded,
	}
	if len(copts.savedRemotes) > 0 {
		registerSavedRemotes(linkCtx, cfg, copts.savedRemotes, targets, federation, sr, markDirty)
	}
	recordTokens := func(txt string) {
		if transcript == nil {
			return
		}
		add := composer.EstimateTokenCount(txt)
		if add <= 0 {
			return
		}
		total := int(usedTokens.Add(int64(add)))
		maxTok := transcript.ContextUsage().MaxTokens
		if maxTok <= 0 {
			maxTok = 200_000
		}
		transcript.SetContextUsage(total, maxTok)
	}

	if provider != nil {
		devBridge := device.NewBridge(targetKind)
		if copts.enableDesktopMyGoAdapters && (targetKind == schema.TargetDesktopMacOS || targetKind == schema.TargetDesktopLinux || targetKind == schema.TargetDesktopWindows) {
			_ = device.RegisterDesktopMyGoAdapters(linkCtx, devBridge)
		}
		wb := app.Workbench
		if wb == nil {
			wb = shell.NewWorkbenchController(shell.NewSlotRegistry())
		}
		driver := grpcwire.NewSessionDriver(grpcwire.DriverConfig{
			Provider:     switcher,
			TargetKind:   targetKind,
			SessionID:    schema.SessionID(cfg.SessionID),
			Workbench:    wb,
			Federation:   federation,
			DeviceBridge: devBridge,
			DeviceID:     fmt.Sprintf("marlin-ui-%s", targetKind),
			DisplayName:  "Marlin UI",
			OnWorkbench:  func(schema.UIWorkbenchSnapshot) { markDirty() },
			OnPendingGate: func(g *gatev1.PendingGate) {
				if app.PromptGate != nil {
					_ = app.ShowAuthPrompt(grpcwire.PromptRequestFromPendingGate(g))
				}
				markDirty()
			},
		})
		if _, err := driver.Start(linkCtx); err != nil {
			log.Printf("SessionDriver start warning: %v", err)
		}
		link.Driver = driver
		stopDriver = driver.Stop

		projBridge := grpcwire.NewProjectionStreamBridge(grpcwire.ProjectionBridgeConfig{
			Provider:   switcher,
			ProjectID:  "default",
			SessionID:  cfg.SessionID,
			Controller: app.Projection,
			OnUpdate:   func(mygoshell.ProjectionSnapshot) { markDirty() },
		})
		if err := projBridge.Start(linkCtx); err != nil {
			log.Printf("ProjectionStreamBridge start warning: %v", err)
		}
		stopProj = projBridge.Stop

		if app.PromptGate != nil {
			app.PromptGate.OnResolve = func(req promptgate.PromptRequest, resp promptgate.PromptResponse) {
				go func() {
					if req.Kind == promptgate.PromptAuth {
						_, _ = grpcwire.DispatchPromptResponseToDriver(linkCtx, driver, req, resp)
					} else if resp.Action == promptgate.ActionSubmit && resp.TextInput != "" && app.OnSendPrompt != nil {
						app.OnSendPrompt(resp.TextInput)
					}
					markDirty()
				}()
			}
		}

		turnCtrl := grpcwire.NewTurnStreamControllerWithProvider(switcher)
		link.TurnCtrl = turnCtrl
		sr.turnCtrl = turnCtrl
		sr.composerBridge = grpcwire.NewComposerBridge(turnCtrl, nil)

		var (
			turnMu    sync.Mutex
			lastTurn  = make(map[string]uint64)
			activeRef *appv1.SessionTurnRef
			subSeq    atomic.Uint64
		)
		sendLive := func(displayPrompt, wirePrompt string, refs []composer.AttachmentRef) {
			if strings.TrimSpace(wirePrompt) == "" {
				return
			}
			if len(refs) > 0 {
				transcript.AppendUserWithAttachments(displayPrompt, refs)
			} else {
				transcript.AppendUser(displayPrompt)
			}
			recordTokens(wirePrompt)
			markDirty()
			sid := sr.currentSession()
			curScope := "chat:" + sid
			go func() {
				turnMu.Lock()
				if !sr.openedSessions[sid] {
					if _, err := turnCtrl.OpenSession(linkCtx, sid, true); err != nil {
						_, _ = turnCtrl.OpenSession(linkCtx, sid, false)
					}
					sr.openedSessions[sid] = true
				}
				after := lastTurn[sid]
				subID := fmt.Sprintf("marlin-ui-sub-%d", subSeq.Add(1))
				turnMu.Unlock()

				ref, err := turnCtrl.StartTurn(linkCtx, sid, wirePrompt, after, subID)
				if err != nil {
					if transcript.Resilience != nil {
						transcript.Resilience.MarkOffline(err.Error())
						_, _ = transcript.Resilience.Outbox.Enqueue(curScope, "user", "", wirePrompt)
						if transcript.OnRetryOutbox != nil {
							transcript.OnRetryOutbox()
						}
						scheduleSave()
						markDirty()
					}
					return
				}
				if transcript.Resilience != nil {
					transcript.Resilience.MarkOnline()
				}
				turnMu.Lock()
				activeRef = ref
				turnMu.Unlock()
				transcript.BeginAssistantTurn(ref.GetOrdinal())
				markDirty()

				_, outcome, watchErr := turnCtrl.WatchTurnResilient(linkCtx, ref, "", grpcwire.WatchRetryPolicy{
					InitialBackoff: 50 * time.Millisecond,
					MaxBackoff:     2 * time.Second,
					MaxRetries:     8,
					OnReconnect: func(attempt int, afterEventID string, rErr error) {
						if transcript.Resilience != nil {
							transcript.Resilience.MarkReconnecting(curScope, attempt, afterEventID, rErr)
							markDirty()
						}
					},
				}, func(upd grpcwire.TurnStreamUpdate) {
					if transcript.Resilience != nil {
						transcript.Resilience.MarkOnline()
						if upd.EventID != "" {
							if dup, _ := transcript.Resilience.AdmitEvent(curScope, upd.EventID, upd.TurnOrdinal, 0); dup {
								return
							}
						}
					}
					if upd.TextDelta != "" {
						transcript.ApplyCheckpoint(upd.TurnOrdinal, upd.EventID, upd.TextDelta)
						recordTokens(upd.TextDelta)
						markDirty()
					}
					if upd.HasOutcome {
						transcript.CompleteTurn(upd.TurnOrdinal, upd.Outcome.String())
						markDirty()
					}
				})
				if watchErr != nil {
					if transcript.Resilience != nil {
						transcript.Resilience.MarkOffline(watchErr.Error())
						scheduleSave()
					}
					transcript.CompleteTurn(ref.GetOrdinal(), "INTERRUPTED")
					markDirty()
				} else if outcome == appv1.SessionTurnOutcome_SESSION_TURN_OUTCOME_COMPLETED {
					turnMu.Lock()
					if ref.GetOrdinal() > lastTurn[sid] {
						lastTurn[sid] = ref.GetOrdinal()
					}
					activeRef = nil
					turnMu.Unlock()
				}
			}()
		}
		sr.sendPromptRaw = sendLive
		app.OnSendPrompt = func(prompt string) { sendLive(prompt, prompt, nil) }
		app.OnCancelTurn = func() {
			turnMu.Lock()
			ref := activeRef
			turnMu.Unlock()
			if ref == nil {
				return
			}
			go func() {
				_, _ = turnCtrl.CancelTurn(linkCtx, ref)
				markDirty()
			}()
		}
	} else {
		var demoTurn atomic.Uint64
		sendPreview := func(displayPrompt, wirePrompt string, refs []composer.AttachmentRef) {
			if strings.TrimSpace(wirePrompt) == "" {
				return
			}
			if len(refs) > 0 {
				transcript.AppendUserWithAttachments(displayPrompt, refs)
			} else {
				transcript.AppendUser(displayPrompt)
			}
			ord := demoTurn.Add(1)
			transcript.BeginAssistantTurn(ord)
			reply := fmt.Sprintf("[standalone preview · %s] Received %q — start marlin-app-server or pass -host <addr> to connect live gRPC.", sr.currentSession(), wirePrompt)
			transcript.ApplyCheckpoint(ord, fmt.Sprintf("preview-%d", ord), reply)
			transcript.CompleteTurn(ord, "PREVIEW_OK")
			recordTokens(wirePrompt + reply)
			markDirty()
		}
		sr.sendPromptRaw = sendPreview
		app.OnSendPrompt = func(prompt string) { sendPreview(prompt, prompt, nil) }
	}

	if adaptive.Mobile != nil {
		adaptive.Mobile.OnSendPrompt = func(p string) {
			if app.OnSendPrompt != nil {
				app.OnSendPrompt(p)
			}
		}
	}
	wireSessionAndHelm(linkCtx, cfg, adaptive, targets, federation, switcher, sr, markDirty)

	// 5. Start ContinuityCoordinator for exponential-backoff reconnect & idempotent outbox draining.
	if transcript != nil && transcript.Resilience != nil {
		coordCfg := grpcwire.ContinuityConfig{
			ScopeID:        scope,
			Resilience:     transcript.Resilience,
			Provider:       switcher,
			InitialBackoff: 200 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
			DeliverFn: func(_ context.Context, it resilience.OutboxItem) error {
				if app.OnSendPrompt != nil {
					app.OnSendPrompt(it.Text)
				}
				return nil
			},
			OnStateChanged: func(_ resilience.ResilienceSnapshot) {
				scheduleSave()
				markDirty()
			},
		}
		if provider == nil {
			coordCfg.ProbeFn = func(_ context.Context) error { return nil }
		}
		coord := grpcwire.NewContinuityCoordinator(coordCfg)
		link.Coord = coord
		go coord.Run(linkCtx)
		transcript.OnRetryOutbox = func() {
			coord.TriggerReconnect("manual outbox retry")
		}
	}

	link.closeFn = func() {
		cancelLink()
		saveMu.Lock()
		if saveTimer != nil {
			saveTimer.Stop()
		}
		saveMu.Unlock()
		saveNow()
		if stopProj != nil {
			stopProj()
		}
		if stopDriver != nil {
			stopDriver()
		}
		if closeTarget != nil {
			closeTarget()
		}
		sr.closeExtras()
		coalescer.Close()
	}

	return link, nil
}
