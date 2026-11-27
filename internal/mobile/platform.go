// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package mobile defines the shared Mobile UI platform contract for Marlin's
// iOS (Apple iOS 26 Liquid Glass / Cupertino) and Android (Material 3) clients.
// Cross-mobile navigation tabs, swipe window order, theme catalog resolution,
// and platform look initialization live here so iOS and Android stay aligned
// without duplicating business or theme state.
package mobile

import (
	"strings"

	mygomobile "github.com/egoist/mygo/ui/mobile"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// Platform identifies a target mobile operating system family.
type Platform uint8

const (
	PlatformIOS Platform = iota
	PlatformAndroid
)

// String returns the canonical lowercase identifier for Platform.
func (p Platform) String() string {
	if p == PlatformAndroid {
		return "android"
	}
	return "ios"
}

// ApplyPlatformLook configures MyGo's mobile widget library look (Cupertino on
// iOS, Material 3 on Android).
func ApplyPlatformLook(p Platform) {
	if p == PlatformAndroid {
		mygomobile.SetLook(mygomobile.Material)
		return
	}
	mygomobile.SetLook(mygomobile.Cupertino)
}

// Tab identifies one top-level Mobile companion destination shared across iOS
// and Android shells.
type Tab uint8

const (
	TabChat Tab = iota
	TabRooms
	TabHome
	TabSearch
	TabSettings
	TabGates
)

// TabSpec describes a mobile tab's title and icon symbol for both native system
// tab bars (iOS UITabBarController / Android BottomNavigation) and adaptive
// floating navigation bars.
type TabSpec struct {
	Tab    Tab
	Title  string
	Symbol string
}

// Tabs lists the canonical Mobile companion destinations (Chat, Rooms, Bento,
// Search, Style). Pending approval gates badge on Bento and open TabGates.
var Tabs = []TabSpec{
	{Tab: TabChat, Title: "Chat", Symbol: "bubble.left.and.bubble.right"},
	{Tab: TabRooms, Title: "Rooms", Symbol: "person.3"},
	{Tab: TabHome, Title: "Bento", Symbol: "square.grid.2x2"},
	{Tab: TabSearch, Title: "Search", Symbol: "magnifyingglass"},
	{Tab: TabSettings, Title: "Style", Symbol: "slider.horizontal.3"},
}

// SwipeableTabs defines the canonical horizontal gesture window order shared
// across mobile shells: Chat <-> Rooms (immersive) <-> Bento <-> Style.
var SwipeableTabs = []Tab{TabChat, TabRooms, TabHome, TabSettings}

// SwipePageIndex returns the index of t in SwipeableTabs, or -1 if t is a
// focused full-screen destination (such as TabSearch or TabGates).
func SwipePageIndex(t Tab) int {
	for i, st := range SwipeableTabs {
		if st == t {
			return i
		}
	}
	return -1
}

// NormalizeThemeID canonicalizes friendly theme aliases (e.g. "tokyo-night")
// to their registered CSS theme pack IDs.
func NormalizeThemeID(raw string) string {
	norm := strings.ToLower(strings.TrimSpace(raw))
	if norm == "tokyo-night" || norm == "tokyo" || norm == "tokyonight" {
		return "doom-tokyo-night"
	}
	return norm
}

// BuildThemeCatalog merges built-in Doom One / Tokyo Night / Nord / Light packs
// with any governed user CSS packs so Mobile mirrors Desktop 1:1.
func BuildThemeCatalog(bundle appconfig.UIThemeBundle) []theme.Theme {
	catalog := theme.Catalog()
	for _, pack := range bundle.Packs {
		base := theme.NordBento()
		if pack.IsLight {
			base = theme.DoomOneLight()
		}
		if built, err := theme.BuildFromCSS(pack.ID, pack.Name, pack.IsLight, &base, pack.StyleCSS); err == nil {
			replaced := false
			for i := range catalog {
				if catalog[i].ID == built.ID {
					catalog[i] = built
					replaced = true
					break
				}
			}
			if !replaced {
				catalog = append(catalog, built)
			}
		}
	}
	return catalog
}

// ShortModelName extracts a compact display model alias for mobile headers.
func ShortModelName(full string) string {
	s := strings.TrimSpace(full)
	if idx := strings.IndexByte(s, ' '); idx > 0 {
		s = s[:idx]
	}
	if prov, model, ok := strings.Cut(s, "/"); ok && model != "" {
		if strings.HasPrefix(strings.ToLower(model), strings.ToLower(prov)) || len(model) >= 8 {
			return model
		}
	}
	return s
}
