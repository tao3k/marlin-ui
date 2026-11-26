// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"github.com/egoist/mygo"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
)

func buildDesktopMenu(fr *frame.Frame, invalidate func()) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: "Marlin",
			Submenu: []*mygo.MenuItem{
				{
					Label: "About Marlin",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						mygo.App.ShowAboutPanel(mygo.AboutPanelOptions{
							ApplicationName:    "Marlin",
							ApplicationVersion: appconfig.Version,
							Version:            appconfig.Commit,
							Copyright:          "Copyright (c) 2026 tao3k team and Contributors (Apache-2.0 AND LGPL-2.1-or-later)",
							Credits:            appconfig.BuildSummary("marlin-desktop"),
						})
					},
				},
				mygo.Separator(),
				{
					Label:       "*Settings*",
					Accelerator: "CmdOrCtrl+,",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						fr.OpenSettings()
						invalidate()
					},
				},
				mygo.Separator(),
				{
					Label:       "Helm Search (SPC s)",
					Accelerator: "CmdOrCtrl+P",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						fr.OpenHelm()
						invalidate()
					},
				},
				{
					Label:       "M-x Commands (SPC SPC)",
					Accelerator: "CmdOrCtrl+K",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						fr.OpenCommands()
						invalidate()
					},
				},
				{
					Label:       "Switch Buffer (SPC b b)",
					Accelerator: "CmdOrCtrl+B",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						fr.OpenBufferSwitch()
						invalidate()
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			},
		},
		{
			Label: "File",
			Submenu: []*mygo.MenuItem{
				{
					Label:       "New Session",
					Accelerator: "CmdOrCtrl+N",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if fr.Adaptive != nil && fr.Adaptive.Desktop != nil && fr.Adaptive.Desktop.OnNewSession != nil {
							fr.Adaptive.Desktop.OnNewSession()
							fr.SelectBuffer(buffer.Conversation)
							fr.Echo("Session: " + fr.Adaptive.Desktop.ActiveSessionID)
						}
						invalidate()
					},
				},
				{
					Label:       "Attach Files...",
					Accelerator: "CmdOrCtrl+O",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if fr.Adaptive != nil && fr.Adaptive.Desktop != nil && fr.Adaptive.Desktop.Transcript != nil && fr.Adaptive.Desktop.Transcript.OnPickAttachment != nil {
							fr.Adaptive.Desktop.Transcript.OnPickAttachment()
						}
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleClose},
			},
		},
		{Role: mygo.RoleEditMenu},
		{
			Label: "Buffers",
			Submenu: []*mygo.MenuItem{
				{
					Label: "*Dashboard*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Dashboard); invalidate() },
				},
				{
					Label: "*conversation*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Conversation); invalidate() },
				},
				{
					Label: "*Sessions*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Sessions); invalidate() },
				},
				{
					Label: "*Targets*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Targets); invalidate() },
				},
				{
					Label: "*Processes*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Processes); invalidate() },
				},
				{
					Label: "*Models*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Models); invalidate() },
				},
				{
					Label: "*Messages*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Messages); invalidate() },
				},
				{
					Label: "*Help*",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.SelectBuffer(buffer.Help); invalidate() },
				},
				mygo.Separator(),
				{
					Label:       "Switch Theme (SPC t t)",
					Accelerator: "CmdOrCtrl+Shift+T",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.OpenThemeSwitch(); invalidate() },
				},
				{
					Label: "Reload Theme CSS (SPC t r)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.ReloadThemes(); invalidate() },
				},
				{
					Label: "Toggle Liquid Glass (SPC t v)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.ToggleGlass(); invalidate() },
				},
				{
					Label: "Toggle 120Hz Motion (SPC t a)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.ToggleMotion(); invalidate() },
				},
				{
					Label: "Toggle Zen Mode (SPC t z)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) { fr.ToggleZenMode(); invalidate() },
				},
				mygo.Separator(),
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
}
