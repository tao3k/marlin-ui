// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

func TestLoadUIThemeBundleGovernedLayout(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	mustWrite("manifest.toml", "schema_version = \"marlin.ui.theme.v1\"\nactive_theme = \"nord-doom\"\n")
	mustWrite("themes/nord-doom/theme.toml", "id = \"nord-doom\"\nname = \"Nord Doom\"\nvariant = \"dark\"\nbase = \"doom-one\"\n")
	mustWrite("themes/nord-doom/style.css", ":root { --bg: #2e3440; --bar: #242933; --info: #88c0d0; }\n.tab.active { border-radius: 6px; }\n")
	mustWrite("theme.css", ".btn { border-radius: 8px; }\n")
	// Invalid directory name should be rejected cleanly into Warnings.
	mustWrite("themes/BAD_ID!/theme.toml", "id = \"BAD_ID!\"\n")

	bundle := appconfig.LoadUIThemeBundle(dir)
	if bundle.Manifest.ActiveTheme != "nord-doom" {
		t.Fatalf("expected active_theme='nord-doom', got %+v", bundle.Manifest)
	}
	if len(bundle.Packs) != 1 || bundle.Packs[0].ID != "nord-doom" || bundle.Packs[0].IsLight {
		t.Fatalf("expected 1 valid user pack 'nord-doom', got %+v", bundle.Packs)
	}
	if bundle.OverrideCSS == "" {
		t.Fatalf("expected theme.css override loaded")
	}
	if len(bundle.Warnings) == 0 {
		t.Fatalf("expected warning for invalid theme dir BAD_ID!")
	}
}

func TestLoadUIThemeBundleSurfaceTOML(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	mustWrite("manifest.toml", `schema_version = "marlin.ui.theme.v1"
active_theme = "clear-glass"

[surface]
glass = true
motion = false
vibrancy = "under-window"
glass_opacity = 0.25
chrome_opacity = 0.12
viewport_opacity = 0.18
`)
	mustWrite("themes/clear-glass/theme.toml", `id = "clear-glass"
name = "Clear Glass"
variant = "dark"
[surface]
vibrancy = "sidebar"
glass_opacity = 0.30
`)
	mustWrite("themes/clear-glass/style.css", ":root { --bg: #1e222a; }\n")

	bundle := appconfig.LoadUIThemeBundle(dir)
	if len(bundle.Warnings) != 0 {
		t.Fatalf("expected zero warnings, got %v", bundle.Warnings)
	}
	s := bundle.Manifest.Surface
	if s.Glass == nil || !*s.Glass || s.Motion == nil || *s.Motion {
		t.Fatalf("unexpected glass/motion overrides: %+v", s)
	}
	if s.Vibrancy != "under-window" {
		t.Fatalf("expected vibrancy=under-window, got %q", s.Vibrancy)
	}
	if s.GlassOpacity == nil || *s.GlassOpacity != 0.25 || s.ChromeOpacity == nil || *s.ChromeOpacity != 0.12 || s.ViewportOpacity == nil || *s.ViewportOpacity != 0.18 {
		t.Fatalf("unexpected surface opacities: %+v", s)
	}
	if len(bundle.Packs) != 1 || bundle.Packs[0].Surface.Vibrancy != "sidebar" || bundle.Packs[0].Surface.GlassOpacity == nil || *bundle.Packs[0].Surface.GlassOpacity != 0.30 {
		t.Fatalf("expected theme pack surface parsed, got %+v", bundle.Packs)
	}

	// Out-of-range values fail closed into warnings.
	mustWrite("manifest.toml", "schema_version = \"marlin.ui.theme.v1\"\n[surface]\nvibrancy = \"bogus\"\nglass_opacity = 1.8\n")
	bad := appconfig.LoadUIThemeBundle(dir)
	if len(bad.Warnings) < 2 {
		t.Fatalf("expected fail-closed warnings for invalid vibrancy and glass_opacity, got %v", bad.Warnings)
	}
	if bad.Manifest.Surface.Vibrancy != "" || bad.Manifest.Surface.GlassOpacity != nil {
		t.Fatalf("expected invalid surface fields rejected fail-closed, got %+v", bad.Manifest.Surface)
	}
}
