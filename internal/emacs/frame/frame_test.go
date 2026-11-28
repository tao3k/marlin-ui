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
	"github.com/tao3k/marlin-mygo/engine/search"
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
	frame.ApplyTestSchemeKeymap(fr)
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

	// 3. Open *Sessions* and *Targets* tabulated-list-mode buffers on demand ('main modeline preset).
	fr.SelectBuffer(buffer.Sessions)
	tt.Frame()
	if fr.ActiveBuffer() != buffer.Sessions || !tt.HasText("Tabulated-List") || !tt.HasText("Primary Workspace") {
		t.Fatalf("expected *Sessions* tabulated buffer + main modeline, got %q", strings.Join(tt.Texts(), " | "))
	}
	fr.SelectBuffer(buffer.Targets)
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

func TestDoomSplitsEscapeDismissPopupAndLiveModelSwitch(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	var invokedCmds []string
	sh.Desktop.OnCommandInvoke = func(name string) {
		invokedCmds = append(invokedCmds, name)
	}
	var savedPrefs appconfig.UserPrefs

	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-splits"},
		OnPrefsChanged: func(p appconfig.UserPrefs) {
			savedPrefs = p
		},
	})
	frame.ApplyTestSchemeKeymap(fr)
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()

	// 1. SPC w v -> vertical split renders both [1] and [2] pane headers.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if fr.KeypadPrefix() != "SPC" {
		t.Fatalf("expected SPC keypad, got %q", fr.KeypadPrefix())
	}
	tt.Key(0, ui.KeyW)
	tt.Frame()
	if fr.KeypadPrefix() != "SPC w" {
		t.Fatalf("expected SPC w keypad, got %q", fr.KeypadPrefix())
	}
	tt.Key(0, ui.KeyV)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.WindowCount() != 2 || !tt.HasText("1") || !tt.HasText("2") || !tt.HasText("*Dashboard*") || !tt.HasText("*conversation*") {
		t.Fatalf("expected vertical split with per-window Doom modeline numbers 1 & 2, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 1b. Switch pane 2 to *conversation*, resize & swap panes via SPC w l and SPC w x, plus nested 3rd window (SPC w s),
	// direct winum jump (SPC w 1), dedicated pin (SPC w t), and winner-mode undo/redo (SPC w u / SPC w U).
	fr.SelectBuffer(buffer.Conversation)
	fr.ResizeSplit(0.10)
	fr.SwapSplitPanes()
	tt.Frame()
	winsAfterSwap := fr.WorkspaceSnapshot().Active.Windows()
	if len(winsAfterSwap) != 2 || winsAfterSwap[0].Buffer != buffer.Conversation || winsAfterSwap[1].Buffer != buffer.Dashboard {
		t.Fatalf("expected swapped split panes (*conversation* <-> *Dashboard*), got %+v", winsAfterSwap)
	}
	fr.SplitHorizontal(buffer.Sessions)
	tt.Frame()
	if got := fr.WorkspaceSnapshot().Active.WindowCount(); got != 3 {
		t.Fatalf("expected nested 3-window split tree, got %d windows", got)
	}
	if !tt.HasText("3") {
		t.Fatalf("expected window number 3 in per-window Doom modeline, got %q", strings.Join(tt.Texts(), " | "))
	}
	fr.SelectWindowNumber(1)
	if got := fr.WorkspaceSnapshot().Active.FocusedWindowNumber(); got != 1 {
		t.Fatalf("expected SPC w 1 to focus window 1, got %d", got)
	}
	fr.ToggleWindowDedicated()
	tt.Frame()
	if !fr.WorkspaceSnapshot().Active.FocusedLeaf().Dedicated || !tt.HasText("D") {
		t.Fatalf("expected dedicated pin indicator on focused window modeline")
	}
	fr.ToggleWindowDedicated()
	// Undo the 3rd split back to 2 windows via Doom winner-mode (SPC w u) and redo via SPC w U.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyW)
	tt.Frame()
	tt.Key(0, ui.KeyU)
	tt.Frame()
	if got := fr.WorkspaceSnapshot().Active.WindowCount(); got != 2 {
		t.Fatalf("expected SPC w u (winner-undo) to restore 2-window layout, got %d", got)
	}
	tt.Key(ui.Shift, ui.KeyU)
	tt.Frame()
	if got := fr.WorkspaceSnapshot().Active.WindowCount(); got != 3 {
		t.Fatalf("expected Shift+U (winner-redo) in SPC w hydra to restore 3-window layout, got %d", got)
	}
	tt.Key(0, ui.KeyU)
	tt.Frame()
	tt.Key(0, ui.KeyQ)
	tt.Frame()

	// 1c. Canonical Doom SPC TAB (+workspaces) muscle-memory check:
	// SPC TAB n creates a workspace, SPC TAB [ switches back to main!
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if fr.KeypadPrefix() != "SPC TAB" {
		t.Fatalf("expected Doom canonical SPC TAB workspace hydra, got %q", fr.KeypadPrefix())
	}
	tt.Key(0, ui.KeyN)
	tt.Frame()
	if got := len(fr.WorkspaceSnapshot().Workspaces); got != 2 {
		t.Fatalf("expected SPC TAB n to create 2nd workspace, got %d", got)
	}
	// Cleanly kill the ephemeral workspace via SPC TAB d so step 2 tests named creation cleanly.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Key(0, ui.KeyD)
	tt.Frame()
	if got := len(fr.WorkspaceSnapshot().Workspaces); got != 1 {
		t.Fatalf("expected SPC TAB d to kill workspace and return to main, got %d", got)
	}

	// 2. SPC l c opens named workspace Minibuffer; creating "research @/repo/lab"
	// isolates layout and persists workspaces into UserPrefs.
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyL)
	tt.Frame()
	tt.Key(0, ui.KeyC)
	tt.Frame()
	if fr.MiniMode() != minibuffer.ModeWorkspaceCreate {
		t.Fatalf("expected ModeWorkspaceCreate, got %v", fr.MiniMode())
	}
	fr.CreateWorkspaceFromSpec("research @/repo/lab")
	tt.Frame()
	if got := fr.WorkspaceSnapshot().Active.SplitMode; got != "single" {
		t.Fatalf("expected new workspace to start single-pane, got %q", got)
	}
	if len(savedPrefs.Workspaces) < 2 || savedPrefs.ActiveWorkspaceID != "research" {
		t.Fatalf("expected persisted workspaces in UserPrefs, got %+v", savedPrefs)
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyL)
	tt.Frame()
	tt.Key(0, ui.Key1)
	tt.Frame()
	if got := fr.WorkspaceSnapshot().Active.SplitMode; got != "vertical" {
		t.Fatalf("expected switching back to workspace #1 to restore vertical split, got %q", got)
	}

	// 3. Open *Preview* file popup drawer; verify source preview renders and ESC dismisses it cleanly.
	fr.OpenFilePreview("main.rs", "src/main.rs:2", "fn main() {\n    println!(\"marlin\");\n}", 0)
	tt.Frame()
	if !fr.WorkspaceSnapshot().Active.PopupVisible || !tt.HasText("src/main.rs:2") || !tt.HasText("+ @cite") {
		t.Fatalf("expected *Preview* popup drawer with source content, got %q", strings.Join(tt.Texts(), " | "))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if fr.WorkspaceSnapshot().Active.PopupVisible {
		t.Fatalf("expected ESC in Normal mode to dismiss active Doom popup drawer")
	}
	if got := fr.WorkspaceSnapshot().Active.SplitMode; got != "vertical" {
		t.Fatalf("expected main split untouched after ESC popup dismiss, got %q", got)
	}

	// 4. Live SelectModel & ToggleDeepThink dispatch model.select + thinking.set to Rust host.
	fr.SelectModel("deepseek-v4-pro", "deepseek-v4-pro · deepseek-reasoner (deepseek)")
	fr.ToggleDeepThink()
	tt.Frame()
	joined := strings.Join(invokedCmds, " | ")
	if !strings.Contains(joined, "model.select deepseek-v4-pro") || !strings.Contains(joined, "thinking.set on") {
		t.Fatalf("expected model.select + thinking.set commands, got %q", joined)
	}
	if !tt.HasText("deepseek-v4-pro · deepseek-reasoner (deepseek)") {
		t.Fatalf("expected live model label in modeline/frame, got %q", strings.Join(tt.Texts(), " | "))
	}

	// 5. Empty *conversation* renders the flagship Hero Stage + ModeLine Marlin dock (clicking opens formal ChatBox).
	fr.DeleteOtherWindows()
	fr.SelectBuffer(buffer.Conversation)
	tt.Frame()
	if !tt.HasText("What would you like to build with Marlin?") || !tt.HasText("Marlin ChatBox") {
		t.Fatalf("expected empty *conversation* Hero Stage + ModeLine Marlin dock, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Marlin ChatBox"); err != nil {
		t.Fatalf("expected ModeLine Marlin dock clickable: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Deep Think") || !tt.HasText("Cite") {
		t.Fatalf("expected formal ChatBox (Deep Think + Cite) after clicking ModeLine Marlin dock, got %q", strings.Join(tt.Texts(), " | "))
	}
}

func TestHelmAndVerticoDrawerHeightRemainsFixedAndDeclarativeSurfaceTOMLApplies(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-fixed-height"},
	})

	// 1. Default unified Liquid Glass translucency & macOS 26 glass-clear vibrancy come from defaults/surface.toml.
	glassOp, chromeOp, viewportOp, vib := fr.EffectiveSurface(fr.Theme)
	if vib != "glass-clear" || glassOp != 0.56 || chromeOp != 0.0 || viewportOp != 0.75 {
		t.Fatalf("expected declarative TOML unified surface defaults (glass-clear, 0.56, 0.00, 0.75), got (%q, %.2f, %.2f, %.2f)", vib, glassOp, chromeOp, viewportOp)
	}

	// 2. Opening Helm keeps the exact same drawer Y anchor across many hits vs 0 hits (zero vertical bounce).
	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	fr.OpenHelm()
	tt.Frame()
	rectAll, okAll := tt.Find("Follow Preview")
	if !okAll {
		t.Fatalf("expected Helm header visible, got %q", strings.Join(tt.Texts(), " | "))
	}
	tt.Type("zzzz_no_matching_candidate_ever")
	tt.Frame()
	rectEmpty, okEmpty := tt.Find("Follow Preview")
	if !okEmpty {
		t.Fatalf("expected Helm header still visible on 0 hits")
	}
	if rectAll.Y != rectEmpty.Y {
		t.Fatalf("Helm drawer bounced vertically when candidate count dropped to 0: y_many=%v y_empty=%v", rectAll.Y, rectEmpty.Y)
	}

	// 3. Opening Vertico M-x keeps the exact same drawer Y anchor across 10 hits vs 1 hit vs 0 hits.
	fr.OpenCommands()
	tt.Frame()
	rectCmdMany, okMany := tt.Find("helm")
	if !okMany {
		t.Fatalf("expected M-x candidates visible")
	}
	tt.Type("zzzz_no_command_match")
	tt.Frame()
	rectCmdEmpty, okCmdEmpty := tt.Find("No matching candidates")
	if !okCmdEmpty {
		t.Fatalf("expected Vertico empty placeholder visible")
	}
	if diff := rectCmdMany.Y - rectCmdEmpty.Y; diff < -16 || diff > 16 {
		t.Fatalf("Vertico drawer bounced vertically on 0 hits: y_many=%v y_empty=%v", rectCmdMany.Y, rectCmdEmpty.Y)
	}
}

func TestHelmZeroBlankFlashDuringAsyncQueryTransition(t *testing.T) {
	sh := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	fr := frame.New(frame.Config{
		Adaptive:  sh,
		AppConfig: appconfig.Config{SessionID: "sess-swr-helm"},
	})

	tt := ui.NewTester(fr.View, 1280, 900)
	tt.Frame()
	fr.OpenHelm()
	gen1 := sh.Desktop.HelmSearch.Engine.Generation()
	sh.Desktop.HelmSearch.Engine.IngestBatch(gen1, search.SourceMeta{
		ID:       "rust-graph",
		Label:    "Workspace Search",
		Priority: 2,
	}, []search.Candidate{
		{
			ID:       "rust:marlin-1",
			SourceID: "rust-graph",
			Title:    "marlin_search.rs",
			Location: "crates/search/src/marlin_search.rs:L10",
			Snippet:  "pub fn query_workspace() {}",
		},
	})
	tt.Frame()
	if !tt.HasText("marlin_search.rs") {
		t.Fatalf("expected initial Helm hit rendered, got %q", strings.Join(tt.Texts(), " | "))
	}
	prevRect, ok := tt.Find("Follow Preview")
	if !ok {
		t.Fatalf("expected Follow Preview control rendered")
	}

	// Simulate user typing a new character while async Rust RPC has not returned yet:
	// tt.Type("m") delivers the TextInput event and settles the immediate post-keystroke
	// frame; HelmEngine.BeginQuery wiped rust-graph, yet SWR MUST hold the committed
	// frame steady with ZERO flash to blank/local-only and pause 1Hz idle ticks!
	tt.Type("m")
	if fr.ShouldTickIdle() {
		t.Fatalf("expected 1Hz background clock tick paused while Helm completion is active")
	}
	if tt.HasText("No matching candidates") || tt.HasText("No candidate selected") {
		t.Fatalf("Helm flashed blank/empty during in-flight async query! texts=%q", strings.Join(tt.Texts(), " | "))
	}
	if !tt.HasText("marlin_search.rs") {
		t.Fatalf("expected SWR to keep previous candidate visible while syncing, got %q", strings.Join(tt.Texts(), " | "))
	}
	midRect, ok := tt.Find("Follow Preview")
	if !ok || midRect.Y != prevRect.Y {
		t.Fatalf("expected 0px vertical shift during SWR transition: before=%v during=%v", prevRect.Y, midRect.Y)
	}

	// Now deliver the new generation's Rust hits (including a very long title that
	// previously overflowed onto its right-side subtitle column) -> atomic single-frame swap!
	gen2 := sh.Desktop.HelmSearch.Engine.Generation()
	longTitle := "SPDX-FileCopyrightText: 2026 tao3k team and Contributors · README.org · extra_long_segment"
	subText := "workspace.file · #blake3-256"
	sh.Desktop.HelmSearch.Engine.IngestBatch(gen2, search.SourceMeta{
		ID:       "rust-graph",
		Label:    "Workspace Search",
		Priority: 2,
	}, []search.Candidate{
		{
			ID:       "rust:marlin-2",
			SourceID: "rust-graph",
			Title:    "marlin_next.rs",
			Subtitle: subText,
			Location: "crates/search/src/marlin_next.rs:L42",
			Snippet:  "pub fn next_query() {}",
		},
		{
			ID:       "rust:long-title",
			SourceID: "rust-graph",
			Title:    longTitle,
			Subtitle: "long.sub.unique",
			Location: "README.org:L1",
			Snippet:  "# SPDX-FileCopyrightText: 2026 tao3k team and Contributors",
		},
	})
	tt.Frame()
	if !tt.HasText("marlin_next.rs") {
		t.Fatalf("expected atomic swap to new generation candidate, got %q", strings.Join(tt.Texts(), " | "))
	}
	if tt.HasText("Rust gRPC") || tt.HasText("Offline · #") {
		t.Fatalf("Helm header must stay clean without noisy Rust gRPC / Offline provenance pills")
	}
	shortSubRect, okShortSub := tt.Find(subText)
	longSubRect, okLongSub := tt.Find("long.sub.unique")
	if !okShortSub || !okLongSub {
		t.Fatalf("expected both short and long candidate subtitles rendered")
	}
	if longSubRect.X != shortSubRect.X {
		t.Fatalf("long candidate title shifted subtitle column: short.X=%.1f long.X=%.1f", shortSubRect.X, longSubRect.X)
	}
	afterRect, ok := tt.Find("Follow Preview")
	if !ok || afterRect.Y != prevRect.Y {
		t.Fatalf("expected 0px vertical shift after atomic swap: before=%v after=%v", prevRect.Y, afterRect.Y)
	}
}
