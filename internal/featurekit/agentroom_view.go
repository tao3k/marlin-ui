// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && (marlin_feat_chatroom || marlin_full || (!marlin_feat_compare && !marlin_feat_canvas))

package featurekit

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/features/chatroom"
	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// renderBentoAgentRoom renders Marlin Desktop's unified Multi-Role Agent Room
// at 1:1 visual & behavioral parity with Mobile: Apple 26 × Vercel Geist
// buttons, strict CHANNELS vs AI BOTS · DIRECT rail, HUMANS vs AI AGENTS roster
// with role-tinted SF Symbol emblems, edge-to-edge grouped Markdown stream,
// inline Tier-0 Gate Embeds, live Bot Thinking status bar, `/` commands, and
// real Rust + DeepSeek multi-agent replies via m.postMessage.
func (m *bentoChatRoomModule) renderBentoAgentRoom(c *ui.Context, t *ui.Theme) {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return
	}
	th := theme.FromMyGoTheme(t)
	snap := AgentRoomSnapshot()
	if !snap.Available {
		snap = m.fullSnapshot()
	}

	canvas := th.Canvas
	panelBg := theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.72)
	insetBg := theme.GlassTone(c, th.Bar, th, 0.34, 0.68)
	hairline := kit.WithAlpha(th.BorderSubtle, 0.55)
	if c.Vibrancy() {
		canvas = kit.WithAlpha(th.Canvas, 0.14)
	}

	activeCh := RoomChannelView{ID: snap.ActiveChannelID, Name: "workspace"}
	for _, ch := range snap.Channels {
		if ch.ID == snap.ActiveChannelID {
			activeCh = ch
			break
		}
	}
	partByID := make(map[string]RoomParticipantView, len(snap.Participants))
	var humans, agents []RoomParticipantView
	currentUserHandle := "you"
	for _, p := range snap.Participants {
		partByID[p.ID] = p
		if p.ID == snap.CurrentActorID && p.Handle != "" {
			currentUserHandle = strings.ToLower(p.Handle)
		}
		if p.IsAgent {
			agents = append(agents, p)
		} else {
			humans = append(humans, p)
		}
	}

	doneCount := 0
	for _, tk := range snap.Tasks {
		if tk.Status == "DONE" {
			doneCount++
		}
	}

	var (
		selectCh         string
		mention          string
		mainMentionPick  string
		mainSlashInsert  string
		mainSlashExec    string
		toggleMainAt     bool
		thMentionPick    string
		toggleThAt       bool
		sendMsg          bool
		openThread       string
		closeThread      bool
		sendThread       bool
		addItem          bool
		cycleTaskID      string
		promoteTitle     string
		toggleRxMsg      string
		toggleRxTag      string
	)

	railW := float32(252)
	if snap.ActiveThreadID != "" {
		railW = 220
	}

	ui.Row(c.Key("bento-room-stage")).Fill().Background(canvas).Padding(14, 18).Gap(12).
		AlignItems(ui.Stretch).Clip().Children(func() {
		// Left Column: Dual-tier Channels & Direct Bots + HUMANS / AI AGENTS Roster.
		ui.Column(c.Key("bento-room-rail")).Width(railW).Shrink(0).Gap(12).AlignItems(ui.Stretch).Clip().Children(func() {
			// Dual-tier Channel Directory Card.
			ui.Column(c.Key("bento-room-channels")).Radius(16).Background(panelBg).Border(1, hairline).
				Padding(12).Gap(10).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Gap(3).AlignItems(ui.Stretch).Children(func() {
					ui.Text(c, "CHANNELS").Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
						Padding(0, 6, 3, 6).SingleLine()
					for _, ch := range snap.Channels {
						if ch.IsDM {
							continue
						}
						item := ch
						active := item.ID == snap.ActiveChannelID
						btn := kit.ActionControl(ui.ButtonBase(c.Key("br-ch-"+item.ID)).Label("#"+item.Name).
							Height(30).Padding(0, 8).Radius(9).Gap(7).AlignItems(ui.Center).Justify(ui.Start))
						fg := th.InkMuted
						weight := 500
						if active {
							btn.Background(kit.WithAlpha(th.Accent, 0.16))
							fg = th.Ink
							weight = 600
						} else if btn.Hovered() {
							btn.Background(insetBg)
							fg = th.Ink
						}
						btn.Children(func() {
							if active {
								ui.Box(c).Width(2.5).Height(14).Radius(2).Background(th.Accent)
							}
							theme.SFIcon(c, "number", 12, th.Accent, active)
							ui.Text(c, item.Name).Font(kit.FontDisplay).FontSize(13).FontWeight(weight).TextColor(fg).
								Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
							if item.UnreadCount > 0 && !active {
								ui.Badge(c, fmt.Sprintf("%d", item.UnreadCount))
							}
						})
						if btn.Clicked() {
							selectCh = item.ID
						}
					}
				})

				ui.Box(c).Height(1).FillWidth().Background(hairline)

				ui.Column(c).Gap(3).AlignItems(ui.Stretch).Children(func() {
					ui.Text(c, "AI BOTS · DIRECT").Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
						Padding(0, 6, 3, 6).SingleLine()
					for _, ch := range snap.Channels {
						if !ch.IsDM {
							continue
						}
						item := ch
						active := item.ID == snap.ActiveChannelID
						bot := partByID[item.BotParticipantID]
						btn := kit.ActionControl(ui.ButtonBase(c.Key("br-dm-"+item.ID)).Label("@"+bot.Handle).
							Height(34).Padding(0, 8).Radius(9).Gap(8).AlignItems(ui.Center).Justify(ui.Start))
						fg := th.InkMuted
						weight := 500
						if active {
							btn.Background(kit.WithAlpha(th.Accent, 0.16))
							fg = th.Ink
							weight = 600
						} else if btn.Hovered() {
							btn.Background(insetBg)
							fg = th.Ink
						}
						btn.Children(func() {
							theme.RoleEmblemAvatar(c, bot.Name, bot.Handle, true, bot.Presence, 22, th)
							ui.Text(c, item.Name).Font(kit.FontDisplay).FontSize(13).FontWeight(weight).TextColor(fg).
								Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
							ui.Text(c, theme.PresenceLabel(bot.Presence)).Font(theme.FontMono).FontSize(9.5).
								TextColor(theme.PresenceColor(bot.Presence, th)).SingleLine()
						})
						if btn.Clicked() {
							selectCh = item.ID
						}
					}
				})
			})

			// Strict HUMANS vs AI AGENTS Roster Card.
			ui.Column(c.Key("bento-room-agents")).Grow(1).MinHeight(0).Radius(16).Background(panelBg).Border(1, hairline).
				Padding(12).Gap(8).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c.Key("br-agents-scroll")).Grow(1).Children(func() {
					ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
						renderRosterGroup := func(header string, list []RoomParticipantView) {
							ui.Column(c).Gap(3).AlignItems(ui.Stretch).Children(func() {
								ui.Text(c, header).Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
									Padding(0, 6, 2, 6).SingleLine()
								for _, p := range list {
									part := p
									row := kit.ActionControl(ui.ButtonBase(c.Key("br-p-"+part.ID)).Label("@"+part.Handle).
										Height(40).Padding(0, 6).Radius(9).Gap(8).AlignItems(ui.Center).Justify(ui.Start))
									if row.Hovered() {
										row.Background(insetBg)
									}
									row.Children(func() {
										theme.RoleEmblemAvatar(c, part.Name, part.Handle, part.IsAgent, part.Presence, 26, th)
										ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
											ui.Row(c).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
												ui.Text(c, part.Name).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Ink).
													SingleLine().Ellipsis("…").MinWidth(0).Clip()
												if part.IsAgent {
													theme.VerifiedAIBadge(c, th)
												} else if part.CanApproveGates {
													theme.ApproverBadge(c, th)
												}
											})
											meta := part.Role
											if part.HostNode != "" {
												meta += " · " + part.HostNode
											}
											ui.Text(c, meta).Font(theme.FontMono).FontSize(10).TextColor(th.InkMuted).
												SingleLine().Ellipsis("…").MinWidth(0).Clip()
										})
									})
									if row.Clicked() {
										mention = "@" + part.Handle
									}
								}
							})
						}
						renderRosterGroup(fmt.Sprintf("HUMANS · %d", len(humans)), humans)
						renderRosterGroup(fmt.Sprintf("AI AGENTS · %d", len(agents)), agents)
					})
				})
			})
		})

		// Main Center Stage: Unified Header + Segmented View (Timeline | Tasks | Pinned | Roles).
		ui.Column(c.Key("bento-room-main")).Basis(0).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Stretch).Children(func() {
			prefix := "#"
			if activeCh.IsDM {
				prefix = "@"
			}
			// Unified Apple 26 × Geist Header Bar.
			ui.Row(c.Key("bento-room-header")).Height(54).MinWidth(0).Radius(16).Background(panelBg).Border(1, hairline).
				Padding(0, 14).Gap(8).AlignItems(ui.Center).Clip().Children(func() {
				if activeCh.IsDM {
					if bot, ok := partByID[activeCh.BotParticipantID]; ok {
						theme.RoleEmblemAvatar(c, bot.Name, bot.Handle, true, bot.Presence, 26, th)
					}
				} else {
					theme.SFIcon(c, "number", 15, th.Accent, false)
				}
				ui.Text(c, prefix+activeCh.Name).Font(kit.FontDisplay).FontSize(15).FontWeight(700).TextColor(th.Ink).SingleLine()
				if snap.SyncLabel != "" {
					syncTone := th.Success
					if snap.SyncMode == "cove-connecting" {
						syncTone = th.Warning
					} else if snap.SyncMode == "local" {
						syncTone = th.Info
					}
					ui.Text(c, snap.SyncLabel).Font(theme.FontMono).FontSize(10).FontWeight(600).TextColor(syncTone).
						Padding(2, 8).Radius(theme.RadiusPill).Background(kit.WithAlpha(syncTone, 0.14)).SingleLine()
				}
				if activeCh.Topic != "" {
					ui.Text(c, activeCh.Topic).Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted).
						Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				} else {
					ui.Spacer(c)
				}

				tabIdx := 0
				switch m.activeTab {
				case chatroom.TabMatters:
					tabIdx = 1
				case chatroom.TabRoles:
					tabIdx = 2
				}
				seg := ui.Segmented(c.Key("br-mode-seg"), &tabIdx, "Timeline", fmt.Sprintf("Tasks %d/%d", doneCount, len(snap.Tasks)), "Roles").
					Label("Room view")
				if seg.Changed() {
					switch tabIdx {
					case 1:
						m.activeTab = chatroom.TabMatters
					case 2:
						m.activeTab = chatroom.TabRoles
					default:
						m.activeTab = chatroom.TabTimeline
					}
				}
			})

			switch m.activeTab {
			case chatroom.TabMatters:
				ui.Column(c.Key("br-matters-card")).Grow(1).MinHeight(0).Radius(16).Background(panelBg).Border(1, hairline).
					Padding(16).Gap(12).AlignItems(ui.Stretch).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Grow(1).Padding(7, 12).Radius(12).Background(insetBg).Border(1, hairline).AlignItems(ui.Center).Children(func() {
							inp := ui.TextInputBase(c.Key("br-new-matter"), &m.itemDraft).
								Placeholder("Add shared channel task (syncs to Dashboard & Mobile Bento)…").
								Font(kit.FontDisplay).FontSize(13.5).TextColor(th.Ink).Grow(1)
							if inp.Submitted() {
								addItem = true
							}
						})
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: "br-add-matter", Label: "Add Task", Icon: theme.IconPlus, Variant: theme.BtnGlassProminent, Size: theme.BtnSizeMD, Tone: th.Accent, Pill: true,
						}, th).Clicked() {
							addItem = true
						}
					})
					ui.Scroll(c.Key("br-matters-scroll")).Grow(1).Children(func() {
						ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
							for _, tk := range snap.Tasks {
								item := tk
								done := item.Status == "DONE"
								tone := th.Info
								sym := "circle"
								if done {
									tone = th.Success
									sym = "checkmark.circle"
								} else if item.Status == "IN_PROGRESS" {
									tone = th.Warning
									sym = "clock"
								}
								ui.Row(c.Key("br-item-"+item.ID)).Radius(12).Background(insetBg).Border(1, hairline).
									Padding(10, 14).Gap(10).AlignItems(ui.Center).Children(func() {
									if theme.StyledButton(c, theme.ButtonSpec{
										Key: "br-st-" + item.ID, Label: item.Status, Icon: sym, IconFilled: done, Variant: theme.BtnTinted, Size: theme.BtnSizeSM, Tone: tone, Pill: true,
									}, th).Clicked() {
										cycleTaskID = item.ID
									}
									ui.Text(c, item.Title).Font(kit.FontDisplay).FontSize(13.5).FontWeight(600).TextColor(th.Ink).
										Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
									ui.Text(c, "@"+strings.TrimPrefix(item.Owner, "@")).Font(theme.FontMono).FontSize(11.5).TextColor(th.InkMuted).SingleLine()
								})
							}
						})
					})
				})

			case chatroom.TabRoles:
				ui.Column(c.Key("br-roles-card")).Grow(1).MinHeight(0).Radius(16).Background(panelBg).Border(1, hairline).
					Padding(16).Gap(12).AlignItems(ui.Stretch).Children(func() {
					ui.Row(c).Padding(10, 12).Gap(8).Radius(12).AlignItems(ui.Center).
						Background(kit.WithAlpha(th.Warning, 0.12)).Border(1, kit.WithAlpha(th.Warning, 0.28)).Children(func() {
						theme.SFIcon(c, "shield.lefthalf.filled", 14, th.Warning, true)
						ui.Text(c, "Tier-0 Governance Invariant: PromptGate approval is exclusively human-owned.").
							Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Ink).SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
					ui.Scroll(c.Key("br-roles-scroll")).Grow(1).Children(func() {
						ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
							for _, p := range snap.Participants {
								part := p
								ui.Row(c.Key("br-role-"+part.ID)).Radius(12).Background(insetBg).Border(1, hairline).
									Padding(10, 14).Gap(10).AlignItems(ui.Center).Children(func() {
									theme.RoleEmblemAvatar(c, part.Name, part.Handle, part.IsAgent, part.Presence, 28, th)
									ui.Column(c).Width(190).Gap(2).Children(func() {
										ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
											ui.Text(c, part.Name).Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(th.Ink).SingleLine()
											if part.IsAgent {
												theme.VerifiedAIBadge(c, th)
											} else if part.CanApproveGates {
												theme.ApproverBadge(c, th)
											}
										})
										ui.Text(c, "@"+part.Handle+" · "+part.HostNode).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).
											SingleLine().Ellipsis("…").MinWidth(0).Clip()
									})
									desktopPermChip(c, "Post", part.CanPost, th.Success, th)
									desktopPermChip(c, "Tasks", part.CanManageItems, th.Info, th)
									desktopPermChip(c, "Tools", part.CanInvokeTools, th.Accent, th)
									desktopPermChip(c, "Gate Req", part.RequireGate, th.Warning, th)
									ui.Spacer(c)
									if part.CanApproveGates {
										desktopPermChip(c, "HUMAN APPROVER", true, th.Success, th)
									}
								})
							}
						})
					})
				})

			default: // Edge-to-Edge Grouped Markdown Timeline
				ui.Column(c.Key("br-timeline-card")).Grow(1).MinHeight(0).Radius(16).Background(panelBg).Border(1, hairline).
					Padding(10, 0, 14, 0).Gap(8).AlignItems(ui.Stretch).Children(func() {
					ui.Scroll(c.Key("br-msg-scroll")).Grow(1).MinHeight(0).Children(func() {
						ui.Column(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
							for i, msg := range snap.Messages {
								item := msg
								grouped := i > 0 && desktopCanGroup(snap.Messages[i-1], item)
								mentionedMe := desktopMsgMentions(item, currentUserHandle)
								var rowBg, stripe ui.Color
								if mentionedMe {
									rowBg = kit.WithAlpha(th.Accent, 0.08)
									stripe = th.Accent
								} else if item.Pinned {
									rowBg = kit.WithAlpha(th.Warning, 0.05)
									stripe = th.Warning
								}
								topPad := float32(8)
								if grouped {
									topPad = 2
								}
								row := ui.Row(c.Key("br-msg-"+item.ID)).FillWidth().Padding(topPad, 18, 6, 18).Gap(10).AlignItems(ui.Start)
								if rowBg.A > 0 {
									row.Background(rowBg)
								}
								row.Children(func() {
									if stripe.A > 0 {
										ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(2.5).Background(stripe)
									}
									if grouped {
										ui.Box(c).Width(34).Shrink(0)
									} else {
										theme.RoleEmblemAvatar(c, item.AuthorName, item.AuthorHandle, item.IsAgent, item.AuthorPresence, 34, th)
									}
									ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(4).AlignItems(ui.Stretch).Children(func() {
										if !grouped {
											ui.Row(c).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
												nameCol := th.Ink
												if item.IsAgent {
													_, _, nameCol = theme.RoleVisualSpec(item.AuthorHandle, true, th)
												}
												ui.Text(c, item.AuthorName).Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(nameCol).SingleLine()
												if item.IsAgent {
													theme.VerifiedAIBadge(c, th)
												} else if part, ok := partByID[item.AuthorID]; ok && part.CanApproveGates {
													theme.ApproverBadge(c, th)
												}
												if item.AuthorRole != "" {
													ui.Text(c, item.AuthorRole).Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).
														SingleLine().Ellipsis("…").MinWidth(0).Clip()
												}
												ui.Spacer(c)
												if item.Pinned {
													ui.Text(c, "Pinned").Font(theme.FontMono).FontSize(10).FontWeight(600).TextColor(th.Warning).SingleLine()
												}
												if item.Timestamp != "" {
													ui.Text(c, item.Timestamp).Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).SingleLine()
												}
											})
										}
										if body := strings.TrimSpace(item.Body); body != "" {
											markdown.View(c.Key("br-md-"+item.ID), body, theme.CompactMarkdownOptions(th))
										}
										if item.GateID != "" {
											m.renderDesktopGateEmbed(c, th, item)
										}
										if len(item.Mentions) > 0 {
											ui.Row(c).Gap(5).Wrap().Children(func() {
												for _, h := range item.Mentions {
													_, _, tone := theme.RoleVisualSpec(h, strings.ToLower(h) != currentUserHandle, th)
													ui.Text(c, "@"+h).Font(theme.FontMono).FontSize(10.5).FontWeight(600).TextColor(tone).
														Padding(2, 7).Radius(theme.RadiusPill).Background(kit.WithAlpha(tone, 0.14)).SingleLine()
												}
											})
										}
										// Reactions + Thread + Promote-to-Task actions.
										ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
											for _, r := range item.Reactions {
												rx := r
												v := theme.BtnGhost
												tCol := th.InkMuted
												if rx.Reacted {
													v = theme.BtnTinted
													tCol = th.Accent
												}
												if theme.StyledButton(c, theme.ButtonSpec{
													Key: fmt.Sprintf("br-rx-%s-%s", item.ID, rx.Tag), Label: fmt.Sprintf("%s %d", rx.Tag, rx.Count),
													Variant: v, Size: theme.BtnSizeSM, Tone: tCol, Pill: true,
												}, th).Clicked() {
													toggleRxMsg = item.ID
													toggleRxTag = rx.Tag
												}
											}
											replyLbl := "Reply"
											if item.ThreadCount > 0 {
												replyLbl = fmt.Sprintf("%d replies", item.ThreadCount)
											}
											if theme.StyledButton(c, theme.ButtonSpec{
												Key: "br-th-" + item.ID, Label: replyLbl, Icon: "arrowshape.turn.up.left", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.Info, Pill: true,
											}, th).Clicked() {
												openThread = item.ID
											}
											if theme.StyledButton(c, theme.ButtonSpec{
												Key: "br-prom-" + item.ID, Label: "To Task", Icon: "checklist", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.InkMuted, Pill: true,
											}, th).Clicked() {
												promoteTitle = summarizeForTask(item.Body)
											}
										})
									})
								})
							}
						})
					})

					// Live Bot Thinking Indicator Bar.
					if len(snap.ThinkingBots) > 0 {
						ui.Row(c.Key("br-typing")).Padding(0, 18).Gap(7).AlignItems(ui.Center).Children(func() {
							ui.Spinner(c).Size(12, 12)
							ui.Text(c, fmt.Sprintf("%s is thinking…", strings.Join(snap.ThinkingBots, ", "))).
								Font(kit.FontDisplay).FontSize(12).TextColor(th.Accent).SingleLine()
						})
					}

					// Context-aware Desktop '/' Slash Command Popover & '@' Mention Autocomplete Popover
					// (1:1 parity with Mobile iOS/Android).
					mainMentionQ, mainTypedAt := ActiveMentionQuery(m.draft)
					mainSlashQ, mainSlashActive := ActiveSlashQuery(m.draft)
					showMainMention := (mainTypedAt || m.mentionForceOpen) && !mainSlashActive
					if mainSlashActive {
						if ins, execCmd := renderDesktopSlashPopover(c, "br-slash", th, panelBg, hairline, mainSlashQ); ins != "" || execCmd != "" {
							mainSlashInsert = ins
							mainSlashExec = execCmd
						}
					}
					if showMainMention {
						if picked := renderDesktopMentionPopover(c, "br-mpop", th, panelBg, hairline, snap.Participants, mainMentionQ); picked != "" {
							mainMentionPick = picked
						}
					}

					// Pure Apple 26 × Geist Liquid Glass Composer with '@' Mention trigger.
					ui.Row(c.Key("br-composer-wrap")).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
						atVar := theme.BtnGlass
						atTone := th.InkMuted
						if showMainMention {
							atVar = theme.BtnTinted
							atTone = th.Accent
						}
						if theme.IconCircleButton(c, "br-at-btn", "Mention agent", "at", 36, atVar, atTone, false, th).Clicked() {
							toggleMainAt = true
						}

						comp := ui.Row(c.Key("br-composer")).Basis(0).Grow(1).MinWidth(0).Height(42).Padding(4, 6, 4, 14).Gap(8).Radius(21).AlignItems(ui.Center)
						if c.Vibrancy() {
							comp.Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.Bar, 0.42)})
						} else {
							comp.Background(insetBg).Border(1, hairline)
						}
						comp.Children(func() {
							inp := ui.TextInputBase(c.Key(fmt.Sprintf("br-draft-%d", m.draftEpoch)), &m.draft).
								Label("Room message").
								Placeholder("Message " + prefix + activeCh.Name + " (@architect, @reviewer, /task …)").
								Font(kit.FontDisplay).FontSize(13.5).TextColor(th.Ink).Basis(0).Grow(1).MinWidth(0)
							if m.focusMainInput {
								inp.Focus()
								m.focusMainInput = false
							}
							if inp.Changed() {
								if _, active := ActiveMentionQuery(m.draft); !active {
									m.mentionForceOpen = false
								}
							}
							if inp.Submitted() {
								sendMsg = true
							}
							hasDraft := strings.TrimSpace(m.draft) != ""
							v := theme.BtnSolidIdle
							tone := th.InkMuted
							if hasDraft {
								v = theme.BtnGlassProminent
								tone = th.Accent
							}
							if theme.IconCircleButton(c, "br-send", "Send", theme.IconSend, 32, v, tone, false, th).Clicked() && hasDraft {
								sendMsg = true
							}
						})
					})
				})
			}
		})

		// Optional Right Thread Drawer Card (unclipped layout + dedicated '@' Mention Autocomplete).
		if snap.ActiveThreadID != "" {
			ui.Column(c.Key("br-thread-card")).Width(316).Shrink(0).MinHeight(0).Radius(16).Background(panelBg).Border(1, hairline).
				Padding(14).Gap(10).AlignItems(ui.Stretch).Clip().Children(func() {
				ui.Row(c).Shrink(0).MinWidth(0).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "THREAD REPLIES").Font(theme.FontMono).FontSize(10.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.Info).SingleLine()
					if theme.IconCircleButton(c, "br-th-close", "Close thread", theme.IconClose, 26, theme.BtnGhost, th.InkMuted, false, th).Clicked() {
						closeThread = true
					}
				})
				ui.Scroll(c.Key("br-th-scroll")).Grow(1).Basis(0).MinHeight(0).Children(func() {
					ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
						// Render root message at top for clear thread context (matching iOS 1:1).
						for _, rootMsg := range snap.Messages {
							if rootMsg.ID == snap.ActiveThreadID {
								rm := rootMsg
								ui.Column(c.Key("br-th-root-"+rm.ID)).MinWidth(0).Radius(12).Background(kit.WithAlpha(th.Accent, 0.08)).
									Border(1, kit.WithAlpha(th.Accent, 0.22)).Padding(10, 12).Gap(4).Clip().Children(func() {
									ui.Row(c).MinWidth(0).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
										theme.RoleEmblemAvatar(c, rm.AuthorName, rm.AuthorHandle, rm.IsAgent, rm.AuthorPresence, 20, th)
										ui.Text(c, rm.AuthorName).Font(kit.FontDisplay).FontSize(12.5).FontWeight(700).TextColor(th.Ink).
											Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
										ui.Text(c, "Root").Font(theme.FontMono).FontSize(9.5).FontWeight(600).TextColor(th.Accent).SingleLine().Shrink(0)
									})
									markdown.View(c.Key("br-th-root-md-"+rm.ID), strings.TrimSpace(rm.Body), theme.CompactMarkdownOptions(th))
								})
								break
							}
						}
						if len(snap.ThreadMessages) == 0 {
							ui.Text(c, "No replies yet. Write below or mention @architect to reply in this thread.").
								Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted).Padding(6, 4)
						}
						for _, rep := range snap.ThreadMessages {
							r := rep
							ui.Column(c.Key("br-rep-"+r.ID)).MinWidth(0).Radius(12).Background(insetBg).Padding(10, 12).Gap(4).Clip().Children(func() {
								ui.Row(c).MinWidth(0).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
									theme.RoleEmblemAvatar(c, r.AuthorName, r.AuthorHandle, r.IsAgent, r.AuthorPresence, 20, th)
									ui.Text(c, r.AuthorName).Font(kit.FontDisplay).FontSize(12.5).FontWeight(700).TextColor(th.Accent).
										Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
									if r.Timestamp != "" {
										ui.Text(c, r.Timestamp).Font(theme.FontMono).FontSize(10).TextColor(th.InkMuted).SingleLine().Shrink(0)
									}
								})
								markdown.View(c.Key("br-rep-md-"+r.ID), strings.TrimSpace(r.Body), theme.CompactMarkdownOptions(th))
							})
						}
						if len(snap.ThinkingBots) > 0 {
							ui.Row(c.Key("br-th-typing")).Padding(4, 6).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Spinner(c).Size(11, 11)
								ui.Text(c, fmt.Sprintf("%s replying in thread…", strings.Join(snap.ThinkingBots, ", "))).
									Font(kit.FontDisplay).FontSize(11.5).TextColor(th.Accent).SingleLine().Ellipsis("…").MinWidth(0).Clip()
							})
						}
					})
				})

				// Thread-scoped '@' Mention Autocomplete Popover.
				thMentionQ, thTypedAt := ActiveMentionQuery(m.threadDraft)
				showThMention := thTypedAt || m.threadMentionForceOpen
				if showThMention {
					if picked := renderDesktopMentionPopover(c, "br-th-mpop", th, panelBg, hairline, snap.Participants, thMentionQ); picked != "" {
						thMentionPick = picked
					}
				}

				ui.Row(c.Key("br-th-composer")).Height(40).Shrink(0).MinWidth(0).Radius(20).Background(insetBg).Border(1, hairline).
					Padding(4, 6, 4, 6).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
					thAtVar := theme.BtnGhost
					thAtTone := th.InkMuted
					if showThMention {
						thAtVar = theme.BtnTinted
						thAtTone = th.Accent
					}
					if theme.IconCircleButton(c, "br-th-at", "Mention in thread", "at", 28, thAtVar, thAtTone, false, th).Clicked() {
						toggleThAt = true
					}
					inp := ui.TextInputBase(c.Key(fmt.Sprintf("br-th-draft-%d", m.threadDraftEpoch)), &m.threadDraft).
						Label("Thread reply").Placeholder("Reply in thread (@architect)…").
						Font(kit.FontDisplay).FontSize(12.5).TextColor(th.Ink).Height(28).Basis(0).Grow(1).MinWidth(0)
					if m.focusThreadInput {
						inp.Focus()
						m.focusThreadInput = false
					}
					if inp.Changed() {
						if _, active := ActiveMentionQuery(m.threadDraft); !active {
							m.threadMentionForceOpen = false
						}
					}
					if inp.Submitted() {
						sendThread = true
					}
					canTh := strings.TrimSpace(m.threadDraft) != ""
					tv := theme.BtnSolidIdle
					tone := th.InkMuted
					if canTh {
						tv = theme.BtnGlassProminent
						tone = th.Accent
					}
					if theme.IconCircleButton(c, "br-th-send", "Send thread reply", theme.IconSend, 28, tv, tone, false, th).Clicked() && canTh {
						sendThread = true
					}
				})
			})
		}
	})

	// Commit all controller mutations cleanly outside layout closures.
	if selectCh != "" {
		m.mentionForceOpen = false
		m.threadMentionForceOpen = false
		_ = SelectAgentRoomChannel(selectCh)
	}
	if toggleMainAt {
		m.mentionForceOpen = !m.mentionForceOpen
		m.focusMainInput = true
		NotifyRoomChanged()
	}
	if toggleThAt {
		m.threadMentionForceOpen = !m.threadMentionForceOpen
		m.focusThreadInput = true
		NotifyRoomChanged()
	}
	if mainMentionPick != "" {
		m.draft = ApplyMentionCompletion(m.draft, mainMentionPick)
		m.draftEpoch++
		m.mentionForceOpen = false
		m.focusMainInput = true
		NotifyRoomChanged()
	}
	if thMentionPick != "" {
		m.threadDraft = ApplyMentionCompletion(m.threadDraft, thMentionPick)
		m.threadDraftEpoch++
		m.threadMentionForceOpen = false
		m.focusThreadInput = true
		NotifyRoomChanged()
	}
	if mainSlashInsert != "" {
		m.draft = mainSlashInsert
		m.draftEpoch++
		m.focusMainInput = true
		NotifyRoomChanged()
	}
	if mainSlashExec == "/clear" {
		m.draft = ""
		m.draftEpoch++
		m.mentionForceOpen = false
		NotifyRoomChanged()
	} else if mainSlashExec == "/pin" {
		m.draft = ""
		m.draftEpoch++
		m.activeTab = chatroom.TabTimeline
		NotifyRoomChanged()
	}
	if mention != "" {
		// When a Thread Drawer is open, clicking a roster member targets the
		// active Thread Composer so thread-scoped @mentions stay in-thread.
		if snap.ActiveThreadID != "" {
			m.threadDraft = ApplyMentionCompletion(m.threadDraft, mention)
			m.threadDraftEpoch++
			m.threadMentionForceOpen = false
			m.focusThreadInput = true
		} else {
			m.draft = ApplyMentionCompletion(m.draft, mention)
			m.draftEpoch++
			m.mentionForceOpen = false
			m.focusMainInput = true
		}
		NotifyRoomChanged()
	}
	if sendMsg && strings.TrimSpace(m.draft) != "" {
		txt := strings.TrimSpace(m.draft)
		lower := strings.ToLower(txt)
		m.mentionForceOpen = false
		if strings.HasPrefix(lower, "/task ") {
			if title := strings.TrimSpace(txt[len("/task "):]); title != "" {
				_ = AddAgentRoomTask(title)
			}
			m.draft = ""
		} else if lower == "/clear" {
			m.draft = ""
		} else {
			if strings.HasPrefix(lower, "/ask ") {
				txt = strings.TrimSpace(txt[len("/ask "):])
			}
			if txt != "" && PostAgentRoomMessage(snap.ActiveChannelID, txt, "") == nil {
				m.draft = ""
			}
		}
	}
	if openThread != "" {
		m.focusThreadInput = true
		m.threadMentionForceOpen = false
		_ = OpenAgentRoomThread(openThread)
	}
	if closeThread {
		m.focusThreadInput = false
		m.threadMentionForceOpen = false
		CloseAgentRoomThread()
	}
	if sendThread && snap.ActiveThreadID != "" && strings.TrimSpace(m.threadDraft) != "" {
		m.threadMentionForceOpen = false
		if PostAgentRoomMessage(snap.ActiveChannelID, m.threadDraft, snap.ActiveThreadID) == nil {
			m.threadDraft = ""
		}
	}
	if addItem && strings.TrimSpace(m.itemDraft) != "" {
		if AddAgentRoomTask(strings.TrimSpace(m.itemDraft)) == nil {
			m.itemDraft = ""
		}
	}
	if promoteTitle != "" {
		_ = AddAgentRoomTask(promoteTitle)
	}
	if cycleTaskID != "" {
		_ = CycleAgentRoomTask(cycleTaskID)
	}
	if toggleRxMsg != "" {
		_ = ToggleAgentRoomReaction(toggleRxMsg, toggleRxTag)
	}
}

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
