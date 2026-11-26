// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
)

// TestPackagesBufferInstallApplyRemoveFlow verifies Marlin's Emacs *Packages*
// Official Marketplace package manager end-to-end via MyGo's native ui.NewTester:
// filtering categories, installing a Marketplace theme, applying it live onto
// the Frame palette, and removing it cleanly.
func TestPackagesBufferInstallApplyRemoveFlow(t *testing.T) {
	uiDir := t.TempDir()
	stateFile := filepath.Join(uiDir, "packages-state.json")

	pkgStore, err := marketplace.NewPackageStore(marketplace.PackageStoreConfig{
		StateFile: stateFile,
		UIDir:     uiDir,
	})
	if err != nil {
		t.Fatalf("NewPackageStore: %v", err)
	}
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	f := frame.New(frame.Config{
		Adaptive:     adaptive,
		PackageStore: pkgStore,
		AppConfig:    appconfig.Config{SessionID: "pkg-test"},
	})
	defer f.Close()

	f.OpenPackages()
	tt := ui.NewTester(f.View, 980, 580)
	tt.Frame()

	if !tt.HasText("Refresh Marketplace") || !tt.HasText("Catppuccin Mocha") {
		t.Fatalf("expected *Packages* buffer to render Refresh Marketplace and Official themes")
	}

	// Filter to Themes via native MyGo click.
	if err := tt.Click("Themes (4)"); err != nil {
		t.Fatalf("click Themes filter: %v", err)
	}
	tt.Frame()
	if tt.HasText("Git Workbench") {
		t.Fatalf("expected Plugins hidden when Themes filter is active")
	}

	// Install + Apply Catppuccin Mocha via Frame package actions, then verify live theme switch & disk materialization.
	f.InstallPackage("tao3k.theme-catppuccin")
	tt.Frame()
	if _, err := os.Stat(filepath.Join(uiDir, "themes", "catppuccin", "style.css")); err != nil {
		t.Fatalf("expected materialized catppuccin style.css: %v", err)
	}
	if !tt.HasText("Apply") || !tt.HasText("Remove") {
		t.Fatalf("expected Apply and Remove buttons after installing theme")
	}

	f.ApplyPackage("tao3k.theme-catppuccin")
	tt.Frame()
	if f.Theme.ID != "catppuccin" {
		t.Fatalf("expected live Frame.Theme.ID=catppuccin, got %q", f.Theme.ID)
	}

	// Remove uninstalls cleanly and deletes materialized theme directory.
	f.RemovePackage("tao3k.theme-catppuccin")
	tt.Frame()
	if _, err := os.Stat(filepath.Join(uiDir, "themes", "catppuccin")); !os.IsNotExist(err) {
		t.Fatalf("expected catppuccin theme dir removed after RemovePackage")
	}
	if f.ActiveBuffer() != buffer.Packages {
		t.Fatalf("expected active buffer *Packages*, got %s", f.ActiveBuffer())
	}
}
