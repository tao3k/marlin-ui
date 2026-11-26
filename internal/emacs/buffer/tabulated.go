// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderSessions(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("buf-sessions")).Fill().AlignItems(ui.Stretch).Children(func() {
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "SESSION ID", Width: 220},
			{Label: "TITLE"},
			{Label: "TURNS", Width: 80},
			{Label: "STATE", Width: 90},
		}, th)
		if bc == nil || bc.Adaptive == nil || bc.Adaptive.Desktop == nil {
			return
		}
		app := bc.Adaptive.Desktop
		ui.Scroll(c.Key("sess-scroll")).Grow(1).Children(func() {
			ui.Column(c).AlignItems(ui.Stretch).Children(func() {
				for idx, s := range app.Sessions {
					sess := s
					active := sess.SessionID == app.ActiveSessionID
					state := "idle"
					stateCol := th.InkMuted
					if active {
						state = "active"
						stateCol = th.Success
					}
					sel := active || idx == bc.TableCursor
					if theme.TabulatedRow(c, "srow-"+sess.SessionID, []theme.TableColumn{
						{Label: sess.SessionID, Width: 220, Color: th.Info, Bold: active, Mono: true},
						{Label: sess.Title, Color: th.Ink},
						{Label: fmt.Sprintf("%d", sess.TurnCount), Width: 80, Color: th.InkMuted, Mono: true},
						{Label: state, Width: 90, Color: stateCol, Mono: true},
					}, sel, th).Clicked() {
						if app.OnSelectSession != nil {
							app.OnSelectSession(sess.SessionID)
						}
						if bc.OnSelectBuffer != nil {
							bc.OnSelectBuffer(Conversation)
						}
						if bc.OnEcho != nil {
							bc.OnEcho("Switched to " + sess.SessionID)
						}
					}
				}
			})
		})
	})
}

func renderTargets(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("buf-targets")).Fill().AlignItems(ui.Stretch).Children(func() {
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "TARGET", Width: 180},
			{Label: "ENDPOINT"},
			{Label: "MODE", Width: 130},
			{Label: "STATUS", Width: 90},
		}, th)
		if bc == nil || bc.Link == nil || bc.Link.Targets == nil {
			return
		}
		activeID := ""
		if bc.Link.Switcher != nil {
			activeID = bc.Link.Switcher.ActiveID()
		}
		ui.Scroll(c.Key("tgt-scroll")).Grow(1).Children(func() {
			ui.Column(c).AlignItems(ui.Stretch).Children(func() {
				for _, tm := range bc.Link.Targets.List() {
					meta := tm
					active := meta.ID == activeID
					phaseCol := th.InkMuted
					if string(meta.Phase) == "ready" {
						phaseCol = th.Success
					}
					if theme.TabulatedRow(c, "trow-"+meta.ID, []theme.TableColumn{
						{Label: meta.DisplayName, Width: 180, Color: th.Info, Bold: active},
						{Label: meta.Address, Color: th.Ink, Mono: true},
						{Label: string(meta.Mode), Width: 130, Color: th.InkMuted, Mono: true},
						{Label: string(meta.Phase), Width: 90, Color: phaseCol, Mono: true},
					}, active, th).Clicked() {
						if bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.FederationUI != nil && bc.Adaptive.Desktop.FederationUI.OnSelectTarget != nil {
							bc.Adaptive.Desktop.FederationUI.OnSelectTarget(meta.ID)
						}
						if bc.OnEcho != nil {
							bc.OnEcho("Target: " + meta.DisplayName)
						}
					}
				}
			})
		})
	})
}

func renderProcesses(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("buf-procs")).Fill().AlignItems(ui.Stretch).Children(func() {
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "PROCESS", Width: 160},
			{Label: "COMMAND"},
			{Label: "STATE", Width: 100},
			{Label: "EXIT", Width: 60},
		}, th)
		if bc == nil || bc.Adaptive == nil || bc.Adaptive.Desktop == nil || bc.Adaptive.Desktop.Projection == nil {
			return
		}
		snap := bc.Adaptive.Desktop.Projection.Snapshot()
		if len(snap.Processes) == 0 {
			ui.Row(c).Padding(12, 16).Children(func() {
				ui.Text(c, "No managed runtime processes reported.").Font(theme.FontMono).FontSize(12).TextColor(th.InkMuted)
			})
			return
		}
		for _, p := range snap.Processes {
			proc := p
			theme.TabulatedRow(c, "prow-"+proc.ProcessID, []theme.TableColumn{
				{Label: proc.ProcessID, Width: 160, Color: th.Info, Mono: true},
				{Label: proc.Command, Color: th.Ink, Mono: true},
				{Label: proc.State, Width: 100, Color: th.Success, Mono: true},
				{Label: fmt.Sprintf("%d", proc.ExitCode), Width: 60, Color: th.InkMuted, Mono: true},
			}, false, th)
		}
	})
}

func renderModels(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("buf-models")).Fill().AlignItems(ui.Stretch).Children(func() {
		if bc != nil && bc.ModelCfg.Loaded {
			info := fmt.Sprintf("Config: %s (%s)", bc.ModelCfg.SourcePath, bc.ModelCfg.SourceOrigin)
			if think := bc.ModelCfg.ThinkingSummary(); think != "" {
				info += "  ·  thinking: " + think
			}
			ui.Row(c.Key("models-cfg-banner")).Padding(8, 16).Background(theme.GlassTone(c, th.Bar, th, 0.36, 0.60)).
				BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
				ui.Text(c, info).Font(theme.FontMono).FontSize(12).TextColor(th.Info).NoWrap()
			})
		}
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "ALIAS", Width: 190},
			{Label: "PROVIDER", Width: 120},
			{Label: "MODEL", Width: 200},
			{Label: "PROTOCOL", Width: 150},
			{Label: "CREDENTIAL"},
		}, th)
		if bc != nil && bc.ModelCfg.Loaded {
			routes := bc.ModelCfg.Routes()
			if len(routes) > 0 {
				ui.Scroll(c.Key("models-scroll")).Grow(1).Children(func() {
					ui.Column(c).AlignItems(ui.Stretch).Children(func() {
						for _, route := range routes {
							rt := route
							cred := rt.CredentialEnv
							credCol := th.InkMuted
							if cred != "" {
								if rt.CredentialConfigured(nil) {
									cred += " (set)"
									credCol = th.Success
								} else {
									cred += " (unset)"
									credCol = th.Warning
								}
							} else {
								cred = "host-managed"
							}
							isActive := rt.IsDefault
							if strings.TrimSpace(bc.ActiveModel) != "" {
								isActive = strings.HasPrefix(bc.ActiveModel, rt.Alias) || strings.Contains(bc.ActiveModel, rt.Model)
							}
							if theme.TabulatedRow(c, "mrow-"+rt.Alias, []theme.TableColumn{
								{Label: rt.Alias, Width: 190, Color: th.Info, Bold: isActive, Mono: true},
								{Label: rt.Provider, Width: 120, Color: th.Accent},
								{Label: rt.Model, Width: 200, Color: th.Ink, Mono: true},
								{Label: rt.Protocol, Width: 150, Color: th.InkMuted, Mono: true},
								{Label: cred, Color: credCol, Mono: true},
							}, isActive, th).Clicked() && bc.OnSelectModel != nil {
								bc.OnSelectModel(rt.Alias, fmt.Sprintf("%s · %s (%s)", rt.Alias, rt.Model, rt.Provider))
							}
						}
					})
				})
				return
			}
		}
		activeModel := "host-default"
		if bc != nil && bc.ActiveModel != "" {
			activeModel = bc.ActiveModel
		}
		theme.TabulatedRow(c, "mrow-default", []theme.TableColumn{
			{Label: activeModel, Width: 190, Color: th.Info, Bold: true, Mono: true},
			{Label: "marlin-core", Width: 120, Color: th.Accent},
			{Label: "no example.toml loaded", Width: 200, Color: th.InkMuted},
			{Label: "grpc", Width: 150, Color: th.InkMuted, Mono: true},
			{Label: "pass -config ./example.toml", Color: th.Warning, Mono: true},
		}, true, th)
	})
}

func renderMessages(c *ui.Context, th theme.Theme, bc *Context) {
	var lines []string
	if bc != nil {
		lines = bc.MessagesLog
	}
	ui.Scroll(c.Key("msgs-scroll")).Grow(1).Padding(12, 16).Children(func() {
		ui.Column(c).Gap(4).AlignItems(ui.Stretch).Children(func() {
			for i, line := range lines {
				ui.Text(c.Key(fmt.Sprintf("msg-%d", i)), line).
					Font(theme.FontMono).FontSize(12).TextColor(th.InkMuted)
			}
		})
	})
}

func renderHelp(c *ui.Context, th theme.Theme) {
	bindings := [][2]string{
		{"Enter", "Send prompt / confirm Vertico or Helm candidate"},
		{"ESC", "Enter Meow NORMAL mode / dismiss active Doom Popup Drawer / close Minibuffer"},
		{"i / a", "Enter Meow INSERT mode (focus Minibuffer)"},
		{"SPC SPC / SPC x / M-x / Cmd+K", "Open Vertico M-x commands"},
		{"SPC b b / Cmd+B", "Switch Emacs buffer (active workspace buffers prioritized first)"},
		{"SPC w v / SPC w s", "Split active workspace vertically (left | right) or horizontally (top / bottom)"},
		{"SPC w w / SPC w d / SPC w o", "Cycle focused pane / close split pane / delete other windows"},
		{"SPC w p / SPC w r", "Toggle Doom Popup Drawer / raise popup buffer into main window"},
		{"SPC l c / SPC l d / SPC l 1..4", "Create, kill, or jump between isolated persp-mode workspaces"},
		{"SPC s / Cmd+P", "Open 120Hz Helm live multi-source search"},
		{"SPC t t / v / a / z", "Switch theme / Liquid Glass / 120Hz Motion / Zen Mode"},
		{"SPC t r", "Reload ~/.config/marlin/ui/ themes, CSS & custom S-exp/TOML widgets"},
		{"SPC h", "Open *Help* keybindings popup drawer"},
		{"Cmd+,", "Open *Settings* buffer (Theme, Remote Targets, Account, Features)"},
		{"C-n / C-p / Up / Down", "Move candidate selection"},
		{"j / k (Normal)", "Move selection down / up in tabulated buffers"},
		{"C-g", "Cancel active keypad or minibuffer action"},
	}
	ui.Column(c.Key("buf-help")).Fill().AlignItems(ui.Stretch).Children(func() {
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "KEYBINDING (MEOW / DOOM)", Width: 280},
			{Label: "COMMAND"},
		}, th)
		ui.Scroll(c.Key("help-scroll")).Grow(1).Children(func() {
			ui.Column(c).AlignItems(ui.Stretch).Children(func() {
				for _, b := range bindings {
					theme.TabulatedRow(c, "hrow-"+b[0], []theme.TableColumn{
						{Label: b[0], Width: 280, Color: th.Info, Bold: true, Mono: true},
						{Label: b[1], Color: th.Ink},
					}, false, th)
				}
			})
		})
	})
}
