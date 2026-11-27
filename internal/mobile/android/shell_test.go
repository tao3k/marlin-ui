// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package android_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/mobile/android"
)

func TestAndroidMaterialShellSharesMobileContractAndThemeParity(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		FormFactor:     mygoshell.FormFactorAndroid,
		MobilePlatform: mygoshell.MobilePlatformAndroid,
	})
	_, _, _ = featurekit.Mount("all", sh, nil, nil)
	shell := android.New(android.Config{
		Adaptive: sh,
		AppConfig: appconfig.Config{
			SessionID: "android-test-1",
		},
	})
	if shell.Platform() != mobile.PlatformAndroid {
		t.Fatalf("expected PlatformAndroid, got %v", shell.Platform())
	}
	if shell.ActiveTheme().ID != "doom-one" {
		t.Fatalf("expected shared default doom-one theme, got %q", shell.ActiveTheme().ID)
	}
	shell.SetTheme("tokyo-night")
	if shell.ActiveTheme().ID != "doom-tokyo-night" {
		t.Fatalf("expected shared tokyo-night alias resolution, got %q", shell.ActiveTheme().ID)
	}
	shell.SelectTab(mobile.TabRooms)
	snap := shell.RoomSnapshot()
	if !snap.Available || len(snap.Channels) == 0 || len(snap.Participants) == 0 {
		t.Fatalf("expected Android shell to share unified featurekit.RoomController snapshot, got available=%v channels=%d participants=%d",
			snap.Available, len(snap.Channels), len(snap.Participants))
	}

	tt := ui.NewTester(shell.View, 412, 915)
	tt.Frame()
}
