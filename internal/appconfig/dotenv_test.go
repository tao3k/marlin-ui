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

func TestLoadDotEnvPopulatesMissingKeysWithoutOverwritingExisting(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "example.toml")
	if err := os.WriteFile(cfgPath, []byte("provider = \"deepseek\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envBody := "# comment\nMARLIN_TEST_DOTENV_NEW=from-file\nexport MARLIN_TEST_DOTENV_QUOTED=\"quoted-val\"\nMARLIN_TEST_DOTENV_EXISTING=should-not-overwrite\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARLIN_TEST_DOTENV_EXISTING", "from-shell")
	t.Setenv("MARLIN_TEST_DOTENV_NEW", "")
	t.Setenv("MARLIN_TEST_DOTENV_QUOTED", "")

	loaded := appconfig.LoadDotEnv(cfgPath)
	if len(loaded) == 0 {
		t.Fatalf("expected .env to be loaded from %s", dir)
	}
	if got := os.Getenv("MARLIN_TEST_DOTENV_NEW"); got != "from-file" {
		t.Fatalf("expected MARLIN_TEST_DOTENV_NEW=from-file, got %q", got)
	}
	if got := os.Getenv("MARLIN_TEST_DOTENV_QUOTED"); got != "quoted-val" {
		t.Fatalf("expected MARLIN_TEST_DOTENV_QUOTED=quoted-val, got %q", got)
	}
	if got := os.Getenv("MARLIN_TEST_DOTENV_EXISTING"); got != "from-shell" {
		t.Fatalf("expected existing shell env preserved, got %q", got)
	}
}
