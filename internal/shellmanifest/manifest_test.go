// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package shellmanifest_test

import (
	"testing"

	"github.com/tao3k/marlin-ui/internal/shellmanifest"
)

func TestSurfaceManifestDeclaresSharedCoreAndPlatformDeltas(t *testing.T) {
	targets := []shellmanifest.TargetKind{
		shellmanifest.TargetDesktopMacOS,
		shellmanifest.TargetMobileIOS,
		shellmanifest.TargetMobileAndroid,
	}
	for _, tk := range targets {
		m := shellmanifest.ManifestFor(tk)
		for _, shared := range shellmanifest.SharedCoreModules() {
			if !m.HasModule(shared) {
				t.Fatalf("surface %s missing shared core module %s", tk, shared)
			}
		}
	}

	desk := shellmanifest.ManifestFor(shellmanifest.TargetDesktopMacOS)
	for _, want := range []shellmanifest.ModuleID{
		shellmanifest.ModWorkspaces,
		shellmanifest.ModWindowTree,
		shellmanifest.ModDoomModeline,
	} {
		if !desk.HasModule(want) {
			t.Fatalf("desktop manifest missing %s", want)
		}
	}

	ios := shellmanifest.ManifestFor(shellmanifest.TargetMobileIOS)
	if ios.LayoutKind != "mobile-stack" || !ios.HasModule(shellmanifest.ModAgentRoom) || ios.HasModule(shellmanifest.ModWindowTree) {
		t.Fatalf("unexpected iOS manifest composition: %+v", ios)
	}
}
