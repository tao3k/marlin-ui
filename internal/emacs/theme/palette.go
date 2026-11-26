// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package theme owns Marlin UI's authentic Doom Emacs face palettes
// (aligned with doom-one-theme.el, sort-tab, doom-modeline, and live
// marlin.ui.v1 gRPC UITheme snapshots), comfortable editor typography, and
// soft-rounded editor primitives over github.com/egoist/mygo/ui.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A02 (emacsTheme).
package theme

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
)

// Editor geometry and typography tokens aligned with Doom Emacs (:size 15).
const (
	FontMono         = "SF Mono, Menlo, JetBrains Mono, monospace"
	FontSizeBody     float32 = 14
	FontSizeUI       float32 = 13.5
	FontSizeMeta     float32 = 12.5
	TabBarHeight     float32 = 30
	ModelineHeight   float32 = 28
	MinibufferHeight float32 = 32
	RowHeight        float32 = 30

	RadiusSmall   float32 = 4
	RadiusControl float32 = 6
	RadiusPill    float32 = 999
)

// Theme defines Marlin UI's Doom Emacs semantic face palette (doom-one-theme.el).
type Theme struct {
	ID      string
	Name    string
	IsLight bool

	Canvas          ui.Color // bg: main editor buffer canvas (#282c34)
	Bar             ui.Color // bg-alt: sort-tab & doom-modeline (#21242b)
	SurfaceElevated ui.Color // subtle elevated control/row surface (#2c323c)
	SurfaceHover    ui.Color // gentle hover highlight (#313744)
	Border          ui.Color // subtle divider/input hairline (#3f444a)
	BorderSubtle    ui.Color // faint internal separator (#2e333d)

	TabActiveBg   ui.Color // sort-tab-current-tab-face background (#504945)
	TabActiveFg   ui.Color // sort-tab-current-tab-face foreground (#fbf1c7)
	TabInactiveFg ui.Color // sort-tab-other-tab-face foreground (#bdae93)
	TabSeparator  ui.Color // sort-tab-separator-face (#665c54)

	BannerInk ui.Color // fg-alt / base5: Doom ASCII banner (#5b6268)
	Ink       ui.Color // fg: primary editor foreground (#bbc2cf)
	InkMuted  ui.Color // base6/base7: comments, keybindings, metadata (#8c929c)

	Accent  ui.Color // magenta / amber (M-x, Helm, brand accents)
	Info    ui.Color // Doom blue (#51afef: dashboard links, modeline path/mode)
	Success ui.Color // Doom green (#98be65: INSERT mode, Online, Ready)
	Warning ui.Color // Doom yellow (#ecbe7b: NORMAL/KEYPAD mode, Standalone)
	Danger  ui.Color // Doom red (#ff6c6b: errors, rejection)
}

// DefaultThemeID matches Doom Emacs's flagship doom-one profile.
const DefaultThemeID = "doom-one"

// Catalog returns Marlin UI's authentic Doom Emacs theme profiles.
func Catalog() []Theme {
	return []Theme{
		DoomOne(),
		Pi(),
		DoomPaper(),
	}
}

// ByID resolves a theme by ID (defaulting to doom-one).
func ByID(id string) Theme {
	norm := strings.ToLower(strings.TrimSpace(id))
	for _, th := range Catalog() {
		if th.ID == norm {
			return th
		}
	}
	return DoomOne()
}

// Next cycles deterministically to the next theme profile.
func Next(currentID string) Theme {
	all := Catalog()
	norm := strings.ToLower(strings.TrimSpace(currentID))
	for i, th := range all {
		if th.ID == norm {
			return all[(i+1)%len(all)]
		}
	}
	return all[0]
}

// FromUITheme projects a live marlin.ui.v1 gRPC schema.UITheme directly into
// Marlin UI's Doom Emacs face palette.
func FromUITheme(id, name string, mt schema.UITheme) Theme {
	if id == "" {
		id = "grpc"
	}
	if name == "" {
		name = id
	}
	isLight := int(mt.Canvas.R)+int(mt.Canvas.G)+int(mt.Canvas.B) > 380
	elev := ui.RGB(clampU8(int(mt.Canvas.R)+8), clampU8(int(mt.Canvas.G)+9), clampU8(int(mt.Canvas.B)+12))
	hover := ui.RGB(clampU8(int(mt.Canvas.R)+14), clampU8(int(mt.Canvas.G)+15), clampU8(int(mt.Canvas.B)+19))
	border := ui.RGB(clampU8(int(mt.Bar.R)+24), clampU8(int(mt.Bar.G)+26), clampU8(int(mt.Bar.B)+32))
	subtle := ui.RGB(clampU8(int(mt.Canvas.R)+6), clampU8(int(mt.Canvas.G)+7), clampU8(int(mt.Canvas.B)+10))
	tabBg := ui.RGB(80, 73, 69)
	tabFg := ui.RGB(251, 241, 199)
	tabOther := ui.RGB(189, 174, 147)
	tabSep := ui.RGB(102, 92, 84)
	banner := ui.RGB(91, 98, 104)
	danger := ui.RGB(255, 108, 107)
	if isLight {
		elev = ui.RGB(240, 241, 245)
		hover = ui.RGB(232, 234, 240)
		border = ui.RGB(210, 214, 222)
		subtle = ui.RGB(228, 230, 236)
		tabBg = ui.RGB(213, 201, 192)
		tabFg = ui.RGB(40, 40, 40)
		tabOther = ui.RGB(102, 92, 84)
		tabSep = ui.RGB(189, 174, 147)
		banner = ui.RGB(156, 160, 168)
		danger = ui.RGB(205, 52, 62)
	}
	return Theme{
		ID:              id,
		Name:            name,
		IsLight:         isLight,
		Canvas:          ui.RGB(mt.Canvas.R, mt.Canvas.G, mt.Canvas.B),
		Bar:             ui.RGB(mt.Bar.R, mt.Bar.G, mt.Bar.B),
		SurfaceElevated: elev,
		SurfaceHover:    hover,
		Border:          border,
		BorderSubtle:    subtle,
		TabActiveBg:     tabBg,
		TabActiveFg:     tabFg,
		TabInactiveFg:   tabOther,
		TabSeparator:    tabSep,
		BannerInk:       banner,
		Ink:             ui.RGB(mt.Ink.R, mt.Ink.G, mt.Ink.B),
		InkMuted:        ui.RGB(mt.Muted.R, mt.Muted.G, mt.Muted.B),
		Accent:          ui.RGB(mt.Accent.R, mt.Accent.G, mt.Accent.B),
		Info:            ui.RGB(mt.Info.R, mt.Info.G, mt.Info.B),
		Success:         ui.RGB(mt.Success.R, mt.Success.G, mt.Success.B),
		Warning:         ui.RGB(mt.Warning.R, mt.Warning.G, mt.Warning.B),
		Danger:          danger,
	}
}

// DoomOne matches doom-one-theme.el and sort-tab.el face colors verbatim.
func DoomOne() Theme {
	return Theme{
		ID:              "doom-one",
		Name:            "doom-one",
		IsLight:         false,
		Canvas:          ui.RGB(40, 44, 52),    // #282c34 (doom-one bg)
		Bar:             ui.RGB(33, 36, 43),    // #21242b (doom-one bg-alt)
		SurfaceElevated: ui.RGB(45, 50, 60),    // #2d323c
		SurfaceHover:    ui.RGB(52, 58, 70),    // #343a46
		Border:          ui.RGB(63, 68, 74),    // #3f444a (doom-one base4)
		BorderSubtle:    ui.RGB(47, 52, 62),    // #2f343e
		TabActiveBg:     ui.RGB(80, 73, 69),    // #504945 (sort-tab-current-tab-face)
		TabActiveFg:     ui.RGB(251, 241, 199), // #fbf1c7
		TabInactiveFg:   ui.RGB(189, 174, 147), // #bdae93 (sort-tab-other-tab-face)
		TabSeparator:    ui.RGB(102, 92, 84),   // #665c54 (sort-tab-separator-face)
		BannerInk:       ui.RGB(91, 98, 104),   // #5b6268 (doom-one base5)
		Ink:             ui.RGB(187, 194, 207), // #bbc2cf (doom-one fg)
		InkMuted:        ui.RGB(140, 146, 156), // #8c929c
		Accent:          ui.RGB(198, 120, 221), // #c678dd (doom-one magenta)
		Info:            ui.RGB(81, 175, 239),  // #51afef (doom-one blue)
		Success:         ui.RGB(152, 190, 101), // #98be65 (doom-one green)
		Warning:         ui.RGB(236, 190, 123), // #ecbe7b (doom-one yellow)
		Danger:          ui.RGB(255, 108, 107), // #ff6c6b (doom-one red)
	}
}

// Pi matches marlin-agent-core/scheme/ui-themes/objects.ss UiPiInspired.
func Pi() Theme {
	return Theme{
		ID:              "pi",
		Name:            "pi",
		IsLight:         false,
		Canvas:          ui.RGB(18, 21, 30),    // #12151e
		Bar:             ui.RGB(23, 26, 36),    // #171a24
		SurfaceElevated: ui.RGB(29, 33, 46),    // #1d212e
		SurfaceHover:    ui.RGB(36, 41, 56),    // #242938
		Border:          ui.RGB(48, 55, 74),    // #30374a
		BorderSubtle:    ui.RGB(31, 36, 50),    // #1f2432
		TabActiveBg:     ui.RGB(48, 54, 72),    // #303648
		TabActiveFg:     ui.RGB(255, 226, 184), // warm amber cream
		TabInactiveFg:   ui.RGB(146, 154, 170),
		TabSeparator:    ui.RGB(62, 69, 88),
		BannerInk:       ui.RGB(78, 86, 106),
		Ink:             ui.RGB(210, 214, 224), // #d2d6e0
		InkMuted:        ui.RGB(134, 143, 157), // #868f9d
		Accent:          ui.RGB(255, 186, 104), // #ffba68
		Info:            ui.RGB(75, 192, 224),  // #4bc0e0
		Success:         ui.RGB(84, 206, 157),  // #54ce9d
		Warning:         ui.RGB(245, 169, 103), // #f5a967
		Danger:          ui.RGB(240, 113, 120), // #f07178
	}
}

// DoomPaper is a crisp Doom Emacs daylight theme (doom-one-light inspired).
func DoomPaper() Theme {
	return Theme{
		ID:              "doom-paper",
		Name:            "doom-paper",
		IsLight:         true,
		Canvas:          ui.RGB(250, 250, 250), // #fafafa
		Bar:             ui.RGB(240, 240, 240), // #f0f0f0
		SurfaceElevated: ui.RGB(238, 240, 244),
		SurfaceHover:    ui.RGB(228, 231, 238),
		Border:          ui.RGB(210, 213, 220),
		BorderSubtle:    ui.RGB(228, 230, 236),
		TabActiveBg:     ui.RGB(213, 201, 192), // #d5c9c0 (sort-tab light)
		TabActiveFg:     ui.RGB(40, 40, 40),    // #282828
		TabInactiveFg:   ui.RGB(102, 92, 84),   // #665c54
		TabSeparator:    ui.RGB(189, 174, 147), // #bdae93
		BannerInk:       ui.RGB(160, 164, 174),
		Ink:             ui.RGB(56, 58, 66),    // #383a42
		InkMuted:        ui.RGB(130, 134, 146), // #828692
		Accent:          ui.RGB(166, 38, 164),  // #a626a4
		Info:            ui.RGB(64, 120, 242),  // #4078f2
		Success:         ui.RGB(80, 161, 79),   // #50a14f
		Warning:         ui.RGB(193, 132, 1),   // #c18401
		Danger:          ui.RGB(228, 86, 73),   // #e45649
	}
}

// ApplyToMyGo configures MyGo's *ui.Theme for soft-rounded, legible Doom Emacs rendering.
func ApplyToMyGo(c *ui.Context, th Theme) {
	t := ui.DarkTheme()
	if th.IsLight {
		t = ui.LightTheme()
	}
	t.Background = th.Canvas
	t.Surface = th.Bar
	t.SurfaceHover = th.SurfaceHover
	t.SurfacePressed = th.SurfaceElevated
	t.Border = th.Border
	t.Text = th.Ink
	t.TextMuted = th.InkMuted
	t.Accent = th.Accent
	t.AccentHover = th.Info
	t.AccentText = th.Canvas
	t.Success = th.Success
	t.Warning = th.Warning
	t.Danger = th.Danger
	t.Font = FontMono
	t.FontSize = FontSizeBody
	t.Radius = RadiusControl
	t.Spacing = 4
	c.SetTheme(t)
}

// ToMapperSkin projects Marlin UI's Doom Emacs Theme into marlin-mygo's
// declarative Scheme-to-MyGo mapper.Skin contract.
func (th Theme) ToMapperSkin() mapper.Skin {
	return mapper.Skin{
		Font:            FontMono,
		FontSizeBody:    FontSizeBody,
		FontSizeUI:      FontSizeUI,
		FontSizeMeta:    FontSizeMeta,
		RadiusSmall:     RadiusSmall,
		RadiusControl:   RadiusControl,
		BarHeight:       ModelineHeight,
		RowHeight:       RowHeight,
		Canvas:          th.Canvas,
		Bar:             th.Bar,
		SurfaceElevated: th.SurfaceElevated,
		SurfaceHover:    th.SurfaceHover,
		Border:          th.Border,
		Ink:             th.Ink,
		InkMuted:        th.InkMuted,
		Accent:          th.Accent,
		Info:            th.Info,
		Success:         th.Success,
		Warning:         th.Warning,
		Danger:          th.Danger,
	}
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
