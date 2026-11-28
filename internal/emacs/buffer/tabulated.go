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

func renderHelp(c *ui.Context, th theme.Theme, bc *Context) {
	bindings := [][2]string{
		{"C-x C-f / SPC f f", "Find file (live Vertico directory browser · vertico-directory)"},
		{"C-c p f / SPC p f", "Find file in current Git/jj project (Orderless fuzzy search)"},
		{"C-x C-r / SPC f r", "Open recently visited file (recentf)"},
		{"C-h m / SPC h m", "Describe active buffer's Major Mode, Minor Modes, Polymode & Keymap"},
		{"C-h k / SPC h k", "Describe key or chord across active Emacs keymap precedence"},
		{"C-h f / SPC h f", "Describe command documentation & all bound keybindings"},
		{"C-h b / SPC h b", "Describe all active local & global keybindings"},
		{"C-h l / SPC h l", "View lossage: chronological log of last 60 commands"},
		{"C-. / SPC . / C-c e", "Embark contextual actions for target at point"},
		{"C-u (Universal Prefix)", "Numeric prefix multiplier (4x / 16x / 64x) for repeatable commands"},
		{"C-c . / SPC m .", "Repeat last executed command (honors C-u count)"},
		{"C-x w m / SPC w m", "Zoom / restore focused window (doom/window-maximize-buffer)"},
		{"C-x w t / SPC w T", "Transpose split orientation Vertical <-> Horizontal"},
		{"C-x x r / SPC b R", "Rename active buffer across tabs, windows & session"},
		{"C-c z / SPC b z", "Bury current buffer to tail of workspace ring"},
		{"C-c t d/a/r/l · SPC t d/a/r/l", "Toggle dedicated / Aitia audit / read-only / LLM assist minor modes"},
		{"C-x g / SPC g g", "Open *magit* AI-Native jj + Git + Lambda Aitia workbench"},
		{"C-x M-g / SPC g d", "Open Magit dispatch transient (?)"},
		{"C-c g / SPC g a", "Open 50/50 AI + Aitia Code Audit split"},
		{"C-x b / SPC b b", "Buffer & Mode Creator Picker (switch buffer / + New LLM Chat / + Clone Window)"},
		{"C-x 4 b / SPC b B", "Switch buffer in other window (auto-splits if single window)"},
		{"C-x 4 0 / SPC b K", "Kill active buffer and close its split window"},
		{"C-x r w / j / l · SPC w R/J/L", "Save / jump / list Emacs window configuration registers"},
		{"M-p / M-n (in Minibuffer)", "Cycle per-mode Emacs Minibuffer history ring"},
		{"C-c b / SPC b i", "Open *Ibuffer* interactive buffer manager grouped by Major Mode"},
		{"C-x k / SPC b d", "Kill active buffer in workspace"},
		{"C-c [ / ] · SPC b [ / ]", "Cycle previous / next buffer in workspace"},
		{"C-x 3 / C-x 2 · SPC w v / s", "Split window right / below (linked view of current buffer)"},
		{"C-x o / C-x 0 / C-x 1", "Other window / close window / delete other windows"},
		{"C-x w d / SPC w t", "Toggle dedicated-window-mode pin on focused window"},
		{"C-c u / U · SPC w u / U", "Winner-mode undo / redo window layout"},
		{"C-x p / SPC ~", "Toggle Doom Popup Drawer"},
		{"C-x t N / SPC TAB N", "Create named isolated perspective workspace (persp-mode)"},
		{"M-x / SPC SPC / Cmd+K", "Open Vertico M-x commands"},
		{"C-s / SPC s / Cmd+P", "Open 120Hz Helm live multi-source search"},
		{"ESC / C-g", "Return to Meow NORMAL mode / cancel keypad or minibuffer"},
	}
	ui.Column(c.Key("buf-help")).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Scroll(c.Key("help-scroll")).Grow(1).Padding(10, 14).Children(func() {
			ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
				if bc != nil && bc.HelpIntrospection != nil {
					hi := bc.HelpIntrospection
					if hi.KeyDoc != nil {
						kd := hi.KeyDoc
						ui.Column(c.Key("help-key-doc")).Padding(10, 14).Gap(5).Radius(8).
							Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.56, 0.86)).
							BorderWidth(1).BorderColor(th.Accent).AlignItems(ui.Stretch).Children(func() {
							ui.Text(c, fmt.Sprintf("%s · %s", kd.Heading, kd.Subject)).
								Font(theme.FontMono).FontSize(13).FontWeight(700).TextColor(th.Accent)
							ui.Text(c, fmt.Sprintf("Keymap Tier: [%s]  ·  Command: %s", kd.Tier, kd.Command)).
								Font(theme.FontMono).FontSize(11.5).FontWeight(600).TextColor(th.Warning)
							if kd.Description != "" {
								ui.Text(c, kd.Description).
									Font(theme.FontMono).FontSize(11.5).TextColor(th.Ink)
							}
							if len(kd.Aliases) > 0 {
								ui.Text(c, "Bound Chords: "+strings.Join(kd.Aliases, "  ·  ")).
									Font(theme.FontMono).FontSize(11).TextColor(th.Info)
							}
						})
					}
					ui.Column(c.Key("help-describe-mode")).Padding(10, 14).Gap(6).Radius(8).
						Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.48, 0.78)).
						BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
						ui.Text(c, fmt.Sprintf("Emacs describe-mode (C-h m) · %s", hi.BufferName)).
							Font(theme.FontMono).FontSize(13).FontWeight(700).TextColor(th.Accent)
						ui.Text(c, fmt.Sprintf("Major Mode: %s (%s) · Display: %s · Read-Only: %v",
							hi.MajorTitle, hi.MajorModeID, hi.DisplayAction, hi.ReadOnly)).
							Font(theme.FontMono).FontSize(11.5).TextColor(th.Ink)
						if len(hi.ParentChain) > 0 {
							ui.Text(c, "Inheritance: "+strings.Join(hi.ParentChain, " → ")).
								Font(theme.FontMono).FontSize(11.5).TextColor(th.Info)
						}
						if len(hi.MinorLighters) > 0 {
							ui.Text(c, "Active Minor Modes: "+strings.Join(hi.MinorLighters, " · ")).
								Font(theme.FontMono).FontSize(11.5).TextColor(th.Success)
						}
						if hi.InnerModeID != "" {
							ui.Text(c, fmt.Sprintf("Polymode Active Span: %s → Inner Mode: %s", hi.FocusedSpan, hi.InnerModeID)).
								Font(theme.FontMono).FontSize(11.5).FontWeight(600).TextColor(th.Warning)
						} else if len(hi.PolymodeRules) > 0 {
							ui.Text(c, fmt.Sprintf("Polymode Host (%d inner span rules; current span: %s)", len(hi.PolymodeRules), hi.FocusedSpan)).
								Font(theme.FontMono).FontSize(11.5).TextColor(th.InkMuted)
						}
						for i, pr := range hi.PolymodeRules {
							ui.Text(c.Key(fmt.Sprintf("hpoly-%d", i)), fmt.Sprintf("  span %-11s → %-24s [%s]", pr[0], pr[1], pr[2])).
								Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted)
						}
					})
					if len(hi.LocalBindings) > 0 {
						theme.TabulatedHeader(c, []theme.TableColumn{
							{Label: "TIER", Width: 90},
							{Label: "KEY", Width: 110},
							{Label: "LOCAL MODE / POLYMODE COMMAND"},
						}, th)
						for i, lb := range hi.LocalBindings {
							theme.TabulatedRow(c, fmt.Sprintf("hlrow-%d", i), []theme.TableColumn{
								{Label: lb[0], Width: 90, Color: th.Warning, Bold: true, Mono: true},
								{Label: lb[1], Width: 110, Color: th.Info, Bold: true, Mono: true},
								{Label: lb[2], Color: th.Ink},
							}, false, th)
						}
					}
				}
				if bc != nil && bc.HelpIntrospection != nil && len(bc.HelpIntrospection.GlobalBindings) > 0 {
					theme.TabulatedHeader(c, []theme.TableColumn{
						{Label: "LIVE HYDRA / GLOBAL CHORD (C-h b)", Width: 280},
						{Label: "COMMAND"},
					}, th)
					for i, gb := range bc.HelpIntrospection.GlobalBindings {
						theme.TabulatedRow(c, fmt.Sprintf("hgb-%d", i), []theme.TableColumn{
							{Label: gb[0], Width: 280, Color: th.Info, Bold: true, Mono: true},
							{Label: gb[1], Color: th.Ink},
						}, false, th)
					}
				}
				theme.TabulatedHeader(c, []theme.TableColumn{
					{Label: "GLOBAL KEYBINDING (EMACS / DOOM)", Width: 280},
					{Label: "COMMAND"},
				}, th)
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
