// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

// CoveHealth mirrors marlin-cove's GET /v1/health response.
type CoveHealth struct {
	Status   string   `json:"status"`
	Service  string   `json:"service"`
	Features []string `json:"features"`
}

// CoveChannel mirrors marlin-cove's RoomChannel wire model.
type CoveChannel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	IsDirect bool   `json:"is_direct"`
}

// CoveParticipant mirrors marlin-cove's RoomParticipant wire model.
type CoveParticipant struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Handle      string   `json:"handle"`
	Kind        string   `json:"kind"` // "human" | "agent"
	Role        string   `json:"role"` // "owner" | "maintainer" | "member" | "external_guest" | "role_agent"
	Scopes      []string `json:"scopes"`
	Presence    string   `json:"presence"`
}

// CoveMessage mirrors marlin-cove's RoomMessage wire model.
type CoveMessage struct {
	ID            string   `json:"id"`
	RoomID        string   `json:"room_id"`
	ChannelID     string   `json:"channel_id"`
	ThreadRootID  string   `json:"thread_root_id,omitempty"`
	AuthorID      string   `json:"author_id"`
	AuthorKind    string   `json:"author_kind"`
	Body          string   `json:"body"`
	Mentions      []string `json:"mentions"`
	GateID        string   `json:"gate_id,omitempty"`
	Origin        string   `json:"origin"`
	BotChainDepth uint8    `json:"bot_chain_depth"`
	CreatedAtUnix int64    `json:"created_at_unix"`
}

// CoveGate mirrors marlin-cove's RoomGate wire model.
type CoveGate struct {
	ID          string `json:"id"`
	RoomID      string `json:"room_id"`
	ChannelID   string `json:"channel_id"`
	RequestedBy string `json:"requested_by"`
	Summary     string `json:"summary"`
	Capability  string `json:"capability"`
	Decision    string `json:"decision"`
}

// CoveRoomSnapshot mirrors marlin-cove's GET /v1/rooms/{room_id}/snapshot response.
type CoveRoomSnapshot struct {
	RoomID       string            `json:"room_id"`
	Title        string            `json:"title"`
	LatestSeq    uint64            `json:"latest_seq"`
	Channels     []CoveChannel     `json:"channels"`
	Participants []CoveParticipant `json:"participants"`
	Messages     []CoveMessage     `json:"messages"`
	Gates        []CoveGate        `json:"gates"`
}

// CoveInvite mirrors marlin-cove's RoomInvite wire model.
type CoveInvite struct {
	Token       string   `json:"token"`
	RoomID      string   `json:"room_id"`
	IssuedBy    string   `json:"issued_by"`
	GrantRole   string   `json:"grant_role"`
	GrantScopes []string `json:"grant_scopes"`
	MaxUses     uint32   `json:"max_uses"`
	Uses        uint32   `json:"uses"`
}

// CoveGovernanceEnvelope mirrors marlin-cove's verified room/people/bot
// governance envelope forwarded to Marlin Gateway (marlin-agent-core) so each
// agent can run its own atomic .org context governance.
type CoveGovernanceEnvelope struct {
	RoomID          string   `json:"room_id"`
	ChannelID       string   `json:"channel_id"`
	ActorID         string   `json:"actor_id"`
	ActorKind       string   `json:"actor_kind"`
	ActorRole       string   `json:"actor_role"`
	GrantedScopes   []string `json:"granted_scopes"`
	Origin          string   `json:"origin"`
	TaintedExternal bool     `json:"tainted_external"`
}

// CoveClient speaks the typed HTTP contract exposed by a Marlin Cove relay server.
type CoveClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewCoveClient builds a Marlin Cove relay client for the given base URL.
func NewCoveClient(baseURL, bearerToken string) *CoveClient {
	return &CoveClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(bearerToken),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NewCoveClientFromConfig returns a configured CoveClient when MARLIN_COVE_URL
// is set, or nil when Marlin UI runs without an external Cove relay.
func NewCoveClientFromConfig(cfg appconfig.Config) *CoveClient {
	if strings.TrimSpace(cfg.CoveURL) == "" {
		return nil
	}
	return NewCoveClient(cfg.CoveURL, cfg.CoveToken)
}

// Health probes GET /v1/health and returns enabled Cove server features.
func (c *CoveClient) Health(ctx context.Context) (CoveHealth, error) {
	var out CoveHealth
	err := c.doJSON(ctx, http.MethodGet, "/v1/health", nil, &out)
	return out, err
}

// FetchSnapshot fetches GET /v1/rooms/{room_id}/snapshot.
func (c *CoveClient) FetchSnapshot(ctx context.Context, roomID string) (CoveRoomSnapshot, error) {
	var out CoveRoomSnapshot
	path := fmt.Sprintf("/v1/rooms/%s/snapshot", url.PathEscape(roomID))
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// CreateInvite issues a scoped invite token via POST /v1/rooms/{room_id}/invites.
func (c *CoveClient) CreateInvite(ctx context.Context, roomID, issuedBy, grantRole string, scopes []string, maxUses uint32) (CoveInvite, error) {
	var out CoveInvite
	path := fmt.Sprintf("/v1/rooms/%s/invites", url.PathEscape(roomID))
	payload := map[string]any{
		"issued_by":    issuedBy,
		"grant_role":   grantRole,
		"grant_scopes": scopes,
		"max_uses":     maxUses,
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// PostMessage posts a room message via POST /v1/rooms/{room_id}/messages.
func (c *CoveClient) PostMessage(ctx context.Context, roomID, channelID, authorID, body, threadRootID string) (CoveMessage, error) {
	var out CoveMessage
	path := fmt.Sprintf("/v1/rooms/%s/messages", url.PathEscape(roomID))
	payload := map[string]any{
		"channel_id": channelID,
		"author_id":  authorID,
		"body":       body,
		"origin":     "cove_native",
	}
	if strings.TrimSpace(threadRootID) != "" {
		payload["thread_root_id"] = threadRootID
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// CoveDispatchResponse mirrors marlin-cove's POST /v1/rooms/{room_id}/dispatch response.
type CoveDispatchResponse struct {
	UserMessage CoveMessage            `json:"user_message"`
	Governance  CoveGovernanceEnvelope `json:"governance"`
	AgentReply  CoveMessage            `json:"agent_reply"`
}

// CoveGateReceipt mirrors marlin-cove's GateApprovalReceipt wire model.
type CoveGateReceipt struct {
	GateID         string `json:"gate_id"`
	ResolvedBy     string `json:"resolved_by"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason"`
	ResolvedAtUnix int64  `json:"resolved_at_unix"`
}

// CoveRoomEvent mirrors marlin-cove's monotonic SSE RoomEvent envelope.
type CoveRoomEvent struct {
	Seq    uint64            `json:"seq"`
	RoomID string            `json:"room_id"`
	Kind   CoveRoomEventKind `json:"kind"`
}

// CoveRoomEventKind mirrors marlin-cove's tagged RoomEventKind payload.
type CoveRoomEventKind struct {
	Type        string           `json:"type"`
	Message     *CoveMessage     `json:"message,omitempty"`
	Participant *CoveParticipant `json:"participant,omitempty"`
	Gate        *CoveGate        `json:"gate,omitempty"`
	Receipt     *CoveGateReceipt `json:"receipt,omitempty"`
}

// FetchGovernanceEnvelope fetches the verified room governance envelope that
// Marlin Cove forwards to Marlin Gateway (marlin-agent-core).
func (c *CoveClient) FetchGovernanceEnvelope(ctx context.Context, roomID, principalID string) (CoveGovernanceEnvelope, error) {
	var out CoveGovernanceEnvelope
	path := fmt.Sprintf("/v1/rooms/%s/governance/%s", url.PathEscape(roomID), url.PathEscape(principalID))
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// RedeemInvite redeems a scoped invite token via POST /v1/invites/{token}/join.
func (c *CoveClient) RedeemInvite(ctx context.Context, token, principalID, displayName, handle, kind string) (CoveParticipant, error) {
	var out CoveParticipant
	path := fmt.Sprintf("/v1/invites/%s/join", url.PathEscape(token))
	payload := map[string]any{
		"principal_id": principalID,
		"display_name": displayName,
		"handle":       handle,
		"kind":         kind,
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// RequestGate opens a Tier-0 PromptGate via POST /v1/rooms/{room_id}/gates.
func (c *CoveClient) RequestGate(ctx context.Context, roomID, channelID, requestedBy, summary, capability string) (CoveGate, error) {
	var out CoveGate
	path := fmt.Sprintf("/v1/rooms/%s/gates", url.PathEscape(roomID))
	payload := map[string]any{
		"channel_id":   channelID,
		"requested_by": requestedBy,
		"summary":      summary,
		"capability":   capability,
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// WatchEvents subscribes to GET /v1/rooms/{room_id}/events (SSE) and invokes
// onEvent for every RoomEvent until ctx is canceled or the stream ends.
func (c *CoveClient) WatchEvents(ctx context.Context, roomID string, onEvent func(CoveRoomEvent)) error {
	path := fmt.Sprintf("/v1/rooms/%s/events", url.PathEscape(roomID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		if strings.HasPrefix(c.token, "Bearer ") {
			req.Header.Set("Authorization", c.token)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
	}

	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("marlin-cove SSE %s failed (%d): %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var evt CoveRoomEvent
		if err := json.Unmarshal([]byte(payload), &evt); err == nil && onEvent != nil {
			onEvent(evt)
		}
	}
	return scanner.Err()
}

// DispatchTurn posts a user turn into Cove and dispatches it live to Marlin
// Gateway (marlin-agent-core) with a verified RoomGovernanceEnvelope.
func (c *CoveClient) DispatchTurn(ctx context.Context, roomID, channelID, authorID, targetAgentHandle, body string) (CoveDispatchResponse, error) {
	var out CoveDispatchResponse
	path := fmt.Sprintf("/v1/rooms/%s/dispatch", url.PathEscape(roomID))
	payload := map[string]any{
		"channel_id":          channelID,
		"author_id":           authorID,
		"target_agent_handle": targetAgentHandle,
		"body":                body,
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// ResolveGate resolves a Tier-0 PromptGate as a human Owner/Maintainer.
func (c *CoveClient) ResolveGate(ctx context.Context, roomID, gateID, actorID string, approved bool, reason string) (CoveGateReceipt, error) {
	var out CoveGateReceipt
	path := fmt.Sprintf("/v1/rooms/%s/gates/%s/resolve", url.PathEscape(roomID), url.PathEscape(gateID))
	payload := map[string]any{
		"actor_id": actorID,
		"approved": approved,
		"reason":   reason,
	}
	err := c.doJSON(ctx, http.MethodPost, path, payload, &out)
	return out, err
}

// ToRoomFullSnapshot projects a Cove relay snapshot into marlin-ui's native
// build-tag-free RoomFullSnapshot used across Desktop, iOS, and Android.
func (s CoveRoomSnapshot) ToRoomFullSnapshot(activeChannelID, currentActorID string) featurekit.RoomFullSnapshot {
	if activeChannelID == "" && len(s.Channels) > 0 {
		activeChannelID = s.Channels[0].ID
	}
	byID := make(map[string]CoveParticipant, len(s.Participants))
	participants := make([]featurekit.RoomParticipantView, 0, len(s.Participants))
	thinking := make([]string, 0)
	for _, p := range s.Participants {
		byID[p.ID] = p
		isAgent := p.Kind == "agent"
		if isAgent && (p.Presence == "thinking" || p.Presence == "working") {
			thinking = append(thinking, p.DisplayName)
		}
		participants = append(participants, featurekit.RoomParticipantView{
			ID:              p.ID,
			Handle:          p.Handle,
			Name:            p.DisplayName,
			Role:            p.Role,
			IsAgent:         isAgent,
			HostNode:        "marlin-cove",
			Presence:        p.Presence,
			CanPost:         true,
			CanApproveGates: !isAgent && (p.Role == "owner" || p.Role == "maintainer"),
		})
	}

	channels := make([]featurekit.RoomChannelView, 0, len(s.Channels))
	for _, ch := range s.Channels {
		channels = append(channels, featurekit.RoomChannelView{
			ID:    ch.ID,
			Name:  ch.Name,
			Topic: ch.Topic,
			IsDM:  ch.IsDirect,
		})
	}

	messages := make([]featurekit.RoomMessageView, 0, len(s.Messages))
	for _, m := range s.Messages {
		if activeChannelID != "" && m.ChannelID != activeChannelID {
			continue
		}
		author := byID[m.AuthorID]
		authorName := author.DisplayName
		if authorName == "" {
			authorName = m.AuthorID
		}
		messages = append(messages, featurekit.RoomMessageView{
			ID:              m.ID,
			ChannelID:       m.ChannelID,
			AuthorID:        m.AuthorID,
			AuthorName:      authorName,
			AuthorHandle:    author.Handle,
			AuthorRole:      author.Role,
			AuthorPresence:  author.Presence,
			IsAgent:         m.AuthorKind == "agent",
			Body:            m.Body,
			Mentions:        m.Mentions,
			GateID:          m.GateID,
			TaintedExternal: m.Origin == "telegram" || m.Origin == "discord",
			Delivery:        m.Origin,
			CreatedUnix:     m.CreatedAtUnix,
		})
	}

	return featurekit.RoomFullSnapshot{
		Available:       true,
		ActiveChannelID: activeChannelID,
		CurrentActorID:  currentActorID,
		ThinkingBots:    thinking,
		Channels:        channels,
		Participants:    participants,
		Messages:        messages,
	}
}

func (c *CoveClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		if strings.HasPrefix(c.token, "Bearer ") {
			req.Header.Set("Authorization", c.token)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("marlin-cove %s %s failed (%d): %s", method, path, resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	if out != nil && len(respBytes) > 0 {
		return json.Unmarshal(respBytes, out)
	}
	return nil
}
