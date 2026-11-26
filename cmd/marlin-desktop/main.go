// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

// Command marlin-desktop is the native 120Hz pure-Go desktop client for Marlin
// Agent Core (macOS, Linux, Windows). Marlin UI owns its hierarchical Doom
// Emacs workbench (internal/emacs/{frame,window,buffer,tabs,modeline,minibuffer,meow,theme}),
// declared in project.sysml and dev/sysml/emacs-workbench.sysml, while
// github.com/tao3k/marlin-mygo provides the headless engines and gRPC wire.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func desktopTargetKind() schema.TargetKind {
	switch runtime.GOOS {
	case "linux":
		return schema.TargetDesktopLinux
	case "windows":
		return schema.TargetDesktopWindows
	default:
		return schema.TargetDesktopMacOS
	}
}

func main() {
	cfg := appconfig.DefaultConfig()
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "print build version and exit")
	flag.StringVar(&cfg.ConfigFile, "config", cfg.ConfigFile, "Marlin model & host TOML config file (e.g. ./example.toml or ~/.config/marlin/config.toml)")
	flag.StringVar(&cfg.AppServerBin, "app-server", cfg.AppServerBin, "local Rust marlin-app-server binary (auto-discovered if omitted)")
	flag.StringVar(&cfg.HostAddr, "host", cfg.HostAddr, "remote Rust marlin-agent-core gRPC address (e.g. 127.0.0.1:50051)")
	flag.StringVar(&cfg.AuthToken, "token", cfg.AuthToken, "Bearer auth token for remote Rust host")
	flag.StringVar(&cfg.TLSCAFile, "ca", cfg.TLSCAFile, "optional TLS CA PEM file")
	flag.StringVar(&cfg.TLSCertFile, "cert", cfg.TLSCertFile, "optional mTLS client certificate PEM file")
	flag.StringVar(&cfg.TLSKeyFile, "key", cfg.TLSKeyFile, "optional mTLS client private key PEM file")
	flag.StringVar(&cfg.SessionID, "session", cfg.SessionID, "Marlin session ID")
	flag.StringVar(&cfg.ThemeID, "theme", cfg.ThemeID, "initial CSS theme: nord-bento | doom-one | doom-tokyo-night | doom-one-light (or ~/.config/marlin/ui/ pack)")
	flag.StringVar(&cfg.FeaturesSpec, "features", cfg.FeaturesSpec, "opt-in UI feature profile: default | all | none | chatroom,compare,canvas")
	flag.StringVar(&cfg.TursoPath, "turso", cfg.TursoPath, "Turso SQLite file path for supervised local Rust host")
	flag.StringVar(&cfg.RecoveryFile, "recovery-file", cfg.RecoveryFile, "atomic fsync crash-recovery snapshot file")
	flag.StringVar(&cfg.PrefsFile, "prefs-file", cfg.PrefsFile, "atomic local UI preferences JSON file")
	flag.BoolVar(&cfg.Standalone, "standalone", cfg.Standalone, "run UI-only offline preview without spawning local Rust sidecar")
	flag.BoolVar(&cfg.RealSession, "real-session", cfg.RealSession, "enable durable ConversationJournal streaming on local Rust sidecar")
	flag.Parse()

	if showVersion {
		fmt.Println(appconfig.BuildSummary("marlin-desktop"))
		return
	}

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })

	cfg.ModelCfg = appconfig.LoadModelConfig(cfg.ConfigFile)
	if cfg.ModelCfg.Warning != "" {
		log.Printf("model config warning: %s", cfg.ModelCfg.Warning)
	} else if cfg.ModelCfg.Loaded {
		cfg.ConfigFile = cfg.ModelCfg.SourcePath
		if !explicitFlags["host"] && cfg.HostAddr == "" && cfg.ModelCfg.Host.Addr != "" {
			cfg.HostAddr = cfg.ModelCfg.Host.Addr
		}
		if !explicitFlags["app-server"] && cfg.AppServerBin == "" && cfg.ModelCfg.Host.AppServerBin != "" {
			cfg.AppServerBin = cfg.ModelCfg.Host.AppServerBin
		}
		if !explicitFlags["session"] && cfg.ModelCfg.Session.Session != "" {
			cfg.SessionID = cfg.ModelCfg.Session.Session
		}
		if !explicitFlags["real-session"] && cfg.ModelCfg.Host.RealSession != nil {
			cfg.RealSession = *cfg.ModelCfg.Host.RealSession
		}
		log.Printf("loaded model config %s (%s): %s", cfg.ModelCfg.SourcePath, cfg.ModelCfg.SourceOrigin, cfg.ModelCfg.SummaryLabel())
	}

	prefs, err := appconfig.LoadPrefs(cfg.PrefsFile)
	if err != nil {
		log.Printf("prefs load warning (%s): %v", cfg.PrefsFile, err)
	}
	themeDir := appconfig.DefaultUIThemeDir()
	themeBundle := appconfig.LoadUIThemeBundle(themeDir)
	if !explicitFlags["theme"] {
		if prefs.ThemeID != "" {
			cfg.ThemeID = prefs.ThemeID
		} else if themeBundle.Manifest.ActiveTheme != "" {
			cfg.ThemeID = themeBundle.Manifest.ActiveTheme
		}
	}
	if !explicitFlags["features"] && prefs.FeaturesSpec != "" {
		cfg.FeaturesSpec = prefs.FeaturesSpec
	}
	if !explicitFlags["session"] && prefs.LastSessionID != "" {
		cfg.SessionID = prefs.LastSessionID
	}
	if !explicitFlags["host"] && cfg.HostAddr == "" && prefs.DefaultRemoteHost != "" {
		cfg.HostAddr = prefs.DefaultRemoteHost
	}
	if !explicitFlags["real-session"] && prefs.RealSession != nil {
		cfg.RealSession = *prefs.RealSession
	}

	mygo.App.SetName("Marlin")
	mygo.App.SetVersion(appconfig.Version)
	if !mygo.App.RequestSingleInstanceLock() {
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bgScope := async.NewStructuredScope(ctx)

	// Headless controllers from marlin-mygo (Transcript, Helm, Projection, Auth, PromptGate, WindowTree).
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	adaptive.Desktop.ActiveSessionID = cfg.SessionID
	adaptive.Desktop.Sessions = []mygoshell.SessionSummaryView{
		{SessionID: cfg.SessionID, Title: "Primary Workspace", TurnCount: 0},
	}

	var (
		prefsMu   sync.Mutex
		win       *mygo.Window
		tray      *mygo.Tray
		emacsWin  *frame.Frame
		link      *hostlink.Link
		stopVoice func()
	)
	// Bind Tier-0 bidirectional Hybrid WebView RPC service (MyGo physically blocks
	// Tier-2 Untrusted pages from calling any bound Go methods).
	mygo.Bind(WorkflowBridge{
		onSave: func(g kit.WorkflowGraph) {
			if emacsWin != nil {
				emacsWin.Echo(fmt.Sprintf("Workflow synced (%d nodes, %d edges)", len(g.Nodes), len(g.Edges)))
			}
			if win != nil {
				win.Invalidate()
			}
		},
	})
	savePrefs := func(next appconfig.UserPrefs) {
		prefsMu.Lock()
		prefs = next
		snap := prefs
		prefsMu.Unlock()
		if err := appconfig.SavePrefs(cfg.PrefsFile, snap); err != nil {
			log.Printf("prefs save warning: %v", err)
		}
	}

	mygo.App.OnSecondInstance(func(_ []string, _ string) {
		mygo.App.Focus()
		if win != nil {
			win.Show()
			win.Focus()
		}
	})
	mygo.App.OnOpenFile(func(path string) {
		if path == "" || adaptive.Desktop == nil || adaptive.Desktop.Transcript == nil {
			return
		}
		_, _ = adaptive.Desktop.Transcript.StageLocalFiles([]string{path})
		if emacsWin != nil {
			emacsWin.Echo("Attached " + path)
		}
		if win != nil {
			win.Invalidate()
		}
	})
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		cancel()
		bgScope.CloseAndDrain()
		if emacsWin != nil {
			emacsWin.Close()
		}
		featurekit.ShutdownWebIslands()
		prefsMu.Lock()
		snap := prefs
		prefsMu.Unlock()
		_ = appconfig.SavePrefs(cfg.PrefsFile, snap)
		if tray != nil {
			tray.Destroy()
		}
		if stopVoice != nil {
			stopVoice()
		}
		if link != nil {
			link.SaveNow()
			link.Close()
		}
	})

	mygo.App.WhenReady(func() {
		invalidate := func() {
			if win != nil {
				win.Invalidate()
			}
		}
		focusWindow := func() {
			mygo.App.Focus()
			if win != nil {
				win.Show()
				win.Focus()
			}
		}

		var applyFeatures func(string)
		authCtrl := hostlink.AttachAuthAndSettings(ctx, adaptive, hostlink.AuthSettingsConfig{
			Config:   cfg,
			Prefs:    prefs,
			CopyText: mygo.Clipboard.WriteText,
			OnApplyFeatures: func(spec string) {
				if applyFeatures != nil {
					applyFeatures(spec)
				}
			},
			Invalidate: invalidate,
		})

		var connErr error
		link, connErr = hostlink.Connect(
			ctx,
			cfg,
			adaptive,
			desktopTargetKind(),
			invalidate,
			hostlink.WithDesktopMyGoAdapters(),
			hostlink.WithClipboardWriter(mygo.Clipboard.WriteText),
			hostlink.WithAuthTokenSource(hostlink.OptionalAuthTokenSource(authCtrl)),
			hostlink.WithSavedRemotes(prefs.SavedRemotes),
			hostlink.WithOnSessionChanged(func(sid string) {
				prefsMu.Lock()
				prefs.LastSessionID = sid
				snap := prefs
				prefsMu.Unlock()
				savePrefs(snap)
			}),
			hostlink.WithOnRemoteAdded(func(r appconfig.SavedRemote) {
				if emacsWin != nil {
					emacsWin.SyncSavedRemote(r)
				}
			}),
		)
		if connErr != nil {
			log.Fatalf("marlin-desktop host connection failed: %v", connErr)
		}

		reg, ledger, host := featurekit.Mount(cfg.FeaturesSpec, adaptive, link.OnDraftChanged, invalidate)
		applyFeatures = func(spec string) {
			reg.Activate(features.ParseFeatureSpec(spec), ledger, host)
			adaptive.SetFeatureHost(host, adaptive.Desktop.GovernanceUI)
		}

		tr := adaptive.Desktop.Transcript
		tr.OnPickAttachment = func() {
			bgScope.Go(func(taskCtx context.Context) {
				paths, err := composer.PickFilesNative(taskCtx, composer.NativePickerOptions{
					Title:         "Attach files to Marlin",
					AllowMultiple: true,
				})
				if err == nil && len(paths) > 0 {
					_, _ = tr.StageLocalFiles(paths)
					if emacsWin != nil {
						emacsWin.Echo(fmt.Sprintf("Attached %d file(s)", len(paths)))
					}
					invalidate()
				}
			})
		}
		tr.OnPasteAttachment = func() {
			bgScope.Go(func(taskCtx context.Context) {
				if in, ok, err := composer.ReadClipboardAttachment(taskCtx); err == nil && ok {
					_, _ = tr.AttachFile(in)
					invalidate()
				}
			})
		}
		stopVoice = wireDesktopVoice(bgScope, tr, invalidate)

		var (
			hasLastVibrancy bool
			lastVibrancy    mygo.Vibrancy
		)
		applyWindowGlass := func(glassOn, isLight bool) {
			if win == nil {
				return
			}
			vibMode := "auto"
			if emacsWin != nil {
				vibMode = emacsWin.VibrancyMode()
				pal, op := emacsWin.ActivePaletteAndOpacity()
				featurekit.SyncWebIslandTheme(pal, op)
			}
			v := desktopVibrancy(glassOn, isLight, vibMode)
			if !hasLastVibrancy || v != lastVibrancy {
				hasLastVibrancy = true
				lastVibrancy = v
				win.SetVibrancy(v)
				win.SetActiveAppearance(v != mygo.VibrancyNone)
			}
		}

		featurekit.RegisterUserIslands(themeBundle.WebIslands)
		syncCfg := marketplace.OfficialSyncConfig{
			LocalRepoDir: strings.TrimSpace(os.Getenv("MARLIN_MARKETPLACE_DIR")),
		}
		syncClient, _ := marketplace.NewOfficialSyncClient(syncCfg)
		pkgStore, pkgErr := marketplace.NewPackageStore(marketplace.PackageStoreConfig{
			SyncClient: syncClient,
			StateFile:  filepath.Join(themeDir, "packages-state.json"),
			UIDir:      themeDir,
		})
		if pkgErr != nil {
			log.Printf("marketplace package store warning: %v", pkgErr)
		}
		emacsWin = frame.New(frame.Config{
			Adaptive:            adaptive,
			Link:                link,
			AppConfig:           cfg,
			Prefs:               prefs,
			FeatureReg:          reg,
			FeatureLedger:       ledger,
			FeatureHost:         host,
			PackageStore:        pkgStore,
			WebHost:               featurekit.WebIslandHost(),
			OnWebIslandAction:     featurekit.DispatchWebIslandAction,
			OnWebIslandVisibility: featurekit.NotifyWebIslandVisibility,
			OnPushArtifact:        featurekit.PushArtifactStagePayload,
			InitialThemeID:        cfg.ThemeID,
			ThemeBundle:         themeBundle,
			OnReloadThemeBundle: func() appconfig.UIThemeBundle {
				b := appconfig.LoadUIThemeBundle(themeDir)
				featurekit.RegisterUserIslands(b.WebIslands)
				return b
			},
			OnGlassChanged: func(on bool) {
				isLight := false
				if emacsWin != nil {
					isLight = emacsWin.IsLightTheme()
				}
				applyWindowGlass(on, isLight)
			},
			OnThemeChanged: func(isLight bool) {
				if emacsWin != nil {
					applyWindowGlass(emacsWin.UIModules().Glass, isLight)
				}
			},
			RoomStats: func() (int, int, int, int) {
				st := featurekit.AgentRoomStats()
				return st.TasksDone, st.TasksTotal, st.AgentsOnline, st.AgentsTotal
			},
			RoomTasks: func() []widgets.TaskItem {
				raw := featurekit.AgentRoomTasks()
				out := make([]widgets.TaskItem, 0, len(raw))
				for _, r := range raw {
					out = append(out, widgets.TaskItem{ID: r.ID, Title: r.Title, Status: r.Status, Owner: r.Owner})
				}
				return out
			},
			OnAddTask:       featurekit.AddAgentRoomTask,
			OnCycleTask:     featurekit.CycleAgentRoomTask,
			OnPrefsChanged:  savePrefs,
			OnApplyFeatures: applyFeatures,
			Invalidate:      invalidate,
		})

		mygo.App.SetMenu(buildDesktopMenu(emacsWin, invalidate))
		tray = setupDesktopTray(emacsWin, focusWindow, invalidate)

		initGlass := emacsWin.UIModules().Glass
		initVibrancy := desktopVibrancy(initGlass, emacsWin.IsLightTheme(), emacsWin.VibrancyMode())
		initPal, initOp := emacsWin.ActivePaletteAndOpacity()
		featurekit.SyncWebIslandTheme(initPal, initOp)
		title := fmt.Sprintf("Marlin — %s", link.Endpoint)
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:         title,
			Width:         1280,
			Height:        820,
			TitleBarStyle: mygo.TitleBarHiddenInset,
			Transparent:   true,
			Vibrancy:      initVibrancy,
			StateKey:      "marlin-desktop-main",
			Content:       ui.View(emacsWin.View),
		})
		win.SetActiveAppearance(initGlass)
		featurekit.SetWebViewInvalidate(invalidate)
		featurekit.SetWebViewFactory(func(islandID string, html, preloadJS string, tier async.TrustTier) (featurekit.WebViewInstance, error) {
			if win == nil {
				return featurekit.WebViewInstance{}, fmt.Errorf("window unavailable")
			}
			pageOpts := mygo.PageOptions{
				PreloadScript: preloadJS,
				Untrusted:     tier != async.TrustTier0FirstParty,
			}
			wv, err := win.NewWebView(mygo.WebViewOptions{
				Transparent: true,
				Page:        pageOpts,
			})
			if err != nil {
				return featurekit.WebViewInstance{}, err
			}
			wv.Page().OnRenderProcessGone(func(_ string) {
				featurekit.RecoverWebIsland(islandID)
			})
			wv.Page().LoadHTML(html, "")
			return featurekit.WebViewInstance{
				View: wv,
				Tier: tier,
				Eval: func(js string) {
					if !wv.IsDestroyed() {
						_, _ = wv.Page().Eval(js)
					}
				},
				ReloadHTML: func(nextHTML string) {
					if !wv.IsDestroyed() {
						wv.Page().LoadHTML(nextHTML, "")
					}
				},
				Destroy: func() {
					if !wv.IsDestroyed() {
						wv.Destroy()
					}
				},
			}, nil
		})
		win.Invalidate()

		// Pre-warm the built-in *Workflow Canvas* WKWebView off the first-click
		// critical path shortly after the initial Dashboard frame has painted,
		// eliminating the 300ms+ WebKit cold-boot stall when switching tabs.
		bgScope.Go(func(warmCtx context.Context) {
			timer := time.NewTimer(350 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-warmCtx.Done():
				return
			case <-timer.C:
				featurekit.PrewarmWebIsland(featurekit.BuiltinWorkflowStudioIslandID)
			}
		})

		// Live auto-reload watcher inside StructuredScope (quiescence-drained on quit).
		bgScope.Go(func(watchCtx context.Context) {
			appconfig.WatchUIThemeDir(watchCtx, themeDir, 600*time.Millisecond, func() {
				if emacsWin != nil {
					emacsWin.ReloadThemesAsync()
				}
			})
		})

		// Demand-driven 1Hz live clock tick inside StructuredScope:
		// automatically pauses when unfocused or viewing non-Dashboard buffers
		// to eliminate macOS transparent-window idle GPU power residency.
		bgScope.Go(func(loopCtx context.Context) {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-loopCtx.Done():
					return
				case <-ticker.C:
					if emacsWin != nil && emacsWin.ShouldTickIdle() && win != nil {
						win.Invalidate()
					}
				}
			}
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	if stopVoice != nil {
		stopVoice()
	}
	if link != nil {
		link.Close()
	}
}

// desktopVibrancy resolves the declarative TOML [surface].vibrancy material
// ("auto", "hud", "under-window", "sidebar", "popover", "none") to a flat 1:1
// OS frosted vibrancy material (NSVisualEffectView on macOS, Acrylic/Mica on
// Windows 11) with zero lens magnification or edge warping.
func desktopVibrancy(glassOn, isLight bool, mode string) mygo.Vibrancy {
	if !glassOn {
		return mygo.VibrancyNone
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return mygo.VibrancyNone
	case "hud":
		return mygo.VibrancyHUD
	case "under-window":
		return mygo.VibrancyUnderWindow
	case "sidebar":
		return mygo.VibrancySidebar
	case "popover":
		return mygo.VibrancyPopover
	default:
		if isLight {
			return mygo.VibrancyUnderWindow
		}
		return mygo.VibrancyHUD
	}
}

func wireDesktopVoice(scope *async.StructuredScope, tr *mygoshell.TranscriptController, invalidate func()) func() {
	rec := composer.NewNativeAudioRecorder()
	tr.OnStartVoice = func() {
		scope.Go(func(taskCtx context.Context) {
			if err := rec.Start(taskCtx, tr.Voice, invalidate); err != nil {
				if tr.Voice != nil {
					tr.Voice.Fail(err)
				}
				invalidate()
			}
		})
	}
	tr.OnStopVoice = func() {
		scope.Go(func(_ context.Context) {
			if name, wav, err := rec.Stop(); err == nil && len(wav) > 0 {
				_, _ = tr.CompleteVoiceAudio(name, wav)
			} else if err != nil && tr.Voice != nil {
				tr.Voice.Fail(err)
			}
			invalidate()
		})
	}
	tr.OnCancelVoice = func() {
		rec.Cancel()
		invalidate()
	}
	return func() {
		rec.Cancel()
	}
}

// WorkflowBridge is the Tier-0 bidirectional RPC service exposed exclusively to
// trusted first-party Hybrid WebView studios (*Workflow Canvas*). Tier-2
// user islands run with PageOptions.Untrusted=true and cannot invoke it.
type WorkflowBridge struct {
	onSave func(kit.WorkflowGraph)
}

func (b WorkflowBridge) SaveGraph(_ context.Context, rawJSON string) (string, error) {
	if err := featurekit.CheckWorkflowBridgeRPC(); err != nil {
		return "", err
	}
	g, err := kit.ValidateWorkflowGraphJSON(rawJSON)
	if err != nil {
		return "", err
	}
	if b.onSave != nil {
		b.onSave(g)
	}
	return fmt.Sprintf("Saved %d nodes, %d edges", len(g.Nodes), len(g.Edges)), nil
}
