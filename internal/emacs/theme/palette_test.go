// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/style"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestDoomEmacsCSSThemePacksAndGRPCProjection(t *testing.T) {
	cat := theme.Catalog()
	if len(cat) != 4 {
		t.Fatalf("expected 4 built-in CSS theme packs, got %d", len(cat))
	}
	for _, th := range cat {
		if th.Sheet == nil {
			t.Fatalf("theme %q missing compiled CSS Sheet", th.ID)
		}
		hit := th.Sheet.Resolve([]string{"match-hit"}, style.StateNone)
		if !hit.HasBackground || hit.Background.A == 0 || hit.Background.A >= 128 {
			t.Fatalf("theme %q .match-hit must have translucent alpha in (0, 128), got %+v", th.ID, hit.Background)
		}
		card := th.Sheet.Resolve([]string{"bento-card"}, style.StateNone)
		if !card.HasBorderRadius || card.BorderRadius < 10 {
			t.Fatalf("theme %q .bento-card must resolve rounded border-radius >= 10, got %+v", th.ID, card)
		}
	}
	if def := theme.ByID("unknown"); def.ID != theme.DefaultThemeID {
		t.Fatalf("expected fallback to %q, got %q", theme.DefaultThemeID, def.ID)
	}
	if next := theme.Next("nord-bento"); next.ID != "doom-one" {
		t.Fatalf("expected Next(nord-bento)=doom-one, got %q", next.ID)
	}
	if next := theme.Next("doom-one"); next.ID != "doom-tokyo-night" {
		t.Fatalf("expected Next(doom-one)=doom-tokyo-night, got %q", next.ID)
	}
	if next := theme.Next("doom-tokyo-night"); next.ID != "doom-one-light" || !next.IsLight {
		t.Fatalf("expected Next(doom-tokyo-night)=doom-one-light (light), got %+v", next)
	}
	proj := theme.FromUITheme("grpc-doom", "grpc-doom", schema.DefaultUITheme())
	if proj.ID != "grpc-doom" || proj.IsLight || proj.Sheet == nil {
		t.Fatalf("unexpected gRPC theme projection: %+v", proj)
	}
}

func TestGovernedCSSUserPackCascadeAndFailClosedAdmission(t *testing.T) {
	base := theme.DoomOne()
	custom, err := theme.BuildFromCSS("nord-custom", "Nord Custom", false, &base, `
		:root {
			--bg: #2e3440;
			--accent: #88c0d0;
		}
		.tab.active {
			background: #434c5e;
			border-radius: 8px;
		}
	`)
	if err != nil {
		t.Fatalf("BuildFromCSS valid pack failed: %v", err)
	}
	if custom.Canvas != ui.RGB(0x2e, 0x34, 0x40) || custom.Accent != ui.RGB(0x88, 0xc0, 0xd0) {
		t.Fatalf("expected :root variables projected into Theme, got bg=%+v accent=%+v", custom.Canvas, custom.Accent)
	}
	tabActive := custom.Sheet.Resolve([]string{"tab", "active"}, style.StateNone)
	if !tabActive.HasBorderRadius || tabActive.BorderRadius != 8 || tabActive.Background != ui.RGB(0x43, 0x4c, 0x5e) {
		t.Fatalf("expected cascaded .tab.active override, got %+v", tabActive)
	}

	// User ~/.config/marlin/ui/theme.css override cascades cleanly.
	overridden, err := custom.WithOverrideCSS(`
		:root { --accent: #ff79c6; }
		.btn { border-radius: 10px; }
	`)
	if err != nil {
		t.Fatalf("WithOverrideCSS valid override failed: %v", err)
	}
	if overridden.Accent != ui.RGB(0xff, 0x79, 0xc6) {
		t.Fatalf("expected overridden --accent, got %+v", overridden.Accent)
	}
	if btn := overridden.Sheet.Resolve([]string{"btn"}, style.StateNone); btn.BorderRadius != 10 {
		t.Fatalf("expected overridden .btn border-radius=10, got %+v", btn)
	}

	// Fail-closed: undeclared class and forbidden CSS properties must be rejected.
	if _, err := custom.WithOverrideCSS(`.undeclared-widget { background: #111111; }`); err == nil {
		t.Fatalf("expected undeclared class to be rejected fail-closed")
	}
	if _, err := custom.WithOverrideCSS(`.btn { position: fixed; }`); err == nil {
		t.Fatalf("expected undeclared property position to be rejected fail-closed")
	}
	if _, err := custom.WithOverrideCSS(`@import url("https://evil.example/a.css");`); err == nil {
		t.Fatalf("expected @import to be rejected fail-closed")
	}
}
