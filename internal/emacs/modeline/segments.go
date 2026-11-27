// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package modeline

import "strings"

// SegmentID identifies one declarative Modern Doom Modeline segment,
// mirroring `scheme/ui-modeline/objects.ss` (ModernDoomModelinePresets.) 1:1.
type SegmentID string

const (
	SegBar           SegmentID = "bar"
	SegWindowState   SegmentID = "window-state"
	SegWindowNumber  SegmentID = "window-number"
	SegModals        SegmentID = "modals"
	SegBufferInfo    SegmentID = "buffer-info"
	SegWorkspacePath SegmentID = "workspace-path"
	SegRemoteHost    SegmentID = "remote-host"
	SegPosition      SegmentID = "position"
	SegHelmTitle     SegmentID = "helm-title"
	SegHelmCount     SegmentID = "helm-count"
	SegHelmHelp      SegmentID = "helm-help"

	SegStatusItems SegmentID = "status-items"
	SegDeepThink   SegmentID = "deep-think"
	SegWorkspace   SegmentID = "workspace"
	SegGates       SegmentID = "gates"
	SegModel       SegmentID = "model"
	SegVCS         SegmentID = "vcs"
	SegMajorMode   SegmentID = "major-mode"
	SegBrand       SegmentID = "brand"
)

// Side indicates whether a segment docks on the left or right zone of the bar.
type Side uint8

const (
	SideLeft Side = iota
	SideRight
)

// WidthTier classifies a window pane's responsive horizontal budget (mirroring
// Doom Emacs `doom-modeline-window-width-limit` and Scheme width facts).
type WidthTier uint8

const (
	TierCompact WidthTier = iota // < 56 cols (e.g. 3-split narrow pane)
	TierMedium                   // 56..95 cols (e.g. 2-split pane)
	TierWide                     // >= 96 cols (single window or wide split)
)

// SegmentRule defines the responsive admission & compaction contract for one segment.
type SegmentRule struct {
	ID           SegmentID
	Side         Side
	MinWidth     uint32
	CompactBelow uint32
	KeepPriority uint32
}

// PresetLayout holds the ordered LHS and RHS segments for a Doom Modeline preset.
type PresetLayout struct {
	LHS []SegmentID
	RHS []SegmentID
}

var segmentRules = map[SegmentID]SegmentRule{
	SegBar:           {ID: SegBar, Side: SideLeft, MinWidth: 0, CompactBelow: 0, KeepPriority: 100},
	SegWindowState:   {ID: SegWindowState, Side: SideLeft, MinWidth: 0, CompactBelow: 0, KeepPriority: 95},
	SegWindowNumber:  {ID: SegWindowNumber, Side: SideLeft, MinWidth: 0, CompactBelow: 0, KeepPriority: 95},
	SegModals:        {ID: SegModals, Side: SideLeft, MinWidth: 0, CompactBelow: 96, KeepPriority: 95},
	SegBufferInfo:    {ID: SegBufferInfo, Side: SideLeft, MinWidth: 0, CompactBelow: 56, KeepPriority: 90},
	SegWorkspacePath: {ID: SegWorkspacePath, Side: SideLeft, MinWidth: 0, CompactBelow: 72, KeepPriority: 85},
	SegRemoteHost:    {ID: SegRemoteHost, Side: SideLeft, MinWidth: 72, CompactBelow: 0, KeepPriority: 45},
	SegPosition:      {ID: SegPosition, Side: SideLeft, MinWidth: 96, CompactBelow: 0, KeepPriority: 40},
	SegHelmTitle:     {ID: SegHelmTitle, Side: SideLeft, MinWidth: 0, CompactBelow: 0, KeepPriority: 95},
	SegHelmCount:     {ID: SegHelmCount, Side: SideLeft, MinWidth: 56, CompactBelow: 0, KeepPriority: 70},
	SegHelmHelp:      {ID: SegHelmHelp, Side: SideRight, MinWidth: 72, CompactBelow: 0, KeepPriority: 60},
	SegStatusItems:   {ID: SegStatusItems, Side: SideRight, MinWidth: 72, CompactBelow: 96, KeepPriority: 55},
	SegDeepThink:     {ID: SegDeepThink, Side: SideRight, MinWidth: 72, CompactBelow: 0, KeepPriority: 50},
	SegWorkspace:     {ID: SegWorkspace, Side: SideRight, MinWidth: 64, CompactBelow: 0, KeepPriority: 65},
	SegGates:         {ID: SegGates, Side: SideRight, MinWidth: 0, CompactBelow: 0, KeepPriority: 88},
	SegModel:         {ID: SegModel, Side: SideRight, MinWidth: 0, CompactBelow: 64, KeepPriority: 92},
	SegVCS:           {ID: SegVCS, Side: SideRight, MinWidth: 96, CompactBelow: 0, KeepPriority: 45},
	SegMajorMode:     {ID: SegMajorMode, Side: SideRight, MinWidth: 0, CompactBelow: 56, KeepPriority: 80},
	SegBrand:         {ID: SegBrand, Side: SideRight, MinWidth: 96, CompactBelow: 0, KeepPriority: 35},
}

// LayoutFor returns the canonical Modern Doom Modeline LHS/RHS layout for preset
// (mirroring `scheme/ui-modeline/objects.ss` and `doom-modeline.el` v4.3).
func LayoutFor(preset Preset) PresetLayout {
	switch preset {
	case PresetDashboard:
		return PresetLayout{
			LHS: []SegmentID{SegBar, SegWindowState, SegWindowNumber, SegModals, SegWorkspacePath, SegRemoteHost},
			RHS: []SegmentID{SegStatusItems, SegWorkspace, SegGates, SegModel, SegVCS, SegBrand},
		}
	case PresetHelm:
		return PresetLayout{
			LHS: []SegmentID{SegBar, SegModals, SegHelmTitle, SegHelmCount},
			RHS: []SegmentID{SegHelmHelp},
		}
	case PresetMinimal:
		return PresetLayout{
			LHS: []SegmentID{SegBar, SegWindowNumber, SegModals, SegBufferInfo},
			RHS: []SegmentID{SegMajorMode},
		}
	default:
		return PresetLayout{
			LHS: []SegmentID{SegBar, SegWindowState, SegWindowNumber, SegModals, SegBufferInfo, SegRemoteHost, SegPosition},
			RHS: []SegmentID{SegStatusItems, SegDeepThink, SegWorkspace, SegGates, SegModel, SegVCS, SegMajorMode},
		}
	}
}

// WidthTierFor maps a pane column budget into Compact / Medium / Wide tiers.
func WidthTierFor(widthCols uint32) WidthTier {
	if widthCols == 0 || widthCols >= 96 {
		return TierWide
	}
	if widthCols >= 56 {
		return TierMedium
	}
	return TierCompact
}

// VisibleSegments filters a preset's LHS and RHS segments according to pane
// column budget, multi-window count, and focus state (mirroring Scheme's
// `marlin-ui-modeline-visible-segments`).
func VisibleSegments(preset Preset, widthCols uint32, focused, multi bool) (lhs, rhs []SegmentID) {
	layout := LayoutFor(preset)
	admit := func(seg SegmentID) bool {
		if seg == SegWindowNumber {
			return multi
		}
		if !focused {
			switch seg {
			case SegBar, SegWindowState, SegWindowNumber, SegBufferInfo, SegWorkspacePath, SegMajorMode:
				return true
			default:
				return false
			}
		}
		rule, ok := segmentRules[seg]
		if !ok {
			return true
		}
		return widthCols == 0 || widthCols >= rule.MinWidth
	}

	lhs = make([]SegmentID, 0, len(layout.LHS))
	for _, s := range layout.LHS {
		if admit(s) {
			lhs = append(lhs, s)
		}
	}
	rhs = make([]SegmentID, 0, len(layout.RHS))
	for _, s := range layout.RHS {
		if admit(s) {
			rhs = append(rhs, s)
		}
	}
	return lhs, rhs
}

// ShrinkPath implements Doom Emacs `shrink-path` compression: collapses
// intermediate path segments to their leading character (or 2 chars for dot
// directories like `.config`), preserving root `~` / `/` and the final leaf.
//
// Example: `~/ghq/github.com/tao3k/marlin-agent-core/` -> `~/g/g/t/marlin-agent-core/`
func ShrinkPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.Contains(trimmed, "/") {
		return trimmed
	}
	trailing := len(trimmed) > 1 && strings.HasSuffix(trimmed, "/")
	body := trimmed
	if trailing {
		body = trimmed[:len(trimmed)-1]
	}
	parts := strings.Split(body, "/")
	total := len(parts)
	out := make([]string, total)
	for i, seg := range parts {
		last := i == total-1
		first := i == 0
		switch {
		case last || seg == "":
			out[i] = seg
		case first && (seg == "~" || seg == "." || seg == ".."):
			out[i] = seg
		case len(seg) <= 1:
			out[i] = seg
		case seg[0] == '.' && len(seg) > 1:
			out[i] = seg[:2]
		default:
			out[i] = seg[:1]
		}
	}
	joined := strings.Join(out, "/")
	if trailing {
		return joined + "/"
	}
	return joined
}

// BufferIcon returns a semantic Apple SF Symbol name for a buffer or major mode,
// bringing authentic Doom Modeline nerd-icons visual recognition into native GPU UI.
func BufferIcon(bufName, majorMode string) string {
	switch strings.TrimSpace(bufName) {
	case "*Dashboard*":
		return "square.grid.2x2"
	case "*conversation*":
		return "sparkles"
	case "*Agent Room*":
		return "bubble.left.and.bubble.right"
	case "*Workflow Canvas*":
		return "point.3.connected.trianglepath.dotted"
	case "*Sessions*":
		return "clock.arrow.circlepath"
	case "*Targets*":
		return "network"
	case "*Processes*":
		return "terminal"
	case "*Models*":
		return "cpu"
	case "*Packages*":
		return "shippingbox"
	case "*Settings*":
		return "slider.horizontal.3"
	case "*Messages*":
		return "text.alignleft"
	case "*Help*":
		return "questionmark.circle"
	case "*Preview*":
		return "doc.text.magnifyingglass"
	}
	if strings.EqualFold(majorMode, "Scheme-Ext") || strings.HasPrefix(bufName, "scheme-ext:") {
		return "puzzlepiece.extension"
	}
	return "doc.plaintext"
}
