// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestAppleGeistButtonVariantsAllThemesAndNoPinkAccents(t *testing.T) {
	variants := []struct {
		name string
		v    theme.ButtonVariant
	}{
		{"prominent", theme.BtnGlassProminent},
		{"glass", theme.BtnGlass},
		{"solid-idle", theme.BtnSolidIdle},
		{"secondary", theme.BtnGeistSecondary},
		{"tinted", theme.BtnTinted},
		{"ghost", theme.BtnGhost},
		{"destructive", theme.BtnDestructive},
	}

	for _, th := range theme.Catalog() {
		// Regression guard: built-in themes must never use dusty pink / magenta UI accents.
		if th.Accent.R > 180 && th.Accent.B > 160 && th.Accent.G < 160 {
			t.Fatalf("theme %q accent %+v violates cool Doom One / Tokyo Night / Nord palette invariant", th.ID, th.Accent)
		}
		for _, vibrant := range []bool{true, false} {
			curTh := th
			clicked := make(map[string]bool)
			tt := ui.NewTester(func(c *ui.Context) {
				theme.ApplyToMyGo(c, curTh)
				ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
					for _, item := range variants {
						spec := theme.ButtonSpec{
							Key:     "btn-" + item.name,
							Label:   "Action " + item.name,
							Icon:    theme.IconSparkles,
							Variant: item.v,
							Pill:    item.v == theme.BtnGlassProminent || item.v == theme.BtnGlass,
						}
						if theme.StyledButton(c, spec, curTh).Clicked() {
							clicked[item.name] = true
						}
					}
					if theme.IconCircleButton(c, "btn-circle-send", "Send", theme.IconSend, 34, theme.BtnGlassProminent, curTh.Accent, false, curTh).Clicked() {
						clicked["circle"] = true
					}
				})
			}, 360, 480)
			tt.SetVibrancy(vibrant)
			tt.Frame()

			for _, item := range variants {
				lbl := "Action " + item.name
				if !tt.HasText(lbl) {
					t.Fatalf("theme=%s vibrant=%v missing button label %q", curTh.ID, vibrant, lbl)
				}
				if err := tt.Click(lbl); err != nil {
					t.Fatalf("theme=%s vibrant=%v click %q failed: %v", curTh.ID, vibrant, lbl, err)
				}
				if !clicked[item.name] {
					t.Fatalf("theme=%s vibrant=%v button %q click did not fire", curTh.ID, vibrant, lbl)
				}
			}
			_ = fmt.Sprintf("%s-%v", curTh.ID, vibrant)
		}
	}
}

// TestIconCircleButtonOpticalCentering ensures circular icon buttons (Send arrow,
// Drawer toggle, Mention '@') have zero stray padding and center their SF Symbol
// glyph optically inside the circular hull instead of drifting off-center.
func TestIconCircleButtonOpticalCentering(t *testing.T) {
	th := theme.DoomOne()
	const diam = 36
	tt := ui.NewTester(func(c *ui.Context) {
		theme.ApplyToMyGo(c, th)
		ui.Box(c).Fill().Background(th.Canvas).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			theme.IconCircleButton(c, "center-send", "Send", theme.IconSend, diam, theme.BtnGlassProminent, th.Accent, false, th)
		})
	}, 120, 120)
	tt.SetVibrancy(true)
	tt.Frame()

	rect, ok := tt.Find("Send")
	if !ok {
		t.Fatalf("expected circular Send button rect")
	}
	if rect.W < diam-1 || rect.W > diam+1 || rect.H < diam-1 || rect.H > diam+1 {
		t.Fatalf("expected %dx%d circle, got %+v", diam, diam, rect)
	}
	// Verify bright white glyph pixels inside the circle sit near its geometric center.
	img := tt.Image()
	if img == nil {
		return
	}
	cx := rect.X + rect.W/2
	cy := rect.Y + rect.H/2
	var sumX, sumY, count float64
	minX := int(rect.X + 4)
	maxX := int(rect.X + rect.W - 4)
	minY := int(rect.Y + 4)
	maxY := int(rect.Y + rect.H - 4)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			col := img.RGBAAt(x, y)
			// White arrow pixels have very high brightness across all RGB channels.
			if col.R > 210 && col.G > 210 && col.B > 210 {
				sumX += float64(x)
				sumY += float64(y)
				count++
			}
		}
	}
	if count > 0 {
		meanX := float32(sumX / count)
		meanY := float32(sumY / count)
		dx := meanX - cx
		dy := meanY - cy
		if dx < -3.0 || dx > 3.0 || dy < -3.0 || dy > 3.0 {
			t.Fatalf("SF Symbol arrow.up drifted off-center inside circle: center=(%.1f,%.1f) glyphMean=(%.1f,%.1f) delta=(%.1f,%.1f)", cx, cy, meanX, meanY, dx, dy)
		}
	}
}
