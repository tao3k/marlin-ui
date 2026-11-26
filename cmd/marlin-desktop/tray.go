// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"

	"github.com/egoist/mygo"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
)

func setupDesktopTray(
	fr *frame.Frame,
	focusWindow func(),
	invalidate func(),
) *mygo.Tray {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: "Show Marlin",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				focusWindow()
			},
		},
		mygo.Separator(),
		{
			Label: "Helm Search",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				fr.OpenHelm()
				focusWindow()
				invalidate()
			},
		},
		{
			Label: "Switch Buffer",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				fr.OpenBufferSwitch()
				focusWindow()
				invalidate()
			},
		},
		{
			Label: "*Settings*",
			Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				fr.OpenSettings()
				focusWindow()
				invalidate()
			},
		},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})

	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "dev.tao3k.marlin-ui.tray",
		Icon:           renderMarlinTrayPNG(),
		IconIsTemplate: true,
		ToolTip:        appconfig.BuildSummary("Marlin"),
		Menu:           menu,
	})
	if err != nil {
		log.Printf("desktop tray unavailable: %v", err)
		return nil
	}
	return tray
}

// renderMarlinTrayPNG synthesizes a crisp 32x32 Retina monochrome template
// icon (stylized "M" glyph) in pure Go with zero external asset files.
func renderMarlinTrayPNG() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ink := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	drawDot := func(cx, cy, r int) {
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				if x >= 0 && x < size && y >= 0 && y < size {
					dx, dy := x-cx, y-cy
					if dx*dx+dy*dy <= r*r {
						img.SetNRGBA(x, y, ink)
					}
				}
			}
		}
	}
	for y := 7; y <= 25; y++ {
		drawDot(7, y, 2)
		drawDot(24, y, 2)
	}
	for i := 0; i <= 8; i++ {
		drawDot(7+i, 8+i, 2)
		drawDot(24-i, 8+i, 2)
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
