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

func TestDiscoverAppServerBinaryExplicitExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "marlin-mygo-host")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, src := appconfig.DiscoverAppServerBinary(bin)
	if got != bin || src != "flag" {
		t.Fatalf("expected (%q, flag), got (%q, %q)", bin, got, src)
	}
}
