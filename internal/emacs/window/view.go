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

// SplitProps describes an active per-workspace multi-pane window split
// (Doom SPC w v vertical or SPC w s horizontal).
type SplitProps struct {
	Mode            workspace.SplitMode
	SecondaryBuffer buffer.ID
	FocusedPane     int
	Ratio           float32 // primary pane ratio in [0.25, 0.75], default 0.50
	OnFocusPane     func(pane int)
	OnSwapPanes     func()
	OnCloseSplit    func()
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

// RenderBody paints the active Emacs buffer (or per-workspace split panes), a
// live Scheme extension buffer (via marlin-mygo/runtime/mapper), or a mounted
// feature renderer inside the primary Emacs Window, optionally flanked by a
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
		if activeExt == nil && activeFeatID == "" &&
			(split.Mode == workspace.SplitVertical || split.Mode == workspace.SplitHorizontal) &&
			split.SecondaryBuffer != "" {
			renderSplitPanes(c, th, activeBuf, bc, split)
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

func renderSplitPanes(c *ui.Context, th theme.Theme, primaryBuf buffer.ID, bc *buffer.Context, split SplitProps) {
	ratio := split.Ratio
	if ratio < 0.25 || ratio > 0.75 {
		ratio = 0.50
	}
	primaryGrow := float32(int(ratio*100 + 0.5))
	secondaryGrow := float32(100) - primaryGrow

	renderPane := func(paneIdx int, buf buffer.ID, growWeight float32, showDivider bool) {
		focused := split.FocusedPane == paneIdx
		pane := ui.Column(c.Key(fmt.Sprintf("doom-split-pane-%d", paneIdx))).
			Grow(growWeight).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip()
		if showDivider {
			divCol := theme.GlassTone(c, th.BorderSubtle, th, 0.55, 0.72)
			if split.Mode == workspace.SplitVertical {
				pane.BorderWidth(0, 0, 0, 1).BorderColor(divCol)
			} else {
				pane.BorderWidth(1, 0, 0, 0).BorderColor(divCol)
			}
		}
		pane.Children(func() {
			hdrBg := theme.GlassTone(c, th.Bar, th, 0.30, 0.52)
			accentLine := th.BorderSubtle
			dotCol := th.InkMuted
			titleCol := th.InkMuted
			weight := 500
			if focused {
				hdrBg = theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.68)
				accentLine = kit.WithAlpha(th.Accent, 0.65)
				dotCol = th.Accent
				titleCol = th.Ink
				weight = 700
			}
			hdr := ui.Row(c.Key(fmt.Sprintf("doom-split-hdr-%d", paneIdx))).
				Height(28).Padding(0, 12).Gap(8).AlignItems(ui.Center).
				Background(hdrBg).BorderWidth(0, 0, 1, 0).BorderColor(accentLine)
			hdr.Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(dotCol)
				lbl := fmt.Sprintf("[%d] %s", paneIdx+1, buf)
				btn := ui.ButtonBase(c.Key(fmt.Sprintf("doom-split-focus-%d", paneIdx))).
					Label(lbl).Padding(2, 4).Radius(theme.RadiusSmall).AlignItems(ui.Center)
				btn.Children(func() {
					ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(12).FontWeight(weight).TextColor(titleCol).NoWrap()
				})
				if btn.Clicked() && split.OnFocusPane != nil {
					split.OnFocusPane(paneIdx)
				}
				if focused {
					ui.Row(c).Padding(1, 6).Radius(theme.RadiusPill).
						Background(kit.WithAlpha(th.Accent, 0.16)).Children(func() {
						ui.Text(c, "ACTIVE").Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(th.Accent).NoWrap()
					})
				}
				ui.Spacer(c)
				if paneIdx == 1 && split.OnSwapPanes != nil {
					if theme.ActionLink(c, "doom-split-swap", "⇄", th.InkMuted, th).Clicked() {
						split.OnSwapPanes()
					}
				}
				if paneIdx == 1 && split.OnCloseSplit != nil {
					if theme.ActionLink(c, "doom-split-close", "×", th.InkMuted, th).Clicked() {
						split.OnCloseSplit()
					}
				}
			})
			ui.Column(c.Key(fmt.Sprintf("doom-split-body-%d", paneIdx))).
				Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
				buffer.Render(c, buf, th, bc)
			})
		})
	}

	if split.Mode == workspace.SplitVertical {
		ui.Row(c.Key("doom-window-vsplit")).Grow(3).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
			renderPane(0, primaryBuf, primaryGrow, false)
			renderPane(1, split.SecondaryBuffer, secondaryGrow, true)
		})
		return
	}
	ui.Column(c.Key("doom-window-hsplit")).Grow(3).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
		renderPane(0, primaryBuf, primaryGrow, false)
		renderPane(1, split.SecondaryBuffer, secondaryGrow, true)
	})
}

func renderPopupDrawer(c *ui.Context, th theme.Theme, bc *buffer.Context, popup PopupProps, isRight bool) {
	drawer := ui.Column(c.Key("doom-popup-drawer")).Grow(2).
		Background(theme.GlassTone(c, th.Bar, th, 0.56, 0.84)).
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
		ui.Row(c.Key("doom-popup-hdr")).Height(34).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.72)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(th.Accent)
			ui.Text(c, fmt.Sprintf("POPUP · %s", popup.Buffer)).
				Font(kit.FontDisplay).FontSize(12.5).FontWeight(700).TextColor(th.Ink).NoWrap()
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
