// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

var (
	desktopNeedsAppCanaryMsg string
	desktopPreRunPanic       any
	desktopPreRunAcquired    bool
)

// TestMain runs on the process OS main thread before mygo.App.Run() is ever
// invoked. MyGo's needsApp() guard specifically checks isMainThread() &&
// !App.initialized, so executing configureDesktopPreRun here faithfully
// reproduces real marlin-desktop startup and catches any pre-Run panic.
func TestMain(m *testing.M) {
	// Canary: prove MyGo's main-thread needsApp guard is active in this test
	// binary so the bootstrap test below cannot silently false-pass.
	func() {
		defer func() {
			if r := recover(); r != nil {
				desktopNeedsAppCanaryMsg = fmt.Sprint(r)
			}
		}()
		_ = mygo.App.Dock.SetIcon(nil)
	}()

	tmpDir, err := os.MkdirTemp("", "marlin-desktop-prerun-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cfg := appconfig.DefaultConfig()
	cfg.Standalone = true
	cfg.SessionID = "test-desktop-prerun"
	cfg.PrefsFile = filepath.Join(tmpDir, "ui-prefs.json")
	cfg.RecoveryFile = filepath.Join(tmpDir, "recovery.json")

	ctx, cancel := context.WithCancel(context.Background())
	bgScope := async.NewStructuredScope(ctx)
	adaptive := newDesktopAdaptiveShell(cfg.SessionID)

	func() {
		defer func() {
			if r := recover(); r != nil {
				desktopPreRunPanic = r
			}
		}()
		_, desktopPreRunAcquired = configureDesktopPreRun(desktopPreRunConfig{
			AppConfig:                 cfg,
			Prefs:                     appconfig.UserPrefs{},
			Cancel:                    cancel,
			BGScope:                   bgScope,
			Adaptive:                  adaptive,
			AcquireSingleInstanceLock: false,
			OnReady:                   func(*desktopRuntimeState, func(appconfig.UserPrefs)) {},
		})
	}()

	cancel()
	bgScope.CloseAndDrain()
	os.Exit(m.Run())
}

func TestDesktopNeedsAppGuardActiveOnMainThread(t *testing.T) {
	if !strings.Contains(desktopNeedsAppCanaryMsg, "App.Dock.SetIcon called before App.Run") {
		t.Fatalf("expected main-thread MyGo needsApp canary panic, got %q", desktopNeedsAppCanaryMsg)
	}
}

func TestDesktopPreRunBootstrapNoMyGoPanic(t *testing.T) {
	if desktopPreRunPanic != nil {
		t.Fatalf("configureDesktopPreRun panicked before mygo.App.Run: %v", desktopPreRunPanic)
	}
	if !desktopPreRunAcquired {
		t.Fatalf("expected configureDesktopPreRun to succeed when single-instance lock is disabled")
	}
}

func TestDesktopAppAndTrayIconsValidPNG(t *testing.T) {
	appPNG := theme.MarlinAppIconPNG()
	if len(appPNG) == 0 {
		t.Fatalf("expected non-empty desktop app icon PNG")
	}
	appImg, err := png.Decode(bytes.NewReader(appPNG))
	if err != nil {
		t.Fatalf("decode desktop app icon PNG: %v", err)
	}
	if b := appImg.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Fatalf("expected 256x256 desktop app icon, got %dx%d", b.Dx(), b.Dy())
	}
	// Outer corner must be transparent due to the Apple HIG squircle mask.
	if _, _, _, a := appImg.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("expected transparent outer squircle corner at (0,0), got alpha=%d", a)
	}

	trayPNG := theme.MarlinTrayIconPNG()
	if len(trayPNG) == 0 {
		t.Fatalf("expected non-empty tray icon PNG")
	}
	trayImg, err := png.Decode(bytes.NewReader(trayPNG))
	if err != nil {
		t.Fatalf("decode tray icon PNG: %v", err)
	}
	if b := trayImg.Bounds(); b.Dx() != 44 || b.Dy() != 44 {
		t.Fatalf("expected 44x44 Retina tray icon, got %dx%d", b.Dx(), b.Dy())
	}

	sheetPNG := theme.MarlinSpriteSheetPNG()
	if len(sheetPNG) == 0 {
		t.Fatalf("expected non-empty 4-frame Marlin animation sprite sheet PNG")
	}
	sheetImg, err := png.Decode(bytes.NewReader(sheetPNG))
	if err != nil {
		t.Fatalf("decode Marlin sprite sheet PNG: %v", err)
	}
	sb := sheetImg.Bounds()
	if sb.Dx() != 720 || sb.Dy() != 480 {
		t.Fatalf("expected 720x480 2x2 Retina Marlin sprite atlas, got %dx%d", sb.Dx(), sb.Dy())
	}
	if _, _, _, a := sheetImg.At(sb.Min.X, sb.Min.Y).RGBA(); a != 0 {
		t.Fatalf("expected transparent background corner on Marlin sprite sheet, got alpha=%d", a)
	}
	// Verify all 4 animation cells contain non-transparent artwork.
	for q := 0; q < 4; q++ {
		qx := (q % 2) * 360
		qy := (q / 2) * 240
		opaque := 0
		for y := qy; y < qy+240; y += 4 {
			for x := qx; x < qx+360; x += 4 {
				if _, _, _, a := sheetImg.At(x, y).RGBA(); a > 0x8000 {
					opaque++
				}
			}
		}
		if opaque < 100 {
			t.Fatalf("expected animated Marlin artwork in quadrant %d, found only %d sampled opaque pixels", q, opaque)
		}
	}
}

func TestDesktopBuildMenuAndVibrancy(t *testing.T) {
	adaptive := newDesktopAdaptiveShell("test-menu")
	fr := frame.New(frame.Config{
		Adaptive:  adaptive,
		AppConfig: appconfig.Config{SessionID: "test-menu"},
	})
	defer fr.Close()

	menu := buildDesktopMenu(fr, func() {})
	if menu == nil {
		t.Fatalf("expected non-nil desktop menu")
	}

	cases := []struct {
		glassOn bool
		isLight bool
		mode    string
		want    mygo.Vibrancy
	}{
		{false, false, "auto", mygo.VibrancyNone},
		{true, false, "none", mygo.VibrancyNone},
		{true, false, "auto", mygo.VibrancyGlassClear},
		{true, true, "auto", mygo.VibrancyGlass},
		{true, false, "hud", mygo.VibrancyHUD},
		{true, false, "sidebar", mygo.VibrancySidebar},
	}
	for _, tc := range cases {
		if got := desktopVibrancy(tc.glassOn, tc.isLight, tc.mode); got != tc.want {
			t.Errorf("desktopVibrancy(%v, %v, %q) = %v, want %v", tc.glassOn, tc.isLight, tc.mode, got, tc.want)
		}
	}
}
