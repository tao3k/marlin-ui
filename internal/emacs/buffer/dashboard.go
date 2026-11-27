// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderDashboard(c *ui.Context, th theme.Theme, bc *Context) {
	var node *widgets.Node
	var sig *widgets.Signals
	var rt *widgets.Runtime
	zen := false
	if bc != nil {
		node = bc.DashboardNode
		sig = bc.Signals
		rt = bc.WidgetRuntime
		zen = bc.ZenMode
	}
	if node == nil {
		node = widgets.DefaultDashboard(nil)
	}
	if sig == nil {
		sig = widgets.NewSignals()
	}
	if rt == nil {
		rt = &widgets.Runtime{Modules: widgets.DefaultModules()}
		if bc != nil {
			rt.OnAction = bc.OnWidgetAction
		}
	}

	pal := th.ToKitPalette()
	opts := kit.SurfaceOptions{Glass: rt.Modules.Glass, GlassOpacity: rt.Modules.GlassOpacity, Motion: rt.Modules.Motion}
	stageBg := th.Canvas
	if rt.Modules.Glass {
		stageBg = ui.Transparent
	}
	kit.SafeCol(c, "bento-dashboard-root-stage", 0).Fill().Background(stageBg).AlignItems(ui.Stretch).Children(func() {
		kit.RenderGlassAtmosphere(c, pal, opts)
		kit.SafeCol(c, "bento-dashboard-tile-area", 12).Fill().Padding(16, 20).AlignItems(ui.Stretch).Children(func() {
			if zen {
				ui.Row(c).Justify(ui.End).AlignItems(ui.Center).Gap(8).Children(func() {
					if theme.ActionLink(c, "zen-exit-pill", "Zen Mode · SPC t z to exit", th.InkMuted, th).Clicked() && bc != nil && bc.OnToggleZenMode != nil {
						bc.OnToggleZenMode()
					}
				})
			}
			renderDashboardNode(c, node, sig, pal, th, rt, 0)
		})
	})
}

func dashboardHasProfileNode(n *widgets.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == widgets.KindProfile {
		return true
	}
	for _, ch := range n.Children {
		if dashboardHasProfileNode(ch) {
			return true
		}
	}
	return false
}

func dashboardEvalVisibleWhen(expr string, sig *widgets.Signals) bool {
	e := strings.TrimSpace(expr)
	if e == "" {
		return true
	}
	if sig == nil {
		return false
	}
	if lhs, rhs, ok := strings.Cut(e, "!="); ok {
		return strings.TrimSpace(sig.Get(strings.TrimSpace(lhs))) != strings.TrimSpace(rhs)
	}
	if lhs, rhs, ok := strings.Cut(e, "=="); ok {
		return strings.TrimSpace(sig.Get(strings.TrimSpace(lhs))) == strings.TrimSpace(rhs)
	}
	neg := false
	if strings.HasPrefix(e, "!") {
		neg = true
		e = strings.TrimSpace(strings.TrimPrefix(e, "!"))
	}
	v := strings.ToLower(strings.TrimSpace(sig.Get(e)))
	truthy := v != "" && v != "0" && v != "false" && v != "off"
	if neg {
		return !truthy
	}
	return truthy
}

func dashboardApplySizing(el ui.Element, n *widgets.Node) {
	if n.Width > 0 {
		el.Width(n.Width)
	}
	if n.Height > 0 {
		el.Height(n.Height)
	}
	if n.Flex > 0 {
		el.Grow(n.Flex)
	}
}

// renderDashboardNode delegates all standard Bento widgets to widgets.Render
// while overriding KindProfile cards to display Marlin's unified sculpted
// MarlinEmblem (matching iOS/Android & Desktop *conversation* brand identity).
func renderDashboardNode(c *ui.Context, n *widgets.Node, sig *widgets.Signals, pal kit.Palette, th theme.Theme, rt *widgets.Runtime, idx int) {
	if n == nil || !dashboardEvalVisibleWhen(n.VisibleWhen, sig) {
		return
	}
	if !dashboardHasProfileNode(n) {
		widgets.Render(c, n, sig, pal, th.Sheet, rt)
		return
	}

	key := n.ID
	if key == "" {
		key = fmt.Sprintf("w-%s-%d", n.Kind, idx)
	}
	opts := kit.SurfaceOptions{
		Glass:        rt.Modules.Glass,
		GlassOpacity: rt.Modules.GlassOpacity,
		Motion:       rt.Modules.Motion,
	}

	switch n.Kind {
	case widgets.KindRow:
		gap := n.Gap
		if gap <= 0 {
			gap = 12
		}
		el := kit.SafeRow(c, key, gap).AlignItems(ui.Stretch)
		dashboardApplySizing(el, n)
		el.Children(func() {
			for i, ch := range n.Children {
				renderDashboardNode(c, ch, sig, pal, th, rt, i)
			}
		})

	case widgets.KindCol:
		gap := n.Gap
		if gap <= 0 {
			gap = 12
		}
		el := kit.SafeCol(c, key, gap).AlignItems(ui.Stretch)
		dashboardApplySizing(el, n)
		el.Children(func() {
			for i, ch := range n.Children {
				renderDashboardNode(c, ch, sig, pal, th, rt, i)
			}
		})

	case widgets.KindScroll:
		gap := n.Gap
		if gap <= 0 {
			gap = 8
		}
		sc := ui.Scroll(c.Key(key)).MinHeight(0)
		dashboardApplySizing(sc, n)
		if n.Flex == 0 && n.Height == 0 {
			sc.Grow(1)
		}
		sc.Children(func() {
			kit.SafeCol(c, key+"-in", gap).AlignItems(ui.Stretch).Children(func() {
				for i, ch := range n.Children {
					renderDashboardNode(c, ch, sig, pal, th, rt, i)
				}
			})
		})

	case widgets.KindCard:
		renderDashboardCardShell(c, key, n, sig, pal, th, rt, opts, func() {
			if title := strings.TrimSpace(sig.Interpolate(n.Title)); title != "" {
				kit.SafeRow(c, key+"-hdr", 8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					kit.Eyebrow(c, title, pal.Muted)
					if b := strings.TrimSpace(sig.Interpolate(n.Badge)); b != "" {
						ui.Row(c).Padding(2, 8).Radius(kit.RadiusPill).
							Background(kit.WithAlpha(pal.Success, 0.16)).AlignItems(ui.Center).Gap(5).Children(func() {
							ui.Box(c).Width(6).Height(6).Radius(kit.RadiusPill).Background(pal.Success)
							kit.Line(c, b, kit.TextSpec{Font: kit.FontDisplay, Size: 10.5, Weight: 600, Color: pal.Success})
						})
					}
				})
			}
			for i, ch := range n.Children {
				renderDashboardNode(c, ch, sig, pal, th, rt, i)
			}
		})

	case widgets.KindProfile:
		renderDashboardProfileCard(c, key, n, sig, pal, th, rt, opts)

	default:
		widgets.Render(c, n, sig, pal, th.Sheet, rt)
	}
}

func renderDashboardCardShell(
	c *ui.Context,
	key string,
	n *widgets.Node,
	sig *widgets.Signals,
	pal kit.Palette,
	th theme.Theme,
	rt *widgets.Runtime,
	opts kit.SurfaceOptions,
	body func(),
) {
	classes := append([]string{"bento-card"}, n.Classes...)
	gap := n.Gap
	if gap <= 0 {
		gap = 8
	}
	if n.OnClick != "" {
		lbl := sig.Interpolate(n.Title)
		if strings.TrimSpace(lbl) == "" {
			lbl = key
		}
		btn := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(lbl).
			MinWidth(0).MinHeight(0).Padding(14, 16).Clip().
			AlignItems(ui.Stretch).Justify(ui.Center))
		dashboardApplySizing(btn, n)
		kit.ApplyCardSurface(btn, classes, btn.Hovered(), pal, th.Sheet, opts)
		btn.Children(func() {
			kit.SafeCol(c, key+"-in", gap).Fill().AlignItems(ui.Stretch).Justify(ui.SpaceBetween).Children(body)
		})
		if btn.Clicked() && rt.OnAction != nil {
			rt.OnAction(sig.Interpolate(n.OnClick))
		}
		return
	}
	card := kit.SafeCol(c, key, gap).Padding(14, 16).AlignItems(ui.Stretch).Justify(ui.SpaceBetween)
	dashboardApplySizing(card, n)
	kit.ApplyCardSurface(card, classes, false, pal, th.Sheet, opts)
	card.Children(body)
}

func renderDashboardProfileCard(
	c *ui.Context,
	key string,
	n *widgets.Node,
	sig *widgets.Signals,
	pal kit.Palette,
	th theme.Theme,
	rt *widgets.Runtime,
	opts kit.SurfaceOptions,
) {
	cardRad := kit.ResolveRadius(th.Sheet, "--radius-card", kit.RadiusCard)
	renderDashboardCardShell(c, key, n, sig, pal, th, rt, opts, func() {
		kit.SafeCol(c, key+"-body", 10).Fill().Justify(ui.SpaceBetween).AlignItems(ui.Stretch).Children(func() {
			kit.SafeRow(c, key+"-top", 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				kit.Eyebrow(c, "WORKSPACE", pal.Muted)
				if badge := sig.Interpolate(n.Badge); badge != "" {
					ui.Row(c).Padding(3, 9).Radius(kit.RadiusPill).
						Background(kit.WithAlpha(pal.Success, 0.16)).AlignItems(ui.Center).Gap(5).Children(func() {
						dot := ui.Box(c.Key(key+"-dot")).Width(6).Height(6).Radius(kit.RadiusPill).Background(pal.Success)
						if opts.Motion {
							pulse := float32(0.55)
							if time.Now().Second()%2 == 0 {
								pulse = 1.0
							}
							dot.Opacity(dot.AnimateTo("pulse", pulse, ui.Smooth))
						}
						kit.Line(c, badge, kit.TextSpec{Font: kit.FontDisplay, Size: 10.5, Weight: 700, Color: pal.Success})
					})
				}
			})

			kit.SafeCol(c, key+"-center", 10).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				// Unified Marlin brand identity across Desktop, iOS, and Android:
				// sculpted liquid-glass blue marlin medallion with serene GPU caustic sheen.
				theme.MarlinEmblem(c, key+"-marlin", 84, th)
				kit.SafeCol(c, key+"-titles", 2).AlignItems(ui.Center).Children(func() {
					kit.Headline(c, sig.Interpolate(n.Title), 18, pal.Ink)
					if sub := sig.Interpolate(n.Subtitle); sub != "" {
						kit.Caption(c, sub, pal.Muted)
					}
				})
			})

			modelVal := strings.TrimSpace(sig.Interpolate(n.Value))
			hostVal := strings.TrimSpace(sig.Interpolate(n.Body))
			if modelVal != "" || hostVal != "" {
				ui.Column(c).Radius(kit.ConcentricRadius(cardRad, 8)).Background(kit.WithAlpha(pal.Bar, 0.62)).
					BorderWidth(1).BorderColor(kit.WithAlpha(ui.RGB(255, 255, 255), 0.08)).
					InsetShadow(0, 1, 2, 0, kit.WithAlpha(ui.RGB(0, 0, 0), 0.24)).
					Padding(8, 10).Gap(4).AlignItems(ui.Stretch).Children(func() {
					if modelVal != "" {
						kit.SafeRow(c, key+"-mod", 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							kit.Caption(c, "Model", pal.Muted)
							kit.Line(c, modelVal, kit.TextSpec{Font: kit.FontMono, Size: 11, Weight: 600, Color: pal.Info})
						})
					}
					if hostVal != "" {
						kit.SafeRow(c, key+"-host", 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							kit.Caption(c, "Endpoint", pal.Muted)
							kit.Line(c, hostVal, kit.TextSpec{Font: kit.FontMono, Size: 10.5, Weight: 500, Color: pal.Ink})
						})
					}
				})
			}
		})
	})
}
