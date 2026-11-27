// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package window

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/emacs/workspace"
)

// SplitProps describes an active per-workspace recursive multi-window split
// tree (Doom persp-mode + winum 1..9 + authentic per-window Doom modeline).
type SplitProps struct {
	Mode               workspace.SplitMode
	SecondaryBuffer    buffer.ID
	FocusedPane        int
	Ratio              float32
	Tree               *workspace.WindowNode
	FocusedWindowID    string
	WindowCount        int
	ShowChrome         bool
	RenderPaneModeline func(c *ui.Context, leaf workspace.WindowLeafView)
	OnFocusWindow      func(windowID string)
	OnFocusPane        func(pane int)
	OnSwapPanes        func()
	OnCloseSplit       func()
}

// PopupProps describes an active Doom Emacs-governed popup drawer docked on the
// right or bottom edge of the primary Emacs Window without disturbing its layout.
type PopupProps struct {
	Visible   bool
	Buffer    buffer.ID
	Side      workspace.PopupSide
	Size      float32
	OnRaise   func()
	OnDismiss func()
}

// RenderBody paints the active Emacs buffer (or recursive per-workspace split
// window tree with authentic per-window Doom modelines), a live Scheme
// extension buffer, or a mounted feature renderer, optionally flanked by a
// non-destructive frosted-glass Doom Popup Drawer (set-popup-rule!).
func RenderBody(
	c *ui.Context,
	th theme.Theme,
	activeBuf buffer.ID,
	activeFeatID string,
	activeExt *schema.UIExtensionRegion,
	featHost *features.Host,
	bc *buffer.Context,
	mapCtx mapper.Context,
	split SplitProps,
	popup PopupProps,
) {
	renderMain := func() {
		if activeExt == nil && activeFeatID == "" && split.WindowCount > 1 && split.Tree != nil {
			renderWindowTree(c, th, bc, split)
			return
		}
		ui.Column(c.Key("doom-window")).Grow(3).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
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

	if !popup.Visible || popup.Buffer == "" {
		renderMain()
		return
	}

	if popup.Side == workspace.SideRight {
		ui.Row(c.Key("doom-window-popup-hsplit")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
			renderMain()
			renderPopupDrawer(c, th, bc, popup, true)
		})
		return
	}

	ui.Column(c.Key("doom-window-popup-vsplit")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
		renderMain()
		renderPopupDrawer(c, th, bc, popup, false)
	})
}

func renderWindowTree(c *ui.Context, th theme.Theme, bc *buffer.Context, split SplitProps) {
	// Pre-assign deterministic 1-based Doom `winum` reading-order numbers.
	winNum := 0
	var renderNode func(node *workspace.WindowNode, keyPath string, growWeight float32, dividerSide workspace.SplitMode)
	renderNode = func(node *workspace.WindowNode, keyPath string, growWeight float32, dividerSide workspace.SplitMode) {
		if node == nil {
			return
		}
		if node.IsSplit() {
			ratio := node.Ratio
			if ratio < 0.25 || ratio > 0.75 {
				ratio = 0.50
			}
			firstGrow := float32(int(ratio*100 + 0.5))
			secondGrow := float32(100) - firstGrow

			var container ui.Element
			if node.Direction == workspace.SplitVertical {
				container = ui.Row(c.Key("doom-split-"+keyPath)).Grow(growWeight).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip()
			} else {
				container = ui.Column(c.Key("doom-split-"+keyPath)).Grow(growWeight).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip()
			}
			applyDivider(c, container, th, dividerSide)
			container.Children(func() {
				renderNode(node.First, keyPath+"-a", firstGrow, "")
				renderNode(node.Second, keyPath+"-b", secondGrow, node.Direction)
			})
			return
		}

		winNum++
		leaf := workspace.WindowLeafView{
			ID:        node.ID,
			Number:    winNum,
			Buffer:    node.Buffer,
			Dedicated: node.Dedicated,
			Focused:   node.ID == split.FocusedWindowID,
		}
		pane := ui.Column(c.Key("doom-pane-"+leaf.ID)).
			Grow(growWeight).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip()
		applyDivider(c, pane, th, dividerSide)
		pane.Children(func() {
			// Subtle 1.5px top focus hairline (zero non-Emacs top header bar clutter!).
			focusAccent := ui.Transparent
			if leaf.Focused {
				focusAccent = kit.WithAlpha(th.Accent, 0.60)
			}
			ui.Box(c).Height(1.5).FillWidth().Background(focusAccent)

			ui.Column(c.Key("doom-pane-body-"+leaf.ID)).
				Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
				buffer.Render(c, leaf.Buffer, th, bc)
			})

			// Authentic per-window Doom Modeline at the bottom of EVERY split pane
			// (injected by frame to keep window -> modeline strictly acyclic).
			if split.ShowChrome && split.RenderPaneModeline != nil {
				split.RenderPaneModeline(c, leaf)
			}
		})
	}

	ui.Column(c.Key("doom-window-tree")).Grow(3).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
		renderNode(split.Tree, "root", 1, "")
	})
}

func applyDivider(c *ui.Context, el ui.Element, th theme.Theme, dir workspace.SplitMode) {
	if dir == "" {
		return
	}
	divCol := theme.GlassTone(c, th.BorderSubtle, th, 0.55, 0.72)
	if dir == workspace.SplitVertical {
		el.BorderWidth(0, 0, 0, 1).BorderColor(divCol)
	} else {
		el.BorderWidth(1, 0, 0, 0).BorderColor(divCol)
	}
}

func renderPopupDrawer(c *ui.Context, th theme.Theme, bc *buffer.Context, popup PopupProps, isRight bool) {
	drawer := ui.Column(c.Key("doom-popup-drawer")).Grow(2).
		Background(theme.GlassTone(c, th.Bar, th, 0.34, 0.58)).
		AlignItems(ui.Stretch).Clip()
	if isRight {
		drawer.MinWidth(320).BorderWidth(0, 0, 0, 1).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.45, 0.70))
	} else {
		drawer.MinHeight(210).BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.45, 0.70))
	}

	sideLabel := string(popup.Side)
	if sideLabel == "" {
		sideLabel = "bottom"
	}

	drawer.Children(func() {
		// Refined Doom Popup header strip: luminous accent dot, buffer title, side badge, Raise & Dismiss.
		ui.Row(c.Key("doom-popup-hdr")).Height(34).MinWidth(0).Padding(0, 14).Gap(8).AlignItems(ui.Center).Clip().
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.72)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			ui.Box(c).Width(7).Height(7).Shrink(0).Radius(theme.RadiusPill).Background(th.Accent)
			ui.Text(c, fmt.Sprintf("POPUP · %s", popup.Buffer)).
				Font(kit.FontDisplay).FontSize(12.5).FontWeight(700).TextColor(th.Ink).
				SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Clip()
			ui.Row(c).Padding(2, 7).Radius(theme.RadiusPill).
				Background(theme.GlassTone(c, th.Bar, th, 0.45, 0.68)).Children(func() {
				ui.Text(c, sideLabel).Font(theme.FontMono).FontSize(10.5).FontWeight(600).TextColor(th.Info).NoWrap()
			})
			ui.Spacer(c)
			if popup.OnRaise != nil {
				if theme.ActionButton(c, "popup-raise-btn", "Raise", th.Info, false, th).Clicked() {
					popup.OnRaise()
				}
			}
			if popup.OnDismiss != nil {
				if theme.ActionButton(c, "popup-dismiss-btn", "Dismiss", th.InkMuted, false, th).Clicked() {
					popup.OnDismiss()
				}
			}
		})

		ui.Column(c.Key("doom-popup-body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
			buffer.Render(c, popup.Buffer, th, bc)
		})
	})
}
