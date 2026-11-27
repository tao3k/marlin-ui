// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

var (
	mobileNeedsAppCanaryMsg string
	mobilePreRunPanic       any
	mobilePreRunReady       bool
)

// TestMain runs on the process OS main thread before mygo.App.Run() so MyGo's
// needsApp() guard catches any illegal pre-Run API call in marlin-mobile.
func TestMain(m *testing.M) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				mobileNeedsAppCanaryMsg = fmt.Sprint(r)
			}
		}()
		_ = mygo.App.Dock.SetIcon(nil)
	}()

	tmpDir, err := os.MkdirTemp("", "marlin-mobile-prerun-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cfg := appconfig.DefaultConfig()
	cfg.Standalone = true
	cfg.SessionID = "test-mobile-prerun"
	cfg.PrefsFile = filepath.Join(tmpDir, "ui-prefs.json")

	func() {
		defer func() {
			if r := recover(); r != nil {
				mobilePreRunPanic = r
			}
		}()
		rt, savePrefs := configureMobilePreRun(mobilePreRunConfig{
			AppConfig: cfg,
			Prefs:     appconfig.UserPrefs{},
			OnReady:   func(*mobileRuntimeState, func()) {},
		})
		mobilePreRunReady = rt != nil && savePrefs != nil
	}()

	os.Exit(m.Run())
}

func TestMobileNeedsAppGuardActiveOnMainThread(t *testing.T) {
	if !strings.Contains(mobileNeedsAppCanaryMsg, "App.Dock.SetIcon called before App.Run") {
		t.Fatalf("expected main-thread MyGo needsApp canary panic, got %q", mobileNeedsAppCanaryMsg)
	}
}

func TestMobilePreRunBootstrapNoMyGoPanic(t *testing.T) {
	if mobilePreRunPanic != nil {
		t.Fatalf("configureMobilePreRun panicked before mygo.App.Run: %v", mobilePreRunPanic)
	}
	if !mobilePreRunReady {
		t.Fatalf("expected configureMobilePreRun to return initialized runtime state and saver")
	}
}

func TestMobilePlatformAndAdaptiveShellSetup(t *testing.T) {
	plat, ff, kind := resolveMobilePlatform("android")
	if plat != mygoshell.MobilePlatformAndroid || ff != mygoshell.FormFactorAndroid || kind != schema.TargetMobileAndroid {
		t.Fatalf("unexpected android resolution: (%v, %v, %v)", plat, ff, kind)
	}
	shell := newMobileAdaptiveShell("midnight", "Pixel 9", ff, plat)
	if shell == nil || shell.Mobile == nil {
		t.Fatalf("expected initialized mobile adaptive shell")
	}
}
