// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tao3k/marlin-mygo/control/grpcwire"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// AttachRoomResponder wires live Agent Room @bot mentions and 1:1 Direct Bot
// channels to the connected Rust marlin-app-server gRPC TurnStreamController,
// giving each role (@architect, @reviewer, @security) its own isolated session
// cursor so room conversations never pollute the user's primary 1:1 Chat transcript.
func AttachRoomResponder(link *Link, cfg appconfig.Config) {
	if link == nil || link.TurnCtrl == nil {
		return
	}
	turnCtrl := link.TurnCtrl
	baseSession := strings.TrimSpace(cfg.SessionID)
	if baseSession == "" {
		baseSession = "marlin-room"
	}

	var (
		mu        sync.Mutex
		opened    = make(map[string]bool)
		lastTurn  = make(map[string]uint64)
		subSeq    atomic.Uint64
		subPrefix = fmt.Sprintf("marlin-room-%d-", time.Now().UnixNano())
	)

	featurekit.SetRoomAgentResponder(func(ctx context.Context, req featurekit.RoomAgentRequest) (string, error) {
		roleKey := strings.ToLower(strings.TrimSpace(req.BotHandle))
		if roleKey == "" {
			roleKey = "agent"
		}
		sid := fmt.Sprintf("%s-room-%s", baseSession, roleKey)

		wirePrompt := formatRoleRoomPrompt(req)

		mu.Lock()
		if !opened[sid] {
			if _, err := turnCtrl.OpenSession(ctx, sid, true); err != nil {
				_, _ = turnCtrl.OpenSession(ctx, sid, false)
				if sum, sErr := turnCtrl.GetSessionSummary(ctx, sid); sErr == nil && sum != nil {
					if ord := sum.GetLatestTurn().GetOrdinal(); ord > lastTurn[sid] {
						lastTurn[sid] = ord
					}
				}
			}
			opened[sid] = true
		}
		after := lastTurn[sid]
		subID := fmt.Sprintf("%s%d", subPrefix, subSeq.Add(1))
		mu.Unlock()

		ref, err := turnCtrl.StartTurn(ctx, sid, wirePrompt, after, subID)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "cursor is stale") {
			if sum, sErr := turnCtrl.GetSessionSummary(ctx, sid); sErr == nil && sum != nil {
				fresh := sum.GetLatestTurn().GetOrdinal()
				mu.Lock()
				if fresh > lastTurn[sid] {
					lastTurn[sid] = fresh
				}
				after = lastTurn[sid]
				subID = fmt.Sprintf("%s%d", subPrefix, subSeq.Add(1))
				mu.Unlock()
				ref, err = turnCtrl.StartTurn(ctx, sid, wirePrompt, after, subID)
			}
		}
		if err != nil {
			return "", err
		}

		var (
			bufMu sync.Mutex
			buf   strings.Builder
		)
		_, _, watchErr := turnCtrl.WatchTurnResilient(ctx, ref, "", grpcwire.WatchRetryPolicy{
			InitialBackoff: 50 * time.Millisecond,
			MaxBackoff:     1500 * time.Millisecond,
			MaxRetries:     4,
		}, func(upd grpcwire.TurnStreamUpdate) {
			if upd.TextDelta == "" {
				return
			}
			bufMu.Lock()
			buf.WriteString(upd.TextDelta)
			bufMu.Unlock()
		})

		mu.Lock()
		if ord := ref.GetOrdinal(); ord > lastTurn[sid] {
			lastTurn[sid] = ord
		}
		mu.Unlock()

		bufMu.Lock()
		out := strings.TrimSpace(buf.String())
		bufMu.Unlock()
		if watchErr != nil && out == "" {
			return "", watchErr
		}
		return out, nil
	})
}

func formatRoleRoomPrompt(req featurekit.RoomAgentRequest) string {
	var b strings.Builder
	scope := "#" + req.ChannelName
	if req.IsDM {
		scope = "Direct 1:1 with @" + req.BotHandle
	}
	fmt.Fprintf(&b, "[Marlin Agent Room | Channel: %s | Role: %s (%s)]\n", scope, req.BotName, req.BotRole)
	fmt.Fprintf(&b, "Respond concisely and directly in Markdown as %s (%s). Do not address yourself or create infinite bot loops.\n", req.BotName, req.BotRole)
	if strings.TrimSpace(req.RecentContext) != "" {
		fmt.Fprintf(&b, "\nRecent room context:\n%s\n", req.RecentContext)
	}
	fmt.Fprintf(&b, "\nUser message:\n%s", strings.TrimSpace(req.UserPrompt))
	return b.String()
}
