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
// binds its own independent SessionID, and opens it in the active window.
func (f *Frame) NewChatBuffer(name string) buffer.ID {
	f.mu.Lock()
	f.chatSeq++
	seq := f.chatSeq
	if f.chatSessions == nil {
		f.chatSessions = make(map[buffer.ID]string, 8)
	}
	f.mu.Unlock()

	clean := strings.TrimSpace(name)
	if clean == "" {
		clean = fmt.Sprintf("%d", seq)
	}
	id := buffer.NewChatBufferID(clean)
	_ = f.BufferModeState(id)
	sessID := "chat-" + strings.ToLower(strings.ReplaceAll(clean, " ", "-"))
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnNewSession != nil {
		f.Adaptive.Desktop.OnNewSession()
		if cur := strings.TrimSpace(f.Adaptive.Desktop.ActiveSessionID); cur != "" {
			sessID = cur
		}
	}
	f.mu.Lock()
	f.chatSessions[id] = sessID
	f.mu.Unlock()
	f.SelectBuffer(id)
	f.Echo(fmt.Sprintf("New LLM Chat buffer: %s (session: %s)", id, sessID))
	return id
}

// ChatSessionFor returns the backend session ID bound to chat buffer id.
func (f *Frame) ChatSessionFor(id buffer.ID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chatSessions != nil {
		if s := f.chatSessions[id]; s != "" {
			return s
		}
	}
	return f.AppCfg.SessionID
}

func (f *Frame) syncChatSessionForBuffer(id buffer.ID) {
	if !buffer.IsChatBuffer(id) {
		return
	}
	f.mu.Lock()
	if f.chatSessions == nil {
		f.chatSessions = make(map[buffer.ID]string, 8)
	}
	sessID, ok := f.chatSessions[id]
	if !ok {
		if id == buffer.Conversation {
			sessID = f.AppCfg.SessionID
			if sessID == "" && f.Adaptive != nil && f.Adaptive.Desktop != nil {
				sessID = f.Adaptive.Desktop.ActiveSessionID
			}
		} else {
			sessID = "chat-" + strings.Trim(strings.TrimPrefix(string(id), "*chat:"), "*")
		}
		if sessID != "" {
			f.chatSessions[id] = sessID
		}
	}
	f.mu.Unlock()
	if sessID != "" && f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSelectSession != nil {
		if f.Adaptive.Desktop.ActiveSessionID != sessID {
			f.Adaptive.Desktop.OnSelectSession(sessID)
		}
	}
}

// HandlePolymodeSpanFocus activates the matching Polymode Inner Mode on the
// currently active buffer (e.g. clicking a code fence or diff hunk inside *chat:*).
func (f *Frame) HandlePolymodeSpanFocus(spanKind, lang, payload string) {
	buf := f.ActiveBuffer()
	span := modes.SpanKind(strings.TrimSpace(spanKind))
	if span == "" {
		span = modes.SpanProse
	}
	f.mu.Lock()
	f.activeSpanLang = strings.TrimSpace(lang)
	f.activeSpanCode = payload
	f.mu.Unlock()
	st := f.FocusPolymodeSpan(buf, span)
	eff := f.ModeRegistry().EffectiveStack(st)
	f.Echo(fmt.Sprintf("Polymode: %s (span: %s)", eff.DisplayTitle, span))
}

// BuildHelpIntrospection reflects the live Major Mode, parent chain, Minor
// Modes, Polymode rules, and merged local keymap for buffer id (`C-h m`).
func (f *Frame) BuildHelpIntrospection(id buffer.ID) *buffer.HelpIntrospectionData {
	reg := f.ModeRegistry()
	st := f.BufferModeState(id)
	eff := reg.EffectiveStack(st)
	out := &buffer.HelpIntrospectionData{
		BufferName:    string(id),
		MajorModeID:   eff.HostMode,
		MajorTitle:    eff.DisplayTitle,
		ParentChain:   eff.ParentChain,
		MinorLighters: eff.MinorLighters,
		FocusedSpan:   string(st.FocusedSpan),
		InnerModeID:   eff.InnerMode,
		DisplayAction: string(eff.DisplayAction),
		ReadOnly:      eff.ReadOnly,
	}
	if poly, ok := reg.PolymodeFor(eff.HostMode); ok {
		for _, r := range poly.InnerRules {
			out.PolymodeRules = append(out.PolymodeRules, [3]string{string(r.SpanKind), r.InnerMode, r.Lighter})
		}
	}
	seenKeys := make(map[string]bool)
	addBinding := func(tier string, b modes.ModeBinding) {
		if seenKeys[b.Key] {
			return
		}
		seenKeys[b.Key] = true
		desc := b.Command
		if b.Label != "" {
			desc = b.Label + "  (" + b.Command + ")"
		}
		out.LocalBindings = append(out.LocalBindings, [3]string{tier, b.Key, desc})
	}
	for _, mid := range eff.MinorModes {
		if m, ok := reg.MinorModeFor(mid); ok {
			for _, b := range m.LocalKeymap {
				addBinding("MINOR", b)
			}
		}
	}
	if eff.InnerMode != "" {
		if im, ok := reg.InnerModeFor(eff.InnerMode); ok {
			for _, b := range im.LocalKeymap {
				addBinding("POLY", b)
			}
		}
	}
	for idx, pid := range eff.ParentChain {
		if m, ok := reg.MajorModeFor(pid); ok {
			tier := "MAJOR"
			if idx > 0 {
				tier = "PARENT"
			}
			for _, b := range m.LocalKeymap {
				addBinding(tier, b)
			}
		}
	}
	return out
}

func (f *Frame) currentHelpIntrospection(activeBuf buffer.ID) *buffer.HelpIntrospectionData {
	f.mu.Lock()
	data := f.helpData
	f.mu.Unlock()
	if data != nil {
		return data
	}
	target := activeBuf
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	return f.BuildHelpIntrospection(target)
}

// DescribeCurrentMode implements Emacs `describe-mode` (`C-h m` / `SPC h m`):
// snapshots the active buffer's live Major/Minor/Polymode introspection BEFORE
// opening *Help* so *Help* describes the buffer the user was actually in.
func (f *Frame) DescribeCurrentMode() {
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("describe-mode: %s (%s)", target, data.MajorTitle))
}

// BuildIbufferEntries builds the live Major-Mode-grouped buffer rows for *Ibuffer*.
func (f *Frame) BuildIbufferEntries(snap frameSnapshot) []buffer.IbufferEntry {
	wsName := snap.Workspaces.Active.Name
	if wsName == "" {
		wsName = "main"
	}
	winNum := make(map[buffer.ID]int)
	for _, lf := range snap.Workspaces.Active.Windows() {
		if _, ok := winNum[lf.Buffer]; !ok {
			winNum[lf.Buffer] = lf.Number
		}
	}
	bufs := snap.Workspaces.Active.Buffers
	if len(bufs) == 0 {
		bufs = snap.OpenTabs
	}
	out := make([]buffer.IbufferEntry, 0, len(bufs))
	for _, bid := range bufs {
		eff := f.EffectiveModeStack(bid)
		innerLbl := ""
		if eff.InnerMode != "" {
			if im, ok := f.ModeRegistry().InnerModeFor(eff.InnerMode); ok {
				innerLbl = im.Title
			} else {
				innerLbl = eff.InnerMode
			}
		}
		majorTitle := eff.HostMode
		if mm, ok := f.ModeRegistry().MajorModeFor(eff.HostMode); ok && mm.Title != "" {
			majorTitle = mm.Title
		}
		out = append(out, buffer.IbufferEntry{
			BufferID:  bid,
			MajorMode: majorTitle,
			InnerMode: innerLbl,
			Workspace: wsName,
			WindowNum: winNum[bid],
			ReadOnly:  eff.ReadOnly,
			Active:    bid == snap.ActiveBuffer,
		})
	}
	return out
}

// OpenIbuffer opens Emacs `*Ibuffer*` grouped by Major Mode (`C-c b` / `SPC b i`).
func (f *Frame) OpenIbuffer() {
	f.SelectBuffer(buffer.Ibuffer)
}

// KillActiveBuffer kills the focused buffer in the active workspace (`C-x k` / `SPC b d`).
func (f *Frame) KillActiveBuffer() bool {
	return f.KillBuffer(f.ActiveBuffer())
}

// KillBuffer kills buf in the active workspace and switches any window showing it
// to the nearest remaining buffer.
func (f *Frame) KillBuffer(id buffer.ID) bool {
	if f.wsMgr == nil {
		return false
	}
	killed, fallback, wsSnap := f.wsMgr.KillBuffer(id)
	if !killed {
		f.Echo("Cannot kill last remaining buffer in workspace")
		return false
	}
	f.mu.Lock()
	delete(f.bufModes, id)
	delete(f.chatSessions, id)
	f.activeBuffer = wsSnap.Active.FocusedBuffer()
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.persistWorkspaces()
	f.requestFrame()
	f.Echo(fmt.Sprintf("Killed %s -> %s", id, fallback))
	return true
}

// StepBuffer cycles the focused window to the previous (-1) or next (+1) buffer
// in the active workspace (`SPC b [` / `SPC b ]` / `C-c [` / `C-c ]`).
func (f *Frame) StepBuffer(delta int) bool {
	if f.wsMgr == nil {
		return false
	}
	nextBuf, ok, wsSnap := f.wsMgr.StepBuffer(delta)
	if !ok {
		return false
	}
	f.syncChatSessionForBuffer(nextBuf)
	f.mu.Lock()
	f.activeBuffer = nextBuf
	f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	f.Echo("Buffer: " + string(nextBuf))
	return true
}

// dispatchModeAction executes Major/Minor/Polymode & buffer-lifecycle commands
// (kept outside dashboard_sync.go to maintain clean modular file bounds).
func (f *Frame) dispatchModeAction(act string) bool {
	switch act {
	case "cmd:describe-mode":
		f.DescribeCurrentMode()
	case "cmd:ibuffer":
		f.OpenIbuffer()
	case "cmd:kill-buffer":
		f.KillActiveBuffer()
	case "cmd:prev-buffer":
		f.StepBuffer(-1)
	case "cmd:next-buffer":
		f.StepBuffer(1)
	case "cmd:span-copy-code":
		f.mu.Lock()
		code := f.activeSpanCode
		f.mu.Unlock()
		if code != "" && f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.Transcript != nil && f.Adaptive.Desktop.Transcript.OnCopyCode != nil {
			f.Adaptive.Desktop.Transcript.OnCopyCode(code)
			f.Echo("Copied active Polymode span")
		} else {
			f.Echo("No active Polymode code span focused")
		}
	case "cmd:span-open-preview":
		f.mu.Lock()
		code, lang := f.activeSpanCode, f.activeSpanLang
		f.mu.Unlock()
		if code != "" {
			if lang == "" {
				lang = "txt"
			}
			f.OpenFilePreview("Polymode Span ("+lang+")", "span."+lang, code, 1)
		}
	case "cmd:span-ai-explain":
		f.mu.Lock()
		code, lang := f.activeSpanCode, f.activeSpanLang
		f.mu.Unlock()
		if code != "" {
			f.submitPrompt(fmt.Sprintf("Explain and audit this %s snippet concisely:\n```%s\n%s\n```", lang, lang, code))
		}
	default:
		return false
	}
	return true
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
