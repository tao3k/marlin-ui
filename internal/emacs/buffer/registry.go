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
	Sessions     ID = "*Sessions*"
	Targets      ID = "*Targets*"
	Processes    ID = "*Processes*"
	Models       ID = "*Models*"
	Messages     ID = "*Messages*"
	Help         ID = "*Help*"
	Settings     ID = "*Settings*"
	Packages     ID = "*Packages*"
)

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
		{ID: Sessions, Description: "Recorded sessions table"},
		{ID: Targets, Description: "Federation Rust gRPC targets table"},
		{ID: Processes, Description: "Managed runtime processes"},
		{ID: Models, Description: "Configured model routes"},
		{ID: Packages, Description: "Official Marketplace package manager (themes, plugins, layouts)"},
		{ID: Messages, Description: "UI diagnostics log"},
		{ID: Help, Description: "Meow & Doom keyboard shortcuts"},
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

// Context carries the live data and callbacks needed to render any Emacs buffer.
type Context struct {
	Adaptive      *mygoshell.AdaptiveShell
	Link          *hostlink.Link
	FeatureReg    *features.Registry
	FeatureLedger *features.GovernanceLedger
	FeatureHost   *features.Host
	WebHost       kit.WebIslandHost
	ActiveModel   string
	TableCursor   int
	MessagesLog     []string
	AvailableThemes []theme.Theme
	DashboardNode   *widgets.Node
	Signals         *widgets.Signals
	WidgetRuntime   *widgets.Runtime
	Modules         widgets.Modules
	GlassOpacity    float32
	ZenMode         bool
	Customize       *CustomizeState
	PackageSnap     marketplace.PackageStoreSnapshot
	PackageFilter   string

	OnSelectBuffer      func(ID)
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
}

// Render dispatches rendering to the active buffer's major mode.
func Render(c *ui.Context, id ID, th theme.Theme, bc *Context) {
	switch id {
	case Dashboard:
		renderDashboard(c, th, bc)
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
		renderHelp(c, th)
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
