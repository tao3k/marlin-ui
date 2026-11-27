// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package mobile

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// RenderSchemeExtensionsCard renders live bounds-validated `marlin.ui.v1`
// Scheme UIExtensionRegions and width-adapted status pills identically across
// iOS Liquid Glass and Android Material 3 companions.
func RenderSchemeExtensionsCard(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	cardBg, border ui.Color,
	cardRadius float32,
	activeModel, sessionID string,
	exts []schema.UIExtensionRegion,
	status []mapper.ResolvedStatusItem,
	onCommand func(string),
) {
	if len(exts) == 0 && len(status) == 0 {
		return
	}
	ui.Column(c.Key(keyPrefix+"-scheme-exts")).Padding(16, 16, 14, 16).Gap(10).Radius(cardRadius).
		Background(cardBg).Border(1, border).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			theme.SFIcon(c, "puzzlepiece.extension", 14, th.Accent, false)
			ui.Text(c, "SCHEME EXTENSIONS").Font(theme.FontMono).FontSize(10.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).SingleLine()
		})
		if len(status) > 0 {
			ui.Row(c.Key(keyPrefix+"-scheme-status-row")).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				for _, st := range status {
					ui.Row(c.Key(keyPrefix+"-st-"+st.ID)).Padding(3, 8).Radius(theme.RadiusPill).
						Background(kit.WithAlpha(th.SurfaceElevated, 0.72)).Children(func() {
						ui.Text(c, st.Text).Font(theme.FontMono).FontSize(10.5).FontWeight(600).TextColor(th.Info).NoWrap()
					})
				}
			})
		}
		mapCtx := mapper.Context{
			WidthCols: 42,
			Sheet:     th.Sheet,
			Bindings: map[string]string{
				"model":   activeModel,
				"session": sessionID,
				"theme":   th.Name,
			},
			OnCommand: func(name string) {
				if onCommand != nil {
					onCommand(name)
				}
			},
			OnAction: func(action, _ string) {
				if onCommand != nil {
					onCommand(action)
				}
			},
		}
		for _, ext := range exts {
			lbl := strings.TrimSpace(ext.Label)
			if lbl == "" {
				lbl = ext.ID
			}
			ui.Column(c.Key(keyPrefix+"-scheme-ext-"+ext.ID)).Padding(10, 12).Gap(6).Radius(max(cardRadius-8, 12)).
				Background(kit.WithAlpha(th.Canvas, 0.45)).Border(1, kit.WithAlpha(th.BorderSubtle, 0.50)).
				AlignItems(ui.Stretch).Children(func() {
				ui.Text(c, lbl).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Accent).SingleLine()
				mapper.MapElement(c, ext.Content, th.ToMapperSkin(), mapCtx)
			})
		}
	})
}

// RenderThemeSwatchGrid renders the shared 2-column visual theme pack picker
// across iOS and Android shells.
func RenderThemeSwatchGrid(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	catalog []theme.Theme,
	cardRadius float32,
	onSelect func(id string),
) {
	ui.Column(c.Key(keyPrefix+"-theme-grid-wrap")).Padding(0, 16).Gap(8).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, "THEME PACKS").Font(theme.FontMono).FontSize(10.5).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).SingleLine()
		for i := 0; i < len(catalog); i += 2 {
			ui.Row(c.Key(fmt.Sprintf("%s-th-row-%d", keyPrefix, i))).Gap(10).AlignItems(ui.Stretch).Children(func() {
				for j := i; j < i+2 && j < len(catalog); j++ {
					pk := catalog[j]
					active := pk.ID == th.ID
					borderCol := kit.WithAlpha(ui.RGB(255, 255, 255), 0.10)
					if th.IsLight {
						borderCol = kit.WithAlpha(ui.RGB(0, 0, 0), 0.10)
					}
					borderW := float32(1)
					if active {
						borderCol = th.Accent
						borderW = 2
					}
					card := kit.ActionControl(ui.ButtonBase(c.Key(keyPrefix+"-th-"+pk.ID)).Label(pk.Name).
						Basis(0).Grow(1).MinWidth(0).Padding(12).Radius(cardRadius).Gap(10).
						Column().AlignItems(ui.Stretch).
						Background(pk.Canvas).Border(borderW, borderCol))
					card.Children(func() {
						ui.Row(c).FillWidth().Height(36).Padding(6, 8).Gap(6).Radius(12).AlignItems(ui.Center).
							Background(pk.SurfaceElevated).Children(func() {
							ui.Box(c).Width(14).Height(14).Radius(7).Background(pk.Accent)
							ui.Column(c).Basis(0).Grow(1).Gap(4).Children(func() {
								ui.Box(c).FillWidth().Height(5).Radius(2.5).Background(kit.WithAlpha(pk.Ink, 0.72))
								ui.Box(c).Width(38).Height(4).Radius(2).Background(kit.WithAlpha(pk.Info, 0.65))
							})
						})
						ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, pk.Name).Font(kit.FontDisplay).FontSize(13).FontWeight(700).TextColor(pk.Ink).
								SingleLine().Ellipsis("…").MinWidth(0).Clip()
							if active {
								theme.SFIcon(c, "checkmark.circle", 16, pk.Accent, true)
							}
						})
					})
					if card.Clicked() && onSelect != nil {
						onSelect(pk.ID)
					}
				}
			})
		}
	})
}

// RenderGateReviewCard renders the shared Tier-0 Approval Gate review card
// (or All Clear state) for mobile companions.
func RenderGateReviewCard(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	pg *mygoshell.PromptGateController,
	cardRadius float32,
	onResolved func(),
) {
	if pg != nil && pg.Queue != nil {
		if req, _, total, ok := pg.Queue.Active(); ok {
			card := ui.Column(c.Key(keyPrefix+"-gate-card")).Padding(18).Gap(12).Radius(cardRadius).
				Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.80)).
				Border(1, kit.WithAlpha(th.Warning, 0.45)).
				Shadow(0, 8, 24, -6, kit.WithAlpha(ui.RGB(0, 0, 0), 0.30))
			card.Children(func() {
				ui.Text(c, fmt.Sprintf("APPROVAL REQUIRED (%d)", total)).
					Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Warning).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, req.Title).Font(kit.FontDisplay).FontSize(17).FontWeight(700).TextColor(th.Ink)
				ui.Text(c, req.Prompt).Font(kit.FontDisplay).FontSize(13.5).LineHeight(1.48).TextColor(th.InkMuted)
				ui.Row(c).Gap(10).Children(func() {
					approve := theme.StyledButton(c, theme.ButtonSpec{
						Key: keyPrefix + "-gate-yes", Label: "Approve", Icon: theme.IconApprove, Variant: theme.BtnGlassProminent, Size: theme.BtnSizeLG, Tone: th.Success, Pill: true, FullWidth: true,
					}, th)
					if approve.Clicked() {
						mygo.Haptics.Success()
						if r, resp, err := pg.Queue.Resolve(req.ID, promptgate.PromptResponse{
							PromptID: req.ID, Action: promptgate.ActionApproveOnce, IdempotencyKey: keyPrefix + "-yes-" + req.ID,
						}); err == nil && pg.OnResolve != nil {
							pg.OnResolve(r, resp)
						}
						if onResolved != nil {
							onResolved()
						}
					}
					reject := theme.StyledButton(c, theme.ButtonSpec{
						Key: keyPrefix + "-gate-no", Label: "Reject", Icon: theme.IconReject, Variant: theme.BtnDestructive, Size: theme.BtnSizeLG, Tone: th.Danger, Pill: true, FullWidth: true,
					}, th)
					if reject.Clicked() {
						mygo.Haptics.Warning()
						if r, resp, err := pg.Queue.Resolve(req.ID, promptgate.PromptResponse{
							PromptID: req.ID, Action: promptgate.ActionReject, IdempotencyKey: keyPrefix + "-no-" + req.ID,
						}); err == nil && pg.OnResolve != nil {
							pg.OnResolve(r, resp)
						}
						if onResolved != nil {
							onResolved()
						}
					}
				})
			})
			return
		}
	}

	ui.Column(c.Key(keyPrefix+"-gates-empty")).Padding(28, 16).Gap(8).Radius(cardRadius).AlignItems(ui.Center).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.68)).
		Border(1, kit.WithAlpha(th.BorderSubtle, 0.50)).Children(func() {
		theme.SFIcon(c, "checkmark.shield", 30, th.Success, true)
		ui.Text(c, "All Clear").Font(kit.FontDisplay).FontSize(17).FontWeight(700).TextColor(th.Ink)
		ui.Text(c, "No pending tool or policy approvals from Marlin Core.").
			Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted)
	})
}

// RenderMobileChatStream renders a platform-neutral mobile chat transcript and
// bottom composer backed by Marlin's shared TranscriptController.
func RenderMobileChatStream(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	tr *mygoshell.TranscriptController,
	cardRadius float32,
	draft *string,
	deepThink bool,
	onSend func(string),
	onToggleDeepThink func(),
) {
	ui.Column(c.Key(keyPrefix+"-chat-root")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		ui.Scroll(c.Key(keyPrefix+"-chat-scroll")).Grow(1).MinHeight(0).Padding(12, 16).Children(func() {
			ui.Column(c.Key(keyPrefix+"-chat-list")).Gap(12).AlignItems(ui.Stretch).Children(func() {
				if tr == nil || tr.Len() == 0 {
					ui.Column(c.Key(keyPrefix+"-chat-empty")).Padding(36, 16).Gap(10).AlignItems(ui.Center).Children(func() {
						theme.MarlinEmblem(c, keyPrefix+"-chat-emblem", 56, th)
						ui.Text(c, "Marlin Mobile").Font(kit.FontDisplay).FontSize(18).FontWeight(700).TextColor(th.Ink)
						ui.Text(c, "Ask anything or switch to Rooms to collaborate with AI agents.").
							Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted)
					})
					return
				}
				n := tr.Len()
				for i := 0; i < n; i++ {
					entry, ok := tr.Entry(i)
					if !ok {
						continue
					}
					if entry.Role == mygoshell.RoleUser {
						ui.Row(c.Key(fmt.Sprintf("%s-u-%d", keyPrefix, i))).Justify(ui.End).Children(func() {
							ui.Column(c).MaxWidth(300).Padding(10, 14).Radius(cardRadius).
								Background(kit.WithAlpha(th.Accent, 0.20)).
								Border(1, kit.WithAlpha(th.Accent, 0.34)).Children(func() {
								ui.Text(c, entry.Text).Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink)
							})
						})
					} else {
						ui.Column(c.Key(fmt.Sprintf("%s-a-%d", keyPrefix, i))).Padding(12, 14).Gap(4).Radius(cardRadius).
							Background(th.SurfaceElevated).Border(1, kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "Marlin").Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Accent).SingleLine()
								if entry.Streaming {
									ui.Text(c, "Streaming…").Font(theme.FontMono).FontSize(10).TextColor(th.Info).SingleLine()
								}
							})
							ui.Text(c, entry.Text).Font(kit.FontDisplay).FontSize(14).LineHeight(1.48).TextColor(th.Ink)
						})
					}
				}
			})
		})

		ui.Row(c.Key(keyPrefix+"-chat-composer")).Padding(8, 12, 10, 12).Gap(8).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			dtVariant := theme.BtnGhost
			dtTone := th.InkMuted
			if deepThink {
				dtVariant = theme.BtnTinted
				dtTone = th.Accent
			}
			if theme.IconCircleButton(c, keyPrefix+"-dt-btn", "Deep Think", "sparkles", 34, dtVariant, dtTone, deepThink, th).Clicked() && onToggleDeepThink != nil {
				onToggleDeepThink()
			}
			inp := ui.TextInputBase(c.Key(keyPrefix+"-chat-input"), draft).
				Placeholder("Ask Marlin anything…").
				Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
			submitted := inp.Submitted()
			if theme.IconCircleButton(c, keyPrefix+"-send-btn", "Send", theme.IconSend, 36, theme.BtnGlassProminent, th.Accent, false, th).Clicked() {
				submitted = true
			}
			if submitted && draft != nil && strings.TrimSpace(*draft) != "" && onSend != nil {
				txt := strings.TrimSpace(*draft)
				*draft = ""
				onSend(txt)
			}
		})
	})
}

// RenderMobileRoomView renders a complete shared Agent Room view backed by
// featurekit.RoomController (channels, compact COVE badge, live messages,
// reactions, shared tasks, and composer) for Android Material 3 and shared surfaces.
func RenderMobileRoomView(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	snap featurekit.RoomFullSnapshot,
	cardRadius float32,
	draft *string,
	onSelectChannel func(id string),
	onPost func(channelID, text string),
	onToggleRx func(msgID, tag string),
	onCycleTask func(taskID string),
) {
	ui.Column(c.Key(keyPrefix+"-room-root")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		// Channel strip + single compact COVE indicator right-aligned.
		ui.Row(c.Key(keyPrefix+"-room-ch-bar")).Padding(8, 12).Gap(6).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			ui.ScrollHorizontal(c.Key(keyPrefix+"-room-ch-scroll")).Grow(1).MinWidth(0).Height(32).Clip().Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					for _, ch := range snap.Channels {
						item := ch
						sel := item.ID == snap.ActiveChannelID
						v := theme.BtnGhost
						tone := th.InkMuted
						if sel {
							v = theme.BtnTinted
							tone = th.Accent
						}
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: keyPrefix + "-ch-" + item.ID, Label: "#" + item.Name, Variant: v, Size: theme.BtnSizeSM, Tone: tone, Pill: true,
						}, th).Clicked() && onSelectChannel != nil {
							onSelectChannel(item.ID)
						}
					}
				})
			})
			coveDot := th.Success
			coveLbl := "CORE"
			switch snap.SyncMode {
			case "cove-live":
				coveLbl = "COVE"
				coveDot = th.Success
			case "cove-connecting":
				if strings.Contains(strings.ToLower(snap.SyncLabel), "auth") {
					coveLbl = "COVE AUTH"
					coveDot = th.Danger
				} else {
					coveLbl = "COVE…"
					coveDot = th.Warning
				}
			}
			ui.Row(c.Key(keyPrefix+"-cove-badge")).Padding(3, 8).Gap(5).Radius(theme.RadiusPill).Shrink(0).
				Background(kit.WithAlpha(coveDot, 0.14)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(coveDot)
				ui.Text(c, coveLbl).Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(coveDot).NoWrap()
			})
		})

		// Live room messages + top tasks.
		ui.Scroll(c.Key(keyPrefix+"-room-scroll")).Grow(1).MinHeight(0).Padding(12, 14).Children(func() {
			ui.Column(c.Key(keyPrefix+"-room-feed")).Gap(10).AlignItems(ui.Stretch).Children(func() {
				for _, m := range snap.Messages {
					msg := m
					ui.Column(c.Key(keyPrefix+"-rmsg-"+msg.ID)).Padding(12, 14).Gap(6).Radius(cardRadius).
						Background(th.SurfaceElevated).Border(1, kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							nameCol := th.Ink
							if msg.IsAgent {
								nameCol = th.Accent
							}
							ui.Text(c, msg.AuthorName).Font(kit.FontDisplay).FontSize(13).FontWeight(700).TextColor(nameCol).SingleLine()
							if msg.AuthorRole != "" {
								ui.Text(c, msg.AuthorRole).Font(theme.FontMono).FontSize(10).TextColor(th.InkMuted).SingleLine()
							}
						})
						ui.Text(c, msg.Body).Font(kit.FontDisplay).FontSize(13.5).LineHeight(1.46).TextColor(th.Ink)
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ackCount := 0
							for _, rx := range msg.Reactions {
								if rx.Tag == "ack" {
									ackCount = rx.Count
									break
								}
							}
							ackLabel := "Ack"
							if ackCount > 0 {
								ackLabel = fmt.Sprintf("Ack %d", ackCount)
							}
							if theme.StyledButton(c, theme.ButtonSpec{
								Key: keyPrefix + "-ack-" + msg.ID, Label: ackLabel, Variant: theme.BtnGhost, Size: theme.BtnSizeSM, Tone: th.Accent, Pill: true,
							}, th).Clicked() && onToggleRx != nil {
								onToggleRx(msg.ID, "ack")
							}
						})
					})
				}
				if len(snap.Tasks) > 0 {
					ui.Text(c, "SHARED TASKS").Font(theme.FontMono).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(th.InkMuted).Padding(6, 2, 0, 2)
					for _, tk := range snap.Tasks {
						item := tk
						if theme.StyledButton(c, theme.ButtonSpec{
							Key: keyPrefix + "-rtk-" + item.ID, Label: fmt.Sprintf("[%s] %s", item.Status, item.Title), Variant: theme.BtnGeistSecondary, Size: theme.BtnSizeSM, FullWidth: true,
						}, th).Clicked() && onCycleTask != nil {
							onCycleTask(item.ID)
						}
					}
				}
			})
		})

		// Room composer bar.
		ui.Row(c.Key(keyPrefix+"-room-composer")).Padding(8, 12, 10, 12).Gap(8).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			inp := ui.TextInputBase(c.Key(keyPrefix+"-room-input"), draft).
				Placeholder("Message room (@architect, @reviewer)…").
				Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
			submitted := inp.Submitted()
			if theme.IconCircleButton(c, keyPrefix+"-room-send", "Post", theme.IconSend, 36, theme.BtnGlassProminent, th.Accent, false, th).Clicked() {
				submitted = true
			}
			if submitted && draft != nil && strings.TrimSpace(*draft) != "" && onPost != nil {
				txt := strings.TrimSpace(*draft)
				*draft = ""
				onPost(snap.ActiveChannelID, txt)
			}
		})
	})
}

// RenderMobileSearchView renders the shared Helm mobile search experience.
func RenderMobileSearchView(
	c *ui.Context,
	keyPrefix string,
	th theme.Theme,
	helm *mygoshell.HelmMinibufferController,
	query *string,
	onCiteChat func(ref string),
) {
	var rows []search.VisibleRow
	if helm != nil && helm.Engine != nil {
		helm.Engine.SetViewportRows(16)
		rows, _, _, _, _, _ = helm.Engine.VisibleWindow(nil)
	}
	ui.Column(c.Key(keyPrefix+"-search-root")).Fill().AlignItems(ui.Stretch).Clip().Children(func() {
		ui.Row(c.Key(keyPrefix+"-search-bar")).Padding(10, 14).Gap(8).AlignItems(ui.Center).
			Background(th.Bar).BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).Children(func() {
			theme.SFIcon(c, "magnifyingglass", 15, th.InkMuted, false)
			inp := ui.TextInputBase(c.Key(keyPrefix+"-search-input"), query).
				Placeholder("Search files, symbols, commands…").
				Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1).MinWidth(0)
			if inp.Changed() && helm != nil && query != nil {
				helm.SetQuery(*query)
			}
		})
		ui.Scroll(c.Key(keyPrefix+"-search-scroll")).Grow(1).MinHeight(0).Padding(8, 12).Children(func() {
			ui.Column(c.Key(keyPrefix+"-search-list")).Gap(6).AlignItems(ui.Stretch).Children(func() {
				if len(rows) == 0 {
					ui.Text(c, "Type to search Rust files, buffers, and Marlin commands.").
						Font(kit.FontDisplay).FontSize(13).TextColor(th.InkMuted).Padding(16, 8)
					return
				}
				for _, r := range rows {
					if r.IsHeader {
						ui.Text(c, fmt.Sprintf("%s · %d", strings.ToUpper(r.SourceLabel), r.SourceCount)).
							Font(theme.FontMono).FontSize(10).FontWeight(700).TextColor(th.InkMuted).Padding(8, 4, 2, 4)
						continue
					}
					cand := r.Candidate
					row := kit.ActionControl(ui.ButtonBase(c.Key(keyPrefix+"-srow-"+cand.ID)).Label(cand.Title).
						FillWidth().Padding(10, 12).Radius(12).Background(th.SurfaceElevated).
						Column().AlignItems(ui.Start).Gap(2))
					row.Children(func() {
						ui.Text(c, cand.Title).Font(kit.FontDisplay).FontSize(13.5).FontWeight(600).TextColor(th.Ink).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
						if cand.Location != "" {
							ui.Text(c, cand.Location).Font(theme.FontMono).FontSize(10.5).TextColor(th.InkMuted).
								SingleLine().Ellipsis("…").MinWidth(0).Clip()
						}
					})
					if row.Clicked() && onCiteChat != nil {
						ref := cand.Location
						if ref == "" {
							ref = cand.Title
						}
						onCiteChat(ref)
					}
				}
			})
		})
	})
}
