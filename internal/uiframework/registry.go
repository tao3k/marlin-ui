// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package uiframework implements Marlin UI's declarative Component Registry
// and live Scheme -> Rust -> gRPC (`marlin.ui.v1`) -> MyGo framework mapper.
//
// Upstream capabilities (Doom keymaps, Hydra transients, workspace windows,
// modeline status items, and slot trust policies) are authored ONCE in Gerbil
// Scheme, streamed over gRPC via `schema.UIWorkbenchSnapshot`, and mapped here
// onto native MyGo surfaces (Desktop, iOS, Android) with zero duplicate state
// machines.
package uiframework

import (
	"fmt"
	"sort"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/shellmanifest"
)

// ScopeKind categorizes whether a Component is shared across every surface or
// scoped to a specific target chrome.
type ScopeKind string

const (
	ScopeSharedCore ScopeKind = "shared-core"
	ScopeDesktop    ScopeKind = "desktop"
	ScopeIOS        ScopeKind = "ios"
	ScopeAndroid    ScopeKind = "android"
)

// Component is one modular, self-describing UI unit registered in the Marlin
// surface assembly graph (mirroring `scheme/init.ss` + `scheme/ui-mygo`).
type Component struct {
	ID             shellmanifest.ModuleID
	Name           string
	Scope          ScopeKind
	Slots          []string
	Requires       []shellmanifest.ModuleID
	SchemeDriven   bool // true when live-projected from marlin.ui.v1 Scheme snapshots
	AllowUntrusted bool // mirrors scheme/ui-mygo restricted-slot trust policy
}

// SlotPolicy describes one mounted UI slot and its trust boundary.
type SlotPolicy struct {
	SlotID         string
	Cardinality    string
	Fallback       string
	AllowUntrusted bool
}

// AssembledSurface is the validated, immutable component composition for one
// Marlin UI target (Desktop, iOS, or Android).
type AssembledSurface struct {
	Target            shellmanifest.TargetKind
	LayoutKind        string
	Shared            []Component
	TargetSpecific    []Component
	Slots             []SlotPolicy
	SchemeDrivenCount int
}

// HasModule reports whether mod is mounted on this assembled surface.
func (s AssembledSurface) HasModule(mod shellmanifest.ModuleID) bool {
	for _, c := range s.Shared {
		if c.ID == mod {
			return true
		}
	}
	for _, c := range s.TargetSpecific {
		if c.ID == mod {
			return true
		}
	}
	return false
}

// AdmitSlot enforces Marlin's fail-closed slot trust invariant (matching
// `scheme/ui-mygo/funcs.ss:marlin-mygo-admit-plugin-slot?`): restricted slots
// such as `approval.gate`, `host.auth`, and `host.secrets` NEVER admit
// untrusted plugins on any surface.
func (s AssembledSurface) AdmitSlot(slotID string, trusted bool) bool {
	switch slotID {
	case "approval.gate", "host.auth", "host.secrets":
		return trusted
	}
	for _, sl := range s.Slots {
		if sl.SlotID == slotID {
			return trusted || sl.AllowUntrusted
		}
	}
	return false
}

// Registry stores all registered modular Marlin UI Components.
type Registry struct {
	components map[shellmanifest.ModuleID]Component
}

// NewRegistry creates an empty modular Component registry.
func NewRegistry() *Registry {
	return &Registry{components: make(map[shellmanifest.ModuleID]Component)}
}

// Register adds or updates a Component descriptor in r.
func (r *Registry) Register(comp Component) error {
	if comp.ID == "" {
		return fmt.Errorf("uiframework: component ID must not be empty")
	}
	r.components[comp.ID] = comp
	return nil
}

// DefaultRegistry returns Marlin UI's canonical Component Registry matching
// `scheme/init.ss` and `scheme/ui-mygo/objects.ss` 1:1.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	builtins := []Component{
		// Shared Cross-Platform Core Modules (Desktop + iOS + Android).
		{ID: shellmanifest.ModThemes, Name: "Themes & Surface Tokens", Scope: ScopeSharedCore, SchemeDriven: true, AllowUntrusted: true},
		{ID: shellmanifest.ModTranscript, Name: "Streaming Markdown Transcript", Scope: ScopeSharedCore, Slots: []string{"workbench.main"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: true},
		{ID: shellmanifest.ModHelm, Name: "Helm Incremental Search", Scope: ScopeSharedCore, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModAgentRoom, Name: "Multi-Role Agent Room (RoomController)", Scope: ScopeSharedCore, Slots: []string{"workbench.main"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes, shellmanifest.ModPromptGate}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModPromptGate, Name: "Tier-0 PromptGate Approval", Scope: ScopeSharedCore, Slots: []string{"approval.gate"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModModelCatalog, Name: "Live Model Route Catalog", Scope: ScopeSharedCore, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: false},

		// Desktop Emacs Workbench Modules.
		{ID: shellmanifest.ModWorkspaces, Name: "Persp-Mode Workspaces & Popups", Scope: ScopeDesktop, Requires: []shellmanifest.ModuleID{shellmanifest.ModWindowTree}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModWindowTree, Name: "Recursive Multi-Window Tree (winum/winner)", Scope: ScopeDesktop, Slots: []string{"workbench.main", "workbench.inspector"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModDoomModeline, Name: "Per-Window Doom Modeline", Scope: ScopeDesktop, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes, shellmanifest.ModMeow}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModCentaurTabs, Name: "Centaur Workspace & Buffer Tabs", Scope: ScopeDesktop, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes, shellmanifest.ModWorkspaces}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModMeow, Name: "Meow Modal Keymap Engine", Scope: ScopeDesktop, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModHydra, Name: "Doom Leader / Hydra Transients", Scope: ScopeDesktop, Requires: []shellmanifest.ModuleID{shellmanifest.ModMeow, shellmanifest.ModThemes}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModMinibuffer, Name: "Vertico / Minibuffer Echo Area", Scope: ScopeDesktop, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes, shellmanifest.ModHydra, shellmanifest.ModHelm}, SchemeDriven: true, AllowUntrusted: false},
		{ID: shellmanifest.ModDashboardBento, Name: "EWW Bento Control Center Dashboard", Scope: ScopeDesktop, Slots: []string{"workbench.main"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: true},
		{ID: shellmanifest.ModWorkflowCanvas, Name: "Hybrid GPU Workflow Studio", Scope: ScopeDesktop, Slots: []string{"workbench.main"}, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: true},

		// Mobile iOS Companion Modules.
		{ID: shellmanifest.ModIOSTabBar, Name: "iOS 26 Floating Liquid Glass Tab Bar", Scope: ScopeIOS, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: false},
		{ID: shellmanifest.ModIOSSheets, Name: "iOS Native Spring Sheets (Channels/Details/Threads)", Scope: ScopeIOS, Requires: []shellmanifest.ModuleID{shellmanifest.ModAgentRoom, shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: false},
		{ID: shellmanifest.ModIOSBento, Name: "iOS 26 WidgetKit Bento Dashboard", Scope: ScopeIOS, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: false},

		// Mobile Android Companion Modules.
		{ID: shellmanifest.ModAndroidMaterial3, Name: "Android Material 3 Companion Chrome", Scope: ScopeAndroid, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: false},
		{ID: shellmanifest.ModAndroidNav, Name: "Android Companion Navigation", Scope: ScopeAndroid, Requires: []shellmanifest.ModuleID{shellmanifest.ModThemes}, SchemeDriven: false, AllowUntrusted: false},
	}
	for _, c := range builtins {
		_ = r.Register(c)
	}
	return r
}

// AssembleSurface validates and composes the declarative surface manifest for
// target, merging live Scheme `marlin.ui.v1` slot declarations when connected.
func (r *Registry) AssembleSurface(target shellmanifest.TargetKind, liveSlots []schema.SlotDeclaration) (AssembledSurface, error) {
	manifest := shellmanifest.ManifestFor(target)
	present := make(map[shellmanifest.ModuleID]bool, len(manifest.SharedModules)+len(manifest.TargetModules))
	for _, id := range manifest.AllModules() {
		present[id] = true
	}

	resolve := func(ids []shellmanifest.ModuleID) ([]Component, error) {
		out := make([]Component, 0, len(ids))
		for _, id := range ids {
			comp, ok := r.components[id]
			if !ok {
				return nil, fmt.Errorf("uiframework: target %s references unregistered module %q", target, id)
			}
			for _, dep := range comp.Requires {
				if !present[dep] {
					return nil, fmt.Errorf("uiframework: module %q on %s requires missing module %q", id, target, dep)
				}
			}
			out = append(out, comp)
		}
		return out, nil
	}

	shared, err := resolve(manifest.SharedModules)
	if err != nil {
		return AssembledSurface{}, err
	}
	targetSpec, err := resolve(manifest.TargetModules)
	if err != nil {
		return AssembledSurface{}, err
	}

	schemeCount := 0
	for _, c := range shared {
		if c.SchemeDriven {
			schemeCount++
		}
	}
	for _, c := range targetSpec {
		if c.SchemeDriven {
			schemeCount++
		}
	}

	slotMap := map[string]SlotPolicy{
		"workbench.main":      {SlotID: "workbench.main", Cardinality: "single", Fallback: "host.main", AllowUntrusted: true},
		"workbench.inspector": {SlotID: "workbench.inspector", Cardinality: "single", Fallback: "host.inspector", AllowUntrusted: true},
		"workbench.tool-card": {SlotID: "workbench.tool-card", Cardinality: "keyed", Fallback: "host.tool-card", AllowUntrusted: true},
		"approval.gate":       {SlotID: "approval.gate", Cardinality: "single", Fallback: "host.gate", AllowUntrusted: false},
	}
	for _, ls := range liveSlots {
		id := string(ls.SlotID)
		if id == "" {
			continue
		}
		allowUntrusted := ls.AllowUntrustedPlugin
		if id == "approval.gate" || id == "host.auth" || id == "host.secrets" {
			allowUntrusted = false // fail-closed invariant
		}
		slotMap[id] = SlotPolicy{
			SlotID:         id,
			Cardinality:    string(ls.Cardinality),
			Fallback:       ls.FallbackRenderer,
			AllowUntrusted: allowUntrusted,
		}
	}
	slots := make([]SlotPolicy, 0, len(slotMap))
	for _, sp := range slotMap {
		slots = append(slots, sp)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].SlotID < slots[j].SlotID })

	return AssembledSurface{
		Target:            target,
		LayoutKind:        manifest.LayoutKind,
		Shared:            shared,
		TargetSpecific:    targetSpec,
		Slots:             slots,
		SchemeDrivenCount: schemeCount,
	}, nil
}
