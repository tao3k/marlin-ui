// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ModelRoute describes one declarative LLM route declared in example.toml /
// marlin.toml (matching crates/marlin-agent-tui/example.toml). Marlin UI never
// invokes LLM endpoints directly in Go; it projects routes into *Models* and
// *conversation* and forwards MARLIN_CONFIG to the authoritative Rust host.
type ModelRoute struct {
	Alias          string
	Provider       string
	Model          string
	Protocol       string
	Endpoint       string
	CredentialEnv  string
	TimeoutSeconds int
	IsDefault      bool
}

// AnthropicThinkingConfig mirrors the optional [anthropic] thinking envelope
// from marlin-agent-tui/example.toml.
type AnthropicThinkingConfig struct {
	ThinkingType string // "", "adaptive", "enabled", "disabled"
	BudgetTokens int
	Effort       string // "", "low", "medium", "high"
	MaxTokens    int
}

// SessionTOMLConfig mirrors the optional [session] section.
type SessionTOMLConfig struct {
	Path    string
	Project string
	Session string
	Agent   string
}

// HostTOMLConfig mirrors the optional [host] section for marlin-desktop.
type HostTOMLConfig struct {
	Addr         string
	AppServerBin string
	RealSession  *bool
}

// ModelConfig is the parsed read-only projection of example.toml / marlin.toml.
type ModelConfig struct {
	SourcePath   string
	SourceOrigin string // "flag", "env", "cwd", "xdg", "example", ""
	Default      ModelRoute
	Named        map[string]ModelRoute
	Anthropic    AnthropicThinkingConfig
	Session      SessionTOMLConfig
	Host         HostTOMLConfig
	Loaded       bool
	Warning      string
}

// DiscoverModelConfigPath resolves the model TOML config path in priority order:
//  1. Explicit `-config` flag
//  2. `MARLIN_CONFIG` environment variable
//  3. `./marlin.toml` in the current working directory
//  4. `~/.config/marlin/config.toml` (or `$XDG_CONFIG_HOME/marlin/config.toml`)
//  5. `./example.toml` in the current working directory
func DiscoverModelConfigPath(explicit string) (path string, origin string) {
	check := func(p, src string) (string, string, bool) {
		p = strings.TrimSpace(p)
		if p == "" {
			return "", "", false
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return "", "", false
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		return p, src, true
	}
	if p, s, ok := check(explicit, "flag"); ok {
		return p, s
	}
	if p, s, ok := check(os.Getenv("MARLIN_CONFIG"), "env"); ok {
		return p, s
	}
	if p, s, ok := check("marlin.toml", "cwd"); ok {
		return p, s
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		if p, s, ok := check(filepath.Join(xdg, "marlin", "config.toml"), "xdg"); ok {
			return p, s
		}
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p, s, ok := check(filepath.Join(home, ".config", "marlin", "config.toml"), "xdg"); ok {
			return p, s
		}
	}
	if p, s, ok := check("example.toml", "example"); ok {
		return p, s
	}
	return "", ""
}

// LoadModelConfig reads and parses a marlin.toml / example.toml file.
// Missing paths return an empty ModelConfig with Loaded=false and no error.
func LoadModelConfig(explicit string) ModelConfig {
	path, origin := DiscoverModelConfigPath(explicit)
	if path == "" {
		if strings.TrimSpace(explicit) != "" {
			return ModelConfig{
				SourcePath:   explicit,
				SourceOrigin: "flag",
				Warning:      fmt.Sprintf("config file not found: %s", explicit),
			}
		}
		return ModelConfig{}
	}
	raw, err := readBoundedFile(path, 64*1024)
	if err != nil {
		return ModelConfig{
			SourcePath:   path,
			SourceOrigin: origin,
			Warning:      fmt.Sprintf("read %s: %v", path, err),
		}
	}
	cfg, err := ParseModelConfigTOML(raw)
	if err != nil {
		return ModelConfig{
			SourcePath:   path,
			SourceOrigin: origin,
			Warning:      fmt.Sprintf("parse %s: %v", path, err),
		}
	}
	cfg.SourcePath = path
	cfg.SourceOrigin = origin
	cfg.Loaded = true
	return cfg
}

// ParseModelConfigTOML parses a strict subset of TOML matching
// crates/marlin-agent-tui/example.toml without external dependencies.
func ParseModelConfigTOML(raw string) (ModelConfig, error) {
	cfg := ModelConfig{
		Default: ModelRoute{Alias: "default", IsDefault: true},
		Named:   make(map[string]ModelRoute),
	}
	section := ""
	lines := strings.Split(raw, "\n")
	for idx, orig := range lines {
		line := stripTOMLComment(orig)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return ModelConfig{}, fmt.Errorf("line %d: expected key = value", idx+1)
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		switch {
		case section == "":
			applyRouteField(&cfg.Default, key, val)
		case section == "anthropic":
			applyAnthropicField(&cfg.Anthropic, key, val)
		case section == "session":
			applySessionField(&cfg.Session, key, val)
		case section == "host":
			applyHostField(&cfg.Host, key, val)
		case strings.HasPrefix(section, "models."):
			alias := strings.TrimSpace(strings.TrimPrefix(section, "models."))
			alias = unquoteTOML(alias)
			if alias == "" {
				continue
			}
			rt := cfg.Named[alias]
			rt.Alias = alias
			applyRouteField(&rt, key, val)
			cfg.Named[alias] = rt
		}
	}
	return cfg, nil
}

// Routes returns the default route (if configured) followed by named routes
// sorted deterministically by alias.
func (m ModelConfig) Routes() []ModelRoute {
	var out []ModelRoute
	if m.Default.Model != "" || m.Default.Provider != "" {
		out = append(out, m.Default)
	}
	if len(m.Named) > 0 {
		aliases := make([]string, 0, len(m.Named))
		for k := range m.Named {
			aliases = append(aliases, k)
		}
		sort.Strings(aliases)
		for _, a := range aliases {
			out = append(out, m.Named[a])
		}
	}
	return out
}

// SummaryLabel returns a concise status string such as
// "deepseek/deepseek-flash" for the header and modeline.
func (m ModelConfig) SummaryLabel() string {
	if !m.Loaded {
		return ""
	}
	prov := strings.TrimSpace(m.Default.Provider)
	mod := strings.TrimSpace(m.Default.Model)
	switch {
	case prov != "" && mod != "":
		return prov + "/" + mod
	case mod != "":
		return mod
	case prov != "":
		return prov
	default:
		return ""
	}
}

// ThinkingSummary returns a human-readable summary of [anthropic] thinking
// settings (e.g. "adaptive · low" or "enabled (4096 tok)").
func (m ModelConfig) ThinkingSummary() string {
	t := strings.TrimSpace(m.Anthropic.ThinkingType)
	if t == "" {
		return ""
	}
	if m.Anthropic.BudgetTokens > 0 {
		return fmt.Sprintf("%s (%d tok)", t, m.Anthropic.BudgetTokens)
	}
	if m.Anthropic.Effort != "" {
		return t + " · " + m.Anthropic.Effort
	}
	return t
}

// CredentialConfigured reports whether the route's declared credential_env
// variable is present and non-empty in the environment (without exposing it).
func (r ModelRoute) CredentialConfigured(lookup func(string) string) bool {
	if lookup == nil {
		lookup = os.Getenv
	}
	envName := strings.TrimSpace(r.CredentialEnv)
	if envName == "" {
		return false
	}
	return strings.TrimSpace(lookup(envName)) != ""
}

func applyRouteField(r *ModelRoute, key, val string) {
	switch key {
	case "provider":
		r.Provider = unquoteTOML(val)
	case "model":
		r.Model = unquoteTOML(val)
	case "protocol":
		r.Protocol = unquoteTOML(val)
	case "endpoint":
		r.Endpoint = unquoteTOML(val)
	case "credential_env":
		r.CredentialEnv = unquoteTOML(val)
	case "timeout_seconds":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			r.TimeoutSeconds = n
		}
	}
}

func applyAnthropicField(a *AnthropicThinkingConfig, key, val string) {
	switch key {
	case "effort":
		a.Effort = unquoteTOML(val)
	case "max_tokens":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			a.MaxTokens = n
		}
	case "thinking":
		// Parse inline table e.g. { type = "adaptive" } or { type = "enabled", budget_tokens = 4096 }.
		trimmed := strings.TrimSpace(val)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			inner := trimmed[1 : len(trimmed)-1]
			for _, part := range strings.Split(inner, ",") {
				ik, iv, ok := strings.Cut(part, "=")
				if !ok {
					continue
				}
				switch strings.TrimSpace(ik) {
				case "type":
					a.ThinkingType = unquoteTOML(strings.TrimSpace(iv))
				case "budget_tokens":
					if n, err := strconv.Atoi(strings.TrimSpace(iv)); err == nil && n > 0 {
						a.BudgetTokens = n
					}
				}
			}
		} else {
			a.ThinkingType = unquoteTOML(trimmed)
		}
	}
}

func applySessionField(s *SessionTOMLConfig, key, val string) {
	switch key {
	case "path":
		s.Path = unquoteTOML(val)
	case "project":
		s.Project = unquoteTOML(val)
	case "session":
		s.Session = unquoteTOML(val)
	case "agent":
		s.Agent = unquoteTOML(val)
	}
}

func applyHostField(h *HostTOMLConfig, key, val string) {
	switch key {
	case "addr":
		h.Addr = unquoteTOML(val)
	case "app_server_bin":
		h.AppServerBin = unquoteTOML(val)
	case "real_session":
		if b, err := strconv.ParseBool(strings.TrimSpace(val)); err == nil {
			h.RealSession = &b
		}
	}
}

func stripTOMLComment(line string) string {
	inDouble := false
	inSingle := false
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inDouble {
			escaped = true
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '#' && !inDouble && !inSingle {
			return line[:i]
		}
	}
	return line
}

func unquoteTOML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}
