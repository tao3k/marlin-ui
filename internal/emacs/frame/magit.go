// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/emacs/modes"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func (f *Frame) initMagitStateLocked() {
	if f.magitVisLevel < 1 {
		f.magitVisLevel = 3
	}
	if f.magitFolded == nil {
		f.magitFolded = make(map[string]bool)
	}
	if f.magitAuditMap == nil {
		f.magitAuditMap = make(map[string]string)
	}
	if f.magitSnap.RepoName == "" {
		f.magitSnap = hostlink.DemoMagitSnapshot()
	}
}

func (f *Frame) initMagitLane() {
	f.magitRefreshLane = async.NewFencedLane[struct{}, hostlink.MagitSnapshot](
		func(ctx context.Context, _ struct{}) (hostlink.MagitSnapshot, error) {
			if f.Link == nil {
				return hostlink.DemoMagitSnapshot(), nil
			}
			return f.Link.FetchMagitSnapshot(ctx)
		},
		func(_ uint64, snap hostlink.MagitSnapshot) {
			f.commitMagitSnapshot(snap, "")
		},
	)
	f.magitExecLane = async.NewFencedLane[string, string](
		func(ctx context.Context, wireCmd string) (string, error) {
			if f.Link == nil {
				return "connect marlin-app-server to run " + wireCmd, nil
			}
			sessionID := f.AppCfg.SessionID
			if sessionID == "" {
				sessionID = "default"
			}
			dctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			if err := f.Link.ExecuteMagitCommand(dctx, sessionID, wireCmd); err != nil {
				return err.Error(), nil
			}
			return "", nil
		},
		func(_ uint64, errMsg string) {
			if errMsg != "" {
				f.commitMagitSnapshot(hostlink.MagitSnapshot{}, errMsg)
				f.Echo("Magit error: " + errMsg)
				return
			}
			f.RefreshMagitAsync()
		},
	)
}

func (f *Frame) captureMagitViewLocked() *buffer.MagitViewState {
	f.initMagitStateLocked()
	folded := make(map[string]bool, len(f.magitFolded))
	for k, v := range f.magitFolded {
		if v {
			folded[k] = true
		}
	}
	audit := make(map[string]string, len(f.magitAuditMap))
	for k, v := range f.magitAuditMap {
		audit[k] = v
	}
	rows := buffer.BuildMagitRows(f.magitSnap, f.magitVisLevel, folded)
	cursor := f.magitCursor
	if len(rows) == 0 {
		cursor = 0
	} else if cursor >= len(rows) {
		cursor = len(rows) - 1
	} else if cursor < 0 {
		cursor = 0
	}
	return &buffer.MagitViewState{
		Snapshot:        f.magitSnap,
		Loading:         f.magitLoading,
		Error:           f.magitErr,
		Cursor:          cursor,
		VisibilityLevel: f.magitVisLevel,
		Folded:          folded,
		AuditMap:        audit,
		Rows:            rows,
	}
}

func (f *Frame) commitMagitSnapshot(snap hostlink.MagitSnapshot, errMsg string) {
	f.mu.Lock()
	f.initMagitStateLocked()
	f.magitLoading = false
	f.magitErr = strings.TrimSpace(errMsg)
	if errMsg == "" {
		f.magitSnap = snap
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

// OpenMagit opens the *magit* AI-Native jj + Git buffer (Doom SPC g g) and
// refreshes live repository status off-lock over gRPC.
func (f *Frame) OpenMagit() {
	f.SelectBuffer(buffer.Magit)
	f.RefreshMagitAsync()
}

// RefreshMagitAsync schedules a coalesced, epoch-fenced off-lock refresh of
// *magit* over ProjectionQueryService::QueryProjectGraph("magit:snapshot").
func (f *Frame) RefreshMagitAsync() {
	f.mu.Lock()
	f.initMagitStateLocked()
	f.magitLoading = true
	f.magitErr = ""
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()

	if f.magitRefreshLane != nil {
		f.magitRefreshLane.Submit(struct{}{})
		return
	}
	f.commitMagitSnapshot(hostlink.DemoMagitSnapshot(), "")
}

// dispatchMagitKey handles authentic Emacs Magit single-key commands in Normal
// mode when *magit* is the active buffer (j/k, TAB, 1-4, s/S/u/U/x, v/!, c/d/b/l/r/z/m/?, g, q).
func (f *Frame) dispatchMagitKey(c *ui.Context, scope ui.Element) bool {
	if f.loadSnapshot().ActiveBuffer != buffer.Magit {
		return false
	}
	hit := func(mods ui.Modifiers, key ui.Key) bool {
		if scope != (ui.Element{}) && scope.Shortcut(mods, key) {
			return true
		}
		return c.Shortcut(mods, key)
	}
	switch {
	case hit(0, ui.KeyJ), hit(0, ui.KeyDown):
		f.stepMagitCursor(1)
		return true
	case hit(0, ui.KeyK), hit(0, ui.KeyUp):
		f.stepMagitCursor(-1)
		return true
	case hit(0, ui.KeyN):
		f.jumpMagitSection(1)
		return true
	case hit(0, ui.KeyP):
		f.jumpMagitSection(-1)
		return true
	case hit(ui.Shift, ui.Key6): // ^ jump to parent section (magit-section-up)
		f.jumpMagitParentSection()
		return true
	case hit(0, ui.KeyEnter): // RET visit thing at point (magit-visit-thing)
		f.visitSelectedMagitThing()
		return true
	case hit(0, ui.KeyTab):
		f.toggleMagitSelectedFold()
		return true
	case hit(0, ui.Key1):
		f.setMagitVisibilityLevel(1)
		return true
	case hit(0, ui.Key2):
		f.setMagitVisibilityLevel(2)
		return true
	case hit(0, ui.Key3):
		f.setMagitVisibilityLevel(3)
		return true
	case hit(0, ui.Key4):
		f.setMagitVisibilityLevel(4)
		return true
	case hit(ui.Shift, ui.KeyS):
		f.handleMagitAction("stage-all", "")
		return true
	case hit(0, ui.KeyS):
		f.handleMagitAction("stage", "")
		return true
	case hit(ui.Shift, ui.KeyU):
		f.handleMagitAction("unstage-all", "")
		return true
	case hit(0, ui.KeyU):
		f.handleMagitAction("unstage", "")
		return true
	case hit(ui.Shift, ui.KeyX):
		f.OpenHydra("magit:X")
		return true
	case hit(0, ui.KeyX):
		f.handleMagitAction("discard", "")
		return true
	case hit(ui.Shift, ui.KeyV):
		f.OpenHydra("magit:V")
		return true
	case hit(0, ui.KeyV):
		f.handleMagitAction("audit-toggle", "")
		return true
	case hit(ui.Shift, ui.Key1):
		f.handleMagitAction("audit-flag", "")
		return true
	case hit(ui.Shift, ui.KeyA):
		f.OpenHydra("magit:A")
		return true
	case hit(ui.Shift, ui.KeyF):
		f.OpenHydra("magit:F")
		return true
	case hit(0, ui.KeyF):
		f.OpenHydra("magit:f")
		return true
	case hit(ui.Shift, ui.KeyP):
		f.OpenHydra("magit:P")
		return true
	case hit(0, ui.KeyG):
		f.RefreshMagitAsync()
		f.Echo("Magit & Lambda Aitia refreshed")
		return true
	case hit(ui.Shift, ui.KeySlash), hit(0, ui.KeyH):
		f.OpenHydra("magit:?")
		return true
	case hit(0, ui.KeyC):
		f.OpenHydra("magit:c")
		return true
	case hit(0, ui.KeyD):
		f.OpenHydra("magit:d")
		return true
	case hit(0, ui.KeyB):
		f.OpenHydra("magit:b")
		return true
	case hit(0, ui.KeyL):
		f.OpenHydra("magit:l")
		return true
	case hit(0, ui.KeyR):
		f.OpenHydra("magit:r")
		return true
	case hit(0, ui.KeyZ):
		f.OpenHydra("magit:z")
		return true
	case hit(0, ui.KeyM):
		f.OpenHydra("magit:m")
		return true
	case hit(0, ui.KeyQ):
		f.SelectBuffer(buffer.Conversation)
		return true
	}
	return false
}

func (f *Frame) syncMagitPolymodeSpanLocked(view *buffer.MagitViewState) {
	span := modes.SpanProse
	if view != nil && f.magitCursor >= 0 && f.magitCursor < len(view.Rows) {
		row := view.Rows[f.magitCursor]
		if row.Section == "conflicts" {
			span = modes.SpanConflict
		} else if row.Kind == buffer.MagitRowHunk {
			span = modes.SpanDiffHunk
		}
	}
	reg := f.ensureModeRegistryLocked()
	cur, ok := f.bufModes[buffer.Magit]
	if !ok {
		cur = reg.MakeBufferState(string(buffer.Magit))
	}
	if cur.FocusedSpan != span {
		f.bufModes[buffer.Magit] = reg.SwitchSpan(cur, span)
	}
}

func (f *Frame) stepMagitCursor(delta int) {
	f.mu.Lock()
	view := f.captureMagitViewLocked()
	if len(view.Rows) > 0 {
		next := view.Cursor + delta
		if next < 0 {
			next = 0
		}
		if next >= len(view.Rows) {
			next = len(view.Rows) - 1
		}
		f.magitCursor = next
	}
	f.syncMagitPolymodeSpanLocked(view)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

func (f *Frame) jumpMagitSection(delta int) {
	f.mu.Lock()
	view := f.captureMagitViewLocked()
	if len(view.Rows) > 0 {
		cur := view.Cursor
		for i := cur + delta; i >= 0 && i < len(view.Rows); i += delta {
			if view.Rows[i].Kind == buffer.MagitRowSection || view.Rows[i].Kind == buffer.MagitRowFile {
				f.magitCursor = i
				break
			}
		}
	}
	f.syncMagitPolymodeSpanLocked(view)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

func (f *Frame) jumpMagitParentSection() {
	f.mu.Lock()
	view := f.captureMagitViewLocked()
	if len(view.Rows) > 0 && view.Cursor > 0 && view.Cursor < len(view.Rows) {
		curKind := view.Rows[view.Cursor].Kind
		for i := view.Cursor - 1; i >= 0; i-- {
			if curKind == buffer.MagitRowHunk && view.Rows[i].Kind == buffer.MagitRowFile {
				f.magitCursor = i
				break
			}
			if view.Rows[i].Kind == buffer.MagitRowSection {
				f.magitCursor = i
				break
			}
		}
	}
	f.syncMagitPolymodeSpanLocked(view)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

func (f *Frame) visitSelectedMagitThing() {
	row, _, ok := f.selectedMagitRow()
	if !ok {
		return
	}
	if row.Hunk != nil {
		f.OpenFilePreview(row.Hunk.FilePath+" "+row.Hunk.Header, row.Hunk.FilePath, row.Hunk.Preview, int(row.Hunk.StartLine))
		return
	}
	if row.File != nil {
		f.OpenFilePreview(row.File.Path, row.File.Path, "", 1)
	}
}

func (f *Frame) setMagitVisibilityLevel(level int) {
	if level < 1 {
		level = 1
	}
	if level > 4 {
		level = 4
	}
	f.mu.Lock()
	f.initMagitStateLocked()
	f.magitVisLevel = level
	f.magitFolded = make(map[string]bool)
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.Echo(fmt.Sprintf("Magit visibility level %d", level))
	f.requestFrame()
}

func (f *Frame) toggleMagitSelectedFold() {
	f.mu.Lock()
	view := f.captureMagitViewLocked()
	if len(view.Rows) > 0 && view.Cursor >= 0 && view.Cursor < len(view.Rows) {
		id := view.Rows[view.Cursor].ID
		f.magitFolded[id] = !f.magitFolded[id]
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

func (f *Frame) selectedMagitRow() (buffer.MagitRow, hostlink.MagitSnapshot, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	view := f.captureMagitViewLocked()
	if len(view.Rows) == 0 || view.Cursor < 0 || view.Cursor >= len(view.Rows) {
		return buffer.MagitRow{}, view.Snapshot, false
	}
	return view.Rows[view.Cursor], view.Snapshot, true
}

func (f *Frame) openMagitPrompt(kind, label string) {
	f.mu.Lock()
	f.magitPromptKind = kind
	f.magitPromptLabel = label
	f.mu.Unlock()
	f.OpenMinibuffer(minibuffer.ModeMagitPrompt)
}

func (f *Frame) currentMagitPromptLabel() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.magitPromptLabel
}

func (f *Frame) submitMagitPrompt(text string) {
	f.mu.Lock()
	kind := f.magitPromptKind
	f.magitPromptKind = ""
	f.magitPromptLabel = ""
	f.mu.Unlock()

	msg := strings.TrimSpace(text)
	if msg == "" {
		return
	}
	switch kind {
	case "commit":
		f.execMagitCommandAsync("magit.commit " + msg)
	case "amend":
		f.execMagitCommandAsync("magit.amend " + msg)
	case "describe":
		f.execMagitCommandAsync("magit.describe " + msg)
	case "branch":
		f.execMagitCommandAsync("magit.branch " + msg)
	case "branch-create":
		f.execMagitCommandAsync("magit.branch.create " + msg)
	case "branch-delete":
		f.execMagitCommandAsync("magit.branch.delete " + msg)
	case "cherry-pick":
		f.execMagitCommandAsync("magit.cherry-pick " + msg)
	case "revert":
		f.execMagitCommandAsync("magit.revert " + msg)
	}
}

func (f *Frame) execMagitCommandAsync(wireCmd string) {
	f.Echo("Magit: " + wireCmd)
	if f.magitExecLane != nil {
		f.magitExecLane.Submit(wireCmd)
		return
	}
	f.commitMagitSnapshot(hostlink.MagitSnapshot{}, "connect marlin-app-server to run "+wireCmd)
}

// handleMagitAction dispatches all Magit buffer, Doom SPC g, and Transient actions.
func (f *Frame) handleMagitAction(action, target string) {
	switch action {
	case "refresh":
		f.RefreshMagitAsync()
	case "dispatch":
		f.OpenHydra("magit:?")
	case "select-row":
		if idx, err := strconv.Atoi(strings.TrimSpace(target)); err == nil && idx >= 0 {
			f.mu.Lock()
			f.magitCursor = idx
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		}
	case "toggle-fold":
		if strings.TrimSpace(target) != "" {
			f.mu.Lock()
			f.initMagitStateLocked()
			f.magitFolded[target] = !f.magitFolded[target]
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		}
	case "toggle-oplog":
		f.mu.Lock()
		f.initMagitStateLocked()
		f.magitFolded["sec:oplog"] = !f.magitFolded["sec:oplog"]
		f.publishSnapshotLocked()
		f.mu.Unlock()
		f.requestFrame()
	case "level-3":
		f.setMagitVisibilityLevel(3)
	case "level-4":
		f.setMagitVisibilityLevel(4)
	case "stage-hunk":
		if id := strings.TrimSpace(target); id != "" {
			f.execMagitCommandAsync("magit.stage-hunk " + id)
		}
	case "unstage-hunk":
		if id := strings.TrimSpace(target); id != "" {
			f.execMagitCommandAsync("magit.unstage-hunk " + id)
		}
	case "discard-hunk":
		if id := strings.TrimSpace(target); id != "" {
			f.execMagitCommandAsync("magit.discard-hunk " + id)
		}
	case "stage":
		path := strings.TrimSpace(target)
		if path == "" {
			if row, _, ok := f.selectedMagitRow(); ok {
				if row.Kind == buffer.MagitRowHunk && row.Hunk != nil {
					f.execMagitCommandAsync("magit.stage-hunk " + row.Hunk.HunkID)
					return
				}
				if row.Kind == buffer.MagitRowSection && (row.Section == "unstaged" || row.Section == "untracked") {
					f.execMagitCommandAsync("magit.stage-all")
					return
				}
				path = row.Path
			}
		}
		if path != "" {
			f.execMagitCommandAsync("magit.stage " + path)
		}
	case "stage-all":
		f.execMagitCommandAsync("magit.stage-all")
	case "unstage":
		path := strings.TrimSpace(target)
		if path == "" {
			if row, _, ok := f.selectedMagitRow(); ok {
				if row.Kind == buffer.MagitRowHunk && row.Hunk != nil {
					f.execMagitCommandAsync("magit.unstage-hunk " + row.Hunk.HunkID)
					return
				}
				if row.Kind == buffer.MagitRowSection && row.Section == "staged" {
					f.execMagitCommandAsync("magit.unstage-all")
					return
				}
				path = row.Path
			}
		}
		if path != "" {
			f.execMagitCommandAsync("magit.unstage " + path)
		}
	case "unstage-all":
		f.execMagitCommandAsync("magit.unstage-all")
	case "discard":
		path := strings.TrimSpace(target)
		if path == "" {
			if row, _, ok := f.selectedMagitRow(); ok {
				if row.Kind == buffer.MagitRowHunk && row.Hunk != nil {
					f.execMagitCommandAsync("magit.discard-hunk " + row.Hunk.HunkID)
					return
				}
				path = row.Path
			}
		}
		if path != "" {
			f.execMagitCommandAsync("magit.discard " + path)
		}
	case "extend":
		f.execMagitCommandAsync("magit.extend")
	case "stash-push":
		f.execMagitCommandAsync("magit.stash.push")
	case "stash-pop":
		f.execMagitCommandAsync("magit.stash.pop")
	case "stash-apply":
		f.execMagitCommandAsync("magit.stash.apply")
	case "stash-drop":
		f.execMagitCommandAsync("magit.stash.drop")
	case "fetch-upstream":
		f.execMagitCommandAsync("magit.fetch")
	case "fetch-all":
		f.execMagitCommandAsync("magit.fetch.all")
	case "pull":
		f.execMagitCommandAsync("magit.pull")
	case "pull-rebase":
		f.execMagitCommandAsync("magit.pull.rebase")
	case "push":
		f.execMagitCommandAsync("magit.push")
	case "reset-soft":
		f.execMagitCommandAsync("magit.reset.soft")
	case "reset-mixed":
		f.execMagitCommandAsync("magit.reset.mixed")
	case "commit-prompt":
		f.openMagitPrompt("commit", "Commit message: ")
	case "amend-prompt":
		f.openMagitPrompt("amend", "Amend HEAD message: ")
	case "describe-prompt":
		f.openMagitPrompt("describe", "jj describe (@): ")
	case "branch-prompt":
		f.openMagitPrompt("branch", "Checkout branch: ")
	case "branch-create-prompt":
		f.openMagitPrompt("branch-create", "Create & checkout branch: ")
	case "branch-delete-prompt":
		f.openMagitPrompt("branch-delete", "Delete branch: ")
	case "cherry-pick-prompt":
		f.openMagitPrompt("cherry-pick", "Cherry-pick revision: ")
	case "revert-prompt":
		f.openMagitPrompt("revert", "Revert revision: ")
	case "jj-new":
		f.execMagitCommandAsync("magit.jj.new")
	case "jj-squash":
		f.execMagitCommandAsync("magit.jj.squash")
	case "jj-undo":
		f.execMagitCommandAsync("magit.jj.undo")
	case "audit-toggle":
		f.markMagitAudit(target, "audited")
	case "audit-flag":
		f.markMagitAudit(target, "flagged")
	case "ai-audit":
		f.launchMagitAIAuditSplit(target)
	case "ai-triage":
		f.launchMagitAITriageSplit()
	case "ai-commit":
		f.launchMagitAICommitSplit()
	case "ai-split":
		f.launchMagitAISplitProposal()
	case "ai-resolve":
		f.launchMagitAIConflictResolve(target)
	}
}

func (f *Frame) markMagitAudit(hunkID, state string) {
	f.mu.Lock()
	f.initMagitStateLocked()
	ids := []string{}
	if strings.TrimSpace(hunkID) != "" {
		ids = append(ids, hunkID)
	} else {
		view := f.captureMagitViewLocked()
		if len(view.Rows) > 0 && view.Cursor >= 0 && view.Cursor < len(view.Rows) {
			row := view.Rows[view.Cursor]
			if row.Hunk != nil {
				ids = append(ids, row.Hunk.HunkID)
			} else if row.File != nil {
				for _, h := range row.File.Hunks {
					ids = append(ids, h.HunkID)
				}
			}
		}
	}
	var recorded []struct{ id, verdict string }
	for _, id := range ids {
		next := state
		if f.magitAuditMap[id] == state {
			next = "unreviewed"
		}
		f.magitAuditMap[id] = next
		recorded = append(recorded, struct{ id, verdict string }{id: id, verdict: next})
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	if f.Link != nil && f.Link.Switcher != nil && f.Link.Switcher.CurrentTarget() != nil {
		for _, rec := range recorded {
			f.execMagitCommandAsync(fmt.Sprintf("magit.audit.record %s|human|%s|Human Magit review", rec.id, rec.verdict))
		}
	}
	if len(ids) > 0 {
		f.Echo(fmt.Sprintf("Marked %d hunk(s) -> %s (evidence chain updated)", len(ids), state))
	}
	f.requestFrame()
}

// openMagitSplitConversation ensures a 50/50 vertical Emacs split (*magit* | *conversation*)
// and sends an Aitia-grounded prompt to Marlin's live LLM without hiding the diff.
func (f *Frame) openMagitSplitConversation(prompt, echoMsg string) {
	f.SelectBuffer(buffer.Magit)
	if f.wsMgr != nil {
		wsSnap := f.wsMgr.Snapshot()
		if wsSnap.Active.WindowCount() < 2 {
			f.SplitVertical(buffer.Conversation)
		} else {
			f.CycleFocusPane()
			f.SelectBuffer(buffer.Conversation)
		}
	} else {
		f.SelectBuffer(buffer.Conversation)
	}
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSendPrompt != nil {
		f.Adaptive.Desktop.OnSendPrompt(prompt)
	}
	f.Echo(echoMsg)
}

func (f *Frame) launchMagitAIAuditSplit(targetHunkID string) {
	row, snap, ok := f.selectedMagitRow()
	var hunk *hostlink.MagitHunk
	if strings.TrimSpace(targetHunkID) != "" {
		hunk = findHunkByID(snap, targetHunkID)
	}
	if hunk == nil && ok {
		if row.Hunk != nil {
			hunk = row.Hunk
		} else if row.File != nil && len(row.File.Hunks) > 0 {
			hCopy := row.File.Hunks[0]
			hunk = &hCopy
		}
	}
	if hunk == nil {
		hunk = firstHighestRiskHunk(snap)
	}
	if hunk == nil {
		f.Echo("No diff hunks available to audit")
		return
	}
	ref := formatMagitRef(snap)
	prompt := fmt.Sprintf(
		"[Magit AI Code Audit | Aitia: %s]\nRepository: %s (%s · %s)\nFile: %s (%s)\nTriage: risk=%s concern=%s — %s\n\n```diff\n%s\n```\n\nAudit concisely for a human reviewer:\n1. Intent & behavioral delta (1-2 sentences)\n2. Security / invariant / concurrency risks\n3. Concrete verification or fix suggestion (if needed)",
		snap.AitiaEvidenceID, snap.RepoName, snap.Branch, ref,
		hunk.FilePath, hunk.Header, hunk.Risk, hunk.Concern, hunk.RiskReason, hunk.Preview,
	)
	f.openMagitSplitConversation(prompt, "AI Code Audit opened in 50/50 split -> "+hunk.FilePath)
}

func (f *Frame) launchMagitAITriageSplit() {
	snap := f.loadSnapshot().Magit.Snapshot
	var b strings.Builder
	fmt.Fprintf(&b, "[Magit Full Risk Triage | Aitia: %s]\nRepo: %s (%s · %s)\n", snap.AitiaEvidenceID, snap.RepoName, snap.Branch, formatMagitRef(snap))
	for _, sec := range [][]hostlink.MagitFile{snap.Conflicts, snap.Unstaged, snap.Staged} {
		for _, mf := range sec {
			fmt.Fprintf(&b, "- [%s/%s] %s (+%d/-%d): %s\n", strings.ToUpper(mf.Risk), mf.Concern, mf.Path, mf.Added, mf.Removed, mf.RiskReason)
		}
	}
	b.WriteString("\nGroup these changes by cognitive-load audit priority for a human reviewer, highlighting what must be read line-by-line vs. what can be skim-approved.")
	f.openMagitSplitConversation(b.String(), "AI Full Risk Triage dispatched in 50/50 split")
}

func (f *Frame) launchMagitAICommitSplit() {
	snap := f.loadSnapshot().Magit.Snapshot
	var b strings.Builder
	fmt.Fprintf(&b, "[Magit AI Commit Draft | Aitia: %s]\nBranch: %s | Backend: %s | Ref: %s\nChanged files:\n",
		snap.AitiaEvidenceID, snap.Branch, snap.Backend, formatMagitRef(snap))
	for _, sec := range [][]hostlink.MagitFile{snap.Staged, snap.Unstaged} {
		for _, mf := range sec {
			fmt.Fprintf(&b, "- (%s) %s [%s/%s] +%d/-%d: %s\n", mf.Section, mf.Path, mf.Risk, mf.Concern, mf.Added, mf.Removed, mf.RiskReason)
		}
	}
	fmt.Fprintf(&b, "\nDraft a concise Conventional Commit message (subject <= 72 chars + 2-4 bullet body) and include an `Aitia-Evidence: %s` trailer.", snap.AitiaEvidenceID)
	f.openMagitSplitConversation(b.String(), "AI Commit Draft dispatched with Aitia trailer")
}

func (f *Frame) launchMagitAISplitProposal() {
	snap := f.loadSnapshot().Magit.Snapshot
	var b strings.Builder
	fmt.Fprintf(&b, "[Magit AI Semantic Split Proposal | Aitia: %s]\nRepo: %s (%s)\nChanges:\n", snap.AitiaEvidenceID, snap.RepoName, formatMagitRef(snap))
	for _, sec := range [][]hostlink.MagitFile{snap.Unstaged, snap.Staged} {
		for _, mf := range sec {
			fmt.Fprintf(&b, "- %s [%s/%s] +%d/-%d\n", mf.Path, mf.Risk, mf.Concern, mf.Added, mf.Removed)
		}
	}
	b.WriteString("\nPropose clean atomic commits (or `jj split` slices) grouped by concern so each commit is small, reviewable, and bisect-safe.")
	f.openMagitSplitConversation(b.String(), "AI Semantic Split Proposal opened in 50/50 split")
}

func (f *Frame) launchMagitAIConflictResolve(targetPath string) {
	snap := f.loadSnapshot().Magit.Snapshot
	path := strings.TrimSpace(targetPath)
	if path == "" && len(snap.Conflicts) > 0 {
		path = snap.Conflicts[0].Path
	}
	prompt := fmt.Sprintf(
		"[Magit AI 3-Way Conflict Resolution | Aitia: %s]\nRepo: %s (%s · %s)\nConflicted file: %s\nExplain both sides of the conflict and propose a safe, invariant-preserving resolution.",
		snap.AitiaEvidenceID, snap.RepoName, snap.Branch, formatMagitRef(snap), path,
	)
	f.openMagitSplitConversation(prompt, "AI 3-Way Conflict Resolution dispatched")
}

func formatMagitRef(snap hostlink.MagitSnapshot) string {
	if snap.ChangeID != "" && snap.HeadSHA != "" {
		return "jj@" + snap.ChangeID + " / git:" + snap.HeadSHA
	}
	if snap.ChangeID != "" {
		return "jj@" + snap.ChangeID
	}
	if snap.HeadSHA != "" {
		return "git:" + snap.HeadSHA
	}
	return "unborn"
}

func findHunkByID(snap hostlink.MagitSnapshot, id string) *hostlink.MagitHunk {
	for _, sec := range [][]hostlink.MagitFile{snap.Conflicts, snap.Unstaged, snap.Staged} {
		for _, mf := range sec {
			for i := range mf.Hunks {
				if mf.Hunks[i].HunkID == id {
					h := mf.Hunks[i]
					return &h
				}
			}
		}
	}
	return nil
}

func firstHighestRiskHunk(snap hostlink.MagitSnapshot) *hostlink.MagitHunk {
	for _, sec := range [][]hostlink.MagitFile{snap.Conflicts, snap.Unstaged, snap.Staged} {
		for _, mf := range sec {
			if len(mf.Hunks) > 0 {
				h := mf.Hunks[0]
				return &h
			}
		}
	}
	return nil
}
