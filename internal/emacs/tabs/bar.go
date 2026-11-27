// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package tabs renders Marlin UI's authentic Doom Emacs Centaur / sort-tab
// header strip aligned with scheme/ui-centaur-tabs:
// seamless dark bar, soft-rounded active tab pill, multi-window winum badges
// (1/2/3), live streaming & modified-draft indicators, group dividers across
// Core Buffers / Feature Buffers / Scheme Extensions / Plugins, and
// overflow-safe horizontal scrolling.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A04 (emacsTabs).
package tabs

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
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

// FeatureTab describes one optional mounted feature, live Scheme extension,
// or marketplace plugin buffer tab.
type FeatureTab struct {
	ID    string
	Title string
	Group string // "feature" | "scheme-ext" | "plugin"
}

// Props configures one Centaur / sort-tab + workspace switcher frame pass.
type Props struct {
	Workspaces        []WorkspaceTab
	OpenTabs          []buffer.ID
	ActiveBuffer      buffer.ID
	ActiveFeatureID   string
	FeatureBuffers    []FeatureTab
	WindowBadges      map[buffer.ID]int  // pane numbers (1..9) when multi-window split is active
	BusyBuffers       map[buffer.ID]bool // live streaming / agent-working indicator dot
	DraftBuffers      map[buffer.ID]bool // unsaved composer draft (modified) indicator dot
	PopupLabel        string
	PopupVisible      bool
	Theme             theme.Theme
	Glass             bool
	GlassOpacity      float32

	OnSelectWorkspace func(id string)
	OnKillWorkspace   func(id string)
	OnNewWorkspace    func()
	OnTogglePopup     func()
	OnSelectBuffer    func(buffer.ID)
	OnSelectFeature   func(featureBufferID string)
	OnNewSession      func()
	OnOpenHelm        func()
	OnOpenThemes      func()
	OnOpenSettings    func()
}

type tabVisualMeta struct {
	winNum   int
	busy     bool
	modified bool
	group    string
}

// Render paints the sleek borderless Centaur / sort-tab strip integrated
// directly into the transparent full-window titlebar (clearing macOS traffic
// lights via c.TitleBar() and acting as a native DragWindow handle).
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
		if p.GlassOpacity > 0 {
			barBg = kit.WithAlpha(th.Bar, p.GlassOpacity)
		} else {
			barBg = ui.Transparent
		}
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
			ui.Row(c.Key("ws-pill-strip")).Gap(4).Padding(0, 6, 0, 0).AlignItems(ui.Center).Shrink(0).Children(func() {
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
					if len(p.Workspaces) > 1 && w.ID != "main" && w.Active && p.OnKillWorkspace != nil {
						if theme.ActionLink(c, "wsp-close-"+w.ID, "×", th.InkMuted, th).Clicked() {
							p.OnKillWorkspace(w.ID)
						}
					}
				}
				if p.OnNewWorkspace != nil {
					if theme.ActionLink(c, "wsp-new", "+ws", th.InkMuted, th).Clicked() {
						p.OnNewWorkspace()
					}
				}
			})
			renderGroupDivider(c, th)
		}

		// Overflow-safe horizontally scrollable Centaur tab strip so 8+ open buffers
		// never crush macOS traffic lights, workspace pills, or right-side Popup controls.
		ui.ScrollHorizontal(c.Key("sort-tab-strip")).Grow(1).MinWidth(0).Height(barH).Clip().Children(func() {
			ui.Row(c).Height(barH).Gap(2).AlignItems(ui.Center).Children(func() {
				for i, id := range p.OpenTabs {
					bid := id
					active := p.ActiveBuffer == bid && p.ActiveFeatureID == ""
					meta := tabVisualMeta{
						winNum:   p.WindowBadges[bid],
						busy:     p.BusyBuffers[bid],
						modified: p.DraftBuffers[bid],
						group:    "core",
					}
					if renderTab(c, "ctab-"+string(bid), string(bid), active, meta, th).Clicked() {
						if p.OnSelectBuffer != nil {
							p.OnSelectBuffer(bid)
						}
					}
					if i+1 < len(p.OpenTabs) {
						renderTabSep(c, th)
					}
				}

				if len(p.OpenTabs) > 0 && len(p.FeatureBuffers) > 0 {
					renderGroupDivider(c, th)
				}

				prevGroup := ""
				for i, fb := range p.FeatureBuffers {
					b := fb
					grp := b.Group
					if grp == "" {
						grp = "feature"
					}
					if i > 0 {
						if grp != prevGroup {
							renderGroupDivider(c, th)
						} else {
							renderTabSep(c, th)
						}
					}
					prevGroup = grp
					active := p.ActiveFeatureID == b.ID
					bid := buffer.ID(b.ID)
					meta := tabVisualMeta{
						winNum:   p.WindowBadges[bid],
						busy:     p.BusyBuffers[bid],
						modified: p.DraftBuffers[bid],
						group:    grp,
					}
					if renderTab(c, "ctab-feat-"+b.ID, "*"+b.Title+"*", active, meta, th).Clicked() {
						if p.OnSelectFeature != nil {
							p.OnSelectFeature(b.ID)
						}
					}
				}
			})
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

func renderTabSep(c *ui.Context, th theme.Theme) {
	ui.Text(c, "|").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).
		TextColor(th.TabSeparator).Padding(0, 2).NoWrap()
}

func renderGroupDivider(c *ui.Context, th theme.Theme) {
	ui.Row(c).Padding(0, 4).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Width(1).Height(14).Background(kit.WithAlpha(th.InkMuted, 0.28))
	})
}

func renderTab(c *ui.Context, key, label string, active bool, meta tabVisualMeta, th theme.Theme) ui.Element {
	btn := kit.ActionControl(ui.Button(c.Key(key), "").Label(label).
		Padding(4, 10).Radius(theme.RadiusSmall+2).Gap(5).Shrink(0).
		AlignItems(ui.Center).Justify(ui.Center).
		Background(ui.Transparent).Border(0, ui.Transparent))

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
	weight := 500
	fontSize := theme.FontSizeUI
	if active {
		ink = th.TabActiveFg
		weight = 600
		if c.Vibrancy() {
			lensTint := kit.WithAlpha(ui.RGB(255, 255, 255), 0.12)
			rim := kit.WithAlpha(ui.RGB(255, 255, 255), 0.20)
			if th.IsLight {
				lensTint = kit.WithAlpha(ui.RGB(0, 0, 0), 0.06)
				rim = kit.WithAlpha(ui.RGB(0, 0, 0), 0.10)
			}
			btn.Material(glass.Glass{Style: glass.Clear, Tint: lensTint, Interactive: true})
			btn.Border(1, rim)
		} else {
			bg = th.TabActiveBg
			btn.Background(bg).Border(1, kit.WithAlpha(th.Accent, 0.30))
		}
	} else if btn.Hovered() {
		bg = theme.GlassTone(c, th.SurfaceHover, th, 0.30, 0.48)
		ink = th.Ink
		btn.Background(bg)
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
	btn.Children(func() {
		// Circular winum badge when this buffer is currently visible in a split pane.
		if meta.winNum > 0 {
			badgeBg := kit.WithAlpha(th.InkMuted, 0.18)
			badgeFg := th.InkMuted
			if active {
				badgeBg = kit.WithAlpha(th.Accent, 0.26)
				badgeFg = th.Accent
			}
			ui.Row(c.Key(key+"-win")).Width(15).Height(15).Radius(theme.RadiusPill).
				Background(badgeBg).Justify(ui.Center).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Text(c, fmt.Sprintf("%d", meta.winNum)).Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(badgeFg).NoWrap()
			})
		}

		// Live streaming (sky-blue) or unsaved composer draft (amber) indicator dot.
		if meta.busy {
			ui.Box(c.Key(key+"-busy")).Width(6).Height(6).Radius(theme.RadiusPill).Background(th.Info).Shrink(0)
		} else if meta.modified {
			ui.Box(c.Key(key+"-mod")).Width(6).Height(6).Radius(theme.RadiusPill).Background(th.Warning).Shrink(0)
		} else if meta.group == "scheme-ext" {
			ui.Box(c.Key(key+"-ext")).Width(5).Height(5).Radius(theme.RadiusPill).Background(kit.WithAlpha(th.Accent, 0.72)).Shrink(0)
		}

		ui.Text(c, label).Font(kit.FontDisplay).FontSize(fontSize).FontWeight(weight).TextColor(ink).
			SingleLine().Ellipsis("…").MinWidth(0).Clip()
	})
	return btn
}
