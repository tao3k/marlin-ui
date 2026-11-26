// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package featurekit wires Rust-Cargo-style compile-time feature modules from
// github.com/tao3k/marlin-mygo into Marlin UI's AdaptiveShell using duck-typed
// capability interfaces so `-tags marlin_minimal` compiles with zero dead code.
package featurekit

import (
	"fmt"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/notify"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/resilience"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
)

var (
	compiled          []features.FeatureModule
	roomStatsProvider func() RoomStats
	roomTasksProvider func() []RoomTask
	roomTaskAdder     func(title string) error
	roomTaskCycler    func(id string) error
)

// RoomTask is a lightweight projection of one collaborative Agent Room matter
// so the Bento *Dashboard* can list, add, and toggle real tasks directly.
type RoomTask struct {
	ID     string
	Title  string
	Status string // "OPEN" | "IN_PROGRESS" | "BLOCKED" | "DONE"
	Owner  string
}

// RoomStats carries live Agent Room task and participant counts projected into
// the Bento *Dashboard* (zero-safe when chatroom is pruned via marlin_minimal).
type RoomStats struct {
	TasksDone    int
	TasksTotal   int
	AgentsOnline int
	AgentsTotal  int
}

// SetRoomStatsProvider registers an optional feature metrics provider.
func SetRoomStatsProvider(fn func() RoomStats) {
	roomStatsProvider = fn
}

// SetRoomTaskBridge registers optional real task CRUD hooks from Agent Room.
func SetRoomTaskBridge(list func() []RoomTask, add func(string) error, cycle func(string) error) {
	roomTasksProvider = list
	roomTaskAdder = add
	roomTaskCycler = cycle
}

// AgentRoomStats returns live Agent Room task & agent telemetry.
func AgentRoomStats() RoomStats {
	if roomStatsProvider != nil {
		return roomStatsProvider()
	}
	return RoomStats{}
}

// AgentRoomTasks returns live collaborative tasks from Agent Room (empty under marlin_minimal).
func AgentRoomTasks() []RoomTask {
	if roomTasksProvider != nil {
		return roomTasksProvider()
	}
	return nil
}

// AddAgentRoomTask creates a real task in the active Agent Room channel.
func AddAgentRoomTask(title string) error {
	if roomTaskAdder != nil {
		return roomTaskAdder(title)
	}
	return nil
}

// CycleAgentRoomTask advances a task's real status (OPEN -> IN_PROGRESS -> DONE -> OPEN).
func CycleAgentRoomTask(id string) error {
	if roomTaskCycler != nil {
		return roomTaskCycler(id)
	}
	return nil
}

// RegisterCompiled registers an opt-in FeatureModule linked via Go build tags.
func RegisterCompiled(mod features.FeatureModule) {
	if mod != nil {
		compiled = append(compiled, mod)
	}
}

// Mount activates selected compiled features under Tier-0 GovernancePolicy and
// bridges shared resilience, PromptGate receipts, and canvas/compare callbacks
// identically across Desktop, Mobile, and Web WASM targets.
func Mount(
	featuresSpec string,
	adaptive *mygoshell.AdaptiveShell,
	onDraftChanged func(scopeID, text string),
	invalidate func(),
) (*features.Registry, *features.GovernanceLedger, *features.Host) {
	reg := features.NewRegistry()
	ledger := features.NewGovernanceLedger(features.DefaultGovernancePolicy())

	type governanceAware interface {
		SetGovernanceLedger(*features.GovernanceLedger)
	}
	type resilienceAware interface {
		SetResilience(*resilience.SessionResilience, func(scopeID, text string))
	}
	type promptGateAware interface {
		SetPromptGate(*promptgate.Queue, func(gateID string))
		ApplyGateDecision(promptgate.PromptRequest, promptgate.PromptResponse)
	}
	type revisionAware interface {
		SetRevisionHandler(func(docID string, version int, instruction string))
	}
	type winnerAware interface {
		SetPromoteHandler(func(modelLabel, text string))
	}

	var gateReceivers []promptGateAware
	for _, mod := range compiled {
		if ga, ok := mod.(governanceAware); ok {
			ga.SetGovernanceLedger(ledger)
		}
		if resAware, ok := mod.(resilienceAware); ok && adaptive != nil && adaptive.Desktop != nil && adaptive.Desktop.Transcript != nil && adaptive.Desktop.Transcript.Resilience != nil {
			resAware.SetResilience(adaptive.Desktop.Transcript.Resilience, onDraftChanged)
		}
		if pgAware, ok := mod.(promptGateAware); ok && adaptive != nil && adaptive.Desktop != nil && adaptive.Desktop.PromptGate != nil {
			pgAware.SetPromptGate(adaptive.Desktop.PromptGate.Queue, func(_ string) {
				if invalidate != nil {
					invalidate()
				}
			})
			gateReceivers = append(gateReceivers, pgAware)
		}
		if ra, ok := mod.(revisionAware); ok && adaptive != nil && adaptive.Desktop != nil {
			ra.SetRevisionHandler(func(docID string, version int, instruction string) {
				if adaptive.Desktop.OnSendPrompt != nil {
					adaptive.Desktop.OnSendPrompt(fmt.Sprintf("[canvas %s@v%d] %s", docID, version, instruction))
				}
			})
		}
		if wa, ok := mod.(winnerAware); ok && adaptive != nil && adaptive.Desktop != nil && adaptive.Desktop.Transcript != nil {
			wa.SetPromoteHandler(func(modelLabel, text string) {
				if text == "" {
					return
				}
				tr := adaptive.Desktop.Transcript
				ord := tr.ActiveTurn() + 1
				tr.BeginAssistantTurn(ord)
				tr.ApplyCheckpoint(ord, fmt.Sprintf("promote-%d", ord), fmt.Sprintf("[%s promoted] %s", modelLabel, text))
				tr.CompleteTurn(ord, "PROMOTED")
				if invalidate != nil {
					invalidate()
				}
			})
		}
		_ = reg.RegisterCompiled(mod)
	}

	host := features.NewHost()
	reg.Activate(features.ParseFeatureSpec(featuresSpec), ledger, host)
	govUI := mygoshell.NewGovernanceController(reg, ledger, host)
	govUI.OnFeatureChanged = func(id features.FeatureID, enabled bool) {
		if adaptive != nil {
			adaptive.SetFeatureHost(host, govUI)
			if adaptive.Desktop != nil && adaptive.Desktop.Notifications != nil {
				state := "disabled"
				if enabled {
					state = "enabled"
				}
				_, _ = adaptive.Desktop.Notifications.Center.Publish(notify.NotificationSpec{
					Severity: notify.SeverityInfo,
					Source:   notify.SourceHost,
					Title:    "UI Feature Updated",
					Message:  fmt.Sprintf("Feature %q %s under Tier-0 Governance", id, state),
				})
			}
		}
		if invalidate != nil {
			invalidate()
		}
	}

	if adaptive != nil {
		adaptive.SetFeatureHost(host, govUI)
		if adaptive.Desktop != nil && adaptive.Desktop.PromptGate != nil {
			prevResolve := adaptive.Desktop.PromptGate.OnResolve
			adaptive.Desktop.PromptGate.OnResolve = func(req promptgate.PromptRequest, resp promptgate.PromptResponse) {
				outcome := features.AuditGateClaimed
				if resp.Action == promptgate.ActionReject || resp.Action == promptgate.ActionCancel {
					outcome = features.AuditDenied
				}
				ledger.RecordAudit("user", req.ID, string(resp.Action), outcome, resp.IdempotencyKey)
				for _, recv := range gateReceivers {
					recv.ApplyGateDecision(req, resp)
				}
				if prevResolve != nil {
					prevResolve(req, resp)
				}
			}
		}
	}
	return reg, ledger, host
}
