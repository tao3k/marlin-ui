// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package modeline implements Marlin UI's Modern Doom Modeline
// (mirroring doom-modeline.el v4.3's declarative presets + AI-native telemetry,
// authentic 4px state bar, Meow circular modal emblem, per-window active/inactive
// rendering, winum circular window-number badges, Emacs 30 dedicated-pin state,
// Doom shrink-path, and 3-tier responsive width alignment).
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A07 (emacsModeline).
package modeline

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mapper"

	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// Preset selects the active doom-modeline layout (mirroring doom-modeline.el
// and scheme/ui-modeline/objects.ss).
type Preset uint8

const (
	PresetDashboard Preset = iota // doom-modeline-def-modeline 'dashboard
	PresetMain                    // doom-modeline-def-modeline 'main
	PresetHelm                    // doom-modeline-def-modeline 'helm
	PresetMinimal                 // doom-modeline-def-modeline 'minimal
)

// Status describes all live Modern Doom Modeline segments projected from
// Scheme/Rust gRPC + local Emacs frame state.
type Status struct {
	Preset            Preset
	KeyMode           meow.Mode
	MiniMode          minibuffer.Mode
	Keypad            string
	BufferName        string
	MajorMode         string
	WorkspaceName     string
	WorkspacePath     string
	VCSBranch         string
	RemoteHost        string
	ActivityLabel     string
	ActivityColor     ui.Color
	AgentWorking      bool
	HasUnsavedDraft   bool
	ReadOnly          bool
	DeepThink         bool
	ShowDeepThink     bool
	ModelLabel        string
	StatusItems       []schema.UIStatusItem // declarative marlin.ui.v1 status items when provided by Rust/Scheme
	SessionID         string
	RowCount          int
	TokenCount        int
	HelmTotal         int
	PendingGates      int
	ShowGateBadge     bool
	OnReviewGate      func()
	OnOpenModels      func()
	OnOpenWorkspaces  func()
	OnToggleDeepThink func()
	OnToggleDedicated func()
	VersionLabel      string
	Glass             bool
	GlassOpacity      float32
	Theme             theme.Theme

	// Responsive width budget in monospace columns (e.g. 120 single window,
	// ~60 on 2-pane vertical split, ~40 on 3-pane split). Zero defaults to 120.
	WidthCols uint32

	// Multi-window Doom Modeline fields (mirroring doom-modeline-def-segment
	// window-number, window-state, and doom-modeline--active):
	//   - WindowCount <= 1: single window -> hides window number (never "W1").
	//   - WindowCount > 1:  per-window modeline -> renders circular window number
	//                       (1, 2, 3...) + compact circular Meow modal emblem
	//                       on the focused pane, and dimmed clickable pane bar
	//                       on inactive panes.
	WindowNumber int
	WindowCount  int
	Focused      bool
	Dedicated    bool
	KeyPrefix    string
	OnFocus      func()
}

// Render paints Marlin's Modern Doom Modeline with strict 2-zone LHS/RHS optical
// alignment, uniform vertical centering, and responsive width-budget pruning.
func Render(c *ui.Context, st Status) {
	th := st.Theme
	multi := st.WindowCount > 1
	focused := st.Focused || !multi
	widthCols := st.WidthCols
	if widthCols == 0 {
		if multi && st.WindowCount >= 3 {
			widthCols = 48
		} else if multi {
			widthCols = 72
		} else {
			widthCols = 120
		}
	}
	tier := WidthTierFor(widthCols)
	lhsSegs, rhsSegs := VisibleSegments(st.Preset, widthCols, focused, multi)

	barColor, fullModalLabel, shortModalLabel := modalSegmentSpec(st, th)
	if !focused {
		barColor = kit.WithAlpha(th.BorderSubtle, 0.65)
	}

	k := func(name string) string {
		if st.KeyPrefix == "" {
			return name
		}
		return st.KeyPrefix + "-" + name
	}

	bg := th.Bar
	if !focused {
		bg = kit.WithAlpha(th.Bar, 0.72)
	}
	row := ui.Row(c.Key(k("doom-modeline"))).Height(theme.ModelineHeight)
	if st.Glass && c.Vibrancy() {
		if st.GlassOpacity > 0 {
			op := st.GlassOpacity
			if !focused {
				op *= 0.65
			}
			bg = kit.WithAlpha(th.Bar, op)
		} else {
			bg = ui.Transparent
		}
		row.BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(ui.RGB(255, 255, 255), 0.09))
	} else {
		row.BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.45))
	}

	row.Background(bg).AlignItems(ui.Center).Clip().Children(func() {
		// Segment 1: Authentic 4px doom-modeline-bar (`doom-modeline-bar-width 4`).
		ui.Box(c.Key(k("dm-bar"))).Width(4).FillHeight().Background(barColor)

		// Strict 2-zone optical layout: LHS flex-grows with truncation; RHS stays crisp.
		ui.Row(c).Grow(1).MinWidth(0).Padding(0, 10).Gap(10).AlignItems(ui.Center).Clip().Children(func() {
			// LHS Zone
			ui.Row(c.Key(k("dm-lhs"))).Basis(0).Grow(1).MinWidth(0).Gap(7).AlignItems(ui.Center).Clip().Children(func() {
				for _, seg := range lhsSegs {
					renderLHSSegment(c, k, seg, st, th, barColor, fullModalLabel, shortModalLabel, multi, focused, tier)
				}
			})

			// RHS Zone
			if len(rhsSegs) > 0 {
				ui.Row(c.Key(k("dm-rhs"))).Gap(7).AlignItems(ui.Center).Shrink(0).Children(func() {
					renderRHSSegments(c, k, rhsSegs, st, th, widthCols, focused, multi, tier)
				})
			}
		})
	})
}

func renderLHSSegment(
	c *ui.Context,
	k func(string) string,
	seg SegmentID,
	st Status,
	th theme.Theme,
	barColor ui.Color,
	fullModal, shortModal string,
	multi, focused bool,
	tier WidthTier,
) {
	switch seg {
	case SegBar:
		// Rendered as the full-height 4px left edge accent bar.
		return

	case SegWindowState:
		if !st.Dedicated {
			return
		}
		renderDedicatedPin(c, k, focused, st.OnToggleDedicated, th)

	case SegWindowNumber:
		if !multi || st.WindowNumber <= 0 {
			return
		}
		if focused {
			renderFocusedWindowNumber(c, k, st.WindowNumber, barColor, th)
		} else {
			renderInactiveWindowNumber(c, k, st.WindowNumber, st.OnFocus, th)
		}

	case SegModals:
		if !focused {
			return
		}
		// Wide single window shows iconic circular Meow emblem + full mode name ("N" + "NORMAL");
		// multi-window or compact panes show razor-sharp circular Meow emblem ("N" / "I" / "K").
		showWord := !multi && tier == TierWide
		renderMeowModalBadge(c, k, shortModal, fullModal, showWord, barColor, th)

	case SegBufferInfo:
		renderBufferInfoSegment(c, k, st, focused, tier, th)

	case SegWorkspacePath:
		renderWorkspacePathSegment(c, k, st, focused, multi, tier, th)

	case SegRemoteHost:
		if !focused || strings.TrimSpace(st.RemoteHost) == "" {
			return
		}
		ui.Row(c.Key(k("dm-remote"))).Height(18).Gap(3).AlignItems(ui.Center).Shrink(0).Children(func() {
			theme.SFIcon(c, "network", 10, th.Accent, false)
			ui.Text(c, "@"+strings.TrimSpace(st.RemoteHost)).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Accent).NoWrap()
		})

	case SegPosition:
		if !focused {
			return
		}
		pos := "All"
		if st.RowCount > 0 {
			pos = fmt.Sprintf("%d rows", st.RowCount)
		}
		if st.TokenCount > 0 {
			pos += fmt.Sprintf(" · %s tok", formatCompactCount(st.TokenCount))
		}
		ui.Text(c.Key(k("dm-pos")), pos).Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap().Shrink(0)

	case SegHelmTitle:
		title := "M-x"
		switch st.MiniMode {
		case minibuffer.ModeHelm:
			title = "HELM"
		case minibuffer.ModeBufferSwitch:
			title = "Switch Buffer"
		case minibuffer.ModeThemeSwitch:
			title = "Switch Theme"
		case minibuffer.ModeWorkspaceCreate:
			title = "New Workspace"
		}
		ui.Text(c.Key(k("dm-helm-title")), title).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(th.Accent).NoWrap().Shrink(0)

	case SegHelmCount:
		if st.HelmTotal > 0 {
			ui.Text(c.Key(k("dm-helm-count")), fmt.Sprintf("%d candidates", st.HelmTotal)).
				Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap().Shrink(0)
		}
	}
}

func renderRHSSegments(
	c *ui.Context,
	k func(string) string,
	segs []SegmentID,
	st Status,
	th theme.Theme,
	widthCols uint32,
	focused, multi bool,
	tier WidthTier,
) {
	renderedAny := false
	addDivider := func(idx int) {
		if renderedAny {
			ui.Box(c.Key(k(fmt.Sprintf("dm-div-%d", idx)))).Width(1).Height(11).
				Background(kit.WithAlpha(th.BorderSubtle, 0.42)).Shrink(0)
		}
	}

	for idx, seg := range segs {
		switch seg {
		case SegHelmHelp:
			ui.Text(c.Key(k("dm-helm-help")), "C-n/C-p move · RET open · C-g quit").
				Font(theme.FontMono).FontSize(11).TextColor(th.InkMuted).NoWrap()
			renderedAny = true

		case SegStatusItems:
			if !focused || len(st.StatusItems) == 0 {
				continue
			}
			// Project live Scheme status items using the pane's ACTUAL width budget!
			projected := mapper.ProjectStatusItems(st.StatusItems, widthCols, nil)
			if len(projected) == 0 {
				continue
			}
			addDivider(idx)
			ui.Row(c.Key(k("dm-scheme-items"))).Height(18).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
				for i, item := range projected {
					lbl := strings.TrimSpace(item.Text)
					if lbl == "" {
						continue
					}
					ui.Text(c.Key(k(fmt.Sprintf("dm-si-%d", i))), lbl).
						Font(theme.FontMono).FontSize(11).FontWeight(600).
						TextColor(statusToneColor(item.Tone, th)).NoWrap()
				}
			})
			renderedAny = true

		case SegDeepThink:
			if !focused || !st.ShowDeepThink {
				continue
			}
			addDivider(idx)
			renderDeepThinkSegment(c, k, st.DeepThink, st.OnToggleDeepThink, th)
			renderedAny = true

		case SegWorkspace:
			if !focused || strings.TrimSpace(st.WorkspaceName) == "" {
				continue
			}
			addDivider(idx)
			renderWorkspacePill(c, k, st.WorkspaceName, st.OnOpenWorkspaces, th)
			renderedAny = true

		case SegVCS:
			if !focused || strings.TrimSpace(st.VCSBranch) == "" {
				continue
			}
			addDivider(idx)
			ui.Row(c.Key(k("dm-vcs"))).Height(18).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
				theme.SFIcon(c, "arrow.triangle.branch", 10, th.Success, false)
				ui.Text(c, strings.TrimSpace(st.VCSBranch)).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(th.Success).NoWrap()
			})
			renderedAny = true

		case SegGates:
			if !focused || st.PendingGates <= 0 || !st.ShowGateBadge {
				continue
			}
			addDivider(idx)
			renderGateBadge(c, k, st.PendingGates, st.OnReviewGate, th)
			renderedAny = true

		case SegModel:
			if !focused {
				continue
			}
			addDivider(idx)
			renderModelCapsule(c, k, st, tier, th)
			renderedAny = true

		case SegMajorMode:
			modeName := strings.TrimSpace(st.MajorMode)
			if modeName == "" {
				modeName = "Marlin"
			}
			if renderedAny {
				addDivider(idx)
			}
			col := th.Info
			if !focused {
				col = th.InkMuted
			}
			ui.Text(c.Key(k("dm-major-mode")), modeName).
				Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(col).NoWrap().Shrink(0)
			renderedAny = true

		case SegBrand:
			if !focused || multi || strings.TrimSpace(st.VersionLabel) == "" {
				continue
			}
			addDivider(idx)
			ui.Text(c.Key(k("dm-brand")), strings.TrimSpace(st.VersionLabel)).
				Font(theme.FontMono).FontSize(11).FontWeight(700).LetterSpacing(0.5).TextColor(th.Info).NoWrap().Shrink(0)
			renderedAny = true
		}
	}
}

// renderMeowModalBadge renders Doom Modeline's signature Meow circular modal
// emblem (`nf-md-alpha_n_circle` -> crisp 18dp circular badge with centered
// letter `N`/`I`/`K`/`H`/`X`), optionally paired with the full state label on
// wide single-window frames.
func renderMeowModalBadge(
	c *ui.Context,
	k func(string) string,
	shortLabel, fullLabel string,
	showWord bool,
	tone ui.Color,
	th theme.Theme,
) {
	if !showWord {
		ui.Row(c.Key(k("dm-modal-badge"))).MinWidth(18).Height(18).Padding(0, 5).Radius(theme.RadiusPill).
			Background(kit.WithAlpha(tone, 0.22)).Border(1, kit.WithAlpha(tone, 0.42)).
			AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, shortLabel).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(tone).NoWrap()
		})
		return
	}

	ui.Row(c.Key(k("dm-modal-badge"))).Height(19).Padding(0, 7, 0, 2).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.68)).
		Border(1, kit.WithAlpha(tone, 0.30)).Gap(5).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Row(c).MinWidth(15).Height(15).Padding(0, 3).Radius(theme.RadiusPill).
			Background(tone).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, shortLabel).Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(th.Bar).NoWrap()
		})
		ui.Text(c, fullLabel).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(tone).NoWrap()
	})
}

func renderFocusedWindowNumber(
	c *ui.Context,
	k func(string) string,
	winNum int,
	tone ui.Color,
	th theme.Theme,
) {
	ui.Row(c.Key(k("dm-win-num"))).MinWidth(18).Height(18).Padding(0, 5).Radius(theme.RadiusPill).
		Background(tone).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, fmt.Sprintf("%d", winNum)).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Bar).NoWrap()
	})
}

func renderInactiveWindowNumber(
	c *ui.Context,
	k func(string) string,
	winNum int,
	onFocus func(),
	th theme.Theme,
) {
	numStr := fmt.Sprintf("%d", winNum)
	btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-win-num"))).Label("Window "+numStr).
		MinWidth(18).Height(18).Padding(0, 5).Radius(theme.RadiusPill).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0))
	if btn.Hovered() {
		btn.Background(kit.WithAlpha(th.Accent, 0.24))
	} else {
		btn.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.32, 0.54))
	}
	btn.Children(func() {
		col := th.InkMuted
		if btn.Hovered() {
			col = th.Accent
		}
		ui.Text(c, numStr).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(col).NoWrap()
	})
	if btn.Clicked() && onFocus != nil {
		onFocus()
	}
}

func renderDedicatedPin(c *ui.Context, k func(string) string, focused bool, onToggle func(), th theme.Theme) {
	pinCol := th.Warning
	if !focused {
		pinCol = th.InkMuted
	}
	if onToggle != nil && focused {
		btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-dedicated"))).Label("D").
			Height(18).Padding(0, 5).Radius(theme.RadiusPill).Gap(3).
			Background(kit.WithAlpha(pinCol, 0.16)).AlignItems(ui.Center).Shrink(0))
		btn.Children(func() {
			theme.SFIcon(c, "pin.fill", 9.5, pinCol, true)
			ui.Text(c, "D").Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(pinCol).NoWrap()
		})
		if btn.Clicked() {
			onToggle()
		}
		return
	}
	ui.Row(c.Key(k("dm-dedicated"))).Height(18).Padding(0, 5).Radius(theme.RadiusPill).
		Background(kit.WithAlpha(pinCol, 0.16)).Gap(3).AlignItems(ui.Center).Shrink(0).Children(func() {
		theme.SFIcon(c, "pin.fill", 9.5, pinCol, true)
		ui.Text(c, "D").Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(pinCol).NoWrap()
	})
}

func renderBufferInfoSegment(c *ui.Context, k func(string) string, st Status, focused bool, tier WidthTier, th theme.Theme) {
	bufName := strings.TrimSpace(st.BufferName)
	if bufName == "" {
		bufName = "*conversation*"
	}
	iconSym := BufferIcon(bufName, st.MajorMode)
	iconCol := th.Info
	textCol := th.Info
	if !focused {
		iconCol = th.InkMuted
		textCol = th.InkMuted
	} else if st.HasUnsavedDraft {
		textCol = th.Warning
	}

	paintContent := func() {
		if tier != TierCompact {
			theme.SFIcon(c, iconSym, 11.5, iconCol, false)
		}
		if st.AgentWorking {
			ui.Box(c.Key(k("dm-buf-busy"))).Width(6).Height(6).Radius(theme.RadiusPill).Background(th.Info).Shrink(0)
		} else if st.HasUnsavedDraft {
			ui.Box(c.Key(k("dm-buf-mod"))).Width(6).Height(6).Radius(theme.RadiusPill).Background(th.Warning).Shrink(0)
		} else if st.ReadOnly {
			theme.SFIcon(c, "lock.fill", 9.5, th.InkMuted, true)
		}
		ui.Text(c, bufName).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(textCol).
			SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Clip()
	}

	if !focused && st.OnFocus != nil {
		btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-inactive-focus"))).Label(bufName).
			Basis(0).Grow(1).MinWidth(0).Height(20).Padding(0, 4).Gap(5).Radius(theme.RadiusSmall).
			AlignItems(ui.Center).Justify(ui.Start).Clip())
		btn.Children(paintContent)
		if btn.Clicked() {
			st.OnFocus()
		}
		return
	}

	ui.Row(c.Key(k("dm-buf-info"))).MinWidth(0).Shrink(1).Height(20).Gap(5).AlignItems(ui.Center).Clip().Children(paintContent)
}

func renderWorkspacePathSegment(c *ui.Context, k func(string) string, st Status, focused, multi bool, tier WidthTier, th theme.Theme) {
	if !focused {
		renderBufferInfoSegment(c, k, st, false, tier, th)
		return
	}
	ws := strings.TrimSpace(st.WorkspacePath)
	if ws == "" {
		ws = "~/marlin/" + st.SessionID + "/"
	}
	displayPath := ShrinkPath(ws)
	iconSym := BufferIcon(st.BufferName, st.MajorMode)
	ui.Row(c.Key(k("dm-ws-path"))).MinWidth(0).Shrink(1).Height(20).Gap(5).AlignItems(ui.Center).Clip().Children(func() {
		if tier != TierCompact {
			theme.SFIcon(c, iconSym, 11.5, th.Info, false)
		}
		if multi && strings.TrimSpace(st.BufferName) != "" {
			ui.Text(c, strings.TrimSpace(st.BufferName)).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(th.Info).NoWrap().Shrink(0)
		}
		if !multi || tier == TierWide {
			ui.Text(c, displayPath).Font(theme.FontMono).FontSize(11.5).FontWeight(600).TextColor(th.Info).
				SingleLine().Ellipsis("…").TruncateMiddle().Shrink(1).MinWidth(0).Clip()
		}
	})
}

func renderWorkspacePill(c *ui.Context, k func(string) string, wsName string, onOpen func(), th theme.Theme) {
	wsLbl := "[" + strings.TrimSpace(wsName) + "]"
	if onOpen != nil {
		btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-ws-pill"))).Label(wsLbl).
			Height(19).Padding(0, 7).Gap(4).Radius(theme.RadiusPill).AlignItems(ui.Center).Shrink(0))
		if btn.Hovered() {
			btn.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.50, 0.72))
		} else {
			btn.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.58))
		}
		btn.Children(func() {
			theme.SFIcon(c, "square.stack.3d.up", 9.5, th.Accent, false)
			ui.Text(c, wsLbl).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Accent).NoWrap()
		})
		if btn.Clicked() {
			onOpen()
		}
		return
	}
	ui.Row(c.Key(k("dm-ws-pill"))).Height(19).Padding(0, 7).Gap(4).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.58)).AlignItems(ui.Center).Shrink(0).Children(func() {
		theme.SFIcon(c, "square.stack.3d.up", 9.5, th.Accent, false)
		ui.Text(c, wsLbl).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Accent).NoWrap()
	})
}

func renderDeepThinkSegment(c *ui.Context, k func(string) string, active bool, onToggle func(), th theme.Theme) {
	col := th.InkMuted
	bg := ui.Transparent
	if active {
		col = th.Accent
		bg = kit.WithAlpha(th.Accent, 0.16)
	}
	if onToggle != nil {
		btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-deep-think"))).Label("Think").
			Height(19).Padding(0, 6).Gap(4).Radius(theme.RadiusPill).Background(bg).AlignItems(ui.Center).Shrink(0))
		if btn.Hovered() && !active {
			btn.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.42, 0.65))
		}
		btn.Children(func() {
			theme.SFIcon(c, "sparkles", 10, col, active)
			ui.Text(c, "Think").Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(col).NoWrap()
		})
		if btn.Clicked() {
			onToggle()
		}
		return
	}
	ui.Row(c.Key(k("dm-deep-think"))).Height(19).Padding(0, 6).Gap(4).Radius(theme.RadiusPill).
		Background(bg).AlignItems(ui.Center).Shrink(0).Children(func() {
		theme.SFIcon(c, "sparkles", 10, col, active)
		ui.Text(c, "Think").Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(col).NoWrap()
	})
}

func renderGateBadge(c *ui.Context, k func(string) string, count int, onReview func(), th theme.Theme) {
	lbl := fmt.Sprintf("%d gate", count)
	if count > 1 {
		lbl = fmt.Sprintf("%d gates", count)
	}
	btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-gate-badge"))).Label(lbl).
		Height(19).Padding(0, 7).Gap(4).Radius(theme.RadiusPill).
		Background(kit.WithAlpha(th.Warning, 0.16)).Border(1, kit.WithAlpha(th.Warning, 0.34)).
		AlignItems(ui.Center).Shrink(0))
	btn.Children(func() {
		theme.SFIcon(c, "exclamationmark.shield.fill", 10, th.Warning, true)
		ui.Text(c, lbl).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(th.Warning).NoWrap()
	})
	if btn.Clicked() && onReview != nil {
		onReview()
	}
}

func renderModelCapsule(c *ui.Context, k func(string) string, st Status, tier WidthTier, th theme.Theme) {
	actCol := st.ActivityColor
	if actCol == (ui.Color{}) {
		actCol = th.Success
	}
	modelLabel := strings.TrimSpace(st.ModelLabel)
	displayModel := modelLabel
	if displayModel == "" {
		displayModel = st.ActivityLabel
	}
	if tier == TierCompact && strings.Contains(displayModel, "/") {
		if _, short, ok := strings.Cut(displayModel, "/"); ok && short != "" {
			displayModel = short
		}
	}
	if displayModel == "" {
		return
	}

	if st.OnOpenModels != nil {
		mbtn := kit.ActionControl(ui.Button(c.Key(k("dm-model-pill")), "").Label(displayModel).
			Height(20).Padding(0, 8).Gap(5).Radius(theme.RadiusPill).AlignItems(ui.Center).
			Background(ui.Transparent).Border(0, ui.Transparent).Shrink(0))
		if c.Vibrancy() {
			mbtn.Material(glass.Glass{Style: glass.Clear, Tint: kit.WithAlpha(th.SurfaceElevated, 0.24), Interactive: true})
			mbtn.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.10))
		} else if mbtn.Hovered() {
			mbtn.Background(th.SurfaceHover)
		} else {
			mbtn.Background(kit.WithAlpha(th.SurfaceElevated, 0.42))
		}
		mbtn.Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(actCol)
			col := th.Ink
			if modelLabel == "" {
				col = actCol
			}
			ui.Text(c, displayModel).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(col).
				SingleLine().Ellipsis("…").MinWidth(0).Clip()
		})
		if mbtn.Clicked() {
			st.OnOpenModels()
		}
		return
	}

	ui.Row(c.Key(k("dm-model-pill"))).Height(20).Padding(0, 6).Gap(5).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(actCol)
		col := th.Ink
		if modelLabel == "" {
			col = actCol
		}
		ui.Text(c, displayModel).Font(theme.FontMono).FontSize(11).FontWeight(600).TextColor(col).NoWrap()
	})
}

func formatCompactCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

func statusToneColor(tone schema.UITone, th theme.Theme) ui.Color {
	return th.ToMapperSkin().ToneColor(tone)
}

func modalSegmentSpec(st Status, th theme.Theme) (color ui.Color, fullLabel, shortLabel string) {
	if st.Keypad != "" {
		return th.Info, "KEYPAD", "K"
	}
	if st.MiniMode == minibuffer.ModeHelm {
		return th.Accent, "HELM", "H"
	}
	if st.MiniMode != minibuffer.ModePrompt {
		return th.Accent, "M-x", "X"
	}
	if st.AgentWorking {
		return th.Info, "NORMAL", "N"
	}
	if st.KeyMode == meow.ModeNormal {
		return th.Warning, "NORMAL", "N"
	}
	return th.Success, "INSERT", "I"
}
