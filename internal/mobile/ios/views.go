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
	"github.com/egoist/mygo/ui/mobile"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
	mob "github.com/tao3k/marlin-ui/internal/mobile"
)

// renderBentoHome renders an authentic iOS 26 WidgetKit × Control Center Bento
// Dashboard: proportional rounded Liquid Glass widgets, live Activity Ring +
// multi-agent telemetry, interactive Control Center Deep Think & Approvals
// tiles, and live 1-tap channel tasks (zero redundant 2019 tab-duplication list).
func (s *Shell) renderBentoHome(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	pal := th.ToKitPalette()
	s.mu.Lock()
	motionOn := s.MotionOn
	deep := s.deepThink
	schemeExts := append([]schema.UIExtensionRegion(nil), s.schemeExts...)
	schemeStatus := append([]mapper.ResolvedStatusItem(nil), s.schemeStatus...)
	activeModel := s.activeModel
	s.mu.Unlock()
	padBot := float32(18)
	if hasFloatingTabBar {
		padBot = 90
	}

	turns := 0
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.Transcript != nil {
		turns = s.Adaptive.Desktop.Transcript.Len()
	}
	hostLbl := "Standalone Preview"
	hostCol := th.Warning
	if s.Link != nil && s.Link.Mode != hostlink.ModeStandalone && strings.TrimSpace(s.Link.Endpoint) != "" {
		hostLbl = s.Link.Endpoint
		hostCol = th.Success
	}
	roomStats := featurekit.AgentRoomStats()
	roomTasks := featurekit.AgentRoomTasks()
	gateCount := s.pendingGateCount()

	widgetFill := theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.66)
	widgetHairline := kit.WithAlpha(ui.RGB(255, 255, 255), 0.08)
	if th.IsLight {
		widgetHairline = kit.WithAlpha(ui.RGB(0, 0, 0), 0.07)
	}

	ui.Scroll(c.Key("ios-bento-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(4, 16, padBot, 16).Children(func() {
		ui.Column(c.Key("ios-bento-col")).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// Row 1: Full-width iOS 26 Workspace Hero Glass Widget with 1-tap New Chat CTA.
			hero := ui.Row(c.Key("ios-bento-hero")).Padding(16).Gap(14).Radius(24).AlignItems(ui.Center).
				Material(glass.Glass{
					Style: glass.Regular,
					Tint:  kit.WithAlpha(th.SurfaceElevated, 0.32),
				}).
				Shadow(0, 8, 22, -8, kit.WithAlpha(ui.RGB(0, 0, 0), 0.28))
			if !c.Vibrancy() {
				hero.Border(1, widgetHairline)
			}
			hero.Children(func() {
				theme.MarlinEmblem(c, "ios-bento-marlin", 52, th)
				ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Width(7).Height(7).Radius(3.5).Background(hostCol)
						ui.Text(c, "WORKSPACE").Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.Accent).SingleLine()
					})
					ui.Text(c, shortModelName(s.currentModelLabel())).Font(kit.FontDisplay).FontSize(17).FontWeight(700).TextColor(th.Ink).
						SingleLine().Ellipsis("…").MinWidth(0).Clip()
					ui.Text(c, hostLbl).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).
						SingleLine().Ellipsis("…").TruncateMiddle().MinWidth(0).Clip()
				})
				if theme.IconCircleButton(c, "ios-bento-new-chat", "New Chat", "square.and.pencil", 40, theme.BtnGlassProminent, th.Accent, false, th).Clicked() {
					s.NewChat()
				}
			})

			// Row 2: 2-Column Proportional iOS 26 Metric Widgets (Activity Ring + Live Agents).
			ui.Row(c.Key("ios-bento-metrics")).Gap(12).Height(132).AlignItems(ui.Stretch).Children(func() {
				// Left Widget: Live Conversation Turns Activity Ring.
				ringCard := ui.Column(c.Key("ios-bento-turns-card")).Basis(0).Grow(1).MinWidth(0).Radius(24).
					Background(widgetFill).Border(1, widgetHairline)
				ringCard.Children(func() {
					kit.RingGauge(c, "ios-ring-turns", "chat", "Turns", fmt.Sprintf("%d", turns), float32(min(turns+2, 10))/10.0, th.Accent, pal, motionOn, func() {
						s.SelectTab(TabChat)
					})
				})

				// Right Widget: Live AI Agents & Room Pulse (taps straight into Rooms).
				agentCard := kit.ActionControl(ui.ButtonBase(c.Key("ios-bento-agents-card")).Label("Agent Rooms").
					Basis(0).Grow(1).MinWidth(0).Padding(14).Radius(24).Gap(8).
					Column().AlignItems(ui.Start).Justify(ui.SpaceBetween).
					Background(widgetFill).Border(1, widgetHairline))
				agentCard.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						ui.Row(c).Gap(-5).AlignItems(ui.Center).Children(func() {
							for _, spec := range []struct {
								id, sym string
								tone    ui.Color
							}{
								{"ar", "building.columns", th.Accent},
								{"rv", "checkmark.seal", th.Success},
								{"sc", "shield.lefthalf.filled", th.Warning},
							} {
								ui.Box(c.Key("ios-b-em-"+spec.id)).Width(24).Height(24).Radius(12).
									AlignItems(ui.Center).Justify(ui.Center).
									Background(kit.WithAlpha(spec.tone, 0.24)).
									Border(1.5, th.Canvas).Children(func() {
									sfIcon(c, spec.sym, 11, spec.tone, true)
								})
							}
						})
						sfIcon(c, "arrow.up.right", 11, th.InkMuted, false)
					})
					ui.Column(c).Gap(2).AlignItems(ui.Start).Children(func() {
						ui.Text(c, fmt.Sprintf("%d Online", roomStats.AgentsOnline)).
							Font(kit.FontDisplay).FontSize(19).FontWeight(700).TextColor(th.Ink).SingleLine()
						ui.Text(c, fmt.Sprintf("AI Agents · %d tasks", roomStats.TasksTotal)).
							Font(kit.FontDisplay).FontSize(11.5).TextColor(th.InkMuted).SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
				})
				if agentCard.Clicked() {
					s.SelectTab(TabRooms)
				}
			})

			// Row 3: 2-Column Apple iOS 26 Control Center Interactive Tiles.
			ui.Row(c.Key("ios-bento-controls")).Gap(12).AlignItems(ui.Stretch).Children(func() {
				// Left Tile: Tier-0 Approvals Gate Control Tile.
				gateTone := th.Success
				gateTitle := "All Clear"
				gateSub := "Tier-0 Gates"
				if gateCount > 0 {
					gateTone = th.Warning
					gateTitle = fmt.Sprintf("%d Pending", gateCount)
					gateSub = "Tap to approve"
				}
				gateTile := kit.ActionControl(ui.ButtonBase(c.Key("ios-bento-gate-tile")).Label("Approvals").
					Basis(0).Grow(1).MinWidth(0).Padding(14).Radius(22).Gap(10).
					Row().AlignItems(ui.Center).
					Background(kit.WithAlpha(gateTone, 0.14)).
					Border(1, kit.WithAlpha(gateTone, 0.28)))
				gateTile.Children(func() {
					ui.Box(c).Width(36).Height(36).Shrink(0).Radius(18).AlignItems(ui.Center).Justify(ui.Center).
						Background(kit.WithAlpha(gateTone, 0.22)).Children(func() {
						sfIcon(c, "checkmark.shield", 16, gateTone, true)
					})
					ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, gateTitle).Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(th.Ink).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
						ui.Text(c, gateSub).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
				})
				if gateTile.Clicked() {
					s.SelectTab(TabGates)
				}

				// Right Tile: Live iOS 26 Control Center Deep Think Toggle Tile.
				dtBg := widgetFill
				dtBorder := widgetHairline
				dtIconBg := kit.WithAlpha(th.Ink, 0.10)
				dtIconCol := th.InkMuted
				dtState := "Off"
				if deep {
					dtBg = kit.WithAlpha(th.Accent, 0.22)
					dtBorder = kit.WithAlpha(th.Accent, 0.44)
					dtIconBg = th.Accent
					dtIconCol = ui.RGB(255, 255, 255)
					dtState = "Active"
				}
				dtTile := kit.ActionControl(ui.ButtonBase(c.Key("ios-bento-dt-tile")).Label("Deep Think").
					Basis(0).Grow(1).MinWidth(0).Padding(14).Radius(22).Gap(10).
					Row().AlignItems(ui.Center).
					Background(dtBg).Border(1, dtBorder))
				dtTile.Children(func() {
					ui.Box(c).Width(36).Height(36).Shrink(0).Radius(18).AlignItems(ui.Center).Justify(ui.Center).
						Background(dtIconBg).Children(func() {
						sfIcon(c, "sparkles", 16, dtIconCol, deep)
					})
					ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, "Deep Think").Font(kit.FontDisplay).FontSize(13.5).FontWeight(700).TextColor(th.Ink).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
						ui.Text(c, dtState).Font(kit.FontDisplay).FontSize(11).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
				})
				if dtTile.Clicked() {
					s.toggleDeepThink()
				}
			})

			// Row 4: Interactive WidgetKit Live Channel Tasks Card (1-tap completion).
			ui.Column(c.Key("ios-bento-tasks-widget")).Padding(16, 16, 12, 16).Gap(10).Radius(24).
				Background(widgetFill).Border(1, widgetHairline).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						sfIcon(c, "checklist", 14, th.Info, false)
						ui.Text(c, "LIVE TASKS").Font(theme.FontMono).FontSize(10.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).SingleLine()
					})
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "ios-bento-open-rooms", Label: "Open Rooms", Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.Accent, Pill: true,
					}, th).Clicked() {
						s.SelectTab(TabRooms)
					}
				})
				if len(roomTasks) == 0 {
					ui.Text(c, "All workspace tasks complete.").Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted).Padding(4, 2)
				} else {
					for i, tk := range roomTasks {
						if i >= 4 {
							break
						}
						item := tk
						done := item.Status == "DONE"
						iconCol := th.Info
						sym := "circle"
						if done {
							iconCol = th.Success
							sym = "checkmark.circle"
						} else if item.Status == "IN_PROGRESS" {
							iconCol = th.Warning
							sym = "clock"
						}
						if i > 0 {
							ui.Box(c).Height(1).FillWidth().Background(kit.WithAlpha(th.BorderSubtle, 0.45))
						}
						row := kit.ActionControl(ui.ButtonBase(c.Key("ios-btk-"+item.ID)).Label(item.Title).
							FillWidth().Padding(7, 2).Gap(10).Radius(10).AlignItems(ui.Center))
						row.Children(func() {
							sfIcon(c, sym, 17, iconCol, done)
							ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(1).Children(func() {
								ui.Text(c, item.Title).Font(kit.FontDisplay).FontSize(13.5).FontWeight(500).TextColor(th.Ink).
									SingleLine().Ellipsis("…").MinWidth(0).Clip()
								ui.Text(c, "@"+strings.TrimPrefix(item.Owner, "@")+" · "+item.Status).
									Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).SingleLine()
							})
						})
						if row.Clicked() {
							mygo.Haptics.Selection()
							_ = featurekit.CycleAgentRoomTask(item.ID)
							s.invalidate()
						}
					}
				}
			})

			// Row 5 (Optional): Live Scheme UIExtensionRegions + Mobile Status Items
			// streamed over marlin.ui.v1 and rendered via shared internal/mobile.
			mob.RenderSchemeExtensionsCard(
				c, "ios", th, widgetFill, widgetHairline, 24,
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

// renderMobileSearch renders an authentic iOS 26 Spotlight Search experience:
// clean edge-to-edge hairlined result list (zero clunky 2019 boxed cards per
// hit), semantic SF Symbol source badges, and a thumb-zone bottom-anchored
// floating Liquid Glass search bar right above the iOS 26 Tab Bar + Search Orb.
func (s *Shell) renderMobileSearch(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	padBot := float32(14)
	if hasFloatingTabBar {
		padBot = 82
	}
	s.mu.Lock()
	query := s.searchQuery
	selected := s.selectedHit
	s.mu.Unlock()

	var eng *search.HelmEngine
	var helm *mygoshell.HelmMinibufferController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil && s.Adaptive.Desktop.HelmSearch != nil {
		helm = s.Adaptive.Desktop.HelmSearch
		eng = helm.Engine
	}

	var rows []search.VisibleRow
	if eng != nil {
		eng.SetViewportRows(16)
		liveRows, _, liveTotal, _, _, liveGen := eng.VisibleWindow(s.helmRowsBuf[:0])
		s.helmRowsBuf = liveRows
		liveSrcs := eng.SourceSummaries(nil)
		hasPrim := false
		for _, sm := range liveSrcs {
			if (sm.ID == "rust-graph" || sm.ID == "rust-dsl") && sm.Count > 0 {
				hasPrim = true
				break
			}
		}
		rustConn := s.Link != nil && s.Link.Mode != hostlink.ModeStandalone
		resolved, _ := s.helmLatch.Resolve(
			helmSnap{hadPrimary: hasPrim, rows: liveRows, srcs: liveSrcs, total: liveTotal},
			liveGen,
			len(liveRows) > 0,
			func(front, cur helmSnap) bool {
				return (rustConn || front.hadPrimary) && (len(cur.rows) == 0 || (front.hadPrimary && !cur.hadPrimary))
			},
			func(front, cur helmSnap) bool {
				return !rustConn || cur.hadPrimary
			},
			func(src, dst helmSnap) helmSnap {
				src.rows = append(dst.rows[:0], src.rows...)
				src.srcs = append(dst.srcs[:0], src.srcs...)
				return src
			},
		)
		rows = resolved.rows
	}

	ui.Column(c.Key("ios-search-col")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		// Top Scope Filter Pill Bar (lightweight floating pills, zero 2019 gray box).
		if eng != nil {
			scopes := []struct {
				id, label string
			}{
				{"", "All"},
				{"rust-graph", "Files"},
				{"buffers", "Buffers"},
				{"rust-commands", "Commands"},
			}
			activeFilter := eng.ActiveSourceFilter()
			ui.ScrollHorizontal(c.Key("ios-search-scopes")).FillWidth().Padding(0, 14, 8, 14).Gap(6).Children(func() {
				for _, sc := range scopes {
					item := sc
					sel := item.id == activeFilter
					v := theme.BtnGhost
					tone := th.InkMuted
					if sel {
						v = theme.BtnTinted
						tone = th.Accent
					}
					if theme.StyledButton(c, theme.ButtonSpec{
						Key: "ios-sscope-" + item.label, Label: item.label, Variant: v, Size: theme.BtnSizeSM, Tone: tone, Pill: true,
					}, th).Clicked() {
						mygo.Haptics.Selection()
						s.helmLatch.Reset()
						eng.SetSourceFilter(item.id)
						s.invalidate()
					}
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(kit.WithAlpha(th.BorderSubtle, 0.50))
		}

		// Edge-to-edge iOS 26 Spotlight candidate stream with subtle indented hairlines.
		ui.Scroll(c.Key("ios-search-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(4, 0, 8, 0).Children(func() {
			ui.Column(c.Key("ios-search-list")).AlignItems(ui.Stretch).Children(func() {
				if len(rows) == 0 {
					ui.Column(c).Padding(42, 24).Gap(8).AlignItems(ui.Center).Children(func() {
						sfIcon(c, "magnifyingglass", 28, th.InkMuted, false)
						ui.Text(c, "Search Workspace").Font(kit.FontDisplay).FontSize(16).FontWeight(600).TextColor(th.Ink)
						ui.Text(c, "Find Rust files, symbols, open buffers, and Marlin commands below.").
							Font(kit.FontDisplay).FontSize(12.5).TextColor(th.InkMuted)
					})
					return
				}
				for idx, r := range rows {
					if r.IsHeader {
						ui.Text(c, fmt.Sprintf("%s · %d", strings.ToUpper(r.SourceLabel), r.SourceCount)).
							Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).
							Padding(10, 16, 4, 16).SingleLine().Ellipsis("…").MinWidth(0).Clip()
						continue
					}
					cand := r.Candidate
					sub := cand.Location
					if sub == "" {
						sub = cand.Subtitle
					}
					sym, badgeCol := searchSourceVisual(cand.SourceID, th)
					isSelected := selected != nil && selected.ID == cand.ID
					row := kit.ActionControl(ui.ButtonBase(c.Key("ios-hrow-"+cand.ID)).Label(cand.Title).
						FillWidth().Padding(10, 16).Gap(11).AlignItems(ui.Center).Clip())
					if isSelected {
						row.Background(kit.WithAlpha(th.Accent, 0.14))
					} else if row.Hovered() || row.Pressed() {
						row.Background(kit.WithAlpha(th.SurfaceElevated, 0.36))
					}
					row.Children(func() {
						ui.Box(c).Width(32).Height(32).Shrink(0).Radius(9).AlignItems(ui.Center).Justify(ui.Center).
							Background(kit.WithAlpha(badgeCol, 0.16)).Children(func() {
							sfIcon(c, sym, 14, badgeCol, false)
						})
						ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, cand.Title).Font(kit.FontDisplay).FontSize(14).FontWeight(600).TextColor(th.Ink).
								SingleLine().Ellipsis("…").MinWidth(0).Clip()
							if sub != "" {
								ui.Text(c, sub).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).
									SingleLine().Ellipsis("…").TruncateMiddle().MinWidth(0).Clip()
							}
						})
						sfIcon(c, "chevron.right", 10.5, th.InkMuted, false)
					})
					if row.Clicked() {
						mygo.Haptics.Light()
						cp := cand
						s.mu.Lock()
						s.selectedHit = &cp
						s.mu.Unlock()
						s.invalidate()
					}
					if idx+1 < len(rows) && !rows[idx+1].IsHeader {
						ui.Box(c).Height(1).Padding(0, 16, 0, 59).Children(func() {
							ui.Box(c).Fill().Background(kit.WithAlpha(th.BorderSubtle, 0.42))
						})
					}
				}
			})
		})

		// Selected Candidate Floating Liquid Glass Preview Sheet above the search pill.
		if selected != nil {
			ui.Column(c.Key("ios-search-preview-wrap")).Padding(4, 14, 4, 14).Children(func() {
				prev := ui.Column(c.Key("ios-search-preview")).Padding(12, 14).Gap(8).Radius(20).
					Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.48)}).
					Shadow(0, 6, 18, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.30))
				if !c.Vibrancy() {
					prev.Border(1, kit.WithAlpha(th.Accent, 0.38))
				}
				prev.Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Clip().Children(func() {
						ui.Text(c, selected.Title).Font(theme.FontMono).FontSize(12.5).FontWeight(700).TextColor(th.Info).
							Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: "ios-cite-room", Label: "Room", Icon: "person.3", Variant: theme.BtnTinted, Size: theme.BtnSizeSM, Tone: th.Info, Pill: true,
						}, th).Clicked() {
							ref := selected.Location
							if ref == "" {
								ref = selected.Title
							}
							s.mu.Lock()
							s.roomDraft = strings.TrimSpace(s.roomDraft + " @cite:" + ref)
							s.selectedHit = nil
							s.mu.Unlock()
							mygo.Haptics.Success()
							s.SelectTab(TabRooms)
						}
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: "ios-cite-hit", Label: "Chat", Icon: theme.IconCite, Variant: theme.BtnGlassProminent, Size: theme.BtnSizeSM, Tone: th.Accent, Pill: true,
						}, th).Clicked() {
							ref := selected.Location
							if ref == "" {
								ref = selected.Title
							}
							s.mu.Lock()
							s.chatDraft = strings.TrimSpace(s.chatDraft + " @cite:" + ref)
							s.selectedHit = nil
							s.mu.Unlock()
							mygo.Haptics.Success()
							s.SelectTab(TabChat)
						}
					})
					if strings.TrimSpace(selected.Snippet) != "" {
						ui.Text(c, strings.TrimSpace(selected.Snippet)).Font(theme.FontMono).FontSize(11).
							TextColor(th.InkMuted).MaxLines(3)
					}
				})
			})
		}

		// iOS 26 Thumb-Zone Bottom-Anchored Floating Liquid Glass Search Capsule.
		ui.Row(c.Key("ios-search-dock")).Padding(6, 14, padBot, 14).AlignItems(ui.Center).Children(func() {
			pill := ui.Row(c.Key("ios-search-pill")).Grow(1).MinWidth(0).Height(44).Padding(0, 8, 0, 14).Gap(8).
				Radius(22).AlignItems(ui.Center).
				Material(glass.Glass{Style: glass.Regular, Tint: kit.WithAlpha(th.SurfaceElevated, 0.38)}).
				Shadow(0, 6, 18, -5, kit.WithAlpha(ui.RGB(0, 0, 0), 0.28))
			if !c.Vibrancy() {
				pill.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.14))
			}
			pill.Children(func() {
				sfIcon(c, "magnifyingglass", 15, th.InkMuted, false)
				inp := ui.TextInputBase(c.Key("ios-helm-input"), &query).
					Placeholder("Search files, symbols, commands…").
					Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
				if inp.Changed() {
					s.mu.Lock()
					s.searchQuery = query
					s.mu.Unlock()
					if helm != nil {
						gen := helm.SetQuery(query)
						s.helmLatch.BeginTransition(gen)
					}
				}
				if query != "" {
					if theme.IconCircleButton(c, "ios-search-clear", "Clear search", theme.IconClose, 26, theme.BtnSolidIdle, th.InkMuted, false, th).Clicked() {
						s.mu.Lock()
						s.searchQuery = ""
						s.mu.Unlock()
						if helm != nil {
							gen := helm.SetQuery("")
							s.helmLatch.BeginTransition(gen)
						}
						s.invalidate()
					}
				}
			})
		})
	})
}

func searchSourceVisual(sourceID string, th theme.Theme) (sym string, col ui.Color) {
	switch sourceID {
	case "rust-graph", "rust-dsl":
		return "doc.text", th.Accent
	case "rust-commands":
		return "terminal", th.Warning
	case "buffers":
		return "square.on.square", th.Success
	default:
		return "sparkles", th.Info
	}
}

func (s *Shell) renderGatesView(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	padBot := float32(18)
	if hasFloatingTabBar {
		padBot = 90
	}
	var pg *mygoshell.PromptGateController
	if s.Adaptive != nil && s.Adaptive.Desktop != nil {
		pg = s.Adaptive.Desktop.PromptGate
	}

	ui.Scroll(c.Key("ios-gates-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(4, 16, padBot, 16).Children(func() {
		ui.Column(c.Key("ios-gates-col")).Gap(12).AlignItems(ui.Stretch).Children(func() {
			mob.RenderGateReviewCard(c, "ios", th, pg, 24, s.invalidate)
		})
	})
}

// renderSettingsView renders an iOS 26 Style Studio with a 2-column visual
// Theme Swatch Bento Grid (previewing each palette's real Canvas, Surface, and
// Accent), Liquid Glass translucency control, Deep Think, and Model Routes.
func (s *Shell) renderSettingsView(c *ui.Context, th theme.Theme, hasFloatingTabBar bool) {
	padBot := float32(18)
	if hasFloatingTabBar {
		padBot = 90
	}
	s.mu.Lock()
	catalog := append([]theme.Theme(nil), s.ThemeCatalog...)
	curOp := s.GlassOpacity
	deep := s.deepThink
	s.mu.Unlock()

	ui.Scroll(c.Key("ios-settings-scroll")).Grow(1).MinWidth(0).MinHeight(0).Padding(4, 0, padBot, 0).Children(func() {
		ui.Column(c.Key("ios-settings-col")).Gap(16).AlignItems(ui.Stretch).Children(func() {
			// 1. Visual 2-Column Theme Swatch Bento Grid shared with Android.
			mob.RenderThemeSwatchGrid(c, "ios", th, catalog, 20, s.SetTheme)

			// 2. Liquid Glass Opacity Studio via native MyGo ui.Segmented.
			ui.Column(c.Key("ios-set-glass-wrap")).Padding(0, 16).Gap(6).AlignItems(ui.Stretch).Children(func() {
				ui.Text(c, fmt.Sprintf("LIQUID GLASS · %d%%", int(curOp*100+0.5))).
					Font(theme.FontMono).FontSize(10.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted)
				presets := []float32{0.65, 0.75, 0.85, 0.92}
				opIdx := 1
				bestDiff := float32(9)
				for i, v := range presets {
					d := curOp - v
					if d < 0 {
						d = -d
					}
					if d < bestDiff {
						bestDiff = d
						opIdx = i
					}
				}
				seg := ui.Segmented(c.Key("ios-op-seg"), &opIdx, "65%", "75%", "85%", "92%").
					FillEqually().Label("Liquid Glass Opacity")
				if seg.Changed() && opIdx >= 0 && opIdx < len(presets) {
					s.SetWindowOpacity(presets[opIdx])
				}
			})

			mobile.Section(c, "REASONING", func() {
				mobile.ListRow(c.Key("ios-deep-row"), mobile.RowOptions{
					Title:    "Deep Think",
					Subtitle: "Adaptive extended reasoning trace",
					Leading: func() {
						ui.Row(c).Width(28).Height(28).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
							Background(kit.WithAlpha(th.Accent, 0.16)).Children(func() {
							sfIcon(c, "sparkles", 13.5, th.Accent, deep)
						})
					},
					Trailing: func() {
						sw := mobile.Switch(c.Key("ios-deep-sw"), &deep)
						if sw.Changed() {
							s.setDeepThink(deep)
						}
					},
				})
			})

			// 3. Live Model Routes from TOML config via mobile.Section + ui.Badge.
			if routes := s.AppCfg.ModelCfg.Routes(); len(routes) > 0 {
				cur := s.currentModelLabel()
				mobile.Section(c, "MODEL ROUTES", func() {
					for _, m := range routes {
						route := m
						alias := route.Alias
						if alias == "" {
							alias = route.Model
						}
						act := strings.Contains(cur, alias) || strings.Contains(cur, route.Model)
						if mobile.ListRow(c.Key("ios-mod-"+alias), mobile.RowOptions{
							Title:    alias,
							Subtitle: route.Model,
							Trailing: func() {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									ui.Badge(c, route.Provider)
									if act {
										sfIcon(c, "checkmark.circle", 17, th.Success, true)
									}
								})
							},
						}).Clicked() {
							s.SelectModel(alias)
						}
					}
				})
			}
		})
	})
}
