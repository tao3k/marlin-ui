// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package appconfig

import (
	"strings"

	"github.com/tao3k/marlin-mygo/pkg/schema"
)

// SettingsSchema returns the typed, bounded SSOT preferences schema rendered
// natively in Marlin UI's Tier-0 Settings & Authorization Center across
// Desktop, Mobile, and Web WASM.
func SettingsSchema() schema.BoundedSchema {
	return schema.BoundedSchema{
		ID:    "marlin.ui.preferences.v1",
		Title: "Marlin Workspace Preferences",
		Fields: []schema.FieldSpec{
			{
				Key:        "theme_id",
				Label:      "Theme Preset",
				Kind:       schema.FieldEnum,
				Required:   true,
				EnumValues: []string{"midnight", "paper", "doom-one", "tokyo-night", "catppuccin"},
				Default:    "midnight",
			},
			{
				Key:        "features_spec",
				Label:      "UI Feature Profile",
				Kind:       schema.FieldEnum,
				Required:   true,
				EnumValues: []string{"default", "all", "none", "chatroom", "chatroom,compare", "chatroom,canvas"},
				Default:    "default",
			},
			{
				Key:      "default_session",
				Label:    "Active Session ID",
				Kind:     schema.FieldString,
				Required: true,
				MinLen:   1,
				MaxLen:   64,
				Default:  "marlin-main",
			},
			{
				Key:     "remote_host",
				Label:   "Default Remote gRPC Host (optional)",
				Kind:    schema.FieldString,
				MaxLen:  128,
				Default: "",
			},
			{
				Key:     "real_session",
				Label:   "Durable ConversationJournal Streaming",
				Kind:    schema.FieldBool,
				Default: true,
			},
			{
				Key:     "sidebar_collapsed",
				Label:   "Compact Mode (Collapse Left Sidebar)",
				Kind:    schema.FieldBool,
				Default: false,
			},
		},
	}
}

// InitialSettingsValues projects runtime Config + persisted UserPrefs into the
// initial values map for SettingsSchema().
func InitialSettingsValues(cfg Config, prefs UserPrefs) map[string]any {
	theme := strings.TrimSpace(cfg.ThemeID)
	if theme == "" {
		theme = "midnight"
	}
	feat := normalizeFeaturesPreset(cfg.FeaturesSpec)
	sess := strings.TrimSpace(cfg.SessionID)
	if sess == "" {
		sess = "marlin-main"
	}
	remote := strings.TrimSpace(cfg.HostAddr)
	if remote == "" {
		remote = strings.TrimSpace(prefs.DefaultRemoteHost)
	}
	realSess := cfg.RealSession
	if prefs.RealSession != nil {
		realSess = *prefs.RealSession
	}
	return map[string]any{
		"theme_id":          theme,
		"features_spec":     feat,
		"default_session":   sess,
		"remote_host":       remote,
		"real_session":      realSess,
		"sidebar_collapsed": prefs.SidebarCollapsed,
	}
}

// ApplySettingsValues merges validated form values back into UserPrefs.
func ApplySettingsValues(prefs *UserPrefs, vals map[string]any) {
	if prefs == nil || vals == nil {
		return
	}
	if v, ok := vals["theme_id"].(string); ok && strings.TrimSpace(v) != "" {
		prefs.ThemeID = strings.TrimSpace(v)
	}
	if v, ok := vals["features_spec"].(string); ok && strings.TrimSpace(v) != "" {
		prefs.FeaturesSpec = strings.TrimSpace(v)
	}
	if v, ok := vals["default_session"].(string); ok && strings.TrimSpace(v) != "" {
		prefs.LastSessionID = strings.TrimSpace(v)
	}
	if v, ok := vals["remote_host"].(string); ok {
		prefs.DefaultRemoteHost = strings.TrimSpace(v)
	}
	if v, ok := vals["real_session"].(bool); ok {
		b := v
		prefs.RealSession = &b
	}
	if v, ok := vals["sidebar_collapsed"].(bool); ok {
		prefs.SidebarCollapsed = v
	}
}

func normalizeFeaturesPreset(spec string) string {
	switch strings.TrimSpace(spec) {
	case "default", "all", "none", "chatroom", "chatroom,compare", "chatroom,canvas":
		return strings.TrimSpace(spec)
	default:
		return "default"
	}
}
