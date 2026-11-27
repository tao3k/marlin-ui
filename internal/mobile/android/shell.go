// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package android implements Marlin's Android Material 3 companion shell under
// internal/mobile, sharing canonical mobile.Tab navigation, Desktop-parity
// Doom One / Tokyo Night themes, and AdaptiveShell Rust host bindings with
// internal/mobile/ios.
package android

import (
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	"github.com/tao3k/marlin-ui/internal/mobile"
	"github.com/tao3k/marlin-ui/internal/shellmanifest"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

// Config configures Marlin's Android Material 3 companion Shell.
type Config struct {
	Adaptive       *mygoshell.AdaptiveShell
	Link           *hostlink.Link
	AppConfig      appconfig.Config
	Prefs          appconfig.UserPrefs
	ThemeBundle    appconfig.UIThemeBundle
	InitialThemeID string
	OnPrefsChanged func(appconfig.UserPrefs)
	Invalidate     func()
}

// Shell owns Marlin's Android Material 3 companion state and renders through
// MyGo's Material look while sharing the unified internal/mobile contract.
type Shell struct {
	mu             sync.Mutex
	Adaptive       *mygoshell.AdaptiveShell
	Link           *hostlink.Link
	AppCfg         appconfig.Config
	Prefs          appconfig.UserPrefs
	Theme            theme.Theme
	ThemeCatalog     []theme.Theme
	assembledSurface uiframework.AssembledSurface
	schemeExts       []schema.UIExtensionRegion
	schemeStatus     []mapper.ResolvedStatusItem
	schemeCmds       []uiframework.MappedCommand
	activeModel      string
	activeTab        mobile.Tab
	onPrefsChanged func(appconfig.UserPrefs)
	invalidate     func()
}

// New constructs an Android Material 3 companion Shell aligned with Desktop
// and iOS theme packs.
func New(cfg Config) *Shell {
	mobile.ApplyPlatformLook(mobile.PlatformAndroid)
	inv := cfg.Invalidate
	if inv == nil {
		inv = func() {}
	}
	catalog := mobile.BuildThemeCatalog(cfg.ThemeBundle)
	selected := catalog[0]
	pref := mobile.NormalizeThemeID(cfg.InitialThemeID)
	if pref == "" {
		pref = mobile.NormalizeThemeID(cfg.ThemeBundle.Manifest.ActiveTheme)
	}
	for _, th := range catalog {
		if th.ID == pref {
			selected = th
			break
		}
	}
	assembled, _ := uiframework.DefaultRegistry().AssembleSurface(shellmanifest.TargetMobileAndroid, nil)
	return &Shell{
		Adaptive:         cfg.Adaptive,
		Link:             cfg.Link,
		AppCfg:           cfg.AppConfig,
		Prefs:            cfg.Prefs,
		Theme:            selected,
		ThemeCatalog:     catalog,
		assembledSurface: assembled,
		activeTab:        mobile.TabChat,
		onPrefsChanged:   cfg.OnPrefsChanged,
		invalidate:       inv,
	}
}

// Platform identifies this shell as Android.
func (s *Shell) Platform() mobile.Platform { return mobile.PlatformAndroid }

// AssembledSurface returns Android's validated modular UI Component surface
// (shared core modules + Android Material 3 modules + slot trust policies).
func (s *Shell) AssembledSurface() uiframework.AssembledSurface {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.assembledSurface
}

// ApplyWorkbenchSnapshot admits a live `marlin.ui.v1` Scheme/Rust workbench
// snapshot into the Android companion via the shared `uiframework.ProjectWorkbench`
// pipeline (identical to Desktop and iOS).
func (s *Shell) ApplyWorkbenchSnapshot(snap schema.UIWorkbenchSnapshot) {
	proj, err := uiframework.ProjectWorkbench(uiframework.DefaultRegistry(), shellmanifest.TargetMobileAndroid, snap)
	if err != nil {
		return
	}
	s.mu.Lock()
	if len(snap.Slots) > 0 {
		s.assembledSurface = proj.Surface
	}
	if len(snap.Extensions) > 0 {
		s.schemeExts = append([]schema.UIExtensionRegion(nil), proj.Extensions...)
	}
	if len(snap.StatusItems) > 0 {
		s.schemeStatus = append([]mapper.ResolvedStatusItem(nil), proj.StatusItems...)
	}
	if len(proj.Commands) > 0 {
		s.schemeCmds = append([]uiframework.MappedCommand(nil), proj.Commands...)
	}
	if strings.TrimSpace(snap.ModelLabel) != "" {
		s.activeModel = strings.TrimSpace(snap.ModelLabel)
	}
	s.mu.Unlock()
	s.invalidate()
}

// SchemeExtensions returns bounds-validated live Scheme UIExtensionRegions.
func (s *Shell) SchemeExtensions() []schema.UIExtensionRegion {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]schema.UIExtensionRegion(nil), s.schemeExts...)
}

// SchemeStatusItems returns mobile-width projected Scheme modeline status items.
func (s *Shell) SchemeStatusItems() []mapper.ResolvedStatusItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mapper.ResolvedStatusItem(nil), s.schemeStatus...)
}

// RoomSnapshot projects the single unified featurekit.RoomController snapshot
// identically to Desktop and iOS (local Core or live Marlin Cove gRPC).
func (s *Shell) RoomSnapshot() featurekit.RoomFullSnapshot {
	return featurekit.AgentRoomSnapshot()
}

// ActiveTheme returns the active theme pack (thread-safe).
func (s *Shell) ActiveTheme() theme.Theme {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Theme
}

// SelectTab updates the active mobile tab.
func (s *Shell) SelectTab(t mobile.Tab) {
	s.mu.Lock()
	changed := s.activeTab != t
	s.activeTab = t
	s.mu.Unlock()
	if changed {
		mygo.Haptics.Selection()
		s.invalidate()
	}
}

// SetTheme switches Android's theme pack live (matching Desktop & iOS 1:1).
func (s *Shell) SetTheme(id string) {
	norm := mobile.NormalizeThemeID(id)
	s.mu.Lock()
	found := s.Theme
	for _, th := range s.ThemeCatalog {
		if th.ID == norm {
			found = th
			break
		}
	}
	s.Theme = found
	s.Prefs.ThemeID = found.ID
	prefs := s.Prefs
	s.mu.Unlock()
	if s.onPrefsChanged != nil {
		s.onPrefsChanged(prefs)
	}
	s.invalidate()
}

// View paints the Android Material 3 companion surface.
func (s *Shell) View(c *ui.Context) {
	s.mu.Lock()
	th := s.Theme
	s.mu.Unlock()
	theme.ApplyToMyGo(c, th)
	if s.Adaptive != nil {
		s.Adaptive.View(c)
	}
}
