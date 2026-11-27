// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

// MagitRowKind identifies one navigable row inside the *magit* status tree.
type MagitRowKind uint8

const (
	MagitRowSection MagitRowKind = iota
	MagitRowFile
	MagitRowHunk
	MagitRowStash
	MagitRowLog
	MagitRowOpLog
)

// MagitRow is one flat navigable item in *magit* so j/k, TAB, s/u/x, v/!, and
// AI Audit (d a) address the exact highlighted section, file, hunk, or commit.
type MagitRow struct {
	ID      string
	Kind    MagitRowKind
	Section string // conflicts | unstaged | staged | untracked | log | oplog
	Path    string
	Hunk    *hostlink.MagitHunk
	File    *hostlink.MagitFile
}

// MagitViewState carries an immutable RCU snapshot of *magit* buffer state for
// wait-free 120Hz GPU rendering.
type MagitViewState struct {
	Snapshot        hostlink.MagitSnapshot
	Loading         bool
	Error           string
	Cursor          int
	VisibilityLevel int // 1=sections, 2=files, 3=hunks, 4=full diff lines
	Folded          map[string]bool
	AuditMap        map[string]string // hunk_id -> "unreviewed" | "audited" | "flagged"
	Rows            []MagitRow
}

// BuildMagitRows constructs the deterministic navigable row list from snap,
// visibilityLevel, and folded map.
func BuildMagitRows(snap hostlink.MagitSnapshot, visibilityLevel int, folded map[string]bool) []MagitRow {
	if visibilityLevel < 1 {
		visibilityLevel = 3
	}
	var rows []MagitRow
	appendFileSection := func(secID string, files []hostlink.MagitFile) {
		if len(files) == 0 {
			return
		}
		rows = append(rows, MagitRow{ID: "sec:" + secID, Kind: MagitRowSection, Section: secID})
		if visibilityLevel < 2 || folded["sec:"+secID] {
			return
		}
		for i := range files {
			f := files[i]
			fid := fmt.Sprintf("file:%s:%s", secID, f.Path)
			fCopy := f
			rows = append(rows, MagitRow{ID: fid, Kind: MagitRowFile, Section: secID, Path: f.Path, File: &fCopy})
			if visibilityLevel < 3 || folded[fid] {
				continue
			}
			for j := range f.Hunks {
				h := f.Hunks[j]
				hCopy := h
				rows = append(rows, MagitRow{
					ID:      "hunk:" + h.HunkID,
					Kind:    MagitRowHunk,
					Section: secID,
					Path:    f.Path,
					File:    &fCopy,
					Hunk:    &hCopy,
				})
			}
		}
	}

	appendFileSection("conflicts", snap.Conflicts)
	appendFileSection("unstaged", snap.Unstaged)
	appendFileSection("staged", snap.Staged)
	appendFileSection("untracked", snap.Untracked)

	if len(snap.Stashes) > 0 {
		rows = append(rows, MagitRow{ID: "sec:stashes", Kind: MagitRowSection, Section: "stashes"})
		if visibilityLevel >= 2 && !folded["sec:stashes"] {
			for i, st := range snap.Stashes {
				rows = append(rows, MagitRow{
					ID:      fmt.Sprintf("stash:%d:%s", i, st.Reference),
					Kind:    MagitRowStash,
					Section: "stashes",
					Path:    st.Reference,
				})
			}
		}
	}
	if len(snap.Unpushed) > 0 {
		rows = append(rows, MagitRow{ID: "sec:unpushed", Kind: MagitRowSection, Section: "unpushed"})
		if visibilityLevel >= 2 && !folded["sec:unpushed"] {
			for i, entry := range snap.Unpushed {
				rows = append(rows, MagitRow{
					ID:      fmt.Sprintf("unpushed:%d:%s", i, entry.CommitSHA),
					Kind:    MagitRowLog,
					Section: "unpushed",
					Path:    entry.CommitSHA,
				})
			}
		}
	}
	if len(snap.Unpulled) > 0 {
		rows = append(rows, MagitRow{ID: "sec:unpulled", Kind: MagitRowSection, Section: "unpulled"})
		if visibilityLevel >= 2 && !folded["sec:unpulled"] {
			for i, entry := range snap.Unpulled {
				rows = append(rows, MagitRow{
					ID:      fmt.Sprintf("unpulled:%d:%s", i, entry.CommitSHA),
					Kind:    MagitRowLog,
					Section: "unpulled",
					Path:    entry.CommitSHA,
				})
			}
		}
	}

	if len(snap.RecentLog) > 0 {
		rows = append(rows, MagitRow{ID: "sec:log", Kind: MagitRowSection, Section: "log"})
		if visibilityLevel >= 2 && !folded["sec:log"] {
			for i, entry := range snap.RecentLog {
				rows = append(rows, MagitRow{
					ID:      fmt.Sprintf("log:%d:%s", i, entry.CommitSHA),
					Kind:    MagitRowLog,
					Section: "log",
					Path:    entry.CommitSHA,
				})
			}
		}
	}
	if len(snap.OpLog) > 0 {
		rows = append(rows, MagitRow{ID: "sec:oplog", Kind: MagitRowSection, Section: "oplog"})
		if visibilityLevel >= 2 && !folded["sec:oplog"] {
			for i, op := range snap.OpLog {
				rows = append(rows, MagitRow{
					ID:      fmt.Sprintf("op:%d:%s", i, op.OpID),
					Kind:    MagitRowOpLog,
					Section: "oplog",
					Path:    op.OpID,
				})
			}
		}
	}
	return rows
}

// ComputeMagitAuditStats tallies Human-AI code audit progress across all hunks.
func ComputeMagitAuditStats(snap hostlink.MagitSnapshot, auditMap map[string]string) (audited, flagged, unreviewed, total, highRemaining int) {
	visit := func(files []hostlink.MagitFile) {
		for _, f := range files {
			for _, h := range f.Hunks {
				total++
				st := auditMap[h.HunkID]
				switch st {
				case "audited":
					audited++
				case "flagged":
					flagged++
					if h.Risk == "high" {
						highRemaining++
					}
				default:
					unreviewed++
					if h.Risk == "high" {
						highRemaining++
					}
				}
			}
		}
	}
	visit(snap.Conflicts)
	visit(snap.Unstaged)
	visit(snap.Staged)
	return audited, flagged, unreviewed, total, highRemaining
}

func renderMagit(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.Magit == nil {
		ui.Column(c.Key("magit-empty")).Fill().Padding(24).Gap(8).Children(func() {
			ui.Text(c, "*magit* initializing...").Font(theme.FontMono).FontSize(theme.FontSizeBody).TextColor(th.InkMuted)
		})
		return
	}
	ms := bc.Magit
	snap := ms.Snapshot
	fire := func(action, target string) {
		if bc.OnMagitAction != nil {
			bc.OnMagitAction(action, target)
		}
	}

	audited, flagged, unreviewed, totalHunks, highRemaining := ComputeMagitAuditStats(snap, ms.AuditMap)
	rowIndexByID := make(map[string]int, len(ms.Rows))
	for i, r := range ms.Rows {
		rowIndexByID[r.ID] = i
	}
	isSelected := func(id string) bool {
		idx, ok := rowIndexByID[id]
		return ok && idx == ms.Cursor
	}

	ui.Column(c.Key("magit-root")).Fill().AlignItems(ui.Stretch).Children(func() {
		// 1. Sticky Magit Status + Human-AI Audit Progress + Aitia Provenance Header
		renderMagitHeader(c, th, ms, audited, flagged, unreviewed, totalHunks, highRemaining, fire)
		ui.Box(c).Height(1).Background(kit.WithAlpha(th.Border, 0.45))

		// 2. Scrollable Collapsible Sections Body
		ui.Scroll(c.Key("magit-scroll")).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c.Key("magit-body")).Padding(12, 16, 20, 16).Gap(10).AlignItems(ui.Stretch).Children(func() {
				if ms.Error != "" {
					ui.Row(c.Key("magit-err")).Padding(8, 12).Radius(theme.RadiusSmall).
						Background(kit.WithAlpha(th.Danger, 0.16)).Border(1, kit.WithAlpha(th.Danger, 0.50)).
						Children(func() {
							ui.Text(c, "Magit error: "+ms.Error).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Danger)
						})
				}
				if r := snap.LastReceipt; r != nil {
					tone := th.Success
					if !r.OK {
						tone = th.Danger
					}
					ui.Row(c.Key("magit-receipt")).Padding(6, 12).Gap(10).Radius(theme.RadiusSmall).
						Background(kit.WithAlpha(tone, 0.11)).Border(1, kit.WithAlpha(tone, 0.35)).
						AlignItems(ui.Center).Children(func() {
						ui.Text(c, "RECEIPT").Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(tone).NoWrap()
						ui.Text(c, fmt.Sprintf("%s — %s", r.Action, r.Summary)).Font(theme.FontMono).FontSize(theme.FontSizeUI).
							TextColor(th.Ink).Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
						if r.AitiaEvidenceID != "" {
							ui.Text(c, r.AitiaEvidenceID).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
						}
					})
				}

				if len(snap.Conflicts) == 0 && len(snap.Unstaged) == 0 && len(snap.Staged) == 0 && len(snap.Untracked) == 0 {
					ui.Row(c.Key("magit-clean")).Padding(14, 16).Radius(theme.RadiusSmall).
						Background(theme.GlassTone(c, th.Bar, th, 0.40, 0.72)).
						Border(1, kit.WithAlpha(th.Success, 0.35)).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "✓ Working tree clean").Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(th.Success)
						ui.Text(c, "Press g to refresh, c n for jj new, or l to inspect commits.").Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.InkMuted)
					})
				}

				renderMagitFileSection(c, th, ms, "conflicts", "Conflicts", th.Danger, snap.Conflicts, isSelected, rowIndexByID, fire)
				renderMagitFileSection(c, th, ms, "unstaged", "Unstaged changes", th.Warning, snap.Unstaged, isSelected, rowIndexByID, fire)
				renderMagitFileSection(c, th, ms, "staged", "Staged changes", th.Success, snap.Staged, isSelected, rowIndexByID, fire)
				renderMagitFileSection(c, th, ms, "untracked", "Untracked files", th.InkMuted, snap.Untracked, isSelected, rowIndexByID, fire)
				renderMagitStashSection(c, th, ms, snap.Stashes, isSelected, fire)
				renderMagitNamedLogSection(c, th, ms, "unpushed", "Unpushed to "+nonEmpty(snap.Upstream, "upstream"), th.Warning, snap.Unpushed, isSelected, fire)
				renderMagitNamedLogSection(c, th, ms, "unpulled", "Unpulled from "+nonEmpty(snap.Upstream, "upstream"), th.Info, snap.Unpulled, isSelected, fire)
				renderMagitLogSection(c, th, ms, snap.RecentLog, isSelected, rowIndexByID, fire)
				renderMagitOpLogSection(c, th, ms, snap.OpLog, isSelected, rowIndexByID, fire)
			})
		})

		// 3. Compact Doom Magit Keyboard Legend Bar
		ui.Box(c).Height(1).Background(kit.WithAlpha(th.Border, 0.45))
		ui.Row(c.Key("magit-footer")).Height(26).Padding(0, 14).Gap(12).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.Bar, th, 0.45, 0.85)).Children(func() {
			ui.Text(c, "j/k n/p ^ move · TAB fold · 1-4 level · RET visit · s/u/x hunk/file · v/! audit · c commit · d diff/Aitia · f/F/P sync · z stash/undo · X reset · ? dispatch (C-x g / C-x M-g)").
				Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).
				Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
		})
	})
}

func renderMagitHeader(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	audited, flagged, unreviewed, totalHunks, highRemaining int,
	fire func(action, target string),
) {
	snap := ms.Snapshot
	ui.Column(c.Key("magit-hdr")).Padding(10, 16).Gap(8).
		Background(theme.GlassTone(c, th.Bar, th, 0.50, 0.85)).AlignItems(ui.Stretch).Children(func() {
		// Top row: Repo + Backend + Branch/ChangeID + Action toolbar
		ui.Row(c.Key("magit-hdr-top")).Gap(8).AlignItems(ui.Center).Children(func() {
			backendLbl := "GIT"
			backendCol := th.Info
			if snap.Backend == "jj-colocated" {
				backendLbl = "JJ+GIT"
				backendCol = th.Accent
			}
			magitPill(c, "magit-backend-pill", backendLbl, backendCol)
			repoTitle := snap.RepoName
			if repoTitle == "" {
				repoTitle = "workspace"
			}
			ui.Text(c, repoTitle).Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(th.Ink).NoWrap()
			magitPill(c, "magit-branch-pill", "⎇ "+snap.Branch, th.Success)
			if snap.Upstream != "" {
				magitPill(c, "magit-up-pill", "→ "+snap.Upstream, th.Info)
			}
			if snap.LatestTag != "" {
				magitPill(c, "magit-tag-pill", "🏷 "+snap.LatestTag, th.InkMuted)
			}
			if snap.ChangeID != "" {
				magitPill(c, "magit-cid-pill", "jj@"+snap.ChangeID, th.Accent)
			}
			if snap.HeadSHA != "" {
				ui.Text(c, snap.HeadSHA).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			}
			if snap.Ahead > 0 || snap.Behind > 0 {
				ui.Text(c, fmt.Sprintf("↑%d ↓%d", snap.Ahead, snap.Behind)).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Warning).NoWrap()
			}
			ui.Box(c.Key("magit-hdr-spacer")).Grow(1)

			if theme.ActionButton(c, "magit-btn-audit", "AI Audit (d a)", th.Accent, true, th).Clicked() {
				fire("ai-audit", "")
			}
			if theme.ActionButton(c, "magit-btn-commit", "AI Commit (c i)", th.Info, false, th).Clicked() {
				fire("ai-commit", "")
			}
			if snap.Backend == "jj-colocated" {
				if theme.ActionButton(c, "magit-btn-undo", "jj Undo (z u)", th.Warning, false, th).Clicked() {
					fire("jj-undo", "")
				}
			}
			if theme.ActionButton(c, "magit-btn-refresh", "Refresh (g)", th.InkMuted, false, th).Clicked() {
				fire("refresh", "")
			}
			if theme.ActionButton(c, "magit-btn-dispatch", "?", th.InkMuted, false, th).Clicked() {
				fire("dispatch", "")
			}
		})

		// Second row: Head description + Human-AI Audit Progress + Aitia Evidence ID
		ui.Row(c.Key("magit-hdr-meta")).Gap(10).AlignItems(ui.Center).Children(func() {
			desc := snap.HeadDescription
			if desc == "" {
				desc = "(no description)"
			}
			ui.Text(c, "HEAD: "+desc).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.InkMuted).
				Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()

			auditTone := th.Success
			if highRemaining > 0 {
				auditTone = th.Danger
			} else if unreviewed > 0 || flagged > 0 {
				auditTone = th.Warning
			}
			auditText := fmt.Sprintf("Audit %d/%d", audited, totalHunks)
			if highRemaining > 0 {
				auditText += fmt.Sprintf(" · %d HIGH left", highRemaining)
			} else if flagged > 0 {
				auditText += fmt.Sprintf(" · %d flagged", flagged)
			}
			magitPill(c, "magit-audit-pill", auditText, auditTone)

			// Real Lambda Aitia gRPC status pill (honest online/offline & real finding count).
			aitiaTone := th.InkMuted
			aitiaLabel := "λ Aitia OFFLINE"
			if snap.AitiaOnline {
				if snap.AitiaTotalFindings > 0 || strings.EqualFold(snap.AitiaDecision, "BLOCK") || strings.EqualFold(snap.AitiaDecision, "DENY") {
					aitiaTone = th.Danger
					aitiaLabel = fmt.Sprintf("λ Aitia %s (%d)", snap.AitiaDecision, snap.AitiaTotalFindings)
				} else {
					aitiaTone = th.Success
					aitiaLabel = fmt.Sprintf("λ Aitia %s (0)", nonEmpty(snap.AitiaDecision, "ADMIT"))
				}
			}
			magitPill(c, "magit-aitia-pill", aitiaLabel, aitiaTone)

			if snap.AitiaEvidenceID != "" {
				ui.Text(c, snap.AitiaEvidenceID).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Info).NoWrap()
			}
			if ms.Loading {
				ui.Text(c, "syncing…").Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Warning).NoWrap()
			}
		})
	})
}

func renderMagitFileSection(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	secID, title string,
	accent ui.Color,
	files []hostlink.MagitFile,
	isSelected func(string) bool,
	rowIndexByID map[string]int,
	fire func(action, target string),
) {
	if len(files) == 0 {
		return
	}
	secKey := "sec:" + secID
	collapsed := ms.VisibilityLevel < 2 || ms.Folded[secKey]
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}

	ui.Column(c.Key("magit-sec-"+secID)).Gap(4).AlignItems(ui.Stretch).Children(func() {
		secRow := ui.ButtonBase(c.Key("magit-sec-hdr-"+secID)).Label(title).FocusRing(false).
			Padding(6, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(secKey) {
			secRow.Background(kit.WithAlpha(th.Accent, 0.20)).Border(1, kit.WithAlpha(th.Accent, 0.55))
		} else {
			secRow.Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.60))
		}
		secRow.Children(func() {
			ui.Row(c.Key("magit-sec-hdr-in-"+secID)).Gap(8).AlignItems(ui.Center).Grow(1).Children(func() {
				ui.Text(c, arrow).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(accent).NoWrap()
				ui.Text(c, fmt.Sprintf("%s (%d)", title, len(files))).Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(accent).NoWrap()
				ui.Box(c.Key("magit-sec-sp-"+secID)).Grow(1)
				if secID == "unstaged" || secID == "untracked" {
					if theme.ActionButton(c, "magit-stage-all-"+secID, "Stage All (S)", th.Success, false, th).Clicked() {
						fire("stage-all", "")
					}
				} else if secID == "staged" {
					if theme.ActionButton(c, "magit-unstage-all", "Unstage All (U)", th.Warning, false, th).Clicked() {
						fire("unstage-all", "")
					}
				}
			})
		})
		if secRow.Clicked() {
			if idx, ok := rowIndexByID[secKey]; ok {
				fire("select-row", fmt.Sprintf("%d", idx))
			}
			fire("toggle-fold", secKey)
		}

		if collapsed {
			return
		}
		for i := range files {
			f := files[i]
			renderMagitFileCard(c, th, ms, secID, f, isSelected, rowIndexByID, fire)
		}
	})
}

func renderMagitFileCard(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	secID string,
	f hostlink.MagitFile,
	isSelected func(string) bool,
	rowIndexByID map[string]int,
	fire func(action, target string),
) {
	fid := fmt.Sprintf("file:%s:%s", secID, f.Path)
	collapsed := ms.VisibilityLevel < 3 || ms.Folded[fid]
	arrow := "▾"
	if collapsed || len(f.Hunks) == 0 {
		arrow = "▸"
	}
	riskCol := magitRiskColor(f.Risk, th)

	ui.Column(c.Key("magit-fwrap-"+fid)).Padding(0, 0, 0, 10).Gap(4).AlignItems(ui.Stretch).Children(func() {
		btn := ui.ButtonBase(c.Key("magit-frow-"+fid)).Label(f.Path).FocusRing(false).
			Padding(5, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(fid) {
			btn.Background(kit.WithAlpha(th.Accent, 0.22)).Border(1, kit.WithAlpha(th.Accent, 0.65))
		} else {
			btn.Background(theme.GlassTone(c, th.SurfaceElevated, th, 0.25, 0.55))
		}
		btn.Children(func() {
			ui.Row(c.Key("magit-frow-in-"+fid)).Gap(8).AlignItems(ui.Center).Grow(1).Children(func() {
				ui.Text(c, arrow).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
				ui.Text(c, f.StatusCode).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(700).TextColor(riskCol).Width(18)
				magitPill(c, "magit-frisk-"+fid, strings.ToUpper(nonEmpty(f.Risk, "low")), riskCol)
				if f.Concern != "" {
					ui.Text(c, f.Concern).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
				}
				ui.Text(c, f.Path).Font(theme.FontMono).FontSize(theme.FontSizeUI).FontWeight(600).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, fmt.Sprintf("+%d -%d", f.Added, f.Removed)).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()

				if secID == "unstaged" || secID == "untracked" {
					if theme.ActionButton(c, "magit-s-"+fid, "Stage (s)", th.Success, false, th).Clicked() {
						fire("stage", f.Path)
					}
					if secID == "unstaged" {
						if theme.ActionButton(c, "magit-x-"+fid, "Discard (x)", th.Danger, false, th).Clicked() {
							fire("discard", f.Path)
						}
					}
				} else if secID == "staged" {
					if theme.ActionButton(c, "magit-u-"+fid, "Unstage (u)", th.Warning, false, th).Clicked() {
						fire("unstage", f.Path)
					}
				} else if secID == "conflicts" {
					if theme.ActionButton(c, "magit-ma-"+fid, "AI Resolve (m a)", th.Danger, true, th).Clicked() {
						fire("ai-resolve", f.Path)
					}
				}
			})
		})
		if btn.Clicked() {
			if idx, ok := rowIndexByID[fid]; ok {
				fire("select-row", fmt.Sprintf("%d", idx))
			}
			fire("toggle-fold", fid)
		}

		if collapsed || len(f.Hunks) == 0 {
			return
		}
		for _, h := range f.Hunks {
			renderMagitHunkCard(c, th, ms, h, isSelected, rowIndexByID, fire)
		}
	})
}

func renderMagitHunkCard(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	h hostlink.MagitHunk,
	isSelected func(string) bool,
	rowIndexByID map[string]int,
	fire func(action, target string),
) {
	hid := "hunk:" + h.HunkID
	riskCol := magitRiskColor(h.Risk, th)
	auditState := ms.AuditMap[h.HunkID]
	if auditState == "" {
		auditState = "unreviewed"
	}
	showDiffLines := ms.VisibilityLevel >= 4 || (ms.VisibilityLevel >= 3 && !ms.Folded[hid])

	borderCol := kit.WithAlpha(th.Border, 0.45)
	if isSelected(hid) {
		borderCol = kit.WithAlpha(th.Accent, 0.78)
	} else if h.Risk == "high" && auditState != "audited" {
		borderCol = kit.WithAlpha(th.Danger, 0.55)
	}

	ui.Column(c.Key("magit-hcard-"+h.HunkID)).Padding(0, 0, 0, 14).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c.Key("magit-hbox-"+h.HunkID)).Radius(theme.RadiusSmall).
			Background(theme.GlassTone(c, th.Canvas, th, 0.45, 0.78)).
			Border(1, borderCol).AlignItems(ui.Stretch).Children(func() {
			// Hunk header bar
			hdrBtn := ui.ButtonBase(c.Key("magit-hhdr-"+h.HunkID)).Label(h.Header).FocusRing(false).
				Padding(4, 10).AlignItems(ui.Center).Justify(ui.Start)
			if isSelected(hid) {
				hdrBtn.Background(kit.WithAlpha(th.Accent, 0.16))
			}
			hdrBtn.Children(func() {
				ui.Row(c.Key("magit-hhdr-in-"+h.HunkID)).Gap(8).AlignItems(ui.Center).Grow(1).Children(func() {
					ui.Text(c, h.Header).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Info).NoWrap()
					ui.Text(c, h.RiskReason).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(riskCol).
						Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()

					switch auditState {
					case "audited":
						magitPill(c, "magit-haud-"+h.HunkID, "✓ AUDITED", th.Success)
					case "flagged":
						magitPill(c, "magit-haud-"+h.HunkID, "⚑ FLAGGED", th.Danger)
					default:
						magitPill(c, "magit-haud-"+h.HunkID, "UNREVIEWED", th.InkMuted)
					}

					if h.Section == "unstaged" {
						if theme.ActionButton(c, "magit-hs-"+h.HunkID, "Stage Hunk (s)", th.Success, false, th).Clicked() {
							fire("stage-hunk", h.HunkID)
						}
					} else if h.Section == "staged" {
						if theme.ActionButton(c, "magit-hu-"+h.HunkID, "Unstage Hunk (u)", th.Warning, false, th).Clicked() {
							fire("unstage-hunk", h.HunkID)
						}
					}
					if theme.ActionButton(c, "magit-hv-"+h.HunkID, "✓ (v)", th.Success, auditState == "audited", th).Clicked() {
						fire("audit-toggle", h.HunkID)
					}
					if theme.ActionButton(c, "magit-hf-"+h.HunkID, "⚑ (!)", th.Danger, auditState == "flagged", th).Clicked() {
						fire("audit-flag", h.HunkID)
					}
					if theme.ActionButton(c, "magit-ha-"+h.HunkID, "AI Audit", th.Accent, false, th).Clicked() {
						if idx, ok := rowIndexByID[hid]; ok {
							fire("select-row", fmt.Sprintf("%d", idx))
						}
						fire("ai-audit", h.HunkID)
					}
				})
			})
			if hdrBtn.Clicked() {
				if idx, ok := rowIndexByID[hid]; ok {
					fire("select-row", fmt.Sprintf("%d", idx))
				}
				fire("toggle-fold", hid)
			}

			// Cited Multi-Source Evidence Chain strip (Aitia gRPC + Formal + Human/LLM + Structural).
			if len(h.Evidence) > 0 {
				ui.Row(c.Key("magit-hev-"+h.HunkID)).Padding(2, 10, 4, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					for ei, ev := range h.Evidence {
						if ei >= 4 {
							break
						}
						evTone := magitEvidenceColor(ev.Source, ev.Verdict, th)
						lbl := fmt.Sprintf("[%s:%s] %s", strings.ToUpper(ev.Source), ev.Verdict, ev.RuleOrActor)
						magitPill(c, fmt.Sprintf("magit-evp-%s-%d", h.HunkID, ei), lbl, evTone)
					}
				})
			}

			// Diff lines
			if showDiffLines && strings.TrimSpace(h.Preview) != "" {
				ui.Column(c.Key("magit-hlines-"+h.HunkID)).Padding(4, 10, 6, 10).Gap(1).AlignItems(ui.Stretch).Children(func() {
					for li, line := range strings.Split(h.Preview, "\n") {
						lineCol := th.InkMuted
						lineBg := ui.Transparent
						if strings.HasPrefix(line, "+") {
							lineCol = th.Success
							lineBg = kit.WithAlpha(th.Success, 0.10)
						} else if strings.HasPrefix(line, "-") {
							lineCol = th.Danger
							lineBg = kit.WithAlpha(th.Danger, 0.10)
						}
						ui.Row(c.Key(fmt.Sprintf("magit-hl-%s-%d", h.HunkID, li))).Padding(0, 4).Background(lineBg).Children(func() {
							ui.Text(c, line).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(lineCol).
								Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
						})
					}
				})
			}
		})
	})
}

func renderMagitLogSection(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	entries []hostlink.MagitLogEntry,
	isSelected func(string) bool,
	rowIndexByID map[string]int,
	fire func(action, target string),
) {
	if len(entries) == 0 {
		return
	}
	secKey := "sec:log"
	collapsed := ms.VisibilityLevel < 2 || ms.Folded[secKey]
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	ui.Column(c.Key("magit-sec-log")).Gap(4).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key("magit-sec-hdr-log")).Label("Recent commits").FocusRing(false).
			Padding(6, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(secKey) {
			hdr.Background(kit.WithAlpha(th.Accent, 0.20)).Border(1, kit.WithAlpha(th.Accent, 0.55))
		} else {
			hdr.Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.60))
		}
		hdr.Children(func() {
			ui.Text(c, fmt.Sprintf("%s Recent commits / jj changes (%d)", arrow, len(entries))).
				Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(th.Info)
		})
		if hdr.Clicked() {
			fire("toggle-fold", secKey)
		}
		if collapsed {
			return
		}
		for i, entry := range entries {
			rid := fmt.Sprintf("log:%d:%s", i, entry.CommitSHA)
			row := ui.Row(c.Key("magit-log-"+rid)).Padding(3, 12).Gap(8).AlignItems(ui.Center).Radius(theme.RadiusSmall)
			if isSelected(rid) {
				row.Background(kit.WithAlpha(th.Accent, 0.18))
			}
			row.Children(func() {
				marker := "○"
				if entry.IsWorkingCopy {
					marker = "@"
				}
				ui.Text(c, marker).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Accent).Width(14)
				if entry.ChangeID != "" {
					ui.Text(c, entry.ChangeID).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Accent).NoWrap()
				}
				ui.Text(c, entry.CommitSHA).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Info).NoWrap()
				for bi, bm := range entry.Bookmarks {
					magitPill(c, fmt.Sprintf("magit-bm-%s-%d", rid, bi), bm, th.Success)
				}
				ui.Text(c, nonEmpty(entry.Summary, "(no description)")).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, entry.RelativeTime).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			})
		}
	})
}

func renderMagitOpLogSection(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	ops []hostlink.MagitOpEntry,
	isSelected func(string) bool,
	rowIndexByID map[string]int,
	fire func(action, target string),
) {
	if len(ops) == 0 {
		return
	}
	secKey := "sec:oplog"
	collapsed := ms.VisibilityLevel < 2 || ms.Folded[secKey]
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	ui.Column(c.Key("magit-sec-oplog")).Gap(4).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key("magit-sec-hdr-oplog")).Label("jj Operation Log").FocusRing(false).
			Padding(6, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(secKey) {
			hdr.Background(kit.WithAlpha(th.Accent, 0.20)).Border(1, kit.WithAlpha(th.Accent, 0.55))
		} else {
			hdr.Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.60))
		}
		hdr.Children(func() {
			ui.Text(c, fmt.Sprintf("%s jj Operation Log — Time Travel (%d)", arrow, len(ops))).
				Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(th.Accent)
		})
		if hdr.Clicked() {
			fire("toggle-fold", secKey)
		}
		if collapsed {
			return
		}
		for i, op := range ops {
			rid := fmt.Sprintf("op:%d:%s", i, op.OpID)
			row := ui.Row(c.Key("magit-op-"+rid)).Padding(3, 12).Gap(8).AlignItems(ui.Center).Radius(theme.RadiusSmall)
			if isSelected(rid) {
				row.Background(kit.WithAlpha(th.Accent, 0.18))
			}
			row.Children(func() {
				mark := "○"
				if op.IsCurrent {
					mark = "@"
				}
				ui.Text(c, mark).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Accent).Width(14)
				ui.Text(c, op.OpID).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.Warning).NoWrap()
				ui.Text(c, op.Description).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, op.Timestamp).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			})
		}
	})
}

func renderMagitStashSection(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	stashes []hostlink.MagitStashEntry,
	isSelected func(string) bool,
	fire func(action, target string),
) {
	if len(stashes) == 0 {
		return
	}
	secKey := "sec:stashes"
	collapsed := ms.VisibilityLevel < 2 || ms.Folded[secKey]
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	ui.Column(c.Key("magit-sec-stashes")).Gap(4).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key("magit-sec-hdr-stashes")).Label("Stashes").FocusRing(false).
			Padding(6, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(secKey) {
			hdr.Background(kit.WithAlpha(th.Accent, 0.20)).Border(1, kit.WithAlpha(th.Accent, 0.55))
		} else {
			hdr.Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.60))
		}
		hdr.Children(func() {
			ui.Row(c.Key("magit-sec-stashes-in")).Gap(8).AlignItems(ui.Center).Grow(1).Children(func() {
				ui.Text(c, fmt.Sprintf("%s Stashes (%d)", arrow, len(stashes))).
					Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(th.Warning)
				ui.Box(c).Grow(1)
				if theme.ActionButton(c, "magit-stash-pop", "Pop (z p)", th.Success, false, th).Clicked() {
					fire("stash-pop", "")
				}
				if theme.ActionButton(c, "magit-stash-drop", "Drop (z k)", th.Danger, false, th).Clicked() {
					fire("stash-drop", "")
				}
			})
		})
		if hdr.Clicked() {
			fire("toggle-fold", secKey)
		}
		if collapsed {
			return
		}
		for i, st := range stashes {
			rid := fmt.Sprintf("stash:%d:%s", i, st.Reference)
			row := ui.Row(c.Key("magit-stash-"+rid)).Padding(3, 12).Gap(8).AlignItems(ui.Center).Radius(theme.RadiusSmall)
			if isSelected(rid) {
				row.Background(kit.WithAlpha(th.Accent, 0.18))
			}
			row.Children(func() {
				ui.Text(c, st.Reference).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(th.Warning).NoWrap()
				ui.Text(c, st.Summary).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, st.RelativeTime).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			})
		}
	})
}

func renderMagitNamedLogSection(
	c *ui.Context,
	th theme.Theme,
	ms *MagitViewState,
	secID, title string,
	accent ui.Color,
	entries []hostlink.MagitLogEntry,
	isSelected func(string) bool,
	fire func(action, target string),
) {
	if len(entries) == 0 {
		return
	}
	secKey := "sec:" + secID
	collapsed := ms.VisibilityLevel < 2 || ms.Folded[secKey]
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	ui.Column(c.Key("magit-sec-"+secID)).Gap(4).AlignItems(ui.Stretch).Children(func() {
		hdr := ui.ButtonBase(c.Key("magit-sec-hdr-"+secID)).Label(title).FocusRing(false).
			Padding(6, 10).Radius(theme.RadiusSmall).AlignItems(ui.Center).Justify(ui.Start)
		if isSelected(secKey) {
			hdr.Background(kit.WithAlpha(th.Accent, 0.20)).Border(1, kit.WithAlpha(th.Accent, 0.55))
		} else {
			hdr.Background(theme.GlassTone(c, th.Bar, th, 0.32, 0.60))
		}
		hdr.Children(func() {
			ui.Text(c, fmt.Sprintf("%s %s (%d)", arrow, title, len(entries))).
				Font(theme.FontMono).FontSize(theme.FontSizeBody).FontWeight(700).TextColor(accent)
		})
		if hdr.Clicked() {
			fire("toggle-fold", secKey)
		}
		if collapsed {
			return
		}
		for i, entry := range entries {
			rid := fmt.Sprintf("%s:%d:%s", secID, i, entry.CommitSHA)
			row := ui.Row(c.Key("magit-nlog-"+rid)).Padding(3, 12).Gap(8).AlignItems(ui.Center).Radius(theme.RadiusSmall)
			if isSelected(rid) {
				row.Background(kit.WithAlpha(th.Accent, 0.18))
			}
			row.Children(func() {
				ui.Text(c, entry.CommitSHA).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(accent).NoWrap()
				ui.Text(c, nonEmpty(entry.Summary, "(no description)")).Font(theme.FontMono).FontSize(theme.FontSizeUI).TextColor(th.Ink).
					Basis(0).Grow(1).SingleLine().Ellipsis("…").MinWidth(0).Clip()
				ui.Text(c, entry.RelativeTime).Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted).NoWrap()
			})
		}
	})
}

func magitEvidenceColor(source, verdict string, th theme.Theme) ui.Color {
	switch strings.ToLower(verdict) {
	case "finding", "flagged", "disputed", "block", "deny":
		return th.Danger
	case "pending":
		return th.Warning
	case "admit", "verified", "audited":
		return th.Success
	}
	if source == "aitia-grpc" {
		return th.Info
	}
	return th.InkMuted
}

func magitPill(c *ui.Context, key, label string, tone ui.Color) {
	ui.Row(c.Key(key)).Padding(2, 7).Radius(theme.RadiusPill).
		Background(kit.WithAlpha(tone, 0.16)).Border(1, kit.WithAlpha(tone, 0.42)).
		AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, label).Font(theme.FontMono).FontSize(theme.FontSizeMeta).FontWeight(700).TextColor(tone).NoWrap()
	})
}

func magitRiskColor(risk string, th theme.Theme) ui.Color {
	switch strings.ToLower(risk) {
	case "high":
		return th.Danger
	case "medium":
		return th.Warning
	case "low":
		return th.Info
	default:
		return th.InkMuted
	}
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
