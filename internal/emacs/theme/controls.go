// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
)

// ButtonVariant selects a canonical Apple 26 HIG × Vercel Geist button surface.
type ButtonVariant uint8

const (
	// BtnGeistSecondary renders a Vercel Geist elevated tactile surface with
	// directional top-edge specular sheen and crisp neutral ink (for content views).
	BtnGeistSecondary ButtonVariant = iota
	// BtnGlassProminent renders an Apple iOS/macOS 26 .glassProminent CTA using
	// luminous accented surface + specular rim and crisp bold pure-white glyph/text.
	BtnGlassProminent
	// BtnGlass renders an Apple iOS/macOS 26 .glass floating navigation/toolbar
	// lens with pure GPU optical refraction and interactive rim highlight.
	BtnGlass
	// BtnTinted renders a soft translucent tinted pill for active semantic states.
	BtnTinted
	// BtnGhost renders a quiet Vercel Geist toolbar action (chrome appears on hover).
	BtnGhost
	// BtnDestructive renders a restrained coral glass/tinted destructive control.
	BtnDestructive
	// BtnSolidIdle renders a borderless, refined filled circular/pill well for
	// controls nested INSIDE an outer Liquid Glass container (obeys Apple iOS 26
	// HIG's rule against stacking raw glass lenses on top of glass).
	BtnSolidIdle
)

// ButtonSize controls Apple HIG ergonomic padding and typography scale.
type ButtonSize uint8

const (
	BtnSizeSM ButtonSize = iota
	BtnSizeMD
	BtnSizeLG
)

// ButtonSpec declaratively configures a reusable Apple × Geist button across
// Desktop, iOS, and Web.
type ButtonSpec struct {
	Key        string
	Label      string
	Icon       string // optional Apple SF Symbol name (see theme.Icon* constants)
	IconFilled bool
	Variant    ButtonVariant
	Size       ButtonSize
	Tone       ui.Color // optional semantic override (defaults to th.Accent / th.Danger)
	Pill       bool
	FullWidth  bool
}

// StyledButton renders a unified Apple 26 HIG × Vercel Geist button backed by
// MyGo's official ui.ButtonBase and plugins/glass GPU materials. It uses
// directional top-edge specular lighting (never flat uniform boxes),
// interactive optical rim refraction on glass surfaces, and Meow-safe
// non-focus-stealing input handling.
func StyledButton(c *ui.Context, spec ButtonSpec, th Theme) ui.Element {
	btn := kit.ActionControl(ui.ButtonBase(c.Key(spec.Key)).Label(spec.Label))

	padV, padH, fontSize, iconSize, rad := buttonMetrics(spec, th)
	btn.Padding(padV, padH).Radius(rad).Gap(6).AlignItems(ui.Center).Justify(ui.Center).Clip().
		Background(ui.Transparent).Border(0, ui.Transparent)
	if spec.FullWidth {
		btn.Basis(0).Grow(1).MinWidth(0)
	}

	fg, weight := applyButtonMaterial(c, btn, spec, th, rad)

	btn.Children(func() {
		if spec.Icon != "" {
			symW := sfsymbols.Medium
			if spec.Variant == BtnGlassProminent || spec.Variant == BtnTinted {
				symW = sfsymbols.Semibold
			}
			SFIconWeighted(c, spec.Icon, iconSize, fg, symW, spec.IconFilled, true)
		}
		if spec.Label != "" {
			ui.Text(c, spec.Label).Font(kit.FontDisplay).FontSize(fontSize).FontWeight(weight).TextColor(fg).
				SingleLine().Ellipsis("…").MinWidth(0).Clip()
		}
	})
	return btn
}

// IconCircleButton renders an optically centered circular Apple 26 Liquid Glass
// or solid-prominent CTA icon button (e.g., Composer Send, Stop, Deep Think,
// Drawer toggle, Mention). Built directly on ui.ButtonBase with zero padding
// and Semibold/Bold SF Symbols so glyphs always sit dead-center inside their
// circular hull without drift.
func IconCircleButton(c *ui.Context, key, a11yLabel, icon string, diameter float32, variant ButtonVariant, tone ui.Color, filled bool, th Theme) ui.Element {
	if diameter <= 0 {
		diameter = 34
	}
	btn := kit.ActionControl(ui.ButtonBase(c.Key(key)).Label(a11yLabel))
	rad := diameter / 2
	btn.Width(diameter).Height(diameter).Shrink(0).Padding(0).Gap(0).Radius(rad).
		AlignItems(ui.Center).Justify(ui.Center).Clip().
		Background(ui.Transparent).Border(0, ui.Transparent)

	fg, _ := applyButtonMaterial(c, btn, ButtonSpec{
		Variant: variant,
		Tone:    tone,
		Pill:    true,
	}, th, rad)

	// Optical sizing & Apple iOS 26 stroke weight: prominent send/stop arrows
	// render crisp Bold, while navigation/utility icons render clean Semibold.
	// Cap icon size at 19pt so larger circular buttons (44–52pt) stay refined
	// and harmonious with adjacent toolbar & tab-bar glyphs.
	iconSz := diameter * 0.44
	symWeight := sfsymbols.Semibold
	if icon == IconSend || icon == IconStop || icon == IconPlus {
		iconSz = diameter * 0.46
		if variant == BtnGlassProminent {
			symWeight = sfsymbols.Bold
		}
	} else if icon == IconSearch {
		iconSz = min(diameter*0.37, 18.5)
		symWeight = sfsymbols.Medium
	}
	if iconSz < 13 {
		iconSz = 13
	}
	if iconSz > 19 {
		iconSz = 19
	}
	btn.Children(func() {
		SFIconWeighted(c, icon, iconSz, fg, symWeight, filled, false)
	})
	return btn
}

func buttonMetrics(spec ButtonSpec, th Theme) (padV, padH, fontSize, iconSize, rad float32) {
	switch spec.Size {
	case BtnSizeLG:
		padV, padH, fontSize, iconSize, rad = 9, 18, 14, 14.5, 13
	case BtnSizeSM:
		padV, padH, fontSize, iconSize, rad = 4, 10, 12, 12.5, 8
	default:
		padV, padH, fontSize, iconSize, rad = 6, 13, 12.5, 13, th.Sheet.VarPx("--radius-control", RadiusControl)
	}
	if spec.Pill {
		rad = RadiusPill
	}
	return padV, padH, fontSize, iconSize, rad
}

func applyButtonMaterial(c *ui.Context, btn ui.Element, spec ButtonSpec, th Theme, rad float32) (fg ui.Color, weight int) {
	accent := spec.Tone
	if accent == (ui.Color{}) {
		accent = th.Accent
	}
	weight = 500
	fg = th.Ink

	switch spec.Variant {
	case BtnGlassProminent:
		// Apple iOS 26 prominent circular/pill CTA: rich vibrant accent gradient
		// with directional overhead specular catchlight and crisp pure-white icon
		// (avoids muddy glass-on-glass outlines inside composer capsules).
		weight = 600
		fg = ui.RGB(255, 255, 255)
		topCol := accent.Mix(ui.RGB(255, 255, 255), 0.14)
		botCol := accent.Mix(ui.RGB(0, 0, 0), 0.10)
		if btn.Hovered() {
			topCol = accent.Mix(ui.RGB(255, 255, 255), 0.22)
			botCol = accent.Mix(ui.RGB(0, 0, 0), 0.04)
		} else if btn.Pressed() {
			topCol = accent.Mix(ui.RGB(0, 0, 0), 0.08)
			botCol = accent.Mix(ui.RGB(0, 0, 0), 0.18)
		}
		btn.LinearGradient(ui.LinearGradient{From: topCol, To: botCol, Angle: 180})
		btn.Shadow(0, 4, 14, -4, kit.WithAlpha(accent, 0.52))
		attachTopSpecularSheen(btn, rad, 0.34)

	case BtnSolidIdle:
		// Refined borderless filled surface well for idle CTAs inside an outer
		// Liquid Glass pill (zero double-stroke wireframe clutter).
		weight = 500
		if spec.Tone != (ui.Color{}) {
			fg = spec.Tone
		} else {
			fg = th.InkMuted
		}
		if th.IsLight {
			bg := kit.WithAlpha(ui.RGB(0, 0, 0), 0.08)
			if btn.Hovered() {
				bg = kit.WithAlpha(ui.RGB(0, 0, 0), 0.13)
			} else if btn.Pressed() {
				bg = kit.WithAlpha(ui.RGB(0, 0, 0), 0.17)
			}
			btn.Background(bg)
		} else {
			bg := kit.WithAlpha(ui.RGB(255, 255, 255), 0.11)
			if btn.Hovered() {
				bg = kit.WithAlpha(ui.RGB(255, 255, 255), 0.17)
			} else if btn.Pressed() {
				bg = kit.WithAlpha(ui.RGB(255, 255, 255), 0.22)
			}
			btn.Background(bg)
		}

	case BtnGlass:
		// Authentic Apple 26 .glass lens for floating standalone navigation &
		// detached composer controls. Pure GPU shader rim — zero flat border.
		weight = 500
		if spec.Tone != (ui.Color{}) {
			fg = spec.Tone
		}
		if c.Vibrancy() {
			tint := kit.WithAlpha(th.SurfaceElevated, 0.24)
			if th.IsLight {
				tint = kit.WithAlpha(ui.RGB(255, 255, 255), 0.48)
			}
			if btn.Hovered() {
				if th.IsLight {
					tint = kit.WithAlpha(ui.RGB(255, 255, 255), 0.66)
				} else {
					tint = kit.WithAlpha(th.SurfaceHover, 0.40)
				}
			}
			btn.Material(glass.Glass{
				Style:       glass.Regular,
				Tint:        tint,
				Interactive: true,
			})
			btn.Shadow(0, 4, 14, -5, kit.WithAlpha(ui.RGB(0, 0, 0), 0.26))
		} else {
			applyGeistTactileSurface(btn, th, rad)
		}

	case BtnTinted:
		weight = 600
		fg = accent
		alpha := float32(0.18)
		if btn.Hovered() {
			alpha = 0.26
		}
		if btn.Pressed() {
			alpha = 0.14
		}
		if c.Vibrancy() {
			btn.Material(glass.Glass{
				Style:       glass.Regular,
				Tint:        kit.WithAlpha(accent, alpha+0.06),
				Interactive: true,
			})
			if !th.IsLight {
				fg = accent.Mix(ui.RGB(255, 255, 255), 0.30)
			}
		} else {
			btn.Background(kit.WithAlpha(accent, alpha)).
				Border(1, kit.WithAlpha(accent, 0.30))
			attachTopSpecularSheen(btn, rad, 0.14)
		}

	case BtnDestructive:
		danger := spec.Tone
		if danger == (ui.Color{}) {
			danger = th.Danger
		}
		weight = 600
		fg = danger
		alpha := float32(0.16)
		if btn.Hovered() {
			alpha = 0.24
		}
		if c.Vibrancy() {
			btn.Material(glass.Glass{
				Style:       glass.Regular,
				Tint:        kit.WithAlpha(danger, alpha+0.06),
				Interactive: true,
			})
			if !th.IsLight {
				fg = danger.Mix(ui.RGB(255, 255, 255), 0.24)
			}
		} else {
			btn.Background(kit.WithAlpha(danger, alpha)).
				Border(1, kit.WithAlpha(danger, 0.34))
			attachTopSpecularSheen(btn, rad, 0.12)
		}

	case BtnGhost:
		weight = 500
		if spec.Tone != (ui.Color{}) {
			fg = spec.Tone
		} else {
			fg = th.InkMuted
		}
		if btn.Hovered() {
			fg = th.Ink
			if th.IsLight {
				btn.Background(kit.WithAlpha(ui.RGB(0, 0, 0), 0.06))
			} else {
				btn.Background(kit.WithAlpha(ui.RGB(255, 255, 255), 0.09))
			}
		} else if btn.Pressed() {
			fg = th.Ink
			if th.IsLight {
				btn.Background(kit.WithAlpha(ui.RGB(0, 0, 0), 0.10))
			} else {
				btn.Background(kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
			}
		}

	default: // BtnGeistSecondary
		weight = 500
		if spec.Tone != (ui.Color{}) {
			fg = spec.Tone
		}
		applyGeistTactileSurface(btn, th, rad)
	}

	return fg, weight
}

func applyGeistTactileSurface(btn ui.Element, th Theme, rad float32) {
	top := th.SurfaceElevated.Mix(ui.RGB(255, 255, 255), 0.05)
	bot := th.SurfaceElevated
	if btn.Hovered() {
		top = th.SurfaceHover.Mix(ui.RGB(255, 255, 255), 0.06)
		bot = th.SurfaceHover
	} else if btn.Pressed() {
		top = th.Bar
		bot = th.SurfaceElevated
	}
	rimCol := kit.WithAlpha(ui.RGB(255, 255, 255), 0.13)
	sheenAlpha := float32(0.14)
	if th.IsLight {
		top = ui.RGB(255, 255, 255)
		bot = th.SurfaceElevated
		if btn.Hovered() {
			bot = th.SurfaceHover
		}
		rimCol = kit.WithAlpha(ui.RGB(0, 0, 0), 0.12)
		sheenAlpha = 0.65
	}
	btn.LinearGradient(ui.LinearGradient{From: top, To: bot, Angle: 180}).
		Border(1, rimCol).
		Shadow(0, 1, 3, 0, kit.WithAlpha(ui.RGB(0, 0, 0), 0.20))
	attachTopSpecularSheen(btn, rad, sheenAlpha)
}

// attachTopSpecularSheen draws a crisp 1px directional top inner highlight
// mimicking Apple & Vercel Geist overhead key-light beveling on rectangular/pill
// controls (skips pure circles so circular CTAs stay optically flawless).
func attachTopSpecularSheen(btn ui.Element, rad, alpha float32) {
	btn.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 6 || r.H <= 4 {
			return
		}
		// Skip pure circular buttons where a horizontal bar conflicts with curvature.
		if r.W <= r.H+4 && rad >= r.H*0.42 {
			return
		}
		inset := min(rad*0.55, r.W*0.25)
		if inset < 4 {
			inset = 4
		}
		w := r.W - 2*inset
		if w <= 2 {
			return
		}
		p.Fill(
			ui.Rect{X: r.X + inset, Y: r.Y + 1, W: w, H: 1},
			kit.WithAlpha(ui.RGB(255, 255, 255), alpha),
			0.5,
		)
	})
}
