// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// Classic Doom-style double-line ASCII banner matching the user's *doom* dashboard.
var doomMarlinBanner = []string{
	"======================================================================",
	" ||\\    //||     //\\\\      ||=====\\    ||        ||||    ||\\     || ",
	" ||\\\\  // ||    //  \\\\     ||     ||   ||         ||     ||\\\\    || ",
	" || \\\\//  ||   //====\\\\    ||=====/    ||         ||     || \\\\   || ",
	" ||       ||  //      \\\\   ||   \\\\     ||         ||     ||  \\\\  || ",
	" ||       || //        \\\\  ||    \\\\    ||======  ||||    ||   \\\\ || ",
	"======================================================================",
}

func renderConversation(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.Adaptive == nil || bc.Adaptive.Desktop == nil || bc.Adaptive.Desktop.Transcript == nil {
		return
	}
	tr := bc.Adaptive.Desktop.Transcript
	n := tr.Len()
	if n == 0 {
		renderDoomDashboard(c, th, bc)
		return
	}

	// Clean full-bleed monospace conversation stream (zero chat bubble cards).
	ui.Scroll(c.Key("conv-scroll")).Grow(1).Padding(18, 28).Children(func() {
		ui.Column(c).Gap(16).AlignItems(ui.Stretch).Children(func() {
			for i := 0; i < n; i++ {
				entry, ok := tr.Entry(i)
				if !ok {
					continue
				}
				roleText := "marlin ›"
				roleTone := th.Accent
				if entry.Role == mygoshell.RoleUser {
					roleText = "you ›"
					roleTone = th.Info
				} else if entry.Role == mygoshell.RoleTool {
					roleText = "tool ›"
					roleTone = th.Warning
				}
				ui.Column(c.Key(fmt.Sprintf("conv-row-%d", i))).Padding(0, 0, 12, 0).Gap(6).
					BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, roleText).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(roleTone).NoWrap()
						if entry.Streaming {
							kit.LoadingIndicator(c, 13, th.Info, false)
							ui.Text(c, "working...").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Info).NoWrap()
						}
						if len(entry.Attachments) > 0 {
							ui.Text(c, fmt.Sprintf("[%d attachment(s)]", len(entry.Attachments))).
								Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
						}
					})
					if entry.Role == mygoshell.RoleAssistant {
						mdOpts := &markdown.Options{WrapCode: true}
						if entry.Streaming {
							markdown.Stream(c.Key(fmt.Sprintf("conv-md-%d", i)), entry.Text, mdOpts)
						} else {
							markdown.View(c.Key(fmt.Sprintf("conv-md-%d", i)), entry.Text, mdOpts)
						}
					} else {
						ui.Text(c, entry.Text).Font(theme.FontMono).FontSize(theme.FontSizeBody).LineHeight(1.5).TextColor(th.Ink)
					}
				})
			}
		})
	})
}

func renderDoomDashboard(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("doom-dashboard")).Fill().Justify(ui.Center).AlignItems(ui.Center).Gap(22).Children(func() {
		// Iconic ASCII Banner in calm base5 slate gray (#5b6268).
		ui.Column(c.Key("doom-ascii-banner")).AlignItems(ui.Center).Gap(2).Children(func() {
			for i, line := range doomMarlinBanner {
				ui.Text(c.Key(fmt.Sprintf("banner-%d", i)), line).
					Font(theme.FontMono).FontSize(13).TextColor(th.BannerInk).NoWrap()
			}
			ui.Box(c).Height(6)
			ui.Text(c, "M A R L I N   A I").Font(theme.FontMono).FontSize(13.5).FontWeight(700).TextColor(th.InkMuted).NoWrap()
		})

		// Authentic Doom Dashboard menu: Doom blue labels on left, muted keybindings on right.
		ui.Column(c.Key("doom-dash-menu")).Width(470).Gap(3).AlignItems(ui.Stretch).Children(func() {
			entries := []struct {
				icon, label, key string
				action           func()
			}{
				{"▤", "Recently opened sessions", "SPC b b", func() {
					if bc.OnSelectBuffer != nil {
						bc.OnSelectBuffer(Sessions)
					}
				}},
				{"⌕", "Helm live search", "SPC s", bc.OnOpenHelm},
				{"⌘", "Execute command (M-x)", "SPC SPC", bc.OnOpenCommands},
				{"◎", "Open federation targets", "SPC t g", func() {
					if bc.OnSelectBuffer != nil {
						bc.OnSelectBuffer(Targets)
					}
				}},
				{"◐", "Switch Doom theme", "SPC t t", bc.OnOpenThemeSwitch},
				{"⚙", "Open private configuration", "Cmd+,", bc.OnOpenSettings},
				{"☰", "Open documentation (*Help*)", "SPC h", func() {
					if bc.OnSelectBuffer != nil {
						bc.OnSelectBuffer(Help)
					}
				}},
			}
			for _, e := range entries {
				item := e
				row := ui.ButtonBase(c.Key("dash-"+item.label)).Label(item.label).
					Height(30).Padding(0, 12).Gap(10).Radius(theme.RadiusControl).
					AlignItems(ui.Center).Justify(ui.Start)
				if row.Hovered() {
					row.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.34, 0.52))
				}
				row.Children(func() {
					ui.Text(c, item.icon).Font(theme.FontMono).FontSize(theme.FontSizeBody).TextColor(th.Info).Width(18).NoWrap()
					ui.Text(c, item.label).Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(500).TextColor(th.Info).Grow(1).NoWrap()
					ui.Text(c, item.key).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.InkMuted).NoWrap()
				})
				if row.Clicked() && item.action != nil {
					item.action()
				}
			}
		})

		// Quiet status summary footer + tiny green ready dot.
		ui.Column(c.Key("doom-dash-footer")).AlignItems(ui.Center).Gap(10).Children(func() {
			footer := fmt.Sprintf("Marlin AI · %d buffers ready · %s", len(Catalog()), th.Name)
			ui.Text(c, footer).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.BannerInk).NoWrap()
			ui.Box(c).Width(10).Height(10).Radius(theme.RadiusPill).Background(th.Success)
		})
	})
}
