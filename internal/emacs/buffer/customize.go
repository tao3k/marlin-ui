// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/auth"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderCustomize(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.Customize == nil {
		return
	}
	st := bc.Customize
	echo := func(msg string) {
		if bc.OnEcho != nil {
			bc.OnEcho(msg)
		}
	}

	ui.Scroll(c.Key("settings-buf-scroll")).Grow(1).Children(func() {
		ui.Column(c).AlignItems(ui.Stretch).Children(func() {
			theme.TabulatedHeader(c, []theme.TableColumn{{Label: "CUSTOMIZE · APPEARANCE & SESSION"}}, th)

			theme.FieldRow(c, "theme-profile", "doom-one / pi / doom-paper", th, func() {
				for _, tp := range theme.Catalog() {
					p := tp
					active := p.ID == th.ID
					tone := th.InkMuted
					if active {
						tone = th.Info
					}
					if theme.ActionButton(c, "set-th-"+p.ID, p.Name, tone, active, th).Clicked() && bc.OnSetTheme != nil {
						bc.OnSetTheme(p.ID)
					}
				}
			})
			theme.FieldRow(c, "active-session", "", th, func() {
				theme.InlineInput(c, "set-sess", &st.SessionID, "marlin-main", 250, th)
			})
			theme.FieldRow(c, "default-remote-host", "optional TLS gRPC", th, func() {
				theme.InlineInput(c, "set-rem", &st.RemoteHost, "host:50051", 250, th)
			})
			theme.FieldRow(c, "durable-streaming", "ConversationJournal", th, func() {
				lbl := "Off"
				tone := th.InkMuted
				if st.RealSession {
					lbl = "On"
					tone = th.Success
				}
				if theme.ActionButton(c, "set-real-tog", lbl, tone, st.RealSession, th).Clicked() {
					st.RealSession = !st.RealSession
				}
			})
			theme.FieldRow(c, "actions", "", th, func() {
				if theme.ActionButton(c, "set-save-btn", "Save Preferences", th.Success, true, th).Clicked() {
					if bc.OnSavePrefs != nil {
						bc.OnSavePrefs(st.SessionID, st.RemoteHost, st.RealSession)
					}
					echo("Saved *Settings* preferences")
				}
			})

			theme.TabulatedHeader(c, []theme.TableColumn{{Label: "FEDERATION · ADD REMOTE GRPC TARGET"}}, th)
			theme.FieldRow(c, "remote-id", "", th, func() {
				theme.InlineInput(c, "add-rem-id", &st.NewRemoteID, "edge-gpu", 200, th)
			})
			theme.FieldRow(c, "remote-endpoint", "TLS 1.3 / mTLS", th, func() {
				theme.InlineInput(c, "add-rem-ep", &st.NewRemoteEndpoint, "10.0.0.5:50051", 250, th)
			})
			theme.FieldRow(c, "bearer-token", "optional", th, func() {
				theme.InlineInput(c, "add-rem-tok", &st.NewRemoteToken, "(optional)", 250, th)
				if theme.ActionButton(c, "add-rem-go", "Connect", th.Info, true, th).Clicked() {
					if bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.OnAddRemoteTarget != nil {
						if err := bc.Adaptive.Desktop.OnAddRemoteTarget(st.NewRemoteID, st.NewRemoteID, st.NewRemoteEndpoint, st.NewRemoteToken); err != nil {
							echo("Error: " + err.Error())
						} else {
							st.NewRemoteID, st.NewRemoteEndpoint, st.NewRemoteToken = "", "", ""
							echo("Connected remote target")
						}
					}
				}
			})

			theme.TabulatedHeader(c, []theme.TableColumn{{Label: "ACCOUNT · OS KEYCHAIN VAULT & OIDC"}}, th)
			if bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.Auth != nil && bc.Adaptive.Desktop.Auth.Manager != nil {
				ac := bc.Adaptive.Desktop.Auth
				vi := ac.CurrentVaultInfo()
				snap := ac.Manager.Snapshot()
				vaultText := vi.BackendName
				if vaultText == "" {
					vaultText = "Encrypted Host Vault"
				}
				theme.FieldRow(c, "credential-vault", vaultText, th, func() {
					if ac.CanToggleVaultLock() {
						lbl := "Lock Vault"
						if vi.Locked {
							lbl = "Unlock Vault"
						}
						if theme.ActionButton(c, "set-vault-lock", lbl, th.Warning, false, th).Clicked() {
							ac.ToggleVaultLock()
						}
					}
				})
				if prof, ok := snap.ActiveProfile(); ok {
					theme.FieldRow(c, "signed-in", prof.DisplayName+" <"+prof.Email+">", th, func() {
						if theme.ActionButton(c, "set-signout", "Sign Out", th.Danger, false, th).Clicked() {
							_ = ac.Manager.SignOut(context.Background(), prof.ID)
							echo("Signed out")
						}
					})
				} else {
					for _, p := range snap.Providers {
						if p.Method == auth.MethodEmailOTP || p.Method == auth.MethodEmailMagicLink {
							continue
						}
						prov := p
						theme.FieldRow(c, "oauth-"+string(prov.ID), "external browser PKCE", th, func() {
							if theme.ActionButton(c, "set-oauth-"+string(prov.ID), "Sign in with "+prov.DisplayName, th.Info, true, th).Clicked() {
								if ac.OnStartOAuth != nil {
									_ = ac.OnStartOAuth(prov.ID)
									echo("Opening browser for " + prov.DisplayName)
								}
							}
						})
					}
				}
				theme.FieldRow(c, "oidc-issuer", "HTTPS discovery URL", th, func() {
					theme.InlineInput(c, "set-oidc-iss", &st.OIDCIssuer, "https://sso.company.com", 260, th)
				})
				theme.FieldRow(c, "oidc-client-id", "", th, func() {
					theme.InlineInput(c, "set-oidc-cid", &st.OIDCClientID, "marlin-desktop", 200, th)
					if theme.ActionButton(c, "set-oidc-go", "Connect OIDC", th.Info, true, th).Clicked() {
						if ac.OnConfigureOIDC != nil {
							if err := ac.OnConfigureOIDC(st.OIDCIssuer, st.OIDCClientID); err != nil {
								echo("OIDC error: " + err.Error())
							} else {
								echo("Connected Enterprise OIDC")
							}
						}
					}
				})
			}

			if bc.FeatureReg != nil && bc.FeatureLedger != nil && bc.FeatureHost != nil {
				theme.TabulatedHeader(c, []theme.TableColumn{{Label: "FEATURES · COMPILED MODULES"}}, th)
				for _, s := range bc.FeatureReg.Snapshot() {
					stFeat := s
					theme.FieldRow(c, string(stFeat.Meta.ID), stFeat.Meta.Description, th, func() {
						lbl := "Disabled"
						tone := th.InkMuted
						if stFeat.Enabled {
							lbl = "Enabled"
							tone = th.Success
						}
						if theme.ActionButton(c, "set-feat-"+string(stFeat.Meta.ID), lbl, tone, stFeat.Enabled, th).Clicked() {
							_ = bc.FeatureReg.SetEnabled(stFeat.Meta.ID, !stFeat.Enabled, bc.FeatureLedger, bc.FeatureHost)
							if bc.Adaptive != nil {
								bc.Adaptive.SetFeatureHost(bc.FeatureHost, bc.Adaptive.Desktop.GovernanceUI)
							}
						}
					})
				}
			}
		})
	})
}
