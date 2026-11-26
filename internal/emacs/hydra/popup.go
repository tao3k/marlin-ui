// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package hydra

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/style"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// RenderPopup paints the multi-column Doom Emacs Which-Key / Hydra Minibuffer
// drawer cleanly above the Echo Area. Every head is both keyboard-accessible
// and mouse-clickable.
func RenderPopup(c *ui.Context, body Body, th theme.Theme, onExit func()) {
	cols := body.GroupColumns()
	if len(cols) == 0 {
		return
	}

	bg := theme.GlassTone(c, th.Bar, th, 0.54, 0.72)
	ui.Column(c.Key("hydra-popup-"+body.ID)).Background(bg).
		BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassTone(c, th.Border, th, 0.45, 0.55)).
		Padding(10, 16).Gap(10).AlignItems(ui.Stretch).Children(func() {
		// Hydra header bar: prefix keycap + proportional title + quit hint.
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Row(c).Padding(2, 8).Radius(theme.RadiusSmall).
				Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.50, 0.74)).
				BorderWidth(1).BorderColor(kit.WithAlpha(th.Accent, 0.34)).Children(func() {
				ui.Text(c, body.Prefix+"-").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Accent).NoWrap()
			})
			ui.Text(c, body.Title).Font(kit.FontDisplay).FontSize(13).FontWeight(700).TextColor(th.Ink).NoWrap()
			ui.Spacer(c)
			if theme.ActionLink(c, "hydra-cancel", "C-g / ESC quit", th.InkMuted, th).Clicked() && onExit != nil {
				onExit()
			}
		})

		// Multi-column aligned Which-Key / Hydra hint table.
		ui.Row(c).Gap(24).AlignItems(ui.Start).Children(func() {
			for _, grp := range cols {
				g := grp
				ui.Column(c.Key("hydra-col-"+g.Name)).Grow(1).Gap(3).AlignItems(ui.Stretch).Children(func() {
					ui.Text(c, g.Name).Font(kit.FontDisplay).FontSize(11.5).FontWeight(700).TextColor(th.InkMuted).Padding(0, 6, 3, 6).NoWrap()
					for _, hd := range g.Heads {
						h := hd
						if renderHeadRow(c, body.ID+"-"+h.Key, h, th).Clicked() {
							if h.Run != nil {
								h.Run()
							}
							if h.Exit && !h.IsPrefix && onExit != nil {
								onExit()
							}
						}
					}
				})
			}
		})
	})
}

func renderHeadRow(c *ui.Context, key string, h Head, th theme.Theme) ui.Element {
	row := ui.ButtonBase(c.Key("hhead-"+key)).Label(h.Label).KeepFocus().FocusRing(false).
		Height(28).Padding(0, 8).Gap(8).Radius(theme.RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start)
	if row.Hovered() {
		row.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.30, 0.48))
	}

	keyCol := th.Accent // blue/magenta exit head
	if !h.Exit && !h.IsPrefix {
		keyCol = th.Danger // classic Hydra red repeatable head
	} else if h.IsPrefix {
		keyCol = th.Info // Doom +prefix group
	}
	labelCol := th.Ink
	if h.IsPrefix {
		labelCol = th.Info
	}

	keyClasses := []string{"hydra-key"}
	if !h.Exit && !h.IsPrefix {
		keyClasses = append(keyClasses, "repeatable")
	} else if h.IsPrefix {
		keyClasses = append(keyClasses, "prefix")
	}
	kcs := th.Sheet.Resolve(keyClasses, style.StateNone)
	kWeight := 700
	if kcs.HasFontWeight {
		kWeight = kcs.FontWeight
	}
	if kcs.HasColor {
		keyCol = kcs.Color
	}

	row.Children(func() {
		badge := ui.Row(c).Width(42).Padding(2, 6).Radius(theme.RadiusSmall).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.48, 0.72)).
			BorderWidth(1).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.75)).
			Justify(ui.Center).AlignItems(ui.Center)
		kcs.ApplyToElement(badge)
		badge.Children(func() {
			ui.Text(c, h.Key).Font(theme.FontMono).FontSize(12).FontWeight(kWeight).TextColor(keyCol).NoWrap()
		})
		ui.Text(c, "→").Font(theme.FontMono).FontSize(11.5).TextColor(th.TabSeparator).NoWrap()
		ui.Text(c, h.Label).Font(kit.FontDisplay).FontSize(theme.FontSizeUI).FontWeight(500).TextColor(labelCol).Grow(1).NoWrap().MaxLines(1)
	})
	return row
}
