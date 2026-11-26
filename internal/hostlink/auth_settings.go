// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tao3k/marlin-mygo/control/auth"
	"github.com/tao3k/marlin-mygo/engine/notify"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
)

// AuthSettingsConfig configures real Tier-0 OS Keychain Auth and Marlin
// BoundedSchema Settings binding onto an AdaptiveShell.
type AuthSettingsConfig struct {
	Config          appconfig.Config
	Prefs           appconfig.UserPrefs
	UseMemoryVault  bool // set true in headless unit tests to avoid touching host OS Keychain
	OnApplyFeatures func(spec string)
	OnSavePrefs     func(vals map[string]any)
	Invalidate      func()
}

// AttachAuthAndSettings binds Marlin's real Tier-0 OS-Keychain-protected
// AuthController (OAuth 2.1 PKCE, Email OTP, Enterprise OIDC Discovery, Vault
// Lock/Unlock) and typed BoundedSchema Workspace Preferences form onto both
// Desktop (bottom-left dock + Cmd+,) and Mobile targets simultaneously.
func AttachAuthAndSettings(
	ctx context.Context,
	adaptive *mygoshell.AdaptiveShell,
	cfg AuthSettingsConfig,
) *mygoshell.AuthController {
	if adaptive == nil {
		return nil
	}
	invalidate := cfg.Invalidate
	if invalidate == nil {
		invalidate = func() {}
	}

	// 1. Open OS-protected AES-256-GCM credential vault (or deterministic memory vault in tests).
	var (
		vault     auth.CredentialVault
		vaultInfo auth.VaultSecurityInfo
	)
	if cfg.UseMemoryVault {
		vault = auth.NewMemoryVault()
		vaultInfo = auth.VaultSecurityInfo{
			BackendName: "In-Memory Vault (ephemeral)",
			CipherSuite: "AES-256-GCM",
		}
	} else {
		vault, vaultInfo = auth.OpenNativeVault(ctx, auth.NativeVaultConfig{})
	}

	mgr := auth.NewManager(auth.ManagerConfig{
		Vault:       vault,
		OpenBrowser: auth.OpenSystemBrowser,
	})
	ghClientID := strings.TrimSpace(os.Getenv("MARLIN_OAUTH_GITHUB_CLIENT_ID"))
	if ghClientID == "" {
		ghClientID = "marlin-ui-github"
	}
	googleClientID := strings.TrimSpace(os.Getenv("MARLIN_OAUTH_GOOGLE_CLIENT_ID"))
	if googleClientID == "" {
		googleClientID = "marlin-ui-google"
	}
	_ = mgr.RegisterProvider(auth.GitHubOAuthProvider(ghClientID))
	_ = mgr.RegisterProvider(auth.GoogleOAuthProvider(googleClientID))
	_ = mgr.RegisterProvider(auth.EmailOTPProvider())
	_, _ = mgr.RestoreFromVault(ctx)

	authCtrl := mygoshell.NewAuthController(mgr)
	authCtrl.VaultInfo = vaultInfo
	if osVault, ok := vault.(*auth.OSProtectedVault); ok {
		authCtrl.Vault = osVault
	}
	authCtrl.OnConfigureOIDC = func(issuerURL, clientID string) error {
		dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		prov, err := auth.DiscoverOIDCProvider(dctx, &http.Client{Timeout: 8 * time.Second}, issuerURL, clientID, "Enterprise SSO", false)
		if err != nil {
			return err
		}
		return mgr.RegisterProvider(prov)
	}
	authCtrl.OnChanged = invalidate
	adaptive.SetAuth(authCtrl)

	// 2. Bind Marlin's typed BoundedSchema Settings form across Desktop + Mobile.
	spec := appconfig.SettingsSchema()
	initial := appconfig.InitialSettingsValues(cfg.Config, cfg.Prefs)
	formState := mygoshell.NewSchemaFormState(spec, initial)
	adaptive.SetSettings(&spec, formState, func(vals map[string]any) {
		if themeID, ok := vals["theme_id"].(string); ok && strings.TrimSpace(themeID) != "" {
			adaptive.ApplyThemePreset(strings.TrimSpace(themeID))
		}
		if collapsed, ok := vals["sidebar_collapsed"].(bool); ok && adaptive.Desktop != nil {
			adaptive.Desktop.SidebarCollapsed = collapsed
		}
		if sess, ok := vals["default_session"].(string); ok && strings.TrimSpace(sess) != "" && adaptive.Desktop != nil {
			sid := strings.TrimSpace(sess)
			if sid != adaptive.Desktop.ActiveSessionID && adaptive.Desktop.OnSelectSession != nil {
				adaptive.Desktop.OnSelectSession(sid)
			}
		}
		if feat, ok := vals["features_spec"].(string); ok && strings.TrimSpace(feat) != "" && cfg.OnApplyFeatures != nil {
			cfg.OnApplyFeatures(strings.TrimSpace(feat))
		}
		if cfg.OnSavePrefs != nil {
			cfg.OnSavePrefs(vals)
		}
		if adaptive.Desktop != nil && adaptive.Desktop.Notifications != nil {
			_, _ = adaptive.Desktop.Notifications.Center.Publish(notify.NotificationSpec{
				Severity: notify.SeverityInfo,
				Source:   notify.SourceHost,
				Title:    "Preferences Saved",
				Message:  fmt.Sprintf("Theme=%v · Session=%v", vals["theme_id"], vals["default_session"]),
			})
		}
		invalidate()
	})

	return authCtrl
}

// OptionalAuthTokenSource bridges a Tier-0 AuthController's OS Keychain Vault
// into grpcwire.DialConfig.AuthTokenSource. When an account is signed in, each
// remote gRPC RPC receives a freshly refreshed Bearer header; when signed out,
// it returns ("", nil) so mTLS-only or unauthenticated loopback hosts still dial.
func OptionalAuthTokenSource(authCtrl *mygoshell.AuthController) func(context.Context) (string, error) {
	if authCtrl == nil || authCtrl.Manager == nil {
		return nil
	}
	mgr := authCtrl.Manager
	return func(ctx context.Context) (string, error) {
		if _, ok := mgr.Snapshot().ActiveProfile(); !ok {
			return "", nil
		}
		return mgr.AuthorizationHeader(ctx)
	}
}
