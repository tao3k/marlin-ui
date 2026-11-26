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

	"github.com/tao3k/marlin-mygo/runtime/style"
)

// UIThemeSchemaVersion pins the strict manifest schema for ~/.config/marlin/ui/.
const UIThemeSchemaVersion = "marlin.ui.theme.v1"

// SurfaceConfig declares window vibrancy and frosted-glass translucency
// parameters in ~/.config/marlin/ui/manifest.toml ([surface]) or
// themes/<id>/theme.toml ([surface]). Nil/empty fields inherit framework defaults.
type SurfaceConfig struct {
	Glass           *bool    // optional override for frosted glass surface
	Motion          *bool    // optional override for 120Hz spring physics
	Vibrancy        string   // "", "auto", "hud", "under-window", "sidebar", "popover", "none"
	GlassOpacity    *float32 // floating card/island opacity in [0.08, 0.92]
	ChromeOpacity   *float32 // top tab bar, modeline, echo bar opacity in [0.00, 0.95]
	ViewportOpacity *float32 // non-dashboard buffer backdrop opacity in [0.00, 0.95]
}

// UIThemeManifest is the typed root manifest at ~/.config/marlin/ui/manifest.toml.
type UIThemeManifest struct {
	SchemaVersion string
	ActiveTheme   string
	Surface       SurfaceConfig
}

// UserThemePack describes one validated theme pack under
// ~/.config/marlin/ui/themes/<id>/{theme.toml, style.css}.
type UserThemePack struct {
	ID       string
	Name     string
	IsLight  bool
	Base     string
	StyleCSS string
	Surface  SurfaceConfig
}

// UserWidgetSpec describes one governed desktop widget under
// ~/.config/marlin/ui/widgets/<id>/{widget.toml | layout.sexp, style.css}.
type UserWidgetSpec struct {
	ID         string
	Format     string // "sexp" or "toml"
	Definition string
	StyleCSS   string
}

// UIThemeBundle captures the strictly governed contents of ~/.config/marlin/ui/.
type UIThemeBundle struct {
	Dir             string
	Manifest        UIThemeManifest
	Packs           []UserThemePack
	TokensJSON      string // optional W3C Design Tokens (tokens.json with oklch() & aliases)
	OverrideCSS     string
	DashboardFormat string // "", "sexp", "toml", or "json"
	DashboardDef    string
	SidebarFormat   string // "", "json"
	SidebarDef      string
	CommandsTOML    string
	KeymapsTOML     string
	Widgets         []UserWidgetSpec
	WebIslands      map[string]string // governed islands/<id>/index.html artifacts
	Warnings        []string
}

// DefaultUIThemeDir returns the governed user UI configuration root:
// ~/.config/marlin/ui (or $XDG_CONFIG_HOME/marlin/ui).
func DefaultUIThemeDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "marlin", "ui")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".config", "marlin", "ui")
	}
	return filepath.Join(home, ".config", "marlin", "ui")
}

// LoadUIThemeBundle loads and strictly validates ~/.config/marlin/ui/:
//   - manifest.toml (optional; schema_version = "marlin.ui.theme.v1")
//   - themes/<id>/theme.toml + themes/<id>/style.css (strict ID & size checks)
//   - theme.css (optional user override layer, bounded to style.DefaultMaxBytes)
//
// Missing files are not errors; malformed files fail closed into Warnings
// without disrupting application startup.
func LoadUIThemeBundle(dir string) UIThemeBundle {
	bundle := UIThemeBundle{Dir: dir}
	if strings.TrimSpace(dir) == "" {
		return bundle
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return bundle
	}

	// 1. Optional root manifest.toml.
	manifestPath := filepath.Join(dir, "manifest.toml")
	if raw, err := readBoundedFile(manifestPath, 16*1024); err == nil {
		kv, parseErr := parseFlatTOML(raw)
		if parseErr != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("manifest.toml: %v", parseErr))
		} else {
			ver := kv["schema_version"]
			if ver != "" && ver != UIThemeSchemaVersion {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("manifest.toml: unsupported schema_version %q (expected %q)", ver, UIThemeSchemaVersion))
			} else {
				bundle.Manifest = UIThemeManifest{
					SchemaVersion: ver,
					ActiveTheme:   strings.TrimSpace(kv["active_theme"]),
					Surface:       parseSurfaceConfig(kv, "manifest.toml", &bundle.Warnings),
				}
			}
		}
	}

	// 2. Governed named theme packs under themes/<id>/{theme.toml, style.css}.
	themesDir := filepath.Join(dir, "themes")
	if entries, err := os.ReadDir(themesDir); err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			id := ent.Name()
			if !isValidThemeID(id) {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("themes/%s: invalid theme directory id", id))
				continue
			}
			packDir := filepath.Join(themesDir, id)
			metaRaw, err := readBoundedFile(filepath.Join(packDir, "theme.toml"), 8*1024)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("themes/%s/theme.toml: %v", id, err))
				continue
			}
			kv, err := parseFlatTOML(metaRaw)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("themes/%s/theme.toml: %v", id, err))
				continue
			}
			if kv["id"] != "" && kv["id"] != id {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("themes/%s/theme.toml: id %q must match directory name", id, kv["id"]))
				continue
			}
			cssRaw, err := readBoundedFile(filepath.Join(packDir, "style.css"), style.DefaultMaxBytes)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("themes/%s/style.css: %v", id, err))
				continue
			}
			name := kv["name"]
			if name == "" {
				name = id
			}
			bundle.Packs = append(bundle.Packs, UserThemePack{
				ID:       id,
				Name:     name,
				IsLight:  strings.EqualFold(kv["variant"], "light"),
				Base:     kv["base"],
				StyleCSS: cssRaw,
				Surface:  parseSurfaceConfig(kv, fmt.Sprintf("themes/%s/theme.toml", id), &bundle.Warnings),
			})
		}
	}

	// 3. Optional W3C Design Tokens (tokens.json) & CSS override (theme.css).
	tokensPath := filepath.Join(dir, "tokens.json")
	if _, err := os.Stat(tokensPath); err == nil {
		raw, err := readBoundedFile(tokensPath, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("tokens.json: %v", err))
		} else {
			bundle.TokensJSON = raw
		}
	}
	overridePath := filepath.Join(dir, "theme.css")
	if _, err := os.Stat(overridePath); err == nil {
		raw, err := readBoundedFile(overridePath, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("theme.css: %v", err))
		} else {
			bundle.OverrideCSS = raw
		}
	}

	// 4. Optional full Bento dashboard override: dashboard.json (TS SDK), dashboard.sexp, or dashboard.toml.
	dashJSON := filepath.Join(dir, "dashboard.json")
	dashSexp := filepath.Join(dir, "dashboard.sexp")
	dashToml := filepath.Join(dir, "dashboard.toml")
	if _, err := os.Stat(dashJSON); err == nil {
		raw, err := readBoundedFile(dashJSON, 128*1024)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("dashboard.json: %v", err))
		} else {
			bundle.DashboardFormat = "json"
			bundle.DashboardDef = raw
		}
	} else if _, err := os.Stat(dashSexp); err == nil {
		raw, err := readBoundedFile(dashSexp, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("dashboard.sexp: %v", err))
		} else {
			bundle.DashboardFormat = "sexp"
			bundle.DashboardDef = raw
		}
	} else if _, err := os.Stat(dashToml); err == nil {
		raw, err := readBoundedFile(dashToml, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("dashboard.toml: %v", err))
		} else {
			bundle.DashboardFormat = "toml"
			bundle.DashboardDef = raw
		}
	}

	// 4a. Optional custom floating left-edge sidebar spec: sidebar.json (emitted by @marlin/ui-kit).
	sidebarJSON := filepath.Join(dir, "sidebar.json")
	if _, err := os.Stat(sidebarJSON); err == nil {
		raw, err := readBoundedFile(sidebarJSON, 128*1024)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("sidebar.json: %v", err))
		} else {
			bundle.SidebarFormat = "json"
			bundle.SidebarDef = raw
		}
	}

	// 4b. Optional user declarative commands.toml & keymaps.toml overrides.
	cmdPath := filepath.Join(dir, "commands.toml")
	if _, err := os.Stat(cmdPath); err == nil {
		raw, err := readBoundedFile(cmdPath, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("commands.toml: %v", err))
		} else {
			bundle.CommandsTOML = raw
		}
	}
	kmPath := filepath.Join(dir, "keymaps.toml")
	if _, err := os.Stat(kmPath); err == nil {
		raw, err := readBoundedFile(kmPath, style.DefaultMaxBytes)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("keymaps.toml: %v", err))
		} else {
			bundle.KeymapsTOML = raw
		}
	}

	// 5. Governed user widgets under widgets/<id>/{layout.sexp | widget.toml, optional style.css}.
	widgetsDir := filepath.Join(dir, "widgets")
	if entries, err := os.ReadDir(widgetsDir); err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			id := ent.Name()
			if !isValidThemeID(id) {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s: invalid widget directory id", id))
				continue
			}
			wDir := filepath.Join(widgetsDir, id)
			var spec UserWidgetSpec
			spec.ID = id
			jsonPath := filepath.Join(wDir, "widget.json")
			sexpPath := filepath.Join(wDir, "layout.sexp")
			tomlPath := filepath.Join(wDir, "widget.toml")
			if _, err := os.Stat(jsonPath); err == nil {
				raw, err := readBoundedFile(jsonPath, 128*1024)
				if err != nil {
					bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s/widget.json: %v", id, err))
					continue
				}
				spec.Format = "json"
				spec.Definition = raw
			} else if _, err := os.Stat(sexpPath); err == nil {
				raw, err := readBoundedFile(sexpPath, style.DefaultMaxBytes)
				if err != nil {
					bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s/layout.sexp: %v", id, err))
					continue
				}
				spec.Format = "sexp"
				spec.Definition = raw
			} else if _, err := os.Stat(tomlPath); err == nil {
				raw, err := readBoundedFile(tomlPath, 16*1024)
				if err != nil {
					bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s/widget.toml: %v", id, err))
					continue
				}
				spec.Format = "toml"
				spec.Definition = raw
			} else {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s: missing widget.json, layout.sexp, or widget.toml", id))
				continue
			}
			cssPath := filepath.Join(wDir, "style.css")
			if _, err := os.Stat(cssPath); err == nil {
				rawCSS, err := readBoundedFile(cssPath, style.DefaultMaxBytes)
				if err != nil {
					bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("widgets/%s/style.css: %v", id, err))
				} else {
					spec.StyleCSS = rawCSS
				}
			}
			bundle.Widgets = append(bundle.Widgets, spec)
		}
	}

	// 6. Governed Hybrid WebView islands under islands/<id>/index.html (bounded to 128KB).
	islandsDir := filepath.Join(dir, "islands")
	if entries, err := os.ReadDir(islandsDir); err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		bundle.WebIslands = make(map[string]string)
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			id := ent.Name()
			if !isValidThemeID(id) {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("islands/%s: invalid island directory id", id))
				continue
			}
			raw, err := readBoundedFile(filepath.Join(islandsDir, id, "index.html"), 128*1024)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("islands/%s/index.html: %v", id, err))
				continue
			}
			bundle.WebIslands[id] = raw
		}
	}

	return bundle
}

func readBoundedFile(path string, maxBytes int) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("expected file, found directory")
	}
	if info.Size() > int64(maxBytes) {
		return "", fmt.Errorf("file exceeds max allowed size (%d > %d bytes)", info.Size(), maxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseFlatTOML parses strict flat key = "value" TOML metadata files with zero
// external dependencies. Rejects unknown/malformed lines fail-closed.
func parseFlatTOML(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for idx, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = \"value\"", idx+1)
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		// Strip inline trailing comment outside quotes.
		if strings.HasPrefix(val, `"`) {
			if end := strings.Index(val[1:], `"`); end >= 0 {
				val = val[1 : 1+end]
			} else {
				return nil, fmt.Errorf("line %d: unclosed string quote", idx+1)
			}
		} else {
			if c := strings.IndexByte(val, '#'); c >= 0 {
				val = strings.TrimSpace(val[:c])
			}
		}
		out[key] = val
	}
	return out, nil
}

func parseSurfaceConfig(kv map[string]string, file string, warnings *[]string) SurfaceConfig {
	var cfg SurfaceConfig
	parseBoolOpt := func(key string) *bool {
		raw, ok := kv[key]
		if !ok || strings.TrimSpace(raw) == "" {
			return nil
		}
		b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("%s: invalid %s %q (expected true|false)", file, key, raw))
			return nil
		}
		return &b
	}
	parseFloatOpt := func(key string, minV, maxV float32) *float32 {
		raw, ok := kv[key]
		if !ok || strings.TrimSpace(raw) == "" {
			return nil
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(raw), 32)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("%s: invalid %s %q", file, key, raw))
			return nil
		}
		f := float32(v)
		if f < minV || f > maxV {
			*warnings = append(*warnings, fmt.Sprintf("%s: %s=%.2f out of allowed range [%.2f, %.2f]", file, key, f, minV, maxV))
			return nil
		}
		return &f
	}
	cfg.Glass = parseBoolOpt("glass")
	cfg.Motion = parseBoolOpt("motion")
	cfg.GlassOpacity = parseFloatOpt("glass_opacity", 0.08, 0.92)
	cfg.ChromeOpacity = parseFloatOpt("chrome_opacity", 0.0, 0.95)
	cfg.ViewportOpacity = parseFloatOpt("viewport_opacity", 0.0, 0.95)
	if vib := strings.ToLower(strings.TrimSpace(kv["vibrancy"])); vib != "" {
		switch vib {
		case "auto", "hud", "under-window", "sidebar", "popover", "none":
			cfg.Vibrancy = vib
		default:
			*warnings = append(*warnings, fmt.Sprintf("%s: unsupported vibrancy %q (expected auto|hud|under-window|sidebar|popover|none)", file, vib))
		}
	}
	return cfg
}

func isValidThemeID(id string) bool {
	if len(id) == 0 || len(id) > 48 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
		if !ok {
			return false
		}
	}
	return id[0] != '-' && id[len(id)-1] != '-'
}
