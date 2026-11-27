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

//go:embed assets/marlin-sprite-sheet.png
var marlinSpriteSheetPNG []byte

// MarlinOceanDockWidth, MarlinOceanDockHeight, and MarlinOceanOverhang define
// the transparent overlay canvas straddling the ModeLine's top border line:
//   - y = 0..110:   open viewport space strictly ABOVE the ModeLine Top Line
//     where both the crystal-clear transparent wave ribbon and the sleek
//     horizontal Blue Marlin glide forward.
//   - y = 110..148: transparent area BELOW the ModeLine Top Line holding ONLY
//     delicate clear glass water droplets (zero opaque blue background blob).
const (
	MarlinOceanDockWidth  float32 = 340
	MarlinOceanDockHeight float32 = 148
	MarlinOceanOverhang   float32 = 10
)

const (
	marlinSheetCellW float32 = 360
	marlinSheetCellH float32 = 240
)

// Kage GPU fragment shader (compiled natively to Metal / WGSL / GLSL) that
// paints a crystal-clear, glass-like transparent ocean wave ribbon lying flat
// ("perched") strictly ON TOP OF the ModeLine Top Line (uv.y <= lineY):
//   - Zero opaque blue background or underside bubble below lineY.
//   - Ultra-transparent glass-like interior tinted gently by the active theme
//     so dark and light themes stay cohesive and airy.
//   - Crisp luminous white-silver liquid specular crest tracing natural rolling
//     horizontal waves right along the ModeLine Top Line.
const marlinClearWaveShaderSrc = `package main

var Time float
var Motion float
var Surfaced float
var Pass float
var Info vec4
var Ink vec4

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / max(size, vec2(1, 1))

	motionAmt := clamp(Motion, 0.0, 1.0)
	surfAmt := clamp(Surfaced, 0.0, 1.0)

	// ModeLine top border line sits at y = 110 / 148 = 0.7432.
	// Water wave rests STRICTLY on top of lineY (never creates a blue blob below!).
	const lineY = 0.7432
	if uv.y > lineY + 0.012 {
		return vec4(0.0, 0.0, 0.0, 0.0)
	}

	// Graceful wide horizontal span tapering smoothly to zero at both ends.
	nx := (uv.x - 0.5) * 2.0
	env := exp(-pow(nx / 0.66, 2.0))
	core := exp(-pow(nx / 0.50, 2.0))

	// Rolling multi-crest horizontal wave lying flat along the ModeLine Top Line.
	speed := 1.8 + 4.2*motionAmt
	w1 := (0.5 + 0.5*sin(uv.x*20.0 - Time*speed)) * (0.042 + 0.032*motionAmt)
	w2 := (0.5 + 0.5*sin(uv.x*34.0 - Time*(speed*1.35) + 1.4)) * (0.022 + 0.018*motionAmt)
	baseH := (0.048 - 0.010*surfAmt + w1 + w2) * env
	waveTopY := lineY - baseH

	// Soft anti-aliased wave silhouette between waveTopY and lineY.
	belowTop := smoothstep(waveTopY - 0.010, waveTopY + 0.008, uv.y)
	aboveLine := 1.0 - smoothstep(lineY - 0.004, lineY + 0.008, uv.y)
	waveMask := belowTop * aboveLine * smoothstep(0.03, 0.20, env)

	// Crystal-clear transparent glass tint (adapts to theme; zero opaque blue slab!).
	heightFrac := clamp((lineY - uv.y)/max(baseH, 0.02), 0.0, 1.0)
	glassTint := mix(Info.rgb, Ink.rgb, 0.35)
	specCol := mix(Info.rgb, vec3(1.0, 1.0, 1.0), 0.84)

	// Delicate internal glass caustic shimmer.
	shimmer := pow(0.5 + 0.5*sin(uv.x*28.0 - Time*(speed*0.9) + uv.y*22.0), 4.0) * (0.14 + 0.20*motionAmt)

	// Crisp glassy white-tinted meniscus highlight along the rolling wave crest.
	crest := exp(-pow((uv.y - waveTopY)/(0.010 + 0.006*motionAmt), 2.0)) * env * (0.42 + 0.34*motionAmt)
	// Subtle specular hairline where the transparent wave kisses the ModeLine Top Line.
	baseRim := exp(-pow((uv.y - lineY)/0.006, 2.0)) * core * 0.24

	if Pass < 0.5 {
		// Pass 0: Ultra-subtle transparent glass wave depth behind the fish.
		fillAlpha := waveMask * (0.06 + 0.06*heightFrac + shimmer*0.18)
		col := glassTint*fillAlpha + specCol*(crest*0.32 + baseRim*0.30)
		return vec4(col, clamp(fillAlpha + crest*0.26 + baseRim*0.24, 0.0, 0.42))
	}

	// Pass 1: Foreground clear glass refraction veil + crisp rolling wave highlight
	// over the submerged lower rim of the Marlin.
	veilAlpha := waveMask * mix(0.14, 0.08, surfAmt) * (0.55 + 0.45*(1.0 - heightFrac))
	// Second fine internal wave filament for true multi-layer liquid glass depth.
	subWaveY := mix(waveTopY, lineY, 0.48) + sin(uv.x*26.0 + Time*(speed*0.7))*0.006*env
	subCrest := exp(-pow((uv.y - subWaveY)/0.008, 2.0)) * waveMask * (0.20 + 0.22*motionAmt)

	outCol := glassTint*veilAlpha + specCol*(crest*0.86 + subCrest + baseRim*0.65 + shimmer*waveMask*0.42)
	outAlpha := clamp(veilAlpha + crest*0.76 + subCrest*0.85 + baseRim*0.52 + shimmer*waveMask*0.30, 0.0, 0.78)
	return vec4(outCol, outAlpha)
}
`

var (
	marlinWaterShaderOnce sync.Once
	marlinWaterShader     *shader.Shader
	marlinSheetOnce       sync.Once
	marlinSheetBitmap     *ui.Bitmap
)

func getMarlinWaterShader() *shader.Shader {
	marlinWaterShaderOnce.Do(func() {
		if s, err := shader.Compile(marlinClearWaveShaderSrc); err == nil {
			marlinWaterShader = s
		}
	})
	return marlinWaterShader
}

func marlinSpriteSheetBitmap() *ui.Bitmap {
	marlinSheetOnce.Do(func() {
		if bm, err := ui.DecodeBitmap(marlinSpriteSheetPNG); err == nil {
			marlinSheetBitmap = bm
		}
	})
	return marlinSheetBitmap
}

// MarlinSpriteSheetPNG returns the embedded 720x480 4-frame spine-aligned
// horizontal Blue Marlin animation atlas PNG.
func MarlinSpriteSheetPNG() []byte {
	return marlinSpriteSheetPNG
}

// MarlinSurfaceAnimDuration is the ~1.40s cinematic movie-like rising onto
// plane and gradual dorsal sail unfurl window before settling cleanly static.
const MarlinSurfaceAnimDuration = 1400 * time.Millisecond

// MarlinClickDashDuration is the ~1.15s horizontal speedboat forward surge
// and smooth underwater dive window played when clicking the Marlin pendant.
const MarlinClickDashDuration = 1150 * time.Millisecond

// MarlinOceanDockProps controls the 4-frame cinematic horizontal Marlin gliding
// strictly on top of the ModeLine Top Line with a transparent glass wave ribbon
// and clear glass water droplets hanging below the Top Line.
type MarlinOceanDockProps struct {
	Key        string
	Hovered    bool
	Open       bool
	HoverStart time.Time
	ClickStart time.Time
	Theme      Theme
}

// smootherstep provides C2-continuous filmic easing (zero acceleration snaps)
// so dorsal fin unfurling and speedboat forward planing feel cinematic.
func smootherstep(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	return x * x * x * (x*(x*6-15) + 10)
}

// MarlinOceanDock paints the crystal-clear transparent wave ribbon perched on
// top of the ModeLine Top Line, the 4-frame cinematic horizontal Blue Marlin,
// unhurried clear glass wake droplets, and transparent droplets hanging below
// the ModeLine Top Line.
func MarlinOceanDock(c *ui.Context, p MarlinOceanDockProps) ui.Element {
	th := p.Theme
	// When the formal ChatBox is open, the Marlin dives cleanly underwater so
	// it never crowds or squeezes the ChatBox upward.
	surfacedTarget := p.Hovered && !p.Open

	return ui.Box(c.Key(p.Key)).
		Width(MarlinOceanDockWidth).
		Height(MarlinOceanDockHeight).
		Shrink(0).
		Clip().
		PassThrough().
		Draw(func(painter *ui.Painter, box ui.Rect) {
			now := time.Now()
			reduceMotion := painter.Preferences().ReduceMotion

			// 1. Compute ~1.40s filmic speedboat rise-onto-plane & gradual sail unfurl.
			var hoverU float64
			var emerge float64
			var hoverMotion float64
			var hoverAnimating bool
			if surfacedTarget {
				if reduceMotion || p.HoverStart.IsZero() {
					hoverU = 1.0
					emerge = 1.0
				} else {
					elapsed := now.Sub(p.HoverStart)
					if elapsed < 0 {
						elapsed = 0
					}
					if elapsed < MarlinSurfaceAnimDuration {
						hoverAnimating = true
						hoverU = elapsed.Seconds() / MarlinSurfaceAnimDuration.Seconds()
						// Gentle filmic rise from 38% cruising to 88% speedboat planing over 0..0.72.
						emerge = smootherstep(hoverU / 0.72)
						// Smooth bell-shaped wave & wake propulsion envelope fading gently to 0 at 1.40s.
						hoverMotion = math.Sin(math.Min(hoverU/0.25, 1.0)*math.Pi*0.5) * math.Pow(1.0-hoverU, 1.15)
					} else {
						hoverU = 1.0
						emerge = 1.0
						hoverMotion = 0.0
					}
				}
			}

			// 2. Compute ~1.15s horizontal speedboat forward surge -> smooth dive underwater.
			var clickU float64
			var clickMotion float64
			var clickAnimating bool
			if !p.ClickStart.IsZero() && !reduceMotion {
				elapsed := now.Sub(p.ClickStart)
				if elapsed >= 0 && elapsed < MarlinClickDashDuration {
					clickAnimating = true
					clickU = elapsed.Seconds() / MarlinClickDashDuration.Seconds()
					clickMotion = math.Sin(math.Min(clickU/0.18, 1.0)*math.Pi*0.5) * math.Pow(1.0-clickU, 1.10)
				}
			}

			if hoverAnimating || clickAnimating {
				painter.AnimationFrameRate(60)
			}

			t := float32(now.Sub(marlinEpoch).Seconds())
			motionAmt := float32(math.Max(hoverMotion, clickMotion))
			emergeF := float32(emerge)
			// ModeLine Top Line sits ModelineHeight + MarlinOceanOverhang above box bottom.
			lineY := box.Y + box.H - ModelineHeight - MarlinOceanOverhang

			sh := getMarlinWaterShader()
			useShader := sh != nil && painter.Accelerated()

			// Layer 1: Back crystal-clear transparent wave ribbon strictly ON TOP OF lineY.
			if useShader {
				sh.Draw(painter, box, shader.Options{
					Uniforms: map[string]any{
						"Time":     t,
						"Motion":   motionAmt,
						"Surfaced": emergeF,
						"Pass":     float32(0),
						"Info":     th.Info,
						"Ink":      th.Ink,
					},
				})
			} else {
				drawFallbackClearWave(painter, box, lineY, t, emergeF, motionAmt, true, th)
			}

			// Layer 2: Spine-aligned 4-frame horizontal Blue Marlin clipped strictly AT
			// lineY so the fish rides cleanly ON TOP OF the ModeLine Top Line.
			clipRect := ui.Rect{X: box.X, Y: box.Y, W: box.W, H: lineY - box.Y}
			painter.Clip(clipRect, 0, func() {
				drawCinematicMarlinGlide(painter, box, lineY, t, hoverU, hoverAnimating, clickU, clickAnimating, surfacedTarget, p.Open)
			})

			// Layer 3: Foreground crystal-clear glass wave meniscus over the Marlin's lower waterline.
			if useShader {
				sh.Draw(painter, box, shader.Options{
					Uniforms: map[string]any{
						"Time":     t,
						"Motion":   motionAmt,
						"Surfaced": emergeF,
						"Pass":     float32(1),
						"Info":     th.Info,
						"Ink":      th.Ink,
					},
				})
			} else {
				drawFallbackClearWave(painter, box, lineY, t, emergeF, motionAmt, false, th)
			}

			// Layer 4: Pure crystal-clear transparent glass water droplets hanging
			// cleanly BELOW the ModeLine Top Line (zero blue background behind them).
			drawTransparentBelowLineDroplets(painter, box, lineY, motionAmt, float32(clickU), clickAnimating, th)

			// Layer 5: Unhurried crystal-clear speedboat wake spray & surface ripples.
			if !reduceMotion {
				if clickAnimating {
					drawClearSpeedboatWakeDrops(painter, box, lineY, float32(clickU), true, th)
				} else if hoverAnimating {
					drawClearSpeedboatWakeDrops(painter, box, lineY, float32(hoverU), false, th)
				}
			}
		})
}

func marlinFrameRect(idx int) ui.Rect {
	col := float32(idx % 2)
	row := float32(idx / 2)
	return ui.Rect{
		X: col * marlinSheetCellW,
		Y: row * marlinSheetCellH,
		W: marlinSheetCellW,
		H: marlinSheetCellH,
	}
}

func drawSpriteFrame(
	painter *ui.Painter,
	bm *ui.Bitmap,
	frameIdx int,
	dst ui.Rect,
	pitchDeg float32,
	alpha float32,
) {
	if bm == nil || alpha <= 0.01 {
		return
	}
	var tint ui.Color
	if alpha < 0.995 {
		tint = ui.RGBA(255, 255, 255, min(max(alpha, 0), 1))
	}
	pivotX := dst.X + dst.W*0.52
	pivotY := dst.Y + dst.H*0.72
	if math.Abs(float64(pitchDeg)) > 0.04 {
		painter.Transform(ui.Rotation(pitchDeg), pivotX, pivotY, func() {
			painter.ImageRegion(bm, marlinFrameRect(frameIdx), dst, ui.ImageOptions{Tint: tint})
		})
		return
	}
	painter.ImageRegion(bm, marlinFrameRect(frameIdx), dst, ui.ImageOptions{Tint: tint})
}

// drawCinematicMarlinGlide orchestrates movie-smooth horizontal speedboat
// planing across the 4 spine-aligned frames:
//   - Frame 0: cruising calmly, dorsal fin folded flat (~38% visible above Top Line)
//   - Frame 1: rising smoothly onto plane, dorsal fin ~45% lifted
//   - Frame 2: planing high on top of the clear wave (~88% visible, sleek sail open)
//   - Frame 3: horizontal forward speedboat propulsion stroke (paired smoothly with Frame 2)
func drawCinematicMarlinGlide(
	painter *ui.Painter,
	box ui.Rect,
	lineY, t float32,
	hoverU float64,
	hoverAnimating bool,
	clickU float64,
	clickAnimating bool,
	surfacedTarget bool,
	chatBoxOpen bool,
) {
	bm := marlinSpriteSheetBitmap()
	if bm == nil {
		return
	}

	// All 4 cells share exact 3:2 aspect ratio and identical bottom-aligned
	// horizontal belly baseline (10px bottom margin at 224x149), so cross-fading
	// between frames keeps the fish body rock-steady while ONLY the dorsal fin,
	// pectoral fin, mouth, and propulsion stroke animate smoothly like a movie.
	const spriteW float32 = 224
	const spriteH float32 = 149
	cx := box.X + box.W*0.5

	rectAt := func(dx, dyFromLine float32) ui.Rect {
		return ui.Rect{
			X: cx - spriteW*0.5 + dx,
			Y: lineY - spriteH + dyFromLine,
			W: spriteW,
			H: spriteH,
		}
	}

	// Vertical offsets relative to lineY (remember: 10px cell bottom pad):
	//   - restDY  = +36: upper ~38-42% (sapphire back, folded fin, eye, bill) above lineY
	//   - planeDY = +13: ~88-90% of the full sleek horizontal fish skimming on top of lineY
	//   - divedDY = +56: tucked deep underwater when ChatBox is open (zero upward squeeze)
	const restDY float32 = 36
	const planeDY float32 = 13
	const divedDY float32 = 56

	// Click: smooth horizontal speedboat charge forward along lineY, then graceful dive.
	if clickAnimating {
		u := float64(clickU)
		if u < 0.56 {
			// Smooth S-curve forward acceleration like a motorboat surging across the wave.
			k := u / 0.56
			surge := float32(math.Sin(smootherstep(k) * math.Pi * 0.88))
			// Subtle rhythmic speedboat hull chug ("planing skim" along the wave).
			skimY := float32(math.Sin(k*math.Pi*3.0)) * 1.4 * (1.0 - float32(k)*0.3)
			dst := rectAt(surge*26.0, planeDY-surge*3.0+skimY)
			// Blend smoothly from Frame 2 into Frame 3 forward charge over first 30%.
			f3 := float32(smootherstep(k / 0.35))
			drawSpriteFrame(painter, bm, 2, dst, -1.5*surge, 1.0-f3)
			drawSpriteFrame(painter, bm, 3, dst, -1.5*surge, f3)
			return
		}
		// Smooth cinematic nose-down dive underwater when ChatBox opens (or calm cruise settle when closed).
		diveK := smootherstep((u - 0.56) / 0.44)
		startX := float32(math.Sin(0.88*math.Pi)) * 26.0
		endDY := restDY
		endAlpha := float32(1.0)
		divePitch := float32(0)
		if chatBoxOpen {
			endDY = divedDY
			endAlpha = 0.48
			divePitch = float32(math.Sin(diveK*math.Pi)) * 6.5
		}
		curX := startX * float32(1.0-diveK)
		curDY := planeDY + (endDY-planeDY)*float32(diveK)
		dst := rectAt(curX, curDY)
		drawSpriteFrame(painter, bm, 3, dst, divePitch, float32(1.0-diveK))
		drawSpriteFrame(painter, bm, 0, dst, divePitch, float32(diveK)*endAlpha)
		return
	}

	// ChatBox open: stay calmly tucked underwater out of the ChatBox's way.
	if chatBoxOpen {
		drawSpriteFrame(painter, bm, 0, rectAt(0, divedDY), 1.5, 0.46)
		return
	}

	// Rest (0fps): Frame 0 horizontal cruising with fins folded, ~38-42% above Top Line.
	if !surfacedTarget {
		drawSpriteFrame(painter, bm, 0, rectAt(0, restDY), 0, 1.0)
		return
	}

	// Hover settled after ~1.40s (0fps): calm static Frame 2 sleek horizontal glide!
	if !hoverAnimating || hoverU >= 1.0 {
		drawSpriteFrame(painter, bm, 2, rectAt(4, planeDY), -0.6, 1.0)
		return
	}

	// Movie-like 3-act hover sequence over ~1.40s (zero abrupt jumps!):
	//   - Continuous filmic lift from restDY -> planeDY + gentle forward speedboat glide (+8px -> +4px)
	//   - Act 1 (0.00..0.38): Frame 0 (folded) -> Frame 1 (dorsal smoothly half-lifted)
	//   - Act 2 (0.38..0.72): Frame 1 (half-lifted) -> Frame 2 (graceful speed-sail open)
	//   - Act 3 (0.72..1.00): Gentle horizontal yacht-like forward skim pulse (Frame 2 <-> subtle Frame 3) -> Frame 2
	rise := smootherstep(hoverU / 0.68)
	glideForward := float32(math.Sin(smootherstep(hoverU)*math.Pi*0.72))*6.0 + float32(rise)*4.0
	// Gentle horizontal speedboat hull ride across the rolling wave crests.
	hullSkim := float32(math.Sin(hoverU*math.Pi*4.0)) * 1.35 * float32(1.0-hoverU)
	curDY := restDY + (planeDY-restDY)*float32(rise) + hullSkim
	bowPitch := float32(-2.2*math.Sin(rise*math.Pi*0.65)) - float32(0.6*rise)
	dst := rectAt(glideForward, curDY)

	if hoverU < 0.38 {
		w := float32(smootherstep(hoverU / 0.38))
		drawSpriteFrame(painter, bm, 0, dst, bowPitch, 1.0-w)
		drawSpriteFrame(painter, bm, 1, dst, bowPitch, w)
		return
	}
	if hoverU < 0.72 {
		w := float32(smootherstep((hoverU - 0.38) / 0.34))
		drawSpriteFrame(painter, bm, 1, dst, bowPitch, 1.0-w)
		drawSpriteFrame(painter, bm, 2, dst, bowPitch, w)
		return
	}
	// Smooth gliding propulsion pulse (Frame 2 -> soft breath of Frame 3 -> Frame 2).
	p := (hoverU - 0.72) / 0.28
	pulse := float32(math.Sin(p*math.Pi)) * 0.55
	drawSpriteFrame(painter, bm, 2, dst, bowPitch, 1.0-pulse)
	drawSpriteFrame(painter, bm, 3, dst, bowPitch-0.5*pulse, pulse)
	_ = t
}

// drawTransparentBelowLineDroplets paints 4 pure crystal-clear glass water
// droplets suspended cleanly BELOW the ModeLine Top Line with zero blue blob
// behind them — transparent theme-tinted glass interior + crisp white specular
// highlight dot + luminous bottom caustic rim.
func drawTransparentBelowLineDroplets(
	painter *ui.Painter,
	box ui.Rect,
	lineY, motion, clickU float32,
	clickAnimating bool,
	th Theme,
) {
	type dropSpec struct {
		xFrac float32
		dropY float32
		rx    float32
		ry    float32
		drip  bool
	}
	drops := []dropSpec{
		{xFrac: 0.33, dropY: 11.5, rx: 2.9, ry: 4.1, drip: false},
		{xFrac: 0.45, dropY: 16.5, rx: 3.6, ry: 5.0, drip: false},
		{xFrac: 0.56, dropY: 22.0, rx: 4.0, ry: 5.6, drip: true},
		{xFrac: 0.68, dropY: 10.0, rx: 2.6, ry: 3.7, drip: false},
	}

	glassTint := th.Info.Mix(th.Ink, 0.30)
	rimCol := th.Info.Mix(ui.RGB(255, 255, 255), 0.78)
	white := ui.RGB(255, 255, 255)

	for _, d := range drops {
		cx := box.X + box.W*d.xFrac
		extraDrip := float32(0)
		if d.drip {
			if clickAnimating {
				extraDrip = float32(math.Sin(float64(min(clickU/0.85, 1.0))*math.Pi)) * 4.2
			} else if motion > 0.05 {
				extraDrip = motion * 2.0
			}
		}
		cy := lineY + d.dropY + extraDrip

		var tear ui.Path
		tear.MoveTo(cx, cy-d.ry*1.22)
		tear.CubeTo(cx+d.rx*0.95, cy-d.ry*0.35, cx+d.rx*1.12, cy+d.ry*0.55, cx, cy+d.ry)
		tear.CubeTo(cx-d.rx*1.12, cy+d.ry*0.55, cx-d.rx*0.95, cy-d.ry*0.35, cx, cy-d.ry*1.22)
		tear.Close()

		// Crystal-clear transparent glass body (low alpha so ModeLine shines straight through!).
		painter.FillPathGradient(&tear, ui.LinearGradient{
			From:  kit.WithAlpha(glassTint, 0.08),
			To:    kit.WithAlpha(rimCol, 0.24),
			Angle: 170,
		})
		painter.StrokePath(&tear, 0.85, kit.WithAlpha(rimCol, 0.56))

		// Crisp pure-white specular highlight dot on upper-left shoulder.
		var spec ui.Path
		spec.Circle(cx-d.rx*0.32, cy-d.ry*0.10, max(d.rx*0.26, 0.85))
		painter.FillPath(&spec, kit.WithAlpha(white, 0.88))

		// Soft luminous glass refraction caustic at the bottom belly of the teardrop.
		var belly ui.Path
		belly.Circle(cx+d.rx*0.10, cy+d.ry*0.44, max(d.rx*0.30, 0.9))
		painter.FillPath(&belly, kit.WithAlpha(rimCol, 0.44))
	}
}

// drawClearSpeedboatWakeDrops paints unhurried, rhythmic crystal-clear glass
// water spray teardrops trailing backward off the surging Marlin like a sleek
// yacht wake ("biu... biu... biu...") plus delicate glass surface ripples.
func drawClearSpeedboatWakeDrops(
	painter *ui.Painter,
	box ui.Rect,
	lineY, u float32,
	isClick bool,
	th Theme,
) {
	type wakeDrop struct {
		startU float32
		durU   float32
		oxFrac float32
		vx     float32 // negative = trailing backward behind the forward-gliding Marlin
		vy     float32
		grav   float32
		size   float32
	}

	// 3 deliberately paced unhurried wake beats trailing gracefully off the bow & stern.
	beats := []wakeDrop{
		{startU: 0.04, durU: 0.42, oxFrac: 0.36, vx: -48, vy: 46, grav: 112, size: 3.1},
		{startU: 0.08, durU: 0.40, oxFrac: 0.58, vx: -18, vy: 38, grav: 105, size: 2.6},
		{startU: 0.26, durU: 0.42, oxFrac: 0.34, vx: -58, vy: 52, grav: 118, size: 3.3},
		{startU: 0.30, durU: 0.40, oxFrac: 0.50, vx: -32, vy: 44, grav: 110, size: 2.7},
		{startU: 0.48, durU: 0.42, oxFrac: 0.38, vx: -44, vy: 40, grav: 106, size: 2.8},
	}
	if isClick {
		beats[0].vx, beats[0].vy = -68, 54
		beats[2].vx, beats[2].vy = -76, 60
	}

	rimCol := th.Info.Mix(ui.RGB(255, 255, 255), 0.78)
	white := ui.RGB(255, 255, 255)

	// Delicate trailing glassy wake ripples along the ModeLine Top Line.
	for i, rStart := range []float32{0.05, 0.27, 0.49} {
		dt := (u - rStart) / 0.46
		if dt <= 0 || dt >= 1 {
			continue
		}
		rx := 16.0 + dt*52.0
		ry := 2.4 + dt*3.6
		alpha := (1.0 - dt) * (1.0 - dt) * 0.42
		cx := box.X + box.W*(0.44-0.05*float32(i))
		var arc ui.Path
		arc.MoveTo(cx-rx, lineY-1)
		arc.QuadTo(cx, lineY-1-ry, cx+rx, lineY-1)
		painter.StrokePath(&arc, 1.1, kit.WithAlpha(rimCol, alpha))
	}

	// Velocity-aligned clear glass teardrops arcing back off the speedboat wake.
	for _, b := range beats {
		dt := (u - b.startU) / b.durU
		if dt <= 0 || dt >= 1 {
			continue
		}
		x := box.X + box.W*b.oxFrac + b.vx*dt
		y := lineY - 4 - b.vy*dt + 0.5*b.grav*dt*dt
		if y > lineY {
			continue
		}
		dx := b.vx
		dy := -b.vy + b.grav*dt
		angleDeg := float32(math.Atan2(float64(dy), float64(dx)) * 180.0 / math.Pi)

		alpha := float32(math.Sin(float64(dt)*math.Pi)) * 0.85
		sz := b.size * (0.85 + 0.25*float32(math.Sin(float64(dt)*math.Pi)))

		painter.Transform(ui.Rotation(angleDeg), x, y, func() {
			var tear ui.Path
			tear.MoveTo(x-sz*1.7, y)
			tear.QuadTo(x-sz*0.15, y-sz*0.80, x+sz*0.82, y)
			tear.QuadTo(x-sz*0.15, y+sz*0.80, x-sz*1.7, y)
			tear.Close()
			painter.FillPathGradient(&tear, ui.LinearGradient{
				From:  kit.WithAlpha(rimCol, alpha*0.24),
				To:    kit.WithAlpha(white, alpha*0.62),
				Angle: 0,
			})
			painter.StrokePath(&tear, 0.75, kit.WithAlpha(white, alpha*0.70))
			var glint ui.Path
			glint.Circle(x+sz*0.16, y-sz*0.14, max(sz*0.26, 0.75))
			painter.FillPath(&glint, kit.WithAlpha(white, alpha*0.92))
		})
	}
}

func drawFallbackClearWave(painter *ui.Painter, box ui.Rect, lineY, t, emerge, motion float32, back bool, th Theme) {
	var wave ui.Path
	leftX := box.X + box.W*0.10
	rightX := box.X + box.W*0.90
	wave.MoveTo(leftX, lineY)
	const steps = 28
	for i := 0; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		x := leftX + frac*(rightX-leftX)
		env := float32(math.Sin(float64(frac) * math.Pi))
		crest := float32(0.5+0.5*math.Sin(float64(frac)*18.0-float64(t)*3.0)) * (4.5 + 3.5*motion)
		y := lineY - (5.5-1.5*emerge+crest)*env
		wave.LineTo(x, y)
	}
	wave.LineTo(rightX, lineY)
	wave.Close()

	glassTint := th.Info.Mix(th.Ink, 0.35)
	specCol := th.Info.Mix(ui.RGB(255, 255, 255), 0.80)
	if back {
		painter.FillPath(&wave, kit.WithAlpha(glassTint, 0.10))
		return
	}
	painter.FillPath(&wave, kit.WithAlpha(glassTint, 0.12))
	painter.StrokePath(&wave, 1.15, kit.WithAlpha(specCol, 0.52+0.26*motion))
}
