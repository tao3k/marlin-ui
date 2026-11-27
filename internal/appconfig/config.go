// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package appconfig owns typed startup, gRPC host connection, TLS, feature
// profile, and continuity configuration shared by Marlin Desktop and Mobile.
package appconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// Config configures one Marlin UI runtime launch.
type Config struct {
	ConfigFile     string
	ModelCfg       ModelConfig
	AppServerBin   string
	HostAddr       string
	AuthToken      string
	CoveURL        string
	CoveToken      string
	CoveRoomID     string
	TLSCAFile      string
	TLSCertFile    string
	TLSKeyFile     string
	SessionID      string
	ThemeID        string
	FeaturesSpec   string
	TursoPath      string
	RecoveryFile   string
	PrefsFile      string
	Standalone     bool
	RealSession    bool
	MobilePlatform string // "ios" | "android"
	MobileDevice   string // "iphone16" | "iphone-se" | "pixel9"
}

// DefaultConfig returns production defaults hydrated from environment overrides.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	recPath := strings.TrimSpace(os.Getenv("MARLIN_RECOVERY_STATE_FILE"))
	if recPath == "" && home != "" {
		recPath = filepath.Join(home, ".config", "marlin", "recovery-state.json")
	}
	prefsPath := strings.TrimSpace(os.Getenv("MARLIN_PREFS_FILE"))
	if prefsPath == "" && home != "" {
		prefsPath = filepath.Join(home, ".config", "marlin", "ui-prefs.json")
	}
	turso := strings.TrimSpace(os.Getenv("MARLIN_TURSO_PATH"))
	if turso == "" && home != "" {
		turso = filepath.Join(home, ".local", "share", "marlin", "session.turso")
	}
	coveRoom := strings.TrimSpace(os.Getenv("MARLIN_COVE_ROOM_ID"))
	if coveRoom == "" {
		coveRoom = "cove-main"
	}
	return Config{
		ConfigFile:     strings.TrimSpace(os.Getenv("MARLIN_CONFIG")),
		AppServerBin:   strings.TrimSpace(os.Getenv("MARLIN_APP_SERVER_BIN")),
		HostAddr:       strings.TrimSpace(os.Getenv("MARLIN_HOST")),
		AuthToken:      strings.TrimSpace(os.Getenv("MARLIN_TOKEN")),
		CoveURL:        strings.TrimSpace(os.Getenv("MARLIN_COVE_URL")),
		CoveToken:      strings.TrimSpace(os.Getenv("MARLIN_COVE_TOKEN")),
		CoveRoomID:     coveRoom,
		TLSCAFile:      strings.TrimSpace(os.Getenv("MARLIN_TLS_CA")),
		TLSCertFile:    strings.TrimSpace(os.Getenv("MARLIN_TLS_CERT")),
		TLSKeyFile:     strings.TrimSpace(os.Getenv("MARLIN_TLS_KEY")),
		SessionID:      "marlin-main",
		ThemeID:        "doom-one",
		FeaturesSpec:   "default",
		TursoPath:      turso,
		RecoveryFile:   recPath,
		PrefsFile:      prefsPath,
		RealSession:    true,
		MobilePlatform: "ios",
	}
}

// BearerHeader normalizes AuthToken into an HTTP/2 Authorization header value.
func (c Config) BearerHeader() string {
	tok := strings.TrimSpace(c.AuthToken)
	if tok == "" {
		return ""
	}
	if strings.HasPrefix(tok, "Bearer ") {
		return tok
	}
	return "Bearer " + tok
}

// DiscoverAppServerBinary locates a Rust marlin-agent-core gRPC server binary
// strictly from explicit or installed release locations (zero CWD-relative dev
// target probing):
//  1. Explicit `-app-server` flag
//  2. Explicit `MARLIN_APP_SERVER_BIN` environment variable
//  3. Bundled sidecar next to the running Marlin UI executable (`os.Executable`)
//  4. Installed `marlin-app-server` on system PATH
func DiscoverAppServerBinary(explicit string) (path string, source string) {
	if p := strings.TrimSpace(explicit); p != "" {
		if isExecutable(p) {
			return p, "flag"
		}
	}
	if env := strings.TrimSpace(os.Getenv("MARLIN_APP_SERVER_BIN")); env != "" {
		if isExecutable(env) {
			return env, "env"
		}
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		bundled := filepath.Join(exeDir, "marlin-app-server")
		if isExecutable(bundled) {
			return bundled, "bundled"
		}
	}
	if found, err := lookPath("marlin-app-server"); err == nil {
		return found, "PATH"
	}
	return "", ""
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func lookPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		full := filepath.Join(dir, name)
		if isExecutable(full) {
			return full, nil
		}
	}
	return "", os.ErrNotExist
}
