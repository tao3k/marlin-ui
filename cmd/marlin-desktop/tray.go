// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"log"

	"github.com/egoist/mygo"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
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
		Icon:           theme.MarlinTrayIconPNG(),
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
