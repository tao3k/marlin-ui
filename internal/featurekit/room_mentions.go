// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package featurekit

import (
	"strings"
	"unicode"
)

const mentionTrimRightChars = ".,:;!?()[]{}\"'，。：；！？（）【】「」『』“”‘’、"

// ActiveMentionQuery returns the lowercase filter query when draft ends with an
// in-progress `@token` (no trailing whitespace), shared identically across
// Desktop, iOS, and Android composers (both main timeline and thread drawers).
func ActiveMentionQuery(draft string) (string, bool) {
	if draft == "" {
		return "", false
	}
	last := draft[len(draft)-1]
	if last == ' ' || last == '\n' || last == '\t' {
		return "", false
	}
	idx := strings.LastIndexAny(draft, " \n\t")
	token := draft[idx+1:]
	if !strings.HasPrefix(token, "@") {
		return "", false
	}
	return strings.ToLower(token[1:]), true
}

// ApplyMentionCompletion replaces an active trailing `@query` token (or appends
// `@handle `) cleanly for Desktop, iOS, and Android room composers.
func ApplyMentionCompletion(draft, handle string) string {
	clean := strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if clean == "" {
		return draft
	}
	if _, ok := ActiveMentionQuery(draft); ok {
		idx := strings.LastIndexAny(draft, " \n\t")
		prefix := draft[:idx+1]
		return prefix + "@" + clean + " "
	}
	trimmed := strings.TrimSpace(draft)
	if trimmed == "" {
		return "@" + clean + " "
	}
	return trimmed + " @" + clean + " "
}

// ActiveSlashQuery returns the lowercase command query when draft starts with
// `/` and has no whitespace yet.
func ActiveSlashQuery(draft string) (string, bool) {
	trimmed := strings.TrimLeft(draft, " \t")
	if !strings.HasPrefix(trimmed, "/") || strings.ContainsAny(trimmed, " \n\t") {
		return "", false
	}
	return strings.ToLower(trimmed[1:]), true
}

// ExtractRoomMentions parses unique `@handle` tokens from a room message,
// tolerating both ASCII and CJK punctuation (e.g. `@architect，`, `@architect：`)
// as well as CJK text immediately following an ASCII handle (`@architect请看`).
func ExtractRoomMentions(body string) []string {
	fields := strings.Fields(body)
	seen := make(map[string]struct{})
	var out []string
	for _, tok := range fields {
		if !strings.HasPrefix(tok, "@") || len(tok) < 2 {
			continue
		}
		raw := strings.TrimRight(tok[1:], mentionTrimRightChars)
		// Split at the first non-handle rune (e.g. Chinese text right after `@architect`).
		var b strings.Builder
		for _, r := range raw {
			if unicode.IsLetter(r) && r <= unicode.MaxASCII || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
				b.WriteRune(r)
			} else {
				break
			}
		}
		clean := strings.TrimRight(b.String(), ".")
		if clean == "" {
			clean = raw
		}
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

// MatchRoomParticipants filters room participants for the shared `@` mention
// autocomplete popover, preserving AI Agents before Humans.
func MatchRoomParticipants(participants []RoomParticipantView, query string) (agents, humans []RoomParticipantView) {
	q := strings.ToLower(strings.TrimSpace(query))
	for _, p := range participants {
		if q != "" &&
			!strings.Contains(strings.ToLower(p.Handle), q) &&
			!strings.Contains(strings.ToLower(p.Name), q) &&
			!strings.Contains(strings.ToLower(p.Role), q) {
			continue
		}
		if p.IsAgent {
			agents = append(agents, p)
		} else {
			humans = append(humans, p)
		}
	}
	return agents, humans
}
