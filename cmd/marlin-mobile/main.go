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
)

func main() {
	cfg := appconfig.DefaultConfig()
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "print build version and exit")
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
	flag.Parse()

	if showVersion {
		fmt.Println(appconfig.BuildSummary("marlin-mobile"))
		return
	}

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })
	prefs, _ := appconfig.LoadPrefs(cfg.PrefsFile)
	if !explicitFlags["theme"] && prefs.ThemeID != "" {
		cfg.ThemeID = prefs.ThemeID
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
		var win *mygo.Window
		invalidate := func() {
			if win != nil {
				win.Invalidate()
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

		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     fmt.Sprintf("Marlin Mobile — %s", spec.Name),
			Width:     spec.LogicalWidth,
			Height:    spec.LogicalHeight,
			MinWidth:  spec.LogicalWidth,
			MinHeight: spec.LogicalHeight,
			MaxWidth:  spec.LogicalWidth,
			MaxHeight: spec.LogicalHeight,
			Content:   ui.View(adaptive.View),
		})
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
