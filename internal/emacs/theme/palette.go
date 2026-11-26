// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package theme owns Marlin UI's governed CSS-stylesheet Doom Emacs themes
// (doom-one, doom-tokyo-night, doom-one-light), user theme-pack cascade over
// github.com/tao3k/marlin-mygo/runtime/style, live marlin.ui.v1 gRPC UITheme
// projection, comfortable editor typography, and soft-rounded editor primitives.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A02 (emacsTheme).
package theme

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/style"
)

//go:embed packs/*.css
var builtinPacks embed.FS

// Editor geometry and typography tokens aligned with Doom Emacs (:size 15).
const (
	FontMono                 = "SF Mono, Menlo, JetBrains Mono, monospace"
	FontSizeBody     float32 = 14
	FontSizeUI       float32 = 13.5
	FontSizeMeta     float32 = 12.5
	TabBarHeight     float32 = 30
	ModelineHeight   float32 = 28
	MinibufferHeight float32 = 32
	RowHeight        float32 = 30

	RadiusSmall   float32 = 4
	RadiusControl float32 = 6
	RadiusCard    float32 = 14
	RadiusPill    float32 = 999
)

// DefaultThemeID selects Marlin UI's flagship Nord Bento CSS theme pack.
const DefaultThemeID = "nord-bento"

// MarlinStyleContract returns Marlin UI's fail-closed CSS-subset admission
// contract declared in dev/sysml/emacs-workbench.sysml (MUI-EMACS-R06).
func MarlinStyleContract() style.Contract {
	return style.DefaultContract(
		"frame",
		"sort-tab",
		"tab",
		"active",
		"window",
		"dashboard",
		"dashboard-row",
		"modeline",
		"modeline-badge",
		"minibuffer",
		"hydra",
		"hydra-head",
		"hydra-key",
		"repeatable",
		"prefix",
		"vertico",
		"vertico-row",
		"selected",
		"match-hit",
		"btn",
		"primary",
		"input",
		"table-header",
		"table-row",
		"preview",
		"bento-card",
		"bento-tile",
		"bento-gauge-track",
		"bento-badge",
		"bento-pill",
	)
}

// Theme defines Marlin UI's Doom Emacs semantic face palette plus its compiled
// governed CSS-subset Sheet (supporting :root variables, class selectors,
// pseudo-states, and user ~/.config/marlin/ui/theme.css overrides).
type Theme struct {
	ID      string
	Name    string
	IsLight bool
	Sheet   *style.Sheet

	Canvas          ui.Color // --bg: main editor buffer canvas
	Bar             ui.Color // --bar: sort-tab & doom-modeline
	SurfaceElevated ui.Color // --surface: subtle elevated control/row surface
	SurfaceHover    ui.Color // --hover: gentle hover highlight
	Border          ui.Color // --border: subtle divider/input hairline
	BorderSubtle    ui.Color // --border-subtle: faint internal separator

	TabActiveBg   ui.Color // --tab-active-bg: sort-tab-current-tab-face background
	TabActiveFg   ui.Color // --tab-active-fg: sort-tab-current-tab-face foreground
	TabInactiveFg ui.Color // --tab-inactive-fg: sort-tab-other-tab-face foreground
	TabSeparator  ui.Color // --tab-sep: sort-tab-separator-face

	BannerInk ui.Color // --banner: startup ASCII banner
	Ink       ui.Color // --fg: primary editor foreground
	InkMuted  ui.Color // --muted: comments, keybindings, metadata

	Accent  ui.Color // --accent: magenta / violet (M-x, Helm, brand accents)
	Info    ui.Color // --info: Doom blue (dashboard links, modeline path/mode)
	Success ui.Color // --success: Doom green (INSERT mode, Online, Ready)
	Warning ui.Color // --warning: Doom yellow (NORMAL/KEYPAD, match hits)
	Danger  ui.Color // --danger: Doom red (errors, rejection, repeatable hydra)
}

var (
	builtinOnce      sync.Once
	builtinNordBento Theme
	builtinDoomOne   Theme
	builtinTokyo     Theme
	builtinDoomLight Theme
)

func initBuiltins() {
	builtinOnce.Do(func() {
		builtinNordBento = mustLoadBuiltin("nord-bento", "nord-bento", false, "packs/nord-bento.css")
		builtinDoomOne = mustLoadBuiltin("doom-one", "doom-one", false, "packs/doom-one.css")
		builtinTokyo = mustLoadBuiltin("doom-tokyo-night", "doom-tokyo-night", false, "packs/doom-tokyo-night.css")
		builtinDoomLight = mustLoadBuiltin("doom-one-light", "doom-one-light", true, "packs/doom-one-light.css")
	})
}

func mustLoadBuiltin(id, name string, isLight bool, path string) Theme {
	raw, err := builtinPacks.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("theme: missing embedded CSS pack %s: %v", path, err))
	}
	fallback := fallbackPalette(isLight)
	th, err := BuildFromCSS(id, name, isLight, &fallback, string(raw))
	if err != nil {
		panic(fmt.Sprintf("theme: invalid embedded CSS pack %s: %v", path, err))
	}
	return th
}

func fallbackPalette(isLight bool) Theme {
	if isLight {
		return Theme{
			ID:              "doom-one-light",
			Name:            "doom-one-light",
			IsLight:         true,
			Canvas:          ui.RGB(250, 250, 250),
			Bar:             ui.RGB(240, 240, 240),
			SurfaceElevated: ui.RGB(231, 233, 238),
			SurfaceHover:    ui.RGB(223, 226, 233),
			Border:          ui.RGB(207, 212, 222),
			BorderSubtle:    ui.RGB(228, 230, 236),
			TabActiveBg:     ui.RGB(213, 201, 192),
			TabActiveFg:     ui.RGB(40, 40, 40),
			TabInactiveFg:   ui.RGB(102, 92, 84),
			TabSeparator:    ui.RGB(189, 174, 147),
			BannerInk:       ui.RGB(156, 160, 170),
			Ink:             ui.RGB(56, 58, 66),
			InkMuted:        ui.RGB(122, 127, 140),
			Accent:          ui.RGB(166, 38, 164),
			Info:            ui.RGB(64, 120, 242),
			Success:         ui.RGB(80, 161, 79),
			Warning:         ui.RGB(193, 132, 1),
			Danger:          ui.RGB(228, 86, 73),
		}
	}
	return Theme{
		ID:              "doom-one",
		Name:            "doom-one",
		IsLight:         false,
		Canvas:          ui.RGB(40, 44, 52),
		Bar:             ui.RGB(33, 36, 43),
		SurfaceElevated: ui.RGB(45, 50, 60),
		SurfaceHover:    ui.RGB(52, 58, 70),
		Border:          ui.RGB(63, 68, 74),
		BorderSubtle:    ui.RGB(47, 52, 62),
		TabActiveBg:     ui.RGB(80, 73, 69),
		TabActiveFg:     ui.RGB(251, 241, 199),
		TabInactiveFg:   ui.RGB(189, 174, 147),
		TabSeparator:    ui.RGB(102, 92, 84),
		BannerInk:       ui.RGB(91, 98, 104),
		Ink:             ui.RGB(187, 194, 207),
		InkMuted:        ui.RGB(140, 146, 156),
		Accent:          ui.RGB(198, 120, 221),
		Info:            ui.RGB(81, 175, 239),
		Success:         ui.RGB(152, 190, 101),
		Warning:         ui.RGB(236, 190, 123),
		Danger:          ui.RGB(255, 108, 107),
	}
}

func projectVarsFromSheet(base Theme, sheet *style.Sheet) Theme {
	out := base
	out.Sheet = sheet
	if sheet == nil {
		return out
	}
	out.Canvas = sheet.VarColor("--bg", out.Canvas)
	out.Bar = sheet.VarColor("--bar", out.Bar)
	out.SurfaceElevated = sheet.VarColor("--surface", out.SurfaceElevated)
	out.SurfaceHover = sheet.VarColor("--hover", out.SurfaceHover)
	out.Border = sheet.VarColor("--border", out.Border)
	out.BorderSubtle = sheet.VarColor("--border-subtle", out.BorderSubtle)
	out.TabActiveBg = sheet.VarColor("--tab-active-bg", out.TabActiveBg)
	out.TabActiveFg = sheet.VarColor("--tab-active-fg", out.TabActiveFg)
	out.TabInactiveFg = sheet.VarColor("--tab-inactive-fg", out.TabInactiveFg)
	out.TabSeparator = sheet.VarColor("--tab-sep", out.TabSeparator)
	out.BannerInk = sheet.VarColor("--banner", out.BannerInk)
	out.Ink = sheet.VarColor("--fg", out.Ink)
	out.InkMuted = sheet.VarColor("--muted", out.InkMuted)
	out.Accent = sheet.VarColor("--accent", out.Accent)
	out.Info = sheet.VarColor("--info", out.Info)
	out.Success = sheet.VarColor("--success", out.Success)
	out.Warning = sheet.VarColor("--warning", out.Warning)
	out.Danger = sheet.VarColor("--danger", out.Danger)
	return out
}

// BuildFromCSS parses rawCSS fail-closed against MarlinStyleContract(), cascades
// it onto base (when provided), and returns a complete Theme ready for 120Hz GPU
// rendering.
func BuildFromCSS(id, name string, isLight bool, base *Theme, rawCSS string) (Theme, error) {
	parsed, err := style.Parse(rawCSS, MarlinStyleContract())
	if err != nil {
		return Theme{}, err
	}
	seed := fallbackPalette(isLight)
	if base != nil {
		seed = *base
	}
	seed.ID = id
	if strings.TrimSpace(name) == "" {
		name = id
	}
	seed.Name = name
	seed.IsLight = isLight

	sheet := parsed
	if base != nil && base.Sheet != nil {
		sheet = base.Sheet.Cascade(parsed)
	}
	return projectVarsFromSheet(seed, sheet), nil
}

// WithDesignTokensJSON compiles a W3C Design Tokens JSON document (supporting
// oklch() perceptual colors & {alias} references) and cascades it onto th
// fail-closed.
func (th Theme) WithDesignTokensJSON(rawJSON string) (Theme, error) {
	if strings.TrimSpace(rawJSON) == "" {
		return th, nil
	}
	tokSheet, err := style.ParseDesignTokensJSON(rawJSON, MarlinStyleContract())
	if err != nil {
		return Theme{}, err
	}
	merged := tokSheet
	if th.Sheet != nil {
		merged = th.Sheet.Cascade(tokSheet)
	}
	return projectVarsFromSheet(th, merged), nil
}

// WithOverrideCSS cascades an optional user override stylesheet (such as
// ~/.config/marlin/ui/theme.css) onto th fail-closed. If rawCSS violates
// MarlinStyleContract(), the returned error carries the exact line diagnostic
// and th remains untouched.
func (th Theme) WithOverrideCSS(rawCSS string) (Theme, error) {
	if strings.TrimSpace(rawCSS) == "" {
		return th, nil
	}
	override, err := style.Parse(rawCSS, MarlinStyleContract())
	if err != nil {
		return Theme{}, err
	}
	merged := override
	if th.Sheet != nil {
		merged = th.Sheet.Cascade(override)
	}
	return projectVarsFromSheet(th, merged), nil
}

// NordBento returns Marlin UI's flagship EWW Nord-Bento CSS theme pack.
func NordBento() Theme {
	initBuiltins()
	return builtinNordBento
}

// DoomOne returns Marlin UI's Doom One CSS theme pack.
func DoomOne() Theme {
	initBuiltins()
	return builtinDoomOne
}

// DoomTokyoNight returns Marlin UI's Doom Tokyo Night CSS theme pack.
func DoomTokyoNight() Theme {
	initBuiltins()
	return builtinTokyo
}

// DoomOneLight returns Marlin UI's Doom One Light CSS theme pack.
func DoomOneLight() Theme {
	initBuiltins()
	return builtinDoomLight
}

// Catalog returns Marlin UI's built-in CSS theme packs.
func Catalog() []Theme {
	return []Theme{
		NordBento(),
		DoomOne(),
		DoomTokyoNight(),
		DoomOneLight(),
	}
}

// ByID resolves a built-in theme by ID (defaulting to nord-bento).
func ByID(id string) Theme {
	norm := strings.ToLower(strings.TrimSpace(id))
	for _, th := range Catalog() {
		if th.ID == norm {
			return th
		}
	}
	return NordBento()
}

// Next cycles deterministically to the next built-in theme profile.
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

// FromUITheme projects a live marlin.ui.v1 gRPC schema.UITheme into Marlin UI's
// Theme while inheriting the compiled base CSS Sheet for widget rules.
func FromUITheme(id, name string, mt schema.UITheme) Theme {
	if id == "" {
		id = "grpc"
	}
	if name == "" {
		name = id
	}
	isLight := int(mt.Canvas.R)+int(mt.Canvas.G)+int(mt.Canvas.B) > 380
	base := DoomOne()
	if isLight {
		base = DoomOneLight()
	}
	elev := ui.RGB(clampU8(int(mt.Canvas.R)+8), clampU8(int(mt.Canvas.G)+9), clampU8(int(mt.Canvas.B)+12))
	hover := ui.RGB(clampU8(int(mt.Canvas.R)+14), clampU8(int(mt.Canvas.G)+15), clampU8(int(mt.Canvas.B)+19))
	border := ui.RGB(clampU8(int(mt.Bar.R)+24), clampU8(int(mt.Bar.G)+26), clampU8(int(mt.Bar.B)+32))
	subtle := ui.RGB(clampU8(int(mt.Canvas.R)+6), clampU8(int(mt.Canvas.G)+7), clampU8(int(mt.Canvas.B)+10))
	if isLight {
		elev = base.SurfaceElevated
		hover = base.SurfaceHover
		border = base.Border
		subtle = base.BorderSubtle
	}
	return Theme{
		ID:              id,
		Name:            name,
		IsLight:         isLight,
		Sheet:           base.Sheet,
		Canvas:          ui.RGB(mt.Canvas.R, mt.Canvas.G, mt.Canvas.B),
		Bar:             ui.RGB(mt.Bar.R, mt.Bar.G, mt.Bar.B),
		SurfaceElevated: elev,
		SurfaceHover:    hover,
		Border:          border,
		BorderSubtle:    subtle,
		TabActiveBg:     base.TabActiveBg,
		TabActiveFg:     base.TabActiveFg,
		TabInactiveFg:   base.TabInactiveFg,
		TabSeparator:    base.TabSeparator,
		BannerInk:       base.BannerInk,
		Ink:             ui.RGB(mt.Ink.R, mt.Ink.G, mt.Ink.B),
		InkMuted:        ui.RGB(mt.Muted.R, mt.Muted.G, mt.Muted.B),
		Accent:          ui.RGB(mt.Accent.R, mt.Accent.G, mt.Accent.B),
		Info:            ui.RGB(mt.Info.R, mt.Info.G, mt.Info.B),
		Success:         ui.RGB(mt.Success.R, mt.Success.G, mt.Success.B),
		Warning:         ui.RGB(mt.Warning.R, mt.Warning.G, mt.Warning.B),
		Danger:          base.Danger,
	}
}

// ToMyGoTheme converts th into a reusable *ui.Theme so Frame snapshots can
// cache it once per theme change (zero per-frame heap allocations on 120Hz render).
func (th Theme) ToMyGoTheme() *ui.Theme {
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
	t.Radius = th.Sheet.VarPx("--radius-control", RadiusControl)
	t.Spacing = 4
	return t
}

// ApplyToMyGo configures MyGo's *ui.Theme from th and its compiled Sheet.
func ApplyToMyGo(c *ui.Context, th Theme) {
	c.SetTheme(th.ToMyGoTheme())
}

// ToKitPalette projects Marlin UI's Theme into marlin-mygo/runtime/kit.Palette
// for flagship GPU Liquid Glass, SF Symbols, shader orbs, and Bento widgets.
func (th Theme) ToKitPalette() kit.Palette {
	return kit.Palette{
		IsLight: th.IsLight,
		Canvas:  th.Canvas,
		Bar:     th.Bar,
		Surface: th.SurfaceElevated,
		Hover:   th.SurfaceHover,
		Border:  th.Border,
		Ink:     th.Ink,
		Muted:   th.InkMuted,
		Accent:  th.Accent,
		Info:    th.Info,
		Success: th.Success,
		Warning: th.Warning,
		Danger:  th.Danger,
	}
}

// ToMapperSkin projects Marlin UI's Theme into marlin-mygo's declarative
// Scheme-to-MyGo mapper.Skin contract.
func (th Theme) ToMapperSkin() mapper.Skin {
	return mapper.Skin{
		Font:            FontMono,
		FontSizeBody:    FontSizeBody,
		FontSizeUI:      FontSizeUI,
		FontSizeMeta:    FontSizeMeta,
		RadiusSmall:     th.Sheet.VarPx("--radius-small", RadiusSmall),
		RadiusControl:   th.Sheet.VarPx("--radius-control", RadiusControl),
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
