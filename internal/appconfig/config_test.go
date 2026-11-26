// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

func TestBearerHeaderNormalization(t *testing.T) {
	if got := (appconfig.Config{AuthToken: "abc"}).BearerHeader(); got != "Bearer abc" {
		t.Fatalf("expected Bearer abc, got %q", got)
	}
	if got := (appconfig.Config{AuthToken: "Bearer xyz"}).BearerHeader(); got != "Bearer xyz" {
		t.Fatalf("expected Bearer xyz, got %q", got)
	}
	if got := (appconfig.Config{}).BearerHeader(); got != "" {
		t.Fatalf("expected empty bearer header, got %q", got)
	}
}

func TestDefaultConfigNeverHardcodesDevToken(t *testing.T) {
	t.Setenv("MARLIN_TOKEN", "")
	cfg := appconfig.DefaultConfig()
	if cfg.AuthToken != "" {
		t.Fatalf("expected empty default AuthToken when MARLIN_TOKEN unset, got %q", cfg.AuthToken)
	}
}

func TestDiscoverAppServerBinaryExplicitExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "marlin-app-server")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, src := appconfig.DiscoverAppServerBinary(bin)
	if got != bin || src != "flag" {
		t.Fatalf("expected (%q, flag), got (%q, %q)", bin, got, src)
	}
}

func TestPrefsAtomicRoundTripAndUpsertRemote(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "ui-prefs.json")

	// Missing file returns zero prefs without error.
	empty, err := appconfig.LoadPrefs(p)
	if err != nil {
		t.Fatalf("LoadPrefs missing: %v", err)
	}
	if empty.ThemeID != "" || len(empty.SavedRemotes) != 0 {
		t.Fatalf("expected empty prefs, got %+v", empty)
	}

	in := appconfig.UserPrefs{
		ThemeID:          "tokyo-night",
		FeaturesSpec:     "all",
		SidebarCollapsed: true,
		LastSessionID:    "marlin-work",
	}
	in.UpsertRemote(appconfig.SavedRemote{ID: "edge-1", DisplayName: "Edge GPU", Endpoint: "edge.internal:50051"})
	in.UpsertRemote(appconfig.SavedRemote{ID: "edge-1", DisplayName: "Edge GPU Primary", Endpoint: "edge.internal:50051"})

	if err := appconfig.SavePrefs(p, in); err != nil {
		t.Fatalf("SavePrefs: %v", err)
	}
	out, err := appconfig.LoadPrefs(p)
	if err != nil {
		t.Fatalf("LoadPrefs: %v", err)
	}
	if out.ThemeID != "tokyo-night" || !out.SidebarCollapsed || out.LastSessionID != "marlin-work" {
		t.Fatalf("unexpected prefs round-trip: %+v", out)
	}
	if len(out.SavedRemotes) != 1 || out.SavedRemotes[0].DisplayName != "Edge GPU Primary" {
		t.Fatalf("expected single upserted remote, got %+v", out.SavedRemotes)
	}
	if sum := appconfig.BuildSummary("marlin-desktop"); !strings.Contains(sum, "marlin-desktop") {
		t.Fatalf("unexpected BuildSummary: %q", sum)
	}
}

func TestSettingsSchemaValidationAndApplyValues(t *testing.T) {
	spec := appconfig.SettingsSchema()
	if err := spec.Validate(); err != nil {
		t.Fatalf("SettingsSchema invalid: %v", err)
	}
	cfg := appconfig.DefaultConfig()
	prefs := appconfig.UserPrefs{SidebarCollapsed: true, DefaultRemoteHost: "core.internal:50051"}
	vals := appconfig.InitialSettingsValues(cfg, prefs)
	if err := spec.ValidateValues(vals); err != nil {
		t.Fatalf("InitialSettingsValues failed schema validation: %v", err)
	}
	vals["theme_id"] = "pi"
	vals["features_spec"] = "all"
	vals["default_session"] = "marlin-prod"
	vals["remote_host"] = "edge.marlin.dev:50051"
	vals["real_session"] = false
	vals["sidebar_collapsed"] = false
	appconfig.ApplySettingsValues(&prefs, vals)
	if prefs.ThemeID != "pi" || prefs.FeaturesSpec != "all" || prefs.LastSessionID != "marlin-prod" ||
		prefs.DefaultRemoteHost != "edge.marlin.dev:50051" || prefs.SidebarCollapsed || prefs.RealSession == nil || *prefs.RealSession {
		t.Fatalf("unexpected applied prefs: %+v", prefs)
	}
}
