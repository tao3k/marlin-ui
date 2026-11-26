// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js && !marlin_minimal

package e2e

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

func TestMain(m *testing.M) {
	if os.Getenv("MARLIN_E2E") == "" {
		fmt.Println("skipping real desktop E2E latency tests; set MARLIN_E2E=1 to run with live Metal/AppKit/WebView")
		os.Exit(0)
	}
	mygo.App.OnWindowAllClosed(func() {})
	code := 1
	mygo.App.WhenReady(func() {
		go func() {
			code = m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// TestRealDesktopTabSwitchLatency measures real macOS Metal + Vibrancy +
// WKWebView tab-switching wall-clock latency across *Workflow Canvas*,
// *Agent Room*, *Sessions*, *Targets*, *Packages*, and *Dashboard*, asserting
// strict latency budgets and zero theme Eval storms after background pre-warm.
func TestRealDesktopTabSwitchLatency(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	var win *mygo.Window
	var invCalls atomic.Int64
	invalidate := func() {
		invCalls.Add(1)
		if win != nil {
			win.Invalidate()
		}
	}
	reg, ledger, host := featurekit.Mount("all", adaptive, nil, invalidate)

	var frameCount atomic.Int64
	var themeEvalCount atomic.Int64
	var visEvalCount atomic.Int64
	var webViewCreates atomic.Int64
	webViewReady := make(chan struct{}, 2)
	frameDone := make(chan int64, 64)

	emacsWin := frame.New(frame.Config{
		Adaptive:              adaptive,
		AppConfig:             appconfig.Config{SessionID: "e2e-latency"},
		FeatureReg:            reg,
		FeatureLedger:         ledger,
		FeatureHost:           host,
		WebHost:               featurekit.WebIslandHost(),
		OnWebIslandAction:     featurekit.DispatchWebIslandAction,
		OnWebIslandVisibility: featurekit.NotifyWebIslandVisibility,
		Invalidate:            invalidate,
	})
	defer emacsWin.Close()

	initPal, initOp := emacsWin.ActivePaletteAndOpacity()
	featurekit.SyncWebIslandTheme(initPal, initOp)

	wrappedView := func(c *ui.Context) {
		emacsWin.View(c)
		n := frameCount.Add(1)
		select {
		case frameDone <- n:
		default:
		}
	}

	win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "Marlin E2E Latency Probe",
		Width:         1280,
		Height:        820,
		TitleBarStyle: mygo.TitleBarHiddenInset,
		Transparent:   true,
		Vibrancy:      mygo.VibrancyHUD,
		Content:       ui.View(wrappedView),
	})
	defer win.Destroy()
	win.SetActiveAppearance(true)

	featurekit.SetWebViewInvalidate(invalidate)
	featurekit.SetWebViewFactory(func(islandID, html, preloadJS string, tier async.TrustTier) (featurekit.WebViewInstance, error) {
		webViewCreates.Add(1)
		tCreate := time.Now()
		wv, err := win.NewWebView(mygo.WebViewOptions{
			Transparent: true,
			Page: mygo.PageOptions{
				PreloadScript: preloadJS,
				Untrusted:     tier != async.TrustTier0FirstParty,
			},
		})
		if err != nil {
			return featurekit.WebViewInstance{}, err
		}
		wv.Page().LoadHTML(html, "")
		t.Logf("e2e: Prewarmed NewWebView(%s) in %v", islandID, time.Since(tCreate))
		select {
		case webViewReady <- struct{}{}:
		default:
		}
		return featurekit.WebViewInstance{
			View: wv,
			Tier: tier,
			Eval: func(js string) {
				if strings.Contains(js, "marlin:theme") {
					themeEvalCount.Add(1)
				} else if strings.Contains(js, "marlin:visibility") {
					visEvalCount.Add(1)
				}
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
	defer featurekit.ShutdownWebIslands()

	drainFrames := func() {
		for {
			select {
			case <-frameDone:
			default:
				return
			}
		}
	}
	waitNextFrame := func(label string) time.Duration {
		t0 := time.Now()
		select {
		case <-frameDone:
			return time.Since(t0)
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for frame after %s (invCalls=%d frameCount=%d)", label, invCalls.Load(), frameCount.Load())
			return 0
		}
	}

	// Wait for initial dashboard paint, then run desktop startup PrewarmWebIsland.
	win.Invalidate()
	firstDur := waitNextFrame("initial-dashboard")
	t.Logf("e2e: initial dashboard frame ready in %v", firstDur)
	featurekit.PrewarmWebIsland(featurekit.BuiltinWorkflowStudioIslandID)
	select {
	case <-webViewReady:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for background WebView pre-warm")
	}
	// Let startup Dashboard spring settle & pre-warmed WebView initial HTML load finish.
	time.Sleep(350 * time.Millisecond)

	// Measure realistic user tab-switching loop with strict real-hardware budgets.
	type step struct {
		name      string
		fn        func()
		maxBudget time.Duration
	}
	steps := []step{
		{"Workflow Canvas (prewarmed)", func() { emacsWin.SelectFeature("feature:webisland") }, 25 * time.Millisecond},
		{"Agent Room (cold)", func() { emacsWin.SelectFeature("feature:chatroom") }, 25 * time.Millisecond},
		{"Workflow Canvas (warm 1)", func() { emacsWin.SelectFeature("feature:webisland") }, 16 * time.Millisecond},
		{"Agent Room (warm 1)", func() { emacsWin.SelectFeature("feature:chatroom") }, 16 * time.Millisecond},
		{"Workflow Canvas (warm 2)", func() { emacsWin.SelectFeature("feature:webisland") }, 16 * time.Millisecond},
		{"Agent Room (warm 2)", func() { emacsWin.SelectFeature("feature:chatroom") }, 16 * time.Millisecond},
		{"Sessions", func() { emacsWin.SelectBuffer(buffer.Sessions) }, 16 * time.Millisecond},
		{"Targets", func() { emacsWin.SelectBuffer(buffer.Targets) }, 16 * time.Millisecond},
		{"Packages", func() { emacsWin.SelectBuffer(buffer.Packages) }, 25 * time.Millisecond},
		{"Dashboard", func() { emacsWin.SelectBuffer(buffer.Dashboard) }, 25 * time.Millisecond},
		{"Workflow Canvas (warm 3)", func() { emacsWin.SelectFeature("feature:webisland") }, 16 * time.Millisecond},
		{"Agent Room (warm 3)", func() { emacsWin.SelectFeature("feature:chatroom") }, 16 * time.Millisecond},
	}

	themeEvalsBeforeLoop := themeEvalCount.Load()
	for _, s := range steps {
		drainFrames()
		framesBefore := frameCount.Load()
		t0 := time.Now()
		s.fn()
		_ = waitNextFrame(s.name)
		mygo.RunOnMain(func() {})
		elapsed := time.Since(t0)
		time.Sleep(35 * time.Millisecond)
		framesUsed := frameCount.Load() - framesBefore
		t.Logf("e2e: switch %-28s wall=%v frames=%d", s.name, elapsed.Round(100*time.Microsecond), framesUsed)
		if elapsed > s.maxBudget {
			t.Errorf("switch %s took %v (exceeded budget %v)", s.name, elapsed, s.maxBudget)
		}
		if s.name != "Dashboard" && framesUsed != 1 {
			t.Errorf("switch %s rendered %d frames (expected single-frame switch = 1, zero duplicate frames)", s.name, framesUsed)
		}
	}

	if delta := themeEvalCount.Load() - themeEvalsBeforeLoop; delta != 0 {
		t.Errorf("expected 0 redundant theme Eval() calls during tab switches, got %d", delta)
	}
	if creates := webViewCreates.Load(); creates != 1 {
		t.Errorf("expected exactly 1 pre-warmed WebView creation, got %d", creates)
	}

	frames, slow, maxView := emacsWin.FrameTelemetry()
	t.Logf("e2e summary: frames=%d slowView=%d maxView=%v webViewCreates=%d themeEvals=%d visEvals=%d",
		frames, slow, maxView, webViewCreates.Load(), themeEvalCount.Load(), visEvalCount.Load())
}
