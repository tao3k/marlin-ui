// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import "github.com/egoist/mygo/ui"

// TableColumn describes one fixed-width or flexible column in Emacs tabulated-list-mode.
type TableColumn struct {
	Label string
	Width float32 // 0 means Grow(1)
	Color ui.Color
	Bold  bool
}

// ActionButton renders a soft-rounded, tactile Doom Emacs action button with
// comfortable padding and subtle surface elevation (zero jagged corners).
func ActionButton(c *ui.Context, key, label string, tone ui.Color, active bool, th Theme) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).
		Padding(5, 12).Radius(RadiusControl).AlignItems(ui.Center).Justify(ui.Center)
	bg := th.SurfaceElevated
	if active {
		bg = th.SurfaceHover
	}
	if btn.Hovered() {
		bg = th.SurfaceHover
	}
	btn.Background(bg)
	weight := 500
	if active {
		weight = 700
	}
	btn.Children(func() {
		ui.Text(c, label).Font(FontMono).FontSize(FontSizeMeta).FontWeight(weight).TextColor(tone).NoWrap()
	})
	return btn
}

// ActionLink renders a quiet soft-rounded hover text action for chrome bars.
func ActionLink(c *ui.Context, key, label string, tone ui.Color, th Theme) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).
		Padding(3, 9).Radius(RadiusSmall).AlignItems(ui.Center).Justify(ui.Center)
	bg := ui.Transparent
	if btn.Hovered() {
		bg = th.SurfaceHover
	}
	btn.Background(bg)
	btn.Children(func() {
		ui.Text(c, label).Font(FontMono).FontSize(FontSizeMeta).FontWeight(500).TextColor(tone).NoWrap()
	})
	return btn
}

// VerticoRow renders one soft-rounded Vertico / Helm candidate row with
// Orderless/fuzzy matched-character highlighting (matching Emacs orderless-match-face-0):
//
//	selected:   subtle elevated tint + "› " (accent) + name (Doom blue bold) + description (muted)
//	unselected: canvas               + "  "          + name (ink)            + description (muted)
func VerticoRow(c *ui.Context, key, name, desc, query string, nameWidth float32, selected bool, th Theme) ui.Element {
	if nameWidth <= 0 {
		nameWidth = 240
	}
	row := ui.ButtonBase(c.Key(key)).Label(name).
		Height(RowHeight).Padding(0, 14).Gap(10).Radius(RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start)

	bg := ui.Transparent
	marker := "  "
	nameCol := th.Ink
	weight := 400
	if selected {
		bg = th.SurfaceElevated
		marker = "› "
		nameCol = th.Info
		weight = 600
	} else if row.Hovered() {
		bg = th.SurfaceHover
	}
	row.Background(bg)
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

	hitBg := ui.RGBA(236, 190, 123, 44) // warm Doom gold translucent highlight pill
	if th.IsLight {
		hitBg = ui.RGBA(193, 132, 1, 36)
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
				ui.Row(c).Background(hitBg).Radius(3).Padding(0, 1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(700).TextColor(th.Warning).NoWrap()
				})
			} else {
				ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(baseWeight).TextColor(baseCol).NoWrap()
			}
		}
	})
}

// TabulatedHeader renders a clean borderless Doom Emacs tabulated-list header strip.
func TabulatedHeader(c *ui.Context, cols []TableColumn, th Theme) {
	ui.Row(c).Height(32).Padding(0, 18).Gap(12).AlignItems(ui.Center).
		Background(th.Bar).Children(func() {
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

// TabulatedRow renders one soft-rounded Emacs tabulated-list row with a › accent cursor.
func TabulatedRow(c *ui.Context, key string, cols []TableColumn, selected bool, th Theme) ui.Element {
	row := ui.ButtonBase(c.Key(key)).Label(firstColLabel(cols)).
		Height(RowHeight + 2).Padding(0, 18).Gap(12).Radius(RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start)
	bg := ui.Transparent
	marker := "  "
	if selected {
		bg = th.SurfaceElevated
		marker = "› "
	} else if row.Hovered() {
		bg = th.SurfaceHover
	}
	row.Background(bg)
	row.Children(func() {
		ui.Text(c, marker).Font(FontMono).FontSize(FontSizeBody).FontWeight(700).TextColor(th.Accent).Width(16).NoWrap()
		for _, col := range cols {
			ink := col.Color
			if ink == (ui.Color{}) {
				ink = th.Ink
			}
			w := 400
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
		BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
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

// InlineInput renders a soft-rounded, legible monospace input field.
func InlineInput(c *ui.Context, key string, value *string, placeholder string, width float32, th Theme) ui.Element {
	var inp ui.Element
	row := ui.Row(c.Key(key+"-wrap")).Padding(6, 10).Background(th.Bar).
		Radius(RadiusControl).Border(1, th.Border).AlignItems(ui.Center)
	if width > 0 {
		row.Width(width)
	} else {
		row.Grow(1)
	}
	row.Children(func() {
		inp = ui.TextInputBase(c.Key(key), value).
			Placeholder(placeholder).Font(FontMono).FontSize(FontSizeUI).TextColor(th.Ink).Grow(1)
	})
	return inp
}

func firstColLabel(cols []TableColumn) string {
	if len(cols) == 0 {
		return ""
	}
	return cols[0].Label
}
