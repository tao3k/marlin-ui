// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"strings"
	"testing"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
)

func TestMagitDoomKeymapsAndTransients(t *testing.T) {
	// 1. Desktop defaults/keymaps.toml and defaults/commands.toml MUST NOT
	// duplicate Scheme-owned Magit keymaps/transients/commands (single source of
	// truth lives in scheme/ui-meow & scheme/ui-magit, delivered over marlin.ui.v1 gRPC).
	desktopTOML := loadDesktopTOMLKeymaps()
	if _, duplicated := desktopTOML["SPC g"]; duplicated {
		t.Fatalf("defaults/keymaps.toml must not duplicate Scheme-owned SPC g leader map")
	}
	if _, duplicated := desktopTOML["magit:?"]; duplicated {
		t.Fatalf("defaults/keymaps.toml must not duplicate Scheme-owned magit:* transients")
	}
	for _, cmd := range loadDefaultCommands() {
		if strings.HasPrefix(cmd.Name, "magit-") {
			t.Fatalf("defaults/commands.toml must not duplicate Scheme-owned command %q", cmd.Name)
		}
	}

	// 2. Apply a live marlin.ui.v1 workbench snapshot carrying Scheme ui-meow +
	// ui-magit keymaps, transients, and commands over gRPC.
	f := New(Config{AppConfig: appconfig.Config{SessionID: "magit-test"}})
	f.ApplyWorkbenchSnapshot(schema.UIWorkbenchSnapshot{
		Generation: 42,
		Commands: []schema.UICommandSpec{
			{Name: "magit-status", Description: "Open *magit* (SPC g g / C-x g)"},
			{Name: "magit-ai-audit", Description: "Open 50/50 AI Code Audit (SPC g a / C-c g)"},
		},
		Keymap: []schema.UIKeyBindingSpec{
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "g", "g"}, CommandName: "magit-status"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "g", "d"}, CommandName: "magit-dispatch"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "g", "a"}, CommandName: "magit-ai-audit"},
			{Scope: schema.KeyScopeNormal, Sequence: []string{"SPC", "g", "c"}, CommandName: "hydra:magit:c"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "g"}, CommandName: "magit-status"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-x", "M-g"}, CommandName: "magit-dispatch"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "g"}, CommandName: "magit-ai-audit"},
			{Scope: schema.KeyScopeGlobal, Sequence: []string{"C-c", "M-g"}, CommandName: "magit-file-dispatch"},
		},
		Transients: []schema.UITransient{
			{
				ID:   "magit:?",
				Hint: schema.UIElement{Title: "Magit Dispatch (? / C-x M-g)"},
				Heads: []schema.UITransientHead{
					{Key: "c", CommandName: "hydra:magit:c", Label: "Commit / jj describe", Exit: false},
					{Key: "d", CommandName: "hydra:magit:d", Label: "Diff & Aitia/AI Audit", Exit: false},
					{Key: "q", CommandName: "exit", Label: "Quit", Exit: true},
				},
			},
			{
				ID:   "magit:c",
				Hint: schema.UIElement{Title: "Magit Commit (c)"},
				Heads: []schema.UITransientHead{
					{Key: "c", CommandName: "magit-commit-prompt", Label: "Commit staged", Exit: true},
					{Key: "e", CommandName: "magit-extend", Label: "Extend HEAD", Exit: true},
					{Key: "i", CommandName: "magit-ai-commit", Label: "AI Commit Draft", Exit: true},
					{Key: "q", CommandName: "exit", Label: "Quit", Exit: true},
				},
			},
			{
				ID:   "magit:d",
				Hint: schema.UIElement{Title: "Magit Diff, Aitia & AI Audit (d)"},
				Heads: []schema.UITransientHead{
					{Key: "a", CommandName: "magit-ai-audit", Label: "AI Audit selected hunk", Exit: true},
					{Key: "A", CommandName: "magit-refresh", Label: "Re-run Lambda Aitia gRPC", Exit: true},
					{Key: "q", CommandName: "exit", Label: "Quit", Exit: true},
				},
			},
		},
	})
	snap := f.loadSnapshot()

	for _, prefix := range []string{"SPC", "SPC g", "C-x", "C-c", "magit:?", "magit:c", "magit:d"} {
		def, ok := snap.TomlHydras[prefix]
		if !ok || len(def.Heads) == 0 {
			t.Fatalf("expected Scheme-projected hydra %q to be registered via gRPC", prefix)
		}
	}
	hasHead := func(prefix, key, action string) bool {
		for _, h := range snap.TomlHydras[prefix].Heads {
			if h.Key == key && h.Action == action {
				return true
			}
		}
		return false
	}
	if !hasHead("SPC", "g", "hydra:SPC g") {
		t.Fatalf("expected authentic Doom SPC g leader head projected from Scheme gRPC")
	}
	if !hasHead("SPC g", "g", "cmd:magit-status") {
		t.Fatalf("expected SPC g g -> cmd:magit-status projected from Scheme gRPC")
	}
	if !hasHead("C-x", "g", "cmd:magit-status") {
		t.Fatalf("expected canonical native Emacs C-x g -> cmd:magit-status binding")
	}
	if !hasHead("C-x", "M-g", "hydra:magit:?") {
		t.Fatalf("expected canonical native Emacs C-x M-g -> hydra:magit:? binding")
	}
	if !hasHead("C-c", "g", "cmd:magit-ai-audit") {
		t.Fatalf("expected native Emacs C-c g -> cmd:magit-ai-audit binding")
	}
	if !hasHead("C-c", "M-g", "hydra:magit:d") {
		t.Fatalf("expected native Emacs C-c M-g -> hydra:magit:d binding")
	}
	if !hasHead("magit:?", "c", "hydra:magit:c") || !hasHead("magit:c", "e", "cmd:magit-extend") {
		t.Fatalf("expected Magit transient heads projected directly from Scheme gRPC")
	}

	// 3. Downstream user TOML (~/.config/marlin/ui/keymaps.toml) overrides any
	// Scheme-delivered Magit key cleanly without touching Scheme or Desktop defaults.
	override, err := parseKeymapsTOML(`
[[heads]]
prefix = "SPC g"
key = "g"
label = "Custom User Magit Refresh"
column = "User"
exit = "true"
action = "cmd:magit-refresh"
`)
	if err != nil {
		t.Fatalf("parse user override TOML: %v", err)
	}
	f.mu.Lock()
	f.userTOMLOverlay = override
	f.rebuildEffectiveHydrasLocked()
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if !hasHead("SPC g", "g", "cmd:magit-status") {
		t.Fatalf("previous snapshot should remain immutable")
	}
	snapAfter := f.loadSnapshot()
	foundOverride := false
	for _, h := range snapAfter.TomlHydras["SPC g"].Heads {
		if h.Key == "g" && h.Action == "cmd:magit-refresh" {
			foundOverride = true
		}
	}
	if !foundOverride {
		t.Fatalf("expected downstream user TOML override to replace Scheme SPC g g binding")
	}
}

func TestMagitAuditProgressAndSplitAIAudit(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	var sentPrompts []string
	adaptive.Desktop.OnSendPrompt = func(prompt string) {
		sentPrompts = append(sentPrompts, prompt)
	}
	f := New(Config{
		Adaptive:  adaptive,
		AppConfig: appconfig.Config{SessionID: "magit-audit"},
	})

	f.OpenMagit()
	if f.ActiveBuffer() != buffer.Magit {
		t.Fatalf("expected active buffer *magit*, got %s", f.ActiveBuffer())
	}

	view := f.loadSnapshot().Magit
	if view == nil || len(view.Rows) == 0 {
		t.Fatalf("expected *magit* view rows in standalone preview")
	}

	// Verify visibility level switching (1..4) and fold toggle.
	f.setMagitVisibilityLevel(1)
	if len(f.loadSnapshot().Magit.Rows) >= len(view.Rows) {
		t.Fatalf("expected visibility level 1 to collapse file/hunk rows")
	}
	f.setMagitVisibilityLevel(3)

	// Mark first hunk audited and verify honest evidence chain + audit stats update cleanly.
	firstHunkID := view.Snapshot.Unstaged[0].Hunks[0].HunkID
	if len(view.Snapshot.Unstaged[0].Hunks[0].Evidence) < 2 {
		t.Fatalf("expected explicit multi-source evidence chain items on demo hunk")
	}
	_, _, _, totalBefore, _ := buffer.ComputeMagitAuditStats(view.Snapshot, nil)
	if totalBefore < 2 {
		t.Fatalf("expected demo snapshot to have >=2 hunks, got total=%d", totalBefore)
	}
	f.handleMagitAction("audit-toggle", firstHunkID)
	afterView := f.loadSnapshot().Magit
	aud, _, _, _, highAfter := buffer.ComputeMagitAuditStats(afterView.Snapshot, afterView.AuditMap)
	if aud != 1 || highAfter != 0 {
		t.Fatalf("expected 1 audited hunk and 0 high-risk remaining, got aud=%d high=%d", aud, highAfter)
	}

	// Trigger AI Code Audit (d a / SPC g a) -> must open 50/50 split and dispatch Aitia-grounded prompt.
	f.handleMagitAction("ai-audit", firstHunkID)
	if len(sentPrompts) != 1 {
		t.Fatalf("expected 1 AI audit prompt sent, got %d", len(sentPrompts))
	}
	if !strings.Contains(sentPrompts[0], "Aitia:") || !strings.Contains(sentPrompts[0], "engine.rs") {
		t.Fatalf("expected Aitia-grounded audit prompt referencing engine.rs, got %q", sentPrompts[0])
	}
	if f.WorkspaceSnapshot().Active.WindowCount() < 2 {
		t.Fatalf("expected AI Code Audit to open a 50/50 split alongside *magit*")
	}

	// Verify interactive Minibuffer commit prompt opens cleanly.
	f.handleMagitAction("commit-prompt", "")
	if f.MiniMode() != minibuffer.ModeMagitPrompt {
		t.Fatalf("expected ModeMagitPrompt, got %v", f.MiniMode())
	}
}
