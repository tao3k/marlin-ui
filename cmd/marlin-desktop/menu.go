// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package main

import (
	"github.com/egoist/mygo"

	"github.com/tao3k/marlin-mygo/engine/shell"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

func buildDesktopMenu(
	adaptive *mygoshell.AdaptiveShell,
	onSavePrefs func(),
	invalidate func(),
) *mygo.Menu {
	app := adaptive.Desktop
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
							Credits:            appconfig.BuildSummary("marlin-desktop") + "\nEngine: github.com/tao3k/marlin-mygo",
						})
					},
				},
				mygo.Separator(),
				{
					Label:       "Command Palette",
					Accelerator: "CmdOrCtrl+K",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.OpenCommandPalette()
						invalidate()
					},
				},
				{
					Label:       "Helm Live Search",
					Accelerator: "CmdOrCtrl+P",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.OpenHelmSearch()
						invalidate()
					},
				},
				mygo.Separator(),
				{
					Label: "Federation Cockpit",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.OpenFederation()
						invalidate()
					},
				},
				{
					Label: "Governance & Features",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.OpenGovernance()
						invalidate()
					},
				},
				{
					Label: "Activity Center",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.OpenActivityCenter()
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
						if app.OnNewSession != nil {
							app.OnNewSession()
						}
						invalidate()
					},
				},
				{
					Label:       "Attach Files...",
					Accelerator: "CmdOrCtrl+O",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if app.Transcript != nil && app.Transcript.OnPickAttachment != nil {
							app.Transcript.OnPickAttachment()
						}
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleClose},
			},
		},
		{Role: mygo.RoleEditMenu},
		{
			Label: "View",
			Submenu: []*mygo.MenuItem{
				{
					Label:       "Toggle Sidebar",
					Accelerator: "CmdOrCtrl+B",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						app.ToggleSidebar()
						onSavePrefs()
						invalidate()
					},
				},
				{
					Label:       "Cycle Theme",
					Accelerator: "CmdOrCtrl+Shift+T",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						adaptive.CycleTheme()
						onSavePrefs()
						invalidate()
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{
			Label: "Workspace",
			Submenu: []*mygo.MenuItem{
				{
					Label:       "Split Right",
					Accelerator: "CmdOrCtrl+D",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							_, cur := adaptive.Windows.FocusedBuffer()
							_, _ = adaptive.Windows.SplitRight(nextWorkspaceBuffer(app, cur))
							invalidate()
						}
					},
				},
				{
					Label:       "Split Below",
					Accelerator: "CmdOrCtrl+Shift+D",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							_, cur := adaptive.Windows.FocusedBuffer()
							_, _ = adaptive.Windows.SplitBelow(nextWorkspaceBuffer(app, cur))
							invalidate()
						}
					},
				},
				{
					Label: "Close Focused Split",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							_ = adaptive.Windows.DeleteFocused()
							invalidate()
						}
					},
				},
				mygo.Separator(),
				{
					Label: "Toggle Zoom Split",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							adaptive.Windows.ToggleZoom()
							invalidate()
						}
					},
				},
				{
					Label: "Balance Splits",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							adaptive.Windows.Balance()
							invalidate()
						}
					},
				},
				{
					Label: "Undo Layout (Winner)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							adaptive.Windows.Undo()
							invalidate()
						}
					},
				},
				{
					Label: "Redo Layout (Winner)",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							adaptive.Windows.Redo()
							invalidate()
						}
					},
				},
				mygo.Separator(),
				{
					Label: "Next Window",
					Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
						if adaptive.Windows != nil {
							adaptive.Windows.FocusNext()
							invalidate()
						}
					},
				},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
}

func nextWorkspaceBuffer(app *mygoshell.WorkbenchApp, current shell.PaneBuffer) shell.PaneBuffer {
	if app != nil && len(app.WindowBuffers) > 1 {
		for i, b := range app.WindowBuffers {
			if b.BufferID == current.BufferID {
				return app.WindowBuffers[(i+1)%len(app.WindowBuffers)]
			}
		}
		return app.WindowBuffers[0]
	}
	return current
}
