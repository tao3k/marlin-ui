// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/tao3k/marlin-mygo/engine/uistream"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func buildConversationMarkdownTheme(th theme.Theme) *markdown.Theme {
	codeBg := kit.WithAlpha(th.SurfaceElevated, 0.72)
	blockBg := kit.WithAlpha(th.Bar, 0.88)
	return &markdown.Theme{
		Font:                kit.FontDisplay,
		CodeFont:            theme.FontMono,
		FontSize:            14.5,
		LineHeight:          1.58,
		CodeLineHeight:      1.45,
		Text:                th.Ink,
		Muted:               th.InkMuted,
		Link:                th.Info,
		Strong:              th.Ink,
		StrongWeight:        600,
		Code:                th.Accent,
		CodeBackground:      codeBg,
		CodeSize:            0.88,
		BlockSpacing:        0.72,
		ItemSpacing:         0.28,
		QuoteBar:            th.Accent,
		QuoteText:           th.InkMuted,
		QuoteBarWidth:       3,
		Rule:                th.BorderSubtle,
		CodeBlockBackground: blockBg,
		CodeBlockBorder:     th.BorderSubtle,
		CodeBlockRadius:     8,
		CodeBlockPadding:    0.85,
		TableBorder:         th.BorderSubtle,
		TableHeader:         codeBg,
		TableStripe:         kit.WithAlpha(th.SurfaceElevated, 0.25),
		Checkbox:            th.Success,
	}
}

func renderConversation(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.Adaptive == nil || bc.Adaptive.Desktop == nil || bc.Adaptive.Desktop.Transcript == nil {
		return
	}
	tr := bc.Adaptive.Desktop.Transcript
	n := tr.Len()
	if n == 0 {
		renderEmptyConversationStage(c, tr, th, bc)
		return
	}

	mdOpts := &markdown.Options{
		Theme:          buildConversationMarkdownTheme(th),
		WrapCode:       true,
		ShareSelection: true,
		CodeBlock: func(ctx *ui.Context, b *markdown.CodeBlockView) {
			renderEditorialCodeBlock(ctx, b, tr, th)
		},
	}

	ui.Column(c.Key("conv-root")).Fill().AlignItems(ui.Stretch).Children(func() {
		renderConversationHeaderBar(c, th, bc)

		// Windowed virtualized slice: cap per-frame layout work to the most recent
		// 60 entries (expandable on demand) so 10k-turn histories keep 120Hz frame budgets.
		const defaultVisibleTail = 60
		startIdx := 0
		if n > defaultVisibleTail {
			showAll := bc.IsThinkingOpen != nil && bc.IsThinkingOpen("conv-show-all-history")
			if !showAll {
				startIdx = n - defaultVisibleTail
			}
		}

		ui.Row(c.Key("conv-main-split")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c.Key("conv-scroll")).Grow(3).MinWidth(0).Padding(20, 28).Children(func() {
				ui.Column(c).MaxWidth(860).Gap(18).AlignItems(ui.Stretch).Children(func() {
					if startIdx > 0 {
						ui.Row(c.Key("conv-load-earlier")).Justify(ui.Center).Children(func() {
							lbl := fmt.Sprintf("Show %d earlier turn(s)", startIdx)
							if theme.ActionButton(c, "conv-show-earlier-btn", lbl, th.Info, false, th).Clicked() && bc.ToggleThinking != nil {
								bc.ToggleThinking("conv-show-all-history")
							}
						})
					}
					for i := startIdx; i < n; {
						entry, ok := tr.Entry(i)
						if !ok {
							i++
							continue
						}
						// Group contiguous Thinking + Tool steps into one high-craft Execution Trace stepper.
						if entry.Role == mygoshell.RoleThinking || entry.Role == mygoshell.RoleTool {
							groupStart := i
							var group []mygoshell.TranscriptEntry
							for i < n {
								e, ok := tr.Entry(i)
								if !ok || (e.Role != mygoshell.RoleThinking && e.Role != mygoshell.RoleTool) {
									break
								}
								group = append(group, e)
								i++
							}
							if len(group) >= 2 {
								renderExecutionTraceGroup(c, groupStart, group, tr, bc, th)
							} else if len(group) == 1 {
								if group[0].Role == mygoshell.RoleThinking {
									renderStandaloneThinkingDrawer(c, groupStart, group[0], tr, th)
								} else {
									renderToolCallCard(c, groupStart, group[0], tr, th)
								}
							}
							continue
						}
						switch entry.Role {
						case mygoshell.RoleUser:
							renderUserMessageCard(c, i, entry, th)
						case mygoshell.RoleGate:
							renderHumanGateCard(c, i, entry, bc, th)
						default:
							renderAssistantMessageCard(c, i, entry, tr, bc, mdOpts, th)
						}
						i++
					}
				})
			})

			if bc.ActiveArtifact != nil {
				renderArtifactSplitStage(c, th, bc)
			}
		})

		renderTransientStatusStrip(c, tr, bc, th)
		renderConversationComposerBar(c, tr, bc, th)
	})
}

func renderConversationHeaderBar(c *ui.Context, th theme.Theme, bc *Context) {
	modelLabel := bc.ActiveModel
	if modelLabel == "" && bc.ModelCfg.Loaded {
		modelLabel = bc.ModelCfg.SummaryLabel()
	}
	if modelLabel == "" {
		modelLabel = "marlin-core (default)"
	}

	connLabel := "Offline Preview"
	connTone := th.Warning
	if bc.Link != nil {
		switch bc.Link.Mode {
		case hostlink.ModeSidecar:
			connLabel = "Rust gRPC · " + bc.Link.Endpoint
			connTone = th.Success
		case hostlink.ModeRemoteGRPC:
			connLabel = "Remote gRPC · " + bc.Link.Endpoint
			connTone = th.Info
		}
	}

	ui.Row(c.Key("conv-header-bar")).Height(36).Padding(0, 24).Gap(10).AlignItems(ui.Center).
		Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.56)).
		BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
		ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(connTone)
		ui.Text(c, modelLabel).Font(kit.FontDisplay).FontSize(13).FontWeight(600).TextColor(th.Ink).NoWrap()

		if bc.ModelCfg.Loaded {
			if proto := strings.TrimSpace(bc.ModelCfg.Default.Protocol); proto != "" {
				renderMetaPill(c, proto, th.Accent, th)
			}
			if think := bc.ModelCfg.ThinkingSummary(); think != "" {
				renderMetaPill(c, "thinking: "+think, th.Info, th)
			}
			if bc.ModelCfg.Default.CredentialEnv != "" {
				if bc.ModelCfg.Default.CredentialConfigured(nil) {
					renderMetaPill(c, bc.ModelCfg.Default.CredentialEnv+" ready", th.Success, th)
				} else {
					renderMetaPill(c, bc.ModelCfg.Default.CredentialEnv+" unset", th.Warning, th)
				}
			}
		}

		ui.Spacer(c)

		ui.Text(c, connLabel).Font(theme.FontMono).FontSize(11.5).TextColor(th.InkMuted).NoWrap()
		if bc.OnSelectBuffer != nil {
			if theme.ActionLink(c, "conv-open-models", "Models", th.Info, th).Clicked() {
				bc.OnSelectBuffer(Models)
			}
		}
	})
}

func renderMetaPill(c *ui.Context, text string, tone ui.Color, th theme.Theme) {
	ui.Row(c).Padding(2, 8).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.40, 0.62)).Children(func() {
		ui.Text(c, text).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(tone).NoWrap()
	})
}

func renderUserMessageCard(c *ui.Context, idx int, entry mygoshell.TranscriptEntry, th theme.Theme) {
	ui.Row(c.Key(fmt.Sprintf("conv-user-%d", idx))).Justify(ui.End).Children(func() {
		ui.Column(c).MaxWidth(680).Padding(12, 16).Gap(6).Radius(14).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.52, 0.78)).
			BorderWidth(1).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.45, 0.65)).
			AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "You").Font(kit.FontDisplay).FontSize(12).FontWeight(700).TextColor(th.Info).NoWrap()
				if len(entry.Attachments) > 0 {
					renderMetaPill(c, fmt.Sprintf("%d attachment(s)", len(entry.Attachments)), th.InkMuted, th)
				}
			})
			ui.Text(c, entry.Text).Font(kit.FontDisplay).FontSize(14.5).LineHeight(1.52).TextColor(th.Ink)
		})
	})
}

func renderAssistantMessageCard(
	c *ui.Context,
	idx int,
	entry mygoshell.TranscriptEntry,
	tr *mygoshell.TranscriptController,
	bc *Context,
	mdOpts *markdown.Options,
	th theme.Theme,
) {
	thinkingText, rawBodyText, thinkingStreaming := uistream.SplitThinkTags(entry.Text)
	dataParts, bodyText := uistream.ExtractDataParts(rawBodyText)
	isStubGateway := strings.Contains(bodyText, "[marlin-core] processed ") &&
		strings.Contains(bodyText, "via durable ConversationJournal.")

	ui.Column(c.Key(fmt.Sprintf("conv-asst-%d", idx))).Padding(6, 4, 14, 4).Gap(10).
		BorderWidth(0, 0, 1, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.55)).
		AlignItems(ui.Stretch).Children(func() {
		// Role header bar with live streaming indicator + Copy button.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Padding(2, 8).Radius(theme.RadiusPill).
				Background(kit.WithAlpha(th.Accent, 0.16)).Children(func() {
				ui.Text(c, "Marlin").Font(kit.FontDisplay).FontSize(12).FontWeight(700).TextColor(th.Accent).NoWrap()
			})
			if entry.Streaming {
				kit.LoadingIndicator(c, 13, th.Info, false)
				statusLbl := "Streaming..."
				if thinkingStreaming {
					statusLbl = "Reasoning..."
				}
				ui.Text(c, statusLbl).Font(kit.FontDisplay).FontSize(12).TextColor(th.Info).NoWrap()
			} else if entry.OutcomeLabel != "" && entry.OutcomeLabel != "SESSION_TURN_OUTCOME_COMPLETED" {
				renderMetaPill(c, entry.OutcomeLabel, th.InkMuted, th)
			}
			ui.Spacer(c)
			if !entry.Streaming && entry.TurnOrdinal > 0 && tr.OnRetryTurn != nil {
				if theme.ActionLink(c, fmt.Sprintf("conv-retry-%d", idx), "Retry", th.InkMuted, th).Clicked() {
					tr.OnRetryTurn(entry.TurnOrdinal)
				}
			}
			if !entry.Streaming && entry.TurnOrdinal > 0 && tr.OnForkTurn != nil {
				if theme.ActionLink(c, fmt.Sprintf("conv-fork-%d", idx), "Fork", th.InkMuted, th).Clicked() {
					tr.OnForkTurn(entry.TurnOrdinal)
				}
			}
			if strings.TrimSpace(bodyText) != "" && tr.OnCopyCode != nil {
				if theme.ActionLink(c, fmt.Sprintf("conv-copy-%d", idx), "Copy", th.InkMuted, th).Clicked() {
					tr.OnCopyCode(bodyText)
				}
			}
		})

		// Collapsible Thinking / Reasoning block (folded by default like modern Agent platforms).
		if strings.TrimSpace(thinkingText) != "" {
			renderCollapsibleThinkingBox(c, fmt.Sprintf("think-inline-%d", idx), thinkingText, thinkingStreaming, bc, th)
		}

		// Rich proportional Markdown body.
		if strings.TrimSpace(bodyText) != "" {
			if entry.Streaming {
				markdown.Stream(c.Key(fmt.Sprintf("conv-md-%d", idx)), bodyText, mdOpts)
			} else {
				markdown.View(c.Key(fmt.Sprintf("conv-md-%d", idx)), bodyText, mdOpts)
			}
		}

		// Layer 2 -> Layer 3 Vercel AI SDK Generative UI Data Parts (data-mermaid, data-workflow, etc.).
		for dIdx, dp := range dataParts {
			renderGenerativeDataPartCard(c, fmt.Sprintf("conv-gen-%d-%d", idx, dIdx), dp, bc, th)
		}

		// Inline source citations pill strip (@cite:<path:line>).
		if cites := extractInlineCitations(bodyText); len(cites) > 0 {
			renderCitationPillStrip(c, fmt.Sprintf("conv-cites-%d", idx), cites, bc, th)
		}

		// Transparent notice when Rust host is running DeterministicLocalGateway.
		if isStubGateway {
			renderDeterministicGatewayNotice(c, bc, th)
		}

		// Structured cancellation receipt evidence if turn was cancelled mid-flight.
		if entry.Cancellation != nil {
			rc := entry.Cancellation
			ui.Row(c.Key(fmt.Sprintf("conv-cancel-%d", idx))).Padding(8, 12).Radius(theme.RadiusControl).Gap(8).
				Background(kit.WithAlpha(th.Warning, 0.10)).
				BorderWidth(1).BorderColor(kit.WithAlpha(th.Warning, 0.35)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Turn Cancelled").Font(kit.FontDisplay).FontSize(12).FontWeight(700).TextColor(th.Warning).NoWrap()
				ui.Text(c, fmt.Sprintf("status=%s · tracked=%d remaining=%d", rc.Status, rc.TrackedTasks, rc.RemainingTasks)).
					Font(theme.FontMono).FontSize(11.5).TextColor(th.InkMuted).NoWrap()
			})
		}
	})
}

// renderCollapsibleThinkingBox renders a folded-by-default reasoning drawer.
func renderCollapsibleThinkingBox(c *ui.Context, key, reasoning string, streaming bool, bc *Context, th theme.Theme) {
	isOpen := false
	if bc != nil && bc.IsThinkingOpen != nil {
		isOpen = bc.IsThinkingOpen(key)
	}
	arrow := "▸"
	if isOpen {
		arrow = "▾"
	}
	title := fmt.Sprintf("%s Thought Process", arrow)
	meta := fmt.Sprintf("%d chars · click to expand", len(reasoning))
	if streaming {
		title = fmt.Sprintf("%s Thinking...", arrow)
		meta = "streaming reasoning trace"
	} else if isOpen {
		meta = "click to collapse"
	}

	ui.Column(c.Key(key)).Radius(10).
		Background(theme.GlassTone(c, th.Bar, th, 0.42, 0.68)).
		BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key(key+"-hdr")).Label(title).Height(32).Padding(0, 12).Gap(8).
			Radius(10).AlignItems(ui.Center).Justify(ui.Start)
		if hdr.Hovered() {
			hdr.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.56))
		}
		hdr.Children(func() {
			if streaming {
				kit.LoadingIndicator(c, 12, th.Accent, false)
			}
			ui.Text(c, title).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.InkMuted).NoWrap()
			ui.Spacer(c)
			ui.Text(c, meta).Font(theme.FontMono).FontSize(11).TextColor(th.BannerInk).NoWrap()
		})
		if hdr.Clicked() && bc != nil && bc.ToggleThinking != nil {
			bc.ToggleThinking(key)
		}

		if isOpen {
			ui.Column(c.Key(key+"-body")).Padding(8, 14, 12, 14).
				BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Padding(0, 0, 0, 10).BorderWidth(0, 0, 0, 2).BorderColor(th.Accent).Children(func() {
					ui.Text(c, strings.TrimSpace(reasoning)).
						Font(kit.FontDisplay).FontSize(13).LineHeight(1.5).TextColor(th.InkMuted)
				})
			})
		}
	})
}

func renderStandaloneThinkingDrawer(c *ui.Context, idx int, entry mygoshell.TranscriptEntry, tr *mygoshell.TranscriptController, th theme.Theme) {
	isOpen := !entry.Folded
	arrow := "▸"
	if isOpen {
		arrow = "▾"
	}
	label := entry.Title
	if label == "" {
		label = "Thought Process"
	}
	headerText := fmt.Sprintf("%s %s", arrow, label)
	ui.Column(c.Key(fmt.Sprintf("conv-think-row-%d", idx))).Radius(10).
		Background(theme.GlassTone(c, th.Bar, th, 0.42, 0.68)).
		BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key(fmt.Sprintf("conv-think-btn-%d", idx))).Label(headerText).
			Height(32).Padding(0, 12).Gap(8).Radius(10).AlignItems(ui.Center).Justify(ui.Start)
		if hdr.Hovered() {
			hdr.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.56))
		}
		hdr.Children(func() {
			ui.Text(c, headerText).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.InkMuted).NoWrap()
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d chars", len(entry.Text))).Font(theme.FontMono).FontSize(11).TextColor(th.BannerInk).NoWrap()
		})
		if hdr.Clicked() {
			tr.ToggleFold(idx)
		}
		if isOpen && strings.TrimSpace(entry.Text) != "" {
			ui.Column(c).Padding(8, 14, 12, 14).BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).Children(func() {
				ui.Text(c, strings.TrimSpace(entry.Text)).Font(kit.FontDisplay).FontSize(13).LineHeight(1.5).TextColor(th.InkMuted)
			})
		}
	})
}

func renderToolCallCard(c *ui.Context, idx int, entry mygoshell.TranscriptEntry, tr *mygoshell.TranscriptController, th theme.Theme) {
	card := entry.ToolCard
	title := entry.Title
	phase := entry.OutcomeLabel
	summary := entry.Text
	receipt := ""
	denial := ""
	if card != nil {
		if card.Title != "" {
			title = card.Title
		} else if card.ToolName != "" {
			title = card.ToolName
		}
		if card.Phase != "" {
			phase = card.Phase
		}
		if card.Summary != "" {
			summary = card.Summary
		}
		receipt = card.ReceiptID
		denial = card.DenialMessage
	}
	if title == "" {
		title = "Tool Call"
	}
	phaseTone := th.Info
	if denial != "" || strings.Contains(strings.ToUpper(phase), "DENIED") || strings.Contains(strings.ToUpper(phase), "FAIL") {
		phaseTone = th.Danger
	} else if strings.Contains(strings.ToUpper(phase), "COMPLET") || strings.Contains(strings.ToUpper(phase), "OK") {
		phaseTone = th.Success
	} else if strings.Contains(strings.ToUpper(phase), "GATE") || strings.Contains(strings.ToUpper(phase), "WAIT") {
		phaseTone = th.Warning
	}

	arrow := "▸"
	if !entry.Folded {
		arrow = "▾"
	}

	ui.Column(c.Key(fmt.Sprintf("conv-tool-%d", idx))).Radius(10).
		Background(theme.GlassTone(c, th.Bar, th, 0.46, 0.72)).
		BorderWidth(1).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key(fmt.Sprintf("conv-tool-btn-%d", idx))).Label(title).
			Height(34).Padding(0, 12).Gap(8).Radius(10).AlignItems(ui.Center).Justify(ui.Start)
		if hdr.Hovered() {
			hdr.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.56))
		}
		hdr.Children(func() {
			ui.Text(c, arrow+" ⚙").Font(theme.FontMono).FontSize(12).TextColor(phaseTone).NoWrap()
			ui.Text(c, title).Font(theme.FontMono).FontSize(12.5).FontWeight(700).TextColor(th.Ink).NoWrap()
			if phase != "" {
				renderMetaPill(c, phase, phaseTone, th)
			}
			ui.Spacer(c)
			if receipt != "" {
				renderMetaPill(c, "receipt:"+receipt, th.Success, th)
			}
		})
		if hdr.Clicked() {
			tr.ToggleFold(idx)
		}

		if !entry.Folded {
			ui.Column(c).Padding(8, 14, 12, 14).Gap(6).
				BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
				if strings.TrimSpace(summary) != "" {
					ui.Text(c, strings.TrimSpace(summary)).Font(theme.FontMono).FontSize(12.5).LineHeight(1.45).TextColor(th.Ink)
				}
				if denial != "" {
					ui.Text(c, "Denied: "+denial).Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Danger)
				}
			})
		}
	})
}

func renderHumanGateCard(c *ui.Context, idx int, entry mygoshell.TranscriptEntry, bc *Context, th theme.Theme) {
	gate := entry.HumanGate
	prompt := entry.Text
	gateID := entry.Title
	resolved := false
	receipt := ""
	if gate != nil {
		if gate.Prompt != "" {
			prompt = gate.Prompt
		}
		if gate.GateID != "" {
			gateID = gate.GateID
		}
		resolved = gate.Resolved
		receipt = gate.ReceiptID
	}
	tone := th.Warning
	badge := "APPROVAL REQUIRED"
	if resolved {
		tone = th.Success
		badge = "RESOLVED"
		if receipt != "" {
			badge = "RESOLVED · " + receipt
		}
	}

	ui.Column(c.Key(fmt.Sprintf("conv-gate-%d", idx))).Padding(12, 14).Gap(8).Radius(10).
		Background(kit.WithAlpha(tone, 0.10)).
		BorderWidth(1).BorderColor(kit.WithAlpha(tone, 0.42)).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Approval Gate").Font(kit.FontDisplay).FontSize(13).FontWeight(700).TextColor(tone).NoWrap()
			renderMetaPill(c, badge, tone, th)
			ui.Spacer(c)
			if gateID != "" {
				ui.Text(c, gateID).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap()
			}
		})
		if strings.TrimSpace(prompt) != "" {
			ui.Text(c, prompt).Font(kit.FontDisplay).FontSize(13.5).LineHeight(1.48).TextColor(th.Ink)
		}
		if !resolved && bc != nil && bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.OnGateDecision != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if theme.ActionButton(c, fmt.Sprintf("conv-gate-approve-%d", idx), "Approve", th.Success, true, th).Clicked() {
					bc.Adaptive.Desktop.OnGateDecision(gateID, true)
				}
				if theme.ActionButton(c, fmt.Sprintf("conv-gate-reject-%d", idx), "Reject", th.Danger, false, th).Clicked() {
					bc.Adaptive.Desktop.OnGateDecision(gateID, false)
				}
			})
		}
	})
}

func renderGenerativeDataPartCard(c *ui.Context, key string, dp uistream.Part, bc *Context, th theme.Theme) {
	rawPayload := strings.TrimSpace(string(dp.Data))
	preview := rawPayload
	if len(preview) > 220 {
		preview = preview[:220] + "..."
	}
	ui.Column(c.Key(key)).Padding(10, 14).Gap(8).Radius(10).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.72)).
		BorderWidth(1).BorderColor(kit.WithAlpha(th.Accent, 0.36)).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			renderMetaPill(c, "AI SDK · "+dp.DataType, th.Accent, th)
			ui.Text(c, "Generative UI Part").Font(kit.FontDisplay).FontSize(12.5).FontWeight(600).TextColor(th.Ink).NoWrap()
			ui.Spacer(c)
			if bc != nil && bc.OnOpenArtifact != nil {
				if theme.ActionButton(c, key+"-preview-stage", "Preview Stage", th.Accent, true, th).Clicked() {
					bc.OnOpenArtifact(dp.DataType, dp.DataType, rawPayload)
				}
			}
			if bc != nil && bc.OnWidgetAction != nil {
				if theme.ActionButton(c, key+"-open-canvas", "Open Canvas", th.Info, false, th).Clicked() {
					bc.OnWidgetAction("feature:feature:webisland")
				}
			}
		})
		if strings.TrimSpace(preview) != "" {
			ui.Text(c, preview).Font(theme.FontMono).FontSize(12).LineHeight(1.45).TextColor(th.InkMuted).MaxLines(5)
		}
	})
}

func renderConversationComposerBar(c *ui.Context, tr *mygoshell.TranscriptController, bc *Context, th theme.Theme) {
	deepThink := tr.DeepThinkEnabled()
	inFlight := tr.InFlight()
	usage := tr.ContextUsage()
	tokText := "0 tok"
	if usage.UsedTokens > 0 {
		maxK := usage.MaxTokens / 1000
		if maxK <= 0 {
			maxK = 200
		}
		tokText = fmt.Sprintf("%d / %dk tok", usage.UsedTokens, maxK)
	}

	submitDraft := func() {
		if bc == nil || bc.ComposerDraft == nil || bc.OnSubmitPrompt == nil {
			return
		}
		text := strings.TrimSpace(*bc.ComposerDraft)
		if text == "" {
			return
		}
		*bc.ComposerDraft = ""
		bc.OnSubmitPrompt(text)
	}

	ui.Row(c.Key("conv-composer-dock")).Padding(10, 24, 12, 24).Justify(ui.Center).
		Background(theme.GlassTone(c, th.Bar, th, 0.24, 0.44)).
		BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.70)).Children(func() {
		ui.Column(c.Key("conv-composer-card")).Grow(1).MaxWidth(860).Padding(10, 14, 8, 14).Gap(8).Radius(14).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.56, 0.84)).
			BorderWidth(1).BorderColor(theme.GlassTone(c, th.Border, th, 0.50, 0.72)).
			AlignItems(ui.Stretch).Children(func() {
			if tr.Composer != nil {
				if atts := tr.Composer.Snapshot().Attachments; len(atts) > 0 {
					ui.Row(c.Key("conv-composer-atts")).Gap(6).AlignItems(ui.Center).Children(func() {
						for _, a := range atts {
							att := a
							ui.Row(c.Key("att-chip-"+att.ID)).Padding(2, 8).Gap(6).Radius(theme.RadiusPill).
								Background(kit.WithAlpha(th.Info, 0.14)).
								BorderWidth(1).BorderColor(kit.WithAlpha(th.Info, 0.32)).
								AlignItems(ui.Center).Children(func() {
								ui.Text(c, att.Name).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Info).NoWrap()
								if theme.ActionLink(c, "att-rm-"+att.ID, "×", th.InkMuted, th).Clicked() {
									tr.Composer.RemoveAttachment(att.ID)
								}
							})
						}
					})
				}
			}
			if bc != nil && bc.ComposerDraft != nil {
				inp := ui.TextInputBase(c.Key("conv-composer-input"), bc.ComposerDraft).
					Placeholder("Ask Marlin anything… (@cite file, Enter to send, ESC for Normal)").
					Font(kit.FontDisplay).FontSize(14).TextColor(th.Ink).Grow(1)
				if inp.Submitted() {
					submitDraft()
				}
			}
			ui.Row(c.Key("conv-composer-toolbar")).Gap(8).AlignItems(ui.Center).Children(func() {
				dtTone := th.InkMuted
				if deepThink {
					dtTone = th.Accent
				}
				if theme.ActionButton(c, "conv-deep-think", "Deep Think", dtTone, deepThink, th).Clicked() {
					if bc != nil && bc.OnToggleDeepThink != nil {
						bc.OnToggleDeepThink()
					} else {
						tr.SetDeepThink(!deepThink)
					}
				}
				if bc != nil && bc.OnOpenHelm != nil {
					if theme.ActionButton(c, "conv-cite-helm", "@ Cite", th.Info, false, th).Clicked() {
						bc.OnOpenHelm()
					}
				}
				if tr.OnPickAttachment != nil {
					if theme.ActionButton(c, "conv-attach-btn", "+ Attach", th.Info, false, th).Clicked() {
						tr.OnPickAttachment()
					}
				}
				if inFlight && bc != nil && bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.OnCancelTurn != nil {
					if theme.ActionButton(c, "conv-stop-turn", "■ Stop", th.Danger, true, th).Clicked() {
						bc.Adaptive.Desktop.OnCancelTurn()
					}
				}
				ui.Spacer(c)
				ui.Text(c, tokText).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap()
				if bc != nil && bc.ComposerDraft != nil && bc.OnSubmitPrompt != nil {
					hasDraft := strings.TrimSpace(*bc.ComposerDraft) != ""
					sendTone := th.InkMuted
					if hasDraft {
						sendTone = th.Accent
					}
					if theme.ActionButton(c, "conv-send-btn", "Send ↵", sendTone, hasDraft, th).Clicked() {
						submitDraft()
					}
				} else if bc != nil && bc.OnWidgetAction != nil {
					if theme.ActionLink(c, "conv-compose-insert", "Compose (i)", th.Info, th).Clicked() {
						bc.OnWidgetAction("cmd:insert")
					}
				}
			})
		})
	})
}

func renderDeterministicGatewayNotice(c *ui.Context, bc *Context, th theme.Theme) {
	routeInfo := "example.toml not loaded — pass -config ./example.toml"
	if bc != nil && bc.ModelCfg.Loaded {
		routeInfo = fmt.Sprintf("TOML route ready: %s (%s)", bc.ModelCfg.SummaryLabel(), bc.ModelCfg.SourcePath)
	}
	ui.Column(c).Padding(10, 12).Gap(4).Radius(8).
		Background(kit.WithAlpha(th.Info, 0.08)).
		BorderWidth(1).BorderColor(kit.WithAlpha(th.Info, 0.28)).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, "Rust Host Info: DeterministicLocalGateway Active").
			Font(kit.FontDisplay).FontSize(12).FontWeight(700).TextColor(th.Info)
		ui.Text(c, "gRPC turn streaming is live, but the connected marlin-mygo-host binary currently uses DeterministicLocalGateway instead of a live DeepSeek/Anthropic/OpenAI provider. "+routeInfo).
			Font(kit.FontDisplay).FontSize(12).LineHeight(1.45).TextColor(th.InkMuted)
	})
}

func renderEmptyConversationStage(c *ui.Context, tr *mygoshell.TranscriptController, th theme.Theme, bc *Context) {
	modelLabel := bc.ActiveModel
	if modelLabel == "" && bc.ModelCfg.Loaded {
		modelLabel = bc.ModelCfg.SummaryLabel()
	}
	if modelLabel == "" {
		modelLabel = "marlin-core"
	}

	starters := []struct {
		title  string
		desc   string
		prompt string
	}{
		{
			title:  "Architecture Audit",
			desc:   "Map crate, Scheme & UI boundaries",
			prompt: "Summarize the current workspace architecture and highlight any open protocol or UI gaps.",
		},
		{
			title:  "Deep Code Review",
			desc:   "Inspect recent diffs & invariants",
			prompt: "Review recent uncommitted changes across this repository and flag concurrency or contract risks.",
		},
		{
			title:  "Live Helm Lookup",
			desc:   "Search symbols via Rust index",
			prompt: "",
		},
	}

	ui.Column(c.Key("conv-empty-root")).Fill().AlignItems(ui.Stretch).Children(func() {
		renderConversationHeaderBar(c, th, bc)

		ui.Column(c.Key("conv-empty-hero")).Grow(1).MinHeight(0).Padding(24, 32).
			Justify(ui.Center).AlignItems(ui.Center).Gap(20).Children(func() {
			ui.Column(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Row(c).Padding(3, 10).Radius(theme.RadiusPill).Gap(6).AlignItems(ui.Center).
					Background(kit.WithAlpha(th.Accent, 0.14)).
					BorderWidth(1).BorderColor(kit.WithAlpha(th.Accent, 0.30)).Children(func() {
					ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(th.Success)
					ui.Text(c, modelLabel).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(th.Accent).NoWrap()
				})
				ui.Text(c, "What would you like to build with Marlin?").
					Font(kit.FontDisplay).FontSize(22).FontWeight(700).TextColor(th.Ink).NoWrap()
				ui.Text(c, "Markdown streaming, folded reasoning traces, Rust Helm search, and Doom workspaces ready.").
					Font(kit.FontDisplay).FontSize(13.5).TextColor(th.InkMuted).NoWrap()
			})

			ui.Row(c.Key("conv-starter-cards")).MaxWidth(820).Gap(12).AlignItems(ui.Stretch).Children(func() {
				for idx, st := range starters {
					item := st
					card := kit.ActionControl(ui.ButtonBase(c.Key(fmt.Sprintf("conv-starter-%d", idx))).Label(item.title).
						Grow(1).Padding(14, 16).Radius(12).AlignItems(ui.Start).Justify(ui.Start))
					if card.Hovered() {
						card.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.48, 0.72))
					} else {
						card.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.62))
					}
					card.BorderWidth(1).BorderColor(theme.GlassTone(c, th.BorderSubtle, th, 0.50, 0.68))
					card.Children(func() {
						ui.Column(c).Gap(4).AlignItems(ui.Start).Children(func() {
							ui.Text(c, item.title).Font(kit.FontDisplay).FontSize(13).FontWeight(700).TextColor(th.Ink).NoWrap()
							ui.Text(c, item.desc).Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted).NoWrap()
						})
					})
					if card.Clicked() {
						if item.prompt != "" && bc.OnSubmitPrompt != nil {
							bc.OnSubmitPrompt(item.prompt)
						} else if bc.OnOpenHelm != nil {
							bc.OnOpenHelm()
						}
					}
				}
			})

			ui.Row(c.Key("conv-empty-shortcuts")).Gap(16).AlignItems(ui.Center).Children(func() {
				shortcuts := [][2]string{
					{"SPC s", "Helm Search"},
					{"SPC w v", "Split Pane"},
					{"SPC l c", "New Workspace"},
					{"SPC b m", "Switch Model"},
				}
				for _, sc := range shortcuts {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Padding(2, 6).Radius(theme.RadiusSmall).
							Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.40, 0.62)).
							BorderWidth(1).BorderColor(th.BorderSubtle).Children(func() {
							ui.Text(c, sc[0]).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Info).NoWrap()
						})
						ui.Text(c, sc[1]).Font(kit.FontDisplay).FontSize(12).TextColor(th.InkMuted).NoWrap()
					})
				}
			})
		})

		renderConversationComposerBar(c, tr, bc, th)
	})
}
