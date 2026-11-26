// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tao3k/marlin-mygo/runtime/style"
)

// MaxThemePackCSSBytes caps remote theme pack downloads fail-closed.
const MaxThemePackCSSBytes = style.DefaultMaxBytes

// ThemePackInstallSpec declares an offline-first, SHA-256-pinned remote or
// local CSS theme pack installed only upon explicit user invocation.
type ThemePackInstallSpec struct {
	ID      string
	Name    string
	Variant string // "dark" | "light"
	Base    string
	URL     string // https:// or file://
	SHA256  string // required 64-hex lowercase SHA-256 digest of style.css
}

// InstallThemePack fetches spec.URL (HTTPS or local file://), verifies its
// exact SHA-256 digest and CSS safety contract fail-closed, and atomically
// writes ~/.config/marlin/ui/themes/<id>/{theme.toml, style.css}.
func InstallThemePack(ctx context.Context, uiDir string, spec ThemePackInstallSpec) (string, error) {
	id := strings.ToLower(strings.TrimSpace(spec.ID))
	if !isValidThemeID(id) {
		return "", fmt.Errorf("invalid theme pack id %q", spec.ID)
	}
	wantSum := strings.ToLower(strings.TrimSpace(spec.SHA256))
	if len(wantSum) != 64 {
		return "", fmt.Errorf("theme pack %q requires a 64-hex sha256 pin (got %d chars)", id, len(wantSum))
	}
	if _, err := hex.DecodeString(wantSum); err != nil {
		return "", fmt.Errorf("invalid sha256 hex for theme pack %q: %w", id, err)
	}

	rawBytes, err := fetchBoundedThemeSource(ctx, strings.TrimSpace(spec.URL))
	if err != nil {
		return "", err
	}
	gotHash := sha256.Sum256(rawBytes)
	gotSum := hex.EncodeToString(gotHash[:])
	if gotSum != wantSum {
		return "", fmt.Errorf("theme pack %q sha256 mismatch: expected %s, got %s (rejected fail-closed)", id, wantSum, gotSum)
	}

	css := string(rawBytes)
	if _, err := style.Parse(css, style.DefaultContract()); err != nil {
		return "", fmt.Errorf("theme pack %q failed CSS contract validation: %w", id, err)
	}

	packDir := filepath.Join(uiDir, "themes", id)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = id
	}
	variant := strings.ToLower(strings.TrimSpace(spec.Variant))
	if variant != "light" {
		variant = "dark"
	}
	base := strings.TrimSpace(spec.Base)
	if base == "" {
		base = "nord-bento"
	}
	meta := fmt.Sprintf("id = %q\nname = %q\nvariant = %q\nbase = %q\nsource_url = %q\nsha256 = %q\n",
		id, name, variant, base, strings.TrimSpace(spec.URL), wantSum)

	if err := writeAtomicFile(filepath.Join(packDir, "theme.toml"), []byte(meta)); err != nil {
		return "", err
	}
	if err := writeAtomicFile(filepath.Join(packDir, "style.css"), rawBytes); err != nil {
		return "", err
	}
	return packDir, nil
}

func fetchBoundedThemeSource(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid theme_url %q: %w", rawURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "file":
		p := u.Path
		data, err := readBoundedFile(p, MaxThemePackCSSBytes)
		if err != nil {
			return nil, err
		}
		return []byte(data), nil
	case "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("theme_url HTTP %d", resp.StatusCode)
		}
		limited := io.LimitReader(resp.Body, int64(MaxThemePackCSSBytes)+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, err
		}
		if len(data) > MaxThemePackCSSBytes {
			return nil, fmt.Errorf("theme_url payload exceeds max allowed size (%d bytes)", MaxThemePackCSSBytes)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported theme_url scheme %q (only https:// or file:// permitted)", u.Scheme)
	}
}

func writeAtomicFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".marlin-theme-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
