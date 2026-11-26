// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"fmt"
	"runtime"
)

// Release build metadata injected at link time via -ldflags -X.
var (
	Version   = "0.1.0-dev"
	Commit    = "local"
	BuildDate = "unknown"
)

// BuildSummary formats a human-readable single-line version banner for CLI and
// native About panels.
func BuildSummary(target string) string {
	if target == "" {
		target = "marlin-ui"
	}
	return fmt.Sprintf("%s %s (%s, %s, %s/%s)", target, Version, Commit, BuildDate, runtime.GOOS, runtime.GOARCH)
}
