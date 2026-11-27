// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"strings"

	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/tao3k/marlin-mygo/runtime/kit"
)

// RoleVisualSpec maps an AI agent handle (or human operator) to Marlin's
// canonical Apple 26 Liquid Glass role semantics (SF Symbol + Doom/Tokyo accent).
func RoleVisualSpec(handle string, isAgent bool, th Theme) (roleTag, symbol string, tone ui.Color) {
	if !isAgent {
		return "Human", "person.fill", th.Info
	}
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@")) {
	case "architect":
		return "Architect", "building.columns", th.Accent
	case "reviewer":
		return "Reviewer", "checkmark.seal", th.Success
	case "security":
		return "Security", "shield.lefthalf.filled", th.Warning
	default:
		return "AI", "sparkles", th.Info
	}
}

// RoleEmblemAvatar draws Marlin's unified Apple 26 role-tinted SF Symbol emblem
// for AI agents and clean initial avatar for human operators, with a 5-state
// live presence dot (shared across Desktop and Mobile).
func RoleEmblemAvatar(c *ui.Context, name, handle string, isAgent bool, presence string, sz float32, th Theme) {
	_, sym, tone := RoleVisualSpec(handle, isAgent, th)
	bg := kit.WithAlpha(tone, 0.20)
	border := kit.WithAlpha(tone, 0.42)
	dotCol := PresenceColor(presence, th)
	dotSz := sz * 0.30
	if dotSz < 8.5 {
		dotSz = 8.5
	}

	ui.Box(c).Width(sz).Height(sz).Shrink(0).Radius(sz / 2).AlignItems(ui.Center).Justify(ui.Center).
		Background(bg).Border(1, border).Children(func() {
		if isAgent {
			SFIconWeighted(c, sym, sz*0.46, tone, sfsymbols.Semibold, true, false)
		} else {
			initials := "Y"
			if trimmed := strings.TrimSpace(name); len(trimmed) > 0 {
				initials = strings.ToUpper(trimmed[:1])
			}
			ui.Text(c, initials).Font(kit.FontDisplay).FontSize(sz * 0.42).FontWeight(700).TextColor(tone).SingleLine()
		}
		ui.Box(c).Absolute().Right(-1).Bottom(-1).Width(dotSz).Height(dotSz).Radius(dotSz / 2).
			Background(dotCol).Border(1.5, th.Canvas)
	})
}

// VerifiedAIBadge renders the compact verified AI pill beside AI agent names.
func VerifiedAIBadge(c *ui.Context, th Theme) {
	ui.Row(c).Padding(1.5, 5.5).Gap(3).Radius(RadiusPill).AlignItems(ui.Center).
		Background(kit.WithAlpha(th.Accent, 0.18)).
		Border(1, kit.WithAlpha(th.Accent, 0.32)).Children(func() {
		SFIconWeighted(c, "checkmark.seal", 9, th.Accent, sfsymbols.Semibold, true, false)
		ui.Text(c, "AI").Font(FontMono).FontSize(9).FontWeight(700).TextColor(th.Accent).SingleLine()
	})
}

// ApproverBadge renders the human-only Tier-0 PromptGate Approver shield pill.
func ApproverBadge(c *ui.Context, th Theme) {
	ui.Row(c).Padding(1.5, 5.5).Gap(3).Radius(RadiusPill).AlignItems(ui.Center).
		Background(kit.WithAlpha(th.Success, 0.16)).
		Border(1, kit.WithAlpha(th.Success, 0.30)).Children(func() {
		SFIconWeighted(c, "shield.lefthalf.filled", 9, th.Success, sfsymbols.Semibold, true, false)
		ui.Text(c, "APPROVER").Font(FontMono).FontSize(8.5).FontWeight(700).TextColor(th.Success).SingleLine()
	})
}

// PresenceColor maps Marlin's 5-state presence enum to theme semantic colors.
func PresenceColor(pres string, th Theme) ui.Color {
	switch strings.ToLower(strings.TrimSpace(pres)) {
	case "online", "active":
		return th.Success
	case "thinking":
		return th.Accent
	case "working", "busy":
		return th.Info
	case "waiting-gate":
		return th.Warning
	default:
		return th.InkMuted
	}
}

// PresenceLabel returns a concise human-readable status label for Presence.
func PresenceLabel(pres string) string {
	switch strings.ToLower(strings.TrimSpace(pres)) {
	case "online", "active":
		return "Online"
	case "thinking":
		return "Thinking…"
	case "working", "busy":
		return "Working…"
	case "waiting-gate":
		return "Waiting Gate"
	default:
		return "Offline"
	}
}

// CompactMarkdownOptions returns unified Doom/Tokyo Markdown rendering options
// shared across Desktop and Mobile Agent Rooms.
func CompactMarkdownOptions(th Theme) *markdown.Options {
	return &markdown.Options{
		Theme: &markdown.Theme{
			Font:                kit.FontDisplay,
			CodeFont:            FontMono,
			FontSize:            13.5,
			LineHeight:          1.48,
			CodeLineHeight:      1.40,
			Text:                th.Ink,
			Muted:               th.InkMuted,
			Link:                th.Info,
			Strong:              th.Ink,
			StrongWeight:        600,
			Code:                th.Accent,
			CodeBackground:      kit.WithAlpha(th.Bar, 0.72),
			CodeSize:            0.88,
			BlockSpacing:        0.48,
			ItemSpacing:         0.20,
			QuoteBar:            th.Accent,
			QuoteText:           th.InkMuted,
			QuoteBarWidth:       2.5,
			Rule:                th.BorderSubtle,
			CodeBlockBackground: kit.WithAlpha(th.Bar, 0.85),
			CodeBlockBorder:     th.BorderSubtle,
			CodeBlockRadius:     10,
			CodeBlockPadding:    0.70,
			TableBorder:         th.BorderSubtle,
			TableHeader:         kit.WithAlpha(th.Bar, 0.70),
			TableStripe:         kit.WithAlpha(th.SurfaceElevated, 0.25),
			Checkbox:            th.Success,
		},
		WrapCode: true,
	}
}

// FromMyGoTheme projects an active MyGo *ui.Theme back into a Marlin theme.Theme
// so feature views receiving *ui.Theme inherit exact Doom/Tokyo colors without
// hardcoding palette values.
func FromMyGoTheme(t *ui.Theme) Theme {
	base := DoomOne()
	if t == nil {
		return base
	}
	base.Canvas = t.Background
	base.Bar = t.Surface
	base.SurfaceElevated = t.SurfacePressed
	base.SurfaceHover = t.SurfaceHover
	base.BorderSubtle = t.Border
	base.Border = t.Border
	base.Ink = t.Text
	base.InkMuted = t.TextMuted
	base.Accent = t.Accent
	if t.AccentHover != (ui.Color{}) {
		base.Info = t.AccentHover
	}
	if t.Success != (ui.Color{}) {
		base.Success = t.Success
	}
	if t.Warning != (ui.Color{}) {
		base.Warning = t.Warning
	}
	if t.Danger != (ui.Color{}) {
		base.Danger = t.Danger
	}
	// Light theme detection from canvas luminance.
	base.IsLight = (int(t.Background.R)+int(t.Background.G)+int(t.Background.B))/3 > 160
	return base
}
