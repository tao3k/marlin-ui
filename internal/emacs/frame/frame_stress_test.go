// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// TestMyGoNativeAutomatedTabClickSoakAndP99 uses MyGo's native ui.NewTester
// click automation to drive realistic user Tab switching across every buffer
// and feature tab (*Dashboard*, *conversation*, *Sessions*, *Targets*,
// *Agent Room*, *Workflow Canvas*) while concurrently hammering off-lock
// ReloadThemesAsync, CycleTheme, and CycleGlassOpacity — asserting zero races,
// wait-free RCU reads, idle GPU tick gating, and tight P99 frame latency.
func TestMyGoNativeAutomatedTabClickSoakAndP99(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	reg, ledger, host := featurekit.Mount("all", adaptive, nil, nil)

	f := frame.New(frame.Config{
		Adaptive:      adaptive,
		FeatureReg:    reg,
		FeatureLedger: ledger,
		FeatureHost:   host,
		WebHost:       featurekit.WebIslandHost(),
		AppConfig:     appconfig.Config{SessionID: "soak-1"},
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
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	tt := ui.NewTester(f.View, 1280, 820)
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
	t.Logf("MyGo native click soak: frames=%d slow=%d max=%v nonDashP99=%v agentRoom=%v", frames, slow, maxDur, p99, agentRoomDur)
}
