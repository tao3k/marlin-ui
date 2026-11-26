// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// renderHelm renders Marlin AI's modern Vertico + Consult + Embark Helm surface:
//   - Top source-filter pills (All, Buffers, Rust Search, Files, Sessions, Targets, Commands)
//   - Left 60%: virtualized candidate list with mark badges (TAB), › pointer, and gold hit highlights
//   - Right 40% (Follow-Mode Live Preview): instant snippet/location preview + Embark action buttons
func renderHelm(c *ui.Context, s *State, th theme.Theme, query string, helm *mygoshell.HelmMinibufferController, onEcho func(string)) {
	if helm == nil || helm.Engine == nil {
		return
	}
	eng := helm.Engine
	eng.SetViewportRows(9)
	rows, _, totalHits, _, _, gen := eng.VisibleWindow(s.helmRowsBuf[:0])
	s.helmRowsBuf = rows
	srcs := eng.SourceSummaries(nil)
	activeFilter := eng.ActiveSourceFilter()
	previewOpen := eng.PreviewOpen()
	markedCount := eng.MarkedCount()
	selCand, hasSel := eng.Selected()
	_, prevTitle, prevBody := eng.Preview()
	if prevTitle == "" && hasSel {
		prevTitle = selCand.Title
	}
	if prevBody == "" && hasSel {
		prevBody = selCand.Snippet
		if prevBody == "" {
			prevBody = selCand.Location
		}
	}

	highlightQuery := query
	if _, right, ok := strings.Cut(query, "#"); ok && strings.TrimSpace(right) != "" {
		highlightQuery = strings.TrimSpace(right)
	}

	ui.Column(c.Key("helm-vertico-list")).Background(theme.GlassTone(c, th.Bar, th, 0.52, 0.72)).Padding(8, 12).Gap(8).
		BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.40, 0.50)).AlignItems(ui.Stretch).Children(func() {
		// 1. Source Filter Pills + Telemetry + Preview Toggle.
		ui.Row(c.Key("helm-source-pills")).Gap(6).AlignItems(ui.Center).Children(func() {
			allActive := activeFilter == ""
			allTone := th.InkMuted
			if allActive {
				allTone = th.Info
			}
			if theme.ActionButton(c, "hsrc-all", fmt.Sprintf("All (%d)", totalHits), allTone, allActive, th).Clicked() {
				eng.SetSourceFilter("")
			}
			for _, sm := range srcs {
				src := sm
				active := activeFilter == src.ID
				tone := th.InkMuted
				if active {
					tone = th.Accent
				}
				lbl := fmt.Sprintf("%s (%d)", src.Label, src.Count)
				if theme.ActionButton(c, "hsrc-"+src.ID, lbl, tone, active, th).Clicked() {
					if active {
						eng.SetSourceFilter("")
					} else {
						eng.SetSourceFilter(src.ID)
					}
				}
			}

			ui.Spacer(c)

			if markedCount > 0 {
				ui.Row(c).Padding(2, 8).Radius(theme.RadiusPill).Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.64)).Children(func() {
					ui.Text(c, fmt.Sprintf("● %d marked", markedCount)).
						Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(th.Warning).NoWrap()
				})
			}
			ui.Text(c, fmt.Sprintf("#%d", gen)).Font(theme.FontMono).FontSize(11.5).TextColor(th.BannerInk).NoWrap()
			prevLbl := "Preview"
			prevTone := th.InkMuted
			if previewOpen {
				prevTone = th.Info
			}
			if theme.ActionLink(c, "helm-toggle-prev", prevLbl, prevTone, th).Clicked() {
				eng.TogglePreview()
			}
		})

		// 2. Main Split Area: Left Candidate Stream | Right Follow-Mode Live Preview & Embark Actions.
		ui.Row(c.Key("helm-split-body")).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// Left column: Vertico candidate list.
			ui.Column(c.Key("helm-cand-col")).Grow(3).MinWidth(0).Gap(2).AlignItems(ui.Stretch).Children(func() {
				if len(rows) == 0 {
					ui.Row(c).Padding(14, 12).Children(func() {
						ui.Text(c, "No matching candidates (tip: use # for Consult local filter, !word to exclude)").
							Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted)
					})
					return
				}
				for _, r := range rows {
					if r.IsHeader {
						ui.Row(c.Key("helm-hdr-"+r.SourceID)).Height(24).Padding(0, 10).
							AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("%s (%d)", r.SourceLabel, r.SourceCount)).
								Font(theme.FontMono).FontSize(12).FontWeight(700).TextColor(th.Accent)
						})
						continue
					}
					cand := r.Candidate
					title := cand.Title
					if r.Marked {
						title = "● " + title
					}
					sub := cand.Subtitle
					if sub == "" {
						sub = cand.Location
					}
					if theme.VerticoRow(c, "hrow-"+cand.ID, title, sub, highlightQuery, 260, r.Selected, th).Clicked() {
						targets := eng.ResolveActionTargets(nil)
						if len(targets) == 0 || markedCount == 0 {
							targets = []search.Candidate{cand}
						}
						if helm.OnExecuteAction != nil {
							helm.OnExecuteAction(search.ActionOpenInFocused, targets)
						}
						s.Close()
						if onEcho != nil {
							onEcho("Opened " + cand.Title)
						}
					}
				}
			})

			// Right column: Follow-Mode Live Preview + Embark Action Strip.
			if previewOpen && hasSel {
				ui.Column(c.Key("helm-preview-col")).Grow(2).MinWidth(260).
					Background(theme.GlassTone(c, th.Canvas, th, 0.44, 0.64)).Radius(theme.RadiusControl).
					Padding(12, 14).Gap(8).AlignItems(ui.Stretch).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Row(c).Padding(2, 7).Radius(theme.RadiusSmall).Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.64)).Children(func() {
							ui.Text(c, selCand.SourceID).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Accent).NoWrap()
						})
						ui.Text(c, prevTitle).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).
							TextColor(th.Info).Grow(1).NoWrap().MaxLines(1)
					})
					if selCand.Location != "" {
						ui.Text(c, selCand.Location).Font(theme.FontMono).FontSize(12).TextColor(th.InkMuted).NoWrap().MaxLines(1)
					}
					if prevBody != "" {
						ui.Box(c).Height(1).Background(th.BorderSubtle)
						ui.Text(c, prevBody).Font(theme.FontMono).FontSize(theme.FontSizeMeta).LineHeight(1.45).
							TextColor(th.Ink).Grow(1).MaxLines(8)
					} else {
						ui.Spacer(c)
					}

					// Embark action bar.
					ui.Row(c.Key("helm-embark-actions")).Gap(6).AlignItems(ui.Center).Children(func() {
						if theme.ActionButton(c, "embark-open", "Open", th.Info, true, th).Clicked() {
							targets := eng.ResolveActionTargets(nil)
							if len(targets) == 0 {
								targets = []search.Candidate{selCand}
							}
							if helm.OnExecuteAction != nil {
								helm.OnExecuteAction(search.ActionOpenInFocused, targets)
							}
							s.Close()
							if onEcho != nil {
								onEcho("Opened " + selCand.Title)
							}
						}
						if theme.ActionButton(c, "embark-attach", "Attach", th.Success, false, th).Clicked() {
							targets := eng.ResolveActionTargets(nil)
							if len(targets) == 0 {
								targets = []search.Candidate{selCand}
							}
							if helm.OnExecuteAction != nil {
								helm.OnExecuteAction(search.ActionMentionInComposer, targets)
							}
							s.Close()
							if onEcho != nil {
								onEcho(fmt.Sprintf("Attached %d item(s)", len(targets)))
							}
						}
						if theme.ActionButton(c, "embark-mark", "Mark [TAB]", th.Warning, false, th).Clicked() {
							eng.ToggleMarkCurrent()
						}
					})
				})
			}
		})
	})
}
