// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
)

func (f *Frame) ensureModeRegistryLocked() *modes.Registry {
	if f.modeReg == nil {
		f.modeReg = modes.DefaultRegistry()
	}
	if f.bufModes == nil {
		f.bufModes = make(map[buffer.ID]modes.BufferState, 16)
	}
	return f.modeReg
}

// ModeRegistry returns the Frame's Major/Minor/Polymode & Hook registry.
func (f *Frame) ModeRegistry() *modes.Registry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensureModeRegistryLocked()
}

// ApplySchemeModeRecords overlays live Scheme->FFI->Rust gRPC mode & Polymode
// records onto the Frame's mode registry and republishes the RCU snapshot.
func (f *Frame) ApplySchemeModeRecords(records []modes.ModeRecord) int {
	reg := f.ModeRegistry()
	applied := reg.ApplyModeRecords(records)
	if applied > 0 {
		f.mu.Lock()
		f.publishSnapshotLocked()
		f.mu.Unlock()
		f.requestFrame()
	}
	return applied
}

// BufferModeState returns (initializing lazily if needed) the Major/Minor/Polymode
// state for buffer id.
func (f *Frame) BufferModeState(id buffer.ID) modes.BufferState {
	f.mu.Lock()
	reg := f.ensureModeRegistryLocked()
	st, ok := f.bufModes[id]
	f.mu.Unlock()
	if ok {
		return st
	}
	st = reg.MakeBufferState(string(id))
	f.mu.Lock()
	if existing, exists := f.bufModes[id]; exists {
		f.mu.Unlock()
		return existing
	}
	f.bufModes[id] = st
	f.mu.Unlock()
	return st
}

// EffectiveModeStack computes the resolved Emacs Major + Minor + Polymode Inner
// mode stack for buffer id (driving Modeline display, keymap lookup, and chrome).
func (f *Frame) EffectiveModeStack(id buffer.ID) modes.EffectiveStack {
	st := f.BufferModeState(id)
	return f.ModeRegistry().EffectiveStack(st)
}

// FocusPolymodeSpan updates the focused structural span (e.g. prose, code-block,
// diff-hunk, tool-gate, artifact) for buffer id, firing `polymode-switch-span`
// hooks whenever the effective Polymode Inner Mode transitions.
func (f *Frame) FocusPolymodeSpan(id buffer.ID, span modes.SpanKind) modes.BufferState {
	cur := f.BufferModeState(id)
	if cur.FocusedSpan == span {
		return cur
	}
	next := f.ModeRegistry().SwitchSpan(cur, span)
	f.mu.Lock()
	f.bufModes[id] = next
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return next
}

// ToggleBufferMinorMode enables or disables minorID on buffer id and fires
// `enter-minor-mode` / `exit-minor-mode` hooks.
func (f *Frame) ToggleBufferMinorMode(id buffer.ID, minorID string) modes.BufferState {
	cur := f.BufferModeState(id)
	next := f.ModeRegistry().ToggleMinorMode(cur, minorID)
	f.mu.Lock()
	f.bufModes[id] = next
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return next
}

// LookupModeKey resolves key against the active buffer's Emacs precedence chain:
//   minor-mode-map-alist -> polymode inner-mode-map -> major-mode-map -> parent-chain.
func (f *Frame) LookupModeKey(id buffer.ID, key string) (modes.ModeBinding, bool) {
	st := f.BufferModeState(id)
	return f.ModeRegistry().LookupKey(st, key)
}

// DispatchModeKey attempts to handle key via the active buffer's Major/Minor/Polymode
// local keymap stack, returning true when consumed.
func (f *Frame) DispatchModeKey(key string) bool {
	buf := f.ActiveBuffer()
	binding, ok := f.LookupModeKey(buf, key)
	if !ok || strings.TrimSpace(binding.Command) == "" {
		return false
	}
	f.dispatchWidgetAction(binding.Command)
	return true
}

// dispatchBufferModeKey is the mode-aware OnBufferKey dispatcher invoked in
// Meow NORMAL mode: delegates Magit buffers to dispatchMagitKey and resolves
// all other Major/Minor/Polymode local single-key & chord bindings via LookupModeKey.
func (f *Frame) dispatchBufferModeKey(c *ui.Context, scope ui.Element) bool {
	snap := f.loadSnapshot()
	buf := snap.ActiveBuffer
	eff := f.EffectiveModeStack(buf)
	if eff.HostMode == "magit-status-mode" || eff.HostMode == "magit-mode" || eff.HostMode == "magit-diff-mode" {
		return f.dispatchMagitKey(c, scope)
	}
	// Check mode-local key bindings in exact Emacs precedence order:
	//   minor-modes -> polymode inner-mode -> major-mode -> parent-chain.
	st := f.BufferModeState(buf)
	candidates := []struct {
		key  string
		mods ui.Modifiers
		code ui.Key
	}{
		{"q", 0, ui.KeyQ},
		{"g", 0, ui.KeyG},
		{"s", 0, ui.KeyS},
		{"u", 0, ui.KeyU},
		{"x", 0, ui.KeyX},
		{"a", 0, ui.KeyA},
		{"y", 0, ui.KeyY},
		{"n", 0, ui.KeyN},
		{"r", 0, ui.KeyR},
		{"U", ui.Shift, ui.KeyU},
		{"RET", 0, ui.KeyEnter},
	}
	matchShortcut := func(mods ui.Modifiers, key ui.Key) bool {
		if scope != (ui.Element{}) && scope.Shortcut(mods, key) {
			return true
		}
		return c.Shortcut(mods, key)
	}
	for _, cand := range candidates {
		binding, ok := f.ModeRegistry().LookupKey(st, cand.key)
		if !ok || strings.TrimSpace(binding.Command) == "" {
			continue
		}
		if matchShortcut(cand.mods, cand.code) {
			f.dispatchWidgetAction(binding.Command)
			return true
		}
	}
	return false
}

// NewChatBuffer creates a fresh multi-instance LLM conversation buffer
// (`*chat:<name>*`) governed by `marlin-chat-mode` + `poly-marlin-chat-mode`,
// binds a new backend session if attached, and opens it in the active window.
func (f *Frame) NewChatBuffer(name string) buffer.ID {
	f.mu.Lock()
	f.chatSeq++
	seq := f.chatSeq
	f.mu.Unlock()

	clean := strings.TrimSpace(name)
	if clean == "" {
		clean = fmt.Sprintf("%d", seq)
	}
	id := buffer.NewChatBufferID(clean)
	_ = f.BufferModeState(id)
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnNewSession != nil {
		f.Adaptive.Desktop.OnNewSession()
	}
	f.SelectBuffer(id)
	f.Echo("New LLM Chat buffer: " + string(id))
	return id
}

// CloneCurrentWindow implements authentic Emacs `split-window-right` (`C-x 3`)
// semantics: splits the focused window so the new sibling window opens on the
// SAME buffer (linked viewport onto the current buffer) rather than jumping to
// an unrelated workspace or buffer.
func (f *Frame) CloneCurrentWindow() {
	cur := f.ActiveBuffer()
	if cur == "" {
		cur = buffer.Conversation
	}
	f.SplitVertical(cur)
}
