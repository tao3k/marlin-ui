// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package hydra

import (
	"github.com/egoist/mygo/ui"

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

	ui.Column(c.Key("hydra-popup-"+body.ID)).Background(th.Bar).
		BorderWidth(1, 0, 0, 0).BorderColor(th.Border).
		Padding(10, 16).Gap(10).AlignItems(ui.Stretch).Children(func() {
		// Hydra header bar: prefix + title + quit hint.
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Row(c).Padding(2, 8).Radius(theme.RadiusSmall).Background(th.SurfaceElevated).Children(func() {
				ui.Text(c, body.Prefix+"-").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Accent).NoWrap()
			})
			ui.Text(c, body.Title).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()
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
					ui.Text(c, g.Name).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(th.InkMuted).Padding(0, 6, 3, 6).NoWrap()
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
	row := ui.ButtonBase(c.Key("hhead-"+key)).Label(h.Label).
		Height(28).Padding(0, 8).Gap(8).Radius(theme.RadiusSmall).
		AlignItems(ui.Center).Justify(ui.Start)
	if row.Hovered() {
		row.Background(th.SurfaceHover)
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

	row.Children(func() {
		ui.Row(c).Width(42).Padding(2, 6).Radius(theme.RadiusSmall).Background(th.SurfaceElevated).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, h.Key).Font(theme.FontMono).FontSize(12).FontWeight(700).TextColor(keyCol).NoWrap()
		})
		ui.Text(c, "→").Font(theme.FontMono).FontSize(12).TextColor(th.TabSeparator).NoWrap()
		ui.Text(c, h.Label).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(labelCol).Grow(1).NoWrap().MaxLines(1)
	})
	return row
}
