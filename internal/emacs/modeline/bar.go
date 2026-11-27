// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package modeline implements Marlin UI's declarative Doom Modeline
// (mirroring doom-modeline.el's 'dashboard, 'main, and 'helm presets,
// per-window active/inactive rendering, Doom `modals` + `window-number`
// (winum) badges, and Emacs 30 `window-state` dedicated-pin indicators).
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

	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// Preset selects the active doom-modeline layout (mirroring doom-modeline.el).
type Preset uint8

const (
	PresetDashboard Preset = iota // doom-modeline-def-modeline 'dashboard
	PresetMain                    // doom-modeline-def-modeline 'main
	PresetHelm                    // doom-modeline-def-modeline 'helm
)

// Status describes all live Doom Modeline segments projected from hostlink + gRPC.
type Status struct {
	Preset           Preset
	KeyMode          meow.Mode
	MiniMode         minibuffer.Mode
	Keypad           string
	BufferName       string
	MajorMode        string
	WorkspaceName    string
	WorkspacePath    string
	RemoteHost       string
	ActivityLabel    string
	ActivityColor    ui.Color
	ModelLabel       string
	StatusItems      []schema.UIStatusItem // declarative marlin.ui.v1 status items when provided by Rust/Scheme
	SessionID        string
	RowCount         int
	HelmTotal        int
	PendingGates     int
	ShowGateBadge    bool
	OnReviewGate     func()
	OnOpenModels     func()
	OnOpenWorkspaces func()
	VersionLabel     string
	Glass            bool
	GlassOpacity     float32
	Theme            theme.Theme

	// Multi-window Doom Modeline fields (mirroring doom-modeline-def-segment
	// window-number, window-state, and doom-modeline--active):
	//   - WindowCount <= 1: single window -> hides window number (never "W1").
	//   - WindowCount > 1:  per-window modeline -> renders modal state (N/I)
	//                       immediately followed by the circular window number
	//                       1, 2, 3... on the focused pane, and dimmed number
	//                       + buffer/major-mode on inactive panes.
	WindowNumber int
	WindowCount  int
	Focused      bool
	Dedicated    bool
	KeyPrefix    string
	OnFocus      func()
}

// Render paints a clean, dynamic Doom Modeline either at the bottom of the
// single frame or at the bottom of each split window pane.
func Render(c *ui.Context, st Status) {
	th := st.Theme
	multi := st.WindowCount > 1
	focused := st.Focused || !multi
	barColor, fullModalLabel, shortModalLabel := modalSegmentSpec(st, th)
	if !focused {
		barColor = kit.WithAlpha(th.BorderSubtle, 0.72)
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
		row.BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(ui.RGB(255, 255, 255), 0.08))
	} else {
		row.BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(th.BorderSubtle, 0.45))
	}
	row.Background(bg).AlignItems(ui.Center).Clip().Children(func() {
		// Segment 1: iconic 3px doom-modeline-bar (bright when focused, muted when inactive).
		ui.Box(c).Width(3).FillHeight().Background(barColor)

		ui.Row(c).Grow(1).MinWidth(0).Padding(0, 12).Gap(8).AlignItems(ui.Center).Clip().Children(func() {
			// Segment 2: Doom `modals` + `window-number` cluster.
			// Single window -> [● NORMAL] / [● INSERT] (zero window number clutter).
			// Multi-window focused -> [● N | 1] / [● I | 1] (compact modal + circular number).
			// Multi-window inactive -> muted clickable circular [2] / [3] badge.
			if focused {
				renderFocusedModalAndNumber(c, k, multi, shortModalLabel, fullModalLabel, st.WindowNumber, barColor, th)
			} else if multi && st.WindowNumber > 0 {
				renderInactiveWindowNumber(c, k, st.WindowNumber, st.OnFocus, th)
			}

			// Perspective workspace pill (shown on focused window only, matching doom-modeline).
			if focused && st.WorkspaceName != "" {
				wsLbl := "[" + st.WorkspaceName + "]"
				if st.OnOpenWorkspaces != nil {
					btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-ws-pill"))).Label(wsLbl).
						Padding(2, 7).Radius(theme.RadiusPill).AlignItems(ui.Center))
					if btn.Hovered() {
						btn.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.48, 0.70))
					} else {
						btn.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.58))
					}
					btn.Children(func() {
						ui.Text(c, wsLbl).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Accent).NoWrap()
					})
					if btn.Clicked() {
						st.OnOpenWorkspaces()
					}
				} else {
					ui.Row(c.Key(k("dm-ws-pill"))).Padding(2, 7).Radius(theme.RadiusPill).
						Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.58)).Children(func() {
						ui.Text(c, wsLbl).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Accent).NoWrap()
					})
				}
			}

			// Emacs 30 `window-state` dedicated pin badge (`toggle-window-dedicated`).
			if st.Dedicated {
				pinCol := th.Warning
				if !focused {
					pinCol = th.InkMuted
				}
				ui.Row(c.Key(k("dm-dedicated"))).Padding(1, 5).Radius(theme.RadiusPill).
					Background(kit.WithAlpha(pinCol, 0.16)).Gap(3).AlignItems(ui.Center).Shrink(0).Children(func() {
					theme.SFIcon(c, "pin.fill", 9.5, pinCol, true)
					ui.Text(c, "D").Font(theme.FontMono).FontSize(9.5).FontWeight(700).TextColor(pinCol).NoWrap()
				})
			}

			if !focused {
				renderInactiveSegments(c, k, st, th)
				return
			}

			switch st.Preset {
			case PresetHelm:
				renderHelmSegments(c, st, th)
			case PresetDashboard:
				renderDashboardSegments(c, k, st, th)
			default:
				renderMainSegments(c, k, st, th)
			}
		})
	})
}

func renderFocusedModalAndNumber(
	c *ui.Context,
	k func(string) string,
	multi bool,
	shortLabel, fullLabel string,
	winNum int,
	tone ui.Color,
	th theme.Theme,
) {
	if !multi || winNum <= 0 {
		ui.Row(c.Key(k("dm-modal-badge"))).Padding(2, 7).Radius(theme.RadiusPill).
			Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.64)).Gap(5).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(tone)
			ui.Text(c, fullLabel).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(tone).NoWrap()
		})
		return
	}

	// Authentic Doom Emacs multi-window cluster: compact modal letter (N / I / K / H)
	// immediately followed by the circular winum badge (1, 2, 3...) inside one crisp pill.
	ui.Row(c.Key(k("dm-modal-badge"))).Padding(2, 4, 2, 7).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.44, 0.68)).
		Border(1, kit.WithAlpha(tone, 0.28)).Gap(5).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(tone)
		ui.Text(c, shortLabel).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(tone).NoWrap()
		ui.Row(c.Key(k("dm-win-num"))).MinWidth(17).Height(17).Padding(0, 4).Radius(theme.RadiusPill).
			Background(kit.WithAlpha(tone, 0.22)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%d", winNum)).Font(theme.FontMono).FontSize(10.5).FontWeight(700).TextColor(tone).NoWrap()
		})
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
		MinWidth(19).Height(19).Padding(0, 5).Radius(theme.RadiusPill).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0))
	if btn.Hovered() {
		btn.Background(kit.WithAlpha(th.Accent, 0.22))
	} else {
		btn.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.30, 0.52))
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

func renderInactiveSegments(c *ui.Context, k func(string) string, st Status, th theme.Theme) {
	bufName := st.BufferName
	if bufName == "" {
		bufName = "*conversation*"
	}
	if st.OnFocus != nil {
		btn := kit.ActionControl(ui.ButtonBase(c.Key(k("dm-inactive-focus"))).Label(bufName).
			Basis(0).Grow(1).MinWidth(0).Padding(2, 4).Radius(theme.RadiusSmall).
			AlignItems(ui.Center).Justify(ui.Start).Clip())
		btn.Children(func() {
			ui.Text(c, bufName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(600).TextColor(th.InkMuted).
				SingleLine().Ellipsis("…").MinWidth(0).Clip()
		})
		if btn.Clicked() {
			st.OnFocus()
		}
	} else {
		ui.Text(c, bufName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(600).TextColor(th.InkMuted).
			Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
	}
	if modeName := strings.TrimSpace(st.MajorMode); modeName != "" {
		ui.Text(c, modeName).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(th.InkMuted).NoWrap().Shrink(0)
	}
}

func renderDashboardSegments(c *ui.Context, k func(string) string, st Status, th theme.Theme) {
	// Initial Dashboard / empty Conversation workspace preset:
	// left = workspace path + optional remote host; right = concise online dot + real model + dynamic workspace context.
	ws := st.WorkspacePath
	if ws == "" {
		ws = "~/marlin/" + st.SessionID + "/"
	}
	if st.WindowCount > 1 && st.BufferName != "" {
		ui.Text(c, st.BufferName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()
	}
	ui.Text(c, ws).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).
		SingleLine().Ellipsis("…").TruncateMiddle().Shrink(1).MinWidth(0).Clip()

	if st.RemoteHost != "" && st.WindowCount <= 1 {
		ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
	}

	ui.Spacer(c)

	renderRightDeclarative(c, k, st, th, true)
}

func renderMainSegments(c *ui.Context, k func(string) string, st Status, th theme.Theme) {
	// Active buffer preset: left = buffer name + row count; right = concise online dot + real model + active MajorMode.
	bufName := st.BufferName
	if bufName == "" {
		bufName = "*conversation*"
	}
	ui.Text(c, bufName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()

	if st.WindowCount <= 1 {
		pos := "All"
		if st.RowCount > 0 {
			pos = fmt.Sprintf("%d rows", st.RowCount)
		}
		ui.Text(c, pos).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()

		if st.RemoteHost != "" {
			ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
		}
	}

	ui.Spacer(c)

	renderRightDeclarative(c, k, st, th, false)

	modeName := st.MajorMode
	if modeName == "" {
		modeName = "Marlin"
	}
	ui.Text(c, modeName).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Info).NoWrap()
}

func renderHelmSegments(c *ui.Context, st Status, th theme.Theme) {
	title := "M-x"
	if st.MiniMode == minibuffer.ModeHelm {
		title = "HELM"
	} else if st.MiniMode == minibuffer.ModeBufferSwitch {
		title = "Switch Buffer"
	} else if st.MiniMode == minibuffer.ModeThemeSwitch {
		title = "Switch Theme"
	} else if st.MiniMode == minibuffer.ModeWorkspaceCreate {
		title = "New Workspace"
	}
	ui.Text(c, title).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Accent).NoWrap()
	if st.HelmTotal > 0 {
		ui.Text(c, fmt.Sprintf("%d candidates", st.HelmTotal)).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
	}

	ui.Spacer(c)

	ui.Text(c, "C-n/C-p move · RET open · C-g quit").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
}

func renderRightDeclarative(c *ui.Context, k func(string) string, st Status, th theme.Theme, showDashboardMeta bool) {
	actCol := st.ActivityColor
	if actCol == (ui.Color{}) {
		actCol = th.Success
	}

	ui.Row(c).Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		if st.PendingGates > 0 && st.ShowGateBadge {
			lbl := fmt.Sprintf("%d gate", st.PendingGates)
			if st.PendingGates > 1 {
				lbl = fmt.Sprintf("%d gates", st.PendingGates)
			}
			if theme.ActionLink(c, k("dm-gate-badge"), lbl, th.Warning, th).Clicked() && st.OnReviewGate != nil {
				st.OnReviewGate()
			}
		}

		// If marlin.ui.v1 gRPC supplied declarative StatusItems, render them directly.
		if len(st.StatusItems) > 0 && st.WindowCount <= 1 {
			for i, item := range st.StatusItems {
				lbl := strings.TrimSpace(item.Label)
				if lbl == "" {
					lbl = strings.TrimSpace(item.CompactLabel)
				}
				if lbl == "" {
					continue
				}
				ui.Text(c.Key(k(fmt.Sprintf("dm-si-%d", i))), lbl).
					Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(statusToneColor(item.Tone, th)).NoWrap()
			}
		}

		// Concise Online dot + clickable active gRPC model label (1-click opens *Models* popup!).
		modelLabel := strings.TrimSpace(st.ModelLabel)
		displayModel := modelLabel
		if displayModel == "" {
			displayModel = st.ActivityLabel
		}
		if st.OnOpenModels != nil && displayModel != "" {
			mbtn := kit.ActionControl(ui.Button(c.Key(k("dm-model-pill")), "").Label(displayModel).
				Padding(3, 9).Gap(6).Radius(theme.RadiusPill).AlignItems(ui.Center).
				Background(ui.Transparent).Border(0, ui.Transparent))
			if c.Vibrancy() {
				mbtn.Material(glass.Glass{Style: glass.Clear, Tint: kit.WithAlpha(th.SurfaceElevated, 0.24), Interactive: true})
				mbtn.Border(1, kit.WithAlpha(ui.RGB(255, 255, 255), 0.10))
			} else if mbtn.Hovered() {
				mbtn.Background(th.SurfaceHover)
			}
			mbtn.Children(func() {
				ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(actCol)
				col := th.Ink
				if modelLabel == "" {
					col = actCol
				}
				ui.Text(c, displayModel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(col).
					SingleLine().Ellipsis("…").MinWidth(0).Clip()
			})
			if mbtn.Clicked() {
				st.OnOpenModels()
			}
		} else {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(actCol)
				if modelLabel != "" {
					ui.Text(c, modelLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(th.Ink).NoWrap()
				} else if st.ActivityLabel != "" {
					ui.Text(c, st.ActivityLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(actCol).NoWrap()
				}
			})
		}

		// Dynamic initial-workspace badge shown ONLY on single-window Dashboard.
		if showDashboardMeta && st.WindowCount <= 1 && strings.TrimSpace(st.VersionLabel) != "" {
			ui.Text(c, st.VersionLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Info).NoWrap()
		}
	})
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
	if st.KeyMode == meow.ModeNormal {
		return th.Warning, "NORMAL", "N"
	}
	return th.Success, "INSERT", "I"
}
