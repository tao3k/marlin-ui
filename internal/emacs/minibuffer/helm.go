// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

const helmDrawerHeight float32 = 356

// renderHelm renders Marlin AI's Rust-backed Vertico + Consult + Embark Helm surface
// inside a fixed-height drawer (Doom Emacs helm-display-buffer-default-height /
// vertico-resize=nil) so candidate count changes NEVER bounce or reflow the viewport:
//   - Row 1: Rust Helm Mode Preset Tabs ([All] [Search] [DSL] [Buffers] [Commands])
//            + live source counts + authoritative Rust gRPC / generation badge + Follow toggle
//   - Row 2: Live Two-Stage Consult / DSL Query Pill Bar (matching Rust MAR-HELM-R02:
//            DSL S-exp, @lang:, @doc:, @prefix:, @ext:, @sym:, @graph:, #local, !negate)
//   - Body:  Left 60% virtualized candidate stream (TAB multi-mark, gold match highlights)
//            Right 40% Follow-Mode live preview + Embark Action Transformer bar
func renderHelm(
	c *ui.Context,
	s *State,
	th theme.Theme,
	query string,
	helm *mygoshell.HelmMinibufferController,
	rustConnected bool,
	onEcho func(string),
	onPreview func(title, path, snippet string, line int),
	onCite func(ref string),
) {
	if helm == nil || helm.Engine == nil {
		return
	}
	eng := helm.Engine
	eng.SetViewportRows(9)
	liveRows, _, liveTotalHits, _, _, liveGen := eng.VisibleWindow(s.helmRowsBuf[:0])
	s.helmRowsBuf = liveRows
	liveSrcs := eng.SourceSummaries(nil)
	activeFilter := eng.ActiveSourceFilter()
	previewOpen := eng.PreviewOpen()
	markedCount := eng.MarkedCount()
	liveSelCand, liveHasSel := eng.Selected()
	_, livePrevTitle, livePrevBody := eng.Preview()
	if livePrevTitle == "" && liveHasSel {
		livePrevTitle = liveSelCand.Title
	}
	if livePrevBody == "" && liveHasSel {
		livePrevBody = liveSelCand.Snippet
		if livePrevBody == "" {
			livePrevBody = liveSelCand.Location
		}
	}

	// Double-buffered Stale-While-Revalidate resolution: while a new keystroke's
	// async Rust gRPC batch is in flight, hold the previous frame rock-steady
	// instead of flashing blank / "No candidate selected".
	rows, srcs, totalHits, _, selCand, hasSel, prevTitle, prevBody, _ := s.resolveHelmFrame(
		rustConnected, liveRows, liveSrcs, liveTotalHits, liveGen, liveSelCand, liveHasSel, livePrevTitle, livePrevBody,
	)

	cq := search.ParseConsultQuery(query)
	highlightQuery := query
	if cq.HasStageSep && len(cq.Positive) > 0 {
		highlightQuery = strings.Join(cq.Positive, " ")
	} else if !cq.IsDSL && strings.TrimSpace(cq.Pattern) != "" {
		highlightQuery = strings.TrimSpace(cq.Pattern)
	}

	srcCount := func(id string) int {
		for _, sm := range srcs {
			if sm.ID == id {
				return sm.Count
			}
		}
		return 0
	}

	ui.Column(c.Key("helm-vertico-list")).Height(helmDrawerHeight).Shrink(0).Clip().
		Background(theme.GlassTone(c, th.Bar, th, 0.34, 0.58)).Padding(10, 14).Gap(8).
		BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.44, 0.56)).AlignItems(ui.Stretch).Children(func() {
		// 1. Fixed-height Rust-Parity Mode Preset Tabs + Provenance Badge + Follow Preview Toggle.
		ui.Row(c.Key("helm-mode-presets")).Height(28).Shrink(0).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
			presets := []struct {
				id       string
				label    string
				filterID string
				count    int
				dslSeed  bool
			}{
				{"all", "All", "", totalHits, false},
				{"search", "Search", "rust-graph", srcCount("rust-graph"), false},
				{"dsl", "DSL", "rust-dsl", srcCount("rust-dsl"), true},
				{"buffers", "Buffers", "buffers", srcCount("buffers"), false},
				{"commands", "Commands", "rust-commands", srcCount("rust-commands"), false},
			}
			for _, p := range presets {
				preset := p
				active := (preset.filterID == "" && activeFilter == "" && !cq.IsDSL) ||
					(preset.filterID != "" && activeFilter == preset.filterID) ||
					(preset.dslSeed && cq.IsDSL && activeFilter == "")
				tone := th.InkMuted
				if active {
					tone = th.Info
					if preset.dslSeed {
						tone = th.Accent
					}
				}
				lbl := preset.label
				if preset.count > 0 || preset.filterID == "" {
					lbl = fmt.Sprintf("%s (%d)", preset.label, preset.count)
				}
				if theme.ActionButton(c, "hpreset-"+preset.id, lbl, tone, active, th).Clicked() {
					s.ClearHelmPending()
					if preset.dslSeed {
						eng.SetSourceFilter("")
						if !strings.HasPrefix(strings.TrimSpace(query), "(") {
							s.SetInput("(query ")
							gen := helm.SetQuery("(query ")
							s.MarkHelmPending(gen)
						}
					} else if active && preset.filterID != "" {
						eng.SetSourceFilter("")
					} else {
						eng.SetSourceFilter(preset.filterID)
					}
				}
			}

			// Additional dynamic sources (Sessions, Targets, etc.) when populated.
			for _, sm := range srcs {
				if sm.ID == "rust-graph" || sm.ID == "rust-dsl" || sm.ID == "buffers" || sm.ID == "rust-commands" {
					continue
				}
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
				renderHelmChip(c, fmt.Sprintf("● %d marked", markedCount), th.Warning, th)
			}
			prevTone := th.InkMuted
			if previewOpen {
				prevTone = th.Info
			}
			if theme.ActionLink(c, "helm-toggle-prev", "Follow Preview", prevTone, th).Clicked() {
				eng.TogglePreview()
			}
		})

		// 2. Live Two-Stage Consult / DSL Query Pill Strip (MAR-HELM-R02 parity).
		renderConsultChipBar(c, cq, th)

		// 3. Main Split Area (fixed-height fill): Left Candidate Stream | Right Follow-Mode Live Preview & Embark Actions.
		ui.Row(c.Key("helm-split-body")).Grow(1).MinHeight(0).Gap(12).AlignItems(ui.Stretch).Clip().Children(func() {
			// Left column: fixed-height Vertico candidate list.
			ui.Column(c.Key("helm-cand-col")).Grow(3).MinWidth(0).MinHeight(0).Gap(2).AlignItems(ui.Stretch).Clip().Children(func() {
				if len(rows) == 0 {
					ui.Row(c).Padding(14, 12).Children(func() {
						ui.Text(c, "No matching candidates — try @lang:rust @sym:name #local !exclude or (dsl ...)").
							Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
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
						if onPreview != nil && (strings.TrimSpace(cand.Location) != "" || strings.TrimSpace(cand.Snippet) != "") {
							onPreview(cand.Title, cand.Location, cand.Snippet, 0)
						}
						dispatchHelmTargets(s, eng, helm, search.ActionOpenInFocused, cand, markedCount, "Opened "+cand.Title, onEcho)
					}
				}
			})

			// Right column: fixed-height Follow-Mode Live Preview + Embark Action Transformer Bar.
			// Always keeps its column footprint while previewOpen is true so horizontal layout never jumps at 0 hits.
			if previewOpen {
				ui.Column(c.Key("helm-preview-col")).Grow(2).MinWidth(280).MinHeight(0).Clip().
					Background(theme.GlassTone(c, th.Canvas, th, 0.46, 0.70)).Radius(theme.RadiusControl).
					BorderWidth(1).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.35, 0.55)).
					Padding(12, 14).Gap(8).AlignItems(ui.Stretch).Children(func() {
					if !hasSel {
						ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "No candidate selected").Font(kit.FontDisplay).FontSize(12.5).TextColor(th.InkMuted)
						})
						return
					}
					ui.Row(c).MinWidth(0).AlignItems(ui.Center).Gap(8).Clip().Children(func() {
						renderHelmChip(c, selCand.SourceID, th.Accent, th)
						ui.Text(c, prevTitle).Font(kit.FontDisplay).FontSize(theme.FontSizeUI).FontWeight(700).
							TextColor(th.Info).Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
					})
					if selCand.Location != "" {
						ui.Text(c, selCand.Location).Font(theme.FontMono).FontSize(11.5).TextColor(th.InkMuted).
							SingleLine().Ellipsis("…").TruncateMiddle().MinWidth(0).Clip()
					}
					if selCand.Subtitle != "" && selCand.Subtitle != selCand.Location {
						ui.Text(c, selCand.Subtitle).Font(theme.FontMono).FontSize(11).TextColor(th.BannerInk).
							SingleLine().Ellipsis("…").MinWidth(0).Clip()
					}
					if prevBody != "" {
						ui.Box(c).Height(1).Background(th.BorderSubtle)
						ui.Scroll(c.Key("helm-preview-scroll")).Grow(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
							ui.Text(c, prevBody).Font(theme.FontMono).FontSize(theme.FontSizeMeta).LineHeight(1.48).
								TextColor(th.Ink).MinWidth(0)
						})
					} else {
						ui.Spacer(c)
					}

					// Embark Action Transformer Bar (all backend actions dispatch via Rust gRPC).
					ui.Row(c.Key("helm-embark-actions")).MinWidth(0).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
						if theme.ActionButton(c, "embark-open", "Open", th.Info, true, th).Clicked() {
							if onPreview != nil && (strings.TrimSpace(selCand.Location) != "" || strings.TrimSpace(selCand.Snippet) != "") {
								onPreview(selCand.Title, selCand.Location, selCand.Snippet, 0)
							}
							dispatchHelmTargets(s, eng, helm, search.ActionOpenInFocused, selCand, markedCount, "Opened "+selCand.Title, onEcho)
						}
						if theme.ActionButton(c, "embark-attach", "Attach @cite", th.Success, false, th).Clicked() {
							if onCite != nil {
								ref := strings.TrimSpace(selCand.Location)
								if ref == "" {
									ref = strings.TrimSpace(selCand.Title)
								}
								if ref != "" {
									onCite(ref)
								}
							}
							dispatchHelmTargets(s, eng, helm, search.ActionMentionInComposer, selCand, markedCount, "Attached citation(s)", onEcho)
						}
						if theme.ActionButton(c, "embark-dispatch", "Dispatch Agent", th.Accent, false, th).Clicked() {
							dispatchHelmTargets(s, eng, helm, search.ActionOpenSplitRight, selCand, markedCount, "Dispatched to Rust agent", onEcho)
						}
						if theme.ActionButton(c, "embark-export", "Export", th.InkMuted, false, th).Clicked() {
							dispatchHelmTargets(s, eng, helm, search.ActionOpenInNewTab, selCand, markedCount, "Exported context to Rust", onEcho)
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

func renderHelmChip(c *ui.Context, text string, tone ui.Color, th theme.Theme) {
	ui.Row(c).Padding(2, 7).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.66)).Children(func() {
		ui.Text(c, text).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(tone).NoWrap()
	})
}

func renderConsultChipBar(c *ui.Context, cq search.ConsultQuery, th theme.Theme) {
	type chip struct {
		label string
		tone  ui.Color
	}
	var chips []chip
	if cq.IsDSL {
		chips = append(chips, chip{"DSL S-exp", th.Accent})
	}
	for _, lang := range cq.Languages {
		chips = append(chips, chip{"@lang:" + lang, th.Info})
	}
	for _, doc := range cq.Documents {
		chips = append(chips, chip{"@doc:" + doc, th.Info})
	}
	if cq.PathPrefix != "" {
		chips = append(chips, chip{"@prefix:" + cq.PathPrefix, th.Info})
	}
	if cq.Extension != "" {
		chips = append(chips, chip{"@ext:" + cq.Extension, th.Info})
	}
	if cq.Symbol != "" {
		chips = append(chips, chip{"@sym:" + cq.Symbol, th.Accent})
	}
	if cq.GraphTarget != "" {
		chips = append(chips, chip{"@graph:" + cq.GraphTarget, th.Accent})
	}
	if cq.HasStageSep && len(cq.Positive) > 0 {
		chips = append(chips, chip{"# " + strings.Join(cq.Positive, " "), th.Success})
	}
	for _, neg := range cq.Negative {
		chips = append(chips, chip{"!" + neg, th.Danger})
	}

	ui.Row(c.Key("helm-consult-chips")).Height(22).Shrink(0).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
		if len(chips) == 0 {
			ui.Text(c, "Syntax: @lang:rust  @sym:fn  @prefix:crates/  #local  !exclude  (dsl ...)").
				Font(theme.FontMono).FontSize(11).TextColor(th.BannerInk).NoWrap()
			return
		}
		for _, ch := range chips {
			renderHelmChip(c, ch.label, ch.tone, th)
		}
	})
}

func dispatchHelmTargets(
	s *State,
	eng *search.HelmEngine,
	helm *mygoshell.HelmMinibufferController,
	action search.TargetAction,
	fallback search.Candidate,
	markedCount int,
	echoMsg string,
	onEcho func(string),
) {
	targets := eng.ResolveActionTargets(nil)
	if len(targets) == 0 || markedCount == 0 {
		targets = []search.Candidate{fallback}
	}
	if helm.OnExecuteAction != nil {
		helm.OnExecuteAction(action, targets)
	}
	s.Close()
	if onEcho != nil {
		onEcho(echoMsg)
	}
}
