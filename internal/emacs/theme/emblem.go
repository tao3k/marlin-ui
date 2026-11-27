// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	_ "embed"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"github.com/tao3k/marlin-mygo/runtime/kit"
)

//go:embed assets/marlin-emblem.png
var marlinEmblemPNG []byte

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
	marlinImgOnce    sync.Once
	marlinImgSrc     ui.ImageSource
	marlinShaderOnce sync.Once
	marlinCaustic    *shader.Shader
	marlinEpoch      = time.Now()
)

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
