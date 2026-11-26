// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package modeline implements the complete authentic Doom Modeline specification
// (matching doom-modeline.el's 'dashboard, 'main, and 'helm modeline presets):
//   - Left:  3px doom-modeline-bar + circular Meow modal state badge + workspace/buffer
//     info (Doom blue bold) + buffer position/rows + remote host indicator
//   - Right: active LLM route (LSP-style dot) + UTF-8 encoding + Doom blue bold
//     major-mode / version + VCS-style host transport status
//   - Seamless matte bar surface (#21242b) with zero harsh 1px outer borders.
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A07 (emacsModeline).
package modeline

import (
	"fmt"

	"github.com/egoist/mygo/ui"

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
	WorkspacePath string
	RemoteHost    string
	ActivityLabel string
	ActivityColor ui.Color
	ModelLabel    string
	SessionID     string
	RowCount      int
	HelmTotal     int
	PendingGates  int
	OnReviewGate  func()
	VersionLabel  string
	Theme         theme.Theme
}

// Render paints the authentic Doom Modeline bar directly above the Emacs Echo Area.
func Render(c *ui.Context, st Status) {
	th := st.Theme
	barColor, modalLetter, modalLabel := modalSegmentSpec(st, th)

	ui.Row(c.Key("doom-modeline")).Height(theme.ModelineHeight).Background(th.Bar).
		AlignItems(ui.Center).Children(func() {
		// Segment 1: iconic 3px doom-modeline-bar.
		ui.Box(c).Width(3).FillHeight().Background(barColor)

		ui.Row(c).Grow(1).Padding(0, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			// Segment 2: circular modal state indicator (mirrors doom-modeline-modals).
			renderModalBadge(c, modalLetter, modalLabel, barColor, th)

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

func renderModalBadge(c *ui.Context, letter, fullLabel string, tone ui.Color, th theme.Theme) {
	ui.Row(c.Key("dm-modal-badge")).Padding(2, 7).Radius(theme.RadiusPill).
		Background(th.SurfaceElevated).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(7).Height(7).Radius(theme.RadiusPill).Background(tone)
		ui.Text(c, fullLabel).Font(theme.FontMono).FontSize(11.5).FontWeight(700).TextColor(tone).NoWrap()
		_ = letter
	})
}

func renderDashboardSegments(c *ui.Context, st Status, th theme.Theme) {
	// Matches (doom-modeline-def-modeline 'dashboard '(bar modals buffer-default-directory-simple remote-host) '(major-mode process))
	ws := st.WorkspacePath
	if ws == "" {
		ws = "~/marlin/" + st.SessionID + "/"
	}
	ui.Text(c, ws).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()

	if st.RemoteHost != "" {
		ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
	}

	ui.Spacer(c)

	renderRightModelAndHost(c, st, th)

	ver := st.VersionLabel
	if ver == "" {
		ver = "MARLIN"
	}
	ui.Text(c, ver).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()
}

func renderMainSegments(c *ui.Context, st Status, th theme.Theme) {
	// Matches (doom-modeline-def-modeline 'main '(bar modals buffer-info remote-host buffer-position) '(lsp buffer-encoding major-mode vcs))
	bufName := st.BufferName
	if bufName == "" {
		bufName = "*conversation*"
	}
	ui.Text(c, bufName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()

	pos := "All"
	if st.RowCount > 0 {
		pos = fmt.Sprintf("%d rows · Bot", st.RowCount)
	}
	ui.Text(c, pos).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()

	if st.RemoteHost != "" {
		ui.Text(c, "@"+st.RemoteHost).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Accent).NoWrap()
	}

	ui.Spacer(c)

	renderRightModelAndHost(c, st, th)

	ui.Text(c, "UTF-8").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()

	modeName := st.MajorMode
	if modeName == "" {
		modeName = "Marlin"
	}
	ui.Text(c, modeName).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()
}

func renderHelmSegments(c *ui.Context, st Status, th theme.Theme) {
	// Matches (doom-modeline-def-modeline 'helm '(bar helm-buffer-id helm-number) '(helm-help))
	title := "M-x"
	if st.MiniMode == minibuffer.ModeHelm {
		title = "HELM"
	} else if st.MiniMode == minibuffer.ModeBufferSwitch {
		title = "Switch Buffer"
	} else if st.MiniMode == minibuffer.ModeThemeSwitch {
		title = "Switch Theme"
	}
	ui.Text(c, title).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Accent).NoWrap()
	if st.HelmTotal > 0 {
		ui.Text(c, fmt.Sprintf("%d candidates", st.HelmTotal)).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
	}

	ui.Spacer(c)

	renderRightModelAndHost(c, st, th)
	ui.Text(c, "C-n/C-p move · RET open · C-g quit").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
}

func renderRightModelAndHost(c *ui.Context, st Status, th theme.Theme) {
	modelLabel := st.ModelLabel
	if modelLabel == "" {
		modelLabel = "gpt-marlin-local"
	}
	actLabel := st.ActivityLabel
	if actLabel == "" {
		actLabel = "Ready"
	}
	actCol := st.ActivityColor
	if actCol == (ui.Color{}) {
		actCol = th.Success
	}

	// Subtle clickable gate badge only when a Tier-0 gate is pending (opens Emacs y-or-n-p in Minibuffer).
	if st.PendingGates > 0 {
		lbl := fmt.Sprintf("%d gate", st.PendingGates)
		if st.PendingGates > 1 {
			lbl = fmt.Sprintf("%d gates", st.PendingGates)
		}
		if theme.ActionLink(c, "dm-gate-badge", lbl, th.Warning, th).Clicked() && st.OnReviewGate != nil {
			st.OnReviewGate()
		}
	}

	// Active LLM route segment (analogous to doom-modeline-lsp).
	ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(6).Height(6).Radius(theme.RadiusPill).Background(actCol)
		ui.Text(c, modelLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Ink).NoWrap()
	})
	ui.Text(c, actLabel).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(actCol).NoWrap()
	ui.Text(c, st.Theme.Name).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
}

func modalSegmentSpec(st Status, th theme.Theme) (color ui.Color, letter, label string) {
	if st.Keypad != "" {
		return th.Info, "K", "KEYPAD"
	}
	if st.MiniMode == minibuffer.ModeHelm {
		return th.Accent, "H", "HELM"
	}
	if st.MiniMode != minibuffer.ModePrompt {
		return th.Accent, "M", "M-x"
	}
	if st.KeyMode == meow.ModeNormal {
		return th.Warning, "N", "NORMAL"
	}
	return th.Success, "I", "INSERT"
}
