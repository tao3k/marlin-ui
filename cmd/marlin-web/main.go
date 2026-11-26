// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build js && wasm

// Command marlin-web compiles Marlin UI's AdaptiveShell (Desktop Emacs
// WindowTree + Mobile Phone Companion + 120Hz Helm Search + compiled featurekit)
// into a static WebAssembly bundle (`GOOS=js GOARCH=wasm`).
package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/auth"
	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
)

func main() {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		BrandSubtitle:  "WebAssembly · Desktop & Phone Adaptive",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorAuto,
	})
	adaptive.Desktop.ActiveSessionID = "marlin-web-main"
	adaptive.Desktop.Sessions = []mygoshell.SessionSummaryView{
		{SessionID: "marlin-web-main", Title: "Web Session", TurnCount: 0},
	}
	_ = adaptive.Desktop.EnableEmacsWindowKeymap()
	mygoshell.BindDefaultHelmSources(adaptive.Desktop, nil)

	continuity := initWebContinuity(adaptive)
	var winRef *mygo.Window
	invalidate := func() {
		if winRef != nil {
			winRef.Invalidate()
		}
	}
	reg, ledger, host := featurekit.Mount("all", adaptive, continuity.onDraftChanged, invalidate)

	// Bind Tier-0 Settings & ephemeral Web Auth Center so Web WASM shares the
	// same bottom-left Settings & Authorization dock as Desktop and Mobile.
	webMgr := auth.NewManager(auth.ManagerConfig{Vault: auth.NewMemoryVault()})
	_ = webMgr.RegisterProvider(auth.GitHubOAuthProvider("marlin-web-github"))
	_ = webMgr.RegisterProvider(auth.GoogleOAuthProvider("marlin-web-google"))
	_ = webMgr.RegisterProvider(auth.EmailOTPProvider())
	webAuth := mygoshell.NewAuthController(webMgr)
	webAuth.VaultInfo = auth.VaultSecurityInfo{
		BackendName: "Web Session Memory Vault",
		CipherSuite: "AES-256-GCM",
	}
	webAuth.OnCopyText = continuity.copyText
	webAuth.OnChanged = invalidate
	adaptive.SetAuth(webAuth)
	adaptive.Desktop.Transcript.OnCopyCode = continuity.copyText
	if adaptive.Desktop.Projection != nil {
		adaptive.Desktop.Projection.OnCopyText = continuity.copyText
	}

	var webPrefs appconfig.UserPrefs
	if raw := continuity.loadPrefsJSON(); raw != "" {
		_ = json.Unmarshal([]byte(raw), &webPrefs)
	}
	if webPrefs.ThemeID != "" {
		adaptive.ApplyThemePreset(webPrefs.ThemeID)
	}
	adaptive.Desktop.SidebarCollapsed = webPrefs.SidebarCollapsed
	if webPrefs.FeaturesSpec != "" {
		reg.Activate(features.ParseFeatureSpec(webPrefs.FeaturesSpec), ledger, host)
		adaptive.SetFeatureHost(host, adaptive.Desktop.GovernanceUI)
	}

	spec := appconfig.SettingsSchema()
	initTheme := webPrefs.ThemeID
	if initTheme == "" {
		initTheme = "midnight"
	}
	initFeat := webPrefs.FeaturesSpec
	if initFeat == "" {
		initFeat = "all"
	}
	formState := mygoshell.NewSchemaFormState(spec, map[string]any{
		"theme_id":          initTheme,
		"features_spec":     initFeat,
		"default_session":   "marlin-web-main",
		"remote_host":       webPrefs.DefaultRemoteHost,
		"real_session":      true,
		"sidebar_collapsed": webPrefs.SidebarCollapsed,
	})
	adaptive.SetSettings(&spec, formState, func(vals map[string]any) {
		appconfig.ApplySettingsValues(&webPrefs, vals)
		if data, err := json.Marshal(webPrefs); err == nil {
			continuity.savePrefsJSON(string(data))
		}
		if themeID, ok := vals["theme_id"].(string); ok && strings.TrimSpace(themeID) != "" {
			adaptive.ApplyThemePreset(strings.TrimSpace(themeID))
		}
		if collapsed, ok := vals["sidebar_collapsed"].(bool); ok {
			adaptive.Desktop.SidebarCollapsed = collapsed
		}
		if feat, ok := vals["features_spec"].(string); ok && strings.TrimSpace(feat) != "" {
			reg.Activate(features.ParseFeatureSpec(strings.TrimSpace(feat)), ledger, host)
			adaptive.SetFeatureHost(host, adaptive.Desktop.GovernanceUI)
		}
		invalidate()
	})

	adaptive.Desktop.Transcript.OnSubmitTurn = func(sub composer.TurnSubmission) {
		refs := make([]composer.AttachmentRef, 0, len(sub.Attachments))
		for _, att := range sub.Attachments {
			refs = append(refs, att.Ref)
		}
		adaptive.Desktop.Transcript.AppendUserWithAttachments(sub.Prompt, refs)
		invalidate()
	}

	mygo.App.WhenReady(func() {
		winRef = mygo.NewWindow(mygo.WindowOptions{
			Title:   "Marlin UI — Web",
			Width:   1280,
			Height:  820,
			Content: ui.View(adaptive.View),
		})
		continuity.attachWindow(winRef)
		winRef.Invalidate()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
