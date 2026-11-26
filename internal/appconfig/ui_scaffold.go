// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"os"
	"path/filepath"
)

const defaultManifestTOML = `# Marlin UI Declarative Surface & Theme Manifest
# Edit and save — Marlin Desktop auto-reloads live (~600ms) or via SPC t r.
schema_version = "marlin.ui.theme.v1"
active_theme = "nord-bento"

[surface]
glass = true
motion = true
vibrancy = "auto"        # "auto" | "hud" | "under-window" | "sidebar" | "popover" | "none"
glass_opacity = 0.34     # floating cards & sidebar [0.08 .. 0.92]
chrome_opacity = 0.18    # top tab bar, modeline & echo bar [0.00 .. 0.95]
viewport_opacity = 0.22  # non-dashboard buffer backdrop [0.00 .. 0.95]
`

const defaultTokensJSON = `{
  "color": {
    "$type": "color",
    "accent": { "$value": { "colorSpace": "oklch", "components": [0.74, 0.11, 330] } },
    "info":   { "$value": { "colorSpace": "oklch", "components": [0.78, 0.08, 215] } }
  },
  "radius": {
    "card": { "$type": "dimension", "$value": "20px" }
  }
}
`

// InitUIConfigDir creates starter declarative TOML & W3C DTCG 2025.10 files in
// dir (~/.config/marlin/ui/) without overwriting any existing user files.
func InitUIConfigDir(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	files := []struct {
		name    string
		content string
	}{
		{"manifest.toml", defaultManifestTOML},
		{"tokens.json", defaultTokensJSON},
	}
	var created []string
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.WriteFile(p, []byte(f.content), 0o600); err != nil {
			return created, err
		}
		created = append(created, f.name)
	}
	return created, nil
}
