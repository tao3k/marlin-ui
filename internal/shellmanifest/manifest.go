// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package shellmanifest provides Marlin UI's single declarative cross-platform
// module assembly manifest — mirroring scheme/init.ss `(yuanxu! ...)` and
// scheme/ui-mygo/objects.ss target profile overlays.
//
// Every surface (Desktop macOS/Linux/Windows, iOS, Android) declares which
// shared core modules and platform-specific chrome modules it composes so
// cross-platform parity and shared-vs-target boundaries are auditable in one
// place.
package shellmanifest

// TargetKind mirrors scheme/ui-mygo MygoTargetKind.
type TargetKind string

const (
	TargetDesktopMacOS   TargetKind = "desktop-macos"
	TargetDesktopLinux   TargetKind = "desktop-linux"
	TargetDesktopWindows TargetKind = "desktop-windows"
	TargetMobileIOS      TargetKind = "mobile-ios"
	TargetMobileAndroid  TargetKind = "mobile-android"
)

// ModuleID identifies one declarative Marlin UI capability module aligned with
// scheme/init.ss and scheme/ui-mygo.
type ModuleID string

const (
	// Shared cross-platform modules (consumed identically by Desktop, iOS & Android).
	ModThemes       ModuleID = "ui-themes"
	ModTranscript   ModuleID = "ui-transcript"
	ModHelm         ModuleID = "ui-helm"
	ModAgentRoom    ModuleID = "ui-chatroom"
	ModPromptGate   ModuleID = "ui-promptgate"
	ModModelCatalog ModuleID = "ui-models"

	// Desktop Emacs workbench modules (mirrored 1:1 with scheme/ui-workspaces & scheme/ui-modeline).
	ModWorkspaces     ModuleID = "ui-workspaces"
	ModWindowTree     ModuleID = "ui-window-tree"
	ModDoomModeline   ModuleID = "ui-modeline"
	ModCentaurTabs    ModuleID = "ui-centaur-tabs"
	ModMeow           ModuleID = "ui-meow"
	ModHydra          ModuleID = "ui-hydra"
	ModMinibuffer     ModuleID = "ui-minibuffer"
	ModDashboardBento ModuleID = "ui-dashboard-bento"
	ModWorkflowCanvas ModuleID = "ui-workflow-canvas"

	// Mobile iOS companion modules.
	ModIOSTabBar ModuleID = "ui-ios-tabbar"
	ModIOSSheets ModuleID = "ui-ios-sheets"
	ModIOSBento  ModuleID = "ui-ios-bento"

	// Mobile Android companion modules.
	ModAndroidMaterial3 ModuleID = "ui-android-material3"
	ModAndroidNav       ModuleID = "ui-android-nav"
)

// SurfaceManifest declares the exact module composition for one Marlin UI target.
type SurfaceManifest struct {
	Target        TargetKind
	LayoutKind    string // "desktop-split" | "mobile-stack"
	SharedModules []ModuleID
	TargetModules []ModuleID
}

// SharedCoreModules returns the authoritative modules mounted on EVERY Marlin
// UI surface (Desktop, iOS, and Android).
func SharedCoreModules() []ModuleID {
	return []ModuleID{
		ModThemes,
		ModTranscript,
		ModHelm,
		ModAgentRoom,
		ModPromptGate,
		ModModelCatalog,
	}
}

// ManifestFor returns the declarative SurfaceManifest for target.
func ManifestFor(target TargetKind) SurfaceManifest {
	shared := SharedCoreModules()
	switch target {
	case TargetMobileIOS:
		return SurfaceManifest{
			Target:        target,
			LayoutKind:    "mobile-stack",
			SharedModules: shared,
			TargetModules: []ModuleID{ModIOSTabBar, ModIOSSheets, ModIOSBento},
		}
	case TargetMobileAndroid:
		return SurfaceManifest{
			Target:        target,
			LayoutKind:    "mobile-stack",
			SharedModules: shared,
			TargetModules: []ModuleID{ModAndroidMaterial3, ModAndroidNav},
		}
	default:
		return SurfaceManifest{
			Target:        target,
			LayoutKind:    "desktop-split",
			SharedModules: shared,
			TargetModules: []ModuleID{
				ModWorkspaces,
				ModWindowTree,
				ModDoomModeline,
				ModCentaurTabs,
				ModMeow,
				ModHydra,
				ModMinibuffer,
				ModDashboardBento,
				ModWorkflowCanvas,
			},
		}
	}
}

// AllModules returns the ordered union of SharedModules and TargetModules.
func (m SurfaceManifest) AllModules() []ModuleID {
	out := make([]ModuleID, 0, len(m.SharedModules)+len(m.TargetModules))
	out = append(out, m.SharedModules...)
	out = append(out, m.TargetModules...)
	return out
}

// HasModule reports whether mod is declared on this surface.
func (m SurfaceManifest) HasModule(mod ModuleID) bool {
	for _, x := range m.SharedModules {
		if x == mod {
			return true
		}
	}
	for _, x := range m.TargetModules {
		if x == mod {
			return true
		}
	}
	return false
}
