// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

func TestInstallThemePackSHA256PinAndFailClosed(t *testing.T) {
	srcDir := t.TempDir()
	uiDir := t.TempDir()

	css := ":root { --bg: oklch(0.22 0.018 258); --accent: oklch(0.74 0.12 330); }\n.tab.active { border-radius: 8px; }\n"
	srcPath := filepath.Join(srcDir, "aurora.css")
	if err := os.WriteFile(srcPath, []byte(css), 0o600); err != nil {
		t.Fatalf("write source css: %v", err)
	}
	sum := sha256.Sum256([]byte(css))
	goodHash := hex.EncodeToString(sum[:])

	// 1. Valid SHA-256 pinned install succeeds and loads cleanly in LoadUIThemeBundle.
	if _, err := appconfig.InstallThemePack(context.Background(), uiDir, appconfig.ThemePackInstallSpec{
		ID:      "aurora-glass",
		Name:    "Aurora Glass",
		Variant: "dark",
		Base:    "nord-bento",
		URL:     "file://" + srcPath,
		SHA256:  goodHash,
	}); err != nil {
		t.Fatalf("InstallThemePack valid failed: %v", err)
	}
	bundle := appconfig.LoadUIThemeBundle(uiDir)
	if len(bundle.Packs) != 1 || bundle.Packs[0].ID != "aurora-glass" || len(bundle.Warnings) != 0 {
		t.Fatalf("expected installed pack loaded cleanly, got packs=%+v warnings=%v", bundle.Packs, bundle.Warnings)
	}

	// 2. Tampered SHA-256 MUST fail closed without installing.
	badHash := strings.Repeat("0", 64)
	if _, err := appconfig.InstallThemePack(context.Background(), uiDir, appconfig.ThemePackInstallSpec{
		ID:     "tampered-pack",
		URL:    "file://" + srcPath,
		SHA256: badHash,
	}); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch fail-closed error, got %v", err)
	}

	// 3. Insecure http:// URL MUST be rejected fail-closed.
	if _, err := appconfig.InstallThemePack(context.Background(), uiDir, appconfig.ThemePackInstallSpec{
		ID:     "insecure-http",
		URL:    "http://example.com/theme.css",
		SHA256: goodHash,
	}); err == nil || !strings.Contains(err.Error(), "unsupported theme_url scheme") {
		t.Fatalf("expected http scheme rejected, got %v", err)
	}
}
