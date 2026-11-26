// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/style"
)

// GlassTone adapts a theme surface color to translucent frosted glass when OS
// window vibrancy is active, preserving solid theme contrast in Matte mode.
func GlassTone(c *ui.Context, col ui.Color, th Theme, darkAlpha, lightAlpha float32) ui.Color {
	if c == nil || !c.Vibrancy() {
		return col
	}
	a := darkAlpha
	if th.IsLight {
		a = lightAlpha
	}
	return kit.WithAlpha(col, a)
}

// GlassWeight applies visionOS / APCA glass typography weight bumping (400 -> 500)
// when OS frosted vibrancy is active so body text punches cleanly through blur.
func GlassWeight(c *ui.Context, base int) int {
	if c != nil && c.Vibrancy() && base < 500 {
		return 500
	}
	return base
}

// TableColumn describes one fixed-width or flexible column in Emacs tabulated-list-mode.
type TableColumn struct {
	Label string
	Width float32 // 0 means Grow(1)
	Color ui.Color
	Bold  bool
}

// ActionButton renders a soft-rounded, Sheet-styled Doom Emacs action button
// resolving `.btn`, `.btn.primary`, and `.btn:hover` from th.Sheet.
func ActionButton(c *ui.Context, key, label string, tone ui.Color, active bool, th Theme) ui.Element {
	btn := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(label).
		Padding(5, 12).Radius(RadiusControl).AlignItems(ui.Center).Justify(ui.Center))

	classes := []string{"btn"}
	if active {
		classes = append(classes, "primary")
	}
	state := style.StateNone
	if btn.Hovered() {
		state |= style.StateHover
	}
	cs := th.Sheet.Resolve(classes, state)

	bg := GlassTone(c, th.SurfaceElevated, th, 0.36, 0.56)
	if active || btn.Hovered() {
		bg = GlassTone(c, th.SurfaceHover, th, 0.50, 0.70)
	}
	if cs.HasBackground {
		bg = cs.Background
	}
	btn.Background(bg)
	if c.Vibrancy() {
		btn.Border(1, kit.WithAlpha(th.Ink, 0.12))
	}
	cs.ApplyToElement(btn)
	if !cs.HasBackground {
		btn.Background(bg)
	}

	weight := 500
	if active {
		weight = 700
	}
	if cs.HasFontWeight {
		weight = cs.FontWeight
	}
	fontSize := FontSizeMeta
	if cs.HasFontSize {
		fontSize = cs.FontSize
	}
	btn.Children(func() {
		ui.Text(c, label).Font(FontMono).FontSize(fontSize).FontWeight(weight).TextColor(tone).NoWrap()
	})
	return btn
}

// ActionLink renders a quiet soft-rounded hover text action for chrome bars.
func ActionLink(c *ui.Context, key, label string, tone ui.Color, th Theme) ui.Element {
	rad := th.Sheet.VarPx("--radius-small", RadiusSmall)
	btn := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(label).
		Padding(3, 9).Radius(rad).AlignItems(ui.Center).Justify(ui.Center))
	bg := ui.Transparent
	if btn.Hovered() {
		bg = GlassTone(c, th.SurfaceHover, th, 0.30, 0.48)
	}
	btn.Background(bg)
	btn.Children(func() {
		ui.Text(c, label).Font(FontMono).FontSize(FontSizeMeta).FontWeight(500).TextColor(tone).NoWrap()
	})
	return btn
}

// VerticoRow renders one soft-rounded Vertico / Helm candidate row styled by
// `.vertico-row`, `.vertico-row.selected`, `.vertico-row:hover`, and `.match-hit`.
func VerticoRow(c *ui.Context, key, name, desc, query string, nameWidth float32, selected bool, th Theme) ui.Element {
	if nameWidth <= 0 {
		nameWidth = 240
	}
	row := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(name).
		Height(RowHeight).Padding(0, 14).Gap(10).Radius(RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start))

	classes := []string{"vertico-row"}
	if selected {
		classes = append(classes, "selected")
	}
	state := style.StateNone
	if selected {
		state |= style.StateSelected
	}
	if row.Hovered() {
		state |= style.StateHover
	}
	cs := th.Sheet.Resolve(classes, state)

	bg := ui.Transparent
	marker := "  "
	nameCol := th.Ink
	weight := 400
	if selected {
		bg = GlassTone(c, th.SurfaceElevated, th, 0.42, 0.62)
		marker = "› "
		nameCol = th.Info
		weight = 600
	} else if row.Hovered() {
		bg = GlassTone(c, th.SurfaceHover, th, 0.28, 0.46)
	}
	row.Background(bg)
	cs.ApplyToElement(row)
	if !cs.HasBackground {
		row.Background(bg)
	}

	row.Children(func() {
		ui.Text(c, marker).Font(FontMono).FontSize(FontSizeBody).FontWeight(700).TextColor(th.Accent).Width(18).NoWrap()
		renderHighlightedRuns(c, name, query, FontSizeUI, weight, nameCol, nameWidth, false, th)
		if desc != "" {
			renderHighlightedRuns(c, desc, query, FontSizeMeta, 400, th.InkMuted, 0, true, th)
		}
	})
	return row
}

func renderHighlightedRuns(
	c *ui.Context,
	text, query string,
	fontSize float32,
	baseWeight int,
	baseCol ui.Color,
	fixedWidth float32,
	grow bool,
	th Theme,
) {
	runs := MatchRuns(text, query)
	hasMatch := false
	for _, r := range runs {
		if r.Matched {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		el := ui.Text(c, text).Font(FontMono).FontSize(fontSize).FontWeight(baseWeight).TextColor(baseCol).NoWrap().MaxLines(1)
		if fixedWidth > 0 {
			el.Width(fixedWidth)
		} else if grow {
			el.Grow(1)
		}
		return
	}

	hitStyle := th.Sheet.Resolve([]string{"match-hit"}, style.StateNone)
	hitBg := ui.RGBA(236, 190, 123, 0.18)
	if th.IsLight {
		hitBg = ui.RGBA(193, 132, 1, 0.16)
	}
	if hitStyle.HasBackground {
		hitBg = hitStyle.Background
	}
	hitFg := th.Warning
	if hitStyle.HasColor {
		hitFg = hitStyle.Color
	}
	hitRadius := float32(3)
	if hitStyle.HasBorderRadius {
		hitRadius = hitStyle.BorderRadius
	}
	hitWeight := 700
	if hitStyle.HasFontWeight {
		hitWeight = hitStyle.FontWeight
	}

	box := ui.Row(c).AlignItems(ui.Center).Clip()
	if fixedWidth > 0 {
		box.Width(fixedWidth)
	} else if grow {
		box.Grow(1)
	}
	box.Children(func() {
		for _, r := range runs {
			if r.Matched {
				ui.Row(c).Background(hitBg).Radius(hitRadius).Padding(0, 1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(hitWeight).TextColor(hitFg).NoWrap()
				})
			} else {
				ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(baseWeight).TextColor(baseCol).NoWrap()
			}
		}
	})
}

// TabulatedHeader renders a clean theme- and glass-aware Doom Emacs tabulated-list header strip.
func TabulatedHeader(c *ui.Context, cols []TableColumn, th Theme) {
	hdrBg := GlassTone(c, th.SurfaceElevated, th, 0.22, 0.44)
	row := ui.Row(c).Height(32).Padding(0, 18).Gap(12).AlignItems(ui.Center).Background(hdrBg)
	if c.Vibrancy() {
		row.BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.Ink, 0.10))
	}
	row.Children(func() {
		ui.Box(c).Width(16)
		for _, col := range cols {
			t := ui.Text(c, col.Label).Font(FontMono).FontSize(FontSizeMeta).FontWeight(600).TextColor(th.InkMuted).NoWrap()
			if col.Width > 0 {
				t.Width(col.Width)
			} else {
				t.Grow(1)
			}
		}
	})
}

// TabulatedRow renders one soft-rounded theme- and glass-aware Emacs tabulated-list row with a › accent cursor.
func TabulatedRow(c *ui.Context, key string, cols []TableColumn, selected bool, th Theme) ui.Element {
	rad := th.Sheet.VarPx("--radius-small", RadiusSmall)
	row := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(firstColLabel(cols)).
		Height(RowHeight + 2).Padding(0, 18).Gap(12).Radius(rad).
		AlignItems(ui.Center).Justify(ui.Start))
	bg := ui.Transparent
	marker := "  "
	if selected {
		bg = GlassTone(c, th.SurfaceElevated, th, 0.34, 0.56)
		marker = "› "
		if c.Vibrancy() {
			row.Border(1, kit.WithAlpha(th.Info, 0.22))
		}
	} else if row.Hovered() {
		bg = GlassTone(c, th.SurfaceHover, th, 0.24, 0.42)
	}
	row.Background(bg)
	row.Children(func() {
		ui.Text(c, marker).Font(FontMono).FontSize(FontSizeBody).FontWeight(700).TextColor(th.Accent).Width(16).NoWrap()
		for _, col := range cols {
			ink := col.Color
			if ink == (ui.Color{}) {
				ink = th.Ink
			}
			w := GlassWeight(c, 400)
			if col.Bold || selected {
				w = 600
			}
			t := ui.Text(c, col.Label).Font(FontMono).FontSize(FontSizeUI).FontWeight(w).TextColor(ink).NoWrap().MaxLines(1)
			if col.Width > 0 {
				t.Width(col.Width)
			} else {
				t.Grow(1)
			}
		}
	})
	return row
}

// FieldRow renders a spacious, soft Emacs customize key/value row inside *Settings*.
func FieldRow(c *ui.Context, keyLabel, hint string, th Theme, control func()) {
	ui.Row(c).Padding(9, 20).Gap(14).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(GlassTone(c, th.BorderSubtle, th, 0.35, 0.45)).Children(func() {
		ui.Text(c, keyLabel).Font(FontMono).FontSize(FontSizeUI).FontWeight(500).TextColor(th.Info).Width(210).NoWrap()
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			if control != nil {
				control()
			}
			if hint != "" {
				ui.Text(c, hint).Font(FontMono).FontSize(FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			}
		})
	})
}

// InlineInput renders a soft-rounded monospace input field styled via `.input`.
func InlineInput(c *ui.Context, key string, value *string, placeholder string, width float32, th Theme) ui.Element {
	var inp ui.Element
	cs := th.Sheet.Resolve([]string{"input"}, style.StateNone)
	row := ui.Row(c.Key(key+"-wrap")).Padding(6, 10).
		Background(GlassTone(c, th.Bar, th, 0.34, 0.58)).
		Radius(RadiusControl).Border(1, GlassTone(c, th.Border, th, 0.55, 0.65)).AlignItems(ui.Center)
	cs.ApplyToElement(row)
	if width > 0 {
		row.Width(width)
	} else {
		row.Grow(1)
	}
	fontSize := FontSizeUI
	if cs.HasFontSize {
		fontSize = cs.FontSize
	}
	textCol := th.Ink
	if cs.HasColor {
		textCol = cs.Color
	}
	row.Children(func() {
		inp = ui.TextInputBase(c.Key(key), value).
			Placeholder(placeholder).Font(FontMono).FontSize(fontSize).TextColor(textCol).Grow(1)
	})
	return inp
}

func firstColLabel(cols []TableColumn) string {
	if len(cols) == 0 {
		return ""
	}
	return cols[0].Label
}
