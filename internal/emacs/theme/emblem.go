// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"github.com/tao3k/marlin-mygo/runtime/kit"
)

//go:embed assets/marlin-emblem.png
var marlinEmblemPNG []byte

// Sleek horizontal leaping blue marlin (swordfish) vector silhouette with
// arched sailfish dorsal fin, needle bill, pectoral fin, and forked tail.
// Rendered in the center of the Doom Modeline as the hidden ChatBox dock and
// rasterized into the macOS Menu Bar Tray template icon.
const marlinSilhouetteSVGSrc = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 20" width="64" height="20">
  <path fill="currentColor" d="M 63.0 10.3 L 47.6 8.8 C 44.2 6.8 39.2 5.4 33.4 4.2 C 29.8 2.0 25.0 1.2 20.6 1.6 C 23.4 4.2 21.8 6.8 16.4 8.2 L 10.8 9.2 L 2.0 3.0 L 6.8 10.3 L 2.0 17.6 L 10.8 11.4 L 16.8 12.1 C 22.8 13.2 29.6 13.7 36.2 13.0 L 32.0 17.2 C 37.2 16.4 41.2 14.4 44.0 12.1 L 47.6 11.6 L 63.0 10.3 Z"/>
</svg>`

// Kage GPU shader (compiled natively to Metal / WGSL / GLSL) that paints a
// serene, slow-breathing liquid-glass oceanic caustic sweep and soft Fresnel
// rim shimmer over the sculpted Marlin emblem (~5.7s calm swell cycle).
const marlinCausticShaderSrc = `package main

var Time float
var Accent vec4
var Info vec4

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / max(size, vec2(1, 1))

	phase := Time * 0.56
	breath := 0.5 + 0.5*sin(phase)

	// Diagonal liquid-glass specular caustic ribbon gliding every ~5.7s with a
	// calm resting pause between sweeps (organic Apple liquid rhythm, never frantic).
	cycle := fract(Time * 0.175)
	sweepPos := cycle*2.6 - 0.8
	diag := uv.x*0.72 + uv.y*0.68
	ripple := 0.032*sin(uv.y*7.5 - phase*1.4) + 0.022*cos(uv.x*6.2 + phase*0.9)
	band := exp(-pow((diag + ripple - sweepPos)/0.145, 2.0))
	fade := smoothstep(0.0, 0.14, cycle) * (1.0 - smoothstep(0.58, 0.78, cycle))
	caustic := band * fade * 0.30

	// Soft top-left liquid lens crescent + gentle pulsing Fresnel rim highlight.
	edgeDist := min(min(uv.x, 1.0-uv.x), min(uv.y, 1.0-uv.y))
	rim := (1.0 - smoothstep(0.0, 0.085, edgeDist)) * (0.14 + 0.12*breath)
	topArc := smoothstep(0.58, 0.0, length(uv - vec2(0.28, 0.16))) * (0.09 + 0.06*breath)

	col := mix(Accent.rgb, Info.rgb, 0.55)
	col = mix(col, vec3(1.0), 0.52)
	alpha := clamp(caustic + rim + topArc, 0.0, 0.52)
	return vec4(col*alpha, alpha)
}
`

var (
	marlinImgOnce       sync.Once
	marlinImgSrc        ui.ImageSource
	marlinShaderOnce    sync.Once
	marlinCaustic       *shader.Shader
	marlinSilSVGOnce    sync.Once
	marlinSilSVG        *ui.SVG
	marlinAppIconOnce   sync.Once
	marlinAppIconBytes  []byte
	marlinTrayIconOnce  sync.Once
	marlinTrayIconBytes []byte
	marlinEpoch         = time.Now()
)

func marlinSilhouetteSVG() *ui.SVG {
	marlinSilSVGOnce.Do(func() {
		if s, err := ui.ParseSVG([]byte(marlinSilhouetteSVGSrc)); err == nil {
			marlinSilSVG = s
		}
	})
	return marlinSilSVG
}

// MarlinSilhouetteIcon paints Marlin's sleek horizontal swordfish silhouette
// in the given dimensions and color (used as the center Doom Modeline ChatBox
// dock icon).
func MarlinSilhouetteIcon(c *ui.Context, w, h float32, col ui.Color) ui.Element {
	svg := marlinSilhouetteSVG()
	if svg == nil {
		return ui.Text(c, "✦").Font(FontMono).FontSize(12).TextColor(col).NoWrap()
	}
	return ui.Icon(c, svg).Width(w).Height(h).TextColor(col).Shrink(0)
}

// MarlinTrayIconPNG returns a crisp 44x44 Retina monochrome template PNG of
// the horizontal Marlin silhouette for the macOS Menu Bar Tray.
func MarlinTrayIconPNG() []byte {
	marlinTrayIconOnce.Do(func() {
		svg := marlinSilhouetteSVG()
		if svg == nil {
			return
		}
		// Render wide 40x14 marlin centered inside a 44x44 Retina tray square.
		glyph := svg.Image(40, 14, color.Black)
		canvas := image.NewNRGBA(image.Rect(0, 0, 44, 44))
		offX, offY := 2, 15
		for y := 0; y < 14; y++ {
			for x := 0; x < 40; x++ {
				r, g, b, a := glyph.At(x, y).RGBA()
				if a > 0 {
					canvas.SetNRGBA(offX+x, offY+y, color.NRGBA{
						R: uint8(r >> 8),
						G: uint8(g >> 8),
						B: uint8(b >> 8),
						A: uint8(a >> 8),
					})
				}
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, canvas); err == nil {
			marlinTrayIconBytes = buf.Bytes()
		}
	})
	return marlinTrayIconBytes
}

// MarlinAppIconPNG returns the Dashboard Marlin emblem sculpted into an Apple
// macOS HIG continuous-corner squircle PNG (with transparent outer margin and
// anti-aliased edge) ready for mygo.App.Dock.SetIcon and win.SetIcon.
func MarlinAppIconPNG() []byte {
	marlinAppIconOnce.Do(func() {
		src, err := png.Decode(bytes.NewReader(marlinEmblemPNG))
		if err != nil {
			marlinAppIconBytes = marlinEmblemPNG
			return
		}
		b := src.Bounds()
		sw, sh := b.Dx(), b.Dy()
		const outSize = 256
		const pad = 22.0 // ~8.6% macOS Dock HIG outer transparent margin
		half := (float64(outSize) - 2*pad) / 2.0
		cx := float64(outSize) / 2.0
		cy := float64(outSize) / 2.0
		const exp = 4.6 // Apple continuous squircle superellipse exponent

		dst := image.NewNRGBA(image.Rect(0, 0, outSize, outSize))
		for y := 0; y < outSize; y++ {
			for x := 0; x < outSize; x++ {
				nx := math.Abs(float64(x)+0.5-cx) / half
				ny := math.Abs(float64(y)+0.5-cy) / half
				d := math.Pow(math.Pow(nx, exp)+math.Pow(ny, exp), 1.0/exp)
				if d > 1.015 {
					continue
				}
				// Smooth 1.5px anti-aliased squircle edge.
				alpha := 1.0
				if d > 0.985 {
					alpha = (1.015 - d) / 0.030
				}
				if alpha <= 0 {
					continue
				}
				// Map inner squircle coordinates back onto source 256x256 emblem.
				u := (float64(x) + 0.5 - pad) / (float64(outSize) - 2*pad)
				v := (float64(y) + 0.5 - pad) / (float64(outSize) - 2*pad)
				sx := b.Min.X + int(min(max(u*float64(sw), 0), float64(sw-1)))
				sy := b.Min.Y + int(min(max(v*float64(sh), 0), float64(sh-1)))
				r, g, bl, _ := src.At(sx, sy).RGBA()
				rr, gg, bb := float64(r>>8), float64(g>>8), float64(bl>>8)
				// Subtle top-lit specular glass rim along the outer squircle bevel.
				if d > 0.945 && d <= 0.992 {
					rim := 0.18
					if v < 0.45 {
						rim = 0.28
					}
					rr = rr*(1-rim) + 255*rim
					gg = gg*(1-rim) + 255*rim
					bb = bb*(1-rim) + 255*rim
				}
				dst.SetNRGBA(x, y, color.NRGBA{
					R: uint8(rr),
					G: uint8(gg),
					B: uint8(bb),
					A: uint8(alpha * 255),
				})
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err == nil {
			marlinAppIconBytes = buf.Bytes()
		} else {
			marlinAppIconBytes = marlinEmblemPNG
		}
	})
	return marlinAppIconBytes
}

func marlinEmblemSource() ui.ImageSource {
	marlinImgOnce.Do(func() {
		if src, err := ui.DecodeImage(marlinEmblemPNG); err == nil {
			marlinImgSrc = src
		}
	})
	return marlinImgSrc
}

func marlinCausticShader() *shader.Shader {
	marlinShaderOnce.Do(func() {
		if s, err := shader.Compile(marlinCausticShaderSrc); err == nil {
			marlinCaustic = s
		}
	})
	return marlinCaustic
}

// MarlinEmblem paints Marlin's signature sculpted liquid-glass blue marlin
// (swordfish) brand mark inside an Apple continuous-corner squircle medallion
// with a top-lit specular glass rim, ambient ocean-cyan halo, and (on hero
// sizes >= 48px) a serene GPU Metal liquid-glass caustic breathing rhythm.
func MarlinEmblem(c *ui.Context, key string, size float32, th Theme) ui.Element {
	if size <= 0 {
		size = 64
	}
	rad := size * 0.27
	animate := size >= 48

	// Gentle ambient glow intensity synced to the ~5.7s oceanic breathing phase.
	glowAlpha := float32(0.42)
	if animate {
		elapsed := time.Since(marlinEpoch).Seconds()
		glowAlpha = 0.34 + 0.16*float32(0.5+0.5*math.Sin(elapsed*1.10))
	}

	frame := ui.Box(c.Key(key)).Width(size).Height(size).Shrink(0).Radius(rad).Clip().
		Background(ui.RGB(24, 33, 47)).
		Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.24)).
		Shadow(0, size*0.14, size*0.38, -4, kit.WithAlpha(th.Accent, glowAlpha))

	src := marlinEmblemSource()
	sh := marlinCausticShader()
	frame.Children(func() {
		if src != nil {
			ui.Image(c, src).Width(size).Height(size)
		}
		if animate && sh != nil {
			// Partial-damage GPU liquid caustic overlay: repaints only this
			// 80x80 squircle at a calm 24fps cadence without rebuilding layout.
			ui.Box(c.Key(key+"-caustic")).Absolute().Top(0).Left(0).Width(size).Height(size).
				PassThrough().Draw(func(p *ui.Painter, box ui.Rect) {
				if !p.Accelerated() || p.Preferences().ReduceMotion {
					return
				}
				p.AnimationFrameRate(24)
				t := float32(time.Since(marlinEpoch).Seconds())
				sh.Draw(p, box, shader.Options{
					Radius: rad,
					Uniforms: map[string]any{
						"Time":   t,
						"Accent": th.Accent,
						"Info":   th.Info,
					},
				})
			})
		}
	})
	return frame
}
