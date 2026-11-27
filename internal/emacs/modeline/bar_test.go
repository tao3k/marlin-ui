// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package modeline_test

import (
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modeline"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestShrinkPathMatchesDoomEmacsStyle(t *testing.T) {
	cases := [][2]string{
		{"~/ghq/github.com/tao3k/marlin-agent-core/", "~/g/g/t/marlin-agent-core/"},
		{"/Users/guangtao/.config/marlin/ui", "/U/g/.c/m/ui"},
		{"*Dashboard*", "*Dashboard*"},
		{"~/marlin/sess-1/", "~/m/sess-1/"},
	}
	for _, tc := range cases {
		if got := modeline.ShrinkPath(tc[0]); got != tc[1] {
			t.Fatalf("ShrinkPath(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}

func TestVisibleSegmentsResponsiveTiersAndFocus(t *testing.T) {
	// Single window wide -> hides window-number, keeps position & vcs.
	lhsW, rhsW := modeline.VisibleSegments(modeline.PresetMain, 120, true, false)
	if containsSeg(lhsW, modeline.SegWindowNumber) {
		t.Fatalf("single window must hide window-number, got %v", lhsW)
	}
	if !containsSeg(lhsW, modeline.SegPosition) || !containsSeg(rhsW, modeline.SegVCS) {
		t.Fatalf("wide single window should include position and vcs, got lhs=%v rhs=%v", lhsW, rhsW)
	}

	// Compact multi-window focused -> shows window-number & essential segments, prunes wide-only items.
	lhsC, rhsC := modeline.VisibleSegments(modeline.PresetMain, 48, true, true)
	if !containsSeg(lhsC, modeline.SegWindowNumber) || containsSeg(lhsC, modeline.SegPosition) {
		t.Fatalf("compact multi-window LHS mismatch: %v", lhsC)
	}
	if containsSeg(rhsC, modeline.SegVCS) || !containsSeg(rhsC, modeline.SegModel) || !containsSeg(rhsC, modeline.SegMajorMode) {
		t.Fatalf("compact multi-window RHS mismatch: %v", rhsC)
	}

	// Inactive multi-window pane -> strictly dimmed minimal set (window-number + buffer-info + major-mode).
	lhsI, rhsI := modeline.VisibleSegments(modeline.PresetMain, 72, false, true)
	if !containsSeg(lhsI, modeline.SegWindowNumber) || containsSeg(lhsI, modeline.SegModals) || len(rhsI) != 1 || rhsI[0] != modeline.SegMajorMode {
		t.Fatalf("inactive pane segments mismatch: lhs=%v rhs=%v", lhsI, rhsI)
	}
}

func TestModernDoomModelineOpticalAlignmentZeroOverlapAndClicks(t *testing.T) {
	th := theme.DoomOne()
	var clickedModel, clickedWS, clickedGate, clickedThink, clickedDed bool

	st := modeline.Status{
		Preset:            modeline.PresetMain,
		KeyMode:           meow.ModeNormal,
		MiniMode:          minibuffer.ModePrompt,
		BufferName:        "*conversation*",
		MajorMode:         "Marlin-Chat",
		WorkspaceName:     "main",
		VCSBranch:         "feat/ui",
		RemoteHost:        "127.0.0.1:63953",
		ModelLabel:        "deepseek/deepseek-flash",
		RowCount:          12,
		TokenCount:        4200,
		PendingGates:      1,
		ShowGateBadge:     true,
		ShowDeepThink:     true,
		DeepThink:         true,
		Dedicated:         true,
		WindowCount:       1,
		Focused:           true,
		WidthCols:         120,
		StatusItems:       []schema.UIStatusItem{{ID: "ctx", Label: "CTX 18%", MinWidth: 80, KeepPriority: 80}},
		OnOpenModels:      func() { clickedModel = true },
		OnOpenWorkspaces:  func() { clickedWS = true },
		OnReviewGate:      func() { clickedGate = true },
		OnToggleDeepThink: func() { clickedThink = true },
		OnToggleDedicated: func() { clickedDed = true },
		Theme:             th,
	}

	// 1. Wide 1280px frame: verify optical vertical centerline alignment and zero LHS/RHS overlap.
	tt := ui.NewTester(func(c *ui.Context) {
		modeline.Render(c, st)
	}, 1280, 40)
	tt.Frame()

	bufRect, okBuf := tt.Find("*conversation*")
	modeRect, okMode := tt.Find("Marlin-Chat")
	modelRect, okModel := tt.Find("deepseek/deepseek-flash")
	normalRect, okNorm := tt.Find("NORMAL")
	if !okBuf || !okMode || !okModel || !okNorm {
		t.Fatalf("expected all Modern Doom segments rendered on wide bar, got %q", strings.Join(tt.Texts(), " | "))
	}
	if bufRect.X+bufRect.W >= modelRect.X {
		t.Fatalf("LHS buffer info overlapped RHS model capsule: buf=%+v model=%+v", bufRect, modelRect)
	}
	if modelRect.X+modelRect.W >= modeRect.X {
		t.Fatalf("RHS model capsule overlapped major-mode anchor: model=%+v mode=%+v", modelRect, modeRect)
	}
	centerBuf := bufRect.Y + bufRect.H/2
	centerMode := modeRect.Y + modeRect.H/2
	centerNorm := normalRect.Y + normalRect.H/2
	if math.Abs(float64(centerBuf-centerMode)) > 2.5 || math.Abs(float64(centerBuf-centerNorm)) > 2.5 {
		t.Fatalf("vertical optical centerline drift > 2.5dp: buf=%.1f mode=%.1f normal=%.1f", centerBuf, centerMode, centerNorm)
	}

	// 2. Interactive clickable segments dispatch cleanly.
	for _, target := range []string{"deepseek/deepseek-flash", "[main]", "1 gate", "Think", "D"} {
		if err := tt.Click(target); err != nil {
			t.Fatalf("expected segment %q clickable: %v", target, err)
		}
		tt.Frame()
	}
	if !clickedModel || !clickedWS || !clickedGate || !clickedThink || !clickedDed {
		t.Fatalf("expected all interactive modeline callbacks fired: model=%v ws=%v gate=%v think=%v ded=%v",
			clickedModel, clickedWS, clickedGate, clickedThink, clickedDed)
	}

	// 3. Narrow 380px split pane: compact mode drops wide telemetry & shortens provider/model with zero overlap!
	narrow := st
	narrow.WindowCount = 3
	narrow.WindowNumber = 2
	narrow.WidthCols = 42
	ttNarrow := ui.NewTester(func(c *ui.Context) {
		modeline.Render(c, narrow)
	}, 380, 40)
	ttNarrow.Frame()
	if ttNarrow.HasText("feat/ui") || ttNarrow.HasText("12 rows") {
		t.Fatalf("narrow 380px split pane must prune wide-only vcs/position segments, got %q", strings.Join(ttNarrow.Texts(), " | "))
	}
	nBuf, okNB := ttNarrow.Find("*conversation*")
	nMod, okNM := ttNarrow.Find("deepseek-flash")
	if !okNB || !okNM || nBuf.X+nBuf.W > nMod.X+1 {
		t.Fatalf("narrow 380px split pane overlapped or missed compact model name: buf=%+v(%v) model=%+v(%v) texts=%q",
			nBuf, okNB, nMod, okNM, strings.Join(ttNarrow.Texts(), " | "))
	}
}

func containsSeg(list []modeline.SegmentID, target modeline.SegmentID) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}
