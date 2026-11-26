// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// WatchUIThemeDir observes ~/.config/marlin/ui/ using a zero-dependency,
// pure-Go recursive stat fingerprint poller until ctx is cancelled. Callers
// should run this inside an async.StructuredScope so shutdown drains cleanly
// (StructuredCancellation.qnt child-quiescence-barrier).
func WatchUIThemeDir(ctx context.Context, dir string, interval time.Duration, onChange func()) {
	if dir == "" || onChange == nil {
		return
	}
	if interval <= 0 {
		interval = 600 * time.Millisecond
	}
	lastSig := fingerprintUIDir(dir)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sig := fingerprintUIDir(dir)
			if sig != lastSig {
				lastSig = sig
				onChange()
			}
		}
	}
}

func fingerprintUIDir(root string) string {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return ""
	}
	var entries []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && len(entries) > 512 {
				return filepath.SkipDir
			}
			return nil
		}
		if len(entries) >= 512 {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		entries = append(entries, fmt.Sprintf("%s|%d|%d", rel, fi.Size(), fi.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(entries)
	h := sha256.New()
	for _, e := range entries {
		_, _ = h.Write([]byte(e))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
