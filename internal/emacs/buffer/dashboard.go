// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
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
			widgets.Render(c, node, sig, pal, th.Sheet, rt)
		})
	})
}
