// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/control/marketplace"
	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func (f *Frame) initPackageStore(store *marketplace.PackageStore) {
	if store == nil {
		if def, err := marketplace.NewPackageStore(marketplace.PackageStoreConfig{}); err == nil {
			store = def
		}
	}
	f.pkgStore = store
	if f.packageFilter == "" {
		f.packageFilter = "all"
	}
	if store == nil {
		return
	}
	f.pkgRefreshLane = async.NewFencedLane[struct{}, marketplace.PackageStoreSnapshot](
		func(ctx context.Context, _ struct{}) (marketplace.PackageStoreSnapshot, error) {
			return store.Refresh(ctx)
		},
		func(_ uint64, snap marketplace.PackageStoreSnapshot) {
			if snap.LastError != "" {
				f.Echo("Marketplace refresh warning: " + snap.LastError)
				return
			}
			f.Echo(fmt.Sprintf("Marketplace synced: %d official packages (%s)", len(snap.Packages), snap.Source))
		},
	)
}

func (f *Frame) closePackageStore() {
	if f.pkgRefreshLane != nil {
		f.pkgRefreshLane.Close()
	}
}

// OpenPackages opens the Emacs *Packages* Official Marketplace manager buffer.
func (f *Frame) OpenPackages() {
	f.SelectBuffer(buffer.Packages)
}

// RefreshPackagesAsync fetches the latest Official Marketplace registry off-lock
// through an epoch-fenced coalescing lane (never blocking the 120Hz GPU thread).
func (f *Frame) RefreshPackagesAsync() {
	if f.pkgRefreshLane == nil {
		f.Echo("Marketplace package store unavailable")
		return
	}
	f.Echo("Refreshing tao3k/marlin-ui-marketplace…")
	f.pkgRefreshLane.Submit(struct{}{})
}

// SetPackageFilter sets the active *Packages* category filter ("all", "theme", "plugin", "layout").
func (f *Frame) SetPackageFilter(kind string) {
	norm := strings.ToLower(strings.TrimSpace(kind))
	switch norm {
	case "theme", "plugin", "layout":
	default:
		norm = "all"
	}
	f.mu.Lock()
	f.packageFilter = norm
	f.tableCursor = 0
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
}

// InstallPackage installs an admitted package from the Official Marketplace registry.
func (f *Frame) InstallPackage(id string) {
	if f.pkgStore == nil {
		return
	}
	pkg, err := f.pkgStore.Install(id)
	if err != nil {
		f.Echo("Install error: " + err.Error())
		return
	}
	if pkg.Kind == marketplace.PackageKindTheme && pkg.ThemePack != nil {
		f.registerMarketplaceTheme(*pkg.ThemePack)
	}
	f.Echo(fmt.Sprintf("Installed %s (%s)", pkg.Name, pkg.Kind))
	f.requestFrame()
}

// UpdatePackage upgrades an installed Official Marketplace package to the latest synced revision.
func (f *Frame) UpdatePackage(id string) {
	if f.pkgStore == nil {
		return
	}
	pkg, err := f.pkgStore.Update(id)
	if err != nil {
		f.Echo("Update error: " + err.Error())
		return
	}
	if pkg.Kind == marketplace.PackageKindTheme && pkg.ThemePack != nil {
		f.registerMarketplaceTheme(*pkg.ThemePack)
		if pkg.Active {
			f.SetTheme(marketplace.ThemePackSlug(pkg.ID))
		}
	}
	f.Echo(fmt.Sprintf("Updated %s (%s)", pkg.Name, pkg.Version))
	f.requestFrame()
}

// RemovePackage uninstalls a package from the local Marketplace package store.
func (f *Frame) RemovePackage(id string) {
	if f.pkgStore == nil {
		return
	}
	if err := f.pkgStore.Remove(id); err != nil {
		f.Echo("Remove error: " + err.Error())
		return
	}
	f.unmountPluginTab(id)
	if f.onReloadThemeBundle != nil {
		f.ReloadThemesAsync()
	}
	f.Echo("Removed package " + id)
	f.requestFrame()
}

// ApplyPackage activates an installed Official Marketplace theme, layout
// perspective, or mounts a plugin into the Emacs Tab bar live.
func (f *Frame) ApplyPackage(id string) {
	if f.pkgStore == nil {
		return
	}
	snap := f.pkgStore.Snapshot()
	var found *marketplace.ManagedPackage
	for i := range snap.Packages {
		if snap.Packages[i].ID == id {
			p := snap.Packages[i]
			found = &p
			break
		}
	}
	if found == nil {
		f.Echo("Package not found: " + id)
		return
	}
	if found.Status == marketplace.PackageStatusAvailable {
		pkg, err := f.pkgStore.Install(id)
		if err != nil {
			f.Echo("Install error: " + err.Error())
			return
		}
		found = &pkg
	}
	_ = f.pkgStore.SetActive(found.ID)
	switch found.Kind {
	case marketplace.PackageKindTheme:
		if found.ThemePack != nil {
			f.registerMarketplaceTheme(*found.ThemePack)
		}
		f.SetTheme(marketplace.ThemePackSlug(found.ID))
	case marketplace.PackageKindLayout:
		f.applyMarketplaceLayout(*found)
	case marketplace.PackageKindPlugin:
		f.mountAndFocusPlugin(*found)
	}
	f.requestFrame()
}

func (f *Frame) mountAndFocusPlugin(pkg marketplace.ManagedPackage) {
	f.miniState.Close()
	f.meowState.ClearKeypad()
	f.mu.Lock()
	exists := false
	for i := range f.mountedPlugins {
		if f.mountedPlugins[i].ID == pkg.ID {
			f.mountedPlugins[i] = pkg
			exists = true
			break
		}
	}
	if !exists {
		f.mountedPlugins = append(f.mountedPlugins, pkg)
	}
	f.activePluginID = pkg.ID
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.Echo("Mounted plugin: " + pkg.Name)
}

func (f *Frame) unmountPluginTab(id string) {
	f.mu.Lock()
	next := f.mountedPlugins[:0]
	for _, p := range f.mountedPlugins {
		if p.ID != id {
			next = append(next, p)
		}
	}
	f.mountedPlugins = append([]marketplace.ManagedPackage(nil), next...)
	if f.activePluginID == id {
		f.activePluginID = ""
		f.activeBuffer = buffer.Packages
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
}

func (f *Frame) applyMarketplaceLayout(pkg marketplace.ManagedPackage) {
	persp := pkg.Category
	if pkg.LayoutSpec != nil && strings.TrimSpace(pkg.LayoutSpec.Perspective) != "" {
		persp = strings.TrimSpace(pkg.LayoutSpec.Perspective)
	}
	switch persp {
	case "focus":
		f.mu.Lock()
		f.openTabs = []buffer.ID{buffer.Dashboard, buffer.Conversation}
		f.railPinned = false
		f.activeBuffer = buffer.Conversation
		f.activeFeatureID = ""
		f.activeExtID = ""
		f.activePluginID = ""
		f.publishSnapshotLocked()
		f.mu.Unlock()
	case "cockpit":
		f.mu.Lock()
		f.openTabs = []buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets, buffer.Packages}
		f.railPinned = true
		f.activeBuffer = buffer.Dashboard
		f.activeFeatureID = ""
		f.activeExtID = ""
		f.activePluginID = ""
		f.publishSnapshotLocked()
		f.mu.Unlock()
	case "review":
		f.mu.Lock()
		f.openTabs = []buffer.ID{buffer.Dashboard, buffer.Conversation, buffer.Sessions, buffer.Targets}
		f.activeBuffer = buffer.Sessions
		f.activeFeatureID = ""
		f.activeExtID = ""
		f.activePluginID = ""
		f.publishSnapshotLocked()
		f.mu.Unlock()
	case "dev-git-github":
		if _, err := f.pkgStore.Install("tao3k.git-workbench"); err == nil {
			f.ApplyPackage("tao3k.git-workbench")
		}
		if _, err := f.pkgStore.Install("tao3k.github-workbench"); err == nil {
			f.ApplyPackage("tao3k.github-workbench")
		}
		f.SelectFeature("pkg:tao3k.git-workbench")
	default:
		f.SelectBuffer(buffer.Dashboard)
	}
	f.Echo("Applied layout perspective: " + pkg.Name)
}

func (f *Frame) appendMountedPluginTabs(snap frameSnapshot, featBufs []tabs.FeatureTab) ([]tabs.FeatureTab, *marketplace.ManagedPackage) {
	var active *marketplace.ManagedPackage
	for i := range snap.MountedPlugins {
		p := snap.MountedPlugins[i]
		featBufs = append(featBufs, tabs.FeatureTab{
			ID:    "pkg:" + p.ID,
			Title: "*" + p.Name + "*",
		})
		if snap.ActivePluginID == p.ID {
			cp := p
			active = &cp
		}
	}
	return featBufs, active
}

func (f *Frame) selectMountedPlugin(id string) bool {
	if !strings.HasPrefix(id, "pkg:") {
		return false
	}
	pid := strings.TrimPrefix(id, "pkg:")
	f.miniState.Close()
	f.meowState.ClearKeypad()
	f.mu.Lock()
	f.activePluginID = pid
	f.activeFeatureID = ""
	f.activeExtID = ""
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.requestFrame()
	return true
}

func renderMountedPluginViewport(c *ui.Context, th theme.Theme, pkg *marketplace.ManagedPackage, bc *buffer.Context) bool {
	if pkg == nil {
		return false
	}
	buffer.RenderMountedPlugin(c, th, *pkg, bc)
	return true
}

func (f *Frame) registerMarketplaceTheme(pack marketplace.ThemePack) {
	slug := marketplace.ThemePackSlug(string(pack.ID))
	if slug == "" {
		return
	}
	isLight := pack.Appearance == "light"
	base := theme.NordBento()
	if isLight {
		base = theme.DoomOneLight()
	}
	css := marketplace.RenderThemePackCSS(pack)
	built, err := theme.BuildFromCSS(slug, pack.Name, isLight, &base, css)
	if err != nil {
		return
	}
	f.mu.Lock()
	replaced := false
	for i := range f.themeCatalog {
		if f.themeCatalog[i].ID == built.ID {
			f.themeCatalog[i] = built
			replaced = true
			break
		}
	}
	if !replaced {
		f.themeCatalog = append(f.themeCatalog, built)
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
}

func (f *Frame) packageSnapshot() marketplace.PackageStoreSnapshot {
	if f.pkgStore == nil {
		return marketplace.PackageStoreSnapshot{}
	}
	return f.pkgStore.Snapshot()
}
