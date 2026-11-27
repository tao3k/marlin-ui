// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !marlin_minimal && (marlin_feat_chatroom || marlin_full || (!marlin_feat_compare && !marlin_feat_canvas))

package featurekit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/input"
	"github.com/tao3k/marlin-mygo/engine/promptgate"
	"github.com/tao3k/marlin-mygo/engine/resilience"
	"github.com/tao3k/marlin-mygo/engine/shell"
	"github.com/tao3k/marlin-mygo/features/chatroom"
	"github.com/tao3k/marlin-mygo/runtime/async"
)

func init() {
	mod := &bentoChatRoomModule{
		inner:             chatroom.NewModule(nil),
		activeTab:         chatroom.TabTimeline,
		dmMsgs:            make(map[string][]dmMessage),
		presenceOverrides: make(map[string]string),
		botScope:          async.NewStructuredScope(context.Background()),
	}
	SetRoomStatsProvider(mod.liveStats)
	SetRoomTaskBridge(mod.listTasks, mod.addTask, mod.cycleTask)
	SetRoomBridge(RoomBridge{
		Snapshot:       mod.fullSnapshot,
		SelectChannel:  mod.selectChannel,
		PostMessage:    mod.postMessage,
		PostGateEmbed:  mod.postGateEmbed,
		ToggleReaction: mod.toggleReaction,
		OpenThread:     mod.openThread,
		CloseThread:    mod.closeThread,
	})
	RegisterCompiled(mod)
}

type dmMessage struct {
	id           string
	channelID    string
	threadRootID string
	authorID     string
	body         string
	mentions     []string
	gateID       string
	reactions    map[string][]string
	createdAt    time.Time
}

type bentoChatRoomModule struct {
	inner       *chatroom.Module
	activeTab   chatroom.ViewTab
	draft       string
	threadDraft string
	itemDraft   string

	dmMu              sync.Mutex
	activeDMID        string
	activeDMThread    string
	dmMsgs            map[string][]dmMessage
	dmSeq             uint64
	gateEventSeq      uint64
	presenceOverrides map[string]string // botID -> "thinking" | "working"
	botScope          *async.StructuredScope
	gateQueue         *promptgate.Queue
}

func (m *bentoChatRoomModule) Meta() features.FeatureMeta { return m.inner.Meta() }

func (m *bentoChatRoomModule) SetGovernanceLedger(l *features.GovernanceLedger) {
	m.inner.SetGovernanceLedger(l)
}

func (m *bentoChatRoomModule) SetResilience(r *resilience.SessionResilience, fn func(string, string)) {
	m.inner.SetResilience(r, fn)
}

func (m *bentoChatRoomModule) SetPromptGate(q *promptgate.Queue, fn func(string)) {
	m.gateQueue = q
	m.inner.SetPromptGate(q, fn)
}

func (m *bentoChatRoomModule) ApplyGateDecision(req promptgate.PromptRequest, resp promptgate.PromptResponse) {
	m.inner.ApplyGateDecision(req, resp)
}

func (m *bentoChatRoomModule) Register(host *features.Host) error {
	host.AddBuffer(shell.PaneBuffer{
		BufferID: chatroom.BufferIDChatRoom,
		Title:    "Agent Room",
		Kind:     "native-gpu",
	}, func(surface any, th any) {
		c, ok1 := surface.(*ui.Context)
		t, ok2 := th.(*ui.Theme)
		if ok1 && ok2 {
			m.renderBentoAgentRoom(c, t)
		}
	})
	host.AddCommand(input.CommandEntry{
		Name:        chatroom.CommandOpenChatRoom,
		Description: "Open Multi-Role Agent Room (Bento matte channels, matters, roles)",
	})
	return nil
}

func (m *bentoChatRoomModule) liveStats() RoomStats {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return RoomStats{}
	}
	snap := m.inner.View.Ctrl.Snapshot()
	var st RoomStats
	st.TasksTotal = len(snap.Items)
	for _, it := range snap.Items {
		if it.Status == chatroom.ItemDone {
			st.TasksDone++
		}
	}
	for _, p := range snap.Participants {
		if p.Kind == chatroom.ParticipantAgent {
			st.AgentsTotal++
			if p.Presence != chatroom.PresenceOffline {
				st.AgentsOnline++
			}
		}
	}
	return st
}

func (m *bentoChatRoomModule) listTasks() []RoomTask {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	snap := m.inner.View.Ctrl.Snapshot()
	owners := make(map[string]string, len(snap.Participants))
	for _, p := range snap.Participants {
		owners[p.ID] = strings.TrimPrefix(strings.TrimSpace(p.Handle), "@")
	}
	out := make([]RoomTask, 0, len(snap.Items))
	for _, it := range snap.Items {
		own := owners[it.OwnerID]
		if own == "" {
			own = it.OwnerID
		}
		out = append(out, RoomTask{
			ID:     it.ID,
			Title:  it.Title,
			Status: string(it.Status),
			Owner:  own,
		})
	}
	return out
}

func (m *bentoChatRoomModule) addTask(title string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	_, err := ctrl.CreateItem(snap.ActiveChannelID, snap.CurrentActorID, title, snap.CurrentActorID)
	return err
}

func (m *bentoChatRoomModule) cycleTask(id string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	for _, it := range snap.Items {
		if it.ID != id {
			continue
		}
		next := chatroom.ItemInProgress
		switch it.Status {
		case chatroom.ItemInProgress:
			next = chatroom.ItemDone
		case chatroom.ItemDone:
			next = chatroom.ItemOpen
		case chatroom.ItemBlocked:
			next = chatroom.ItemInProgress
		}
		return ctrl.TransitionItem(id, snap.CurrentActorID, next)
	}
	return nil
}

func cleanRoomHandle(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "@")
}

func cleanRoomChannelName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimPrefix(s, "@")
	return s
}

func extractRoomMentions(body string) []string {
	fields := strings.Fields(body)
	seen := make(map[string]struct{})
	var out []string
	for _, tok := range fields {
		if !strings.HasPrefix(tok, "@") || len(tok) < 2 {
			continue
		}
		clean := strings.TrimRight(tok[1:], ".,:;!?()[]{}\"'")
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			out = append(out, clean)
		}
	}
	return out
}

func (m *bentoChatRoomModule) fullSnapshot() RoomFullSnapshot {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return RoomFullSnapshot{}
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()

	m.dmMu.Lock()
	activeDM := m.activeDMID
	activeDMThread := m.activeDMThread
	presCopy := make(map[string]string, len(m.presenceOverrides))
	for k, v := range m.presenceOverrides {
		presCopy[k] = v
	}
	var dmCopy []dmMessage
	if activeDM != "" {
		dmCopy = append(dmCopy, m.dmMsgs[activeDM]...)
	}
	m.dmMu.Unlock()

	byID := make(map[string]chatroom.Participant, len(snap.Participants))
	parts := make([]RoomParticipantView, 0, len(snap.Participants))
	var thinkingBots []string
	for _, p := range snap.Participants {
		byID[p.ID] = p
		pres := string(p.Presence)
		if ov, ok := presCopy[p.ID]; ok && ov != "" {
			pres = ov
		}
		if p.Kind == chatroom.ParticipantAgent && (pres == "thinking" || pres == "working") {
			if _, live := presCopy[p.ID]; live {
				thinkingBots = append(thinkingBots, p.DisplayName)
			}
		}
		parts = append(parts, RoomParticipantView{
			ID:              p.ID,
			Handle:          cleanRoomHandle(p.Handle),
			Name:            p.DisplayName,
			Role:            p.RoleTitle,
			IsAgent:         p.Kind == chatroom.ParticipantAgent,
			HostNode:        p.HostNode,
			Presence:        pres,
			CanPost:         p.Permissions.CanPost,
			CanManageItems:  p.Permissions.CanManageItems,
			CanInvokeTools:  p.Permissions.CanInvokeTools,
			RequireGate:     p.Permissions.RequireGate,
			CanApproveGates: p.Permissions.CanApproveGates,
		})
	}

	chans := make([]RoomChannelView, 0, len(snap.Channels)+len(snap.Participants))
	for _, ch := range snap.Channels {
		chans = append(chans, RoomChannelView{
			ID:          ch.ID,
			Name:        cleanRoomChannelName(ch.Name),
			Topic:       ch.Topic,
			IsDM:        ch.IsDM,
			UnreadCount: ch.UnreadCount,
		})
	}
	// Expose dedicated 1v1 Direct Bot channels alongside shared workspace channels.
	for _, p := range snap.Participants {
		if p.Kind != chatroom.ParticipantAgent {
			continue
		}
		topic := p.RoleTitle
		if p.HostNode != "" {
			topic += " · " + p.HostNode
		}
		chans = append(chans, RoomChannelView{
			ID:               "dm:" + p.ID,
			Name:             p.DisplayName,
			Topic:            topic,
			IsDM:             true,
			BotParticipantID: p.ID,
		})
	}

	buildReactions := func(raw map[string][]string) []RoomReactionView {
		tags := make([]string, 0, len(raw))
		for tag := range raw {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		var reacts []RoomReactionView
		for _, tag := range tags {
			actors := raw[tag]
			if len(actors) == 0 {
				continue
			}
			reacted := false
			for _, who := range actors {
				if who == snap.CurrentActorID {
					reacted = true
					break
				}
			}
			reacts = append(reacts, RoomReactionView{Tag: tag, Count: len(actors), Reacted: reacted})
		}
		return reacts
	}

	toMsg := func(rm chatroom.RoomMessage) RoomMessageView {
		au := byID[rm.AuthorID]
		name := au.DisplayName
		if name == "" {
			name = rm.AuthorID
		}
		handle := cleanRoomHandle(au.Handle)
		if handle == "" {
			handle = rm.AuthorID
		}
		pres := string(au.Presence)
		if ov, ok := presCopy[rm.AuthorID]; ok && ov != "" {
			pres = ov
		}
		ts := ""
		var createdUnix int64
		if !rm.CreatedAt.IsZero() {
			ts = rm.CreatedAt.Format("15:04")
			createdUnix = rm.CreatedAt.Unix()
		}
		mentions := make([]string, 0, len(rm.Mentions))
		for _, mn := range rm.Mentions {
			if cl := cleanRoomHandle(mn); cl != "" {
				mentions = append(mentions, cl)
			}
		}
		if len(mentions) == 0 {
			mentions = extractRoomMentions(rm.Body)
		}
		return RoomMessageView{
			ID:              rm.ID,
			ChannelID:       rm.ChannelID,
			AuthorID:        rm.AuthorID,
			AuthorName:      name,
			AuthorHandle:    handle,
			AuthorRole:      au.RoleTitle,
			AuthorPresence:  pres,
			IsAgent:         au.Kind == chatroom.ParticipantAgent,
			Body:            rm.Body,
			Mentions:        mentions,
			Pinned:          rm.Pinned,
			GateID:          rm.GateID,
			TaintedExternal: rm.TaintedExternal,
			Delivery:        string(rm.Delivery),
			Timestamp:       ts,
			CreatedUnix:     createdUnix,
			ThreadCount:     ctrl.ThreadReplyCount(rm.ID),
			Reactions:       buildReactions(rm.Reactions),
		}
	}

	if activeDM != "" {
		replyCounts := make(map[string]int)
		for _, dm := range dmCopy {
			if dm.threadRootID != "" {
				replyCounts[dm.threadRootID]++
			}
		}
		toDMMsg := func(dm dmMessage) RoomMessageView {
			au := byID[dm.authorID]
			name := au.DisplayName
			if name == "" {
				name = dm.authorID
			}
			handle := cleanRoomHandle(au.Handle)
			if handle == "" {
				handle = dm.authorID
			}
			pres := string(au.Presence)
			if ov, ok := presCopy[dm.authorID]; ok && ov != "" {
				pres = ov
			}
			ts := ""
			var unix int64
			if !dm.createdAt.IsZero() {
				ts = dm.createdAt.Format("15:04")
				unix = dm.createdAt.Unix()
			}
			return RoomMessageView{
				ID:             dm.id,
				ChannelID:      dm.channelID,
				AuthorID:       dm.authorID,
				AuthorName:     name,
				AuthorHandle:   handle,
				AuthorRole:     au.RoleTitle,
				AuthorPresence: pres,
				IsAgent:        au.Kind == chatroom.ParticipantAgent,
				Body:           dm.body,
				Mentions:       dm.mentions,
				GateID:         dm.gateID,
				Timestamp:      ts,
				CreatedUnix:    unix,
				ThreadCount:    replyCounts[dm.id],
				Reactions:      buildReactions(dm.reactions),
			}
		}
		var msgs, threadMsgs []RoomMessageView
		for _, dm := range dmCopy {
			if dm.threadRootID == "" {
				msgs = append(msgs, toDMMsg(dm))
			} else if activeDMThread != "" && dm.threadRootID == activeDMThread {
				threadMsgs = append(threadMsgs, toDMMsg(dm))
			}
		}
		return RoomFullSnapshot{
			Available:       true,
			ActiveChannelID: activeDM,
			ActiveThreadID:  activeDMThread,
			CurrentActorID:  snap.CurrentActorID,
			ThinkingBots:    thinkingBots,
			Channels:        chans,
			Participants:    parts,
			Messages:        msgs,
			ThreadMessages:  threadMsgs,
			Tasks:           m.listTasks(),
		}
	}

	var msgs, pinnedMsgs, threadMsgs []RoomMessageView
	for _, rm := range snap.Messages {
		if rm.ChannelID != snap.ActiveChannelID {
			continue
		}
		mv := toMsg(rm)
		if rm.ThreadRootID == "" {
			msgs = append(msgs, mv)
			if rm.Pinned {
				pinnedMsgs = append(pinnedMsgs, mv)
			}
		} else if snap.ActiveThreadID != "" && rm.ThreadRootID == snap.ActiveThreadID {
			threadMsgs = append(threadMsgs, mv)
		}
	}
	return RoomFullSnapshot{
		Available:       true,
		ActiveChannelID: snap.ActiveChannelID,
		ActiveThreadID:  snap.ActiveThreadID,
		CurrentActorID:  snap.CurrentActorID,
		ThinkingBots:    thinkingBots,
		Channels:        chans,
		Participants:    parts,
		Messages:        msgs,
		PinnedMessages:  pinnedMsgs,
		ThreadMessages:  threadMsgs,
		Tasks:           m.listTasks(),
	}
}

func (m *bentoChatRoomModule) selectChannel(channelID string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	if strings.HasPrefix(channelID, "dm:") {
		botID := strings.TrimPrefix(channelID, "dm:")
		snap := m.inner.View.Ctrl.Snapshot()
		var bot *chatroom.Participant
		for i := range snap.Participants {
			if snap.Participants[i].ID == botID && snap.Participants[i].Kind == chatroom.ParticipantAgent {
				bot = &snap.Participants[i]
				break
			}
		}
		if bot == nil {
			return chatroom.ErrChannelNotFound
		}
		m.dmMu.Lock()
		m.activeDMID = channelID
		m.activeDMThread = ""
		if len(m.dmMsgs[channelID]) == 0 {
			m.dmSeq++
			m.dmMsgs[channelID] = []dmMessage{{
				id:        fmt.Sprintf("dm-msg-%d", m.dmSeq),
				channelID: channelID,
				authorID:  bot.ID,
				body:      fmt.Sprintf("Direct 1:1 channel with **%s** (%s). Every message here routes directly to `%s`.", bot.DisplayName, bot.RoleTitle, bot.Handle),
				reactions: map[string][]string{},
				createdAt: time.Now().UTC(),
			}}
		}
		m.dmMu.Unlock()
		return nil
	}
	m.dmMu.Lock()
	m.activeDMID = ""
	m.activeDMThread = ""
	m.dmMu.Unlock()
	return m.inner.View.Ctrl.SelectChannel(channelID)
}

func (m *bentoChatRoomModule) postMessage(channelID, body, threadRootID string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	txt := strings.TrimSpace(body)
	if txt == "" {
		return chatroom.ErrEmptyContent
	}

	m.dmMu.Lock()
	activeDM := m.activeDMID
	if channelID == "" && activeDM != "" {
		channelID = activeDM
	}
	if strings.HasPrefix(channelID, "dm:") {
		m.dmSeq++
		m.dmMsgs[channelID] = append(m.dmMsgs[channelID], dmMessage{
			id:           fmt.Sprintf("dm-msg-%d", m.dmSeq),
			channelID:    channelID,
			threadRootID: threadRootID,
			authorID:     snap.CurrentActorID,
			body:         txt,
			mentions:     extractRoomMentions(txt),
			reactions:    map[string][]string{},
			createdAt:    time.Now().UTC(),
		})
		m.dmMu.Unlock()
		// 1:1 Direct Bot rule: every human message auto-routes directly to that bot.
		botID := strings.TrimPrefix(channelID, "dm:")
		for _, p := range snap.Participants {
			if p.ID == botID && p.Kind == chatroom.ParticipantAgent {
				m.dispatchBotReplies(channelID, p.DisplayName, true, threadRootID, txt, []chatroom.Participant{p}, snap)
				break
			}
		}
		return nil
	}
	m.dmMu.Unlock()

	if channelID == "" {
		channelID = snap.ActiveChannelID
	}
	if _, err := ctrl.PostMessage(channelID, snap.CurrentActorID, txt, threadRootID); err != nil {
		return err
	}

	// Group channel routing rule (Anti-Loop Guardrail: human-authored only,
	// explicit @mention triggers targeted bots, capped at 2 bots per turn).
	mentions := extractRoomMentions(txt)
	if len(mentions) > 0 {
		chName := channelID
		for _, ch := range snap.Channels {
			if ch.ID == channelID {
				chName = cleanRoomChannelName(ch.Name)
				break
			}
		}
		var targets []chatroom.Participant
		seen := make(map[string]struct{})
		for _, mn := range mentions {
			want := strings.ToLower(mn)
			for _, p := range snap.Participants {
				if p.Kind != chatroom.ParticipantAgent {
					continue
				}
				if strings.ToLower(cleanRoomHandle(p.Handle)) == want || strings.ToLower(p.DisplayName) == want {
					if _, dup := seen[p.ID]; !dup {
						seen[p.ID] = struct{}{}
						targets = append(targets, p)
					}
				}
			}
			if len(targets) >= 2 {
				break
			}
		}
		if len(targets) > 0 {
			m.dispatchBotReplies(channelID, chName, false, threadRootID, txt, targets, snap)
		}
	}
	return nil
}

func (m *bentoChatRoomModule) dispatchBotReplies(
	channelID, chanName string,
	isDM bool,
	threadRootID, userPrompt string,
	targets []chatroom.Participant,
	snap chatroom.RoomSnapshot,
) {
	responder := getRoomAgentResponder()
	if responder == nil {
		return
	}
	// Collect up to 4 recent channel lines for compact shared context.
	var ctxLines []string
	for i := len(snap.Messages) - 1; i >= 0 && len(ctxLines) < 4; i-- {
		rm := snap.Messages[i]
		if rm.ChannelID == channelID && rm.ThreadRootID == threadRootID {
			ctxLines = append([]string{fmt.Sprintf("[%s]: %s", rm.AuthorID, rm.Body)}, ctxLines...)
		}
	}
	recentCtx := strings.Join(ctxLines, "\n")

	for _, bot := range targets {
		b := bot
		m.dmMu.Lock()
		m.presenceOverrides[b.ID] = "thinking"
		m.dmMu.Unlock()
		notifyRoomInvalidated()

		m.botScope.Go(func(parentCtx context.Context) {
			ctx, cancel := context.WithTimeout(parentCtx, 45*time.Second)
			defer cancel()
			reply, err := responder(ctx, RoomAgentRequest{
				ChannelID:     channelID,
				ChannelName:   chanName,
				IsDM:          isDM,
				ThreadRootID:  threadRootID,
				BotID:         b.ID,
				BotHandle:     cleanRoomHandle(b.Handle),
				BotName:       b.DisplayName,
				BotRole:       b.RoleTitle,
				UserPrompt:    userPrompt,
				RecentContext: recentCtx,
			})
			m.dmMu.Lock()
			delete(m.presenceOverrides, b.ID)
			m.dmMu.Unlock()

			body := strings.TrimSpace(reply)
			if err != nil {
				body = fmt.Sprintf("_%s could not reach Marlin Core:_ `%v`", b.DisplayName, err)
			}
			if body != "" {
				m.appendBotReply(channelID, b.ID, body, threadRootID)
			}
			notifyRoomInvalidated()
		})
	}
}

func (m *bentoChatRoomModule) appendBotReply(channelID, botID, body, threadRootID string) {
	if strings.HasPrefix(channelID, "dm:") {
		m.dmMu.Lock()
		m.dmSeq++
		m.dmMsgs[channelID] = append(m.dmMsgs[channelID], dmMessage{
			id:           fmt.Sprintf("dm-msg-%d", m.dmSeq),
			channelID:    channelID,
			threadRootID: threadRootID,
			authorID:     botID,
			body:         body,
			mentions:     extractRoomMentions(body),
			reactions:    map[string][]string{},
			createdAt:    time.Now().UTC(),
		})
		m.dmMu.Unlock()
		return
	}
	if m.inner != nil && m.inner.View != nil && m.inner.View.Ctrl != nil {
		_, _ = m.inner.View.Ctrl.PostMessage(channelID, botID, body, threadRootID)
	}
}

func (m *bentoChatRoomModule) postGateEmbed(channelID, botID, body, gateID string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	snap := ctrl.Snapshot()
	if channelID == "" {
		channelID = snap.ActiveChannelID
	}
	if botID == "" {
		botID = "agent-security"
	}
	if strings.HasPrefix(channelID, "dm:") {
		m.dmMu.Lock()
		m.dmSeq++
		m.dmMsgs[channelID] = append(m.dmMsgs[channelID], dmMessage{
			id:        fmt.Sprintf("dm-msg-%d", m.dmSeq),
			channelID: channelID,
			authorID:  botID,
			body:      strings.TrimSpace(body),
			gateID:    gateID,
			reactions: map[string][]string{},
			createdAt: time.Now().UTC(),
		})
		m.dmMu.Unlock()
		notifyRoomInvalidated()
		return nil
	}
	m.dmMu.Lock()
	m.gateEventSeq++
	seq := m.gateEventSeq
	m.dmMu.Unlock()
	ctrl.IngestRemoteMessage(fmt.Sprintf("gate-embed-%s-%d", gateID, seq), seq, chatroom.RoomMessage{
		ID:        fmt.Sprintf("msg-gate-%s-%d", gateID, seq),
		ChannelID: channelID,
		AuthorID:  botID,
		Body:      strings.TrimSpace(body),
		GateID:    gateID,
		Reactions: map[string][]string{},
		CreatedAt: time.Now().UTC(),
	})
	notifyRoomInvalidated()
	return nil
}

func (m *bentoChatRoomModule) toggleReaction(msgID, tag string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	ctrl := m.inner.View.Ctrl
	actorID := ctrl.Snapshot().CurrentActorID
	if strings.HasPrefix(msgID, "dm-msg-") {
		m.dmMu.Lock()
		defer m.dmMu.Unlock()
		for chID, list := range m.dmMsgs {
			for i := range list {
				if list[i].id != msgID {
					continue
				}
				if list[i].reactions == nil {
					list[i].reactions = map[string][]string{}
				}
				actors := list[i].reactions[tag]
				removed := false
				next := make([]string, 0, len(actors)+1)
				for _, who := range actors {
					if who == actorID {
						removed = true
						continue
					}
					next = append(next, who)
				}
				if !removed {
					next = append(next, actorID)
				}
				list[i].reactions[tag] = next
				m.dmMsgs[chID] = list
				return nil
			}
		}
		return chatroom.ErrMessageNotFound
	}
	return ctrl.ToggleReaction(msgID, actorID, tag)
}

func (m *bentoChatRoomModule) openThread(msgID string) error {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return nil
	}
	if strings.HasPrefix(msgID, "dm-msg-") {
		m.dmMu.Lock()
		m.activeDMThread = msgID
		m.dmMu.Unlock()
		return nil
	}
	return m.inner.View.Ctrl.OpenThread(msgID)
}

func (m *bentoChatRoomModule) closeThread() {
	if m == nil || m.inner == nil || m.inner.View == nil || m.inner.View.Ctrl == nil {
		return
	}
	m.dmMu.Lock()
	m.activeDMThread = ""
	m.dmMu.Unlock()
	m.inner.View.Ctrl.CloseThread()
}
