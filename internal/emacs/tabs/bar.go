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
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/style"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// WorkspaceTab describes one isolated Doom perspective workspace pill.
type WorkspaceTab struct {
	ID     string
	Name   string
	Active bool
}

// FeatureTab describes one optional mounted feature buffer tab.
type FeatureTab struct {
	ID    string
	Title string
}

// Props configures one sort-tab + workspace switcher frame pass.
type Props struct {
	Workspaces        []WorkspaceTab
	OpenTabs          []buffer.ID
	ActiveBuffer      buffer.ID
	ActiveFeatureID   string
	FeatureBuffers    []FeatureTab
	PopupLabel        string
	PopupVisible      bool
	Theme             theme.Theme
	Glass             bool
	GlassOpacity      float32

	OnSelectWorkspace func(id string)
	OnNewWorkspace    func()
	OnTogglePopup     func()
	OnSelectBuffer    func(buffer.ID)
	OnSelectFeature   func(featureBufferID string)
	OnNewSession      func()
	OnOpenHelm        func()
	OnOpenThemes      func()
	OnOpenSettings    func()
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
		// Left-side sleek Doom :ui workspaces (persp-mode) pill switcher.
		if len(p.Workspaces) > 0 {
			ui.Row(c.Key("ws-pill-strip")).Gap(4).Padding(0, 6, 0, 0).AlignItems(ui.Center).Children(func() {
				for i, ws := range p.Workspaces {
					w := ws
					lbl := fmt.Sprintf("%d:%s", i+1, w.Name)
					pill := kit.ActionControl(ui.ButtonBase(c.Key("wsp-"+w.ID)).Label(lbl).
						Padding(2, 8).Radius(theme.RadiusPill).Gap(5).AlignItems(ui.Center).Justify(ui.Center))
					if w.Active {
						pill.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.50, 0.78))
						pill.Border(1, kit.WithAlpha(th.Accent, 0.42))
					} else if pill.Hovered() {
						pill.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.30, 0.48))
					}
					pill.Children(func() {
						dotCol := th.InkMuted
						txtCol := th.TabInactiveFg
						weight := 500
						if w.Active {
							dotCol = th.Accent
							txtCol = th.Ink
							weight = 700
						}
						ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(dotCol)
						ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(11.5).FontWeight(weight).TextColor(txtCol).NoWrap()
					})
					if pill.Clicked() && p.OnSelectWorkspace != nil {
						p.OnSelectWorkspace(w.ID)
					}
				}
				if p.OnNewWorkspace != nil {
					if theme.ActionLink(c, "wsp-new", "+ws", th.InkMuted, th).Clicked() {
						p.OnNewWorkspace()
					}
				}
			})
			ui.Box(c).Width(1).Height(14).Background(kit.WithAlpha(th.InkMuted, 0.25))
			ui.Box(c).Width(4)
		}

		// Overflow-protected middle buffer tab strip: clips cleanly so macOS traffic
		// lights, workspace switcher pills, and right-side Popup controls never collide.
		ui.Row(c.Key("sort-tab-strip")).Grow(1).MinWidth(0).Gap(2).AlignItems(ui.Center).Clip().Children(func() {
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
		})

		if p.PopupLabel != "" && p.OnTogglePopup != nil {
			popTone := th.InkMuted
			if p.PopupVisible {
				popTone = th.Accent
			}
			if theme.ActionButton(c, "ctab-popup-toggle", "Popup: "+p.PopupLabel, popTone, p.PopupVisible, th).Clicked() {
				p.OnTogglePopup()
			}
			ui.Box(c).Width(4)
		}

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
		ui.Text(c, label).Font(kit.FontDisplay).FontSize(fontSize).FontWeight(weight).TextColor(ink).NoWrap()
	})
	return btn
}
