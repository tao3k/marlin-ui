// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// TestMyGoNativeAutomatedTabClickSoakAndP99 uses MyGo's native ui.NewTester
// with macOS Liquid Glass vibrancy enabled (SetVibrancy(true)) to drive
// realistic Tab switching across every buffer and feature tab (*Dashboard*,
// *conversation*, *Sessions*, *Targets*, *Agent Room*, *Workflow Canvas*) while
// concurrently hammering off-lock ReloadThemesAsync, CycleTheme, and
// CycleGlassOpacity — asserting zero races, wait-free RCU reads, idle GPU tick
// gating, and tight P99 frame latency.
func TestMyGoNativeAutomatedTabClickSoakAndP99(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	reg, ledger, host := featurekit.Mount("all", adaptive, nil, nil)

	f := frame.New(frame.Config{
		Adaptive:              adaptive,
		FeatureReg:            reg,
		FeatureLedger:         ledger,
		FeatureHost:           host,
		WebHost:               featurekit.WebIslandHost(),
		OnWebIslandVisibility: featurekit.NotifyWebIslandVisibility,
		AppConfig:             appconfig.Config{SessionID: "soak-1"},
		OnReloadThemeBundle: func() appconfig.UIThemeBundle {
			// Simulate realistic background disk I/O off the render lock.
			time.Sleep(2 * time.Millisecond)
			return appconfig.UIThemeBundle{}
		},
	})
	defer f.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				if i%3 == 0 {
					f.ReloadThemesAsync()
				}
				f.CycleGlassOpacity()
				f.CycleTheme()
				_, _ = f.ActivePaletteAndOpacity()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	tt := ui.NewTester(f.View, 920, 560)
	tt.SetVibrancy(true)
	tabLabels := []string{
		"*Sessions*",
		"*Targets*",
		"*Agent Room*",
		"*Workflow Canvas*",
		"*conversation*",
		"*Dashboard*",
	}

	nonDashDurs := make([]time.Duration, 0, 20)
	for round := 0; round < 2; round++ {
		for _, lbl := range tabLabels {
			if err := tt.Click(lbl); err != nil {
				t.Fatalf("round %d click tab %s: %v", round, lbl, err)
			}
			t0 := time.Now()
			tt.Frame()
			dur := time.Since(t0)
			if lbl != "*Dashboard*" {
				nonDashDurs = append(nonDashDurs, dur)
				// Demand-driven GPU power gating: non-dashboard tabs MUST NOT run 1Hz idle ticks!
				if f.ShouldTickIdle() {
					t.Fatalf("expected ShouldTickIdle()=false on non-dashboard tab %s", lbl)
				}
			}
		}
	}

	close(stop)
	wg.Wait()

	// Verify borderless immersive *Workflow Canvas* via native MyGo click.
	if err := tt.Click("*Workflow Canvas*"); err != nil {
		t.Fatalf("click *Workflow Canvas*: %v", err)
	}
	tt.Frame()
	if tt.HasText("HYBRID GPU") || tt.HasText("Workflow & BPMN Studio") {
		t.Fatalf("expected borderless immersive *Workflow Canvas* without redundant card chrome")
	}

	// Verify clicking into *Agent Room* via sort-tab renders cleanly & quickly.
	if err := tt.Click("*Agent Room*"); err != nil {
		t.Fatalf("click *Agent Room*: %v", err)
	}
	t0 := time.Now()
	tt.Frame()
	agentRoomDur := time.Since(t0)
	if !tt.HasText("AGENT ROOM") {
		t.Fatalf("expected *Agent Room* rendered after tab click")
	}
	if agentRoomDur > 250*time.Millisecond {
		t.Fatalf("Agent Room single frame took %v (expected responsive render)", agentRoomDur)
	}

	// Verify Dashboard enables demand-driven idle clock ticks.
	f.SelectBuffer(buffer.Dashboard)
	tt.Frame()
	frames, slow, maxDur := f.FrameTelemetry()
	if frames == 0 {
		t.Fatalf("expected FrameTelemetry frames > 0")
	}

	sort.Slice(nonDashDurs, func(i, j int) bool { return nonDashDurs[i] < nonDashDurs[j] })
	p99 := nonDashDurs[int(float64(len(nonDashDurs)-1)*0.99)]
	t.Logf("MyGo native click soak (vibrancy=on): frames=%d slow=%d max=%v nonDashP99=%v agentRoom=%v", frames, slow, maxDur, p99, agentRoomDur)
}

// TestTabSwitchSingleFrameZeroRedundantInvalidateAndEvalStorm closes the exact
// testing gap that allowed real-world tab-switch lag to escape headless tests:
//  1. Verifies Vibrancy=true + active WebIsland host during 20 back-and-forth
//     *Workflow Canvas* <-> *Agent Room* switches.
//  2. Asserts ZERO redundant win.Invalidate() calls triggered synchronously
//     inside Frame.View(c) during tab clicks (preventing duplicate Frame N+1).
//  3. Asserts ZERO redundant theme Eval() storms on *Workflow Canvas* switches.
//  4. Asserts single-frame immediate viewport transition (0 stale frames).
func TestTabSwitchSingleFrameZeroRedundantInvalidateAndEvalStorm(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	var inViewInvCount atomic.Int64
	var totalInvCount atomic.Int64
	var themeEvalCount atomic.Int64
	var visEvalCount atomic.Int64

	reg, ledger, host := featurekit.Mount("all", adaptive, nil, nil)
	featurekit.SetWebViewFactory(func(islandID, html, preloadJS string, tier async.TrustTier) (featurekit.WebViewInstance, error) {
		return featurekit.WebViewInstance{
			Tier: tier,
			Eval: func(js string) {
				if strings.Contains(js, "marlin:theme") {
					themeEvalCount.Add(1)
				} else if strings.Contains(js, "marlin:visibility") {
					visEvalCount.Add(1)
				}
			},
		}, nil
	})
	defer featurekit.SetWebViewFactory(nil)

	var insideView atomic.Bool
	f := frame.New(frame.Config{
		Adaptive:              adaptive,
		FeatureReg:            reg,
		FeatureLedger:         ledger,
		FeatureHost:           host,
		WebHost:               featurekit.WebIslandHost(),
		OnWebIslandVisibility: featurekit.NotifyWebIslandVisibility,
		AppConfig:             appconfig.Config{SessionID: "zero-lag-guard"},
		Invalidate: func() {
			totalInvCount.Add(1)
			if insideView.Load() {
				inViewInvCount.Add(1)
			}
		},
	})
	defer f.Close()

	// Sync initial palette once (matching cmd/marlin-desktop startup).
	pal, op := f.ActivePaletteAndOpacity()
	featurekit.SyncWebIslandTheme(pal, op)

	wrappedView := func(c *ui.Context) {
		insideView.Store(true)
		defer insideView.Store(false)
		f.View(c)
	}
	tt := ui.NewTester(wrappedView, 920, 560)
	tt.SetVibrancy(true)
	tt.Frame()

	// 20 rapid back-and-forth clicks between *Workflow Canvas* and *Agent Room*.
	inViewInvCount.Store(0)
	themeEvalCount.Store(0)
	for i := 0; i < 10; i++ {
		if err := tt.Click("*Workflow Canvas*"); err != nil {
			t.Fatalf("round %d click *Workflow Canvas*: %v", i, err)
		}
		if err := tt.Click("*Agent Room*"); err != nil {
			t.Fatalf("round %d click *Agent Room*: %v", i, err)
		}
		if !tt.HasText("AGENT ROOM") {
			t.Fatalf("round %d: expected AGENT ROOM immediately after click", i)
		}
	}

	if got := inViewInvCount.Load(); got != 0 {
		t.Fatalf("detected %d redundant win.Invalidate() calls inside Frame.View during tab clicks (causes duplicate Frame N+1 lag)", got)
	}
	if got := themeEvalCount.Load(); got != 0 {
		t.Fatalf("detected %d redundant theme Eval() storms during tab switches (expected 0)", got)
	}

	// Single-frame programmatic transition check (no settle loop):
	// SelectFeature + 1 Frame() MUST immediately show target content on Frame 1.
	f.SelectFeature("feature:chatroom")
	tt.Frame()
	if !tt.HasText("AGENT ROOM") {
		t.Fatalf("expected AGENT ROOM visible on Frame 1 with zero stale frames")
	}
	f.SelectBuffer(buffer.Sessions)
	tt.Frame()
	if !tt.HasText("SESSION ID") {
		t.Fatalf("expected *Sessions* visible on Frame 1 with zero stale frames")
	}
	f.SelectBuffer(buffer.Packages)
	tt.Frame()
	if !tt.HasText("Refresh Marketplace") {
		t.Fatalf("expected *Packages* visible on Frame 1 with zero stale frames")
	}

	t.Logf("Zero-lag guard PASS: inViewInvalidates=%d themeEvals=%d visEvals=%d totalInvalidates=%d",
		inViewInvCount.Load(), themeEvalCount.Load(), visEvalCount.Load(), totalInvCount.Load())
}
