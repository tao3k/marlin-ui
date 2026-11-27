// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && (marlin_feat_chatroom || marlin_full || (!marlin_feat_compare && !marlin_feat_canvas))

package featurekit

import (
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func (m *bentoChatRoomModule) renderDesktopGateEmbed(c *ui.Context, th theme.Theme, item RoomMessageView) {
	var (
		q       *promptgate.Queue
		pending *promptgate.PromptRequest
	)
	if m.gateQueue != nil {
		q = m.gateQueue
		if req, _, _, ok := q.Active(); ok && req.ID == item.GateID {
			cp := req
			pending = &cp
		}
	}
	stripe := th.Warning
	if pending == nil {
		stripe = th.Success
	}
	ui.Row(c.Key("br-gate-"+item.ID)).FillWidth().Radius(12).Clip().
		Background(kit.WithAlpha(th.Bar, 0.56)).Border(1, kit.WithAlpha(stripe, 0.30)).Children(func() {
		ui.Box(c).Width(3).FillHeight().Background(stripe)
		ui.Column(c).Basis(0).Grow(1).Padding(10, 12).Gap(6).Children(func() {
			if pending != nil {
				ui.Text(c, "APPROVAL REQUIRED · "+pending.Title).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Warning).SingleLine()
				if pending.Prompt != "" {
					ui.Text(c, pending.Prompt).Font(kit.FontDisplay).FontSize(12.5).TextColor(th.InkMuted)
				}
				ui.Row(c).Gap(8).Children(func() {
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "br-gate-yes-" + item.GateID, Label: "Approve", Icon: theme.IconApprove, Variant: theme.BtnGlassProminent, Size: theme.BtnSizeSM, Tone: th.Success, Pill: true,
					}, th).Clicked() {
						_, _, _ = q.Resolve(pending.ID, promptgate.PromptResponse{
							PromptID: pending.ID, Action: promptgate.ActionApproveOnce, IdempotencyKey: "desk-room-yes-" + pending.ID,
						})
						notifyRoomInvalidated()
					}
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "br-gate-no-" + item.GateID, Label: "Reject", Icon: theme.IconReject, Variant: theme.BtnDestructive, Size: theme.BtnSizeSM, Tone: th.Danger, Pill: true,
					}, th).Clicked() {
						_, _, _ = q.Resolve(pending.ID, promptgate.PromptResponse{
							PromptID: pending.ID, Action: promptgate.ActionReject, IdempotencyKey: "desk-room-no-" + pending.ID,
						})
						notifyRoomInvalidated()
					}
				})
			} else {
				ui.Text(c, "Gate Resolved · Tier-0 Audited").Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Success).SingleLine()
			}
		})
	})
}

func desktopCanGroup(prev, cur RoomMessageView) bool {
	if prev.AuthorID == "" || prev.AuthorID != cur.AuthorID || cur.Pinned || cur.GateID != "" {
		return false
	}
	if prev.CreatedUnix > 0 && cur.CreatedUnix > 0 {
		diff := cur.CreatedUnix - prev.CreatedUnix
		if diff < 0 || diff > 300 {
			return false
		}
	}
	return true
}

func desktopMsgMentions(msg RoomMessageView, handle string) bool {
	target := strings.ToLower(strings.TrimPrefix(handle, "@"))
	for _, m := range msg.Mentions {
		if strings.ToLower(strings.TrimPrefix(m, "@")) == target {
			return true
		}
	}
	return false
}

func desktopPermChip(c *ui.Context, label string, enabled bool, tone ui.Color, th theme.Theme) {
	col := th.InkMuted
	bg := kit.WithAlpha(th.BorderSubtle, 0.28)
	if enabled {
		col = tone
		bg = kit.WithAlpha(tone, 0.16)
	}
	ui.Text(c, label).Font(theme.FontMono).FontSize(10).FontWeight(600).TextColor(col).
		Padding(3, 8).Radius(theme.RadiusPill).Background(bg).SingleLine()
}

func renderDesktopMentionPopover(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	panelBg, hairline ui.Color,
	participants []RoomParticipantView,
	query string,
) string {
	agents, humans := MatchRoomParticipants(participants, query)
	if len(agents) == 0 && len(humans) == 0 {
		return ""
	}
	picked := ""
	marginX := float32(16)
	if strings.HasPrefix(keyPrefix, "br-th-") {
		marginX = 0
	}
	ui.Row(c.Key(keyPrefix+"-wrap")).Padding(0, marginX).Shrink(0).Children(func() {
		pop := ui.Column(c.Key(keyPrefix+"-box")).Basis(0).Grow(1).MinWidth(0).Padding(8, 10).Gap(5).Radius(14).AlignItems(ui.Stretch).
			Background(panelBg).Border(1, hairline).Clip()
		if c.Vibrancy() {
			pop.Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.56)})
		}
		pop.Children(func() {
			renderGroup := func(header string, list []RoomParticipantView) {
				if len(list) == 0 {
					return
				}
				ui.Text(c, header).Font(theme.FontMono).FontSize(9.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
					Padding(2, 6, 0, 6).SingleLine()
				for _, p := range list {
					mem := p
					btnLabel := "Mention @" + mem.Handle
					if strings.HasPrefix(keyPrefix, "br-th-") {
						btnLabel = "Thread mention @" + mem.Handle
					}
					btn := kit.ActionControl(ui.ButtonBase(c.Key(keyPrefix+"-"+mem.ID)).Label(btnLabel).
						FillWidth().Padding(5, 8).Gap(8).Radius(10).AlignItems(ui.Center).Justify(ui.Start))
					if btn.Hovered() || btn.Pressed() {
						btn.Background(kit.WithAlpha(th.Accent, 0.16))
					}
					btn.Children(func() {
						theme.RoleEmblemAvatar(c, mem.Name, mem.Handle, mem.IsAgent, mem.Presence, 22, th)
						ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).AlignItems(ui.Start).Children(func() {
							ui.Row(c).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
								ui.Text(c, mem.Name).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Ink).SingleLine()
								ui.Text(c, "@"+mem.Handle).Font(theme.FontMono).FontSize(10.5).TextColor(th.Accent).SingleLine()
							})
							if mem.Role != "" {
								ui.Text(c, mem.Role).Font(kit.FontDisplay).FontSize(10.5).TextColor(th.InkMuted).
									SingleLine().Ellipsis("…").MinWidth(0).Clip()
							}
						})
						ui.Text(c, theme.PresenceLabel(mem.Presence)).Font(theme.FontMono).FontSize(9.5).
							TextColor(theme.PresenceColor(mem.Presence, th)).SingleLine().Shrink(0)
					})
					if btn.Clicked() {
						picked = mem.Handle
					}
				}
			}
			renderGroup("AI AGENTS", agents)
			renderGroup("HUMANS", humans)
		})
	})
	return picked
}

func renderDesktopSlashPopover(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	panelBg, hairline ui.Color,
	query string,
) (insert, execCmd string) {
	cmds := []struct {
		cmd, desc, insert, icon string
	}{
		{"/task", "Create a shared channel task", "/task ", "checklist"},
		{"/ask", "Route a question to @architect", "/ask @architect ", "sparkles"},
		{"/pin", "View pinned channel messages", "/pin", "pin"},
		{"/clear", "Clear current draft", "/clear", "xmark.circle"},
	}
	var matched []int
	for i, item := range cmds {
		if query == "" || strings.Contains(strings.TrimPrefix(item.cmd, "/"), query) {
			matched = append(matched, i)
		}
	}
	if len(matched) == 0 {
		return "", ""
	}
	ui.Row(c.Key(keyPrefix+"-wrap")).Padding(0, 16).Shrink(0).Children(func() {
		pop := ui.Column(c.Key(keyPrefix+"-box")).Basis(0).Grow(1).MinWidth(0).Padding(8, 10).Gap(4).Radius(14).AlignItems(ui.Stretch).
			Background(panelBg).Border(1, hairline).Clip()
		if c.Vibrancy() {
			pop.Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.56)})
		}
		pop.Children(func() {
			ui.Text(c, "ROOM COMMANDS").Font(theme.FontMono).FontSize(9.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
				Padding(2, 6, 0, 6).SingleLine()
			for _, idx := range matched {
				item := cmds[idx]
				btn := kit.ActionControl(ui.ButtonBase(c.Key(keyPrefix+"-"+item.cmd)).Label(item.cmd).
					FillWidth().Padding(6, 8).Gap(8).Radius(10).AlignItems(ui.Center).Justify(ui.Start))
				if btn.Hovered() || btn.Pressed() {
					btn.Background(kit.WithAlpha(th.Accent, 0.16))
				}
				btn.Children(func() {
					theme.SFIcon(c, item.icon, 13, th.Accent, false)
					ui.Text(c, item.cmd).Font(theme.FontMono).FontSize(12).FontWeight(700).TextColor(th.Ink).SingleLine()
					ui.Text(c, item.desc).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
				})
				if btn.Clicked() {
					if item.insert == "/pin" || item.insert == "/clear" {
						execCmd = item.insert
					} else {
						insert = item.insert
					}
				}
			}
		})
	})
	return insert, execCmd
}

func summarizeForTask(body string) string {
	s := strings.TrimSpace(strings.ReplaceAll(body, "\n", " "))
	if len(s) > 72 {
		s = strings.TrimSpace(s[:72]) + "…"
	}
	return s
}
