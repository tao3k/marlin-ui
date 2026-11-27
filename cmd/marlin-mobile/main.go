// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

// Command marlin-mobile launches Marlin's iOS (Apple HIG 393x852) and Android
// (Material 3 412x915) phone companion shell backed by github.com/tao3k/marlin-mygo
// and connected over TLS 1.3 / mTLS gRPC to Marlin Agent Core.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/mobile/android"
	"github.com/tao3k/marlin-ui/internal/mobile/ios"
)

func main() {
	cfg := appconfig.DefaultConfig()
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "print build version and exit")
	flag.StringVar(&cfg.ConfigFile, "config", cfg.ConfigFile, "Marlin model & host TOML config file (e.g. ./example.toml or ~/.config/marlin/config.toml)")
	flag.StringVar(&cfg.MobilePlatform, "platform", cfg.MobilePlatform, "mobile platform profile: ios | android")
	flag.StringVar(&cfg.MobileDevice, "device", cfg.MobileDevice, "optional viewport preset: iphone16 | iphone-se | pixel9")
	flag.StringVar(&cfg.ThemeID, "theme", cfg.ThemeID, "initial theme preset: midnight | paper | doom-one | tokyo-night | catppuccin")
	flag.StringVar(&cfg.FeaturesSpec, "features", cfg.FeaturesSpec, "opt-in UI feature profile: default | all | none | chatroom,compare,canvas")
	flag.StringVar(&cfg.HostAddr, "host", cfg.HostAddr, "remote Rust marlin-agent-core gRPC address (e.g. tailnet:50051)")
	flag.StringVar(&cfg.AppServerBin, "app-server", cfg.AppServerBin, "optional local Rust host binary for desktop simulator preview")
	flag.StringVar(&cfg.AuthToken, "token", cfg.AuthToken, "Bearer auth token for remote Rust host")
	flag.StringVar(&cfg.TLSCAFile, "ca", cfg.TLSCAFile, "optional TLS CA PEM file")
	flag.StringVar(&cfg.TLSCertFile, "cert", cfg.TLSCertFile, "optional mTLS client certificate PEM file")
	flag.StringVar(&cfg.TLSKeyFile, "key", cfg.TLSKeyFile, "optional mTLS client private key PEM file")
	flag.StringVar(&cfg.SessionID, "session", "marlin-mobile", "Marlin session ID")
	flag.StringVar(&cfg.RecoveryFile, "recovery-file", cfg.RecoveryFile, "atomic fsync crash-recovery snapshot file")
	flag.StringVar(&cfg.PrefsFile, "prefs-file", cfg.PrefsFile, "atomic local UI preferences JSON file")
	flag.BoolVar(&cfg.Standalone, "standalone", cfg.Standalone, "run standalone mobile preview without connecting to Rust host")
	flag.BoolVar(&cfg.RealSession, "real-session", cfg.RealSession, "enable durable ConversationJournal streaming on local Rust sidecar")
	flag.Parse()

	if showVersion {
		fmt.Println(appconfig.BuildSummary("marlin-mobile"))
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

	// On native iOS / Android devices, allow environment injection (e.g. from Xcode / simctl).
	if !explicitFlags["host"] && cfg.HostAddr == "" {
		if v := strings.TrimSpace(os.Getenv("MARLIN_HOST")); v != "" {
			cfg.HostAddr = v
		}
	}
	if !explicitFlags["token"] && cfg.AuthToken == "" {
		if v := strings.TrimSpace(os.Getenv("MARLIN_TOKEN")); v != "" {
			cfg.AuthToken = v
		}
	}
	if !explicitFlags["session"] {
		if v := strings.TrimSpace(os.Getenv("MARLIN_SESSION")); v != "" {
			cfg.SessionID = v
		}
	}
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		// Native mobile OS sandboxes never spawn a local macOS/Linux sidecar binary.
		cfg.AppServerBin = ""
		if cfg.HostAddr == "" {
			cfg.Standalone = true
		}
	}

	prefs, _ := appconfig.LoadPrefs(cfg.PrefsFile)
	themeDir := appconfig.DefaultUIThemeDir()
	themeBundle := appconfig.LoadUIThemeBundle(themeDir)
	if !explicitFlags["theme"] {
		if prefs.ThemeID != "" {
			cfg.ThemeID = prefs.ThemeID
		} else if themeBundle.Manifest.ActiveTheme != "" {
			cfg.ThemeID = themeBundle.Manifest.ActiveTheme
		}
	}

	mygo.App.SetName("Marlin Mobile")
	mygo.App.SetVersion(appconfig.Version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	plat := mygoshell.MobilePlatformIOS
	formFactor := mygoshell.FormFactorIOS
	targetKind := schema.TargetMobileIOS
	if strings.EqualFold(strings.TrimSpace(cfg.MobilePlatform), "android") || runtime.GOOS == "android" {
		plat = mygoshell.MobilePlatformAndroid
		formFactor = mygoshell.FormFactorAndroid
		targetKind = schema.TargetMobileAndroid
	}
	spec := mygoshell.BuiltinMobileDeviceSpec(plat, cfg.MobileDevice)

	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		BrandSubtitle:  spec.Name,
		InitialThemeID: cfg.ThemeID,
		FormFactor:     formFactor,
		MobilePlatform: plat,
	})
	adaptive.Mobile.SetPacks([]mygoshell.MobileDeclarativePack{
		{
			PluginID: "tao3k.daily-planner-pack",
			Version:  "0.1.0",
			Summary:  "Declarative theme + keymap overlay (App Store 2.5.2 compliant)",
			Runtime:  schema.RuntimeDeclarative,
			Active:   true,
		},
	})

	var (
		prefsMu sync.Mutex
		link    *hostlink.Link
	)
	savePrefs := func() {
		prefsMu.Lock()
		snap := prefs
		prefsMu.Unlock()
		_ = appconfig.SavePrefs(cfg.PrefsFile, snap)
	}
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		savePrefs()
		if link != nil {
			link.SaveNow()
			link.Close()
		}
	})

	mygo.App.WhenReady(func() {
		var (
			win     *mygo.Window
			tabWins []*mygo.Window
			tabBar  *mygo.Tabs
		)
		invalidate := func() {
			if win != nil {
				win.Invalidate()
			}
			for _, w := range tabWins {
				if w != nil {
					w.Invalidate()
				}
			}
		}

		var applyFeatures func(string)
		authCtrl := hostlink.AttachAuthAndSettings(ctx, adaptive, hostlink.AuthSettingsConfig{
			Config: cfg,
			Prefs:  prefs,
			OnApplyFeatures: func(spec string) {
				if applyFeatures != nil {
					applyFeatures(spec)
				}
			},
			OnSavePrefs: func(vals map[string]any) {
				prefsMu.Lock()
				appconfig.ApplySettingsValues(&prefs, vals)
				prefsMu.Unlock()
				savePrefs()
			},
			Invalidate: invalidate,
		})

		var err error
		link, err = hostlink.Connect(
			ctx,
			cfg,
			adaptive,
			targetKind,
			invalidate,
			hostlink.WithAuthTokenSource(hostlink.OptionalAuthTokenSource(authCtrl)),
			hostlink.WithSavedRemotes(prefs.SavedRemotes),
			hostlink.WithOnRemoteAdded(func(r appconfig.SavedRemote) {
				prefsMu.Lock()
				prefs.UpsertRemote(r)
				prefsMu.Unlock()
				savePrefs()
			}),
		)
		if err != nil {
			log.Fatalf("marlin-mobile host connection failed: %v", err)
		}
		reg, ledger, host := featurekit.Mount(cfg.FeaturesSpec, adaptive, link.OnDraftChanged, invalidate)
		applyFeatures = func(spec string) {
			reg.Activate(features.ParseFeatureSpec(spec), ledger, host)
			adaptive.SetFeatureHost(host, adaptive.Desktop.GovernanceUI)
		}
		adaptive.Mobile.TargetLabel = fmt.Sprintf("%s · %s", spec.Name, link.Endpoint)

		// On iOS (both native GOOS=ios Swift UIKit host and macOS Apple HIG dev preview),
		// mount Marlin's flagship Desktop-parity Liquid Glass iOS 26 shell under internal/mobile/ios.
		if plat == mygoshell.MobilePlatformIOS {
			isNativeIOS := runtime.GOOS == "ios"
			iosShell := ios.New(ios.Config{
				Adaptive:       adaptive,
				Link:           link,
				AppConfig:      cfg,
				Prefs:          prefs,
				ThemeBundle:    themeBundle,
				InitialThemeID: cfg.ThemeID,
				NativeTabBar:   isNativeIOS,
				OnPrefsChanged: func(next appconfig.UserPrefs) {
					prefsMu.Lock()
					prefs = next
					prefsMu.Unlock()
					savePrefs()
				},
				OnBadgeChanged: func(tabIdx int, badge string) {
					if tabBar != nil {
						tabBar.SetBadge(tabIdx, badge)
					}
				},
				Invalidate: invalidate,
			})

			if isNativeIOS {
				// Native Swift UITabBarController + SF Symbols (automatic iOS 26 Liquid Glass tab bar!).
				items := make([]mygo.TabItem, 0, len(mobile.Tabs))
				for _, ts := range mobile.Tabs {
					tw := mygo.NewWindow(mygo.WindowOptions{
						Title:   ts.Title,
						Hidden:  true,
						Content: ui.View(iosShell.TabView(ts.Tab)),
					})
					tabWins = append(tabWins, tw)
					items = append(items, mygo.TabItem{
						Title:  ts.Title,
						Symbol: ts.Symbol,
						Window: tw,
					})
				}
				if tb, tbErr := mygo.NewTabs(items...); tbErr == nil {
					tabBar = tb
					tb.OnSelect(func(idx int) {
						if idx >= 0 && idx < len(mobile.Tabs) {
							iosShell.SelectTab(mobile.Tabs[idx].Tab)
						}
					})
					invalidate()
					return
				}
			}

			// macOS Apple HIG iPhone 16 Pro dev window with native macOS 26 NSGlassEffectView
			// regular frosted Liquid Glass (blurs busy background terminals cleanly).
			win = mygo.NewWindow(mygo.WindowOptions{
				Title:         fmt.Sprintf("Marlin iOS — %s", spec.Name),
				Width:         spec.LogicalWidth,
				Height:        spec.LogicalHeight,
				MinWidth:      spec.LogicalWidth,
				MinHeight:     spec.LogicalHeight,
				MaxWidth:      spec.LogicalWidth,
				MaxHeight:     spec.LogicalHeight,
				TitleBarStyle: mygo.TitleBarHiddenInset,
				Transparent:   true,
				Vibrancy:      mygo.VibrancyGlass,
				Content:       ui.View(iosShell.View),
			})
			win.SetActiveAppearance(true)
			win.Invalidate()
			return
		}

		// Android Material 3 companion shell under internal/mobile/android.
		androidShell := android.New(android.Config{
			Adaptive:       adaptive,
			Link:           link,
			AppConfig:      cfg,
			Prefs:          prefs,
			ThemeBundle:    themeBundle,
			InitialThemeID: cfg.ThemeID,
			OnPrefsChanged: func(next appconfig.UserPrefs) {
				prefsMu.Lock()
				prefs = next
				prefsMu.Unlock()
				savePrefs()
			},
			Invalidate: invalidate,
		})
		winOpts := mygo.WindowOptions{
			Title:   fmt.Sprintf("Marlin Android — %s", spec.Name),
			Width:   spec.LogicalWidth,
			Height:  spec.LogicalHeight,
			Content: ui.View(androidShell.View),
		}
		if runtime.GOOS != "ios" && runtime.GOOS != "android" {
			winOpts.MinWidth = spec.LogicalWidth
			winOpts.MinHeight = spec.LogicalHeight
			winOpts.MaxWidth = spec.LogicalWidth
			winOpts.MaxHeight = spec.LogicalHeight
		}
		win = mygo.NewWindow(winOpts)
		win.Invalidate()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	savePrefs()
	if link != nil {
		link.Close()
	}
}
