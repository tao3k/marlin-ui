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
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
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
	flag.StringVar(&cfg.CoveURL, "cove", cfg.CoveURL, "Marlin Cove cloud room relay gRPC HTTP/2 address (e.g. 127.0.0.1:18090)")
	flag.StringVar(&cfg.CoveToken, "cove-token", cfg.CoveToken, "Bearer auth token for Marlin Cove gRPC relay")
	flag.StringVar(&cfg.CoveRoomID, "cove-room", cfg.CoveRoomID, "Target Marlin Cove room ID (default cove-main)")
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

	appconfig.LoadDotEnv(cfg.ConfigFile)
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bgScope := async.NewStructuredScope(ctx)

	// Headless controllers from marlin-mygo (Transcript, Helm, Projection, Auth, PromptGate, WindowTree).
	adaptive := newDesktopAdaptiveShell(cfg.SessionID)

	rt, acquired := configureDesktopPreRun(desktopPreRunConfig{
		AppConfig:                 cfg,
		Prefs:                     prefs,
		Cancel:                    cancel,
		BGScope:                   bgScope,
		Adaptive:                  adaptive,
		AcquireSingleInstanceLock: true,
		OnReady: func(rt *desktopRuntimeState, savePrefs func(appconfig.UserPrefs)) {
			if iconPNG := theme.MarlinAppIconPNG(); len(iconPNG) > 0 {
				if err := mygo.App.Dock.SetIcon(iconPNG); err != nil {
					log.Printf("desktop dock icon warning: %v", err)
				}
			}
			invalidate := func() {
				if rt.win != nil {
					rt.win.Invalidate()
				}
			}
			focusWindow := func() {
				mygo.App.Focus()
				if rt.win != nil {
					rt.win.Show()
					rt.win.Focus()
				}
			}

			var applyFeatures func(string)
			authCtrl := hostlink.AttachAuthAndSettings(ctx, adaptive, hostlink.AuthSettingsConfig{
				Config:   cfg,
				Prefs:    rt.snapshotPrefs(),
				CopyText: mygo.Clipboard.WriteText,
				OnApplyFeatures: func(spec string) {
					if applyFeatures != nil {
						applyFeatures(spec)
					}
				},
				Invalidate: invalidate,
			})

			var connErr error
			rt.link, connErr = hostlink.Connect(
				ctx,
				cfg,
				adaptive,
				desktopTargetKind(),
				invalidate,
				hostlink.WithDesktopMyGoAdapters(),
				hostlink.WithClipboardWriter(mygo.Clipboard.WriteText),
				hostlink.WithAuthTokenSource(hostlink.OptionalAuthTokenSource(authCtrl)),
				hostlink.WithSavedRemotes(rt.snapshotPrefs().SavedRemotes),
				hostlink.WithOnSessionChanged(func(sid string) {
					snap := rt.snapshotPrefs()
					snap.LastSessionID = sid
					savePrefs(snap)
				}),
				hostlink.WithOnRemoteAdded(func(r appconfig.SavedRemote) {
					if rt.emacsWin != nil {
						rt.emacsWin.SyncSavedRemote(r)
					}
				}),
			)
			if connErr != nil {
				log.Fatalf("marlin-desktop host connection failed: %v", connErr)
			}

			reg, ledger, host := featurekit.Mount(cfg.FeaturesSpec, adaptive, rt.link.OnDraftChanged, invalidate)
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
						if rt.emacsWin != nil {
							rt.emacsWin.Echo(fmt.Sprintf("Attached %d file(s)", len(paths)))
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
			rt.stopVoice = wireDesktopVoice(bgScope, tr, invalidate)

			var (
				hasLastVibrancy bool
				lastVibrancy    mygo.Vibrancy
			)
			applyWindowGlass := func(glassOn, isLight bool) {
				if rt.win == nil {
					return
				}
				vibMode := "auto"
				if rt.emacsWin != nil {
					vibMode = rt.emacsWin.VibrancyMode()
					pal, op := rt.emacsWin.ActivePaletteAndOpacity()
					featurekit.SyncWebIslandTheme(pal, op)
				}
				v := desktopVibrancy(glassOn, isLight, vibMode)
				if !hasLastVibrancy || v != lastVibrancy {
					hasLastVibrancy = true
					lastVibrancy = v
					rt.win.SetVibrancy(v)
					rt.win.SetActiveAppearance(v != mygo.VibrancyNone)
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
			rt.emacsWin = frame.New(frame.Config{
				Adaptive:              adaptive,
				Link:                  rt.link,
				AppConfig:             cfg,
				Prefs:                 rt.snapshotPrefs(),
				FeatureReg:            reg,
				FeatureLedger:         ledger,
				FeatureHost:           host,
				PackageStore:          pkgStore,
				WebHost:               featurekit.WebIslandHost(),
				OnWebIslandAction:     featurekit.DispatchWebIslandAction,
				OnWebIslandVisibility: featurekit.NotifyWebIslandVisibility,
				OnPushArtifact:        featurekit.PushArtifactStagePayload,
				InitialThemeID:        cfg.ThemeID,
				ThemeBundle:           themeBundle,
				OnReloadThemeBundle: func() appconfig.UIThemeBundle {
					b := appconfig.LoadUIThemeBundle(themeDir)
					featurekit.RegisterUserIslands(b.WebIslands)
					return b
				},
				OnGlassChanged: func(on bool) {
					isLight := false
					if rt.emacsWin != nil {
						isLight = rt.emacsWin.IsLightTheme()
					}
					applyWindowGlass(on, isLight)
				},
				OnThemeChanged: func(isLight bool) {
					if rt.emacsWin != nil {
						applyWindowGlass(rt.emacsWin.UIModules().Glass, isLight)
					}
				},
				OnZenModeChanged: func(writeroom, fullscreen bool) {
					featurekit.SetRoomWriteroom(writeroom)
					if rt.win != nil {
						rt.win.SetFullScreen(fullscreen)
					}
				},
				OnQuit: func() {
					mygo.App.Quit()
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

			mygo.App.SetMenu(buildDesktopMenu(rt.emacsWin, invalidate))
			featurekit.SetRoomWriteroomExit(func() {
				if rt.emacsWin != nil {
					rt.emacsWin.ToggleZenMode()
					invalidate()
				}
			})
			rt.tray = setupDesktopTray(rt.emacsWin, focusWindow, invalidate)

			initGlass := rt.emacsWin.UIModules().Glass
			initVibrancy := desktopVibrancy(initGlass, rt.emacsWin.IsLightTheme(), rt.emacsWin.VibrancyMode())
			initPal, initOp := rt.emacsWin.ActivePaletteAndOpacity()
			featurekit.SyncWebIslandTheme(initPal, initOp)
			title := fmt.Sprintf("Marlin — %s", rt.link.Endpoint)
			rt.win = mygo.NewWindow(mygo.WindowOptions{
				Title:         title,
				Width:         1280,
				Height:        820,
				TitleBarStyle: mygo.TitleBarHiddenInset,
				Transparent:   true,
				Vibrancy:      initVibrancy,
				StateKey:      "marlin-desktop-main",
				Content:       ui.View(rt.emacsWin.View),
			})
			rt.win.SetActiveAppearance(initGlass)
			if iconPNG := theme.MarlinAppIconPNG(); len(iconPNG) > 0 {
				_ = rt.win.SetIcon(iconPNG)
			}
			featurekit.SetWebViewInvalidate(invalidate)
			featurekit.SetWebViewFactory(func(islandID string, html, preloadJS string, tier async.TrustTier) (featurekit.WebViewInstance, error) {
				if rt.win == nil {
					return featurekit.WebViewInstance{}, fmt.Errorf("window unavailable")
				}
				pageOpts := mygo.PageOptions{
					PreloadScript: preloadJS,
					Untrusted:     tier != async.TrustTier0FirstParty,
				}
				wv, err := rt.win.NewWebView(mygo.WebViewOptions{
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
			rt.win.Invalidate()

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
					if rt.emacsWin != nil {
						rt.emacsWin.ReloadThemesAsync()
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
						if rt.emacsWin != nil && rt.emacsWin.ShouldTickIdle() && rt.win != nil {
							rt.win.Invalidate()
						}
					}
				}
			})
		},
	})
	if !acquired {
		os.Exit(0)
	}

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	if rt.stopVoice != nil {
		rt.stopVoice()
	}
	if rt.link != nil {
		rt.link.Close()
	}
}

type desktopRuntimeState struct {
	prefsMu   sync.Mutex
	prefs     appconfig.UserPrefs
	win       *mygo.Window
	tray      *mygo.Tray
	emacsWin  *frame.Frame
	link      *hostlink.Link
	stopVoice func()
}

func (rt *desktopRuntimeState) snapshotPrefs() appconfig.UserPrefs {
	rt.prefsMu.Lock()
	defer rt.prefsMu.Unlock()
	return rt.prefs
}

type desktopPreRunConfig struct {
	AppConfig                 appconfig.Config
	Prefs                     appconfig.UserPrefs
	Cancel                    context.CancelFunc
	BGScope                   *async.StructuredScope
	Adaptive                  *mygoshell.AdaptiveShell
	AcquireSingleInstanceLock bool
	OnReady                   func(*desktopRuntimeState, func(appconfig.UserPrefs))
}

func newDesktopAdaptiveShell(sessionID string) *mygoshell.AdaptiveShell {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	adaptive.Desktop.ActiveSessionID = sessionID
	adaptive.Desktop.Sessions = []mygoshell.SessionSummaryView{
		{SessionID: sessionID, Title: "Primary Workspace", TurnCount: 0},
	}
	return adaptive
}

// configureDesktopPreRun performs every pre-App.Run MyGo registration step.
// Keeping this isolated lets main_test.go execute the exact startup registration
// sequence on the OS main thread before mygo.App.Run so any accidental needsApp
// call fails go test immediately.
func configureDesktopPreRun(p desktopPreRunConfig) (*desktopRuntimeState, bool) {
	mygo.App.SetName("Marlin")
	mygo.App.SetVersion(appconfig.Version)
	if p.AcquireSingleInstanceLock && !mygo.App.RequestSingleInstanceLock() {
		return nil, false
	}

	rt := &desktopRuntimeState{prefs: p.Prefs}
	// Bind Tier-0 bidirectional Hybrid WebView RPC service (MyGo physically blocks
	// Tier-2 Untrusted pages from calling any bound Go methods).
	mygo.Bind(WorkflowBridge{
		onSave: func(g kit.WorkflowGraph) {
			if rt.emacsWin != nil {
				rt.emacsWin.Echo(fmt.Sprintf("Workflow synced (%d nodes, %d edges)", len(g.Nodes), len(g.Edges)))
			}
			if rt.win != nil {
				rt.win.Invalidate()
			}
		},
	})
	savePrefs := func(next appconfig.UserPrefs) {
		rt.prefsMu.Lock()
		rt.prefs = next
		snap := rt.prefs
		rt.prefsMu.Unlock()
		if err := appconfig.SavePrefs(p.AppConfig.PrefsFile, snap); err != nil {
			log.Printf("prefs save warning: %v", err)
		}
	}

	mygo.App.OnSecondInstance(func(_ []string, _ string) {
		mygo.App.Focus()
		if rt.win != nil {
			rt.win.Show()
			rt.win.Focus()
		}
	})
	mygo.App.OnOpenFile(func(path string) {
		if path == "" || p.Adaptive == nil || p.Adaptive.Desktop == nil || p.Adaptive.Desktop.Transcript == nil {
			return
		}
		_, _ = p.Adaptive.Desktop.Transcript.StageLocalFiles([]string{path})
		if rt.emacsWin != nil {
			rt.emacsWin.Echo("Attached " + path)
		}
		if rt.win != nil {
			rt.win.Invalidate()
		}
	})
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		if p.Cancel != nil {
			p.Cancel()
		}
		if p.BGScope != nil {
			p.BGScope.CloseAndDrain()
		}
		if rt.emacsWin != nil {
			rt.emacsWin.Close()
		}
		featurekit.ShutdownWebIslands()
		_ = appconfig.SavePrefs(p.AppConfig.PrefsFile, rt.snapshotPrefs())
		if rt.tray != nil {
			rt.tray.Destroy()
		}
		if rt.stopVoice != nil {
			rt.stopVoice()
		}
		if rt.link != nil {
			rt.link.SaveNow()
			rt.link.Close()
		}
	})
	mygo.App.WhenReady(func() {
		if p.OnReady != nil {
			p.OnReady(rt, savePrefs)
		}
	})
	return rt, true
}

// desktopVibrancy resolves the declarative TOML [surface].vibrancy material
// ("glass-clear", "glass", "hud", "popover", "under-window", "sidebar", "auto", "none")
// to native macOS 26 NSGlassEffectView Liquid Glass or NSVisualEffectView / Acrylic.
func desktopVibrancy(glassOn, isLight bool, mode string) mygo.Vibrancy {
	if !glassOn {
		return mygo.VibrancyNone
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return mygo.VibrancyNone
	case "glass-clear":
		return mygo.VibrancyGlassClear
	case "glass":
		return mygo.VibrancyGlass
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
			return mygo.VibrancyGlass
		}
		return mygo.VibrancyGlassClear
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
