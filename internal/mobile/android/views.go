// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package android

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygomobile "github.com/egoist/mygo/ui/mobile"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	"github.com/tao3k/marlin-ui/internal/mobile"
)

const materialCardRadius float32 = 16

// View paints the complete Material 3 Android companion shell: Top App Bar,
// active destination content (Chat, Rooms, Bento Home + Scheme Extensions,
// Search, Style Settings, Approvals), and Material 3 Bottom Navigation bar.
func (s *Shell) View(c *ui.Context) {
	s.mu.Lock()
	th := s.Theme
	tab := s.activeTab
	modelLbl := s.currentModelLabelLocked()
	s.mu.Unlock()

	theme.ApplyToMyGo(c, th)
	gateCount := s.pendingGateCount()

	ui.Column(c.Key("and-shell")).Fill().Background(th.Canvas).AlignItems(ui.Stretch).Children(func() {
		// Material 3 Top App Bar.
		ui.Row(c.Key("and-top-bar")).Height(52).Padding(0, 14).Gap(10).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			connCol := th.Warning
			if s.Link != nil && s.Link.Mode != hostlink.ModeStandalone {
				connCol = th.Success
			}
			ui.Box(c).Width(8).Height(8).Radius(4).Background(connCol)
			ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, tabTitle(tab)).Font(kit.FontDisplay).FontSize(16).FontWeight(700).TextColor(th.Ink).SingleLine()
				if modelLbl != "" {
					ui.Text(c, mobile.ShortModelName(modelLbl)).Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
				}
			})
			if gateCount > 0 {
				if theme.StyledButton(c, theme.ButtonSpec{
					Key: "and-top-gates", Label: fmt.Sprintf("%d Gate", gateCount), Icon: "checkmark.shield", Variant: theme.BtnTinted, Size: theme.BtnSizeSM, Tone: th.Warning, Pill: true,
				}, th).Clicked() {
					s.SelectTab(mobile.TabGates)
				}
			}
			if tab == mobile.TabChat {
				if theme.IconCircleButton(c, "and-new-chat", "New Chat", "square.and.pencil", 34, theme.BtnGhost, th.Accent, false, th).Clicked() {
					s.NewChat()
				}
			}
		})

		// Active destination viewport.
		ui.Column(c.Key("and-body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Clip().Children(func() {
			switch tab {
			case mobile.TabChat:
				s.renderChat(c, th)
			case mobile.TabRooms:
				s.renderRooms(c, th)
			case mobile.TabHome:
				s.renderHome(c, th)
			case mobile.TabSearch:
				s.renderSearch(c, th)
			case mobile.TabSettings:
				s.renderSettings(c, th)
			case mobile.TabGates:
				s.renderGates(c, th)
			}
		})

		// Material 3 Bottom Navigation Bar (tonal active pill indicator).
		ui.Row(c.Key("and-bottom-nav")).Height(62).Padding(6, 8).Gap(4).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			for _, spec := range mobile.Tabs {
				t := spec
				active := tab == t.Tab || (tab == mobile.TabGates && t.Tab == mobile.TabHome)
				btn := kit.ActionControl(ui.ButtonBase(c.Key("and-nav-"+t.Title)).Label(t.Title).
					Basis(0).Grow(1).MinWidth(0).Padding(4, 2).Gap(3).
					Column().AlignItems(ui.Center).Justify(ui.Center))
				btn.Children(func() {
					pillBg := ui.Transparent
					iconCol := th.InkMuted
					lblCol := th.InkMuted
					lblW := 500
					if active {
						pillBg = kit.WithAlpha(th.Accent, 0.20)
						iconCol = th.Accent
						lblCol = th.Ink
						lblW = 700
					}
					ui.Row(c).Width(48).Height(26).Radius(13).Background(pillBg).
						AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						theme.SFIcon(c, t.Symbol, 15, iconCol, active)
					})
					ui.Text(c, t.Title).Font(kit.FontDisplay).FontSize(11).FontWeight(lblW).TextColor(lblCol).SingleLine()
				})
				if btn.Clicked() {
					s.SelectTab(t.Tab)
				}
			}
		})
	})
}

func (s *Shell) renderChat(c *ui.Context, th theme.Theme) {
	var tr *mygoshell.TranscriptController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		tr = s.Adaptive.Desktop.Transcript
	}
	s.mu.Lock()
	draft := s.chatDraft
	deep := s.deepThink
	s.mu.Unlock()

	mobile.RenderMobileChatStream(
		c, "and", th, tr, materialCardRadius, &draft, deep,
		func(txt string) { s.SendPrompt(txt) },
		func() { s.toggleDeepThink() },
	)

	s.mu.Lock()
	s.chatDraft = draft
	s.mu.Unlock()
}

func (s *Shell) renderRooms(c *ui.Context, th theme.Theme) {
	snap := featurekit.AgentRoomSnapshot()
	s.mu.Lock()
	draft := s.roomDraft
	s.mu.Unlock()

	mobile.RenderMobileRoomView(
		c, "and", th, snap, materialCardRadius, &draft,
		func(chID string) {
			_ = featurekit.SelectAgentRoomChannel(chID)
			s.invalidate()
		},
		func(chID, text string) {
			_ = featurekit.PostAgentRoomMessage(chID, text, "")
			s.invalidate()
		},
		func(msgID, tag string) {
			_ = featurekit.ToggleAgentRoomReaction(msgID, tag)
			s.invalidate()
		},
		func(taskID string) {
			_ = featurekit.CycleAgentRoomTask(taskID)
			s.invalidate()
		},
	)

	s.mu.Lock()
	s.roomDraft = draft
	s.mu.Unlock()
}

func (s *Shell) renderHome(c *ui.Context, th theme.Theme) {
	s.mu.Lock()
	schemeExts := append([]schema.UIExtensionRegion(nil), s.schemeExts...)
	schemeStatus := append([]mapper.ResolvedStatusItem(nil), s.schemeStatus...)
	activeModel := s.currentModelLabelLocked()
	deep := s.deepThink
	s.mu.Unlock()

	turns := 0
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.Transcript != nil {
		turns = s.Adaptive.Desktop.Transcript.Len()
	}
	roomStats := featurekit.AgentRoomStats()
	gateCount := s.pendingGateCount()
	cardBg := th.SurfaceElevated
	cardBorder := kit.WithAlpha(th.BorderSubtle, 0.55)

	ui.Scroll(c.Key("and-home-scroll")).Grow(1).MinHeight(0).Padding(12, 16, 18, 16).Children(func() {
		ui.Column(c.Key("and-home-col")).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// Workspace summary card.
			ui.Row(c.Key("and-home-hero")).Padding(16).Gap(12).Radius(materialCardRadius).
				Background(cardBg).Border(1, cardBorder).AlignItems(ui.Center).Children(func() {
				theme.MarlinEmblem(c, "and-home-emblem", 44, th)
				ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "MARLIN WORKSPACE").Font(theme.FontMono).FontSize(10).FontWeight(700).TextColor(th.Accent).SingleLine()
					ui.Text(c, mobile.ShortModelName(activeModel)).Font(kit.FontDisplay).FontSize(16).FontWeight(700).TextColor(th.Ink).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
					ui.Text(c, fmt.Sprintf("%d turns · %d agents online · %d tasks", turns, roomStats.AgentsOnline, roomStats.TasksTotal)).
						Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).SingleLine()
				})
			})

			// Quick action cards: Approvals + Deep Think.
			ui.Row(c.Key("and-home-tiles")).Gap(10).AlignItems(ui.Stretch).Children(func() {
				gateLbl := "All Clear"
				gateTone := th.Success
				if gateCount > 0 {
					gateLbl = fmt.Sprintf("%d Pending", gateCount)
					gateTone = th.Warning
				}
				if theme.StyledButton(c, theme.ButtonSpec{
					Key: "and-home-gates", Label: "Gates: " + gateLbl, Icon: "checkmark.shield", Variant: theme.BtnTinted, Size: theme.BtnSizeMD, Tone: gateTone, FullWidth: true,
				}, th).Clicked() {
					s.SelectTab(mobile.TabGates)
				}
				dtLbl := "Deep Think: Off"
				dtTone := th.InkMuted
				if deep {
					dtLbl = "Deep Think: On"
					dtTone = th.Accent
				}
				if theme.StyledButton(c, theme.ButtonSpec{
					Key: "and-home-dt", Label: dtLbl, Icon: "sparkles", Variant: theme.BtnTinted, Size: theme.BtnSizeMD, Tone: dtTone, FullWidth: true,
				}, th).Clicked() {
					s.toggleDeepThink()
				}
			})

			// Shared live Scheme UIExtensionRegions + Mobile Status Items!
			mobile.RenderSchemeExtensionsCard(
				c, "and", th, cardBg, cardBorder, materialCardRadius,
				activeModel, s.AppCfg.SessionID, schemeExts, schemeStatus,
				func(name string) {
					if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.OnCommandInvoke != nil {
						s.Adaptive.Desktop.OnCommandInvoke(name)
					}
					s.invalidate()
				},
			)
		})
	})
}

func (s *Shell) renderSearch(c *ui.Context, th theme.Theme) {
	var helm *mygoshell.HelmMinibufferController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		helm = s.Adaptive.Desktop.HelmSearch
	}
	s.mu.Lock()
	q := s.searchQuery
	s.mu.Unlock()

	mobile.RenderMobileSearchView(c, "and", th, helm, &q, func(ref string) {
		s.mu.Lock()
		s.chatDraft = strings.TrimSpace(s.chatDraft + " @cite:" + ref)
		s.mu.Unlock()
		mygo.Haptics.Success()
		s.SelectTab(mobile.TabChat)
	})

	s.mu.Lock()
	s.searchQuery = q
	s.mu.Unlock()
}

func (s *Shell) renderSettings(c *ui.Context, th theme.Theme) {
	s.mu.Lock()
	catalog := append([]theme.Theme(nil), s.ThemeCatalog...)
	deep := s.deepThink
	s.mu.Unlock()

	ui.Scroll(c.Key("and-settings-scroll")).Grow(1).MinHeight(0).Padding(12, 0, 20, 0).Children(func() {
		ui.Column(c.Key("and-settings-col")).Gap(16).AlignItems(ui.Stretch).Children(func() {
			mobile.RenderThemeSwatchGrid(c, "and", th, catalog, materialCardRadius, s.SetTheme)

			mygomobile.Section(c, "REASONING", func() {
				mygomobile.ListRow(c.Key("and-deep-row"), mygomobile.RowOptions{
					Title:    "Deep Think",
					Subtitle: "Adaptive extended reasoning trace",
					Trailing: func() {
						sw := mygomobile.Switch(c.Key("and-deep-sw"), &deep)
						if sw.Changed() {
							s.setDeepThink(deep)
						}
					},
				})
			})
		})
	})
}

func (s *Shell) renderGates(c *ui.Context, th theme.Theme) {
	var pg *mygoshell.PromptGateController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		pg = s.Adaptive.Desktop.PromptGate
	}
	ui.Scroll(c.Key("and-gates-scroll")).Grow(1).MinHeight(0).Padding(12, 16, 20, 16).Children(func() {
		mobile.RenderGateReviewCard(c, "and", th, pg, materialCardRadius, s.invalidate)
	})
}

func tabTitle(t mobile.Tab) string {
	switch t {
	case mobile.TabChat:
		return "Chat"
	case mobile.TabRooms:
		return "Agent Rooms"
	case mobile.TabHome:
		return "Bento"
	case mobile.TabSearch:
		return "Search"
	case mobile.TabSettings:
		return "Style"
	case mobile.TabGates:
		return "Approvals"
	default:
		return "Marlin"
	}
}
