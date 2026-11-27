// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package ios implements Marlin's flagship iOS 26 Liquid Glass companion
// workbench under internal/mobile, sharing canonical mobile.Tab navigation,
// swipeable window ordering, and Desktop-parity theme packs with Android.
package ios

import (
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	mob "github.com/tao3k/marlin-ui/internal/mobile"
)

// Tab aliases the shared internal/mobile.Tab destination contract.
type Tab = mob.Tab

// TabSpec aliases the shared internal/mobile.TabSpec contract.
type TabSpec = mob.TabSpec

const (
	TabChat     = mob.TabChat
	TabRooms    = mob.TabRooms
	TabHome     = mob.TabHome
	TabSearch   = mob.TabSearch
	TabSettings = mob.TabSettings
	TabGates    = mob.TabGates
)

// Tabs references the canonical shared Mobile companion tab list.
var Tabs = mob.Tabs

// sfIcon renders a native Apple CoreGraphics SF Symbol with SwiftUI optical
// sizing (.Natural), optional .Fill() variant, and crisp SVG fallback.
func sfIcon(c *ui.Context, name string, size float32, col ui.Color, filled bool) ui.Element {
	el := sfsymbols.Symbol(c, name).Size(size).Color(col).Weight(sfsymbols.Medium).Natural()
	if filled {
		el = el.Fill()
	}
	return el.Element
}

// Config configures Marlin's iOS 26 Liquid Glass companion Shell.
type Config struct {
	Adaptive       *mygoshell.AdaptiveShell
	Link           *hostlink.Link
	AppConfig      appconfig.Config
	Prefs          appconfig.UserPrefs
	ThemeBundle    appconfig.UIThemeBundle
	InitialThemeID string
	NativeTabBar   bool // true on GOOS=ios when Swift UITabBarController owns bottom tabs
	OnThemeChanged func(isLight bool)
	OnPrefsChanged func(appconfig.UserPrefs)
	OnBadgeChanged func(tabIndex int, badge string)
	Invalidate     func()
}

type helmSnap struct {
	hadPrimary bool
	rows       []search.VisibleRow
	srcs       []search.SourceSummary
	total      int
}

// Shell owns Marlin's iOS 26 Liquid Glass companion state and 120Hz GPU views.
type Shell struct {
	mu           sync.Mutex
	Adaptive     *mygoshell.AdaptiveShell
	Link         *hostlink.Link
	AppCfg       appconfig.Config
	Prefs        appconfig.UserPrefs
	NativeTabBar bool

	Theme        theme.Theme
	ThemeCatalog []theme.Theme
	Surface      appconfig.SurfaceConfig
	GlassEnabled bool
	GlassOpacity float32
	MotionOn     bool

	activeTab            Tab
	pageState            mobile.PageState
	chatDraft            string
	deepThink            bool
	roomChanSheetOpen    bool
	roomDetailsSheetOpen bool
	roomThreadSheetOpen  bool
	roomDetailsTabIdx    int
	roomMentionForceOpen bool
	roomDraft            string
	roomThreadDraft      string
	roomTaskDraft        string
	searchQuery          string
	selectedHit    *search.Candidate
	helmRowsBuf    []search.VisibleRow
	helmLatch      *theme.FrameLatch[helmSnap]
	thinkingOpen   map[string]bool
	activeModel    string
	statusBanner   string
	lastGateBadge  string
	onThemeChanged func(bool)
	onPrefsChanged func(appconfig.UserPrefs)
	onBadgeChanged func(int, string)
	invalidate     func()
}

// New constructs an iOS 26 Liquid Glass companion Shell aligned with Desktop's
// theme catalog and declarative [surface] TOML configuration.
func New(cfg Config) *Shell {
	// Configure MyGo's mobile widget library for Apple Cupertino HIG via shared internal/mobile.
	mob.ApplyPlatformLook(mob.PlatformIOS)

	inv := cfg.Invalidate
	if inv == nil {
		inv = func() {}
	}
	catalog := mob.BuildThemeCatalog(cfg.ThemeBundle)
	selected := catalog[0]
	pref := mob.NormalizeThemeID(cfg.InitialThemeID)
	if pref == "" {
		pref = mob.NormalizeThemeID(cfg.ThemeBundle.Manifest.ActiveTheme)
	}
	for _, th := range catalog {
		if th.ID == pref {
			selected = th
			break
		}
	}

	surf := appconfig.MergeSurfaceConfig(appconfig.DefaultSurfaceConfig(), cfg.AppConfig.ModelCfg.Surface)
	surf = appconfig.MergeSurfaceConfig(surf, cfg.ThemeBundle.Manifest.Surface)
	glassOn := true
	if surf.Glass != nil {
		glassOn = *surf.Glass
	}
	motionOn := true
	if surf.Motion != nil {
		motionOn = *surf.Motion
	}
	var op float32 = 0.75
	if surf.ViewportOpacity != nil && *surf.ViewportOpacity > 0 {
		op = *surf.ViewportOpacity
	}
	activeModel := ""
	if cfg.AppConfig.ModelCfg.Loaded {
		activeModel = cfg.AppConfig.ModelCfg.SummaryLabel()
	}

	return &Shell{
		Adaptive:       cfg.Adaptive,
		Link:           cfg.Link,
		AppCfg:         cfg.AppConfig,
		Prefs:          cfg.Prefs,
		NativeTabBar:   cfg.NativeTabBar,
		Theme:          selected,
		ThemeCatalog:   catalog,
		Surface:        surf,
		GlassEnabled:   glassOn,
		GlassOpacity:   op,
		MotionOn:       motionOn,
		activeTab:      TabChat,
		helmRowsBuf:    make([]search.VisibleRow, 0, 12),
		helmLatch:      theme.NewFrameLatch[helmSnap](theme.DefaultLatchGrace),
		thinkingOpen:   make(map[string]bool),
		activeModel:    activeModel,
		onThemeChanged: cfg.OnThemeChanged,
		onPrefsChanged: cfg.OnPrefsChanged,
		onBadgeChanged: cfg.OnBadgeChanged,
		invalidate:     inv,
	}
}

// Platform identifies this shell as Apple iOS.
func (s *Shell) Platform() mob.Platform { return mob.PlatformIOS }

// ActiveTheme returns the current theme pack (thread-safe).
func (s *Shell) ActiveTheme() theme.Theme {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Theme
}

var swipeableTabs = mob.SwipeableTabs

func swipePageIndex(t Tab) int { return mob.SwipePageIndex(t) }

// SelectTab switches the active tab and triggers native iOS selection haptics.
func (s *Shell) SelectTab(t Tab) {
	s.mu.Lock()
	changed := s.activeTab != t
	s.activeTab = t
	if idx := swipePageIndex(t); idx >= 0 {
		s.pageState.Page = idx
	}
	s.mu.Unlock()
	if changed {
		mygo.Haptics.Selection()
		if t == TabSearch && s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.HelmSearch != nil {
			s.mu.Lock()
			q := s.searchQuery
			s.mu.Unlock()
			gen := s.Adaptive.Desktop.HelmSearch.SetQuery(q)
			s.helmLatch.BeginTransition(gen)
		}
		s.invalidate()
	}
}

// SetTheme switches Marlin iOS's theme pack live (matching Desktop & Android 1:1).
func (s *Shell) SetTheme(id string) {
	norm := mob.NormalizeThemeID(id)
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
	mygo.Haptics.Light()
	if s.onThemeChanged != nil {
		s.onThemeChanged(found.IsLight)
	}
	if s.onPrefsChanged != nil {
		s.onPrefsChanged(prefs)
	}
	s.invalidate()
}

// SetWindowOpacity updates the unified Liquid Glass background opacity live.
func (s *Shell) SetWindowOpacity(op float32) {
	if op < 0.45 {
		op = 0.45
	}
	if op > 0.96 {
		op = 0.96
	}
	s.mu.Lock()
	s.GlassOpacity = op
	s.mu.Unlock()
	mygo.Haptics.Selection()
	s.invalidate()
}

// SelectModel dispatches a live model route switch to the Rust host.
func (s *Shell) SelectModel(alias string) {
	a := strings.TrimSpace(alias)
	if a == "" {
		return
	}
	s.mu.Lock()
	s.activeModel = a
	s.statusBanner = "Model: " + a
	s.mu.Unlock()
	mygo.Haptics.Medium()
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.OnCommandInvoke != nil {
		s.Adaptive.Desktop.OnCommandInvoke("model.select " + a)
	}
	s.invalidate()
}

// View paints the complete single-window iOS 26 Liquid Glass shell (used on
// macOS dev preview with floating Liquid Glass capsule Tab Bar).
func (s *Shell) View(c *ui.Context) {
	s.mu.Lock()
	tab := s.activeTab
	s.mu.Unlock()
	s.renderScreen(c, tab, !s.NativeTabBar)
}

// TabView returns a dedicated ui.View bound to one tab for native Swift
// UITabBarController hosting via mygo.NewTabs on GOOS=ios.
func (s *Shell) TabView(tab Tab) func(*ui.Context) {
	return func(c *ui.Context) {
		s.renderScreen(c, tab, false)
	}
}

func (s *Shell) renderScreen(c *ui.Context, tab Tab, showFloatingTabBar bool) {
	s.mu.Lock()
	th := s.Theme
	glassOn := s.GlassEnabled
	winOp := s.GlassOpacity
	s.mu.Unlock()

	// Project Marlin's active CSS theme into MyGo's native UI library theme so
	// ui.Button, ui.PrimaryButton, ui.Segmented, ui.SearchField, ui.Collapsible,
	// mobile.Section, mobile.ListRow, and mobile.Switch inherit Apple iOS 26
	// Liquid Glass depth and authentic Doom One / Tokyo Night accents.
	mygoTheme := th.ToMyGoTheme()
	mygoTheme.Font = kit.FontDisplay
	mygoTheme.FontSize = 14
	mygoTheme.Radius = 12
	if glassOn && c.Vibrancy() {
		mygoTheme.Surface = theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.74)
		mygoTheme.SurfaceHover = theme.GlassTone(c, th.SurfaceHover, th, 0.56, 0.84)
		mygoTheme.SurfacePressed = theme.GlassTone(c, th.Bar, th, 0.52, 0.80)
		if th.IsLight {
			mygoTheme.Border = kit.WithAlpha(ui.RGB(0, 0, 0), 0.11)
		} else {
			mygoTheme.Border = kit.WithAlpha(ui.RGB(255, 255, 255), 0.11)
		}
	}
	c.SetTheme(mygoTheme)

	// Sync native Swift UITabBar badge count when running inside UITabBarController.
	gateCount := s.pendingGateCount()
	badgeStr := ""
	if gateCount > 0 {
		badgeStr = fmt.Sprintf("%d", gateCount)
	}
	s.mu.Lock()
	if badgeStr != s.lastGateBadge {
		s.lastGateBadge = badgeStr
		cb := s.onBadgeChanged
		s.mu.Unlock()
		if cb != nil {
			cb(int(TabHome), badgeStr)
		}
	} else {
		s.mu.Unlock()
	}

	// Unified frosted Liquid Glass canvas: softly blurs background windows while
	// preserving luminous Nordic depth and razor-sharp APCA text legibility.
	rootBg := th.Canvas
	if glassOn && c.Vibrancy() {
		kit.ApplyWindowTransparency(c, true)
		effOp := winOp
		if effOp < 0.78 {
			effOp = 0.78
		}
		rootBg = kit.WithAlpha(th.Canvas, effOp)
	}

	root := ui.Column(c.Key("ios-shell-root")).Fill().Background(rootBg).AlignItems(ui.Stretch).Clip()
	root.Children(func() {
		// Subtle cool oceanic aurora wash near the top of the viewport (Doom One
		// blue / Tokyo Night sapphire) so Liquid Glass capsules refract crisp light.
		ui.Box(c.Key("ios-ambient-wash")).Absolute().Top(0).Left(0).Right(0).Height(260).PassThrough().
			LinearGradient(ui.LinearGradient{
				From:  kit.WithAlpha(th.Accent, 0.10),
				To:    ui.Transparent,
				Angle: 180,
			})

		// Native MyGo mobile.PageView: strictly 1 page per swipe with smooth native
		// spring physics (zero manual ScrollState teleportation or momentum fighting).
		usePager := showFloatingTabBar && swipePageIndex(tab) >= 0
		if usePager {
			if tb := c.TitleBar(); tb.Height > 0 {
				ui.Row(c.Key("ios-titlebar-drag")).Height(tb.Height).Background(ui.Transparent).DragWindow()
			}
			// Optional desktop-preview keyboard navigation (Cmd+1..4) without any scroll hacks.
			root.OnShortcut(ui.Super, ui.Key1, func() { s.SelectTab(TabChat) })
			root.OnShortcut(ui.Super, ui.Key2, func() { s.SelectTab(TabRooms) })
			root.OnShortcut(ui.Super, ui.Key3, func() { s.SelectTab(TabHome) })
			root.OnShortcut(ui.Super, ui.Key4, func() { s.SelectTab(TabSettings) })

			prevPage := s.pageState.Page
			mobile.PageView(c.Key("ios-win-pager"), &s.pageState, len(swipeableTabs), func(i int) {
				pageTab := swipeableTabs[i]
				pageHasBar := showFloatingTabBar && pageTab != TabRooms
				ui.Column(c.Key(fmt.Sprintf("ios-page-%d", i))).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
					s.renderTopHeader(c, th, pageTab, true)
					ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
						switch pageTab {
						case TabRooms:
							s.renderAgentRoom(c, th, pageHasBar)
						case TabHome:
							s.renderBentoHome(c, th, pageHasBar)
						case TabSettings:
							s.renderSettingsView(c, th, pageHasBar)
						default:
							s.renderChatView(c, th, pageHasBar)
						}
					})
				})
			}).Grow(1).MinWidth(0).MinHeight(0)
			if s.pageState.Page != prevPage && s.pageState.Page >= 0 && s.pageState.Page < len(swipeableTabs) {
				nextTab := swipeableTabs[s.pageState.Page]
				s.mu.Lock()
				s.activeTab = nextTab
				s.mu.Unlock()
				tab = nextTab
				mygo.Haptics.Selection()
			}
		} else {
			// Seamless top safe-area / macOS traffic-light drag bar + iOS 26 header.
			s.renderTopHeader(c, th, tab, false)

			// Main scrollable / flexible content body (TabSearch, TabGates, or native Swift UITabBarController).
			ui.Column(c.Key("ios-body-host")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
				pageHasBar := showFloatingTabBar && tab != TabRooms
				switch tab {
				case TabRooms:
					s.renderAgentRoom(c, th, pageHasBar)
				case TabHome:
					s.renderBentoHome(c, th, pageHasBar)
				case TabSearch:
					s.renderMobileSearch(c, th, pageHasBar)
				case TabGates:
					s.renderGatesView(c, th, pageHasBar)
				case TabSettings:
					s.renderSettingsView(c, th, pageHasBar)
				default:
					s.renderChatView(c, th, pageHasBar)
				}
			})
		}

		// iOS 26 Floating Liquid Glass Capsule Tab Bar + refined Search Orb.
		// Auto-hides inside immersive Agent Rooms (swipe left/right to switch windows!).
		if showFloatingTabBar && tab != TabRooms {
			s.renderFloatingGlassTabBar(c, th, tab, gateCount)
		}
	})
}

func (s *Shell) renderTopHeader(c *ui.Context, th theme.Theme, tab Tab, dragHandled bool) {
	tb := c.TitleBar()
	topPad := float32(14)
	if tb.Height > 0 {
		if !dragHandled {
			// macOS dev preview: clean invisible drag strip clearing traffic lights.
			ui.Row(c.Key("ios-titlebar-drag")).Height(tb.Height).Background(ui.Transparent).DragWindow()
		}
		topPad = 6
	}

	// Discord Mobile spatial layout: Rooms owns a single unified channel
	// navigation header (#channel › + drawer + live AI avatar stack) instead
	// of stacking a redundant generic "Rooms" bar above it.
	if tab == TabRooms {
		return
	}

	title := "Marlin"
	if tab == TabGates {
		title = "Approvals"
	} else {
		for _, spec := range Tabs {
			if spec.Tab == tab && tab != TabChat {
				title = spec.Title
				break
			}
		}
	}
	modelLbl := shortModelName(s.currentModelLabel())
	onlineCol := th.Success
	if s.Link == nil || s.Link.Mode == hostlink.ModeStandalone {
		onlineCol = th.Warning
	}

	ui.Row(c.Key("ios-top-header")).Padding(topPad, 16, 10, 16).Gap(8).AlignItems(ui.Center).Clip().Children(func() {
		if tab == TabChat {
			theme.MarlinEmblem(c, "ios-hdr-marlin", 26, th)
		}
		ui.Text(c, title).Font(kit.FontDisplay).FontSize(26).FontWeight(700).LetterSpacing(-0.5).TextColor(th.Ink).
			SingleLine().Ellipsis("…").MinWidth(0).Clip()
		ui.Spacer(c)

		// Apple iOS 26 glass.Group merging New Chat (on Chat tab), Model Pill,
		// and Theme Toggle into one cohesive Liquid Glass control cluster.
		grp := glass.NewGroup(c)
		grp.Radius = 16
		grp.Tint = kit.WithAlpha(th.SurfaceElevated, 0.28)
		grp.Gap(6).AlignItems(ui.Center).Children(func() {
			if tab == TabChat {
				newBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-hdr-new-chat")).Label("New Chat").
					Width(30).Height(30).Radius(15).AlignItems(ui.Center).Justify(ui.Center))
				newBtn.Children(func() {
					sfIcon(c, "square.and.pencil", 13.5, th.Ink, false)
				})
				grp.Pane(newBtn)
				if newBtn.Clicked() {
					s.NewChat()
				}
			}

			pill := kit.ActionControl(ui.ButtonBase(c.Key("ios-hdr-model-pill")).Label(modelLbl).
				Height(30).Padding(0, 11).Gap(6).Radius(15).AlignItems(ui.Center))
			pill.Children(func() {
				ui.Box(c).Width(6.5).Height(6.5).Shrink(0).Radius(theme.RadiusPill).Background(onlineCol)
				ui.Text(c, modelLbl).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Ink).
					SingleLine().Ellipsis("…").MaxWidth(124).MinWidth(0).Clip()
			})
			grp.Pane(pill)
			if pill.Clicked() {
				s.SelectTab(TabSettings)
			}

			themeBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-hdr-theme-cycle")).Label("Cycle theme").
				Width(30).Height(30).Radius(15).AlignItems(ui.Center).Justify(ui.Center))
			themeBtn.Children(func() {
				sym := theme.IconThemeDark
				if th.IsLight {
					sym = theme.IconThemeLight
				}
				sfIcon(c, sym, 13, th.Accent, true)
			})
			grp.Pane(themeBtn)
			if themeBtn.Clicked() {
				s.cycleTheme()
			}
		})
	})
}

func (s *Shell) renderFloatingGlassTabBar(c *ui.Context, th theme.Theme, active Tab, gateCount int) {
	// Soft iOS 26 scroll edge fade rising beneath the floating capsule tab bar.
	ui.Box(c.Key("ios-tab-scroll-edge")).Absolute().Left(0).Right(0).Bottom(0).Height(88).PassThrough().
		Material(glass.ScrollEdge{Bottom: true, Background: kit.WithAlpha(th.Canvas, 0.82)})

	rimBorder := kit.WithAlpha(ui.RGB(255, 255, 255), 0.15)
	selFill := kit.WithAlpha(ui.RGB(255, 255, 255), 0.13)
	selBorder := kit.WithAlpha(ui.RGB(255, 255, 255), 0.22)
	hoverFill := kit.WithAlpha(ui.RGB(255, 255, 255), 0.06)
	barTint := kit.WithAlpha(th.Bar, 0.44)
	if th.IsLight {
		rimBorder = kit.WithAlpha(ui.RGB(0, 0, 0), 0.10)
		selFill = kit.WithAlpha(ui.RGB(0, 0, 0), 0.07)
		selBorder = kit.WithAlpha(ui.RGB(0, 0, 0), 0.10)
		hoverFill = kit.WithAlpha(ui.RGB(0, 0, 0), 0.04)
		barTint = kit.WithAlpha(ui.RGB(255, 255, 255), 0.56)
	}

	// Iconic Apple iOS 26 Split Floating Liquid Glass Tab Bar:
	// Left: 4-tab floating refractive glass capsule (Chat · Rooms · Bento · Style)
	// Right: Refined 48x48 circular Liquid Glass Search Orb (iOS 26 Tab(role: .search))
	ui.Row(c.Key("ios-tab-dock")).Absolute().Left(14).Right(14).Bottom(14).Gap(8).AlignItems(ui.Center).Children(func() {
		bar := ui.Row(c.Key("ios-tab-capsule")).Basis(0).Grow(1).MinWidth(0).Height(52).Padding(4, 5).Gap(2).
			Radius(26).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Material(glass.Glass{
				Style:       glass.Regular,
				Tint:        barTint,
				Interactive: false,
			}).
			Shadow(0, 10, 26, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.36))
		if !c.Vibrancy() {
			bar.Border(1, rimBorder)
		}

		bar.Children(func() {
			for _, spec := range Tabs {
				if spec.Tab == TabSearch {
					continue
				}
				sp := spec
				isSel := sp.Tab == active
				btn := kit.ActionControl(ui.ButtonBase(c.Key("iostab-"+sp.Title)).Label(sp.Title).
					Basis(0).Grow(1).MinWidth(0).Height(44).Radius(22).Gap(2).
					Column().AlignItems(ui.Center).Justify(ui.Center))
				if isSel {
					btn.Background(selFill).
						Shadow(0, 2, 8, -2, kit.WithAlpha(ui.RGB(0, 0, 0), 0.16))
					if !c.Vibrancy() {
						btn.Border(1, selBorder)
					}
				} else if btn.Hovered() {
					btn.Background(hoverFill)
				}
				btn.Children(func() {
					col := th.InkMuted
					weight := 500
					if isSel {
						col = th.Accent
						weight = 600
					}
					ui.Box(c).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						sfIcon(c, sp.Symbol, 16.5, col, isSel)
						if sp.Tab == TabHome && gateCount > 0 {
							ui.Text(c, fmt.Sprintf("%d", gateCount)).
								Absolute().Top(-5).Right(-11).
								Padding(1, 4.5).Radius(theme.RadiusPill).
								Background(th.Danger).TextColor(ui.RGB(255, 255, 255)).
								Font(kit.FontDisplay).FontSize(9).FontWeight(700).SingleLine()
						}
					})
					ui.Text(c, sp.Title).Font(kit.FontDisplay).FontSize(10).FontWeight(weight).TextColor(col).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
				})
				if btn.Clicked() {
					s.SelectTab(sp.Tab)
				}
			}
		})

		// Refined iOS 26 Circular Liquid Glass Search Orb (48x48, crisp 18pt glyph).
		searchActive := active == TabSearch
		orbVariant := theme.BtnGlass
		orbTone := th.Ink
		if searchActive {
			orbVariant = theme.BtnGlassProminent
			orbTone = th.Accent
		}
		if theme.IconCircleButton(c, "iostab-Search", "Search", theme.IconSearch, 48, orbVariant, orbTone, false, th).Clicked() {
			s.SelectTab(TabSearch)
		}
	})
}

func shortModelName(full string) string { return mob.ShortModelName(full) }

func (s *Shell) cycleTheme() {
	s.mu.Lock()
	next := s.Theme.ID
	for i, th := range s.ThemeCatalog {
		if th.ID == s.Theme.ID {
			next = s.ThemeCatalog[(i+1)%len(s.ThemeCatalog)].ID
			break
		}
	}
	s.mu.Unlock()
	s.SetTheme(next)
}

func (s *Shell) currentModelLabel() string {
	s.mu.Lock()
	m := strings.TrimSpace(s.activeModel)
	s.mu.Unlock()
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.Workbench != nil {
		if snap, ok := s.Adaptive.Desktop.Workbench.Snapshot(); ok && snap != nil && strings.TrimSpace(snap.ModelLabel) != "" {
			return strings.TrimSpace(snap.ModelLabel)
		}
	}
	if m != "" {
		return m
	}
	return "marlin-core"
}

func (s *Shell) pendingGateCount() int {
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.PromptGate != nil && s.Adaptive.Desktop.PromptGate.Queue != nil {
		if _, _, total, ok := s.Adaptive.Desktop.PromptGate.Queue.Active(); ok {
			return total
		}
	}
	return 0
}

// NewChat creates a brand-new isolated Marlin session with a clean empty
// conversation stage (preserving previous sessions in history) and switches to TabChat.
func (s *Shell) NewChat() {
	mygo.Haptics.Medium()
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		if s.Adaptive.Desktop.OnNewSession != nil {
			s.Adaptive.Desktop.OnNewSession()
		} else {
			// Standalone/unit-test fallback without hostlink: mount a fresh empty transcript.
			prev := s.Adaptive.Desktop.Transcript
			fresh := mygoshell.NewTranscriptController()
			if prev != nil {
				fresh.Resilience = prev.Resilience
				fresh.OnCopyCode = prev.OnCopyCode
				fresh.SetDeepThink(prev.DeepThinkEnabled())
			}
			s.Adaptive.Desktop.Transcript = fresh
		}
	}
	s.mu.Lock()
	s.chatDraft = ""
	s.thinkingOpen = make(map[string]bool)
	s.activeTab = TabChat
	s.mu.Unlock()
	s.invalidate()
}

func (s *Shell) sendPrompt(prompt string) {
	text := strings.TrimSpace(prompt)
	if text == "" {
		return
	}
	mygo.Haptics.Medium()
	s.mu.Lock()
	s.chatDraft = ""
	s.mu.Unlock()
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.OnSendPrompt != nil {
		s.Adaptive.Desktop.OnSendPrompt(text)
	}
	s.SelectTab(TabChat)
	s.invalidate()
}

func (s *Shell) toggleDeepThink() {
	s.mu.Lock()
	next := !s.deepThink
	s.mu.Unlock()
	s.setDeepThink(next)
}

func (s *Shell) setDeepThink(on bool) {
	s.mu.Lock()
	changed := s.deepThink != on
	s.deepThink = on
	s.mu.Unlock()
	if !changed {
		return
	}
	mygo.Haptics.Selection()
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.OnCommandInvoke != nil {
		if on {
			s.Adaptive.Desktop.OnCommandInvoke("thinking.set on")
		} else {
			s.Adaptive.Desktop.OnCommandInvoke("thinking.set off")
		}
	}
	s.invalidate()
}
