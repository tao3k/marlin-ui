// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && !js

package featurekit

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
)

func TestWebIslandDefaultDenyTier0AndFocusRevocation(t *testing.T) {
	// Reset global cache cleanly for deterministic unit test.
	ShutdownWebIslands()
	webIslandMu.Lock()
	webViewCache = map[string]*WebViewInstance{}
	webViewPending = map[string]bool{}
	userIslandHTMLs = map[string]string{}
	webViewScope = async.NewStructuredScope(t.Context())
	webViewPermits = async.NewPermitPool(1, 2)
	webIslandMu.Unlock()
	defer ShutdownWebIslands()

	var (
		mu       sync.Mutex
		calls    []string
		gotTiers = map[string]async.TrustTier{}
		doneCh   = make(chan string, 4)
	)
	SetWebViewFactory(func(islandID, html, preloadJS string, tier async.TrustTier) (WebViewInstance, error) {
		mu.Lock()
		calls = append(calls, islandID)
		gotTiers[islandID] = tier
		mu.Unlock()
		defer func() { doneCh <- islandID }()
		// Non-nil sentinel NativeWebView isn't required for lifecycle test,
		// so return an error-free stub; lifecycle only caches non-nil View.
		return WebViewInstance{Tier: tier}, nil
	})
	defer SetWebViewFactory(nil)

	// 1. CheckWorkflowBridgeRPC MUST fail closed before Tier-0 studio is mounted.
	if err := CheckWorkflowBridgeRPC(); err == nil {
		t.Fatalf("expected CheckWorkflowBridgeRPC to fail closed when studio is unmounted")
	}

	// 2. Default-DENY: unregistered external/Marketplace island ID MUST NOT spawn Tier-0 studio.
	host := WebIslandHost()
	if nv := host.Resolve("tao3k.git-workbench"); nv != nil {
		t.Fatalf("expected nil for unregistered external island")
	}
	time.Sleep(25 * time.Millisecond)
	mu.Lock()
	if len(calls) != 0 {
		t.Fatalf("default-deny violated: unregistered island invoked WebViewFactory: %v", calls)
	}
	mu.Unlock()

	// 3. Registered user island MUST receive TrustTier2Untrusted (never Tier-0!).
	RegisterUserIslands(map[string]string{"tao3k.git-workbench": "<html><body>Git</body></html>"})
	_ = host.Resolve("tao3k.git-workbench")
	select {
	case id := <-doneCh:
		if id != "tao3k.git-workbench" {
			t.Fatalf("unexpected island id %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for user island factory")
	}
	mu.Lock()
	if gotTiers["tao3k.git-workbench"] != async.TrustTier2Untrusted {
		t.Fatalf("expected TrustTier2Untrusted for user/marketplace island, got %v", gotTiers["tao3k.git-workbench"])
	}
	mu.Unlock()

	// 4. ONLY BuiltinWorkflowStudioIslandID receives TrustTier0FirstParty.
	_ = host.Resolve(BuiltinWorkflowStudioIslandID)
	select {
	case id := <-doneCh:
		if id != BuiltinWorkflowStudioIslandID {
			t.Fatalf("unexpected island id %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for built-in workflow studio factory")
	}
	mu.Lock()
	if gotTiers[BuiltinWorkflowStudioIslandID] != async.TrustTier0FirstParty {
		t.Fatalf("expected TrustTier0FirstParty for workflow-graph, got %v", gotTiers[BuiltinWorkflowStudioIslandID])
	}
	mu.Unlock()

	// 5. NotifyWebIslandVisibility revokes focus (HideTab) cleanly on cached lifecycles.
	lc := async.NewWebViewLifecycle(async.TrustTier0FirstParty)
	lc.RequestAsyncMount()
	lc.MarkReady()
	lc.Focus()
	webIslandMu.Lock()
	webViewCache[BuiltinWorkflowStudioIslandID] = &WebViewInstance{
		View:      ui.NativeWebView(nil),
		Tier:      async.TrustTier0FirstParty,
		lifecycle: lc,
	}
	webIslandMu.Unlock()

	if err := CheckWorkflowBridgeRPC(); err != nil {
		t.Fatalf("expected CheckWorkflowBridgeRPC allowed for ready Tier-0 studio: %v", err)
	}
	NotifyWebIslandVisibility("")
	if _, shown, focused, _ := lc.Snapshot(); shown || focused {
		t.Fatalf("expected NotifyWebIslandVisibility(\"\") to revoke shown & focused, got shown=%v focused=%v", shown, focused)
	}
}

func TestWorkflowCanvasMatchesViewportThemeAndOpacity(t *testing.T) {
	pal := kit.Palette{
		Canvas:  ui.Hex("#282c34"),
		Bar:     ui.Hex("#21242b"),
		Surface: ui.Hex("#2d333f"),
		Ink:     ui.Hex("#d5dce6"),
	}
	html := workflowStudioThemedHTML(pal, 0.75)
	if !strings.Contains(html, "rgba(40, 44, 52, 0.750)") {
		t.Fatalf("expected Workflow Canvas HTML to embed theme viewport background rgba(40, 44, 52, 0.750), got snippet without it")
	}
	if !strings.Contains(html, "html,body{background:var(--viewport-bg) !important;}") {
		t.Fatalf("expected Workflow Canvas HTML to override raw transparent html/body background")
	}

	script := webIslandViewportThemeScript(pal, 1.0)
	if !strings.Contains(script, "rgba(40, 44, 52, 1.000)") || !strings.Contains(script, "document.body.style.background=bg") {
		t.Fatalf("expected live theme sync script to update html/body viewport background opacity cleanly")
	}

	artHTML := appleGeistArtifactStageHTML(pal, 0.75)
	if !strings.Contains(artHTML, "--marlin-viewport-bg: rgba(40, 44, 52, 0.750)") || !strings.Contains(artHTML, "background: var(--marlin-viewport-bg)") {
		t.Fatalf("expected Artifact Stage HTML to follow theme viewport opacity")
	}
}
