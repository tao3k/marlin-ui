// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package modes

import (
	"strconv"
	"strings"
)

// ModeRecord is a transport-neutral projection record mirroring Rust
// `marlin.projection.v1.GraphRecordView` for live Scheme->FFI->gRPC mode sync.
type ModeRecord struct {
	RecordID string
	Kind     string
	Title    string
	Facets   map[string]string
}

// DefaultRegistry returns the canonical Mode & Polymode registry matching
// `scheme/ui-modes/objects.ss` for standalone offline mode. When connected to
// `marlin-scheme-host` over gRPC, `ApplyGraphRecords` overlays live Scheme FFI
// projections directly onto this registry.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	majors := []MajorMode{
		{
			ID: "fundamental-mode", Title: "Fundamental", DisplayAction: DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "workspace-path", "activity"},
			NewBufferActions:  []string{"cmd:new-chat-buffer", "cmd:switch-buffer"},
		},
		{
			ID: "marlin-special-mode", Title: "Special", Parent: "fundamental-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "workspace-path", "activity"},
			LocalKeymap: []ModeBinding{
				{Key: "q", Command: "cmd:win-delete", Label: "Quit window", Exit: true},
				{Key: "g", Command: "cmd:buffer-refresh", Label: "Refresh buffer", Exit: true},
				{Key: "j", Command: "cmd:cursor-down", Label: "Next item"},
				{Key: "k", Command: "cmd:cursor-up", Label: "Previous item"},
			},
		},
		{
			ID: "marlin-chat-mode", Title: "Marlin-Chat", Parent: "fundamental-mode",
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "llm-inline-assist-mode", "aitia-audit-minor-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "polymode-inner", "deep-think", "composer-dock", "model-label", "session-id", "activity", "gate-badge"},
			NewBufferActions:  []string{"cmd:new-chat-buffer", "cmd:clone-window", "cmd:magit-status"},
			LocalKeymap: []ModeBinding{
				{Key: "i", Command: "cmd:insert", Label: "Focus prompt (Insert)", Exit: true},
				{Key: "C-c C-n", Command: "cmd:new-chat-buffer", Label: "New LLM Chat buffer", Exit: true},
				{Key: "C-c C-d", Command: "cmd:toggle-deep-think", Label: "Toggle DeepThink", Exit: true},
				{Key: "C-c C-m", Command: "buffer:*Models*", Label: "Switch LLM model", Exit: true},
			},
		},
		{
			ID: "marlin-room-mode", Title: "Agent-Room", Parent: "fundamental-mode",
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "llm-inline-assist-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "polymode-inner", "room-stats", "activity", "gate-badge"},
			NewBufferActions:  []string{"cmd:new-chat-buffer", "cmd:clone-window"},
		},
		{
			ID: "magit-mode", Title: "Magit", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "aitia-audit-minor-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "polymode-inner", "aitia-status", "branch-ref", "activity", "gate-badge"},
			LocalKeymap: []ModeBinding{
				{Key: "g", Command: "cmd:magit-refresh", Label: "Refresh & re-run Aitia gRPC", Exit: true},
				{Key: "?", Command: "hydra:magit:?", Label: "Magit dispatch"},
				{Key: "h", Command: "hydra:magit:?", Label: "Magit dispatch"},
				{Key: "TAB", Command: "cmd:magit-toggle-fold", Label: "Toggle section fold"},
				{Key: "1", Command: "cmd:magit-level-1", Label: "Visibility Level 1"},
				{Key: "2", Command: "cmd:magit-level-2", Label: "Visibility Level 2"},
				{Key: "3", Command: "cmd:magit-level-3", Label: "Visibility Level 3"},
				{Key: "4", Command: "cmd:magit-level-4", Label: "Visibility Level 4"},
				{Key: "n", Command: "cmd:magit-next-sibling", Label: "Next sibling section"},
				{Key: "p", Command: "cmd:magit-prev-sibling", Label: "Previous sibling section"},
				{Key: "^", Command: "cmd:magit-parent-section", Label: "Jump to parent section"},
				{Key: "RET", Command: "cmd:magit-visit-thing", Label: "Visit file at hunk line", Exit: true},
			},
		},
		{
			ID: "magit-status-mode", Title: "Magit-Status", Parent: "magit-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "aitia-audit-minor-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "polymode-inner", "aitia-status", "branch-ref", "activity", "gate-badge"},
			NewBufferActions:  []string{"cmd:magit-refresh", "cmd:magit-ai-audit", "cmd:clone-window"},
			LocalKeymap: []ModeBinding{
				{Key: "s", Command: "cmd:magit-stage", Label: "Stage hunk or file", Exit: true},
				{Key: "u", Command: "cmd:magit-unstage", Label: "Unstage hunk or file", Exit: true},
				{Key: "x", Command: "cmd:magit-discard", Label: "Discard hunk", Exit: true},
				{Key: "c", Command: "hydra:magit:c", Label: "Commit / jj describe"},
				{Key: "d", Command: "hydra:magit:d", Label: "Diff & Aitia/AI audit"},
				{Key: "b", Command: "hydra:magit:b", Label: "Branch / jj bookmark"},
				{Key: "f", Command: "hydra:magit:f", Label: "Fetch"},
				{Key: "F", Command: "hydra:magit:F", Label: "Pull"},
				{Key: "P", Command: "hydra:magit:P", Label: "Push"},
				{Key: "l", Command: "hydra:magit:l", Label: "Log / jj op log"},
				{Key: "r", Command: "hydra:magit:r", Label: "Rebase / squash / AI split"},
				{Key: "z", Command: "hydra:magit:z", Label: "Stash / jj undo"},
				{Key: "X", Command: "hydra:magit:X", Label: "Reset"},
				{Key: "A", Command: "hydra:magit:A", Label: "Cherry-pick"},
				{Key: "V", Command: "hydra:magit:V", Label: "Revert"},
				{Key: "m", Command: "hydra:magit:m", Label: "Merge / AI resolve"},
				{Key: "a", Command: "cmd:magit-ai-audit", Label: "AI Code Audit (50/50 split)", Exit: true},
			},
		},
		{
			ID: "magit-diff-mode", Title: "Magit-Diff", Parent: "magit-mode", ReadOnly: true,
			DisplayAction:     DisplaySplitRight,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "aitia-audit-minor-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "aitia-status", "activity", "gate-badge"},
		},
		{
			ID: "magit-log-mode", Title: "Magit-Log", Parent: "magit-mode", ReadOnly: true,
			DisplayAction:     DisplaySplitRight,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "branch-ref", "activity", "gate-badge"},
		},
		{
			ID: "marlin-dashboard-mode", Title: "Bento-Dashboard", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "workspace-path", "dashboard-preset", "version"},
		},
		{
			ID: "marlin-workflow-mode", Title: "Workflow-Canvas", Parent: "marlin-special-mode",
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "workspace-path", "activity", "gate-badge"},
		},
		{
			ID: "marlin-tabulated-mode", Title: "Tabulated-List", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "row-count", "activity", "gate-badge"},
			LocalKeymap: []ModeBinding{
				{Key: "RET", Command: "cmd:table-select", Label: "Open selected row", Exit: true},
				{Key: "U", Command: "cmd:ibuffer-unmark-all", Label: "Unmark all buffers", Exit: true},
				{Key: "x", Command: "cmd:ibuffer-kill-marked", Label: "Kill marked buffers", Exit: true},
			},
		},
		{
			ID: "marlin-preview-mode", Title: "Preview-Mode", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction:     DisplaySplitRight,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "aitia-audit-minor-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "workspace-path", "gate-badge"},
		},
		{
			ID: "marlin-help-mode", Title: "Help", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction: DisplayPopupBottom, Dedicated: true,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "dedicated-window-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "gate-badge"},
		},
		{
			ID: "marlin-messages-mode", Title: "Messages", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction: DisplayPopupBottom, Dedicated: true,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode", "dedicated-window-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "gate-badge"},
		},
		{
			ID: "marlin-settings-mode", Title: "Customize", Parent: "marlin-special-mode",
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "gate-badge"},
		},
		{
			ID: "marlin-packages-mode", Title: "Package-Menu", Parent: "marlin-special-mode", ReadOnly: true,
			DisplayAction:     DisplayReuseWindow,
			DefaultMinorModes: []string{"meow-normal-mode", "read-only-mode"},
			ModelineSegments:  []string{"key-mode", "buffer-name", "major-mode", "activity", "gate-badge"},
		},
	}
	for _, m := range majors {
		r.majors[m.ID] = m
	}

	minors := []MinorMode{
		{ID: "meow-normal-mode", Title: "Meow Normal", Lighter: "NORMAL", Priority: 100},
		{ID: "meow-insert-mode", Title: "Meow Insert", Lighter: "INSERT", Priority: 110},
		{
			ID: "aitia-audit-minor-mode", Title: "Lambda Aitia Audit", Lighter: "λAitia", Priority: 80,
			LocalKeymap: []ModeBinding{
				{Key: "C-c g", Command: "cmd:magit-ai-audit", Label: "AI + Aitia Code Audit", Exit: true},
				{Key: "C-c M-g", Command: "hydra:magit:d", Label: "Aitia Diff Dispatch"},
			},
		},
		{ID: "llm-inline-assist-mode", Title: "LLM Inline Assist", Lighter: "AI", Priority: 60},
		{ID: "zen-writeroom-mode", Title: "Zen Writeroom", Lighter: "Zen", Priority: 50},
		{ID: "liquid-glass-mode", Title: "Liquid Glass", Lighter: "Glass", Priority: 40},
		{ID: "dedicated-window-mode", Title: "Dedicated Window", Lighter: "Pin", Priority: 30},
		{ID: "read-only-mode", Title: "Read-Only", Lighter: "RO", Priority: 20},
	}
	for _, m := range minors {
		r.minors[m.ID] = m
	}

	inners := []MajorMode{
		{
			ID: "prog-code-inner-mode", Title: "Code-Span", Parent: "fundamental-mode", ReadOnly: true,
			LocalKeymap: []ModeBinding{
				{Key: "y", Command: "cmd:span-copy-code", Label: "Copy code block", Exit: true},
				{Key: "RET", Command: "cmd:span-open-preview", Label: "Open code in *Preview* split", Exit: true},
				{Key: "a", Command: "cmd:span-ai-explain", Label: "AI explain / audit code span", Exit: true},
			},
		},
		{
			ID: "magit-diff-inner-mode", Title: "Diff-Hunk", Parent: "magit-diff-mode", ReadOnly: true,
			LocalKeymap: []ModeBinding{
				{Key: "s", Command: "cmd:magit-stage", Label: "Stage diff hunk", Exit: true},
				{Key: "u", Command: "cmd:magit-unstage", Label: "Unstage diff hunk", Exit: true},
				{Key: "x", Command: "cmd:magit-discard", Label: "Discard diff hunk", Exit: true},
				{Key: "a", Command: "cmd:magit-ai-audit", Label: "AI + Aitia Audit hunk", Exit: true},
				{Key: "SPC", Command: "cmd:magit-audit-toggle", Label: "Toggle hunk audited", Exit: true},
				{Key: "RET", Command: "cmd:magit-visit-thing", Label: "Visit file at hunk line", Exit: true},
			},
		},
		{
			ID: "marlin-gate-inner-mode", Title: "Approval-Gate", Parent: "marlin-special-mode", ReadOnly: true,
			LocalKeymap: []ModeBinding{
				{Key: "y", Command: "cmd:gate-approve", Label: "Approve tool/action gate", Exit: true},
				{Key: "n", Command: "cmd:gate-reject", Label: "Reject tool/action gate", Exit: true},
				{Key: "i", Command: "cmd:review-gate", Label: "Inspect gate details", Exit: true},
			},
		},
		{
			ID: "marlin-artifact-inner-mode", Title: "Artifact-Span", Parent: "marlin-special-mode", ReadOnly: true,
			LocalKeymap: []ModeBinding{
				{Key: "RET", Command: "cmd:artifact-open-stage", Label: "Open artifact in Stage split", Exit: true},
				{Key: "y", Command: "cmd:span-copy-code", Label: "Copy artifact source", Exit: true},
			},
		},
	}
	for _, im := range inners {
		r.inners[im.ID] = im
	}

	polys := []PolymodeSpec{
		{
			ID: "poly-marlin-chat-mode", HostMode: "marlin-chat-mode",
			InnerRules: []PolymodeInnerRule{
				{SpanKind: SpanCodeBlock, InnerMode: "prog-code-inner-mode", Lighter: "Code"},
				{SpanKind: SpanDiffHunk, InnerMode: "magit-diff-inner-mode", Lighter: "Diff·Aitia"},
				{SpanKind: SpanToolGate, InnerMode: "marlin-gate-inner-mode", Lighter: "Gate"},
				{SpanKind: SpanArtifact, InnerMode: "marlin-artifact-inner-mode", Lighter: "Artifact"},
			},
		},
		{
			ID: "poly-magit-status-mode", HostMode: "magit-status-mode",
			InnerRules: []PolymodeInnerRule{
				{SpanKind: SpanDiffHunk, InnerMode: "magit-diff-inner-mode", Lighter: "Hunk·Aitia"},
				{SpanKind: SpanConflict, InnerMode: "magit-diff-inner-mode", Lighter: "Conflict"},
				{SpanKind: SpanCodeBlock, InnerMode: "prog-code-inner-mode", Lighter: "Code"},
			},
		},
		{
			ID: "poly-marlin-room-mode", HostMode: "marlin-room-mode",
			InnerRules: []PolymodeInnerRule{
				{SpanKind: SpanCodeBlock, InnerMode: "prog-code-inner-mode", Lighter: "Code"},
				{SpanKind: SpanDiffHunk, InnerMode: "magit-diff-inner-mode", Lighter: "Diff·Aitia"},
				{SpanKind: SpanToolGate, InnerMode: "marlin-gate-inner-mode", Lighter: "Gate"},
			},
		},
	}
	for _, p := range polys {
		r.polymodes[p.HostMode] = p
	}

	r.bufferRules = []BufferModeRule{
		{Prefix: "*chat:", MajorMode: "marlin-chat-mode", Creatable: true, CreateLabel: "New LLM Chat (*chat:*)", CreateAction: "cmd:new-chat-buffer"},
		{Prefix: "*conversation*", MajorMode: "marlin-chat-mode", Creatable: false, CreateLabel: "Conversation (*conversation*)", CreateAction: "buffer:*conversation*"},
		{Prefix: "*magit", MajorMode: "magit-status-mode", Creatable: true, CreateLabel: "Magit Status (*magit*)", CreateAction: "cmd:magit-status"},
		{Prefix: "*Agent Room*", MajorMode: "marlin-room-mode", Creatable: true, CreateLabel: "Agent Room (*Agent Room*)", CreateAction: "feature:chatroom"},
		{Prefix: "feature:chatroom", MajorMode: "marlin-room-mode", Creatable: false, CreateLabel: "Agent Room", CreateAction: "feature:chatroom"},
		{Prefix: "*Workflow Canvas*", MajorMode: "marlin-workflow-mode", Creatable: true, CreateLabel: "Workflow Canvas (*Workflow Canvas*)", CreateAction: "feature:webisland"},
		{Prefix: "feature:webisland", MajorMode: "marlin-workflow-mode", Creatable: false, CreateLabel: "Workflow Canvas", CreateAction: "feature:webisland"},
		{Prefix: "*Dashboard*", MajorMode: "marlin-dashboard-mode", Creatable: true, CreateLabel: "Dashboard (*Dashboard*)", CreateAction: "buffer:*Dashboard*"},
		{Prefix: "*Sessions*", MajorMode: "marlin-tabulated-mode", Creatable: true, CreateLabel: "Sessions (*Sessions*)", CreateAction: "buffer:*Sessions*"},
		{Prefix: "*Ibuffer*", MajorMode: "marlin-tabulated-mode", Creatable: true, CreateLabel: "Ibuffer (*Ibuffer* by Major Mode)", CreateAction: "cmd:ibuffer"},
		{Prefix: "*Targets*", MajorMode: "marlin-tabulated-mode", Creatable: true, CreateLabel: "Targets (*Targets*)", CreateAction: "buffer:*Targets*"},
		{Prefix: "*Processes*", MajorMode: "marlin-tabulated-mode", Creatable: true, CreateLabel: "Processes (*Processes*)", CreateAction: "buffer:*Processes*"},
		{Prefix: "*Models*", MajorMode: "marlin-tabulated-mode", Creatable: true, CreateLabel: "Models (*Models*)", CreateAction: "buffer:*Models*"},
		{Prefix: "*Preview*", MajorMode: "marlin-preview-mode", Creatable: false, CreateLabel: "Preview (*Preview*)", CreateAction: "buffer:*Preview*"},
		{Prefix: "*Help*", MajorMode: "marlin-help-mode", Creatable: true, CreateLabel: "Help (*Help*)", CreateAction: "buffer:*Help*"},
		{Prefix: "*Messages*", MajorMode: "marlin-messages-mode", Creatable: true, CreateLabel: "Messages (*Messages*)", CreateAction: "buffer:*Messages*"},
		{Prefix: "*Settings*", MajorMode: "marlin-settings-mode", Creatable: true, CreateLabel: "Settings (*Settings*)", CreateAction: "buffer:*Settings*"},
		{Prefix: "*Packages*", MajorMode: "marlin-packages-mode", Creatable: true, CreateLabel: "Packages (*Packages*)", CreateAction: "buffer:*Packages*"},
	}
	return r
}

// ApplyGraphRecords overlays live Scheme->FFI->Rust gRPC mode/Polymode records
// (`ui.mode.major`, `ui.mode.minor`, `ui.mode.inner`, `ui.polymode`, `ui.buffer_rule`)
// onto the registry without losing registered Go hooks.
func (r *Registry) ApplyModeRecords(records []ModeRecord) int {
	if len(records) == 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	applied := 0
	var newRules []BufferModeRule
	for _, rec := range records {
		switch rec.Kind {
		case "ui.mode.major", "ui.mode.inner":
			id := strings.TrimSpace(rec.Facets["id"])
			if id == "" {
				continue
			}
			m := MajorMode{
				ID:                id,
				Title:             rec.Title,
				Parent:            strings.TrimSpace(rec.Facets["parent"]),
				ReadOnly:          strings.EqualFold(rec.Facets["read_only"], "true"),
				DisplayAction:     DisplayAction(strings.TrimSpace(rec.Facets["display_action"])),
				Dedicated:         strings.EqualFold(rec.Facets["dedicated"], "true"),
				LocalKeymap:       parseCompactBindings(rec.Facets["keymap"]),
				DefaultMinorModes: splitCSV(rec.Facets["default_minors"]),
				ModelineSegments:  splitCSV(rec.Facets["modeline_segments"]),
				NewBufferActions:  splitCSV(rec.Facets["new_actions"]),
			}
			if rec.Kind == "ui.mode.major" {
				r.majors[id] = m
			} else {
				r.inners[id] = m
			}
			applied++
		case "ui.mode.minor":
			id := strings.TrimSpace(rec.Facets["id"])
			if id == "" {
				continue
			}
			prio, _ := strconv.Atoi(strings.TrimSpace(rec.Facets["priority"]))
			r.minors[id] = MinorMode{
				ID:          id,
				Title:       rec.Title,
				Lighter:     rec.Facets["lighter"],
				Priority:    prio,
				LocalKeymap: parseCompactBindings(rec.Facets["keymap"]),
			}
			applied++
		case "ui.polymode":
			host := strings.TrimSpace(rec.Facets["host_mode"])
			if host == "" {
				continue
			}
			var innerRules []PolymodeInnerRule
			for _, line := range strings.Split(rec.Facets["inner_rules"], "\n") {
				parts := strings.Split(line, "\t")
				if len(parts) < 3 {
					continue
				}
				innerRules = append(innerRules, PolymodeInnerRule{
					SpanKind:  SpanKind(strings.TrimSpace(parts[0])),
					InnerMode: strings.TrimSpace(parts[1]),
					Lighter:   strings.TrimSpace(parts[2]),
				})
			}
			r.polymodes[host] = PolymodeSpec{
				ID:         strings.TrimSpace(rec.Facets["id"]),
				HostMode:   host,
				InnerRules: innerRules,
			}
			applied++
		case "ui.buffer_rule":
			pref := strings.TrimSpace(rec.Facets["prefix"])
			if pref == "" {
				continue
			}
			newRules = append(newRules, BufferModeRule{
				Prefix:       pref,
				MajorMode:    strings.TrimSpace(rec.Facets["major_mode"]),
				Creatable:    strings.EqualFold(rec.Facets["creatable"], "true"),
				CreateLabel:  rec.Facets["create_label"],
				CreateAction: rec.Facets["create_action"],
			})
			applied++
		}
	}
	if len(newRules) > 0 {
		r.bufferRules = newRules
	}
	return applied
}

func parseCompactBindings(raw string) []ModeBinding {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []ModeBinding
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		out = append(out, ModeBinding{
			Key:     strings.TrimSpace(parts[0]),
			Command: strings.TrimSpace(parts[1]),
			Label:   strings.TrimSpace(parts[2]),
			Exit:    strings.EqualFold(strings.TrimSpace(parts[3]), "true"),
		})
	}
	return out
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
