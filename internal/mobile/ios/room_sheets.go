// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package ios

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/mobile"

	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// renderRoomChannelsSheet presents the Discord-style dual-tier channel drawer:
// CHANNELS (#workspace, #architecture, #governance) and AI BOTS · DIRECT
// (@architect, @reviewer, @security 1:1 direct channels).
func (s *Shell) renderRoomChannelsSheet(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	partByID map[string]featurekit.RoomParticipantView,
	open *bool,
) {
	mobile.Sheet(c, open, mobile.SheetOptions{Detents: []float32{0.62, 0.90}, Label: "Channels"}, func() {
		ui.Scroll(c.Key("ios-room-chan-scroll")).Grow(1).Basis(0).Padding(6, 0, 24, 0).Children(func() {
			ui.Column(c).Gap(16).AlignItems(ui.Stretch).Children(func() {
				mobile.Section(c, "CHANNELS", func() {
					for _, ch := range snap.Channels {
						if ch.IsDM {
							continue
						}
						item := ch
						sel := item.ID == snap.ActiveChannelID
						if mobile.ListRow(c.Key("ios-rch-"+item.ID), mobile.RowOptions{
							Title:    "#" + item.Name,
							Subtitle: item.Topic,
							Leading: func() {
								col := th.InkMuted
								if sel {
									col = th.Accent
								}
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									if sel {
										ui.Box(c).Width(3).Height(18).Radius(2).Background(th.Accent)
									}
									sfIcon(c, "number", 15, col, sel)
								})
							},
							Trailing: func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									if item.UnreadCount > 0 {
										ui.Badge(c, fmt.Sprintf("%d", item.UnreadCount))
									}
									if sel {
										sfIcon(c, "checkmark.circle", 17, th.Accent, true)
									}
								})
							},
						}).Clicked() {
							mygo.Haptics.Selection()
							_ = featurekit.SelectAgentRoomChannel(item.ID)
							*open = false
							s.invalidate()
						}
					}
				})

				mobile.Section(c, "AI BOTS · DIRECT", func() {
					for _, ch := range snap.Channels {
						if !ch.IsDM {
							continue
						}
						item := ch
						sel := item.ID == snap.ActiveChannelID
						bot := partByID[item.BotParticipantID]
						if mobile.ListRow(c.Key("ios-rdm-"+item.ID), mobile.RowOptions{
							Title:    item.Name,
							Subtitle: "@" + bot.Handle + " · " + item.Topic,
							Leading: func() {
								renderRoleEmblemAvatar(c, bot.Name, bot.Handle, true, bot.Presence, 30, th)
							},
							Trailing: func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, presenceLabel(bot.Presence)).Font(theme.FontMono).FontSize(10).
										TextColor(presenceColor(bot.Presence, th)).SingleLine()
									if sel {
										sfIcon(c, "checkmark.circle", 17, th.Accent, true)
									}
								})
							},
						}).Clicked() {
							mygo.Haptics.Selection()
							_ = featurekit.SelectAgentRoomChannel(item.ID)
							*open = false
							s.invalidate()
						}
					}
				})
			})
		})
	})
}

// renderRoomDetailsSheet presents the Discord Mobile Channel Details Sheet with
// Segmented navigation across Members (HUMANS vs AI AGENTS), Tasks, Pinned, and Roles.
func (s *Shell) renderRoomDetailsSheet(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	activeChan featurekit.RoomChannelView,
	partByID map[string]featurekit.RoomParticipantView,
	currentUserHandle string,
	open *bool,
	tabIdx *int,
	taskDraft *string,
) {
	mobile.Sheet(c, open, mobile.SheetOptions{Detents: []float32{0.68, 0.92}, Label: "Channel Details"}, func() {
		ui.Column(c.Key("ios-room-det-col")).Fill().Padding(6, 14, 16, 14).Gap(10).AlignItems(ui.Stretch).Children(func() {
			titlePrefix := "#"
			if activeChan.IsDM {
				titlePrefix = "@"
			}
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
				ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, titlePrefix+activeChan.Name).Font(kit.FontDisplay).FontSize(17).FontWeight(700).TextColor(th.Ink).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
					if activeChan.Topic != "" {
						ui.Text(c, activeChan.Topic).Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
					}
				})
				if theme.IconCircleButton(c, "ios-room-det-close", "Close details", theme.IconClose, 28, theme.BtnGlass, th.InkMuted, false, th).Clicked() {
					*open = false
				}
			})

			seg := ui.Segmented(c.Key("ios-room-det-seg"), tabIdx, "Members", "Tasks", "Pinned", "Roles").Label("Channel details")
			if seg.Changed() {
				mygo.Haptics.Selection()
				s.invalidate()
			}

			ui.Scroll(c.Key("ios-room-det-scroll")).Grow(1).Basis(0).Padding(4, 0, 14, 0).Children(func() {
				switch *tabIdx {
				case 1:
					s.renderDetailsTasksTab(c, th, snap, taskDraft)
				case 2:
					s.renderDetailsPinnedTab(c, th, snap, partByID, currentUserHandle)
				case 3:
					s.renderDetailsRolesTab(c, th, snap)
				default:
					s.renderDetailsMembersTab(c, th, snap, open)
				}
			})
		})
	})
}

func (s *Shell) renderDetailsMembersTab(c *ui.Context, th theme.Theme, snap featurekit.RoomFullSnapshot, sheetOpen *bool) {
	var humans, agents []featurekit.RoomParticipantView
	for _, p := range snap.Participants {
		if p.IsAgent {
			agents = append(agents, p)
		} else {
			humans = append(humans, p)
		}
	}

	ui.Column(c).Gap(14).AlignItems(ui.Stretch).Children(func() {
		mobile.Section(c, fmt.Sprintf("HUMANS · %d", len(humans)), func() {
			for _, h := range humans {
				mem := h
				sub := "@" + mem.Handle + " · " + mem.Role
				if mem.HostNode != "" {
					sub += " · " + mem.HostNode
				}
				if mobile.ListRow(c.Key("ios-rmem-h-"+mem.ID), mobile.RowOptions{
					Title:    mem.Name,
					Subtitle: sub,
					Leading: func() {
						renderRoleEmblemAvatar(c, mem.Name, mem.Handle, false, mem.Presence, 32, th)
					},
					Trailing: func() {
						if mem.CanApproveGates {
							renderApproverBadge(c, th)
						}
					},
				}).Clicked() {
					mygo.Haptics.Light()
					s.mu.Lock()
					s.roomDraft = applyMentionCompletion(s.roomDraft, mem.Handle)
					s.mu.Unlock()
					*sheetOpen = false
					s.invalidate()
				}
			}
		})

		mobile.Section(c, fmt.Sprintf("AI AGENTS · %d", len(agents)), func() {
			for _, a := range agents {
				mem := a
				sub := "@" + mem.Handle + " · " + mem.Role
				if mem.HostNode != "" {
					sub += " · " + mem.HostNode
				}
				if mobile.ListRow(c.Key("ios-rmem-a-"+mem.ID), mobile.RowOptions{
					Title:    mem.Name,
					Subtitle: sub,
					Leading: func() {
						renderRoleEmblemAvatar(c, mem.Name, mem.Handle, true, mem.Presence, 32, th)
					},
					Trailing: func() {
						ui.Column(c).Gap(3).AlignItems(ui.End).Children(func() {
							renderVerifiedAIBadge(c, th)
							ui.Text(c, presenceLabel(mem.Presence)).Font(theme.FontMono).FontSize(9.5).
								TextColor(presenceColor(mem.Presence, th)).SingleLine()
						})
					},
				}).Clicked() {
					mygo.Haptics.Light()
					s.mu.Lock()
					s.roomDraft = applyMentionCompletion(s.roomDraft, mem.Handle)
					s.mu.Unlock()
					*sheetOpen = false
					s.invalidate()
				}
			}
		})
	})
}

func (s *Shell) renderDetailsTasksTab(c *ui.Context, th theme.Theme, snap featurekit.RoomFullSnapshot, taskDraft *string) {
	ui.Column(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		// Quick-add shared channel task bar.
		ui.Row(c.Key("ios-rtask-addbar")).Padding(6, 8, 6, 12).Gap(8).Radius(18).AlignItems(ui.Center).
			Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.36)}).Children(func() {
			inp := ui.TextInputBase(c.Key("ios-rtask-in"), taskDraft).
				Placeholder("Add shared channel task…").Font(kit.FontDisplay).FontSize(13.5).TextColor(th.Ink).Grow(1).MinWidth(0)
			if inp.Changed() {
				s.mu.Lock()
				s.roomTaskDraft = *taskDraft
				s.mu.Unlock()
			}
			addFn := func() {
				title := strings.TrimSpace(*taskDraft)
				if title == "" {
					return
				}
				mygo.Haptics.Medium()
				_ = featurekit.AddAgentRoomTask(title)
				*taskDraft = ""
				s.mu.Lock()
				s.roomTaskDraft = ""
				s.mu.Unlock()
				s.invalidate()
			}
			if inp.Submitted() {
				addFn()
			}
			canAdd := strings.TrimSpace(*taskDraft) != ""
			v := theme.BtnSolidIdle
			if canAdd {
				v = theme.BtnGlassProminent
			}
			if theme.IconCircleButton(c, "ios-rtask-add-btn", "Add task", "plus", 30, v, th.Accent, false, th).Clicked() && canAdd {
				addFn()
			}
		})

		mobile.Section(c, "CHANNEL TASKS", func() {
			if len(snap.Tasks) == 0 {
				mobile.ListRow(c.Key("ios-rtask-empty"), mobile.RowOptions{
					Title:    "No active channel tasks",
					Subtitle: "Tasks sync live across Desktop & iOS.",
				})
				return
			}
			for _, tk := range snap.Tasks {
				item := tk
				done := item.Status == "DONE"
				col := th.Info
				sym := "circle"
				if done {
					col = th.Success
					sym = "checkmark.circle"
				} else if item.Status == "IN_PROGRESS" {
					col = th.Warning
					sym = "clock"
				}
				if mobile.ListRow(c.Key("ios-rtask-"+item.ID), mobile.RowOptions{
					Title:    item.Title,
					Subtitle: "@" + strings.TrimPrefix(item.Owner, "@") + " · " + item.Status,
					Leading: func() {
						sfIcon(c, sym, 18, col, done)
					},
					Trailing: func() {
						ui.Badge(c, item.Status)
					},
				}).Clicked() {
					mygo.Haptics.Selection()
					_ = featurekit.CycleAgentRoomTask(item.ID)
					s.invalidate()
				}
			}
		})
	})
}

func (s *Shell) renderDetailsPinnedTab(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	partByID map[string]featurekit.RoomParticipantView,
	currentUserHandle string,
) {
	if len(snap.PinnedMessages) == 0 {
		ui.Column(c).Padding(24, 14).Gap(6).AlignItems(ui.Center).Children(func() {
			sfIcon(c, "pin", 24, th.InkMuted, false)
			ui.Text(c, "No pinned messages").Font(kit.FontDisplay).FontSize(14).FontWeight(600).TextColor(th.Ink)
			ui.Text(c, "Key architectural decisions and gate receipts pinned in this channel appear here.").
				Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted)
		})
		return
	}
	ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
		for _, pm := range snap.PinnedMessages {
			s.renderRoomStreamMessage(c, th, pm, partByID, currentUserHandle, false, true, true)
		}
	})
}

func (s *Shell) renderDetailsRolesTab(c *ui.Context, th theme.Theme, snap featurekit.RoomFullSnapshot) {
	ui.Column(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Padding(10, 12).Gap(8).Radius(14).AlignItems(ui.Center).
			Background(kit.WithAlpha(th.Warning, 0.12)).
			Border(1, kit.WithAlpha(th.Warning, 0.28)).Children(func() {
			sfIcon(c, "shield.lefthalf.filled", 15, th.Warning, true)
			ui.Text(c, "Tier-0 Invariant: PromptGate approval is exclusively human-owned.").
				Font(kit.FontDisplay).FontSize(12).FontWeight(600).TextColor(th.Ink).Basis(0).Grow(1).MinWidth(0)
		})

		for _, p := range snap.Participants {
			mem := p
			ui.Column(c.Key("ios-rrole-"+mem.ID)).Padding(10, 12).Gap(7).Radius(14).AlignItems(ui.Stretch).
				Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.72)).
				Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.08)).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					renderRoleEmblemAvatar(c, mem.Name, mem.Handle, mem.IsAgent, mem.Presence, 28, th)
					ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							ui.Text(c, mem.Name).Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(th.Ink).SingleLine()
							if mem.IsAgent {
								renderVerifiedAIBadge(c, th)
							} else if mem.CanApproveGates {
								renderApproverBadge(c, th)
							}
						})
						ui.Text(c, fmt.Sprintf("@%s · %s · %s", mem.Handle, mem.Role, mem.HostNode)).
							Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
				})
				ui.Row(c).Gap(5).Wrap().Children(func() {
					renderPermChip(c, "Post", mem.CanPost, th.Success, th)
					renderPermChip(c, "Tasks", mem.CanManageItems, th.Info, th)
					renderPermChip(c, "Tools", mem.CanInvokeTools, th.Accent, th)
					renderPermChip(c, "Gate Required", mem.RequireGate, th.Warning, th)
					if mem.CanApproveGates {
						renderPermChip(c, "HUMAN APPROVER", true, th.Success, th)
					}
				})
			})
		}
	})
}

func renderPermChip(c *ui.Context, label string, enabled bool, tone ui.Color, th theme.Theme) {
	col := th.InkMuted
	bg := kit.WithAlpha(th.BorderSubtle, 0.28)
	if enabled {
		col = tone
		bg = kit.WithAlpha(tone, 0.16)
	}
	ui.Text(c, label).Font(theme.FontMono).FontSize(9.5).FontWeight(600).TextColor(col).
		Padding(2, 7).Radius(theme.RadiusPill).Background(bg).SingleLine()
}

func (s *Shell) renderRoomThreadSheet(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	partByID map[string]featurekit.RoomParticipantView,
	currentUserHandle string,
	threadOpen *bool,
	threadDraft *string,
) {
	mobile.Sheet(c, threadOpen, mobile.SheetOptions{Detents: []float32{0.64, 0.90}, Label: "Thread"}, func() {
		ui.Column(c.Key("ios-room-thread-col")).Fill().Padding(6, 0, 16, 0).Gap(8).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Padding(0, 14).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, "Thread Replies").Font(kit.FontDisplay).FontSize(16).FontWeight(700).TextColor(th.Ink)
				if theme.IconCircleButton(c, "ios-room-thread-close", "Close thread", theme.IconClose, 28, theme.BtnGlass, th.InkMuted, false, th).Clicked() {
					*threadOpen = false
				}
			})
			ui.Scroll(c.Key("ios-room-thread-scroll")).Grow(1).Basis(0).Children(func() {
				ui.Column(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
					// Render root message at top for clear thread context.
					for _, rootMsg := range snap.Messages {
						if rootMsg.ID == snap.ActiveThreadID {
							s.renderRoomStreamMessage(c, th, rootMsg, partByID, currentUserHandle, false, false, true)
							ui.Box(c).Height(1).FillWidth().Background(kit.WithAlpha(th.BorderSubtle, 0.6))
							break
						}
					}
					if len(snap.ThreadMessages) == 0 {
						ui.Text(c, "No replies yet. Mention an @agent below to start a sub-thread.").
							Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted).Padding(12, 16)
					}
					for i, tm := range snap.ThreadMessages {
						grouped := i > 0 && canGroupRoomMessages(snap.ThreadMessages[i-1], tm)
						nextGrouped := i+1 < len(snap.ThreadMessages) && canGroupRoomMessages(tm, snap.ThreadMessages[i+1])
						s.renderRoomStreamMessage(c, th, tm, partByID, currentUserHandle, grouped, !nextGrouped, true)
					}
				})
			})
			ui.Row(c.Key("ios-room-thread-comp")).Padding(0, 14).Children(func() {
				ui.Row(c).Grow(1).Padding(6, 8, 6, 12).Gap(8).Radius(22).AlignItems(ui.Center).
					Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.42)}).Children(func() {
					inp := ui.TextInputBase(c.Key("ios-room-th-in"), threadDraft).
						Placeholder("Reply in thread…").Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
					sendThread := func() {
						txt := strings.TrimSpace(*threadDraft)
						if txt == "" {
							return
						}
						mygo.Haptics.Medium()
						_ = featurekit.PostAgentRoomMessage(snap.ActiveChannelID, txt, snap.ActiveThreadID)
						*threadDraft = ""
						s.invalidate()
					}
					if inp.Submitted() {
						sendThread()
					}
					canSend := strings.TrimSpace(*threadDraft) != ""
					varType := theme.BtnSolidIdle
					if canSend {
						varType = theme.BtnGlassProminent
					}
					if theme.IconCircleButton(c, "ios-room-th-send", "Reply", theme.IconSend, 32, varType, th.Accent, false, th).Clicked() && canSend {
						sendThread()
					}
				})
			})
		})
	})
}

func roleVisualSpec(handle string, isAgent bool, th theme.Theme) (roleTag, symbol string, tone ui.Color) {
	return theme.RoleVisualSpec(handle, isAgent, th)
}

func renderRoleEmblemAvatar(c *ui.Context, name, handle string, isAgent bool, presence string, sz float32, th theme.Theme) {
	theme.RoleEmblemAvatar(c, name, handle, isAgent, presence, sz, th)
}

func renderVerifiedAIBadge(c *ui.Context, th theme.Theme) {
	theme.VerifiedAIBadge(c, th)
}

func renderApproverBadge(c *ui.Context, th theme.Theme) {
	theme.ApproverBadge(c, th)
}

func presenceColor(pres string, th theme.Theme) ui.Color {
	return theme.PresenceColor(pres, th)
}

func presenceLabel(pres string) string {
	return theme.PresenceLabel(pres)
}

func roomMarkdownOpts(th theme.Theme) *markdown.Options {
	return theme.CompactMarkdownOptions(th)
}
