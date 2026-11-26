// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package window renders Marlin UI's primary Emacs Window viewport (occupying
// 90%+ of frame height), hosting the focused native buffer, a live Scheme
// UIExtensionRegion mapped through marlin-mygo/runtime/mapper, or a compiled
// feature renderer.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A05 (emacsWindow).
package window

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// RenderBody paints the active Emacs buffer, a live Scheme extension buffer
// (via marlin-mygo/runtime/mapper), or a mounted feature renderer inside the
// full-bleed primary Emacs Window.
func RenderBody(
	c *ui.Context,
	th theme.Theme,
	activeBuf buffer.ID,
	activeFeatID string,
	activeExt *schema.UIExtensionRegion,
	featHost *features.Host,
	bc *buffer.Context,
	mapCtx mapper.Context,
) {
	ui.Column(c.Key("doom-window")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
		if activeExt != nil {
			ui.Scroll(c.Key("scheme-ext-scroll-"+activeExt.ID)).Grow(1).Padding(16, 24).Children(func() {
				mapper.MapElement(c, activeExt.Content, th.ToMapperSkin(), mapCtx)
			})
			return
		}
		if activeFeatID != "" && featHost != nil {
			if renderFn, ok := featHost.Renderer(activeFeatID); ok {
				renderFn(c, c.Theme())
				return
			}
		}
		buffer.Render(c, activeBuf, th, bc)
	})
}
