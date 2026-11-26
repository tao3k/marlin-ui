// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SavedRemote records a non-secret remote Marlin Core gRPC endpoint added by
// the user. Authentication tokens are intentionally never persisted here.
type SavedRemote struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Endpoint    string `json:"endpoint"`
}

// UserPrefs stores non-secret local UI preferences persisted atomically across
// Desktop and Mobile launches.
type UserPrefs struct {
	ThemeID           string        `json:"theme_id,omitempty"`
	FeaturesSpec      string        `json:"features_spec,omitempty"`
	SidebarCollapsed  bool          `json:"sidebar_collapsed,omitempty"`
	Perspective       string        `json:"perspective,omitempty"`
	LastSessionID     string        `json:"last_session_id,omitempty"`
	DefaultRemoteHost string        `json:"default_remote_host,omitempty"`
	RealSession       *bool         `json:"real_session,omitempty"`
	SavedRemotes      []SavedRemote `json:"saved_remotes,omitempty"`
}

// LoadPrefs reads persisted UI preferences if present. Missing files return a
// zero UserPrefs and nil error.
func LoadPrefs(path string) (UserPrefs, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return UserPrefs{}, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return UserPrefs{}, nil
		}
		return UserPrefs{}, err
	}
	var prefs UserPrefs
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return UserPrefs{}, err
	}
	return prefs, nil
}

// SavePrefs writes UI preferences atomically (temp file + fsync + rename) with
// owner-only permissions (0600).
func SavePrefs(path string, prefs UserPrefs) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".marlin-ui-prefs-*.tmp")
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
	if err := os.Rename(tmpPath, p); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// UpsertRemote inserts or updates a remote endpoint by ID without storing any
// credentials.
func (p *UserPrefs) UpsertRemote(r SavedRemote) {
	if p == nil || strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Endpoint) == "" {
		return
	}
	for i := range p.SavedRemotes {
		if p.SavedRemotes[i].ID == r.ID {
			p.SavedRemotes[i] = r
			return
		}
	}
	p.SavedRemotes = append(p.SavedRemotes, r)
}
