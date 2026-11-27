// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv loads missing environment variables (such as ANTHROPIC_API_KEY and
// ANTHROPIC_BASE_URL) from .env files co-located with configPath, the current
// working directory, or its parent workspace root. Already-set process
// environment variables are never overwritten, and values are never logged.
func LoadDotEnv(configPath string) []string {
	var candidates []string
	add := func(p string) {
		p = filepath.Clean(p)
		for _, existing := range candidates {
			if existing == p {
				return
			}
		}
		candidates = append(candidates, p)
	}
	if strings.TrimSpace(configPath) != "" {
		if abs, err := filepath.Abs(configPath); err == nil {
			add(filepath.Join(filepath.Dir(abs), ".env"))
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, ".env"))
		// When launched from .data/marlin-ui inside marlin-agent-core, also check workspace root .env.
		parent := filepath.Dir(cwd)
		add(filepath.Join(parent, ".env"))
		add(filepath.Join(filepath.Dir(parent), ".env"))
	}

	var loaded []string
	for _, path := range candidates {
		if loadDotEnvFile(path) {
			loaded = append(loaded, path)
		}
	}
	return loaded
}

func loadDotEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	applied := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if existing, exists := os.LookupEnv(key); exists && strings.TrimSpace(existing) != "" {
			continue
		}
		val := strings.TrimSpace(v)
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if val == "" {
			continue
		}
		if err := os.Setenv(key, val); err == nil {
			applied = true
		}
	}
	return applied
}
