// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderPackages(c *ui.Context, th theme.Theme, bc *Context) {
	ui.Column(c.Key("buf-packages")).Fill().AlignItems(ui.Stretch).Children(func() {
		var snap marketplace.PackageStoreSnapshot
		activeFilter := "all"
		if bc != nil {
			snap = bc.PackageSnap
			if strings.TrimSpace(bc.PackageFilter) != "" {
				activeFilter = strings.ToLower(strings.TrimSpace(bc.PackageFilter))
			}
		}

		var totalAll, totalThemes, totalPlugins, totalLayouts int
		filtered := make([]marketplace.ManagedPackage, 0, len(snap.Packages))
		for _, p := range snap.Packages {
			totalAll++
			switch p.Kind {
			case marketplace.PackageKindTheme:
				totalThemes++
			case marketplace.PackageKindPlugin:
				totalPlugins++
			case marketplace.PackageKindLayout:
				totalLayouts++
			}
			if activeFilter == "all" || string(p.Kind) == activeFilter {
				filtered = append(filtered, p)
			}
		}

		// Top Emacs Package-Menu toolbar: category filters + Official Marketplace refresh.
		barBg := theme.GlassTone(c, th.Bar, th, 0.24, 0.42)
		ui.Row(c.Key("pkg-toolbar")).Padding(8, 14).Gap(8).AlignItems(ui.Center).
			Background(barBg).Children(func() {
			filterSpecs := []struct {
				id    string
				label string
			}{
				{"all", fmt.Sprintf("All (%d)", totalAll)},
				{"theme", fmt.Sprintf("Themes (%d)", totalThemes)},
				{"plugin", fmt.Sprintf("Plugins (%d)", totalPlugins)},
				{"layout", fmt.Sprintf("Layouts (%d)", totalLayouts)},
			}
			for _, fs := range filterSpecs {
				spec := fs
				if theme.ActionButton(c, "pkg-flt-"+spec.id, spec.label, th.Ink, activeFilter == spec.id, th).Clicked() {
					if bc != nil && bc.OnPackageFilter != nil {
						bc.OnPackageFilter(spec.id)
					}
				}
			}

			ui.Box(c).Grow(1)

			statusText := "Official Registry · tao3k/marlin-ui-marketplace"
			statusColor := th.InkMuted
			if snap.Refreshing {
				statusText = "Syncing Official Marketplace…"
				statusColor = th.Info
			} else if snap.LastError != "" {
				statusText = "Offline cache (sync error)"
				statusColor = th.Warning
			} else if !snap.SyncedAt.IsZero() {
				statusText = fmt.Sprintf("Synced %s", snap.SyncedAt.Local().Format("15:04:05"))
				statusColor = th.Success
			}
			ui.Text(c, statusText).Font(theme.FontMono).FontSize(theme.FontSizeMeta).
				FontWeight(theme.GlassWeight(c, 400)).TextColor(statusColor).NoWrap()

			refreshLabel := "Refresh Marketplace"
			if snap.Refreshing {
				refreshLabel = "Syncing…"
			}
			if theme.ActionButton(c, "pkg-refresh", refreshLabel, th.Accent, snap.Refreshing, th).Clicked() {
				if bc != nil && bc.OnPackageRefresh != nil {
					bc.OnPackageRefresh()
				}
			}
		})
		ui.Box(c).Height(1).Background(kit.WithAlpha(th.Border, 0.35))

		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "PACKAGE", Width: 210},
			{Label: "KIND", Width: 82},
			{Label: "STATUS", Width: 112},
			{Label: "SUMMARY"},
			{Label: "ACTIONS", Width: 176},
		}, th)

		ui.Scroll(c.Key("pkg-scroll")).Grow(1).Children(func() {
			ui.Column(c).AlignItems(ui.Stretch).Children(func() {
				if len(filtered) == 0 {
					ui.Row(c).Padding(16, 18).Children(func() {
						ui.Text(c, "No packages in this category. Click Refresh Marketplace to sync.").
							Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted)
					})
					return
				}
				for idx, item := range filtered {
					pkg := item
					selected := bc != nil && idx == bc.TableCursor
					renderPackageRow(c, pkg, selected, th, bc)
				}
			})
		})
	})
}

func renderPackageRow(c *ui.Context, pkg marketplace.ManagedPackage, selected bool, th theme.Theme, bc *Context) {
	bg := ui.Transparent
	if selected {
		bg = theme.GlassTone(c, th.SurfaceElevated, th, 0.34, 0.56)
	}
	kindCol := th.Info
	switch pkg.Kind {
	case marketplace.PackageKindTheme:
		kindCol = th.Accent
	case marketplace.PackageKindLayout:
		kindCol = th.Warning
	}
	statusLbl := string(pkg.Status)
	statusCol := th.InkMuted
	switch pkg.Status {
	case marketplace.PackageStatusInstalled:
		if pkg.Active {
			statusLbl = "active"
			statusCol = th.Accent
		} else {
			statusLbl = "installed"
			statusCol = th.Success
		}
	case marketplace.PackageStatusUpdateAvailable:
		statusLbl = "update!"
		statusCol = th.Warning
	}

	row := ui.Row(c.Key("pkg-row-"+pkg.ID)).Padding(7, 14).Gap(10).AlignItems(ui.Center).
		Background(bg)
	row.Children(func() {
		ui.Column(c).Width(210).Gap(1).Children(func() {
			ui.Text(c, pkg.Name).Font(theme.FontMono).FontSize(theme.FontSizeUI).
				FontWeight(600).TextColor(th.Ink).NoWrap().MaxLines(1)
			ui.Text(c, pkg.ID).Font(theme.FontMono).FontSize(10).
				FontWeight(theme.GlassWeight(c, 400)).TextColor(th.InkMuted).NoWrap().MaxLines(1)
		})
		ui.Text(c, string(pkg.Kind)).Width(82).Font(theme.FontMono).FontSize(theme.FontSizeMeta).
			FontWeight(600).TextColor(kindCol).NoWrap()
		ui.Text(c, statusLbl).Width(112).Font(theme.FontMono).FontSize(theme.FontSizeMeta).
			FontWeight(600).TextColor(statusCol).NoWrap()
		ui.Text(c, pkg.Summary).Grow(1).MinWidth(0).Font(theme.FontMono).FontSize(theme.FontSizeMeta).
			FontWeight(theme.GlassWeight(c, 400)).TextColor(th.InkMuted).NoWrap().MaxLines(1)

		ui.Row(c).Width(176).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			switch pkg.Status {
			case marketplace.PackageStatusAvailable:
				if theme.ActionButton(c, "pkg-inst-"+pkg.ID, "Install", th.Success, false, th).Clicked() {
					if bc != nil && bc.OnPackageInstall != nil {
						bc.OnPackageInstall(pkg.ID)
					}
				}
			case marketplace.PackageStatusUpdateAvailable:
				if theme.ActionButton(c, "pkg-upd-"+pkg.ID, "Update", th.Warning, true, th).Clicked() {
					if bc != nil && bc.OnPackageUpdate != nil {
						bc.OnPackageUpdate(pkg.ID)
					}
				}
				if theme.ActionButton(c, "pkg-rm-"+pkg.ID, "Remove", th.Danger, false, th).Clicked() {
					if bc != nil && bc.OnPackageRemove != nil {
						bc.OnPackageRemove(pkg.ID)
					}
				}
			case marketplace.PackageStatusInstalled:
				if pkg.Kind == marketplace.PackageKindPlugin {
					if theme.ActionButton(c, "pkg-open-"+pkg.ID, "Open", th.Info, false, th).Clicked() {
						if bc != nil && bc.OnPackageApply != nil {
							bc.OnPackageApply(pkg.ID)
						}
					}
				} else if (pkg.Kind == marketplace.PackageKindTheme || pkg.Kind == marketplace.PackageKindLayout) && !pkg.Active {
					if theme.ActionButton(c, "pkg-use-"+pkg.ID, "Apply", th.Accent, false, th).Clicked() {
						if bc != nil && bc.OnPackageApply != nil {
							bc.OnPackageApply(pkg.ID)
						}
					}
				}
				if theme.ActionButton(c, "pkg-rm-"+pkg.ID, "Remove", th.Danger, false, th).Clicked() {
					if bc != nil && bc.OnPackageRemove != nil {
						bc.OnPackageRemove(pkg.ID)
					}
				}
			}
		})
	})
}

// RenderMountedPlugin renders an installed Official Marketplace plugin mounted
// as a live Emacs Tab in the primary viewport.
func RenderMountedPlugin(c *ui.Context, th theme.Theme, pkg marketplace.ManagedPackage, bc *Context) {
	ui.Column(c.Key("mounted-plugin-"+pkg.ID)).Fill().AlignItems(ui.Stretch).Children(func() {
		barBg := theme.GlassTone(c, th.Bar, th, 0.24, 0.42)
		ui.Row(c.Key("mplug-bar-"+pkg.ID)).Padding(8, 14).Gap(10).AlignItems(ui.Center).
			Background(barBg).Children(func() {
			ui.Text(c, pkg.Name).Font(theme.FontMono).FontSize(theme.FontSizeUI).
				FontWeight(700).TextColor(th.Ink).NoWrap()
			runtimeLbl := "declarative"
			if pkg.PluginEntry != nil && pkg.PluginEntry.Runtime != "" {
				runtimeLbl = string(pkg.PluginEntry.Runtime)
			}
			ui.Text(c, fmt.Sprintf("%s · %s · %s", pkg.ID, runtimeLbl, pkg.Version)).
				Font(theme.FontMono).FontSize(theme.FontSizeMeta).
				FontWeight(theme.GlassWeight(c, 400)).TextColor(th.InkMuted).NoWrap()
			ui.Box(c).Grow(1)
			if theme.ActionButton(c, "mplug-back-"+pkg.ID, "Packages", th.Info, false, th).Clicked() {
				if bc != nil && bc.OnSelectBuffer != nil {
					bc.OnSelectBuffer(Packages)
				}
			}
			if theme.ActionButton(c, "mplug-rm-"+pkg.ID, "Remove", th.Danger, false, th).Clicked() {
				if bc != nil && bc.OnPackageRemove != nil {
					bc.OnPackageRemove(pkg.ID)
				}
			}
		})
		ui.Box(c).Height(1).Background(kit.WithAlpha(th.Border, 0.35))

		cardBg := theme.GlassTone(c, th.SurfaceElevated, th, 0.34, 0.54)
		ui.Column(c).Grow(1).Padding(24, 28).Gap(14).Children(func() {
			ui.Column(c.Key("mplug-card-"+pkg.ID)).Padding(18, 20).Gap(10).Radius(14).
				Background(cardBg).Border(1, kit.WithAlpha(th.Border, 0.40)).Children(func() {
				ui.Text(c, "OFFICIAL MARKETPLACE PLUGIN").Font(theme.FontMono).FontSize(11).
					FontWeight(700).TextColor(th.Accent).NoWrap()
				ui.Text(c, pkg.Name).Font(theme.FontMono).FontSize(18).
					FontWeight(700).TextColor(th.Ink).NoWrap()
				ui.Text(c, pkg.Summary).Font(theme.FontMono).FontSize(theme.FontSizeUI).
					FontWeight(theme.GlassWeight(c, 400)).TextColor(th.Ink)
				slot := "workbench.main"
				if pkg.PluginEntry != nil && pkg.PluginEntry.SlotID != "" {
					slot = string(pkg.PluginEntry.SlotID)
				}
				ui.Text(c, fmt.Sprintf("Trust: %s  |  Publisher: %s  |  Slot: %s  |  Revision: %s",
					pkg.TrustTier, pkg.PublisherID, slot, pkg.Version)).
					Font(theme.FontMono).FontSize(theme.FontSizeMeta).TextColor(th.InkMuted)
			})
		})
	})
}
