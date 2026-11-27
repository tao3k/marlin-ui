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

	"github.com/tao3k/marlin-mygo/engine/uistream"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func (s *Shell) renderChatView(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	var tr *mygoshell.TranscriptController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		tr = s.Adaptive.Desktop.Transcript
	}
	turnCount := 0
	inFlight := false
	if tr != nil {
		turnCount = tr.Len()
		inFlight = tr.InFlight()
	}

	bottomReserve := float32(14)
	if hasFloatingTabBar {
		bottomReserve = 82
	}

	ui.Column(c.Key("ios-chat-col")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		if turnCount == 0 {
			s.renderChatHero(c, th, inFlight)
		} else {
			s.renderChatTranscript(c, th, tr, inFlight)
		}

		// Floating Liquid Glass Capsule Composer docked smoothly above the Tab Bar.
		s.renderGlassComposer(c, th, inFlight, bottomReserve)
	})
}

func (s *Shell) renderChatHero(c *ui.Context, th theme.Theme, _ bool) {
	ui.Scroll(c.Key("ios-hero-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(16, 18, 12, 18).Children(func() {
		ui.Column(c.Key("ios-hero-col")).Gap(22).AlignItems(ui.Stretch).Children(func() {
			// Centerpiece Sculpted Liquid-Glass Marlin Emblem + Editorial Greeting.
			ui.Column(c.Key("ios-hero-top")).Padding(14, 8, 2, 8).Gap(12).AlignItems(ui.Center).Children(func() {
				theme.MarlinEmblem(c, "ios-hero-marlin", 80, th)
				ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "What can we build today?").
						Font(kit.FontDisplay).FontSize(22).FontWeight(700).LetterSpacing(-0.4).TextColor(th.Ink).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
					ui.Text(c, "Marlin Studio · "+shortModelName(s.currentModelLabel())).
						Font(kit.FontDisplay).FontSize(12.5).FontWeight(500).TextColor(th.InkMuted).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
				})
			})

			// 2x2 Apple HIG Editorial Liquid Glass Starter Grid with native SF Symbols.
			starters := [2][2]struct {
				id     string
				icon   string
				title  string
				sub    string
				prompt string
				tone   ui.Color
			}{
				{
					{"sum", "doc.text.magnifyingglass", "Summarize", "Workspace overview", "Summarize the current workspace status and recent edits.", th.Accent},
					{"health", "waveform.path.ecg", "Host Health", "Routes & diagnostics", "Check Marlin host health, active model route, and open tasks.", th.Success},
				},
				{
					{"tasks", "checklist", "Open Tasks", "Review priorities", "Review open tasks and suggest the next highest-leverage step.", th.Info},
					{"search", "magnifyingglass", "Helm Search", "Files & symbols", "", th.Warning},
				},
			}

			ui.Column(c.Key("ios-hero-grid")).Gap(11).AlignItems(ui.Stretch).Children(func() {
				for rIdx, row := range starters {
					ui.Row(c.Key(fmt.Sprintf("ios-hero-r%d", rIdx))).Gap(11).AlignItems(ui.Stretch).Children(func() {
						for _, card := range row {
							cd := card
							btn := kit.ActionControl(ui.ButtonBase(c.Key("ios-start-"+cd.id)).Label(cd.title).
								Basis(0).Grow(1).MinWidth(0).Padding(14, 14).Radius(20).Gap(8).
								Column().AlignItems(ui.Start).Justify(ui.Start).
								Material(glass.Glass{
									Style:       glass.Regular,
									Tint:        kit.WithAlpha(th.SurfaceElevated, 0.34),
									Interactive: true,
								}).
								Shadow(0, 6, 18, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.24)))
							if !c.Vibrancy() {
								btn.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.12))
							}
							btn.Children(func() {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
									ui.Row(c).Width(30).Height(30).Radius(10).AlignItems(ui.Center).Justify(ui.Center).
										Background(kit.WithAlpha(cd.tone, 0.16)).
										Border(1, kit.WithAlpha(cd.tone, 0.28)).Children(func() {
										sfIcon(c, cd.icon, 14, cd.tone, false)
									})
									sfIcon(c, "arrow.up.right", 10.5, th.InkMuted, false)
								})
								ui.Column(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
									ui.Text(c, cd.title).Font(kit.FontDisplay).FontSize(14).FontWeight(600).TextColor(th.Ink).
										SingleLine().Ellipsis("…").MinWidth(0).Clip()
									ui.Text(c, cd.sub).Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).
										SingleLine().Ellipsis("…").MinWidth(0).Clip()
								})
							})
							if btn.Clicked() {
								if cd.prompt == "" {
									s.SelectTab(TabSearch)
								} else {
									s.sendPrompt(cd.prompt)
								}
							}
						}
					})
				}
			})
		})
	})
}

func (s *Shell) renderChatTranscript(c *ui.Context, th theme.Theme, tr *mygoshell.TranscriptController, inFlight bool) {
	n := tr.Len()
	start := 0
	if n > 80 {
		start = n - 80
	}

	ui.Scroll(c.Key("ios-chat-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(8, 14, 14, 14).Children(func() {
		ui.Column(c.Key("ios-chat-feed")).Gap(12).AlignItems(ui.Stretch).Children(func() {
			for i := start; i < n; i++ {
				entry, ok := tr.Entry(i)
				if !ok {
					continue
				}
				switch entry.Role {
				case mygoshell.RoleUser:
					ui.Row(c.Key(fmt.Sprintf("ios-msg-u-%d", i))).Justify(ui.End).Children(func() {
						ui.Column(c).MaxWidth(296).Padding(10, 14).Radius(20).
							Background(kit.WithAlpha(th.Accent, 0.24)).
							Border(1, kit.WithAlpha(th.Accent, 0.40)).Children(func() {
							ui.Text(c, strings.TrimSpace(entry.Text)).
								Font(kit.FontDisplay).FontSize(14).LineHeight(1.45).TextColor(th.Ink)
						})
					})
				case mygoshell.RoleThinking:
					key := fmt.Sprintf("think-%d", i)
					s.mu.Lock()
					open := s.thinkingOpen[key]
					s.mu.Unlock()
					prevOpen := open
					body := strings.TrimSpace(entry.Text)
					ui.Collapsible(c.Key("ios-"+key), "Thought process", &open, func() {
						if body != "" {
							ui.Text(c, body).Font(theme.FontMono).FontSize(11.5).LineHeight(1.45).TextColor(th.InkMuted)
						}
					})
					if open != prevOpen {
						mygo.Haptics.Selection()
						s.mu.Lock()
						s.thinkingOpen[key] = open
						s.mu.Unlock()
					}
				default:
					thinkText, rawBody, _ := uistream.SplitThinkTags(entry.Text)
					dataParts, bodyText := uistream.ExtractDataParts(rawBody)
					ui.Column(c.Key(fmt.Sprintf("ios-msg-a-%d", i))).Padding(12, 14).Radius(18).Gap(8).
						Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.70)).
						Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.09)).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							sfIcon(c, "sparkles", 12, th.Accent, true)
							ui.Text(c, "Marlin").Font(kit.FontDisplay).FontSize(11.5).FontWeight(700).TextColor(th.Accent).
								SingleLine().Ellipsis("…").MinWidth(0).Clip()
						})
						if strings.TrimSpace(thinkText) != "" {
							tKey := fmt.Sprintf("ios-inline-think-%d", i)
							s.mu.Lock()
							open := s.thinkingOpen[tKey]
							s.mu.Unlock()
							prev := open
							ui.Collapsible(c.Key(tKey), "Thought process", &open, func() {
								ui.Text(c, strings.TrimSpace(thinkText)).Font(theme.FontMono).FontSize(11.5).LineHeight(1.45).TextColor(th.InkMuted)
							})
							if open != prev {
								s.mu.Lock()
								s.thinkingOpen[tKey] = open
								s.mu.Unlock()
							}
						}
						if strings.TrimSpace(bodyText) != "" {
							mdOpts := &markdown.Options{
								Theme: &markdown.Theme{
									Font:                kit.FontDisplay,
									CodeFont:            theme.FontMono,
									FontSize:            14,
									LineHeight:          1.52,
									CodeLineHeight:      1.42,
									Text:                th.Ink,
									Muted:               th.InkMuted,
									Link:                th.Info,
									Strong:              th.Ink,
									StrongWeight:        600,
									Code:                th.Accent,
									CodeBackground:      kit.WithAlpha(th.Bar, 0.72),
									CodeSize:            0.88,
									BlockSpacing:        0.60,
									ItemSpacing:         0.24,
									QuoteBar:            th.Accent,
									QuoteText:           th.InkMuted,
									QuoteBarWidth:       3,
									Rule:                th.BorderSubtle,
									CodeBlockBackground: kit.WithAlpha(th.Bar, 0.85),
									CodeBlockBorder:     th.BorderSubtle,
									CodeBlockRadius:     10,
									CodeBlockPadding:    0.75,
									TableBorder:         th.BorderSubtle,
									TableHeader:         kit.WithAlpha(th.Bar, 0.70),
									TableStripe:         kit.WithAlpha(th.SurfaceElevated, 0.25),
									Checkbox:            th.Success,
								},
								WrapCode: true,
							}
							if entry.Streaming {
								markdown.Stream(c.Key(fmt.Sprintf("ios-md-%d", i)), strings.TrimSpace(bodyText), mdOpts)
							} else {
								markdown.View(c.Key(fmt.Sprintf("ios-md-%d", i)), strings.TrimSpace(bodyText), mdOpts)
							}
						}
						for dIdx, dp := range dataParts {
							ui.Column(c.Key(fmt.Sprintf("ios-gen-%d-%d", i, dIdx))).Padding(10, 12).Radius(14).Gap(6).
								Background(kit.WithAlpha(th.Bar, 0.56)).
								Border(1, kit.WithAlpha(th.Accent, 0.34)).Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Badge(c, "AI SDK · "+dp.DataType)
								})
								if p := strings.TrimSpace(string(dp.Data)); p != "" {
									ui.Text(c, p).Font(theme.FontMono).FontSize(11).LineHeight(1.42).TextColor(th.InkMuted).MaxLines(5)
								}
							})
						}
					})
				}
			}
			if inFlight {
				ui.Row(c.Key("ios-chat-streaming")).Padding(6, 10).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Size(14, 14)
					ui.Text(c, "Marlin is thinking…").Font(kit.FontDisplay).FontSize(12.5).TextColor(th.Info).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
				})
			}
			if tr != nil && tr.Resilience != nil {
				rs := tr.Resilience.Snapshot("chat:" + s.AppCfg.SessionID)
				if strings.TrimSpace(rs.LastError) != "" && string(rs.State) != "online" {
					ui.Row(c.Key("ios-chat-err")).Padding(10, 12).Radius(14).Gap(8).AlignItems(ui.Center).
						Background(kit.WithAlpha(th.Danger, 0.14)).
						Border(1, kit.WithAlpha(th.Danger, 0.42)).Children(func() {
						sfIcon(c, "exclamationmark.triangle", 14, th.Danger, true)
						ui.Text(c, rs.LastError).Font(kit.FontDisplay).FontSize(12).TextColor(th.Ink).
							Basis(0).Grow(1).MinWidth(0)
					})
				}
			}
		})
	})
}

func (s *Shell) renderGlassComposer(c *ui.Context, th theme.Theme, inFlight bool, bottomReserve float32) {
	s.mu.Lock()
	draft := s.chatDraft
	deep := s.deepThink
	s.mu.Unlock()

	// Apple iOS 26 HIG Detached Composer: standalone floating Liquid Glass
	// Deep Think circle beside a pure Liquid Glass input capsule (zero
	// glass-on-glass nesting, concentric inner radius, bold solid Send CTA).
	ui.Row(c.Key("ios-composer-wrap")).Padding(6, 14, bottomReserve, 14).Gap(8).AlignItems(ui.Center).Children(func() {
		thinkVariant := theme.BtnGlass
		thinkTone := th.InkMuted
		if deep {
			thinkVariant = theme.BtnTinted
			thinkTone = th.Accent
		}
		if theme.IconCircleButton(c, "ios-comp-think", "Deep Think", theme.IconSparkles, 40, thinkVariant, thinkTone, deep, th).Clicked() {
			s.toggleDeepThink()
		}

		box := ui.Row(c.Key("ios-composer-capsule")).Grow(1).MinWidth(0).Height(42).Padding(4, 5, 4, 14).Gap(6).
			Radius(21).AlignItems(ui.Center).
			Material(glass.Glass{
				Style:       glass.Regular,
				Tint:        kit.WithAlpha(th.SurfaceElevated, 0.34),
				Interactive: false,
			}).
			Shadow(0, 6, 18, -4, kit.WithAlpha(ui.RGB(0, 0, 0), 0.28))
		if !c.Vibrancy() {
			box.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
		}

		box.Children(func() {
			inp := ui.TextInputBase(c.Key("ios-comp-input"), &draft).
				Placeholder("Ask Marlin…").
				Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
			if inp.Changed() {
				s.mu.Lock()
				s.chatDraft = draft
				s.mu.Unlock()
			}
			if inp.Submitted() && strings.TrimSpace(draft) != "" {
				s.sendPrompt(draft)
			}

			// Quiet borderless @ Helm citation trigger inside the glass pill.
			if theme.IconCircleButton(c, "ios-comp-cite", "Search and cite", theme.IconCite, 30, theme.BtnGhost, th.InkMuted, false, th).Clicked() {
				s.SelectTab(TabSearch)
			}

			// Concentric 32pt Send / Stop circular CTA (borderless solid idle well
			// -> luminous sapphire prominent CTA -> coral stop square).
			canSend := strings.TrimSpace(draft) != ""
			sendVariant := theme.BtnSolidIdle
			sendTone := th.InkMuted
			sendIcon := theme.IconSend
			if inFlight {
				sendVariant = theme.BtnGlassProminent
				sendTone = th.Danger
				sendIcon = theme.IconStop
			} else if canSend {
				sendVariant = theme.BtnGlassProminent
				sendTone = th.Accent
			}
			if theme.IconCircleButton(c, "ios-comp-send", "Send", sendIcon, 32, sendVariant, sendTone, inFlight, th).Clicked() {
				if inFlight && s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.OnCancelTurn != nil {
					mygo.Haptics.Warning()
					s.Adaptive.Desktop.OnCancelTurn()
				} else if canSend {
					s.sendPrompt(draft)
				}
			}
		})
	})
}
