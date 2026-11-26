// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"context"
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
		OnPrefsChanged: func(p appconfig.UserPrefs) { savedPrefs = p },
	})

	tt := ui.NewTester(fr.View, 1280, 820)

	// 1. Authentic Doom Emacs launch: sort-tab, Doom ASCII Dashboard, Doom Modeline ('dashboard preset, NORMAL mode), clean Echo Area.
	if fr.KeyMode() != meow.ModeNormal || !tt.HasText("*conversation*") || !tt.HasText("Recently opened sessions") || !tt.HasText("NORMAL") || !tt.HasText("~/marlin/marlin-main/") {
		t.Fatalf("expected Doom Emacs dashboard + modeline, got %q", strings.Join(tt.Texts(), " | "))
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
	if err := tt.Click("pi"); err != nil {
		t.Fatalf("click pi head in hydra: %v", err)
	}
	tt.Frame()
	if fr.Theme.ID != "pi" || savedPrefs.ThemeID != "pi" || fr.KeypadPrefix() != "SPC t" {
		t.Fatalf("expected repeatable theme head to switch to pi while staying in SPC t hydra, got theme=%q prefix=%q", fr.Theme.ID, fr.KeypadPrefix())
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

	// 4. Vertico M-x & Helm ('helm modeline preset).
	fr.OpenHelm()
	tt.Frame()
	if fr.MiniMode() != minibuffer.ModeHelm || !tt.HasText("HELM") {
		t.Fatalf("expected Helm minibuffer mode, got %q", strings.Join(tt.Texts(), " | "))
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
	fr.OpenCommands()
	tt.Frame()
	tt.Type("doctor")
	tt.Frame()
	if !tt.HasText("claude-sonnet-marlin") || !tt.HasText("doom-doctor") || !tt.HasText("*Doctor*") {
		t.Fatalf("expected live marlin.ui.v1 model label, M-x command, and *Doctor* tab, got %q", strings.Join(tt.Texts(), " | "))
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
