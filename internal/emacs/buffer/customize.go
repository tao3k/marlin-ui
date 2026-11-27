// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"context"
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/auth"
	"github.com/tao3k/marlin-mygo/runtime/kit"

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

			themes := bc.AvailableThemes
			if len(themes) == 0 {
				themes = theme.Catalog()
			}
			theme.FieldRow(c, "theme-profile", "Doom One · Tokyo Night · Nord Frost · Light (~/.config/marlin/ui/)", th, func() {
				thIdx := 0
				labels := make([]string, len(themes))
				for i, tp := range themes {
					labels[i] = tp.Name
					if tp.ID == th.ID {
						thIdx = i
					}
				}
				seg := ui.Segmented(c.Key("set-th-seg"), &thIdx, labels...).Label("Theme Profile")
				if seg.Changed() && thIdx >= 0 && thIdx < len(themes) && bc.OnSetTheme != nil {
					bc.OnSetTheme(themes[thIdx].ID)
				}
			})
			theme.FieldRow(c, "glass-surface", "optional Liquid Glass GPU material + Oklab aurora (SPC t v)", th, func() {
				on := bc.Modules.Glass
				sw := ui.Switch(c.Key("set-glass-tog"), &on).Label("Liquid Glass")
				if sw.Changed() && bc.OnToggleGlass != nil {
					bc.OnToggleGlass()
				}
				lbl, tone := "Matte", th.InkMuted
				if bc.Modules.Glass {
					lbl, tone = "Liquid Glass", th.Info
				}
				ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(tone).SingleLine()
			})
			theme.FieldRow(c, "glass-opacity", "frosted glass transparency over OS vibrancy · tokens.json --glass-opacity (SPC t o)", th, func() {
				op := bc.GlassOpacity
				if op <= 0 {
					op = kit.DefaultGlassOpacity
				}
				pct := int(math.Round(float64(op) * 100))
				lbl := fmt.Sprintf("Custom (%d%%)", pct)
				switch {
				case op <= 0.26:
					lbl = fmt.Sprintf("Clear (%d%%)", pct)
				case op <= 0.42:
					lbl = fmt.Sprintf("Balanced (%d%%)", pct)
				case op <= 0.62:
					lbl = fmt.Sprintf("Frosted (%d%%)", pct)
				default:
					lbl = fmt.Sprintf("Deep (%d%%)", pct)
				}
				tone := th.InkMuted
				if bc.Modules.Glass {
					tone = th.Info
				}
				if theme.ActionButton(c, "set-glass-opacity", lbl, tone, bc.Modules.Glass, th).Clicked() && bc.OnCycleGlassOpacity != nil {
					bc.OnCycleGlassOpacity()
				}
			})
			theme.FieldRow(c, "live-motion", "optional 120Hz spring gauges, hover lift & live clock (SPC t a)", th, func() {
				on := bc.Modules.Motion
				sw := ui.Switch(c.Key("set-motion-tog"), &on).Label("120Hz Spring Motion")
				if sw.Changed() && bc.OnToggleMotion != nil {
					bc.OnToggleMotion()
				}
				lbl, tone := "Static", th.InkMuted
				if bc.Modules.Motion {
					lbl, tone = "120Hz Spring", th.Accent
				}
				ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(tone).SingleLine()
			})
			theme.FieldRow(c, "zen-mode", "hide tabs, modeline & resting echo bar (SPC t z)", th, func() {
				on := bc.ZenMode
				sw := ui.Switch(c.Key("set-zen-tog"), &on).Label("Zen Mode")
				if sw.Changed() && bc.OnToggleZenMode != nil {
					bc.OnToggleZenMode()
				}
				lbl, tone := "Off", th.InkMuted
				if bc.ZenMode {
					lbl, tone = "Active", th.Success
				}
				ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(tone).SingleLine()
			})
			theme.FieldRow(c, "stylesheets", "~/.config/marlin/ui/{manifest.toml, theme.css, dashboard.sexp|toml, widgets/<id>/}", th, func() {
				if theme.ActionButton(c, "set-reload-themes", "Reload UI & Widgets (SPC t r)", th.Accent, false, th).Clicked() && bc.OnReloadThemes != nil {
					bc.OnReloadThemes()
				}
			})
			theme.FieldRow(c, "active-session", "", th, func() {
				theme.InlineInput(c, "set-sess", &st.SessionID, "marlin-main", 250, th)
			})
			theme.FieldRow(c, "default-remote-host", "optional TLS gRPC", th, func() {
				theme.InlineInput(c, "set-rem", &st.RemoteHost, "host:50051", 250, th)
			})
			theme.FieldRow(c, "durable-streaming", "ConversationJournal", th, func() {
				on := st.RealSession
				sw := ui.Switch(c.Key("set-real-tog"), &on).Label("Durable Streaming")
				if sw.Changed() {
					st.RealSession = on
				}
				lbl, tone := "Off", th.InkMuted
				if st.RealSession {
					lbl, tone = "On", th.Success
				}
				ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(tone).SingleLine()
			})
			theme.FieldRow(c, "actions", "", th, func() {
				if theme.ActionButton(c, "set-save-btn", "Save Preferences", th.Accent, true, th).Clicked() {
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
				if theme.ActionButton(c, "add-rem-go", "Connect", th.Accent, true, th).Clicked() {
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
							if theme.ActionButton(c, "set-oauth-"+string(prov.ID), "Sign in with "+prov.DisplayName, th.Accent, true, th).Clicked() {
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
					if theme.ActionButton(c, "set-oidc-go", "Connect OIDC", th.Accent, true, th).Clicked() {
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
						on := stFeat.Enabled
						sw := ui.Switch(c.Key("set-feat-"+string(stFeat.Meta.ID)), &on).Label(string(stFeat.Meta.ID))
						if sw.Changed() {
							_ = bc.FeatureReg.SetEnabled(stFeat.Meta.ID, on, bc.FeatureLedger, bc.FeatureHost)
							if bc.Adaptive != nil {
								bc.Adaptive.SetFeatureHost(bc.FeatureHost, bc.Adaptive.Desktop.GovernanceUI)
							}
						}
						lbl, tone := "Disabled", th.InkMuted
						if stFeat.Enabled {
							lbl, tone = "Enabled", th.Success
						}
						ui.Text(c, lbl).Font(kit.FontDisplay).FontSize(theme.FontSizeMeta).FontWeight(600).TextColor(tone).SingleLine()
					})
				}
			}
		})
	})
}
