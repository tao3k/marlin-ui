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

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/style"

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
	Glass           bool
	GlassOpacity    float32

	OnSelectBuffer  func(buffer.ID)
	OnSelectFeature func(featureBufferID string)
	OnNewSession    func()
	OnOpenHelm      func()
	OnOpenThemes    func()
	OnOpenSettings  func()
}

// Render paints the sleek borderless sort-tab strip integrated directly into
// the transparent full-window titlebar (clearing macOS traffic lights via
// c.TitleBar() and acting as a native DragWindow handle).
func Render(c *ui.Context, p Props) {
	th := p.Theme
	tb := c.TitleBar()
	barH := max(tb.Height, theme.TabBarHeight)
	padLeft := max(tb.Left+8, float32(8))
	padRight := max(tb.Right+8, float32(8))

	barBg := th.Bar
	var borderBottom ui.Color
	hasBottomBorder := false
	if p.Glass && c.Vibrancy() {
		a := p.GlassOpacity
		if a <= 0 {
			a = 0.18
		}
		barBg = kit.WithAlpha(th.Bar, a)
		borderBottom = kit.WithAlpha(ui.RGB(255, 255, 255), 0.08)
		hasBottomBorder = true
	}

	bar := ui.Row(c.Key("sort-tab-bar")).Height(barH).Background(barBg).
		Padding(0, padRight, 0, padLeft).Gap(2).AlignItems(ui.Center).DragWindow()
	if hasBottomBorder {
		bar.BorderWidth(0, 0, 1, 0).BorderColor(borderBottom)
	}
	bar.Children(func() {
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
	btn := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(label).
		Padding(3, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Center))

	classes := []string{"tab"}
	if active {
		classes = append(classes, "active")
	}
	state := style.StateNone
	if active {
		state |= style.StateActive
	}
	if btn.Hovered() {
		state |= style.StateHover
	}
	cs := th.Sheet.Resolve(classes, state)

	bg := ui.Transparent
	ink := th.TabInactiveFg
	weight := 400
	fontSize := theme.FontSizeUI
	if active {
		bg = theme.GlassTone(c, th.TabActiveBg, th, 0.42, 0.66)
		ink = th.TabActiveFg
		weight = 700
		if c.Vibrancy() {
			btn.Border(1, kit.WithAlpha(th.Ink, 0.14))
		}
	} else if btn.Hovered() {
		bg = theme.GlassTone(c, th.SurfaceHover, th, 0.28, 0.46)
		ink = th.Ink
	}
	if cs.HasBackground {
		bg = cs.Background
	}
	if cs.HasColor {
		ink = cs.Color
	}
	if cs.HasFontWeight {
		weight = cs.FontWeight
	}
	if cs.HasFontSize {
		fontSize = cs.FontSize
	}
	btn.Background(bg)
	cs.ApplyToElement(btn)
	if !cs.HasBackground {
		btn.Background(bg)
	}
	btn.Children(func() {
		ui.Text(c, label).Font(theme.FontMono).FontSize(fontSize).FontWeight(weight).TextColor(ink).NoWrap()
	})
	return btn
}
