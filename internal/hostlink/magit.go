// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
	uiv1 "github.com/tao3k/marlin-mygo/gen/marlin/ui/v1"
)

// MagitSnapshot is the Go UI projection of Rust's authoritative MagitSnapshot
// (jj-colocated or git-native + live Lambda Aitia gRPC Defense Audit) streamed
// over marlin.projection.v1 gRPC.
type MagitSnapshot struct {
	Revision           uint64
	RepoName           string
	RepoRoot           string
	Backend            string // "jj-colocated" or "git"
	Branch             string
	Upstream           string
	LatestTag          string
	HeadSHA            string
	ChangeID           string
	HeadDescription    string
	Ahead              uint32
	Behind             uint32
	AitiaEvidenceID    string
	AitiaOnline        bool
	AitiaDecision      string // "ADMIT" | "BLOCK" | "REVIEW" | "OFFLINE"
	AitiaEngineOwner   string
	AitiaReportDigest  string
	CausalAcyclic      bool
	HasCausalAcyclic   bool
	AitiaTotalFindings uint32
	AitiaNote          string
	OpID               string
	HighRiskHunks      uint32
	MediumRiskHunks    uint32
	TotalHunks         uint32
	ConflictCount      uint32
	LastReceipt        *MagitReceipt
	Conflicts          []MagitFile
	Unstaged           []MagitFile
	Staged             []MagitFile
	Untracked          []MagitFile
	Stashes            []MagitStashEntry
	Unpushed           []MagitLogEntry
	Unpulled           []MagitLogEntry
	RecentLog          []MagitLogEntry
	OpLog              []MagitOpEntry
	DemoMode           bool
	FetchedAt          time.Time
}

// MagitReceipt carries the latest Rust-executed Magit mutation receipt.
type MagitReceipt struct {
	Action          string
	Summary         string
	OK              bool
	OpIDBefore      string
	OpIDAfter       string
	AitiaEvidenceID string
}

// MagitEvidenceItem is one cited record inside a hunk's multi-source audit chain.
type MagitEvidenceItem struct {
	Source      string // aitia-grpc | formal-model | human-audit | llm-audit | structural
	Verdict     string
	RuleOrActor string
	Summary     string
}

// MagitFile represents one changed file entry inside a Magit status section.
type MagitFile struct {
	Path         string
	OriginalPath string
	Section      string // conflicts | unstaged | staged | untracked
	StatusCode   string
	Added        uint32
	Removed      uint32
	Risk         string // high | medium | unassessed | low | info
	Concern      string
	RiskReason   string
	Hunks        []MagitHunk
}

// MagitHunk represents one parsed unified diff hunk with cited multi-source evidence.
type MagitHunk struct {
	HunkID       string
	FilePath     string
	Section      string
	Header       string
	Added        uint32
	Removed      uint32
	Risk         string // high | medium | unassessed | low | info
	Concern      string
	FormalStatus string // verified | model-present-unrun | none
	RiskReason   string
	Evidence     []MagitEvidenceItem
	Preview      string
	StartLine    uint32
	EndLine      uint32
}

// MagitStashEntry represents one Git stash entry (`magit-insert-stashes`).
type MagitStashEntry struct {
	Reference    string
	Summary      string
	RelativeTime string
}

// MagitLogEntry represents one recent Git commit or Jujutsu change.
type MagitLogEntry struct {
	ChangeID      string
	CommitSHA     string
	Bookmarks     []string
	Summary       string
	Author        string
	RelativeTime  string
	HasConflict   bool
	IsWorkingCopy bool
}

// MagitOpEntry represents one Jujutsu operation log entry for time-travel undo.
type MagitOpEntry struct {
	OpID        string
	Description string
	Timestamp   string
	IsCurrent   bool
}

// FetchMagitSnapshot queries the authoritative Rust host over
// ProjectionQueryService::QueryProjectGraph("magit:snapshot") and reconstructs
// the typed MagitSnapshot for the default workspace repository.
func (l *Link) FetchMagitSnapshot(ctx context.Context) (MagitSnapshot, error) {
	return l.FetchMagitSnapshotFor(ctx, "")
}

// FetchMagitSnapshotFor queries the authoritative Rust host for the live Git/jj
// repository at workspaceRoot (or the host's default workspace root when empty).
// Never fabricates demo data when disconnected or on RPC failure.
func (l *Link) FetchMagitSnapshotFor(ctx context.Context, workspaceRoot string) (MagitSnapshot, error) {
	if l == nil || l.Switcher == nil {
		return MagitSnapshot{}, fmt.Errorf("not connected to marlin-app-server — launch via `just dev-desktop` or connect a Rust host")
	}
	target := l.Switcher.CurrentTarget()
	if target == nil || target.Projection() == nil {
		return MagitSnapshot{}, fmt.Errorf("no active Rust gRPC projection target — connect marlin-app-server")
	}
	q := "magit:snapshot"
	if root := strings.TrimSpace(workspaceRoot); root != "" {
		q = "magit:snapshot@" + root
	}
	qctx, cancel := context.WithTimeout(ctx, 14*time.Second)
	defer cancel()

	resp, err := target.Projection().QueryProjectGraph(qctx, &projectionv1.QueryProjectGraphRequest{
		ProjectId:        "default",
		Query:            q,
		SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_UNSPECIFIED,
	})
	if err != nil {
		return MagitSnapshot{}, fmt.Errorf("magit snapshot rpc: %w", err)
	}
	hasMagitMeta := false
	for _, rec := range resp.GetRecords() {
		if rec != nil && strings.HasPrefix(rec.GetKind(), "magit.") {
			hasMagitMeta = true
			break
		}
	}
	if !hasMagitMeta {
		return MagitSnapshot{}, fmt.Errorf("connected Rust host binary does not expose live `magit:snapshot` — rebuild `marlin-mygo-host` / `marlin-app-server`")
	}
	return DecodeMagitGraphRecords(resp.GetRecords()), nil
}

// ExecuteMagitCommand dispatches a declarative `magit.*` VCS mutation through
// the governance-gated UiWorkbenchService::DispatchUiEvent(RecordCommand) RPC
// (zero os/exec in Go — Rust owns 100% of git/jj execution and Aitia receipts).
func (l *Link) ExecuteMagitCommand(ctx context.Context, sessionID, command string) error {
	cmd := strings.TrimSpace(command)
	if !strings.HasPrefix(cmd, "magit.") {
		return fmt.Errorf("invalid magit command %q", cmd)
	}
	if l == nil || l.Switcher == nil {
		return fmt.Errorf("connect to marlin-app-server to execute %s", cmd)
	}
	target := l.Switcher.CurrentTarget()
	if target == nil || target.UI() == nil {
		return fmt.Errorf("no active Rust gRPC host for %s", cmd)
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	dctx, cancel := context.WithTimeout(ctx, 14*time.Second)
	defer cancel()

	resp, err := target.UI().DispatchUiEvent(dctx, &uiv1.DispatchUiEventRequest{
		SessionId:          sessionID,
		ExpectedGeneration: 0,
		Event: &uiv1.DispatchUiEventRequest_RecordCommand{
			RecordCommand: cmd,
		},
	})
	if err != nil {
		return fmt.Errorf("magit dispatch rpc: %w", err)
	}
	if !resp.GetAdmitted() {
		msg := "denied by host"
		if d := resp.GetDenial(); d != nil && strings.TrimSpace(d.GetMessage()) != "" {
			msg = d.GetMessage()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// DecodeMagitGraphRecords decodes structured `magit.*` GraphRecordView records
// returned by Rust's WorkspaceMagitBridge::project_snapshot.
func DecodeMagitGraphRecords(records []*projectionv1.GraphRecordView) MagitSnapshot {
	snap := MagitSnapshot{
		Backend:       "git",
		Branch:        "HEAD",
		AitiaDecision: "OFFLINE",
		FetchedAt:     time.Now(),
	}
	fileIndex := make(map[string]*MagitFile)
	var fileOrder []string

	decodeLog := func(rec *projectionv1.GraphRecordView, f map[string]string) MagitLogEntry {
		var bms []string
		if raw := strings.TrimSpace(f["bookmarks"]); raw != "" {
			for _, b := range strings.Split(raw, ",") {
				if tb := strings.TrimSpace(b); tb != "" {
					bms = append(bms, tb)
				}
			}
		}
		return MagitLogEntry{
			ChangeID:      f["change_id"],
			CommitSHA:     f["commit_sha"],
			Bookmarks:     bms,
			Summary:       rec.GetTitle(),
			Author:        f["author"],
			RelativeTime:  f["relative_time"],
			HasConflict:   f["has_conflict"] == "true",
			IsWorkingCopy: f["is_working_copy"] == "true",
		}
	}

	for _, rec := range records {
		if rec == nil {
			continue
		}
		f := rec.GetFacets()
		switch rec.GetKind() {
		case "magit.head":
			snap.RepoName = rec.GetTitle()
			snap.RepoRoot = rec.GetSourcePath()
			snap.HeadDescription = rec.GetSummary()
			snap.Revision = parseUint64(f["revision"])
			if b := f["backend"]; b != "" {
				snap.Backend = b
			}
			if br := f["branch"]; br != "" {
				snap.Branch = br
			}
			snap.Upstream = f["upstream"]
			snap.LatestTag = f["latest_tag"]
			snap.HeadSHA = f["head_sha"]
			snap.ChangeID = f["change_id"]
			snap.Ahead = uint32(parseUint64(f["ahead"]))
			snap.Behind = uint32(parseUint64(f["behind"]))
			snap.AitiaEvidenceID = f["aitia_evidence_id"]
			snap.AitiaOnline = f["aitia_online"] == "true"
			if dec := f["aitia_decision"]; dec != "" {
				snap.AitiaDecision = dec
			}
			snap.AitiaEngineOwner = f["aitia_engine_owner"]
			snap.AitiaReportDigest = f["aitia_report_digest"]
			if ca, ok := f["causal_acyclic"]; ok {
				snap.HasCausalAcyclic = true
				snap.CausalAcyclic = ca == "true"
			}
			snap.AitiaTotalFindings = uint32(parseUint64(f["aitia_total_findings"]))
			snap.AitiaNote = f["aitia_note"]
			snap.OpID = f["op_id"]
			snap.HighRiskHunks = uint32(parseUint64(f["high_risk_hunks"]))
			snap.MediumRiskHunks = uint32(parseUint64(f["medium_risk_hunks"]))
			snap.TotalHunks = uint32(parseUint64(f["total_hunks"]))
			snap.ConflictCount = uint32(parseUint64(f["conflict_count"]))

		case "magit.receipt":
			snap.LastReceipt = &MagitReceipt{
				Action:          rec.GetTitle(),
				Summary:         rec.GetSummary(),
				OK:              f["ok"] == "true",
				OpIDBefore:      f["op_id_before"],
				OpIDAfter:       f["op_id_after"],
				AitiaEvidenceID: f["aitia_evidence_id"],
			}

		case "magit.file":
			sec := f["section"]
			if sec == "" {
				sec = "unstaged"
			}
			key := sec + ":" + rec.GetTitle()
			mf := &MagitFile{
				Path:         rec.GetTitle(),
				OriginalPath: f["original_path"],
				Section:      sec,
				StatusCode:   f["status_code"],
				Added:        uint32(parseUint64(f["added"])),
				Removed:      uint32(parseUint64(f["removed"])),
				Risk:         f["risk"],
				Concern:      f["concern"],
				RiskReason:   rec.GetSummary(),
			}
			fileIndex[key] = mf
			fileOrder = append(fileOrder, key)

		case "magit.hunk":
			sec := f["section"]
			if sec == "" {
				sec = "unstaged"
			}
			path := rec.GetSourcePath()
			hunk := MagitHunk{
				HunkID:       rec.GetRecordId(),
				FilePath:     path,
				Section:      sec,
				Header:       rec.GetTitle(),
				Added:        uint32(parseUint64(f["added"])),
				Removed:      uint32(parseUint64(f["removed"])),
				Risk:         f["risk"],
				Concern:      f["concern"],
				FormalStatus: f["formal_status"],
				RiskReason:   f["risk_reason"],
				Evidence:     parseEvidenceChainFacet(f["evidence_chain"]),
				Preview:      rec.GetSummary(),
				StartLine:    rec.GetStartLine(),
				EndLine:      rec.GetEndLine(),
			}
			key := sec + ":" + path
			if mf, ok := fileIndex[key]; ok {
				mf.Hunks = append(mf.Hunks, hunk)
			}

		case "magit.stash":
			snap.Stashes = append(snap.Stashes, MagitStashEntry{
				Reference:    f["reference"],
				Summary:      rec.GetTitle(),
				RelativeTime: f["relative_time"],
			})

		case "magit.unpushed":
			snap.Unpushed = append(snap.Unpushed, decodeLog(rec, f))

		case "magit.unpulled":
			snap.Unpulled = append(snap.Unpulled, decodeLog(rec, f))

		case "magit.log":
			snap.RecentLog = append(snap.RecentLog, decodeLog(rec, f))

		case "magit.oplog":
			snap.OpLog = append(snap.OpLog, MagitOpEntry{
				OpID:        f["op_id"],
				Description: rec.GetTitle(),
				Timestamp:   rec.GetSummary(),
				IsCurrent:   f["is_current"] == "true",
			})
		}
	}

	for _, key := range fileOrder {
		mf := *fileIndex[key]
		switch mf.Section {
		case "conflicts":
			snap.Conflicts = append(snap.Conflicts, mf)
		case "staged":
			snap.Staged = append(snap.Staged, mf)
		case "untracked":
			snap.Untracked = append(snap.Untracked, mf)
		default:
			snap.Unstaged = append(snap.Unstaged, mf)
		}
	}
	return snap
}

func parseEvidenceChainFacet(raw string) []MagitEvidenceItem {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []MagitEvidenceItem
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		out = append(out, MagitEvidenceItem{
			Source:      strings.TrimSpace(parts[0]),
			Verdict:     strings.TrimSpace(parts[1]),
			RuleOrActor: strings.TrimSpace(parts[2]),
			Summary:     strings.TrimSpace(parts[3]),
		})
	}
	return out
}

// DemoMagitSnapshot returns a realistic standalone preview MagitSnapshot with
// explicit evidence-chain provenance (honestly showing Aitia / Formal / Structural
// sources without fabricating regex risk scores).
func DemoMagitSnapshot() MagitSnapshot {
	return MagitSnapshot{
		Revision:           1,
		RepoName:           "marlin-agent-core (standalone preview)",
		RepoRoot:           "~/marlin-agent-core",
		Backend:            "jj-colocated",
		Branch:             "main",
		Upstream:           "origin/main",
		HeadSHA:            "a1b2c3d4",
		ChangeID:           "kntqzsqt",
		HeadDescription:    "feat(magit): AI-native jj + git + Lambda Aitia evidence workbench",
		AitiaEvidenceID:    "aitia:preview:kntqzsqt:r1",
		AitiaOnline:        false,
		AitiaDecision:      "OFFLINE",
		AitiaEngineOwner:   "standalone-preview",
		AitiaTotalFindings: 0,
		AitiaNote:          "Connect marlin-app-server for live Lambda Aitia gRPC defense audit",
		OpID:               "op7f2e91c4",
		HighRiskHunks:      0,
		MediumRiskHunks:    1,
		TotalHunks:         2,
		DemoMode:           true,
		FetchedAt:          time.Now(),
		Unstaged: []MagitFile{
			{
				Path:       "crates/marlin-agent-gitops/src/magit/engine.rs",
				Section:    "unstaged",
				StatusCode: "M",
				Added:      24,
				Removed:    4,
				Risk:       "medium",
				Concern:    "security-boundary",
				RiskReason: "Formal model pending (dev/quint/gitops-context.qnt) · Aitia preview",
				Hunks: []MagitHunk{
					{
						HunkID:       "unstaged:crates/marlin-agent-gitops/src/magit/engine.rs:h1:L88",
						FilePath:     "crates/marlin-agent-gitops/src/magit/engine.rs",
						Section:      "unstaged",
						Header:       "@@ -88,6 +88,12 @@ impl MagitEngine",
						Added:        6,
						Removed:      1,
						Risk:         "medium",
						Concern:      "security-boundary",
						FormalStatus: "model-present-unrun",
						RiskReason:   "Formal model pending (dev/quint/gitops-context.qnt) · awaiting Aitia gRPC",
						Evidence: []MagitEvidenceItem{
							{Source: "structural", Verdict: "diffstat", RuleOrActor: "security-boundary", Summary: "Structural diffstat (+6/-1) on security-boundary surface"},
							{Source: "formal-model", Verdict: "pending", RuleOrActor: "dev/quint/gitops-context.qnt", Summary: "Formal model exists — verification not yet attested for this diff"},
							{Source: "aitia-grpc", Verdict: "offline", RuleOrActor: "aitia.marlin.v1", Summary: "Standalone preview — connect marlin-app-server for live Aitia gRPC"},
						},
						Preview:   " pub async fn execute(&self, action: MagitAction) -> Result<MagitActionReceipt, MagitError> {\n-    run_cmd(&self.repo_root, action)\n+    validate_rel_path(path)?;\n+    let out = run_git_with_stdin(&self.repo_root, &[\"apply\", \"--cached\", \"-\"], patch.as_bytes()).await?;\n+    Ok(mint_aitia_receipt(action, out))\n }",
						StartLine: 88,
						EndLine:   100,
					},
				},
			},
			{
				Path:       "scheme/ui-magit/funcs.ss",
				Section:    "unstaged",
				StatusCode: "M",
				Added:      18,
				Removed:    2,
				Risk:       "unassessed",
				Concern:    "core-logic",
				RiskReason: "Aitia offline — structural diffstat only (+18/-2)",
				Hunks: []MagitHunk{
					{
						HunkID:       "unstaged:scheme/ui-magit/funcs.ss:h1:L95",
						FilePath:     "scheme/ui-magit/funcs.ss",
						Section:      "unstaged",
						Header:       "@@ -95,4 +95,9 @@ (def (marlin-magit-fuse-evidence-risk ...)",
						Added:        5,
						Removed:      0,
						Risk:         "unassessed",
						Concern:      "core-logic",
						FormalStatus: "none",
						RiskReason:   "Aitia offline — structural diffstat only (+5/-0)",
						Evidence: []MagitEvidenceItem{
							{Source: "structural", Verdict: "diffstat", RuleOrActor: "core-logic", Summary: "Structural diffstat (+5/-0) on core-logic surface"},
							{Source: "aitia-grpc", Verdict: "offline", RuleOrActor: "aitia.marlin.v1", Summary: "Standalone preview"},
						},
						Preview:   " (def (marlin-magit-fuse-evidence-risk aitia-status hits formal joint conflict?)\n+  ;; Derive risk strictly from cited Aitia + Formal + Human/LLM evidence\n   (cond ...))",
						StartLine: 95,
						EndLine:   104,
					},
				},
			},
		},
		RecentLog: []MagitLogEntry{
			{
				ChangeID:      "kntqzsqt",
				CommitSHA:     "a1b2c3d4",
				Bookmarks:     []string{"main"},
				Summary:       "feat(magit): AI-native jj + git + Lambda Aitia evidence workbench",
				Author:        "marlin",
				RelativeTime:  "just now",
				IsWorkingCopy: true,
			},
		},
		OpLog: []MagitOpEntry{
			{
				OpID:        "op7f2e91c4",
				Description: "snapshot working copy (standalone preview)",
				Timestamp:   "just now",
				IsCurrent:   true,
			},
		},
	}
}

func parseUint64(raw string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	return v
}
