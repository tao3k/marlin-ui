// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
)

func (f *Frame) ensureModeRegistryLocked() *modes.Registry {
	if f.modeReg == nil {
		f.modeReg = modes.DefaultRegistry()
	}
	if f.bufModes == nil {
		f.bufModes = make(map[buffer.ID]modes.BufferState, 16)
	}
	if f.ibufferMarked == nil {
		f.ibufferMarked = make(map[buffer.ID]bool, 8)
	}
	if !f.modeHooksInstalled {
		f.modeHooksInstalled = true
		f.installDefaultModeHooksLocked(f.modeReg)
	}
	return f.modeReg
}

// installDefaultModeHooksLocked wires authentic Emacs lifecycle hooks onto
// the Frame's mode registry so minor-mode toggles drive live runtime effects
// (e.g. dedicated-window-mode pin sync) without ever re-acquiring f.mu.
func (f *Frame) installDefaultModeHooksLocked(reg *modes.Registry) {
	reg.AddHook(modes.HookEnterMinorMode, func(ev modes.HookEvent) {
		if ev.MinorMode == "dedicated-window-mode" && f.wsMgr != nil {
			f.wsMgr.SetWindowDedicated(true)
		}
	})
	reg.AddHook(modes.HookExitMinorMode, func(ev modes.HookEvent) {
		if ev.MinorMode == "dedicated-window-mode" && f.wsMgr != nil {
			f.wsMgr.SetWindowDedicated(false)
		}
	})
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
	if next.InnerMode != cur.InnerMode {
		f.logMessage(fmt.Sprintf("hook[polymode-switch-span]: %s span=%s (%s -> %s)", id, span, cur.InnerMode, next.InnerMode))
	}
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
	f.logMessage(fmt.Sprintf("hook[minor-mode]: toggled %s on %s", minorID, id))
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return next
}

// ToggleMinorModeOnActive toggles minorID on the currently focused buffer,
// runs registered enter/exit hooks, and echoes the new state in the Echo Area.
func (f *Frame) ToggleMinorModeOnActive(minorID string) bool {
	buf := f.ActiveBuffer()
	st := f.ToggleBufferMinorMode(buf, minorID)
	enabled := false
	for _, m := range st.MinorModes {
		if m == minorID {
			enabled = true
			break
		}
	}
	if enabled {
		f.Echo(fmt.Sprintf("%s enabled in %s", minorID, buf))
	} else {
		f.Echo(fmt.Sprintf("%s disabled in %s", minorID, buf))
	}
	return enabled
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

// OpenDescribeKey opens the Emacs `C-h k` (`describe-key`) Minibuffer prompt.
func (f *Frame) OpenDescribeKey() {
	f.OpenMinibuffer(minibuffer.ModeDescribeKey)
}

// OpenDescribeFunction opens the Emacs `C-h f` (`describe-function`) Vertico picker.
func (f *Frame) OpenDescribeFunction() {
	f.OpenMinibuffer(minibuffer.ModeDescribeFunc)
}

// DescribeKey implements Emacs `C-h k` (`describe-key`): resolves chord against
// the active buffer's exact Emacs precedence hierarchy
// (`MINOR` -> `POLY` -> `MAJOR` -> `PARENT` -> `GLOBAL` / Doom/Emacs Hydras)
// and displays a structured documentation card in *Help*.
func (f *Frame) DescribeKey(chord string) {
	query := strings.TrimSpace(chord)
	if query == "" {
		return
	}
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	doc := &buffer.HelpKeyDoc{
		Heading: "C-h k · describe-key",
		Subject: query,
		Tier:    "UNBOUND",
		Command: "(unbound)",
	}
	// 1. Check active buffer's resolved mode-local stack first (MINOR -> POLY -> MAJOR -> PARENT).
	for _, lb := range data.LocalBindings {
		if strings.EqualFold(lb[1], query) {
			doc.Tier = lb[0]
			doc.Description = lb[2]
			doc.Command = lb[2]
			if idx := strings.LastIndex(lb[2], "("); idx >= 0 && strings.HasSuffix(lb[2], ")") {
				doc.Command = strings.TrimSpace(lb[2][idx+1 : len(lb[2])-1])
			}
			doc.Aliases = []string{lb[1] + " (buffer-local in " + string(target) + ")"}
			break
		}
	}
	// 2. Fall through to live Global / Doom SPC / Emacs C-x / C-c / C-h Hydras.
	if doc.Tier == "UNBOUND" {
		f.mu.Lock()
		hydras := f.tomlHydras
		f.mu.Unlock()
		var matchedAct string
		for pref, def := range hydras {
			for _, hd := range def.Heads {
				full := strings.TrimSpace(pref + " " + hd.Key)
				if strings.EqualFold(full, query) {
					doc.Tier = "GLOBAL"
					doc.Command = hd.Action
					doc.Description = hd.Label
					matchedAct = hd.Action
					break
				}
			}
			if matchedAct != "" {
				break
			}
		}
		if matchedAct != "" {
			for pref, def := range hydras {
				for _, hd := range def.Heads {
					if hd.Action == matchedAct {
						doc.Aliases = append(doc.Aliases, strings.TrimSpace(pref+" "+hd.Key))
					}
				}
			}
			sort.Strings(doc.Aliases)
		} else if strings.EqualFold(query, "M-x") {
			doc.Tier = "GLOBAL"
			doc.Command = "cmd:mx"
			doc.Description = "Execute extended command (Vertico + Orderless)"
			doc.Aliases = []string{"M-x", "SPC SPC", "SPC x"}
		} else if strings.EqualFold(query, "C-s") {
			doc.Tier = "GLOBAL"
			doc.Command = "cmd:helm"
			doc.Description = "Open 120Hz Helm live multi-source search"
			doc.Aliases = []string{"C-s", "SPC s"}
		}
	}
	data.KeyDoc = doc
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("describe-key: %s -> [%s] %s", query, doc.Tier, doc.Command))
}

// DescribeFunction implements Emacs `C-h f` (`describe-function`): looks up
// command metadata and all local/global chords bound to cmdName and opens *Help*.
func (f *Frame) DescribeFunction(cmdName string) {
	name := strings.TrimSpace(cmdName)
	if name == "" {
		return
	}
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	doc := &buffer.HelpKeyDoc{
		Heading: "C-h f · describe-function",
		Subject: name,
		Tier:    "COMMAND",
		Command: name,
	}
	for _, cand := range f.commandCandidates("") {
		if strings.EqualFold(cand.Name, name) {
			doc.Description = cand.Desc
			if cand.Annotation != "" {
				doc.Aliases = append(doc.Aliases, cand.Annotation)
			}
			break
		}
	}
	if mm, ok := f.ModeRegistry().MinorModeFor(name); ok {
		doc.Tier = "MINOR-MODE"
		doc.Description = fmt.Sprintf("Toggleable Emacs Minor Mode %s (lighter: %q, priority: %d)", mm.Title, mm.Lighter, mm.Priority)
	}
	data.KeyDoc = doc
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("describe-function: %s", name))
}

// DescribeBindings implements Emacs `C-h b` (`describe-bindings`): snapshots
// every active local binding plus the complete live Doom/Emacs global keymap
// table into *Help*.
func (f *Frame) DescribeBindings() {
	target := f.ActiveBuffer()
	if target == "" || target == buffer.Help {
		target = buffer.Conversation
	}
	data := f.BuildHelpIntrospection(target)
	f.mu.Lock()
	hydras := f.tomlHydras
	f.mu.Unlock()
	var globals [][2]string
	for pref, def := range hydras {
		for _, hd := range def.Heads {
			if hd.Action == "" || hd.IsPrefix {
				continue
			}
			chord := strings.TrimSpace(pref + " " + hd.Key)
			desc := hd.Label
			if desc == "" {
				desc = hd.Action
			} else {
				desc = desc + "  (" + hd.Action + ")"
			}
			globals = append(globals, [2]string{chord, desc})
		}
	}
	sort.Slice(globals, func(i, j int) bool { return globals[i][0] < globals[j][0] })
	data.GlobalBindings = globals
	data.KeyDoc = &buffer.HelpKeyDoc{
		Heading:     "C-h b · describe-bindings",
		Subject:     string(target),
		Tier:        "ALL-KEYMAPS",
		Command:     fmt.Sprintf("%d local · %d global bindings", len(data.LocalBindings), len(globals)),
		Description: "Precedence: MINOR -> POLYMODE INNER -> MAJOR -> PARENT -> GLOBAL",
	}
	f.mu.Lock()
	f.helpData = data
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.SelectBuffer(buffer.Help)
	f.Echo(fmt.Sprintf("describe-bindings: %d local, %d global", len(data.LocalBindings), len(globals)))
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
	f.mu.Lock()
	marked := make(map[buffer.ID]bool, len(f.ibufferMarked))
	for k, v := range f.ibufferMarked {
		if v {
			marked[k] = true
		}
	}
	f.mu.Unlock()
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
			Marked:    marked[bid],
		})
	}
	return out
}

// ToggleIbufferMark marks or unmarks buf for batch deletion (`d` in *Ibuffer*).
func (f *Frame) ToggleIbufferMark(id buffer.ID) bool {
	f.mu.Lock()
	if f.ibufferMarked == nil {
		f.ibufferMarked = make(map[buffer.ID]bool, 8)
	}
	next := !f.ibufferMarked[id]
	if next {
		f.ibufferMarked[id] = true
	} else {
		delete(f.ibufferMarked, id)
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return next
}

// UnmarkAllIbuffer clears all [D] marks in *Ibuffer* (`U`).
func (f *Frame) UnmarkAllIbuffer() {
	f.mu.Lock()
	clear(f.ibufferMarked)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	f.Echo("Ibuffer: cleared all marks")
}

// KillMarkedIbuffer kills every buffer marked with [D] in *Ibuffer* (`x`),
// preserving at least one remaining buffer in the workspace.
func (f *Frame) KillMarkedIbuffer() int {
	f.mu.Lock()
	targets := make([]buffer.ID, 0, len(f.ibufferMarked))
	for bid, on := range f.ibufferMarked {
		if on {
			targets = append(targets, bid)
		}
	}
	clear(f.ibufferMarked)
	f.mu.Unlock()
	killed := 0
	for _, bid := range targets {
		if f.KillBuffer(bid) {
			killed++
		}
	}
	if killed > 0 {
		f.Echo(fmt.Sprintf("Ibuffer: killed %d marked buffer(s)", killed))
	}
	return killed
}

// SetIbufferFilter filters *Ibuffer* to a single Major Mode group ("" = All).
func (f *Frame) SetIbufferFilter(majorMode string) {
	f.mu.Lock()
	f.ibufferFilter = strings.TrimSpace(majorMode)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

// IbufferFilter returns the active *Ibuffer* Major Mode filter ("" = All).
func (f *Frame) IbufferFilter() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ibufferFilter
}

// IbufferMarkedCount returns the number of currently marked buffers in *Ibuffer*.
func (f *Frame) IbufferMarkedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, on := range f.ibufferMarked {
		if on {
			n++
		}
	}
	return n
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
	case "cmd:describe-key":
		f.OpenDescribeKey()
	case "cmd:describe-function":
		f.OpenDescribeFunction()
	case "cmd:describe-bindings":
		f.DescribeBindings()
	case "cmd:toggle-dedicated-mode":
		f.ToggleMinorModeOnActive("dedicated-window-mode")
	case "cmd:toggle-aitia-mode":
		f.ToggleMinorModeOnActive("aitia-audit-minor-mode")
	case "cmd:toggle-readonly-mode":
		f.ToggleMinorModeOnActive("read-only-mode")
	case "cmd:toggle-llm-assist-mode":
		f.ToggleMinorModeOnActive("llm-inline-assist-mode")
	case "cmd:ibuffer":
		f.OpenIbuffer()
	case "cmd:ibuffer-unmark-all":
		f.UnmarkAllIbuffer()
	case "cmd:ibuffer-kill-marked":
		f.KillMarkedIbuffer()
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
