// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"

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
						{Label: sess.SessionID, Width: 220, Color: th.Info, Bold: active},
						{Label: sess.Title, Color: th.Ink},
						{Label: fmt.Sprintf("%d", sess.TurnCount), Width: 80, Color: th.InkMuted},
						{Label: state, Width: 90, Color: stateCol},
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
						{Label: meta.Address, Color: th.Ink},
						{Label: string(meta.Mode), Width: 130, Color: th.InkMuted},
						{Label: string(meta.Phase), Width: 90, Color: phaseCol},
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
				{Label: proc.ProcessID, Width: 160, Color: th.Info},
				{Label: proc.Command, Color: th.Ink},
				{Label: proc.State, Width: 100, Color: th.Success},
				{Label: fmt.Sprintf("%d", proc.ExitCode), Width: 60, Color: th.InkMuted},
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
						for _, rt := range routes {
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
							theme.TabulatedRow(c, "mrow-"+rt.Alias, []theme.TableColumn{
								{Label: rt.Alias, Width: 190, Color: th.Info, Bold: rt.IsDefault},
								{Label: rt.Provider, Width: 120, Color: th.Accent},
								{Label: rt.Model, Width: 200, Color: th.Ink},
								{Label: rt.Protocol, Width: 150, Color: th.InkMuted},
								{Label: cred, Color: credCol},
							}, rt.IsDefault, th)
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
			{Label: activeModel, Width: 190, Color: th.Info, Bold: true},
			{Label: "marlin-core", Width: 120, Color: th.Accent},
			{Label: "no example.toml loaded", Width: 200, Color: th.InkMuted},
			{Label: "grpc", Width: 150, Color: th.InkMuted},
			{Label: "pass -config ./example.toml", Color: th.Warning},
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
		{"ESC", "Enter Meow NORMAL mode / close Minibuffer list"},
		{"i / a", "Enter Meow INSERT mode (focus Minibuffer)"},
		{"SPC SPC / SPC x / M-x / Cmd+K", "Open Vertico M-x commands"},
		{"SPC b b / Cmd+B", "Switch Emacs buffer (*conversation*, *Sessions*, *Targets*, ...)"},
		{"SPC s / Cmd+P", "Open 120Hz Helm live multi-source search"},
		{"SPC t t", "Switch CSS theme pack (nord-bento, doom-one, doom-tokyo-night, doom-one-light)"},
		{"SPC t v", "Toggle hot-pluggable Liquid Glass GPU surface module"},
		{"SPC t a", "Toggle hot-pluggable 120Hz Spring Motion module"},
		{"SPC t z", "Toggle immersive Zen Mode (borderless Bento widget desktop)"},
		{"SPC t r", "Reload ~/.config/marlin/ui/ themes, CSS & custom S-exp/TOML widgets"},
		{"SPC h", "Open *Help* keybindings buffer"},
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
						{Label: b[0], Width: 280, Color: th.Info, Bold: true},
						{Label: b[1], Color: th.Ink},
					}, false, th)
				}
			})
		})
	})
}
