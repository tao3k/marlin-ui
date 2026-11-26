// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// renderEditorialCodeBlock wraps MyGo's syntax-highlighted Markdown code body
// in a modern Agent Platform frame: language badge, live streaming indicator,
// and one-click Copy button.
func renderEditorialCodeBlock(c *ui.Context, b *markdown.CodeBlockView, tr *mygoshell.TranscriptController, th theme.Theme) {
	lang := strings.TrimSpace(b.Language)
	if lang == "" {
		lang = "code"
	}
	codeStr := b.Code()
	ui.Column(c.Key(fmt.Sprintf("conv-codeblk-%d", b.Index))).Radius(10).
		Background(theme.GlassTone(c, th.Bar, th, 0.54, 0.84)).
		BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(30).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.70)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			ui.Text(c, strings.ToUpper(lang)).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Accent).NoWrap()
			if b.Open {
				kit.LoadingIndicator(c, 11, th.Info, false)
				ui.Text(c, "writing...").Font(theme.FontMono).FontSize(10.5).TextColor(th.Info).NoWrap()
			}
			ui.Spacer(c)
			if tr != nil && tr.OnCopyCode != nil && strings.TrimSpace(codeStr) != "" {
				if theme.ActionLink(c, fmt.Sprintf("code-copy-%d", b.Index), "Copy", th.InkMuted, th).Clicked() {
					tr.OnCopyCode(codeStr)
				}
			}
		})
		ui.Column(c).Padding(10, 14).AlignItems(ui.Stretch).Children(func() {
			b.Body(c)
		})
	})
}

// renderExecutionTraceGroup collapses a contiguous sequence of RoleThinking +
// RoleTool transcript entries into a single high-craft Execution Trace stepper
// pill (folded by default, expanding into a vertical step timeline).
func renderExecutionTraceGroup(
	c *ui.Context,
	startIdx int,
	entries []mygoshell.TranscriptEntry,
	tr *mygoshell.TranscriptController,
	bc *Context,
	th theme.Theme,
) {
	key := fmt.Sprintf("trace-group-%d", startIdx)
	isOpen := bc != nil && bc.IsThinkingOpen != nil && bc.IsThinkingOpen(key)
	arrow := "▸"
	if isOpen {
		arrow = "▾"
	}

	toolCount := 0
	thoughtCount := 0
	verifiedCount := 0
	hasDenial := false
	hasRunning := false
	for _, e := range entries {
		if e.Role == mygoshell.RoleThinking {
			thoughtCount++
		} else if e.Role == mygoshell.RoleTool {
			toolCount++
			if e.ToolCard != nil {
				if e.ToolCard.ReceiptID != "" {
					verifiedCount++
				}
				if e.ToolCard.DenialMessage != "" {
					hasDenial = true
				}
				up := strings.ToUpper(e.ToolCard.Phase)
				if strings.Contains(up, "RUN") || strings.Contains(up, "START") || strings.Contains(up, "WAIT") {
					hasRunning = true
				}
			}
		}
	}

	var parts []string
	if thoughtCount > 0 {
		parts = append(parts, fmt.Sprintf("%d thought", thoughtCount))
	}
	if toolCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tool step(s)", toolCount))
	}
	summary := strings.Join(parts, " · ")
	if summary == "" {
		summary = fmt.Sprintf("%d step(s)", len(entries))
	}

	statusTone := th.Success
	statusLbl := "Verified"
	if hasDenial {
		statusTone = th.Danger
		statusLbl = "Denied"
	} else if hasRunning {
		statusTone = th.Info
		statusLbl = "Running"
	} else if verifiedCount == 0 {
		statusTone = th.InkMuted
		statusLbl = "Completed"
	}

	ui.Column(c.Key(key)).Radius(10).
		Background(theme.GlassTone(c, th.Bar, th, 0.44, 0.70)).
		BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		hdrTitle := fmt.Sprintf("%s Execution Trace · %s", arrow, summary)
		hdr := ui.ButtonBase(c.Key(key+"-hdr")).Label(hdrTitle).
			Height(34).Padding(0, 12).Gap(8).Radius(10).AlignItems(ui.Center).Justify(ui.Start)
		if hdr.Hovered() {
			hdr.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.56))
		}
		hdr.Children(func() {
			if hasRunning {
				kit.LoadingIndicator(c, 12, th.Info, false)
			}
			ui.Text(c, hdrTitle).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Ink).NoWrap()
			renderMetaPill(c, statusLbl, statusTone, th)
			ui.Spacer(c)
			if verifiedCount > 0 {
				renderMetaPill(c, fmt.Sprintf("%d receipt(s)", verifiedCount), th.Success, th)
			}
		})
		if hdr.Clicked() && bc != nil && bc.ToggleThinking != nil {
			bc.ToggleThinking(key)
		}

		if isOpen {
			ui.Column(c.Key(key+"-steps")).Padding(8, 14, 12, 16).Gap(8).
				BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
				for offset, e := range entries {
					rowIdx := startIdx + offset
					ui.Row(c.Key(fmt.Sprintf("%s-step-%d", key, offset))).Gap(10).AlignItems(ui.Start).Children(func() {
						dotCol := th.Accent
						if e.Role == mygoshell.RoleTool {
							dotCol = th.Info
							if e.ToolCard != nil && e.ToolCard.DenialMessage != "" {
								dotCol = th.Danger
							} else if e.ToolCard != nil && e.ToolCard.ReceiptID != "" {
								dotCol = th.Success
							}
						}
						ui.Column(c).Width(14).Padding(8, 0, 0, 0).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(dotCol)
						})
						ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
							if e.Role == mygoshell.RoleThinking {
								renderStandaloneThinkingDrawer(c, rowIdx, e, tr, th)
							} else {
								renderToolCallCard(c, rowIdx, e, tr, th)
							}
						})
					})
				}
			})
		}
	})
}

// extractInlineCitations extracts unique `@cite:<ref>` source tokens from assistant prose.
func extractInlineCitations(text string) []string {
	if !strings.Contains(text, "@cite:") {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, field := range strings.Fields(text) {
		trimmed := strings.Trim(field, ".,;:()[]`\"'")
		if !strings.HasPrefix(trimmed, "@cite:") {
			continue
		}
		ref := strings.TrimSpace(strings.TrimPrefix(trimmed, "@cite:"))
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// renderCitationPillStrip renders inline Vercel AI SDK source-url / HELM @cite pills.
func renderCitationPillStrip(c *ui.Context, key string, cites []string, bc *Context, th theme.Theme) {
	if len(cites) == 0 {
		return
	}
	ui.Row(c.Key(key)).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Sources").Font(kit.FontDisplay).FontSize(11.5).FontWeight(600).TextColor(th.InkMuted).NoWrap()
		for i, cite := range cites {
			ref := cite
			lbl := fmt.Sprintf("[%d] %s", i+1, ref)
			if len(lbl) > 38 {
				lbl = lbl[:35] + "..."
			}
			if theme.ActionButton(c, fmt.Sprintf("%s-cite-%d", key, i), lbl, th.Info, false, th).Clicked() {
				if bc != nil && bc.OnOpenFilePreview != nil {
					bc.OnOpenFilePreview(ref, ref, "", 0)
				} else if bc != nil && bc.OnEcho != nil {
					bc.OnEcho("Source: " + ref)
				}
			}
		}
	})
}

// renderTransientStatusStrip renders a Vercel AI SDK transient live progress
// pulse bar above the composer while a turn is actively streaming, vanishing
// automatically once the turn finishes so history stays clean.
func renderTransientStatusStrip(c *ui.Context, tr *mygoshell.TranscriptController, bc *Context, th theme.Theme) {
	if tr == nil || !tr.InFlight() {
		return
	}
	model := bc.ActiveModel
	if model == "" {
		model = "marlin-core"
	}
	ui.Row(c.Key("conv-transient-pulse")).Height(28).Padding(0, 24).Gap(8).AlignItems(ui.Center).
		Background(kit.WithAlpha(th.Info, 0.09)).
		BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.Info, 0.25)).Children(func() {
		kit.LoadingIndicator(c, 12, th.Info, false)
		ui.Text(c, fmt.Sprintf("Live Stream · %s · Turn #%d", model, tr.ActiveTurn())).
			Font(theme.FontMono).FontSize(11.5).FontWeight(600).TextColor(th.Info).NoWrap()
		ui.Spacer(c)
		ui.Text(c, "transient · 120Hz").Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap()
	})
}

// renderArtifactSplitStage renders the right-hand Layer 3 Sandboxed Generative
// Artifact Stage alongside *conversation* (Claude Artifacts / v0 split stage).
func renderArtifactSplitStage(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.ActiveArtifact == nil {
		return
	}
	art := bc.ActiveArtifact
	title := art.Title
	if title == "" {
		title = art.DataType
	}
	ui.Column(c.Key("conv-artifact-stage")).Grow(2).MinWidth(320).
		Background(theme.GlassTone(c, th.Bar, th, 0.44, 0.72)).
		BorderWidth(0, 0, 0, 1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c.Key("conv-artifact-hdr")).Height(38).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.68)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			renderMetaPill(c, art.DataType, th.Accent, th)
			ui.Text(c, title).Font(kit.FontDisplay).FontSize(13).FontWeight(600).TextColor(th.Ink).Grow(1).NoWrap().MaxLines(1)
			if bc.OnCloseArtifact != nil {
				if theme.ActionButton(c, "conv-artifact-close", "Close", th.InkMuted, false, th).Clicked() {
					bc.OnCloseArtifact()
				}
			}
		})

		// Mount live Layer-3 Sandboxed WebView Artifact Stage when desktop WebHost is active;
		// otherwise render a crisp native GPU structured preview (headless / minimal / WASM).
		if bc.WebHost != nil && bc.WebHost.Resolve("artifact-stage") != nil {
			kit.WebIslandFrame(c, kit.WebIslandConfig{
				Key:        "conv-artifact-webisland",
				IslandID:   "artifact-stage",
				Borderless: true,
				Host:       bc.WebHost,
				Palette:    th.ToKitPalette(),
				Surface:    kit.SurfaceOptions{Glass: bc.Modules.Glass, GlassOpacity: bc.GlassOpacity, Motion: bc.Modules.Motion},
			})
			return
		}

		ui.Scroll(c.Key("conv-artifact-fallback")).Grow(1).Padding(14, 16).Children(func() {
			ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
				ui.Text(c, "Generative UI Artifact Preview").Font(kit.FontDisplay).FontSize(12.5).FontWeight(700).TextColor(th.Info)
				ui.Column(c).Padding(12, 14).Radius(10).
					Background(theme.GlassTone(c, th.Canvas, th, 0.55, 0.84)).
					BorderWidth(1).BorderColor(th.BorderSubtle).Children(func() {
					ui.Text(c, art.Payload).Font(theme.FontMono).FontSize(12).LineHeight(1.5).TextColor(th.Ink)
				})
			})
		})
	})
}
