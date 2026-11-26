// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && (marlin_feat_chatroom || marlin_full || (!marlin_feat_compare && !marlin_feat_canvas))

package featurekit

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/features/chatroom"
	"github.com/tao3k/marlin-mygo/runtime/kit"
)

// renderBentoAgentRoom paints Marlin's Multi-Role Agent Room using the unified
// Nord-Bento matte card aesthetic (soft 14px elevated cards on deep canvas,
// zero harsh grid borders, pastel presence/status pills, and live task syncing
// back to the *Dashboard* Bento widgets).
func (m *bentoChatRoomModule) renderBentoAgentRoom(c *ui.Context, t *ui.Theme) {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()

	canvas := t.Background
	cardBg := t.SurfacePressed // maps to Theme.SurfaceElevated (#2b303c in nord-bento)
	insetBg := t.Surface       // maps to Theme.Bar (#191d24)
	hoverBg := t.SurfaceHover
	if c.Vibrancy() {
		canvas = kit.WithAlpha(canvas, 0.14)
		cardBg = kit.WithAlpha(cardBg, 0.40)
		insetBg = kit.WithAlpha(insetBg, 0.34)
		hoverBg = kit.WithAlpha(hoverBg, 0.44)
	}
	ink := t.Text
	muted := t.TextMuted
	accent := t.Accent
	info := t.AccentHover
	success := t.Success
	warning := t.Warning
	danger := t.Danger
	font := t.Font
	const cardRadius float32 = 14

	partsByID := make(map[string]chatroom.Participant, len(snap.Participants))
	for _, p := range snap.Participants {
		partsByID[p.ID] = p
	}
	var activeCh chatroom.Channel
	for _, ch := range snap.Channels {
		if ch.ID == snap.ActiveChannelID {
			activeCh = ch
			break
		}
	}
	doneCount := 0
	for _, it := range snap.Items {
		if it.Status == chatroom.ItemDone {
			doneCount++
		}
	}

	var (
		selectCh     string
		mention      string
		sendMsg      bool
		openThread   string
		closeThread  bool
		sendThread   bool
		addItem      bool
		nextItemID   string
		nextItemStat chatroom.ItemStatus
	)

	ui.Row(c.Key("bento-room-stage")).Fill().Background(canvas).Padding(18, 22).Gap(14).
		AlignItems(ui.Stretch).Children(func() {
		// Left Bento Column: Channels card + Live Agent Roster card.
		ui.Column(c.Key("bento-room-rail")).Width(236).Gap(14).AlignItems(ui.Stretch).Children(func() {
			// Channels Bento card.
			ui.Column(c.Key("bento-room-channels")).Radius(cardRadius).Background(cardBg).
				Padding(14, 14).Gap(6).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(0, 4, 4, 4).Children(func() {
					ui.Text(c, "CHANNELS").Font(font).FontSize(11).FontWeight(700).TextColor(muted).NoWrap()
					ui.Text(c, fmt.Sprintf("%d", len(snap.Channels))).Font(font).FontSize(11).FontWeight(700).TextColor(info).NoWrap()
				})
				for _, ch := range snap.Channels {
					item := ch
					active := item.ID == snap.ActiveChannelID
					btn := kit.ActionControl(ui.ButtonBase(c.Key("br-ch-"+item.ID)).Label(item.Name).
						Height(32).Padding(0, 10).Radius(8).Gap(8).AlignItems(ui.Center).Justify(ui.Start))
					bg := ui.Transparent
					fg := muted
					w := 500
					if active {
						bg = hoverBg
						fg = ink
						w = 700
					} else if btn.Hovered() {
						bg = insetBg
						fg = ink
					}
					btn.Background(bg)
					btn.Children(func() {
						prefix := "#"
						if item.IsDM {
							prefix = "@"
						}
						ui.Text(c, prefix).Font(font).FontSize(13).FontWeight(700).TextColor(accent).NoWrap()
						ui.Text(c, strings.TrimPrefix(strings.TrimPrefix(item.Name, "#"), "@")).
							Font(font).FontSize(13).FontWeight(w).TextColor(fg).Grow(1).NoWrap().MaxLines(1)
					})
					if btn.Clicked() {
						selectCh = item.ID
					}
				}
			})

			// Teammates / AI Agents Bento card.
			ui.Column(c.Key("bento-room-agents")).Grow(1).MinHeight(0).Radius(cardRadius).Background(cardBg).
				Padding(14, 14).Gap(6).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(0, 4, 4, 4).Children(func() {
					ui.Text(c, "AGENTS & TEAM").Font(font).FontSize(11).FontWeight(700).TextColor(muted).NoWrap()
					ui.Text(c, fmt.Sprintf("%d/%d tasks", doneCount, len(snap.Items))).
						Font(font).FontSize(11).FontWeight(700).TextColor(success).NoWrap()
				})
				ui.Scroll(c.Key("br-agents-scroll")).Grow(1).Children(func() {
					ui.Column(c).Gap(4).AlignItems(ui.Stretch).Children(func() {
						for _, p := range snap.Participants {
							part := p
							dot := success
							switch part.Presence {
							case chatroom.PresenceWorking, chatroom.PresenceThinking:
								dot = info
							case chatroom.PresenceWaitingGate:
								dot = warning
							case chatroom.PresenceOffline:
								dot = muted
							}
							row := kit.ActionControl(ui.ButtonBase(c.Key("br-p-"+part.ID)).Label(part.Handle).
								Height(34).Padding(0, 8).Radius(8).Gap(8).AlignItems(ui.Center).Justify(ui.Start))
							if row.Hovered() {
								row.Background(hoverBg)
							}
							row.Children(func() {
								ui.Box(c).Width(8).Height(8).Radius(999).Background(dot)
								ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
									ui.Text(c, part.DisplayName).Font(font).FontSize(12.5).FontWeight(600).TextColor(ink).NoWrap().MaxLines(1)
									ui.Text(c, part.Handle+" · "+part.RoleTitle).Font(font).FontSize(10.5).TextColor(muted).NoWrap().MaxLines(1)
								})
							})
							if row.Clicked() {
								mention = part.Handle
							}
						}
					})
				})
			})
		})

		// Main Center Bento Stage: Header card + active surface card (Timeline / Matters / Roles).
		ui.Column(c.Key("bento-room-main")).Grow(1).MinWidth(0).Gap(14).AlignItems(ui.Stretch).Children(func() {
			// Sleek matte Bento header bar.
			ui.Row(c.Key("bento-room-header")).Height(56).Radius(cardRadius).Background(cardBg).
				Padding(0, 18).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Padding(4, 10).Radius(999).Background(insetBg).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Width(7).Height(7).Radius(999).Background(accent)
					ui.Text(c, "AGENT ROOM").Font(font).FontSize(11).FontWeight(700).TextColor(accent).NoWrap()
				})
				ui.Text(c, activeCh.Name).Font(font).FontSize(15).FontWeight(700).TextColor(ink).NoWrap()
				if activeCh.Topic != "" {
					ui.Text(c, "· "+activeCh.Topic).Font(font).FontSize(12).TextColor(muted).Grow(1).NoWrap().MaxLines(1)
				} else {
					ui.Spacer(c)
				}
				if g := ctrl.PendingGateCount(); g > 0 {
					ui.Row(c).Padding(4, 10).Radius(999).Background(insetBg).Children(func() {
						ui.Text(c, fmt.Sprintf("%d gate", g)).Font(font).FontSize(11.5).FontWeight(700).TextColor(warning).NoWrap()
					})
				}

				// Clean pill mode switchers.
				tabs := []struct {
					tab   chatroom.ViewTab
					label string
				}{
					{chatroom.TabTimeline, "Timeline"},
					{chatroom.TabMatters, fmt.Sprintf("Tasks (%d/%d)", doneCount, len(snap.Items))},
					{chatroom.TabRoles, "Roles"},
				}
				ui.Row(c).Padding(3, 4).Radius(10).Background(insetBg).Gap(4).AlignItems(ui.Center).Children(func() {
					for _, tb := range tabs {
						item := tb
						active := m.activeTab == item.tab
						b := kit.ActionControl(ui.ButtonBase(c.Key("br-tab-"+string(item.tab))).Label(item.label).
							Padding(5, 11).Radius(8).AlignItems(ui.Center).Justify(ui.Center))
						fg := muted
						w := 500
						if active {
							b.Background(cardBg)
							fg = info
							w = 700
						} else if b.Hovered() {
							b.Background(hoverBg)
							fg = ink
						}
						b.Children(func() {
							ui.Text(c, item.label).Font(font).FontSize(12).FontWeight(w).TextColor(fg).NoWrap()
						})
						if b.Clicked() {
							m.activeTab = item.tab
						}
					}
				})
			})

			if snap.LastError != "" {
				ui.Row(c).Radius(10).Background(cardBg).Padding(8, 14).Children(func() {
					ui.Text(c, snap.LastError).Font(font).FontSize(12).FontWeight(600).TextColor(danger)
				})
			}

			// Active Bento workspace card.
			switch m.activeTab {
			case chatroom.TabMatters:
				ui.Column(c.Key("br-matters-card")).Grow(1).MinHeight(0).Radius(cardRadius).Background(cardBg).
					Padding(16, 18).Gap(12).AlignItems(ui.Stretch).Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Grow(1).Padding(7, 12).Radius(10).Background(insetBg).AlignItems(ui.Center).Children(func() {
							inp := ui.TextInputBase(c.Key("br-new-matter"), &m.itemDraft).
								Placeholder("Add collaborative task / matter...").Font(font).FontSize(13).TextColor(ink).Grow(1)
							if inp.Submitted() {
								addItem = true
							}
						})
						btn := ui.ButtonBase(c.Key("br-add-matter")).Label("Add Task").
							Padding(7, 14).Radius(10).Background(info).AlignItems(ui.Center)
						btn.Children(func() {
							ui.Text(c, "+ Add Task").Font(font).FontSize(12.5).FontWeight(700).TextColor(canvas).NoWrap()
						})
						if btn.Clicked() {
							addItem = true
						}
					})
					ui.Scroll(c.Key("br-matters-scroll")).Grow(1).Children(func() {
						ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
							for _, it := range snap.Items {
								item := it
								pillCol := info
								nextSt := chatroom.ItemInProgress
								switch item.Status {
								case chatroom.ItemInProgress:
									pillCol = warning
									nextSt = chatroom.ItemDone
								case chatroom.ItemDone:
									pillCol = success
									nextSt = chatroom.ItemOpen
								case chatroom.ItemBlocked:
									pillCol = danger
									nextSt = chatroom.ItemInProgress
								}
								owner := item.OwnerID
								if p, ok := partsByID[item.OwnerID]; ok {
									owner = p.Handle
								}
								ui.Row(c.Key("br-item-"+item.ID)).Radius(10).Background(insetBg).
									Padding(10, 14).Gap(12).AlignItems(ui.Center).Children(func() {
									sBtn := ui.ButtonBase(c.Key("br-st-"+item.ID)).Label(string(item.Status)).
										Padding(4, 10).Radius(999).Background(cardBg).AlignItems(ui.Center)
									if sBtn.Hovered() {
										sBtn.Background(hoverBg)
									}
									sBtn.Children(func() {
										ui.Text(c, string(item.Status)).Font(font).FontSize(11).FontWeight(700).TextColor(pillCol).NoWrap()
									})
									if sBtn.Clicked() {
										nextItemID = item.ID
										nextItemStat = nextSt
									}
									ui.Text(c, item.Title).Font(font).FontSize(13.5).FontWeight(600).TextColor(ink).Grow(1)
									ui.Text(c, owner).Font(font).FontSize(12).TextColor(muted).NoWrap()
								})
							}
						})
					})
				})

			case chatroom.TabRoles:
				ui.Column(c.Key("br-roles-card")).Grow(1).MinHeight(0).Radius(cardRadius).Background(cardBg).
					Padding(16, 18).Gap(10).AlignItems(ui.Stretch).Children(func() {
					ui.Text(c, "Role-Governed Agent Capabilities (human-only gate approval invariant)").
						Font(font).FontSize(12).TextColor(muted).NoWrap()
					ui.Scroll(c.Key("br-roles-scroll")).Grow(1).Children(func() {
						ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
							for _, p := range snap.Participants {
								part := p
								ui.Row(c.Key("br-role-"+part.ID)).Radius(10).Background(insetBg).
									Padding(10, 14).Gap(12).AlignItems(ui.Center).Children(func() {
									ui.Column(c).Width(200).Gap(2).Children(func() {
										ui.Text(c, part.DisplayName).Font(font).FontSize(13).FontWeight(700).TextColor(ink).NoWrap()
										ui.Text(c, part.Handle+" · "+part.HostNode).Font(font).FontSize(11).TextColor(muted).NoWrap()
									})
									roleChip(c, font, "post", part.Permissions.CanPost, success, muted, cardBg)
									roleChip(c, font, "tasks", part.Permissions.CanManageItems, info, muted, cardBg)
									roleChip(c, font, "tools", part.Permissions.CanInvokeTools, warning, muted, cardBg)
									roleChip(c, font, "gate-req", part.Permissions.RequireGate, accent, muted, cardBg)
									ui.Spacer(c)
									if part.Permissions.CanApproveGates {
										roleChip(c, font, "HUMAN GATE APPROVER", true, danger, muted, cardBg)
									}
								})
							}
						})
					})
				})

			default: // Timeline
				ui.Column(c.Key("br-timeline-card")).Grow(1).MinHeight(0).Radius(cardRadius).Background(cardBg).
					Padding(16, 18).Gap(12).AlignItems(ui.Stretch).Children(func() {
					ui.Scroll(c.Key("br-msg-scroll")).Grow(1).Children(func() {
						ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
							for _, msg := range snap.Messages {
								item := msg
								author := item.AuthorID
								handle := ""
								roleCol := info
								if p, ok := partsByID[item.AuthorID]; ok {
									author = p.DisplayName
									handle = p.Handle
									if p.Kind == chatroom.ParticipantHuman {
										roleCol = success
									} else {
										roleCol = accent
									}
								}
								ui.Column(c.Key("br-msg-"+item.ID)).Radius(12).Background(insetBg).
									Padding(12, 14).Gap(6).AlignItems(ui.Stretch).Children(func() {
									ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
										ui.Text(c, author).Font(font).FontSize(13).FontWeight(700).TextColor(roleCol).NoWrap()
										if handle != "" {
											ui.Text(c, handle).Font(font).FontSize(11.5).TextColor(muted).NoWrap()
										}
										ui.Spacer(c)
										tb := ui.ButtonBase(c.Key("br-th-"+item.ID)).Label("Thread").
											Padding(2, 8).Radius(6).AlignItems(ui.Center)
										if tb.Hovered() {
											tb.Background(hoverBg)
										}
										tb.Children(func() {
											ui.Text(c, "Reply").Font(font).FontSize(11.5).TextColor(muted).NoWrap()
										})
										if tb.Clicked() {
											openThread = item.ID
										}
									})
									ui.Text(c, item.Body).Font(font).FontSize(13.5).LineHeight(1.45).TextColor(ink)
								})
							}
						})
					})

					// Rounded matte composer bar.
					ui.Row(c.Key("br-composer")).Radius(12).Background(insetBg).Padding(8, 12).Gap(10).AlignItems(ui.Center).Children(func() {
						inp := ui.TextInputBase(c.Key("br-draft"), &m.draft).
							Placeholder("Message " + activeCh.Name + " (mention @architect, @reviewer)...").
							Font(font).FontSize(13.5).TextColor(ink).Grow(1)
						if inp.Submitted() {
							sendMsg = true
						}
						sb := ui.ButtonBase(c.Key("br-send")).Label("Send").
							Padding(6, 14).Radius(9).Background(accent).AlignItems(ui.Center)
						sb.Children(func() {
							ui.Text(c, "Send").Font(font).FontSize(12.5).FontWeight(700).TextColor(canvas).NoWrap()
						})
						if sb.Clicked() {
							sendMsg = true
						}
					})
				})
			}
		})

		// Optional Thread Drawer Bento Card.
		if snap.ActiveThreadID != "" {
			ui.Column(c.Key("br-thread-card")).Width(280).Radius(cardRadius).Background(cardBg).
				Padding(14, 16).Gap(10).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, "THREAD").Font(font).FontSize(11.5).FontWeight(700).TextColor(info).NoWrap()
					cb := ui.ButtonBase(c.Key("br-th-close")).Label("Close").Padding(2, 8).Radius(6)
					if cb.Hovered() {
						cb.Background(hoverBg)
					}
					cb.Children(func() {
						ui.Text(c, "✕").Font(font).FontSize(12).TextColor(muted).NoWrap()
					})
					if cb.Clicked() {
						closeThread = true
					}
				})
				ui.Scroll(c.Key("br-th-scroll")).Grow(1).Children(func() {
					ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
						for _, rep := range snap.ThreadReplies {
							r := rep
							author := r.AuthorID
							if p, ok := partsByID[r.AuthorID]; ok {
								author = p.DisplayName
							}
							ui.Column(c.Key("br-rep-"+r.ID)).Radius(10).Background(insetBg).Padding(10, 12).Gap(4).Children(func() {
								ui.Text(c, author).Font(font).FontSize(12).FontWeight(700).TextColor(accent).NoWrap()
								ui.Text(c, r.Body).Font(font).FontSize(12.5).LineHeight(1.4).TextColor(ink)
							})
						}
					})
				})
				ui.Row(c).Radius(10).Background(insetBg).Padding(6, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					inp := ui.TextInputBase(c.Key("br-th-draft"), &m.threadDraft).
						Placeholder("Reply in thread...").Font(font).FontSize(12.5).TextColor(ink).Grow(1)
					if inp.Submitted() {
						sendThread = true
					}
				})
			})
		}
	})

	// Commit controller mutations cleanly outside layout closures.
	if selectCh != "" {
		_ = ctrl.SelectChannel(selectCh)
	}
	if mention != "" {
		if strings.TrimSpace(m.draft) == "" {
			m.draft = mention + " "
		} else {
			m.draft = strings.TrimSpace(m.draft) + " " + mention + " "
		}
	}
	if sendMsg && strings.TrimSpace(m.draft) != "" {
		if _, err := ctrl.PostMessage(snap.ActiveChannelID, snap.CurrentActorID, m.draft, ""); err == nil {
			m.draft = ""
		}
	}
	if openThread != "" {
		_ = ctrl.OpenThread(openThread)
	}
	if closeThread {
		ctrl.CloseThread()
	}
	if sendThread && snap.ActiveThreadID != "" && strings.TrimSpace(m.threadDraft) != "" {
		if _, err := ctrl.PostMessage(snap.ActiveChannelID, snap.CurrentActorID, m.threadDraft, snap.ActiveThreadID); err == nil {
			m.threadDraft = ""
		}
	}
	if addItem && strings.TrimSpace(m.itemDraft) != "" {
		if _, err := ctrl.CreateItem(snap.ActiveChannelID, snap.CurrentActorID, m.itemDraft, snap.CurrentActorID); err == nil {
			m.itemDraft = ""
		}
	}
	if nextItemID != "" {
		_ = ctrl.TransitionItem(nextItemID, snap.CurrentActorID, nextItemStat)
	}
}

func roleChip(c *ui.Context, font, label string, on bool, onCol, offCol, bg ui.Color) {
	col := offCol
	if on {
		col = onCol
	}
	ui.Row(c).Padding(3, 8).Radius(999).Background(bg).Children(func() {
		ui.Text(c, label).Font(font).FontSize(11).FontWeight(600).TextColor(col).NoWrap()
	})
}
