// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package modeline implements Marlin UI's declarative Doom Modeline
// (mirroring doom-modeline.el's 'dashboard, 'main, and 'helm presets and
// consuming live marlin.ui.v1 gRPC StatusItems & active ModelLabel).
//
// Right-side segments adapt dynamically to the active Window, Buffer, and
// Workspace — never hardcoding stale fallback model names or cluttering the bar
// with redundant status text.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A07 (emacsModeline).
package modeline

import (
	"fmt"
	"strings"

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
	Preset        Preset
	KeyMode       meow.Mode
	MiniMode      minibuffer.Mode
	Keypad        string
	BufferName    string
	MajorMode     string
	WorkspaceName string
	WorkspacePath string
	RemoteHost    string
	ActivityLabel string
	ActivityColor ui.Color
	ModelLabel    string
	StatusItems   []schema.UIStatusItem // declarative marlin.ui.v1 status items when provided by Rust/Scheme
	SessionID     string
	RowCount      int
	HelmTotal     int
	PendingGates      int
	ShowGateBadge     bool
	OnReviewGate      func()
	OnOpenModels      func()
	OnOpenWorkspaces  func()
	VersionLabel      string
	Glass         bool
	GlassOpacity  float32
	Theme         theme.Theme
}

// Render paints the clean, dynamic Doom Modeline directly above the Echo Area.
func Render(c *ui.Context, st Status) {
	th := st.Theme
	barColor, modalLabel := modalSegmentSpec(st, th)

	bg := th.Bar
	row := ui.Row(c.Key("doom-modeline")).Height(theme.ModelineHeight)
	if st.Glass && c.Vibrancy() {
		a := st.GlassOpacity
		if a <= 0 {
			a = 0.24
		}
		bg = kit.WithAlpha(th.Bar, a)
		row.BorderWidth(1, 0, 0, 0).BorderColor(kit.WithAlpha(ui.RGB(255, 255, 255), 0.08))
	}
	row.Background(bg).AlignItems(ui.Center).Clip().Children(func() {
		// Segment 1: iconic 3px doom-modeline-bar.
		ui.Box(c).Width(3).FillHeight().Background(barColor)

		ui.Row(c).Grow(1).MinWidth(0).Padding(0, 14).Gap(10).AlignItems(ui.Center).Clip().Children(func() {
			// Segment 2: circular modal state indicator + active perspective workspace pill.
			renderModalBadge(c, modalLabel, barColor, th)
			if st.WorkspaceName != "" {
				wsLbl := "[" + st.WorkspaceName + "]"
				if st.OnOpenWorkspaces != nil {
					btn := kit.ActionControl(ui.ButtonBase(c.Key("dm-ws-pill")).Label(wsLbl).
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
					ui.Row(c.Key("dm-ws-pill")).Padding(2, 7).Radius(theme.RadiusPill).
						Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.36, 0.58)).Children(func() {
						ui.Text(c, wsLbl).Font(theme.FontMono).FontSize(11).FontWeight(700).TextColor(th.Accent).NoWrap()
					})
				}
			}

			switch st.Preset {
			case PresetHelm:
				renderHelmSegments(c, st, th)
			case PresetDashboard:
				renderDashboardSegments(c, st, th)
			default:
				renderMainSegments(c, st, th)
			}
		})
	})
}

func renderModalBadge(c *ui.Context, fullLabel string, tone ui.Color, th theme.Theme) {
	ui.Row(c.Key("dm-modal-badge")).Padding(2, 7).Radius(theme.RadiusPill).
		Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.42, 0.64)).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(tone)
		ui.Text(c, fullLabel).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(tone).NoWrap()
	})
}

func renderDashboardSegments(c *ui.Context, st Status, th theme.Theme) {
	// Initial Dashboard / empty Conversation workspace preset:
	// left = workspace path + optional remote host; right = concise online dot + real model + dynamic workspace context.
	ws := st.WorkspacePath
	if ws == "" {
		ws = "~/marlin/" + st.SessionID + "/"
	}
	ui.Text(c, ws).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).MinWidth(0).Ellipsis("...").NoWrap().MaxLines(1)

	if st.RemoteHost != "" {
		ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
	}

	ui.Spacer(c)

	renderRightDeclarative(c, st, th, true)
}

func renderMainSegments(c *ui.Context, st Status, th theme.Theme) {
	// Active buffer preset: left = buffer name + row count; right = concise online dot + real model + active MajorMode.
	bufName := st.BufferName
	if bufName == "" {
		bufName = "*conversation*"
	}
	ui.Text(c, bufName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()

	pos := "All"
	if st.RowCount > 0 {
		pos = fmt.Sprintf("%d rows", st.RowCount)
	}
	ui.Text(c, pos).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()

	if st.RemoteHost != "" {
		ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
	}

	ui.Spacer(c)

	renderRightDeclarative(c, st, th, false)

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

func renderRightDeclarative(c *ui.Context, st Status, th theme.Theme, showDashboardMeta bool) {
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
			if theme.ActionLink(c, "dm-gate-badge", lbl, th.Warning, th).Clicked() && st.OnReviewGate != nil {
				st.OnReviewGate()
			}
		}

		// If marlin.ui.v1 gRPC supplied declarative StatusItems, render them directly.
		if len(st.StatusItems) > 0 {
			for i, item := range st.StatusItems {
				lbl := strings.TrimSpace(item.Label)
				if lbl == "" {
					lbl = strings.TrimSpace(item.CompactLabel)
				}
				if lbl == "" {
					continue
				}
				ui.Text(c.Key(fmt.Sprintf("dm-si-%d", i)), lbl).
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
			mbtn := kit.ActionControl(ui.ButtonBase(c.Key("dm-model-pill")).Label(displayModel).
				Padding(2, 8).Gap(6).Radius(theme.RadiusPill).AlignItems(ui.Center))
			if mbtn.Hovered() {
				mbtn.Background(theme.GlassTone(c, th.SurfaceHover, th, 0.44, 0.66))
			}
			mbtn.Children(func() {
				ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(actCol)
				col := th.Ink
				if modelLabel == "" {
					col = actCol
				}
				ui.Text(c, displayModel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(col).NoWrap()
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

		// Dynamic initial-workspace badge shown ONLY on Dashboard / initial empty Conversation.
		if showDashboardMeta && strings.TrimSpace(st.VersionLabel) != "" {
			ui.Text(c, st.VersionLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Info).NoWrap()
		}
	})
}

func statusToneColor(tone schema.UITone, th theme.Theme) ui.Color {
	return th.ToMapperSkin().ToneColor(tone)
}

func modalSegmentSpec(st Status, th theme.Theme) (color ui.Color, label string) {
	if st.Keypad != "" {
		return th.Info, "KEYPAD"
	}
	if st.MiniMode == minibuffer.ModeHelm {
		return th.Accent, "HELM"
	}
	if st.MiniMode != minibuffer.ModePrompt {
		return th.Accent, "M-x"
	}
	if st.KeyMode == meow.ModeNormal {
		return th.Warning, "NORMAL"
	}
	return th.Success, "INSERT"
}
