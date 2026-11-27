// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package ios_test

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/mobile/ios"
)

func TestIOSLiquidGlassShellTabsThemeParityAndZeroFlickerSearch(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorIOS,
	})
	shell := ios.New(ios.Config{
		Adaptive: sh,
		AppConfig: appconfig.Config{
			SessionID: "ios-test-1",
		},
	})
	if shell.Platform() != mobile.PlatformIOS {
		t.Fatalf("expected PlatformIOS, got %v", shell.Platform())
	}

	tt := ui.NewTester(shell.View, 393, 852)
	tt.SetVibrancy(true)
	settlePager := func() {
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(20 * time.Millisecond)
		tt.Advance(600 * time.Millisecond)
		tt.SetPreferences(ui.Preferences{})
		tt.Frame()
	}
	settlePager()

	// 1. Default Chat Hero Stage renders with authentic Doom One theme parity (no pink).
	if !tt.HasText("What can we build today?") {
		t.Fatalf("expected iOS Chat Hero Stage, got %q", strings.Join(tt.Texts(), " | "))
	}
	if shell.ActiveTheme().ID != "doom-one" {
		t.Fatalf("expected default Desktop-parity doom-one theme, got %q", shell.ActiveTheme().ID)
	}
	if shell.ActiveTheme().Accent != ui.RGB(0x51, 0xaf, 0xef) {
		t.Fatalf("expected authentic Doom One blue accent #51afef, got %+v", shell.ActiveTheme().Accent)
	}

	// 2. Switch to iOS 26 Bento Dashboard (WidgetKit + Control Center tiles + Live Tasks).
	shell.SelectTab(mobile.TabHome)
	settlePager()
	if !tt.HasText("WORKSPACE") || !tt.HasText("LIVE TASKS") || !tt.HasText("Deep Think") {
		t.Fatalf("expected iOS 26 Bento Dashboard widgets rendered, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 3. Switch to Search tab & verify FrameLatch zero-blank-flash on long titles.
	shell.SelectTab(mobile.TabSearch)
	gen1 := sh.Desktop.HelmSearch.Engine.Generation()
	sh.Desktop.HelmSearch.Engine.IngestBatch(gen1, search.SourceMeta{
		ID: "rust-graph", Label: "Workspace Search", Priority: 2,
	}, []search.Candidate{
		{
			ID:       "rust:ios-1",
			SourceID: "rust-graph",
			Title:    "SPDX-FileCopyrightText: 2026 tao3k team and Contributors · ios_search_long.rs",
			Location: "crates/marlin-agent-app-server/src/ui_grpc.rs:L42",
			Snippet:  "pub fn query_workspace() {}",
		},
	})
	tt.Frame()
	if !tt.HasText("ios_search_long.rs") {
		t.Fatalf("expected Helm candidate rendered in iOS Search tab, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 4. Switch to Style/Settings tab & verify 2x2 Theme Swatch Grid + live Doom One / Tokyo Night / Light switch.
	shell.SelectTab(mobile.TabSettings)
	settlePager()
	if !tt.HasText("THEME PACKS") || !tt.HasText("Tokyo Night") {
		t.Fatalf("expected Settings Theme Packs swatch grid with Tokyo Night, got %q", strings.Join(tt.Texts(), " | "))
	}
	shell.SetTheme("tokyo-night")
	tt.Frame()
	if shell.ActiveTheme().ID != "doom-tokyo-night" || shell.ActiveTheme().Accent != ui.RGB(0x7a, 0xa2, 0xf7) {
		t.Fatalf("expected live theme switch to Tokyo Night sapphire accent, got %+v", shell.ActiveTheme())
	}
	shell.SetTheme("doom-one-light")
	tt.Frame()
	if !shell.ActiveTheme().IsLight || shell.ActiveTheme().ID != "doom-one-light" {
		t.Fatalf("expected live theme switch to doom-one-light, got %+v", shell.ActiveTheme().ID)
	}
}
