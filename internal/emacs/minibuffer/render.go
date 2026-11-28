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

	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// RenderProps wires the Emacs Minibuffer & Hydra popup area to its Frame providers.
type RenderProps struct {
	State       *State
	Theme        theme.Theme
	Glass        bool
	GlassOpacity float32
	KeyMode      meow.Mode
	Keypad      string
	ActiveHydra   *hydra.Body
	Helm          *mygoshell.HelmMinibufferController
	RustConnected bool
	GateTitle     string
	GatePrompt  string

	CommandCandidates       func(query string) []Candidate
	BufferCandidates        func(query string) []Candidate
	BufferOtherWinCands     func(query string) []Candidate
	ThemeCandidates         func(query string) []Candidate
	DescribeFuncCands       func(query string) []Candidate
	RegisterJumpCands       func(query string) []Candidate
	OnSubmitPrompt          func(prompt string)
	OnCreateWorkspace       func(spec string)
	OnDescribeKey           func(chord string)
	OnSaveRegister          func(name string)
	MagitPromptLabel        string
	OnSubmitMagitPrompt func(text string)
	OnHelmPreview       func(title, path, snippet string, line int)
	OnHelmCite        func(ref string)
	OnEnterInsert     func()
	OnExitHydra       func()
	OnResolveGate     func(approve bool)
	OnEcho            func(msg string)
	OnInvalidate      func()
}

// Render paints the bottom Emacs Minibuffer (inline Vertico/Helm list,
// multi-column Doom Hydra / Which-Key popup on SPC, and clean Echo Area).
func Render(c *ui.Context, p RenderProps) {
	th := p.Theme
	s := p.State
	mode, val, sel, focusNow, echo := s.snapshot()

	rowBg := th.Canvas
	if p.Glass && c.Vibrancy() {
		if p.GlassOpacity > 0 {
			rowBg = kit.WithAlpha(th.Canvas, p.GlassOpacity)
		} else {
			rowBg = ui.Transparent
		}
	}

	ui.Column(c.Key("doom-minibuffer")).Background(ui.Transparent).AlignItems(ui.Stretch).Children(func() {
		// 1. Expandable multi-column Doom Hydra / Which-Key drawer when SPC leader is active.
		if p.Keypad != "" && p.ActiveHydra != nil {
			hydra.RenderPopup(c, *p.ActiveHydra, th, p.OnExitHydra)
		}

		// 2. Expandable Vertico / Helm candidate list.
		var items []Candidate
		switch mode {
		case ModeHelm:
			renderHelm(c, s, th, val, p.Helm, p.RustConnected, p.OnEcho, p.OnHelmPreview, p.OnHelmCite)
		case ModeCommand:
			if p.CommandCandidates != nil {
				items = p.CommandCandidates(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		case ModeBufferSwitch:
			if p.BufferCandidates != nil {
				items = p.BufferCandidates(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		case ModeBufferOtherWindow:
			if p.BufferOtherWinCands != nil {
				items = p.BufferOtherWinCands(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		case ModeThemeSwitch:
			if p.ThemeCandidates != nil {
				items = p.ThemeCandidates(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		case ModeDescribeFunc:
			if p.DescribeFuncCands != nil {
				items = p.DescribeFuncCands(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		case ModeRegisterJump:
			if p.RegisterJumpCands != nil {
				items = p.RegisterJumpCands(val)
			}
			renderVertico(c, s, th, val, items, p.Glass)
		}

		// 3. Echo Area when SPC Keypad/Hydra is active.
		if p.Keypad != "" {
			ui.Row(c.Key("keypad-echo")).Height(theme.MinibufferHeight).Padding(0, 14).Gap(10).
				AlignItems(ui.Center).Background(rowBg).Clip().Children(func() {
				ui.Text(c, p.Keypad+"-").Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Accent).Shrink(0).NoWrap()
				ui.Text(c, meow.KeypadHints(p.Keypad)).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
			})
			return
		}

		// 3b. Authentic Emacs y-or-n-p approval prompt inside Minibuffer when requested.
		if mode == ModeGateReview {
			ui.Row(c.Key("mini-gate-yorn")).Height(theme.MinibufferHeight).Padding(0, 14).Gap(10).
				AlignItems(ui.Center).Background(rowBg).Clip().Children(func() {
				ui.Text(c, "Approve (y or n):").Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Warning).Shrink(0).NoWrap()
				msg := p.GatePrompt
				if p.GateTitle != "" && msg != "" {
					msg = p.GateTitle + " — " + msg
				} else if msg == "" {
					msg = "No pending approvals"
				}
				ui.Text(c, msg).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				if p.GatePrompt != "" {
					if theme.ActionButton(c, "mini-gate-yes", "Yes (y)", th.Success, true, th).Clicked() {
						if p.OnResolveGate != nil {
							p.OnResolveGate(true)
						}
						s.Close()
					}
					if theme.ActionButton(c, "mini-gate-no", "No (n)", th.Danger, false, th).Clicked() {
						if p.OnResolveGate != nil {
							p.OnResolveGate(false)
						}
						s.Close()
					}
				}
			})
			return
		}

		// 4. Clean single-line Emacs Minibuffer / Echo Area.
		prompt := "› "
		placeholder := ""
		switch mode {
		case ModeCommand:
			prompt = fmt.Sprintf("%d/%d M-x: ", clampSel(sel, len(items)), len(items))
		case ModeBufferSwitch:
			prompt = fmt.Sprintf("%d/%d Switch to: ", clampSel(sel, len(items)), len(items))
		case ModeBufferOtherWindow:
			prompt = fmt.Sprintf("%d/%d Other window (C-x 4 b): ", clampSel(sel, len(items)), len(items))
		case ModeThemeSwitch:
			prompt = fmt.Sprintf("%d/%d Theme: ", clampSel(sel, len(items)), len(items))
		case ModeRegisterSave:
			prompt = "Save window register (C-x r w): "
			placeholder = "Register name e.g. a, audit, magit (Enter to save)"
		case ModeRegisterJump:
			prompt = fmt.Sprintf("%d/%d Jump to register (C-x r j): ", clampSel(sel, len(items)), len(items))
		case ModeHelm:
			prompt = "Helm ›"
			placeholder = "Search workspace, @lang:rust, @sym:name, #local, !exclude, (dsl ...)"
		case ModeWorkspaceCreate:
			prompt = "New workspace: "
			placeholder = "name or name @/path/to/project (Enter to create)"
		case ModeMagitPrompt:
			prompt = p.MagitPromptLabel
			if strings.TrimSpace(prompt) == "" {
				prompt = "Magit: "
			}
			placeholder = "Enter message or branch name (Enter to execute, Esc to cancel)"
		case ModeDescribeKey:
			prompt = "Describe key (C-h k): "
			placeholder = "Type chord e.g. SPC g g, C-x 3, C-h m, s, or Enter"
		case ModeDescribeFunc:
			prompt = fmt.Sprintf("%d/%d Describe function (C-h f): ", clampSel(sel, len(items)), len(items))
			placeholder = "Orderless filter command name (Enter to inspect in *Help*)"
		default:
			if p.KeyMode == meow.ModeInsert {
				placeholder = "Type message and press Enter..."
			}
		}

		ui.Row(c.Key("mini-editor-row")).Height(theme.MinibufferHeight).Padding(0, 14).Gap(6).
			AlignItems(ui.Center).Background(rowBg).Children(func() {
			if mode == ModePrompt && p.KeyMode == meow.ModeNormal {
				// Quiet authentic Emacs Echo Area; clicking enters INSERT mode immediately.
				echoLabel := echo
				echoCol := th.InkMuted
				if echoLabel == "" {
					echoLabel = "SPC leader · i message"
					echoCol = th.BannerInk
				}
				bar := ui.ButtonBase(c.Key("echo-enter-insert")).Label(echoLabel).KeepFocus().FocusRing(false).
					Grow(1).Height(theme.MinibufferHeight - 4).Padding(0, 4).Radius(theme.RadiusSmall).
					AlignItems(ui.Center).Justify(ui.Start)
				if bar.Hovered() {
					bar.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.34, 0.52))
				}
				bar.Children(func() {
					ui.Text(c, echoLabel).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(echoCol).
						Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				})
				if bar.Clicked() && p.OnEnterInsert != nil {
					p.OnEnterInsert()
				}
				return
			}

			promptEl := ui.Text(c, prompt).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(th.Info).NoWrap()
			switch mode {
			case ModeHelm:
				promptEl.Width(64)
			case ModeCommand, ModeBufferSwitch, ModeBufferOtherWindow, ModeThemeSwitch, ModeDescribeFunc, ModeRegisterJump:
				promptEl.MinWidth(96)
			}

			inp := ui.TextInputBase(c.Key("doom-mini-input"), &val).
				Placeholder(placeholder).Font(theme.FontMono).FontSize(theme.FontSizeBody).TextColor(th.Ink).Grow(1)
			if focusNow {
				inp.Focus()
			}
			if inp.Changed() {
				s.SetInput(val)
				if mode == ModeHelm && p.Helm != nil {
					gen := p.Helm.SetQuery(val)
					s.MarkHelmPending(gen)
				}
			}
			if inp.Submitted() {
				submit(s, mode, val, items, p)
			}

			if echo != "" {
				ui.Text(c, echo).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			}
		})
	})
}

func submit(s *State, mode Mode, raw string, items []Candidate, p RenderProps) {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		s.RecordHistory(mode, trimmed)
	}
	switch mode {
	case ModePrompt:
		if trimmed == "" {
			return
		}
		s.SetInput("")
		if p.OnSubmitPrompt != nil {
			p.OnSubmitPrompt(trimmed)
		}
	case ModeHelm:
		if p.Helm != nil && p.Helm.Engine != nil {
			if cand, ok := p.Helm.Engine.Selected(); ok {
				if p.OnHelmPreview != nil && (strings.TrimSpace(cand.Location) != "" || strings.TrimSpace(cand.Snippet) != "") {
					p.OnHelmPreview(cand.Title, cand.Location, cand.Snippet, 0)
				}
				if p.Helm.OnExecuteAction != nil {
					p.Helm.OnExecuteAction(search.ActionOpenInFocused, []search.Candidate{cand})
				}
				if p.OnEcho != nil {
					p.OnEcho("Opened " + cand.Title)
				}
			}
		}
		s.Close()
	case ModeWorkspaceCreate:
		s.Close()
		if p.OnCreateWorkspace != nil {
			p.OnCreateWorkspace(trimmed)
		}
	case ModeMagitPrompt:
		s.Close()
		if trimmed != "" && p.OnSubmitMagitPrompt != nil {
			p.OnSubmitMagitPrompt(trimmed)
		}
	case ModeDescribeKey:
		s.Close()
		if trimmed != "" && p.OnDescribeKey != nil {
			p.OnDescribeKey(trimmed)
		}
	case ModeRegisterSave:
		s.Close()
		if trimmed != "" && p.OnSaveRegister != nil {
			p.OnSaveRegister(trimmed)
		}
	default:
		s.mu.Lock()
		sel := s.selected
		s.mu.Unlock()
		s.Close()
		if len(items) > 0 {
			if sel < 0 || sel >= len(items) {
				sel = 0
			}
			if items[sel].Run != nil {
				items[sel].Run()
			}
		}
	}
	if p.OnInvalidate != nil {
		p.OnInvalidate()
	}
}
