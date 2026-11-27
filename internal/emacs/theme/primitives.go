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
	Mono  bool // true for IDs, paths, keybindings, code, and tabular numbers
}

// ActionButton renders an Apple 26 HIG × Vercel Geist button backed by
// StyledButton (BtnGlassProminent when active, BtnGeistSecondary otherwise).
func ActionButton(c *ui.Context, key, label string, tone ui.Color, active bool, th Theme) ui.Element {
	variant := BtnGeistSecondary
	if active {
		variant = BtnGlassProminent
	} else if tone == th.Danger {
		variant = BtnDestructive
	}
	return StyledButton(c, ButtonSpec{
		Key:     key,
		Label:   label,
		Variant: variant,
		Tone:    tone,
	}, th)
}

// ActionLink renders a quiet Vercel Geist ghost button for chrome & action bars.
func ActionLink(c *ui.Context, key, label string, tone ui.Color, th Theme) ui.Element {
	return StyledButton(c, ButtonSpec{
		Key:     key,
		Label:   label,
		Variant: BtnGhost,
		Size:    BtnSizeSM,
		Tone:    tone,
	}, th)
}

// VerticoRow renders one soft-rounded Vertico / Helm candidate row styled by
// `.vertico-row`, `.vertico-row.selected`, `.vertico-row:hover`, and `.match-hit`.
func VerticoRow(c *ui.Context, key, name, desc, query string, nameWidth float32, selected bool, th Theme) ui.Element {
	return VerticoRowAnnotated(c, key, name, desc, "", query, nameWidth, selected, th)
}

// VerticoRowAnnotated renders a Vertico / Marginalia candidate row with an
// optional right-aligned monospace annotation pill (e.g., live keybindings
// `SPC w v · C-x 3` or window badges `win #2`) without shifting the title or
// description columns.
func VerticoRowAnnotated(c *ui.Context, key, name, desc, annotation, query string, nameWidth float32, selected bool, th Theme) ui.Element {
	if nameWidth <= 0 {
		nameWidth = 240
	}
	row := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(name).
		Height(RowHeight).MinWidth(0).Padding(0, 14).Gap(10).Radius(RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start).Clip())

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
		ui.Text(c, marker).Font(FontMono).FontSize(FontSizeBody).FontWeight(700).TextColor(th.Accent).Width(18).Shrink(0).NoWrap()
		renderHighlightedRuns(c, name, query, FontSizeUI, weight, nameCol, nameWidth, false, th)
		if desc != "" {
			renderHighlightedRuns(c, desc, query, FontSizeMeta, 400, th.InkMuted, 0, true, th)
		} else if annotation != "" {
			ui.Spacer(c)
		}
		if annotation != "" {
			pillFg := th.InkMuted
			pillBg := GlassTone(c, th.SurfaceElevated, th, 0.36, 0.62)
			pillRim := kit.WithAlpha(th.BorderSubtle, 0.55)
			if selected {
				pillFg = th.Accent
				pillBg = kit.WithAlpha(th.Accent, 0.14)
				pillRim = kit.WithAlpha(th.Accent, 0.34)
			}
			ui.Row(c.Key(key+"-marg")).Padding(2, 8).Radius(RadiusSmall).Shrink(0).
				Background(pillBg).Border(1, pillRim).AlignItems(ui.Center).Children(func() {
				ui.Text(c, annotation).Font(FontMono).FontSize(10.5).FontWeight(600).TextColor(pillFg).NoWrap()
			})
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
		// Use SingleLine().Ellipsis("…").Clip() (NOT NoWrap, which intentionally lets
		// overflowing text paint outside its bounding box on top of adjacent columns).
		el := ui.Text(c, text).Font(FontMono).FontSize(fontSize).FontWeight(baseWeight).TextColor(baseCol).
			SingleLine().Ellipsis("…").MinWidth(0).Clip()
		if fixedWidth > 0 {
			el.Width(fixedWidth).Shrink(0)
		} else if grow {
			el.Basis(0).Grow(1)
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

	box := ui.Row(c).AlignItems(ui.Center).MinWidth(0).Clip()
	if fixedWidth > 0 {
		box.Width(fixedWidth).Shrink(0)
	} else if grow {
		box.Basis(0).Grow(1)
	}
	box.Children(func() {
		for _, r := range runs {
			if r.Matched {
				ui.Row(c).Background(hitBg).Radius(hitRadius).Padding(0, 1).Shrink(0).AlignItems(ui.Center).Children(func() {
					ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(hitWeight).TextColor(hitFg).NoWrap()
				})
			} else {
				ui.Text(c, r.Text).Font(FontMono).FontSize(fontSize).FontWeight(baseWeight).TextColor(baseCol).Shrink(0).NoWrap()
			}
		}
	})
}

// TabulatedHeader renders a clean theme- and glass-aware Doom Emacs tabulated-list header strip.
func TabulatedHeader(c *ui.Context, cols []TableColumn, th Theme) {
	hdrBg := GlassTone(c, th.SurfaceElevated, th, 0.22, 0.44)
	row := ui.Row(c).Height(32).MinWidth(0).Padding(0, 18).Gap(12).AlignItems(ui.Center).Background(hdrBg).Clip()
	if c.Vibrancy() {
		row.BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.Ink, 0.10))
	}
	row.Children(func() {
		ui.Box(c).Width(16).Shrink(0)
		for _, col := range cols {
			t := ui.Text(c, col.Label).Font(kit.FontDisplay).FontSize(FontSizeMeta).FontWeight(600).
				TextColor(th.InkMuted).SingleLine().Ellipsis("…").MinWidth(0).Clip()
			if col.Width > 0 {
				t.Width(col.Width).Shrink(0)
			} else {
				t.Basis(0).Grow(1)
			}
		}
	})
}

// TabulatedRow renders one soft-rounded theme- and glass-aware Emacs tabulated-list row with a › accent cursor.
func TabulatedRow(c *ui.Context, key string, cols []TableColumn, selected bool, th Theme) ui.Element {
	rad := th.Sheet.VarPx("--radius-small", RadiusSmall)
	row := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(firstColLabel(cols)).
		Height(RowHeight+2).MinWidth(0).Padding(0, 18).Gap(12).Radius(rad).
		AlignItems(ui.Center).Justify(ui.Start).Clip())
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
		ui.Text(c, marker).Font(FontMono).FontSize(FontSizeBody).FontWeight(700).TextColor(th.Accent).Width(16).Shrink(0).NoWrap()
		for _, col := range cols {
			ink := col.Color
			if ink == (ui.Color{}) {
				ink = th.Ink
			}
			w := GlassWeight(c, 400)
			if col.Bold || selected {
				w = 600
			}
			fontFam := kit.FontDisplay
			if col.Mono {
				fontFam = FontMono
			}
			t := ui.Text(c, col.Label).Font(fontFam).FontSize(FontSizeUI).FontWeight(w).TextColor(ink).
				SingleLine().Ellipsis("…").MinWidth(0).Clip()
			if col.Width > 0 {
				t.Width(col.Width).Shrink(0)
			} else {
				t.Basis(0).Grow(1)
			}
		}
	})
	return row
}

// FieldRow renders a spacious, soft Emacs customize key/value row inside *Settings*.
func FieldRow(c *ui.Context, keyLabel, hint string, th Theme, control func()) {
	ui.Row(c).MinWidth(0).Padding(9, 20).Gap(14).AlignItems(ui.Center).Clip().
		BorderWidth(0, 0, 1, 0).BorderColor(GlassTone(c, th.BorderSubtle, th, 0.35, 0.45)).Children(func() {
		ui.Text(c, keyLabel).Font(kit.FontDisplay).FontSize(FontSizeUI).FontWeight(600).TextColor(th.Info).
			Width(210).Shrink(0).SingleLine().Ellipsis("…").Clip()
		ui.Row(c).Basis(0).Grow(1).MinWidth(0).Gap(10).AlignItems(ui.Center).Clip().Children(func() {
			if control != nil {
				control()
			}
			if hint != "" {
				ui.Text(c, hint).Font(kit.FontDisplay).FontSize(FontSizeMeta).TextColor(th.InkMuted).
					SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Clip()
			}
		})
	})
}

// InlineInput renders an Apple-styled native MyGo ui.TextInput field with
// monospace value rendering, subtle glass surface, and clean focus ring.
func InlineInput(c *ui.Context, key string, value *string, placeholder string, width float32, th Theme) ui.Element {
	cs := th.Sheet.Resolve([]string{"input"}, style.StateNone)
	rad := th.Sheet.VarPx("--radius-control", RadiusControl)
	fontSize := FontSizeUI
	if cs.HasFontSize {
		fontSize = cs.FontSize
	}
	textCol := th.Ink
	if cs.HasColor {
		textCol = cs.Color
	}
	inp := ui.TextInput(c.Key(key), value).
		Placeholder(placeholder).
		Font(FontMono).FontSize(fontSize).TextColor(textCol).
		Radius(rad).
		Background(GlassTone(c, th.Bar, th, 0.38, 0.64)).
		Border(1, GlassTone(c, th.Border, th, 0.55, 0.72))
	if width > 0 {
		inp.Width(width)
	} else {
		inp.Grow(1)
	}
	return inp
}

func firstColLabel(cols []TableColumn) string {
	if len(cols) == 0 {
		return ""
	}
	return cols[0].Label
}
