// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package tabs renders Marlin UI's authentic Doom Emacs sort-tab header strip:
// seamless dark bar, soft-rounded warm active tab pill (#504945 / #fbf1c7),
// quiet muted inactive tab labels separated by subtle "|" dividers, and zero
// chunky rectangular border boxes.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A04 (emacsTabs).
package tabs

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// FeatureTab describes one optional mounted feature buffer tab.
type FeatureTab struct {
	ID    string
	Title string
}

// Props configures one sort-tab frame pass.
type Props struct {
	OpenTabs        []buffer.ID
	ActiveBuffer    buffer.ID
	ActiveFeatureID string
	FeatureBuffers  []FeatureTab
	Theme           theme.Theme

	OnSelectBuffer  func(buffer.ID)
	OnSelectFeature func(featureBufferID string)
	OnNewSession    func()
	OnOpenHelm      func()
	OnOpenThemes    func()
	OnOpenSettings  func()
}

// Render paints the sleek borderless sort-tab strip at the top of the Doom Emacs frame.
func Render(c *ui.Context, p Props) {
	th := p.Theme
	ui.Row(c.Key("sort-tab-bar")).Height(theme.TabBarHeight).Background(th.Bar).
		Padding(0, 8).Gap(2).AlignItems(ui.Center).Children(func() {
		totalTabs := len(p.OpenTabs) + len(p.FeatureBuffers)
		idx := 0
		for _, id := range p.OpenTabs {
			bid := id
			active := p.ActiveBuffer == bid && p.ActiveFeatureID == ""
			if renderTab(c, "ctab-"+string(bid), string(bid), active, th).Clicked() {
				if p.OnSelectBuffer != nil {
					p.OnSelectBuffer(bid)
				}
			}
			idx++
			if idx < totalTabs {
				ui.Text(c, "|").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).
					TextColor(th.TabSeparator).Padding(0, 3).NoWrap()
			}
		}
		for _, fb := range p.FeatureBuffers {
			b := fb
			active := p.ActiveFeatureID == b.ID
			if renderTab(c, "ctab-feat-"+b.ID, "*"+b.Title+"*", active, th).Clicked() {
				if p.OnSelectFeature != nil {
					p.OnSelectFeature(b.ID)
				}
			}
			idx++
			if idx < totalTabs {
				ui.Text(c, "|").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).
					TextColor(th.TabSeparator).Padding(0, 3).NoWrap()
			}
		}

		ui.Spacer(c)

		if theme.ActionLink(c, "ctab-new", "+", th.TabInactiveFg, th).Clicked() && p.OnNewSession != nil {
			p.OnNewSession()
		}
	})
}

func renderTab(c *ui.Context, key, label string, active bool, th theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).
		Padding(3, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Center)
	bg := ui.Transparent
	ink := th.TabInactiveFg
	weight := 400
	if active {
		bg = th.TabActiveBg
		ink = th.TabActiveFg
		weight = 700
	} else if btn.Hovered() {
		bg = th.SurfaceHover
		ink = th.Ink
	}
	btn.Background(bg)
	btn.Children(func() {
		ui.Text(c, label).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(weight).TextColor(ink).NoWrap()
	})
	return btn
}
