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

	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// SetRoomDraft updates the Agent Room composer draft (useful for external
// shortcuts and automated rendering tests).
func (s *Shell) SetRoomDraft(draft string) {
	s.mu.Lock()
	s.roomDraft = draft
	s.mu.Unlock()
	s.invalidate()
}

// renderAgentRoom renders Marlin iOS's first-principles AI-Native collaborative
// Agent Room: Discord-inspired spatial navigation (Channels & Direct Bots
// drawer + tappable Channel Details sheet), Apple 26 Liquid Glass role-tinted
// SF Symbol bot emblems, edge-to-edge consecutive-grouped Markdown message
// stream, compact thread preview cards, and a context-aware @ mention popover.
func (s *Shell) renderAgentRoom(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	snap := featurekit.AgentRoomSnapshot()
	bottomReserve := float32(14)
	if hasFloatingTabBar {
		bottomReserve = 82
	}

	if !snap.Available {
		ui.Column(c.Key("ios-room-unavail")).Fill().Padding(24, 20, bottomReserve, 20).AlignItems(ui.Center).Justify(ui.Center).Gap(10).Children(func() {
			sfIcon(c, "person.3", 32, th.InkMuted, false)
			ui.Text(c, "Agent Room Unavailable").Font(kit.FontDisplay).FontSize(16).FontWeight(700).TextColor(th.Ink)
			ui.Text(c, "Rebuild without marlin_minimal to enable multi-agent collaborative channels.").
				Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted)
		})
		return
	}

	activeChan := featurekit.RoomChannelView{ID: snap.ActiveChannelID, Name: "workspace"}
	for _, ch := range snap.Channels {
		if ch.ID == snap.ActiveChannelID {
			activeChan = ch
			break
		}
	}

	var onlineAgents []featurekit.RoomParticipantView
	partByID := make(map[string]featurekit.RoomParticipantView, len(snap.Participants))
	currentUserHandle := "you"
	for _, p := range snap.Participants {
		partByID[p.ID] = p
		if p.ID == snap.CurrentActorID && p.Handle != "" {
			currentUserHandle = strings.ToLower(p.Handle)
		}
		if p.IsAgent && strings.ToLower(p.Presence) != "offline" {
			onlineAgents = append(onlineAgents, p)
		}
	}

	s.mu.Lock()
	mentionForceOpen := s.roomMentionForceOpen
	draft := s.roomDraft
	s.mu.Unlock()

	ui.Column(c.Key("ios-room-root")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		// Unified Discord Mobile × Apple 26 Top Navigation Header:
		// [Drawer] | [#channel › + topic] | [Live Bot Avatar Stack + Theme]
		s.renderRoomNavHeader(c, th, activeChan, onlineAgents, partByID)

		// Compact high-signal quick-switch strip: workspace channels + Tasks shortcut
		// (replaces wasted vertical space with 1-tap mobile navigation).
		s.renderRoomQuickStrip(c, th, snap)

		// Crisp 1px hairline separating navigation chrome from the message stream.
		ui.Box(c).Height(1).FillWidth().Background(kit.WithAlpha(th.BorderSubtle, 0.58))

		// Bottom-anchored edge-to-edge grouped message stream (Discord/iMessage ergonomics).
		s.renderRoomFeed(c, th, snap, activeChan, partByID, currentUserHandle)

		// Context-aware floating @ mention popover + detached Apple 26 Liquid Glass composer.
		s.renderRoomComposer(c, th, snap, activeChan, draft, mentionForceOpen, bottomReserve)
	})

	// Sheet 1: Channels & Direct AI Bots Drawer (stable pointer required by mobile.Sheet spring state).
	s.renderRoomChannelsSheet(c, th, snap, partByID, &s.roomChanSheetOpen)

	// Sheet 2: Channel Details (Members | Tasks | Pinned | Roles).
	s.renderRoomDetailsSheet(c, th, snap, activeChan, partByID, currentUserHandle, &s.roomDetailsSheetOpen, &s.roomDetailsTabIdx, &s.roomTaskDraft)

	// Sheet 3: Thread Replies Sheet.
	s.roomThreadSheetOpen = snap.ActiveThreadID != ""
	prevThreadOpen := s.roomThreadSheetOpen
	s.renderRoomThreadSheet(c, th, snap, partByID, currentUserHandle, &s.roomThreadSheetOpen, &s.roomThreadDraft)
	if prevThreadOpen && !s.roomThreadSheetOpen {
		featurekit.CloseAgentRoomThread()
		s.invalidate()
	}
}

func (s *Shell) renderRoomNavHeader(
	c *ui.Context,
	th theme.Theme,
	activeChan featurekit.RoomChannelView,
	onlineAgents []featurekit.RoomParticipantView,
	partByID map[string]featurekit.RoomParticipantView,
) {
	topPad := float32(10)
	if c.TitleBar().Height > 0 {
		topPad = 4
	}
	ui.Row(c.Key("ios-room-subhdr")).Padding(topPad, 14, 10, 14).Gap(8).AlignItems(ui.Center).Clip().Children(func() {
		// Left: Apple 26 Liquid Glass Navigation Cluster: Back to Chat (<) + Channels Drawer.
		// Users can also swipe left/right anywhere to move seamlessly between Chat, Rooms, Bento, and Style.
		leftGrp := glass.NewGroup(c)
		leftGrp.Radius = 17
		leftGrp.Tint = kit.WithAlpha(th.SurfaceElevated, 0.28)
		leftGrp.Gap(2).AlignItems(ui.Center).Children(func() {
			backBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-room-back-btn")).Label("Back").
				Width(32).Height(34).Radius(17).AlignItems(ui.Center).Justify(ui.Center))
			backBtn.Children(func() {
				sfIcon(c, "chevron.left", 14, th.Ink, false)
			})
			leftGrp.Pane(backBtn)
			if backBtn.Clicked() {
				s.SelectTab(TabChat)
			}

			drawerBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-room-drawer-btn")).Label("Channels").
				Width(34).Height(34).Radius(17).AlignItems(ui.Center).Justify(ui.Center))
			drawerBtn.Children(func() {
				sfIcon(c, "sidebar.left", 14.5, th.Ink, false)
			})
			leftGrp.Pane(drawerBtn)
			if drawerBtn.Clicked() {
				mygo.Haptics.Selection()
				s.mu.Lock()
				s.roomChanSheetOpen = true
				s.mu.Unlock()
				s.invalidate()
			}
		})

		// Center: Single unified #channel / @Bot title + 1-line topic -> Channel Details Sheet.
		titlePrefix := "#"
		if activeChan.IsDM {
			titlePrefix = "@"
		}
		displayName := titlePrefix + strings.TrimPrefix(strings.TrimPrefix(activeChan.Name, "#"), "@")
		titleBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-room-title-btn")).Label(displayName).
			Basis(0).Grow(1).MinWidth(0).Padding(3, 6).Radius(12).
			Column().AlignItems(ui.Start).Justify(ui.Center).Gap(1))
		if titleBtn.Hovered() || titleBtn.Pressed() {
			titleBtn.Background(kit.WithAlpha(th.SurfaceElevated, 0.32))
		}
		titleBtn.Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
				if activeChan.IsDM {
					if bot, ok := partByID[activeChan.BotParticipantID]; ok {
						_, sym, col := roleVisualSpec(bot.Handle, true, th)
						sfIcon(c, sym, 13.5, col, true)
					}
				}
				ui.Text(c, displayName).Font(kit.FontDisplay).FontSize(17).FontWeight(700).LetterSpacing(-0.2).TextColor(th.Ink).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
				sfIcon(c, "chevron.right", 10.5, th.InkMuted, false)
			})
			if strings.TrimSpace(activeChan.Topic) != "" {
				ui.Text(c, activeChan.Topic).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
			}
		})
		if titleBtn.Clicked() {
			mygo.Haptics.Selection()
			s.mu.Lock()
			s.roomDetailsSheetOpen = true
			s.mu.Unlock()
			s.invalidate()
		}

		// Right: Unified Apple 26 Liquid Glass cluster (Live Bot Avatar Stack + Theme Switch).
		grp := glass.NewGroup(c)
		grp.Radius = 17
		grp.Tint = kit.WithAlpha(th.SurfaceElevated, 0.28)
		grp.Gap(4).AlignItems(ui.Center).Children(func() {
			stackBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-room-members-pill")).Label("Members").
				Height(34).Padding(0, 9).Gap(6).Radius(17).AlignItems(ui.Center))
			stackBtn.Children(func() {
				shown := len(onlineAgents)
				if shown > 3 {
					shown = 3
				}
				if shown > 0 {
					ui.Row(c).Gap(-5).AlignItems(ui.Center).Children(func() {
						for i := 0; i < shown; i++ {
							ag := onlineAgents[i]
							_, sym, tone := roleVisualSpec(ag.Handle, true, th)
							ui.Box(c.Key("ios-astack-"+ag.ID)).Width(20).Height(20).Shrink(0).Radius(10).
								AlignItems(ui.Center).Justify(ui.Center).
								Background(kit.WithAlpha(tone, 0.24)).
								Border(1.5, th.Canvas).Children(func() {
								sfIcon(c, sym, 10, tone, true)
							})
						}
					})
				}
				ui.Text(c, fmt.Sprintf("%d AI", len(onlineAgents))).
					Font(theme.FontMono).FontSize(10.5).FontWeight(600).TextColor(th.Ink).SingleLine()
			})
			grp.Pane(stackBtn)
			if stackBtn.Clicked() {
				mygo.Haptics.Selection()
				s.mu.Lock()
				s.roomDetailsTabIdx = 0
				s.roomDetailsSheetOpen = true
				s.mu.Unlock()
				s.invalidate()
			}

			themeBtn := kit.ActionControl(ui.ButtonBase(c.Key("ios-room-theme-cycle")).Label("Cycle theme").
				Width(32).Height(34).Radius(17).AlignItems(ui.Center).Justify(ui.Center))
			themeBtn.Children(func() {
				sym := theme.IconThemeDark
				if th.IsLight {
					sym = theme.IconThemeLight
				}
				sfIcon(c, sym, 13, th.Accent, true)
			})
			grp.Pane(themeBtn)
			if themeBtn.Clicked() {
				s.cycleTheme()
			}
		})
	})
}

func (s *Shell) renderRoomQuickStrip(c *ui.Context, th theme.Theme, snap featurekit.RoomFullSnapshot) {
	openTasks := 0
	for _, tk := range snap.Tasks {
		if tk.Status != "DONE" {
			openTasks++
		}
	}
	ui.ScrollHorizontal(c.Key("ios-room-qstrip")).FillWidth().Padding(0, 12, 8, 12).Gap(4).Children(func() {
		// Unified Sync Status Pill (shared 1:1 with Desktop: Cove Live · gRPC / Cove Connecting… / Local Core).
		if strings.TrimSpace(snap.SyncLabel) != "" {
			syncTone := th.Info
			switch snap.SyncMode {
			case "cove-grpc":
				syncTone = th.Success
			case "cove-connecting":
				syncTone = th.Warning
			}
			ui.Row(c.Key("ios-qstrip-sync")).Height(26).Padding(0, 9).Radius(13).Gap(5).
				AlignItems(ui.Center).Shrink(0).
				Background(kit.WithAlpha(syncTone, 0.14)).
				Border(1, kit.WithAlpha(syncTone, 0.28)).Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(syncTone)
				ui.Text(c, snap.SyncLabel).Font(theme.FontMono).FontSize(10).FontWeight(700).TextColor(syncTone).SingleLine()
			})
		}

		// High-signal Tasks shortcut pill pinned first so it is always visible without horizontal scrolling.
		taskLbl := fmt.Sprintf("Tasks %d", openTasks)
		if theme.StyledButton(c, theme.ButtonSpec{
			Key: "ios-qstrip-tasks", Label: taskLbl, Icon: "checklist", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.Info, Pill: true,
		}, th).Clicked() {
			mygo.Haptics.Selection()
			s.roomDetailsTabIdx = 1
			s.roomDetailsSheetOpen = true
			s.invalidate()
		}

		for _, ch := range snap.Channels {
			if ch.IsDM {
				continue
			}
			item := ch
			sel := item.ID == snap.ActiveChannelID
			lbl := "#" + item.Name
			if item.UnreadCount > 0 && !sel {
				lbl = fmt.Sprintf("#%s · %d", item.Name, item.UnreadCount)
			}
			v := theme.BtnGhost
			tone := th.InkMuted
			if sel {
				v = theme.BtnTinted
				tone = th.Accent
			}
			if theme.StyledButton(c, theme.ButtonSpec{
				Key: "ios-qch-" + item.ID, Label: lbl, Variant: v, Size: theme.BtnSizeSM, Tone: tone, Pill: true,
			}, th).Clicked() {
				mygo.Haptics.Selection()
				_ = featurekit.SelectAgentRoomChannel(item.ID)
				s.invalidate()
			}
		}
	})
}

func (s *Shell) renderRoomFeed(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	activeChan featurekit.RoomChannelView,
	partByID map[string]featurekit.RoomParticipantView,
	currentUserHandle string,
) {
	ui.Scroll(c.Key("ios-room-feed-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(8, 0, 8, 0).Children(func() {
		// Bottom-anchored feed (Discord Mobile / iMessage ergonomics): short channels
		// anchor cleanly right above the thumb-zone composer with full immersive height.
		ui.Column(c.Key("ios-room-feed")).MinHeight(640).Justify(ui.End).Gap(2).AlignItems(ui.Stretch).Children(func() {
			if len(snap.Messages) == 0 {
				s.renderChannelEmptyState(c, th, activeChan, partByID)
				return
			}

			// Minimal Discord-style date/channel hairline divider (zero wasted hero space).
			ui.Row(c.Key("ios-room-day-div")).Padding(6, 16, 8, 16).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Height(1).Basis(0).Grow(1).Background(kit.WithAlpha(th.BorderSubtle, 0.55))
				label := "Today · #" + activeChan.Name
				if activeChan.IsDM {
					label = "Direct · @" + activeChan.Name
				}
				ui.Text(c, label).Font(theme.FontMono).FontSize(10).FontWeight(600).TextColor(th.InkMuted).SingleLine()
				ui.Box(c).Height(1).Basis(0).Grow(1).Background(kit.WithAlpha(th.BorderSubtle, 0.55))
			})

			for i, msg := range snap.Messages {
				grouped := i > 0 && canGroupRoomMessages(snap.Messages[i-1], msg)
				nextGrouped := i+1 < len(snap.Messages) && canGroupRoomMessages(msg, snap.Messages[i+1])
				s.renderRoomStreamMessage(c, th, msg, partByID, currentUserHandle, grouped, !nextGrouped, false)
			}
		})
	})
}

func (s *Shell) renderChannelEmptyState(
	c *ui.Context,
	th theme.Theme,
	activeChan featurekit.RoomChannelView,
	partByID map[string]featurekit.RoomParticipantView,
) {
	ui.Column(c.Key("ios-room-empty-"+activeChan.ID)).Padding(24, 18).Gap(10).AlignItems(ui.Start).Children(func() {
		if activeChan.IsDM {
			if bot, ok := partByID[activeChan.BotParticipantID]; ok {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					renderRoleEmblemAvatar(c, bot.Name, bot.Handle, true, bot.Presence, 38, th)
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, bot.Name).Font(kit.FontDisplay).FontSize(16).FontWeight(700).TextColor(th.Ink).SingleLine()
							renderVerifiedAIBadge(c, th)
						})
						ui.Text(c, activeChan.Topic).Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted)
					})
				})
			}
		} else {
			ui.Text(c, "Start coordinating in #"+activeChan.Name).Font(kit.FontDisplay).FontSize(15).FontWeight(700).TextColor(th.Ink)
			ui.Text(c, "Tap @ below to route tasks to @architect, @reviewer, or @security.").
				Font(kit.FontDisplay).FontSize(12.5).LineHeight(1.45).TextColor(th.InkMuted)
		}
	})
}

func canGroupRoomMessages(prev, cur featurekit.RoomMessageView) bool {
	if prev.AuthorID == "" || prev.AuthorID != cur.AuthorID {
		return false
	}
	if cur.Pinned || cur.GateID != "" {
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

func msgMentionsHandle(msg featurekit.RoomMessageView, handle string) bool {
	if handle == "" {
		return false
	}
	target := strings.ToLower(strings.TrimPrefix(handle, "@"))
	for _, m := range msg.Mentions {
		if strings.ToLower(strings.TrimPrefix(m, "@")) == target {
			return true
		}
	}
	return false
}

func (s *Shell) renderRoomStreamMessage(
	c *ui.Context,
	th theme.Theme,
	msg featurekit.RoomMessageView,
	partByID map[string]featurekit.RoomParticipantView,
	currentUserHandle string,
	grouped bool,
	showActions bool,
	inThread bool,
) {
	m := msg
	mentionedMe := msgMentionsHandle(m, currentUserHandle)

	var rowBg ui.Color
	var accentStripe ui.Color
	if mentionedMe {
		rowBg = kit.WithAlpha(th.Accent, 0.08)
		accentStripe = th.Accent
	} else if m.Pinned {
		rowBg = kit.WithAlpha(th.Warning, 0.05)
		accentStripe = th.Warning
	}

	topPad := float32(7)
	if grouped {
		topPad = 2
	}

	paintRow := func() {
		row := ui.Row(c.Key("ios-rmsg-"+m.ID)).FillWidth().Padding(topPad, 14, 5, 14).Gap(10).AlignItems(ui.Start)
		if rowBg.A > 0 {
			row.Background(rowBg)
		}
		row.Children(func() {
			if accentStripe.A > 0 {
				ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(2.5).Background(accentStripe)
			}

			if grouped {
				// Preserve exact 36pt avatar gutter so consecutive lines align cleanly like Discord.
				ui.Box(c).Width(36).Shrink(0)
			} else {
				renderRoleEmblemAvatar(c, m.AuthorName, m.AuthorHandle, m.IsAgent, m.AuthorPresence, 36, th)
			}

			ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(4).AlignItems(ui.Stretch).Children(func() {
				if !grouped {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
						nameCol := th.Ink
						if m.IsAgent {
							_, _, nameCol = roleVisualSpec(m.AuthorHandle, true, th)
						}
						ui.Text(c, m.AuthorName).Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(nameCol).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
						if m.IsAgent {
							renderVerifiedAIBadge(c, th)
						} else if part, ok := partByID[m.AuthorID]; ok && part.CanApproveGates {
							renderApproverBadge(c, th)
						}
						if m.AuthorRole != "" {
							ui.Text(c, m.AuthorRole).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
								SingleLine().Ellipsis("…").MinWidth(0).Clip()
						}
						ui.Spacer(c)
						if m.TaintedExternal {
							ui.Text(c, "External").Font(theme.FontMono).FontSize(9).FontWeight(600).TextColor(th.Warning).
								Padding(1.5, 5.5).Radius(theme.RadiusPill).Background(kit.WithAlpha(th.Warning, 0.16)).SingleLine()
						}
						if m.Delivery == "QUEUED" || m.Delivery == "FAILED" {
							ui.Text(c, m.Delivery).Font(theme.FontMono).FontSize(9).FontWeight(600).TextColor(th.InkMuted).
								Padding(1.5, 5.5).Radius(theme.RadiusPill).Background(kit.WithAlpha(th.BorderSubtle, 0.36)).SingleLine()
						}
						if m.Pinned {
							ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
								sfIcon(c, "pin", 10, th.Warning, true)
								ui.Text(c, "Pinned").Font(theme.FontMono).FontSize(9.5).FontWeight(600).TextColor(th.Warning).SingleLine()
							})
						}
						if m.Timestamp != "" {
							ui.Text(c, m.Timestamp).Font(theme.FontMono).FontSize(10).TextColor(th.InkMuted).SingleLine()
						}
					})
				}

				// Rich Markdown message body (code blocks, bold, inline code, lists).
				if body := strings.TrimSpace(m.Body); body != "" {
					markdown.View(c.Key("ios-rmd-"+m.ID), body, roomMarkdownOpts(th))
				}

				// C4: Inline Tier-0 Bot Gate Approval Embed Card (3px amber left stripe + Approve / Reject buttons).
				if m.GateID != "" {
					s.renderInlineGateEmbed(c, th, m)
				}

				// Highlighted @mention pills when the message addresses participants.
				if len(m.Mentions) > 0 {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Wrap().Children(func() {
						for _, handle := range m.Mentions {
							h := handle
							_, _, tone := roleVisualSpec(h, strings.ToLower(h) != currentUserHandle, th)
							ui.Text(c, "@"+h).Font(theme.FontMono).FontSize(10.5).FontWeight(600).TextColor(tone).
								Padding(2, 7).Radius(theme.RadiusPill).
								Background(kit.WithAlpha(tone, 0.14)).
								Border(1, kit.WithAlpha(tone, 0.24)).SingleLine()
						}
					})
				}

				// Discord Thread Preview Card (indented hairline card when replies exist).
				if !inThread && m.ThreadCount > 0 {
					lbl := fmt.Sprintf("%d replies", m.ThreadCount)
					if m.ThreadCount == 1 {
						lbl = "1 reply"
					}
					thCard := kit.ActionControl(ui.ButtonBase(c.Key("ios-rth-card-"+m.ID)).Label("View thread").
						Padding(6, 10).Gap(7).Radius(11).AlignItems(ui.Center).
						Background(kit.WithAlpha(th.SurfaceElevated, 0.34)).
						Border(1, kit.WithAlpha(th.Accent, 0.22)))
					thCard.Children(func() {
						sfIcon(c, "arrow.turn.down.right", 12, th.Accent, false)
						ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(12).FontWeight(600).TextColor(th.Accent).SingleLine()
						ui.Text(c, "· View thread").Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
						ui.Spacer(c)
						sfIcon(c, "chevron.right", 10, th.InkMuted, false)
					})
					if thCard.Clicked() {
						mygo.Haptics.Selection()
						_ = featurekit.OpenAgentRoomThread(m.ID)
						s.invalidate()
					}
				}

				// Compact reactions + subtle quick actions (shown on group tail, when
				// reactions exist, or on hover — avoiding noisy repeated rows per line).
				if showActions || len(m.Reactions) > 0 || row.Hovered() {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Wrap().Children(func() {
						for _, r := range m.Reactions {
							rx := r
							v := theme.BtnGlass
							tCol := th.InkMuted
							if rx.Reacted {
								v = theme.BtnTinted
								tCol = th.Accent
							}
							if theme.StyledButton(c, theme.ButtonSpec{
								Key: fmt.Sprintf("ios-rx-%s-%s", m.ID, rx.Tag), Label: fmt.Sprintf("%s %d", rx.Tag, rx.Count),
								Variant: v, Size: theme.BtnSizeSM, Tone: tCol, Pill: true,
							}, th).Clicked() {
								mygo.Haptics.Light()
								_ = featurekit.ToggleAgentRoomReaction(m.ID, rx.Tag)
								s.invalidate()
							}
						}
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: "ios-rx-ack-" + m.ID, Icon: "hand.thumbsup", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.InkMuted, Pill: true,
						}, th).Clicked() {
							mygo.Haptics.Light()
							_ = featurekit.ToggleAgentRoomReaction(m.ID, "Ack")
							s.invalidate()
						}
						if !inThread && m.ThreadCount == 0 {
							if theme.StyledButton(c, theme.ButtonSpec{
								Key: "ios-rth-" + m.ID, Label: "Reply", Icon: "arrowshape.turn.up.left", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.InkMuted, Pill: true,
							}, th).Clicked() {
								mygo.Haptics.Selection()
								_ = featurekit.OpenAgentRoomThread(m.ID)
								s.invalidate()
							}
						}
					})
				}
			})
		})
	}

	if inThread {
		paintRow()
		return
	}
	// C6: Native MyGo mobile.SwipeRow left-swipe quick reply / thread trigger.
	replyCol := th.Accent
	mobile.SwipeRow(c.Key("ios-rsw-"+m.ID), nil, mobile.SwipeOptions{
		Trailing: []mobile.SwipeAction{
			{
				Label: "Reply",
				Color: &replyCol,
				OnTap: func() {
					mygo.Haptics.Selection()
					_ = featurekit.OpenAgentRoomThread(m.ID)
					s.invalidate()
				},
			},
		},
	}, paintRow)
}

func (s *Shell) renderInlineGateEmbed(c *ui.Context, th theme.Theme, m featurekit.RoomMessageView) {
	var (
		pg      *mygoshell.PromptGateController
		pending *promptgate.PromptRequest
	)
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		pg = s.Adaptive.Desktop.PromptGate
	}
	if pg != nil && pg.Queue != nil {
		if req, _, _, ok := pg.Queue.Active(); ok && req.ID == m.GateID {
			cp := req
			pending = &cp
		}
	}

	embedBg := kit.WithAlpha(th.SurfaceElevated, 0.44)
	stripeCol := th.Warning
	if pending == nil {
		stripeCol = th.Success
	}
	ui.Row(c.Key("ios-rgate-"+m.ID)).FillWidth().Radius(14).Clip().
		Background(embedBg).
		Border(1, kit.WithAlpha(stripeCol, 0.30)).Children(func() {
		ui.Box(c).Width(3.5).FillHeight().Background(stripeCol)
		ui.Column(c).Basis(0).Grow(1).MinWidth(0).Padding(10, 12).Gap(7).AlignItems(ui.Stretch).Children(func() {
			if pending != nil {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					sfIcon(c, "shield.lefthalf.filled", 12, th.Warning, true)
					ui.Text(c, "APPROVAL REQUIRED").Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(th.Warning).SingleLine()
				})
				ui.Text(c, pending.Title).Font(kit.FontDisplay).FontSize(14).FontWeight(700).TextColor(th.Ink)
				if strings.TrimSpace(pending.Prompt) != "" {
					ui.Text(c, pending.Prompt).Font(kit.FontDisplay).FontSize(12.5).LineHeight(1.42).TextColor(th.InkMuted)
				}
				ui.Row(c).Gap(8).Children(func() {
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "ios-rgate-yes-" + m.GateID, Label: "Approve", Icon: theme.IconApprove, Variant: theme.BtnGlassProminent, Size: theme.BtnSizeMD, Tone: th.Success, Pill: true, FullWidth: true,
					}, th).Clicked() {
						mygo.Haptics.Success()
						if r, resp, err := pg.Queue.Resolve(pending.ID, promptgate.PromptResponse{
							PromptID: pending.ID, Action: promptgate.ActionApproveOnce, IdempotencyKey: "room-yes-" + pending.ID,
						}); err == nil && pg.OnResolve != nil {
							pg.OnResolve(r, resp)
						}
						s.invalidate()
					}
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "ios-rgate-no-" + m.GateID, Label: "Reject", Icon: theme.IconReject, Variant: theme.BtnDestructive, Size: theme.BtnSizeMD, Tone: th.Danger, Pill: true, FullWidth: true,
					}, th).Clicked() {
						mygo.Haptics.Warning()
						if r, resp, err := pg.Queue.Resolve(pending.ID, promptgate.PromptResponse{
							PromptID: pending.ID, Action: promptgate.ActionReject, IdempotencyKey: "room-no-" + pending.ID,
						}); err == nil && pg.OnResolve != nil {
							pg.OnResolve(r, resp)
						}
						s.invalidate()
					}
				})
			} else {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					sfIcon(c, "checkmark.shield", 13, th.Success, true)
					ui.Text(c, "Gate Resolved · Tier-0 Audited").Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Success).SingleLine()
				})
			}
		})
	})
}

func activeMentionQuery(draft string) (string, bool) {
	if draft == "" {
		return "", false
	}
	// If draft ends with whitespace, the user already finished the @token.
	last := draft[len(draft)-1]
	if last == ' ' || last == '\n' || last == '\t' {
		return "", false
	}
	idx := strings.LastIndexAny(draft, " \n\t")
	token := draft[idx+1:]
	if !strings.HasPrefix(token, "@") {
		return "", false
	}
	return strings.ToLower(token[1:]), true
}

func applyMentionCompletion(draft, handle string) string {
	clean := strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if _, ok := activeMentionQuery(draft); ok {
		idx := strings.LastIndexAny(draft, " \n\t")
		prefix := draft[:idx+1]
		return prefix + "@" + clean + " "
	}
	trimmed := strings.TrimSpace(draft)
	if trimmed == "" {
		return "@" + clean + " "
	}
	return trimmed + " @" + clean + " "
}

func activeSlashQuery(draft string) (string, bool) {
	trimmed := strings.TrimLeft(draft, " \t")
	if !strings.HasPrefix(trimmed, "/") || strings.ContainsAny(trimmed, " \n\t") {
		return "", false
	}
	return strings.ToLower(trimmed[1:]), true
}

// SubmitRoomDraft executes slash commands (/task, /pin, /clear, /ask) or posts
// a normal message into the active Agent Room channel.
func (s *Shell) SubmitRoomDraft(channelID, draft string) {
	txt := strings.TrimSpace(draft)
	if txt == "" {
		return
	}
	mygo.Haptics.Medium()
	clearDraft := func() {
		s.mu.Lock()
		s.roomDraft = ""
		s.roomMentionForceOpen = false
		s.mu.Unlock()
		s.invalidate()
	}
	lower := strings.ToLower(txt)
	switch {
	case lower == "/clear":
		clearDraft()
		return
	case lower == "/pin":
		s.roomDetailsTabIdx = 2
		s.roomDetailsSheetOpen = true
		clearDraft()
		return
	case strings.HasPrefix(lower, "/task "):
		title := strings.TrimSpace(txt[len("/task "):])
		if title != "" {
			_ = featurekit.AddAgentRoomTask(title)
		}
		clearDraft()
		return
	case strings.HasPrefix(lower, "/ask "):
		txt = strings.TrimSpace(txt[len("/ask "):])
	}
	if txt != "" {
		_ = featurekit.PostAgentRoomMessage(channelID, txt, "")
	}
	clearDraft()
}

func (s *Shell) renderRoomComposer(
	c *ui.Context,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	activeChan featurekit.RoomChannelView,
	draft string,
	forceMentionOpen bool,
	bottomReserve float32,
) {
	query, typedMention := activeMentionQuery(draft)
	slashQuery, slashActive := activeSlashQuery(draft)
	showMentionPopover := (typedMention || forceMentionOpen) && !slashActive

	ui.Column(c.Key("ios-room-comp-wrap")).Padding(4, 14, bottomReserve, 14).Gap(6).AlignItems(ui.Stretch).Children(func() {
		// D3: Live Bot Typing / Thinking Indicator Bar above composer.
		if len(snap.ThinkingBots) > 0 {
			verb := "is thinking…"
			if len(snap.ThinkingBots) > 1 {
				verb = "are thinking…"
			}
			ui.Row(c.Key("ios-room-typing")).Padding(2, 8).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(12, 12)
				ui.Text(c, fmt.Sprintf("%s %s", strings.Join(snap.ThinkingBots, ", "), verb)).
					Font(kit.FontDisplay).FontSize(12).FontWeight(500).TextColor(th.Accent).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
			})
		}

		// D2: Floating Liquid Glass Slash Command Popover when draft starts with '/'.
		if slashActive {
			s.renderSlashPopover(c, th, slashQuery)
		}

		// D1: Context-aware floating Liquid Glass @ Mention Autocomplete Popover.
		if showMentionPopover {
			s.renderMentionPopover(c, th, snap.Participants, draft, query)
		}

		placeholder := "Message #" + activeChan.Name + "…"
		if activeChan.IsDM {
			placeholder = "Message @" + activeChan.Name + "…"
		}

		// Apple iOS 26 HIG Detached Room Composer: standalone floating Liquid Glass
		// '@' trigger beside a pure Liquid Glass input capsule (zero glass-on-glass
		// nesting, concentric 32pt Send CTA inside a 42pt pill).
		ui.Row(c.Key("ios-room-comp-row")).Gap(8).AlignItems(ui.Center).Children(func() {
			atVariant := theme.BtnGlass
			atTone := th.InkMuted
			if showMentionPopover {
				atVariant = theme.BtnTinted
				atTone = th.Accent
			}
			if theme.IconCircleButton(c, "ios-room-at-btn", "Mention", "at", 40, atVariant, atTone, false, th).Clicked() {
				mygo.Haptics.Selection()
				s.mu.Lock()
				s.roomMentionForceOpen = !s.roomMentionForceOpen
				s.mu.Unlock()
				s.invalidate()
			}

			pill := ui.Row(c.Key("ios-room-comp-bar")).Grow(1).MinWidth(0).Height(42).Padding(4, 5, 4, 14).Gap(8).Radius(21).AlignItems(ui.Center).
				Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.34)}).
				Shadow(0, 6, 18, -5, kit.WithAlpha(ui.RGB(0, 0, 0), 0.28))
			if !c.Vibrancy() {
				pill.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
			}
			pill.Children(func() {
				inp := ui.TextInputBase(c.Key("ios-room-input"), &draft).
					Placeholder(placeholder).
					Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
				if inp.Changed() {
					s.mu.Lock()
					s.roomDraft = draft
					if _, active := activeMentionQuery(draft); !active {
						s.roomMentionForceOpen = false
					}
					s.mu.Unlock()
				}
				sendRoom := func() {
					s.SubmitRoomDraft(snap.ActiveChannelID, draft)
				}
				if inp.Submitted() {
					sendRoom()
				}
				canSend := strings.TrimSpace(draft) != ""
				varType := theme.BtnSolidIdle
				tone := th.InkMuted
				if canSend {
					varType = theme.BtnGlassProminent
					tone = th.Accent
				}
				if theme.IconCircleButton(c, "ios-room-send", "Send room message", theme.IconSend, 32, varType, tone, false, th).Clicked() && canSend {
					sendRoom()
				}
			})
		})
	})
}

func (s *Shell) renderSlashPopover(c *ui.Context, th theme.Theme, query string) {
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
		return
	}
	pop := ui.Column(c.Key("ios-room-slash-popover")).Padding(8, 10).Gap(4).Radius(18).AlignItems(ui.Stretch).
		Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.56)}).
		Shadow(0, 8, 22, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.34))
	if !c.Vibrancy() {
		pop.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
	}
	pop.Children(func() {
		ui.Text(c, "ROOM COMMANDS").Font(theme.FontMono).FontSize(9.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
			Padding(2, 6, 0, 6).SingleLine()
		for _, idx := range matched {
			item := cmds[idx]
			btn := kit.ActionControl(ui.ButtonBase(c.Key("ios-scmd-"+item.cmd)).Label(item.cmd).
				FillWidth().Padding(7, 8).Gap(9).Radius(12).AlignItems(ui.Center))
			if btn.Hovered() || btn.Pressed() {
				btn.Background(kit.WithAlpha(th.Accent, 0.14))
			}
			btn.Children(func() {
				sfIcon(c, item.icon, 14, th.Accent, false)
				ui.Text(c, item.cmd).Font(theme.FontMono).FontSize(12.5).FontWeight(700).TextColor(th.Ink).SingleLine()
				ui.Text(c, item.desc).Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
			})
			if btn.Clicked() {
				mygo.Haptics.Selection()
				if item.insert == "/pin" || item.insert == "/clear" {
					s.SubmitRoomDraft("", item.insert)
				} else {
					s.mu.Lock()
					s.roomDraft = item.insert
					s.mu.Unlock()
					s.invalidate()
				}
			}
		}
	})
}

func (s *Shell) renderMentionPopover(
	c *ui.Context,
	th theme.Theme,
	participants []featurekit.RoomParticipantView,
	draft, query string,
) {
	var agents, humans []featurekit.RoomParticipantView
	for _, p := range participants {
		if query != "" &&
			!strings.Contains(strings.ToLower(p.Handle), query) &&
			!strings.Contains(strings.ToLower(p.Name), query) &&
			!strings.Contains(strings.ToLower(p.Role), query) {
			continue
		}
		if p.IsAgent {
			agents = append(agents, p)
		} else {
			humans = append(humans, p)
		}
	}
	if len(agents) == 0 && len(humans) == 0 {
		return
	}

	pop := ui.Column(c.Key("ios-room-mention-popover")).Padding(8, 10).Gap(6).Radius(18).AlignItems(ui.Stretch).
		Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.56)}).
		Shadow(0, 8, 22, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.34))
	if !c.Vibrancy() {
		pop.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
	}
	pop.Children(func() {
		renderGroup := func(header string, list []featurekit.RoomParticipantView) {
			if len(list) == 0 {
				return
			}
			ui.Text(c, header).Font(theme.FontMono).FontSize(9.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
				Padding(2, 6, 0, 6).SingleLine()
			for _, p := range list {
				mem := p
				btn := kit.ActionControl(ui.ButtonBase(c.Key("ios-mpop-"+mem.ID)).Label("@"+mem.Handle).
					FillWidth().Padding(6, 8).Gap(9).Radius(12).AlignItems(ui.Center))
				if btn.Hovered() || btn.Pressed() {
					btn.Background(kit.WithAlpha(th.Accent, 0.14))
				}
				btn.Children(func() {
					renderRoleEmblemAvatar(c, mem.Name, mem.Handle, mem.IsAgent, mem.Presence, 26, th)
					ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).AlignItems(ui.Start).Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							ui.Text(c, mem.Name).Font(kit.FontDisplay).FontSize(13).FontWeight(600).TextColor(th.Ink).SingleLine()
							ui.Text(c, "@"+mem.Handle).Font(theme.FontMono).FontSize(11).TextColor(th.Accent).SingleLine()
						})
						ui.Text(c, mem.Role).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
					ui.Text(c, presenceLabel(mem.Presence)).Font(theme.FontMono).FontSize(10).
						TextColor(presenceColor(mem.Presence, th)).SingleLine()
				})
				if btn.Clicked() {
					mygo.Haptics.Selection()
					s.mu.Lock()
					s.roomDraft = applyMentionCompletion(draft, mem.Handle)
					s.roomMentionForceOpen = false
					s.mu.Unlock()
					s.invalidate()
				}
			}
		}
		renderGroup("AI AGENTS", agents)
		renderGroup("HUMANS", humans)
	})
}
