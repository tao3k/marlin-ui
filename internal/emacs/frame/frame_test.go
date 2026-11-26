// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/input"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
	"github.com/tao3k/marlin-ui/internal/emacs/meow"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func TestDoomEmacsFrameHydraModelineAndGRPCProjection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tmp := t.TempDir()
	cfg := appconfig.Config{
		SessionID:    "marlin-main",
		ThemeID:      "doom-one",
		FeaturesSpec: "all",
		RecoveryFile: filepath.Join(tmp, "rec.json"),
		PrefsFile:    filepath.Join(tmp, "prefs.json"),
		Standalone:   true,
	}
	prefs := appconfig.UserPrefs{
		ThemeID: "doom-one",
		SavedRemotes: []appconfig.SavedRemote{
			{ID: "edge-1", DisplayName: "Edge GPU", Endpoint: "127.0.0.1:59993"},
		},
	}

	as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	authCtrl := hostlink.AttachAuthAndSettings(ctx, as, hostlink.AuthSettingsConfig{
		Config:         cfg,
		Prefs:          prefs,
		UseMemoryVault: true,
	})
	link, err := hostlink.Connect(
		ctx,
		cfg,
		as,
		schema.TargetDesktopMacOS,
		nil,
		hostlink.WithAuthTokenSource(hostlink.OptionalAuthTokenSource(authCtrl)),
		hostlink.WithSavedRemotes(prefs.SavedRemotes),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer link.Close()

	reg, ledger, host := featurekit.Mount(cfg.FeaturesSpec, as, link.OnDraftChanged, nil)

	var savedPrefs appconfig.UserPrefs
	fr := frame.New(frame.Config{
		Adaptive:       as,
		Link:           link,
		AppConfig:      cfg,
		Prefs:          prefs,
		FeatureReg:     reg,
		FeatureLedger:  ledger,
		FeatureHost:    host,
		InitialThemeID: cfg.ThemeID,
		RoomStats: func() (int, int, int, int) {
			return 5, 8, 3, 4
		},
		OnPrefsChanged: func(p appconfig.UserPrefs) { savedPrefs = p },
	})

	tt := ui.NewTester(fr.View, 1280, 820)

	// 1. Authentic launch: Tab 1 is *Dashboard* (fluid EWW Bento desktop) with live tasks, Liquid Glass & Spring Motion modules enabled.
	if fr.ActiveBuffer() != buffer.Dashboard || fr.KeyMode() != meow.ModeNormal || !tt.HasText("*Dashboard*") || !tt.HasText("*conversation*") || !tt.HasText("Marlin AI") || !tt.HasText("5/8 Done") || !tt.HasText("NORMAL") {
		t.Fatalf("expected fluid EWW Bento *Dashboard* + modeline on launch, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 1b. Hot-pluggable UI Modules: Glass, Motion, and Zen Mode toggle live with zero restart.
	if !fr.UIModules().Glass || !fr.UIModules().Motion {
		t.Fatalf("expected Glass and Motion modules enabled by default")
	}
	fr.ToggleGlass()
	fr.ToggleMotion()
	tt.Frame()
	if fr.UIModules().Glass || fr.UIModules().Motion {
		t.Fatalf("expected Glass and Motion modules to hot-unplug live")
	}
	fr.ToggleGlass()
	fr.ToggleMotion()

	fr.ToggleZenMode()
	tt.Frame()
	if !fr.ZenMode() || tt.HasText("NORMAL") || !tt.HasText("Zen Mode") {
		t.Fatalf("expected Zen Mode to hide modeline while keeping fluid Bento dashboard, got %q", strings.Join(tt.Texts(), " | "))
	}
	fr.ToggleZenMode()
	tt.Frame()
	if fr.ZenMode() || !tt.HasText("NORMAL") {
		t.Fatalf("expected Zen Mode toggle off to restore modeline, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 2. Pressing SPC pops up the multi-column Doom Leader Hydra Minibuffer drawer!
	fr.OpenHydra("SPC")
	tt.Frame()
	if fr.KeypadPrefix() != "SPC" || !tt.HasText("Doom Leader / Hydra") || !tt.HasText("+buffers") || !tt.HasText("+themes & toggles") {
		t.Fatalf("expected multi-column SPC Hydra popup, got %q", strings.Join(tt.Texts(), " | "))
	}
	// Drill into +themes Hydra and test a repeatable preset head (keeps Hydra open!).
	if err := tt.Click("+themes & toggles"); err != nil {
		t.Fatalf("click +themes prefix: %v", err)
	}
	tt.Frame()
	if fr.KeypadPrefix() != "SPC t" || !tt.HasText("Cycle next theme (repeat)") {
		t.Fatalf("expected +themes sub-hydra, got prefix=%q texts=%q", fr.KeypadPrefix(), strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("doom-tokyo-night"); err != nil {
		t.Fatalf("click doom-tokyo-night head in hydra: %v", err)
	}
	tt.Frame()
	if fr.Theme.ID != "doom-tokyo-night" || savedPrefs.ThemeID != "doom-tokyo-night" || fr.KeypadPrefix() != "SPC t" {
		t.Fatalf("expected repeatable theme head to switch to doom-tokyo-night while staying in SPC t hydra, got theme=%q prefix=%q", fr.Theme.ID, fr.KeypadPrefix())
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatalf("click Done to exit hydra: %v", err)
	}
	tt.Frame()
	if fr.KeypadPrefix() != "" {
		t.Fatalf("expected hydra to close on Done, got prefix=%q", fr.KeypadPrefix())
	}

	// 3. Switch to *Sessions* and *Targets* tabulated-list-mode buffers ('main modeline preset).
	if err := tt.Click("*Sessions*"); err != nil {
		t.Fatalf("click *Sessions*: %v", err)
	}
	tt.Frame()
	if fr.ActiveBuffer() != buffer.Sessions || !tt.HasText("Tabulated-List") || !tt.HasText("Primary Workspace") {
		t.Fatalf("expected *Sessions* tabulated buffer + main modeline, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("*Targets*"); err != nil {
		t.Fatalf("click *Targets*: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Edge GPU") {
		t.Fatalf("expected restored remote in *Targets* buffer, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 4. Modern Vertico/Consult Helm ('helm modeline preset, Source Filter Pills, Follow-Mode Preview, Embark actions).
	fr.OpenHelm()
	tt.Frame()
	if fr.MiniMode() != minibuffer.ModeHelm || !tt.HasText("HELM") || !tt.HasText("Preview") || !tt.HasText("Attach") || !tt.HasText("Mark [TAB]") {
		t.Fatalf("expected modern Helm with Preview & Embark actions, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 5. *Settings* Emacs customize buffer (zero popup modal).
	fr.OpenSettings()
	tt.Frame()
	if fr.ActiveBuffer() != buffer.Settings || !tt.HasText("CUSTOMIZE · APPEARANCE & SESSION") || !tt.HasText("Customize") {
		t.Fatalf("expected *Settings* Emacs customize buffer, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 6. Live marlin.ui.v1 gRPC WorkbenchSnapshot (Commands + Scheme UIExtensionRegion) projects via runtime/mapper.
	invoked := ""
	as.Desktop.Commands = append(as.Desktop.Commands, input.CommandEntry{
		Name:        "doom-doctor",
		Description: "Run Scheme runtime health diagnostics",
	})
	as.Desktop.OnCommandInvoke = func(name string) { invoked = name }
	if err := as.Desktop.Workbench.AdmitSnapshot(schema.UIWorkbenchSnapshot{
		Generation:   42,
		ProfileName:  "doom-one",
		ModelLabel:   "claude-sonnet-marlin",
		SessionLabel: "marlin-main",
		Theme: schema.UITheme{
			Canvas: schema.RGB{R: 40, G: 44, B: 52},
			Ink:    schema.RGB{R: 187, G: 194, B: 207},
		},
		FrameTree: schema.UIElement{
			ID:   "root",
			Kind: schema.ElemRows,
		},
		Extensions: []schema.UIExtensionRegion{
			{
				ID:        "doctor",
				Label:     "*Doctor*",
				Focusable: true,
				Content: schema.UIElement{
					Kind: schema.ElemText,
					Spans: []schema.UISpan{
						{Text: "Scheme runtime diagnostics nominal", Tone: schema.ToneSuccess},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("AdmitSnapshot: %v", err)
	}
	tt.Frame()
	if !tt.HasText("claude-sonnet-marlin") || !tt.HasText("*Doctor*") {
		t.Fatalf("expected live marlin.ui.v1 model label and *Doctor* tab, got %q", strings.Join(tt.Texts(), " | "))
	}
	fr.OpenCommands()
	tt.Frame()
	tt.Type("doctor")
	tt.Frame()
	if !tt.HasText("doom-doctor") || !tt.HasText("*Doctor*") {
		t.Fatalf("expected live marlin.ui.v1 M-x command and *Doctor* tab, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("doom-doctor"); err != nil {
		t.Fatalf("click doom-doctor command: %v", err)
	}
	tt.Frame()
	if invoked != "doom-doctor" {
		t.Fatalf("expected OnCommandInvoke('doom-doctor'), got %q", invoked)
	}
	if err := tt.Click("*Doctor*"); err != nil {
		t.Fatalf("click *Doctor* Scheme extension tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Scheme runtime diagnostics nominal") {
		t.Fatalf("expected Scheme extension content rendered via runtime/mapper, got %q", strings.Join(tt.Texts(), " | "))
	}
}

func TestGovernedUserThemePackHotReloadAndFailClosedCSS(t *testing.T) {
	uiDir := filepath.Join(t.TempDir(), "ui")
	packDir := filepath.Join(uiDir, "themes", "nord-custom")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "manifest.toml"), []byte(
		"schema_version = \"marlin.ui.theme.v1\"\nactive_theme = \"nord-custom\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "theme.toml"), []byte(
		"id = \"nord-custom\"\nname = \"Nord Custom\"\nvariant = \"dark\"\nbase = \"doom-one\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "style.css"), []byte(
		":root { --bg: #2e3440; --accent: #88c0d0; }\n.tab.active { background: #434c5e; }\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "theme.css"), []byte(
		":root { --accent: #ff79c6; }\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	// Also test governed user custom widgets in both TOML and S-expression formats!
	wTomlDir := filepath.Join(uiDir, "widgets", "tasks-toml")
	wSexpDir := filepath.Join(uiDir, "widgets", "status-sexp")
	if err := os.MkdirAll(wTomlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wSexpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wTomlDir, "widget.toml"), []byte(
		"kind = \"kpi\"\ntitle = \"TOML Tasks\"\nvalue = \"{{tasks.completed}} OK\"\nprogress = \"{{tasks.progress}}\"\ntone = \"success\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wSexpDir, "layout.sexp"), []byte(
		"(card :id \"sexp-card\" (kpi :icon \"★\" :title \"Sexp Sensor\" :value \"Ready\" :tone \"info\"))\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	fr := frame.New(frame.Config{
		AppConfig: appconfig.Config{SessionID: "marlin-main"},
		RoomStats: func() (int, int, int, int) { return 5, 8, 2, 3 },
		OnReloadThemeBundle: func() appconfig.UIThemeBundle {
			return appconfig.LoadUIThemeBundle(uiDir)
		},
	})
	fr.ReloadThemes()
	if fr.Theme.ID != "nord-custom" || fr.Theme.Canvas != ui.RGB(0x2e, 0x34, 0x40) || fr.Theme.Accent != ui.RGB(0xff, 0x79, 0xc6) {
		t.Fatalf("expected hot-reloaded nord-custom + theme.css override, got id=%q bg=%+v accent=%+v", fr.Theme.ID, fr.Theme.Canvas, fr.Theme.Accent)
	}
	tt := ui.NewTester(fr.View, 1280, 900)
	if !tt.HasText("TOML Tasks") || !tt.HasText("5 OK") || !tt.HasText("Sexp Sensor") {
		t.Fatalf("expected both TOML and S-exp user widgets rendered on *Dashboard*, got %q", strings.Join(tt.Texts(), " | "))
	}

	// Now write an undeclared CSS class to theme.css; reload must fail closed and keep current valid theme.
	if err := os.WriteFile(filepath.Join(uiDir, "theme.css"), []byte(
		".arbitrary-unregistered { background: #000000; }\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	fr.ReloadThemes()
	if fr.Theme.ID != "nord-custom" || fr.Theme.Accent != ui.RGB(0xff, 0x79, 0xc6) {
		t.Fatalf("expected fail-closed reload to preserve active valid theme, got id=%q accent=%+v", fr.Theme.ID, fr.Theme.Accent)
	}
}

func TestDashboardZenToggleDoesNotHijackSpaceKeyMap(t *testing.T) {
	fr := frame.New(frame.Config{
		AppConfig: appconfig.Config{SessionID: "marlin-main"},
	})
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.SetTitleBar(ui.TitleBar{Height: 38, Left: 78})
	tt.Frame()

	if fr.ZenMode() {
		t.Fatalf("expected ZenMode initially false")
	}
	// Click the Zen module switch on *Dashboard* (which used to leave keyboard
	// focus on ButtonBase so pressing Space repeatedly toggled Zen on/off).
	if err := tt.Click("Zen"); err != nil {
		t.Fatalf("click Zen switch on *Dashboard*: %v", err)
	}
	tt.Frame()
	if !fr.ZenMode() {
		t.Fatalf("expected ZenMode true after clicking Zen switch")
	}

	// Pressing Space MUST trigger the SPC Hydra KeyMap — NOT re-click Zen!
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !fr.ZenMode() {
		t.Fatalf("Space hijacked Zen toggle instead of triggering SPC KeyMap!")
	}
	if got := fr.KeypadPrefix(); got != "SPC" {
		t.Fatalf("expected Space after clicking Zen to open SPC Hydra, got keypad=%q", got)
	}

	// Pressing Space a second time (SPC SPC) MUST open M-x while Zen stays ON!
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !fr.ZenMode() {
		t.Fatalf("second Space hijacked Zen toggle instead of triggering SPC SPC M-x!")
	}
	if fr.MiniMode() != minibuffer.ModeCommand {
		t.Fatalf("expected SPC SPC to open ModeCommand, got %v", fr.MiniMode())
	}
}

func TestW3CTokensJSONCustomSidebarAndWebIslandIntegration(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "tokens.json"), []byte(`{
		"color": {
			"$type": "color",
			"accent": { "$value": "oklch(0.76 0.14 210)" }
		},
		"radius": {
			"card": { "$type": "dimension", "$value": "24px" }
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "sidebar.json"), []byte(`{
		"schemaVersion": "marlin.ui.sidebar.v1",
		"width": 260,
		"pinned": true,
		"body": {
			"id": "custom-sb",
			"kind": "col",
			"gap": 8,
			"children": [
				{"id": "sb-kpi", "kind": "kpi", "icon": "sparkles", "title": "TS SIDEBAR", "value": "Mounted", "tone": "info"},
				{"id": "sb-web", "kind": "web-island", "title": "BPMN Studio", "subtitle": "Hybrid GPU", "islandId": "workflow-graph", "height": 140}
			]
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	bundle := appconfig.LoadUIThemeBundle(uiDir)
	if len(bundle.Warnings) != 0 {
		t.Fatalf("unexpected bundle warnings: %v", bundle.Warnings)
	}
	fr := frame.New(frame.Config{
		AppConfig:   appconfig.Config{SessionID: "marlin-main"},
		ThemeBundle: bundle,
	})
	if got := fr.Theme.Sheet.VarPx("--radius-card", 0); got != 24 {
		t.Fatalf("expected W3C tokens.json --radius-card=24, got %g", got)
	}

	tt := ui.NewTester(fr.View, 1280, 900)
	if !tt.HasText("TS SIDEBAR") || !tt.HasText("BPMN Studio") {
		t.Fatalf("expected custom JSON sidebar + WebIsland rendered inside EdgeRail, got %q", strings.Join(tt.Texts(), " | "))
	}
}
