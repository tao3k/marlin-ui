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
active_theme = "doom-one"

[surface]
glass = true
motion = true
vibrancy = "glass-clear"  # "glass-clear" | "glass" | "hud" | "popover" | "under-window" | "sidebar" | "none"
window_opacity = 0.75     # unified smoked glass opacity across entire window (0.75 = 75% opaque / 25% see-through)
glass_opacity = 0.56      # floating cards, popups & sidebar [0.08 .. 0.92]
chrome_opacity = 0.0      # 0.0 inherits unified window surface (zero seams); set >0 to tint bars separately
`

const defaultTokensJSON = `{
  "color": {
    "$type": "color",
    "accent": { "$value": { "colorSpace": "oklch", "components": [0.72, 0.13, 240] } },
    "info":   { "$value": { "colorSpace": "oklch", "components": [0.78, 0.10, 215] } }
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
