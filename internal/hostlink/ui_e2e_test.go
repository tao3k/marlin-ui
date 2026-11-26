// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/auth"
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/featurekit"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func TestFullStackDesktopUIE2E_SettingsAuthHelmAndSavedRemotes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tmp := t.TempDir()
	cfg := appconfig.Config{
		SessionID:    "marlin-e2e-main",
		ThemeID:      "midnight",
		FeaturesSpec: "all",
		RecoveryFile: filepath.Join(tmp, "recovery.json"),
		PrefsFile:    filepath.Join(tmp, "ui-prefs.json"),
		Standalone:   true,
	}
	prefs := appconfig.UserPrefs{
		ThemeID:       "midnight",
		LastSessionID: "marlin-e2e-main",
		SavedRemotes: []appconfig.SavedRemote{
			{ID: "saved-edge-1", DisplayName: "Saved Edge GPU", Endpoint: "127.0.0.1:59991"},
		},
	}

	as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		BrandSubtitle:  "Desktop E2E",
		InitialThemeID: cfg.ThemeID,
		FormFactor:     mygoshell.FormFactorDesktop,
	})
	var applyFeatures func(string)
	authCtrl := hostlink.AttachAuthAndSettings(ctx, as, hostlink.AuthSettingsConfig{
		Config:         cfg,
		Prefs:          prefs,
		UseMemoryVault: true,
		OnApplyFeatures: func(spec string) {
			if applyFeatures != nil {
				applyFeatures(spec)
			}
		},
		OnSavePrefs: func(vals map[string]any) {
			appconfig.ApplySettingsValues(&prefs, vals)
			_ = appconfig.SavePrefs(cfg.PrefsFile, prefs)
		},
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
	applyFeatures = func(spec string) {
		reg.Activate(features.ParseFeatureSpec(spec), ledger, host)
		as.SetFeatureHost(host, as.Desktop.GovernanceUI)
	}

	// 1. OptionalAuthTokenSource returns empty cleanly while signed out.
	tokSrc := hostlink.OptionalAuthTokenSource(authCtrl)
	if hdr, err := tokSrc(ctx); err != nil || hdr != "" {
		t.Fatalf("expected empty auth header when signed out, got (%q, %v)", hdr, err)
	}

	// 2. Saved remotes are registered in TargetRegistry & searchable via Helm.
	foundSaved := false
	for _, tm := range link.Targets.List() {
		if tm.ID == "saved-edge-1" && tm.DisplayName == "Saved Edge GPU" {
			foundSaved = true
			break
		}
	}
	if !foundSaved {
		t.Fatalf("expected saved remote 'saved-edge-1' restored into TargetRegistry, got %+v", link.Targets.List())
	}
	as.Desktop.HelmSearch.SetQuery("Saved Edge")
	hits, _, _, _ := as.Desktop.HelmSearch.Engine.Stats()
	if hits == 0 {
		t.Fatalf("expected HelmSearch to return restored saved remote target")
	}

	// 3. Render real 1360x880 Desktop frame and exercise bottom-left Settings & Auth dock!
	tt := ui.NewTester(as.View, 1360, 880)
	if !tt.HasText("Settings") || !tt.HasText("Account & Vault") {
		t.Fatalf("expected bottom-left Settings & Account dock on Desktop, got %q", strings.Join(tt.Texts(), " | "))
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatalf("click Settings: %v", err)
	}
	tt.Frame()
	if !as.Desktop.IsSettingsOpen() || !tt.HasText("Marlin Workspace Preferences") {
		t.Fatalf("expected Marlin Workspace Preferences modal open, got %q", strings.Join(tt.Texts(), " | "))
	}
	as.Desktop.OpenSettingsTab(mygoshell.SettingsTabAuth)
	tt.Frame()
	if !tt.HasText("Continue with GitHub") || !tt.HasText("Continue with Google") {
		t.Fatalf("expected GitHub/Google OAuth options in Auth tab, got %q", strings.Join(tt.Texts(), " | "))
	}
	as.Desktop.OpenSettingsTab(mygoshell.SettingsTabGeneral)
	tt.Frame()
	if err := tt.Click("Save Settings"); err != nil {
		t.Fatalf("click Save Settings in Settings modal: %v", err)
	}
	tt.Frame()
	if as.Desktop.IsSettingsOpen() {
		t.Fatalf("expected Settings modal closed after Save")
	}
	loadedPrefs, err := appconfig.LoadPrefs(cfg.PrefsFile)
	if err != nil || loadedPrefs.LastSessionID != "marlin-e2e-main" {
		t.Fatalf("expected prefs persisted to disk after Save, got (%+v, %v)", loadedPrefs, err)
	}

	// 4. Verify signing in via real AuthManager + Vault immediately yields Bearer
	// header through OptionalAuthTokenSource without leaking raw tokens to UI.
	signedMgr := auth.NewManager(auth.ManagerConfig{
		Vault:        auth.NewMemoryVault(),
		EmailBackend: stubEmailBackend{},
	})
	_ = signedMgr.RegisterProvider(auth.EmailOTPProvider())
	if _, err := signedMgr.RequestEmailLogin(ctx, "dev@tao3k.dev", auth.MethodEmailOTP); err != nil {
		t.Fatalf("RequestEmailLogin: %v", err)
	}
	if _, err := signedMgr.VerifyEmailLogin(ctx, "654321"); err != nil {
		t.Fatalf("VerifyEmailLogin: %v", err)
	}
	signedCtrl := mygoshell.NewAuthController(signedMgr)
	as.SetAuth(signedCtrl)
	signedTokSrc := hostlink.OptionalAuthTokenSource(signedCtrl)
	hdr, err := signedTokSrc(ctx)
	if err != nil || hdr != "Bearer SECRET_OAUTH_TOKEN_SHOULD_NEVER_APPEAR_IN_UI" {
		t.Fatalf("expected Bearer header from OptionalAuthTokenSource, got (%q, %v)", hdr, err)
	}
	tt.Frame()
	if !tt.HasText("Tao Dev") {
		t.Fatalf("expected signed-in display name in Desktop UI, got %q", strings.Join(tt.Texts(), " | "))
	}
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "SECRET_OAUTH_TOKEN_SHOULD_NEVER_APPEAR_IN_UI") {
			t.Fatalf("SECURITY VIOLATION: raw OAuth token leaked into UI text: %q", txt)
		}
	}
}

type stubEmailBackend struct{}

func (stubEmailBackend) RequestChallenge(_ context.Context, _ string, _ auth.AuthMethod) (auth.EmailChallengeResponse, error) {
	return auth.EmailChallengeResponse{
		ChallengeID: "e2e-ch-1",
		ExpiresIn:   5 * time.Minute,
		Cooldown:    time.Second,
		MaxAttempts: 5,
	}, nil
}

func (stubEmailBackend) VerifyChallenge(_ context.Context, _, email, _ string, _ auth.AuthMethod) (auth.TokenSet, auth.AccountProfile, error) {
	return auth.TokenSet{
			AccessToken: "SECRET_OAUTH_TOKEN_SHOULD_NEVER_APPEAR_IN_UI",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(time.Hour),
		}, auth.AccountProfile{
			Subject:       "usr-42",
			Email:         email,
			DisplayName:   "Tao Dev",
			EmailVerified: true,
		}, nil
}

func TestFullStackMobileIOSAndAndroidUIE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cases := []struct {
		name       string
		formFactor mygoshell.ShellFormFactor
		platform   mygoshell.MobilePlatform
		targetKind schema.TargetKind
		w, h       int
	}{
		{"iOS-iPhone16", mygoshell.FormFactorIOS, mygoshell.MobilePlatformIOS, schema.TargetMobileIOS, 393, 852},
		{"Android-Pixel9", mygoshell.FormFactorAndroid, mygoshell.MobilePlatformAndroid, schema.TargetMobileAndroid, 412, 915},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := appconfig.Config{
				SessionID:    "mobile-" + tc.name,
				ThemeID:      "midnight",
				FeaturesSpec: "all",
				RecoveryFile: filepath.Join(t.TempDir(), "mob-recovery.json"),
				Standalone:   true,
			}
			as := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
				BrandTitle:     "Marlin",
				InitialThemeID: cfg.ThemeID,
				FormFactor:     tc.formFactor,
				MobilePlatform: tc.platform,
			})
			authCtrl := hostlink.AttachAuthAndSettings(ctx, as, hostlink.AuthSettingsConfig{
				Config:         cfg,
				UseMemoryVault: true,
			})
			link, err := hostlink.Connect(
				ctx,
				cfg,
				as,
				tc.targetKind,
				nil,
				hostlink.WithAuthTokenSource(hostlink.OptionalAuthTokenSource(authCtrl)),
			)
			if err != nil {
				t.Fatalf("Connect %s: %v", tc.name, err)
			}
			defer link.Close()
			featurekit.Mount("all", as, link.OnDraftChanged, nil)

			tt := ui.NewTester(as.View, tc.w, tc.h)
			if !tt.HasText("Chat") || !tt.HasText("Gates") {
				t.Fatalf("expected authentic mobile tabs on %s, got %q", tc.name, strings.Join(tt.Texts(), " | "))
			}
			as.Mobile.OpenSettings()
			tt.Frame()
			if !as.Mobile.IsSettingsOpen() || !tt.HasText("Marlin Workspace Preferences") {
				t.Fatalf("expected mobile Settings modal open on %s, got %q", tc.name, strings.Join(tt.Texts(), " | "))
			}
		})
	}
}
