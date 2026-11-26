// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

// Command marlin-desktop is the native 120Hz pure-Go desktop client for Marlin
// Agent Core (macOS, Linux, Windows), built on github.com/tao3k/marlin-mygo.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
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
	flag.StringVar(&cfg.AppServerBin, "app-server", cfg.AppServerBin, "local Rust marlin-app-server binary (auto-discovered if omitted)")
	flag.StringVar(&cfg.HostAddr, "host", cfg.HostAddr, "remote Rust marlin-agent-core gRPC address (e.g. 127.0.0.1:50051)")
	flag.StringVar(&cfg.AuthToken, "token", cfg.AuthToken, "Bearer auth token for remote Rust host")
	flag.StringVar(&cfg.TLSCAFile, "ca", cfg.TLSCAFile, "optional TLS CA PEM file")
	flag.StringVar(&cfg.TLSCertFile, "cert", cfg.TLSCertFile, "optional mTLS client certificate PEM file")
	flag.StringVar(&cfg.TLSKeyFile, "key", cfg.TLSKeyFile, "optional mTLS client private key PEM file")
	flag.StringVar(&cfg.SessionID, "session", cfg.SessionID, "Marlin session ID")
	flag.StringVar(&cfg.ThemeID, "theme", cfg.ThemeID, "initial theme preset: midnight | paper | doom-one | tokyo-night | catppuccin")
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

	prefs, err := appconfig.LoadPrefs(cfg.PrefsFile)
	if err != nil {
		log.Printf("prefs load warning (%s): %v", cfg.PrefsFile, err)
	}
	if !explicitFlags["theme"] && prefs.ThemeID != "" {
		cfg.ThemeID = prefs.ThemeID
	}
	if !explicitFlags["features"] && prefs.FeaturesSpec != "" {
		cfg.FeaturesSpec = prefs.FeaturesSpec
	}
	if !explicitFlags["session"] && prefs.LastSessionID != "" {
		cfg.SessionID = prefs.LastSessionID
	}

	mygo.App.SetName("Marlin")
	mygo.App.SetVersion(appconfig.Version)
	if !mygo.App.RequestSingleInstanceLock() {
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		BrandSubtitle:  "Desktop · 120Hz Native GPU",
		InitialThemeID: cfg.ThemeID,
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	adaptive.Desktop.SidebarCollapsed = prefs.SidebarCollapsed
	adaptive.Desktop.ActiveSessionID = cfg.SessionID
	adaptive.Desktop.Sessions = []mygoshell.SessionSummaryView{
		{SessionID: cfg.SessionID, Title: "Primary Workspace", TurnCount: 0},
	}

	var (
		prefsMu   sync.Mutex
		win       *mygo.Window
		link      *hostlink.Link
		stopVoice func()
	)
	savePrefs := func() {
		prefsMu.Lock()
		if adaptive.Desktop != nil {
			prefs.SidebarCollapsed = adaptive.Desktop.IsSidebarCollapsed()
			if adaptive.Desktop.ActiveSessionID != "" {
				prefs.LastSessionID = adaptive.Desktop.ActiveSessionID
			}
			if adaptive.Desktop.ActiveThemeNameFn != nil {
				for _, tp := range mygoshell.BuiltinThemes() {
					if tp.Name == adaptive.Desktop.ActiveThemeNameFn() {
						prefs.ThemeID = tp.ID
						break
					}
				}
			}
		}
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
		if win != nil {
			win.Invalidate()
		}
	})
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		savePrefs()
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

		var connErr error
		link, connErr = hostlink.Connect(
			ctx,
			cfg,
			adaptive,
			desktopTargetKind(),
			invalidate,
			hostlink.WithDesktopMyGoAdapters(),
			hostlink.WithOnSessionChanged(func(_ string) { savePrefs() }),
			hostlink.WithOnRemoteAdded(func(r appconfig.SavedRemote) {
				prefsMu.Lock()
				prefs.UpsertRemote(r)
				prefsMu.Unlock()
				savePrefs()
			}),
		)
		if connErr != nil {
			log.Fatalf("marlin-desktop host connection failed: %v", connErr)
		}
		featurekit.Mount(cfg.FeaturesSpec, adaptive, link.OnDraftChanged, invalidate)

		// Persist theme cycles triggered from the in-app chrome as well as OS menu.
		prevCycle := adaptive.Desktop.OnCycleTheme
		adaptive.Desktop.OnCycleTheme = func() {
			if prevCycle != nil {
				prevCycle()
			}
			savePrefs()
			invalidate()
		}
		prevToggle := adaptive.Desktop.OnToggleTheme
		adaptive.Desktop.OnToggleTheme = func() {
			if prevToggle != nil {
				prevToggle()
			}
			savePrefs()
			invalidate()
		}

		tr := adaptive.Desktop.Transcript
		tr.OnPickAttachment = func() {
			go func() {
				paths, err := composer.PickFilesNative(ctx, composer.NativePickerOptions{
					Title:         "Attach files to Marlin",
					AllowMultiple: true,
				})
				if err == nil && len(paths) > 0 {
					_, _ = tr.StageLocalFiles(paths)
					invalidate()
				}
			}()
		}
		tr.OnPasteAttachment = func() {
			go func() {
				if in, ok, err := composer.ReadClipboardAttachment(ctx); err == nil && ok {
					_, _ = tr.AttachFile(in)
					invalidate()
				}
			}()
		}
		stopVoice = wireDesktopVoice(ctx, tr, invalidate)

		mygo.App.SetMenu(buildDesktopMenu(adaptive, savePrefs, invalidate))

		title := fmt.Sprintf("Marlin — %s (%s)", link.Mode, link.Endpoint)
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:    title,
			Width:    1360,
			Height:   880,
			StateKey: "marlin-desktop-main",
			Content:  ui.View(adaptive.View),
		})
		win.Invalidate()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	savePrefs()
	if stopVoice != nil {
		stopVoice()
	}
	if link != nil {
		link.Close()
	}
}

func wireDesktopVoice(ctx context.Context, tr *mygoshell.TranscriptController, invalidate func()) func() {
	rec := composer.NewNativeAudioRecorder()
	tr.OnStartVoice = func() {
		go func() {
			if err := rec.Start(ctx, tr.Voice, invalidate); err != nil {
				if tr.Voice != nil {
					tr.Voice.Fail(err)
				}
				invalidate()
			}
		}()
	}
	tr.OnStopVoice = func() {
		go func() {
			if name, wav, err := rec.Stop(); err == nil && len(wav) > 0 {
				_, _ = tr.CompleteVoiceAudio(name, wav)
			} else if err != nil && tr.Voice != nil {
				tr.Voice.Fail(err)
			}
			invalidate()
		}()
	}
	tr.OnCancelVoice = func() {
		rec.Cancel()
		invalidate()
	}
	return func() {
		rec.Cancel()
	}
}
