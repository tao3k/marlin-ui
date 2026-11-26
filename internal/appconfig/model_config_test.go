// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModelConfigTOMLMatchesTUIExample(t *testing.T) {
	raw := `
# DeepSeek Anthropic-compatible default route
provider = "deepseek"
model = "deepseek-flash"
protocol = "anthropic"
credential_env = "ANTHROPIC_API_KEY"
timeout_seconds = 120

[anthropic]
thinking = { type = "enabled", budget_tokens = 4096 }
effort = "low"
max_tokens = 8192

[models.deepseek-v4-pro]
provider = "deepseek"
model = "deepseek-v4-pro"
protocol = "anthropic"
credential_env = "ANTHROPIC_API_KEY"

[models.openai]
provider = "openai"
model = "gpt-4.1-mini"
endpoint = "https://api.openai.com/v1/responses"
protocol = "open-ai-responses"
credential_env = "OPENAI_API_KEY"

[session]
project = "project:marlin"
session = "session:main"

[host]
addr = "127.0.0.1:50051"
real_session = true
`
	cfg, err := ParseModelConfigTOML(raw)
	if err != nil {
		t.Fatalf("ParseModelConfigTOML failed: %v", err)
	}
	cfg.Loaded = true

	if cfg.Default.Provider != "deepseek" || cfg.Default.Model != "deepseek-flash" || cfg.Default.Protocol != "anthropic" {
		t.Fatalf("unexpected default route: %+v", cfg.Default)
	}
	if cfg.Default.TimeoutSeconds != 120 {
		t.Fatalf("expected timeout 120, got %d", cfg.Default.TimeoutSeconds)
	}
	if cfg.Anthropic.ThinkingType != "enabled" || cfg.Anthropic.BudgetTokens != 4096 || cfg.Anthropic.MaxTokens != 8192 {
		t.Fatalf("unexpected anthropic config: %+v", cfg.Anthropic)
	}
	if got := cfg.SummaryLabel(); got != "deepseek/deepseek-flash" {
		t.Fatalf("unexpected summary label: %q", got)
	}
	if got := cfg.ThinkingSummary(); got != "enabled (4096 tok)" {
		t.Fatalf("unexpected thinking summary: %q", got)
	}
	routes := cfg.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes (default + 2 named), got %d", len(routes))
	}
	if routes[1].Alias != "deepseek-v4-pro" || routes[2].Alias != "openai" {
		t.Fatalf("expected deterministic sorted named routes, got %+v", routes)
	}
	if cfg.Session.Session != "session:main" || cfg.Host.Addr != "127.0.0.1:50051" || cfg.Host.RealSession == nil || !*cfg.Host.RealSession {
		t.Fatalf("unexpected session/host config: %+v %+v", cfg.Session, cfg.Host)
	}
	if !cfg.Default.CredentialConfigured(func(k string) string {
		if k == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}) {
		t.Fatal("expected ANTHROPIC_API_KEY to be detected as configured")
	}
}

func TestLoadModelConfigFromExplicitFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.toml")
	content := "provider = \"deepseek\"\nmodel = \"deepseek-flash\"\nprotocol = \"anthropic\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp toml: %v", err)
	}
	cfg := LoadModelConfig(path)
	if !cfg.Loaded || cfg.Warning != "" {
		t.Fatalf("expected clean load, got loaded=%v warn=%q", cfg.Loaded, cfg.Warning)
	}
	if cfg.SourceOrigin != "flag" || cfg.Default.Model != "deepseek-flash" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
}
