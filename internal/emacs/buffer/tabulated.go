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
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "ALIAS", Width: 220},
			{Label: "PROVIDER", Width: 140},
			{Label: "ROUTE"},
		}, th)
		activeModel := "gpt-marlin-local"
		if bc != nil && bc.ActiveModel != "" {
			activeModel = bc.ActiveModel
		}
		rows := [][3]string{
			{activeModel, "marlin-core", "active marlin.ui.v1 ConversationJournal route"},
		}
		for _, r := range rows {
			theme.TabulatedRow(c, "mrow-"+r[0], []theme.TableColumn{
				{Label: r[0], Width: 220, Color: th.Info, Bold: true},
				{Label: r[1], Width: 140, Color: th.Accent},
				{Label: r[2], Color: th.InkMuted},
			}, true, th)
		}
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
		{"SPC t t", "Switch Doom Emacs theme profile (doom-one, pi, doom-paper)"},
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
