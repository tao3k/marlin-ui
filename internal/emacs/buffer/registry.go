// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package buffer owns Marlin UI's categorized Emacs buffer catalog and
// major-mode renderers (*conversation*, tabulated-list-mode buffers, and the
// *Settings* Emacs customize buffer).
//
// Declared in dev/sysml/emacs-workbench.sysml as MUI-EMACS-A06 (emacsBuffer).
package buffer

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

// ID identifies one Marlin Emacs buffer (matching scheme/ui-components/buffers.ss).
type ID string

const (
	Dashboard    ID = "*Dashboard*"
	Conversation ID = "*conversation*"
	Magit        ID = "*magit*"
	Sessions     ID = "*Sessions*"
	Targets      ID = "*Targets*"
	Processes    ID = "*Processes*"
	Models       ID = "*Models*"
	Messages     ID = "*Messages*"
	Help         ID = "*Help*"
	Ibuffer      ID = "*Ibuffer*"
	Settings     ID = "*Settings*"
	Packages     ID = "*Packages*"
	Preview      ID = "*Preview*"
)

// IsChatBuffer reports whether id is a marlin-chat-mode buffer (*conversation*
// or any multi-instance LLM conversation buffer *chat:<name>*).
func IsChatBuffer(id ID) bool {
	return id == Conversation || strings.HasPrefix(string(id), "*chat:")
}

// NewChatBufferID constructs a canonical multi-instance LLM chat buffer ID
// (e.g. "*chat:auth-fix*") governed by `marlin-chat-mode` + `poly-marlin-chat-mode`.
func NewChatBufferID(name string) ID {
	clean := strings.TrimSpace(strings.Trim(name, "*"))
	if clean == "" {
		return Conversation
	}
	clean = strings.TrimPrefix(clean, "chat:")
	return ID("*chat:" + clean + "*")
}

// Descriptor describes a registered built-in Emacs buffer for switch-buffer completion.
type Descriptor struct {
	ID          ID
	Description string
}

// Catalog returns all built-in Emacs buffers in canonical order.
func Catalog() []Descriptor {
	return []Descriptor{
		{ID: Dashboard, Description: "EWW Bento widget desktop dashboard"},
		{ID: Conversation, Description: "Conversation buffer"},
		{ID: Magit, Description: "AI-Native Emacs Magit (jj + Git + Aitia code audit)"},
		{ID: Sessions, Description: "Recorded sessions table"},
		{ID: Targets, Description: "Federation Rust gRPC targets table"},
		{ID: Processes, Description: "Managed runtime processes"},
		{ID: Models, Description: "Configured model routes"},
		{ID: Packages, Description: "Official Marketplace package manager (themes, plugins, layouts)"},
		{ID: Messages, Description: "UI diagnostics log"},
		{ID: Help, Description: "Live Emacs describe-mode & keybindings introspection"},
		{ID: Ibuffer, Description: "Interactive buffer manager grouped by Major Mode (C-c b / SPC b i)"},
		{ID: Preview, Description: "Read-only source file preview popup"},
		{ID: Settings, Description: "Customize appearance, remotes, account, features"},
	}
}

// CustomizeState holds mutable working fields for the *Settings* customize buffer.
type CustomizeState struct {
	SessionID         string
	RemoteHost        string
	RealSession       bool
	NewRemoteID       string
	NewRemoteEndpoint string
	NewRemoteToken    string
	OIDCIssuer        string
	OIDCClientID      string
}

// ArtifactSelection identifies the active Vercel AI SDK Generative UI part
// displayed in *conversation*'s right-hand Layer 3 Artifact Split Stage.
type ArtifactSelection struct {
	DataType string
	Title    string
	Payload  string
}

// HelpIntrospectionData carries live Emacs `describe-mode` (C-h m) reflection
// data for the buffer that was focused when *Help* was invoked.
type HelpIntrospectionData struct {
	BufferName    string
	MajorModeID   string
	MajorTitle    string
	ParentChain   []string
	MinorLighters []string
	FocusedSpan   string
	InnerModeID   string
	DisplayAction string
	ReadOnly      bool
	LocalBindings [][3]string // [tier, key, command + label]
	PolymodeRules [][3]string // [span-kind, inner-mode, lighter]
}

// IbufferEntry describes one open buffer row in *Ibuffer* grouped by Major Mode.
type IbufferEntry struct {
	BufferID  ID
	MajorMode string
	InnerMode string
	Workspace string
	WindowNum int
	ReadOnly  bool
	Active    bool
}

// Context carries the live data and callbacks needed to render any Emacs buffer.
type Context struct {
	Adaptive       *mygoshell.AdaptiveShell
	Link           *hostlink.Link
	FeatureReg     *features.Registry
	FeatureLedger  *features.GovernanceLedger
	FeatureHost    *features.Host
	WebHost        kit.WebIslandHost
	ModelCfg       appconfig.ModelConfig
	ActiveModel    string
	ActiveArtifact *ArtifactSelection
	FilePreview    *FilePreviewState
	ComposerDraft  *string
	TableCursor    int
	MessagesLog     []string
	AvailableThemes []theme.Theme
	DashboardNode   *widgets.Node
	Signals         *widgets.Signals
	WidgetRuntime   *widgets.Runtime
	Modules         widgets.Modules
	GlassOpacity     float32
	ZenMode          bool
	ComposerExpanded bool
	ComposerPinned   bool
	Customize        *CustomizeState
	PackageSnap     marketplace.PackageStoreSnapshot
	PackageFilter       string
	Magit               *MagitViewState
	HelpIntrospection   *HelpIntrospectionData
	IbufferRows         []IbufferEntry
	ActivePolymodeSpan  string

	OnSelectBuffer      func(ID)
	OnKillBuffer        func(ID)
	OnFocusPolymodeSpan func(spanKind, lang, payload string)
	OnSetTheme          func(themeID string)
	OnReloadThemes      func()
	OnToggleGlass       func()
	OnCycleGlassOpacity func()
	OnToggleMotion      func()
	OnToggleZenMode    func()
	OnWidgetAction     func(action string)
	OnOpenCommands     func()
	OnOpenHelm         func()
	OnOpenBufferSwitch func()
	OnOpenThemeSwitch  func()
	OnOpenSettings     func()
	OnPackageFilter    func(kind string)
	OnPackageRefresh   func()
	OnPackageInstall   func(id string)
	OnPackageUpdate    func(id string)
	OnPackageRemove    func(id string)
	OnPackageApply     func(id string)
	OnSavePrefs        func(sessionID, remoteHost string, realSession bool)
	OnEcho             func(msg string)
	IsThinkingOpen     func(key string) bool
	ToggleThinking     func(key string)
	OnOpenArtifact     func(dataType, title, payload string)
	OnCloseArtifact    func()
	OnSubmitPrompt     func(text string)
	OnSelectModel      func(alias, displayLabel string)
	OnToggleDeepThink  func()
	OnCiteInComposer    func(ref string)
	OnOpenFilePreview   func(title, path, snippet string, line int)
	OnComposerHover     func(hovered bool)
	OnComposerFocus     func(focused bool)
	OnToggleComposerPin func()
	OnMagitAction       func(action, target string)
}

// Render dispatches rendering to the active buffer's major mode.
func Render(c *ui.Context, id ID, th theme.Theme, bc *Context) {
	switch id {
	case Dashboard:
		renderDashboard(c, th, bc)
	case Magit:
		renderMagit(c, th, bc)
	case Sessions:
		renderSessions(c, th, bc)
	case Targets:
		renderTargets(c, th, bc)
	case Processes:
		renderProcesses(c, th, bc)
	case Models:
		renderModels(c, th, bc)
	case Packages:
		renderPackages(c, th, bc)
	case Messages:
		renderMessages(c, th, bc)
	case Help:
		renderHelp(c, th, bc)
	case Ibuffer:
		renderIbuffer(c, th, bc)
	case Preview:
		renderFilePreview(c, th, bc)
	case Settings:
		renderCustomize(c, th, bc)
	default:
		renderConversation(c, th, bc)
	}
}

// NewCustomizeState initializes *Settings* buffer inputs from Config and UserPrefs.
func NewCustomizeState(cfg appconfig.Config, prefs appconfig.UserPrefs) *CustomizeState {
	realSess := cfg.RealSession
	if prefs.RealSession != nil {
		realSess = *prefs.RealSession
	}
	remoteHost := cfg.HostAddr
	if remoteHost == "" {
		remoteHost = prefs.DefaultRemoteHost
	}
	return &CustomizeState{
		SessionID:   cfg.SessionID,
		RemoteHost:  remoteHost,
		RealSession: realSess,
	}
}
