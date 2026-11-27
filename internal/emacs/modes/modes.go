// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package modes implements Marlin's Emacs-authentic Major Mode inheritance,
// composable Minor Modes, Mode Hooks, display-buffer-alist policies, and modern
// zero-indirect-buffer Span-driven Polymode (Host Mode + Inner Modes) runtime.
//
// Authoritative declarations live in `scheme/ui-modes/{types,objects,funcs}.ss`
// and stream live via Gerbil C-ABI FFI -> Rust gRPC; `DefaultRegistry()` serves
// as the matching offline mirror when running standalone.
package modes

import (
	"sort"
	"strings"
	"sync"
)

// SpanKind classifies the active structural span inside an AI-Native Polymode buffer.
type SpanKind string

const (
	SpanProse     SpanKind = "prose"
	SpanCodeBlock SpanKind = "code-block"
	SpanDiffHunk  SpanKind = "diff-hunk"
	SpanToolGate  SpanKind = "tool-gate"
	SpanArtifact  SpanKind = "artifact"
	SpanConflict  SpanKind = "conflict"
	SpanLogEntry  SpanKind = "log-entry"
	SpanTableRow  SpanKind = "table-row"
)

// DisplayAction mirrors Emacs `display-buffer-alist` placement policy.
type DisplayAction string

const (
	DisplayReuseWindow DisplayAction = "reuse-window"
	DisplaySplitRight  DisplayAction = "split-right"
	DisplaySplitBelow  DisplayAction = "split-below"
	DisplayPopupBottom DisplayAction = "popup-bottom"
	DisplayPopupRight  DisplayAction = "popup-right"
)

// HookPoint identifies a deterministic lifecycle hook point.
type HookPoint string

const (
	HookAfterChangeMajorMode HookPoint = "after-change-major-mode"
	HookEnterMinorMode       HookPoint = "enter-minor-mode"
	HookExitMinorMode        HookPoint = "exit-minor-mode"
	HookPolymodeSwitchSpan   HookPoint = "polymode-switch-span"
	HookWindowSelection      HookPoint = "window-selection-change"
	HookBufferCreate         HookPoint = "buffer-create"
	HookBufferKill           HookPoint = "buffer-kill"
)

// ModeBinding is one keymap entry owned by a Major, Minor, or Polymode Inner mode.
type ModeBinding struct {
	Key     string
	Command string
	Label   string
	Exit    bool
}

// MajorMode mirrors Emacs `define-derived-mode` declarations from `scheme/ui-modes`.
type MajorMode struct {
	ID                string
	Title             string
	Parent            string
	ReadOnly          bool
	DisplayAction     DisplayAction
	Dedicated         bool
	LocalKeymap       []ModeBinding
	DefaultMinorModes []string
	ModelineSegments  []string
	NewBufferActions  []string
}

// MinorMode is an orthogonal, priority-ordered modifier mode composable on any buffer.
type MinorMode struct {
	ID          string
	Title       string
	Lighter     string
	Priority    int
	LocalKeymap []ModeBinding
}

// PolymodeInnerRule maps a SpanKind inside a Polymode host buffer to an Inner Mode.
type PolymodeInnerRule struct {
	SpanKind  SpanKind
	InnerMode string
	Lighter   string
}

// PolymodeSpec attaches zero-indirect-buffer inner modes to a Host Major Mode.
type PolymodeSpec struct {
	ID         string
	HostMode   string
	InnerRules []PolymodeInnerRule
}

// BufferModeRule routes a buffer ID prefix (e.g. "*chat:", "*magit") to its
// Major Mode and drives the Tab Bar "+" Buffer / New Buffer picker.
type BufferModeRule struct {
	Prefix       string
	MajorMode    string
	Creatable    bool
	CreateLabel  string
	CreateAction string
}

// BufferState holds the live mode & Polymode span state for one buffer instance.
type BufferState struct {
	BufferID    string
	MajorMode   string
	MinorModes  []string
	FocusedSpan SpanKind
	InnerMode   string
}

// EffectiveStack is the resolved Emacs mode stack for a buffer at its current span.
type EffectiveStack struct {
	HostMode         string
	InnerMode        string
	MinorModes       []string
	MinorLighters    []string
	ParentChain      []string
	DisplayTitle     string // e.g. "Marlin-Chat[Diff·Aitia]" or "Magit-Status[Hunk·Aitia]"
	DisplayAction    DisplayAction
	ReadOnly         bool
	ModelineSegments []string
	HasSegment       map[string]bool
}

// HookEvent carries typed context to registered mode/Polymode hooks.
type HookEvent struct {
	Point     HookPoint
	BufferID  string
	MajorMode string
	InnerMode string
	PrevInner string
	MinorMode string
	Span      SpanKind
}

// HookFunc is a synchronous hook callback invoked on mode/span transitions.
type HookFunc func(ev HookEvent)

// Registry holds the authoritative Major Modes, Minor Modes, Polymode Inner
// Modes, Polymode Specs, Buffer routing rules, and live hooks.
type Registry struct {
	mu          sync.RWMutex
	majors      map[string]MajorMode
	minors      map[string]MinorMode
	inners      map[string]MajorMode
	polymodes   map[string]PolymodeSpec // keyed by HostMode
	bufferRules []BufferModeRule
	hooks       map[HookPoint][]HookFunc
}

// NewRegistry creates an empty Mode/Polymode registry.
func NewRegistry() *Registry {
	return &Registry{
		majors:    make(map[string]MajorMode),
		minors:    make(map[string]MinorMode),
		inners:    make(map[string]MajorMode),
		polymodes: make(map[string]PolymodeSpec),
		hooks:     make(map[HookPoint][]HookFunc),
	}
}

// AddHook registers a hook callback at a deterministic Emacs hook point.
func (r *Registry) AddHook(point HookPoint, fn HookFunc) {
	if fn == nil {
		return
	}
	r.mu.Lock()
	r.hooks[point] = append(r.hooks[point], fn)
	r.mu.Unlock()
}

func (r *Registry) runHooks(ev HookEvent) {
	r.mu.RLock()
	fns := append([]HookFunc(nil), r.hooks[ev.Point]...)
	r.mu.RUnlock()
	for _, fn := range fns {
		fn(ev)
	}
}

// ResolveBufferMode maps any buffer ID (including multi-instance "*chat:auth-fix*")
// to its Major Mode ID.
func (r *Registry) ResolveBufferMode(bufferID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rule := range r.bufferRules {
		if strings.HasPrefix(bufferID, rule.Prefix) {
			return rule.MajorMode
		}
	}
	if strings.HasPrefix(bufferID, "scheme-ext:") {
		return "marlin-special-mode"
	}
	return "fundamental-mode"
}

// ParentChain walks the Major/Inner Mode parent inheritance hierarchy safely (cycle-guarded).
func (r *Registry) ParentChain(modeID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.parentChainLocked(modeID)
}

func (r *Registry) parentChainLocked(modeID string) []string {
	var chain []string
	seen := make(map[string]bool, 8)
	cur := modeID
	for cur != "" && !seen[cur] {
		seen[cur] = true
		chain = append(chain, cur)
		if m, ok := r.majors[cur]; ok {
			cur = m.Parent
		} else if im, ok := r.inners[cur]; ok {
			cur = im.Parent
		} else {
			break
		}
	}
	return chain
}

// ResolvePolymodeInner returns the active Inner Mode ID and lighter for (hostMode, spanKind).
func (r *Registry) ResolvePolymodeInner(hostMode string, span SpanKind) (innerMode string, lighter string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resolvePolymodeInnerLocked(hostMode, span)
}

func (r *Registry) resolvePolymodeInnerLocked(hostMode string, span SpanKind) (string, string) {
	poly, ok := r.polymodes[hostMode]
	if !ok {
		return "", ""
	}
	for _, rule := range poly.InnerRules {
		if rule.SpanKind == span {
			return rule.InnerMode, rule.Lighter
		}
	}
	return "", ""
}

// MakeBufferState initializes a BufferState from the buffer's resolved Major Mode
// and fires `after-change-major-mode` hooks.
func (r *Registry) MakeBufferState(bufferID string) BufferState {
	majorID := r.ResolveBufferMode(bufferID)
	r.mu.RLock()
	m := r.majors[majorID]
	minors := append([]string(nil), m.DefaultMinorModes...)
	r.mu.RUnlock()
	st := BufferState{
		BufferID:    bufferID,
		MajorMode:   majorID,
		MinorModes:  minors,
		FocusedSpan: SpanProse,
	}
	r.runHooks(HookEvent{
		Point:     HookAfterChangeMajorMode,
		BufferID:  bufferID,
		MajorMode: majorID,
		Span:      SpanProse,
	})
	return st
}

// SwitchSpan updates a buffer's focused Polymode span and fires `polymode-switch-span`
// hooks ONLY when the effective Inner Mode actually changes.
func (r *Registry) SwitchSpan(st BufferState, nextSpan SpanKind) BufferState {
	prevInner := st.InnerMode
	nextInner, _ := r.ResolvePolymodeInner(st.MajorMode, nextSpan)
	st.FocusedSpan = nextSpan
	st.InnerMode = nextInner
	if nextInner != prevInner {
		r.runHooks(HookEvent{
			Point:     HookPolymodeSwitchSpan,
			BufferID:  st.BufferID,
			MajorMode: st.MajorMode,
			InnerMode: nextInner,
			PrevInner: prevInner,
			Span:      nextSpan,
		})
	}
	return st
}

// ToggleMinorMode enables or disables minorID on st and fires enter/exit hooks.
func (r *Registry) ToggleMinorMode(st BufferState, minorID string) BufferState {
	idx := -1
	for i, id := range st.MinorModes {
		if id == minorID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		st.MinorModes = append(append([]string(nil), st.MinorModes[:idx]...), st.MinorModes[idx+1:]...)
		r.runHooks(HookEvent{
			Point:     HookExitMinorMode,
			BufferID:  st.BufferID,
			MajorMode: st.MajorMode,
			MinorMode: minorID,
		})
	} else {
		st.MinorModes = append(append([]string(nil), st.MinorModes...), minorID)
		r.runHooks(HookEvent{
			Point:     HookEnterMinorMode,
			BufferID:  st.BufferID,
			MajorMode: st.MajorMode,
			MinorMode: minorID,
		})
	}
	return st
}

func (r *Registry) sortedMinorsLocked(ids []string) []MinorMode {
	out := make([]MinorMode, 0, len(ids))
	for _, id := range ids {
		if m, ok := r.minors[id]; ok {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out
}

// EffectiveStack computes the complete Major + Minor + Polymode Inner stack for
// rendering Modeline badges, display-buffer placement, and chrome policy.
func (r *Registry) EffectiveStack(st BufferState) EffectiveStack {
	r.mu.RLock()
	defer r.mu.RUnlock()
	host := r.majors[st.MajorMode]
	innerID, innerLighter := r.resolvePolymodeInnerLocked(st.MajorMode, st.FocusedSpan)
	minors := r.sortedMinorsLocked(st.MinorModes)

	minorIDs := make([]string, 0, len(minors))
	lighters := make([]string, 0, len(minors))
	for _, m := range minors {
		minorIDs = append(minorIDs, m.ID)
		if m.Lighter != "" {
			lighters = append(lighters, m.Lighter)
		}
	}

	title := host.Title
	if title == "" {
		title = st.MajorMode
	}
	if innerID != "" && innerLighter != "" {
		title = title + "[" + innerLighter + "]"
	}

	segs := append([]string(nil), host.ModelineSegments...)
	segMap := make(map[string]bool, len(segs))
	for _, s := range segs {
		segMap[s] = true
	}
	disp := host.DisplayAction
	if disp == "" {
		disp = DisplayReuseWindow
	}
	return EffectiveStack{
		HostMode:         st.MajorMode,
		InnerMode:        innerID,
		MinorModes:       minorIDs,
		MinorLighters:    lighters,
		ParentChain:      r.parentChainLocked(st.MajorMode),
		DisplayTitle:     title,
		DisplayAction:    disp,
		ReadOnly:         host.ReadOnly,
		ModelineSegments: segs,
		HasSegment:       segMap,
	}
}

// LookupKey resolves a key chord using exact Emacs precedence:
//   1. Active Minor Modes (highest priority first)
//   2. Polymode Inner Mode (walking its parent chain when inside an inner span)
//   3. Buffer Major Mode (walking its parent chain child -> parent)
// Returns (binding, true) on match, or (zero, false) to fall through to global-map.
func (r *Registry) LookupKey(st BufferState, key string) (ModeBinding, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Tier 1: Active Minor Modes (priority order)
	for _, m := range r.sortedMinorsLocked(st.MinorModes) {
		for _, b := range m.LocalKeymap {
			if b.Key == key {
				return b, true
			}
		}
	}

	// Tier 2: Polymode Inner Mode (if cursor is focused inside a matching span)
	if innerID, _ := r.resolvePolymodeInnerLocked(st.MajorMode, st.FocusedSpan); innerID != "" {
		for _, id := range r.parentChainLocked(innerID) {
			if im, ok := r.inners[id]; ok {
				for _, b := range im.LocalKeymap {
					if b.Key == key {
						return b, true
					}
				}
			} else if mm, ok := r.majors[id]; ok {
				for _, b := range mm.LocalKeymap {
					if b.Key == key {
						return b, true
					}
				}
			}
		}
	}

	// Tier 3: Host Major Mode -> Parent Chain
	for _, id := range r.parentChainLocked(st.MajorMode) {
		if m, ok := r.majors[id]; ok {
			for _, b := range m.LocalKeymap {
				if b.Key == key {
					return b, true
				}
			}
		}
	}
	return ModeBinding{}, false
}

// CreatableBufferRules returns the ordered list of buffer-creation entries
// powering the Tab Bar "+" Buffer / New Buffer picker.
func (r *Registry) CreatableBufferRules() []BufferModeRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BufferModeRule, 0, len(r.bufferRules))
	for _, rule := range r.bufferRules {
		if rule.Creatable {
			out = append(out, rule)
		}
	}
	return out
}

// MajorModeFor returns a copy of the MajorMode definition for id.
func (r *Registry) MajorModeFor(id string) (MajorMode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.majors[id]
	return m, ok
}

// InnerModeFor returns a copy of a Polymode Inner Mode definition for id.
func (r *Registry) InnerModeFor(id string) (MajorMode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.inners[id]
	return m, ok
}

// MinorModeFor returns a copy of a MinorMode definition for id.
func (r *Registry) MinorModeFor(id string) (MinorMode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.minors[id]
	return m, ok
}

// PolymodeFor returns the PolymodeSpec bound to hostMode, if any.
func (r *Registry) PolymodeFor(hostMode string) (PolymodeSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.polymodes[hostMode]
	return p, ok
}
