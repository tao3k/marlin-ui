// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package android_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/mobile/android"
	"github.com/tao3k/marlin-ui/internal/shellmanifest"
)

func TestAndroidMaterialShellSharesMobileContractAndThemeParity(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		FormFactor:     mygoshell.FormFactorAndroid,
		MobilePlatform: mygoshell.MobilePlatformAndroid,
	})
	_, _, _ = featurekit.Mount("all", sh, nil, nil)
	shell := android.New(android.Config{
		Adaptive: sh,
		AppConfig: appconfig.Config{
			SessionID: "android-test-1",
		},
	})
	if shell.Platform() != mobile.PlatformAndroid {
		t.Fatalf("expected PlatformAndroid, got %v", shell.Platform())
	}
	surf := shell.AssembledSurface()
	if !surf.HasModule(shellmanifest.ModAgentRoom) || !surf.HasModule(shellmanifest.ModAndroidMaterial3) || surf.HasModule(shellmanifest.ModIOSTabBar) {
		t.Fatalf("unexpected Android assembled surface composition: %+v", surf)
	}
	if surf.AdmitSlot("approval.gate", false) || !surf.AdmitSlot("approval.gate", true) {
		t.Fatalf("Android surface must enforce fail-closed approval.gate trust invariant")
	}
	if shell.ActiveTheme().ID != "doom-one" {
		t.Fatalf("expected shared default doom-one theme, got %q", shell.ActiveTheme().ID)
	}
	shell.SetTheme("tokyo-night")
	if shell.ActiveTheme().ID != "doom-tokyo-night" {
		t.Fatalf("expected shared tokyo-night alias resolution, got %q", shell.ActiveTheme().ID)
	}
	shell.SelectTab(mobile.TabRooms)
	snap := shell.RoomSnapshot()
	if !snap.Available || len(snap.Channels) == 0 || len(snap.Participants) == 0 {
		t.Fatalf("expected Android shell to share unified featurekit.RoomController snapshot, got available=%v channels=%d participants=%d",
			snap.Available, len(snap.Channels), len(snap.Participants))
	}

	tt := ui.NewTester(shell.View, 412, 915)
	tt.Frame()

	// Live Scheme UIWorkbenchSnapshot projects identically on Android via shared uiframework.ProjectWorkbench.
	shell.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation: 6,
		StatusItems: []schema.UIStatusItem{
			{ID: "git", Label: "main", KeepPriority: 100},
			{ID: "desktop-wide", Label: "WIDE ONLY", MinWidth: 100, KeepPriority: 10},
		},
		Extensions: []schema.UIExtensionRegion{
			{
				ID:    "android-trace",
				Label: "*Trace*",
				Content: schema.UIElement{
					Kind:  schema.ElemText,
					Spans: []schema.UISpan{{Text: "Android Scheme Telemetry Ready", Tone: schema.ToneInfo}},
				},
			},
		},
	})
	if len(shell.SchemeExtensions()) != 1 || len(shell.SchemeStatusItems()) != 1 {
		t.Fatalf("expected 1 Scheme extension and 1 mobile-width status item on Android, got exts=%d status=%d",
			len(shell.SchemeExtensions()), len(shell.SchemeStatusItems()))
	}

	// Verify all 5 Material 3 tabs render real shared views cleanly!
	// 1. Rooms tab renders live channels + compact status badge.
	shell.SelectTab(mobile.TabRooms)
	tt.Frame()
	if !tt.HasText("Agent Rooms") || !tt.HasText("#workspace") {
		t.Fatalf("expected Android Rooms tab to render Agent Rooms + #workspace, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 2. Bento Home tab renders live Scheme UIExtensionRegion + mobile status item!
	shell.SelectTab(mobile.TabHome)
	tt.Frame()
	if !tt.HasText("SCHEME EXTENSIONS") || !tt.HasText("Android Scheme Telemetry Ready") || !tt.HasText("main") {
		t.Fatalf("expected Android Bento Home to render Scheme Extensions & status items, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 3. Chat tab renders sent prompts cleanly.
	shell.SendPrompt("Hello from Android Material 3")
	shell.SelectTab(mobile.TabChat)
	tt.Frame()
	if !tt.HasText("Hello from Android Material 3") {
		t.Fatalf("expected Android Chat tab to render sent prompt, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 4. Search & Style tabs render shared views cleanly.
	shell.SelectTab(mobile.TabSearch)
	tt.Frame()
	if !tt.HasText("Search") {
		t.Fatalf("expected Android Search tab header")
	}
	shell.SelectTab(mobile.TabSettings)
	tt.Frame()
	if !tt.HasText("THEME PACKS") || !tt.HasText("Deep Think") {
		t.Fatalf("expected Android Style tab to render shared THEME PACKS & Deep Think, got %q", strings.Join(tt.Texts(), " | "))
	}
}
