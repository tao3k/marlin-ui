// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
	"github.com/tao3k/marlin-mygo/runtime/style"
	"github.com/tao3k/marlin-mygo/runtime/widgets"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/tabs"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

func (f *Frame) applyThemeBundleLocked(bundle appconfig.UIThemeBundle, preferredThemeID string, preserveOnErr bool) []string {
	catalog := theme.Catalog()
	warnings := append([]string(nil), bundle.Warnings...)

	findIn := func(list []theme.Theme, id string) (theme.Theme, bool) {
		norm := strings.ToLower(strings.TrimSpace(id))
		for _, item := range list {
			if item.ID == norm {
				return item, true
			}
		}
		return theme.Theme{}, false
	}

	f.themeSurfaces = make(map[string]appconfig.SurfaceConfig, len(bundle.Packs))
	for _, pack := range bundle.Packs {
		f.themeSurfaces[strings.ToLower(strings.TrimSpace(pack.ID))] = pack.Surface
		base := theme.NordBento()
		if pack.IsLight {
			base = theme.DoomOneLight()
		}
		if strings.TrimSpace(pack.Base) != "" {
			if found, ok := findIn(catalog, pack.Base); ok {
				base = found
			}
		}
		built, err := theme.BuildFromCSS(pack.ID, pack.Name, pack.IsLight, &base, pack.StyleCSS)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("themes/%s/style.css: %v", pack.ID, err))
			continue
		}
		replaced := false
		for i := range catalog {
			if catalog[i].ID == built.ID {
				catalog[i] = built
				replaced = true
				break
			}
		}
		if !replaced {
			catalog = append(catalog, built)
		}
	}

	f.themeCatalog = catalog
	selected, ok := findIn(catalog, preferredThemeID)
	if !ok {
		selected = catalog[0]
	}

	// Cascade optional W3C Design Tokens (tokens.json with oklch() & aliases) first.
	tokensRaw := strings.TrimSpace(bundle.TokensJSON)
	if tokensRaw != "" {
		cascaded, err := selected.WithDesignTokensJSON(tokensRaw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("tokens.json: %v", err))
			if preserveOnErr {
				return warnings
			}
		} else {
			selected = cascaded
			f.userTokensJSON = tokensRaw
		}
	} else {
		f.userTokensJSON = ""
	}

	// Merge global theme.css plus any per-widget optional style.css layers.
	var combinedCSS strings.Builder
	if strings.TrimSpace(bundle.OverrideCSS) != "" {
		combinedCSS.WriteString(bundle.OverrideCSS)
		combinedCSS.WriteByte('\n')
	}
	for _, w := range bundle.Widgets {
		if strings.TrimSpace(w.StyleCSS) != "" {
			combinedCSS.WriteString(w.StyleCSS)
			combinedCSS.WriteByte('\n')
		}
	}
	overrideCSS := strings.TrimSpace(combinedCSS.String())
	if overrideCSS != "" {
		cascaded, err := selected.WithOverrideCSS(overrideCSS)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("theme.css: %v", err))
			if preserveOnErr {
				return warnings
			}
		} else {
			selected = cascaded
			f.userOverrideCSS = overrideCSS
		}
	} else {
		f.userOverrideCSS = ""
	}
	// Audit user-customized palettes with APCA perceptual Lc contrast thresholds.
	if f.userTokensJSON != "" || f.userOverrideCSS != "" {
		warnings = append(warnings, style.AuditPaletteContrast(selected.Ink, selected.InkMuted, selected.Canvas, f.uiModules.Glass)...)
	}
	f.Theme = selected
	f.surfaceCfg = bundle.Manifest.Surface
	if bundle.Manifest.Surface.Glass != nil {
		f.uiModules.Glass = *bundle.Manifest.Surface.Glass
	}
	if bundle.Manifest.Surface.Motion != nil {
		f.uiModules.Motion = *bundle.Manifest.Surface.Motion
	}
	// Reset runtime opacity override on reload so manifest.toml / theme.toml / tokens.json take effect.
	f.uiModules.GlassOpacity = 0

	// Compile optional user Bento widgets (JSON from TS SDK, S-exp, or TOML) fail-closed.
	var userWidgets []*widgets.Node
	for _, w := range bundle.Widgets {
		var node *widgets.Node
		var err error
		switch w.Format {
		case "json":
			node, err = widgets.ParseWidgetJSON(w.Definition)
		case "sexp":
			node, err = widgets.ParseSexp(w.Definition)
		default:
			node, err = widgets.ParseWidgetTOML(w.ID, w.Definition)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("widgets/%s: %v", w.ID, err))
			continue
		}
		userWidgets = append(userWidgets, node)
	}

	if strings.TrimSpace(bundle.DashboardDef) != "" {
		var customDash *widgets.Node
		var err error
		switch bundle.DashboardFormat {
		case "json":
			customDash, err = widgets.ParseWidgetJSON(bundle.DashboardDef)
		case "sexp":
			customDash, err = widgets.ParseSexp(bundle.DashboardDef)
		default:
			customDash, err = widgets.ParseDashboardTOML(bundle.DashboardDef)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("dashboard.%s: %v", bundle.DashboardFormat, err))
			if !preserveOnErr || f.dashboardNode == nil {
				f.dashboardNode = widgets.DefaultDashboard(userWidgets)
			}
		} else {
			f.dashboardNode = customDash
		}
	} else {
		f.dashboardNode = widgets.DefaultDashboard(userWidgets)
	}

	// Optional custom floating left-edge sidebar spec (sidebar.json emitted by @marlin/ui-kit).
	if strings.TrimSpace(bundle.SidebarDef) != "" {
		spec, err := widgets.ParseSidebarJSON(bundle.SidebarDef)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("sidebar.%s: %v", bundle.SidebarFormat, err))
		} else {
			f.customSidebar = spec
			if spec.Pinned {
				f.railPinned = true
			}
		}
	} else {
		f.customSidebar = nil
	}

	// Load declarative TOML command & Hydra keymap catalogs (defaults + ~/.config/marlin/ui/ overrides).
	cmds := loadDefaultCommands()
	if strings.TrimSpace(bundle.CommandsTOML) != "" {
		if extra, err := parseCommandsTOML(bundle.CommandsTOML); err != nil {
			warnings = append(warnings, fmt.Sprintf("commands.toml: %v", err))
		} else {
			byName := make(map[string]int, len(cmds))
			for i, c := range cmds {
				byName[c.Name] = i
			}
			for _, ec := range extra {
				if idx, exists := byName[ec.Name]; exists {
					cmds[idx] = ec
				} else {
					byName[ec.Name] = len(cmds)
					cmds = append(cmds, ec)
				}
			}
		}
	}
	f.tomlCommands = cmds

	hydras := loadDefaultKeymaps()
	if strings.TrimSpace(bundle.KeymapsTOML) != "" {
		if extra, err := parseKeymapsTOML(bundle.KeymapsTOML); err != nil {
			warnings = append(warnings, fmt.Sprintf("keymaps.toml: %v", err))
		} else {
			for pref, def := range extra {
				hydras[pref] = def
			}
		}
	}
	f.tomlHydras = hydras

	return warnings
}

func (f *Frame) findThemeLocked(id string) theme.Theme {
	norm := strings.ToLower(strings.TrimSpace(id))
	for _, th := range f.themeCatalog {
		if th.ID == norm {
			return th
		}
	}
	if len(f.themeCatalog) > 0 {
		return f.themeCatalog[0]
	}
	return theme.NordBento()
}

func (f *Frame) buildLiveSignals(th theme.Theme, modelLbl, sessLbl string, pendingGates int) *widgets.Signals {
	sig := widgets.NewSignals()

	now := time.Now()
	hr := now.Hour()
	ampm := "AM"
	if hr >= 12 {
		ampm = "PM"
	}
	hr12 := hr % 12
	if hr12 == 0 {
		hr12 = 12
	}
	sig.Set("clock.hour", fmt.Sprintf("%02d", hr12))
	sig.Set("clock.min", fmt.Sprintf("%02d", now.Minute()))
	sig.Set("clock.sec", fmt.Sprintf("%02d", now.Second()))
	sig.Set("clock.ampm", ampm)
	sig.Set("clock.weekday", now.Weekday().String())

	up := time.Since(f.startedAt)
	if up < 0 {
		up = 0
	}
	sig.SetInt("uptime.hours", int(up.Hours()))
	sig.SetInt("uptime.mins", int(up.Minutes())%60)

	status := "Ready"
	hostLabel := "local"
	if f.Link != nil {
		if strings.TrimSpace(f.Link.Endpoint) != "" {
			hostLabel = f.Link.Endpoint
		}
		switch f.Link.Mode {
		case hostlink.ModeSidecar:
			status = "Online"
		case hostlink.ModeRemoteGRPC:
			status = "Remote"
		default:
			status = "Local"
		}
	}
	sessID := f.AppCfg.SessionID
	if sessLbl != "" {
		sessID = sessLbl
	}
	turns := 0
	sessCount := 1
	targetCount := 1
	if f.Adaptive != nil && f.Adaptive.Desktop != nil {
		if f.Adaptive.Desktop.ActiveSessionID != "" {
			sessID = f.Adaptive.Desktop.ActiveSessionID
		}
		if f.Adaptive.Desktop.Transcript != nil {
			turns = f.Adaptive.Desktop.Transcript.Len()
			if f.Adaptive.Desktop.Transcript.InFlight() {
				status = "Working"
			}
		}
		if n := len(f.Adaptive.Desktop.Sessions); n > 0 {
			sessCount = n
		}
	}
	if f.Link != nil && f.Link.Targets != nil {
		if n := len(f.Link.Targets.List()); n > 0 {
			targetCount = n
		}
	}
	if modelLbl == "" {
		modelLbl = "marlin-core"
	}

	tasksDone, tasksTotal, agentsOnline, agentsTotal := 0, 0, 1, 1
	if f.roomStatsFn != nil {
		d, tot, ao, at := f.roomStatsFn()
		tasksDone, tasksTotal, agentsOnline, agentsTotal = d, tot, ao, at
	}
	tasksProg := 0.25
	if tasksTotal > 0 {
		tasksProg = float64(tasksDone) / float64(tasksTotal)
	}
	agentsProg := 0.75
	if agentsTotal > 0 {
		agentsProg = float64(agentsOnline) / float64(agentsTotal)
	}
	sessProg := float64(turns+1) / 10.0
	if sessProg > 1.0 {
		sessProg = 1.0
	}
	targetProg := 0.90
	if f.Link != nil && f.Link.Mode == hostlink.ModeStandalone {
		targetProg = 0.45
	}

	sig.Set("agent.status", status)
	sig.Set("agent.model", modelLbl)
	sig.Set("agent.session", sessID)
	sig.Set("agent.host", hostLabel)
	sig.Set("theme.name", th.Name)
	mods := f.UIModules()
	onOff := func(b bool) string {
		if b {
			return "ON"
		}
		return "OFF"
	}
	sig.Set("mod.glass", onOff(mods.Glass))
	sig.Set("mod.glass_opacity", fmt.Sprintf("%d%%", int(f.EffectiveGlassOpacity(th)*100+0.5)))
	sig.Set("mod.motion", onOff(mods.Motion))
	sig.Set("mod.zen", onOff(mods.Zen))
	sig.SetInt("turns.count", turns)
	sig.SetInt("sessions.count", sessCount)
	sig.SetInt("targets.count", targetCount)
	sig.SetInt("tasks.completed", tasksDone)
	sig.SetInt("tasks.total", tasksTotal)
	sig.SetFloat("tasks.progress", tasksProg)
	sig.SetInt("agents.online", agentsOnline)
	sig.SetInt("agents.total", agentsTotal)
	sig.SetFloat("agents.progress", agentsProg)
	sig.SetFloat("sessions.progress", sessProg)
	sig.SetFloat("targets.progress", targetProg)
	sig.SetInt("gates.pending", pendingGates)
	if pendingGates > 0 {
		sig.Set("gates.badge", fmt.Sprintf("%d Gate", pendingGates))
	} else {
		sig.Set("gates.badge", "Tasks")
	}
	sig.SetInt("buffers.count", len(buffer.Catalog()))

	return sig
}

func (f *Frame) dispatchWidgetAction(action string) {
	act := strings.TrimSpace(action)
	if act == "" {
		return
	}
	if strings.HasPrefix(act, "buffer:") {
		f.SelectBuffer(buffer.ID(strings.TrimPrefix(act, "buffer:")))
		return
	}
	if strings.HasPrefix(act, "feature:") {
		f.SelectFeature(act)
		return
	}
	if strings.HasPrefix(act, "hydra:") {
		f.meowState.SetKeypad(strings.TrimPrefix(act, "hydra:"))
		f.invalidate()
		return
	}
	if strings.HasPrefix(act, "theme:") {
		f.SetTheme(strings.TrimPrefix(act, "theme:"))
		return
	}
	if strings.HasPrefix(act, "island:") {
		if id, ev, ok := strings.Cut(strings.TrimPrefix(act, "island:"), ":"); ok && id != "" && ev != "" {
			if f.onWebIslandAction != nil && f.onWebIslandAction(id, ev) {
				f.Echo("Island " + id + " <- " + ev)
			} else {
				f.Echo("Island " + id + " not active")
			}
		}
		return
	}
	if strings.HasPrefix(act, "rpc:") {
		name := strings.TrimPrefix(act, "rpc:")
		if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnCommandInvoke != nil {
			f.Adaptive.Desktop.OnCommandInvoke(name)
		}
		f.Echo("Executed " + name)
		return
	}
	switch act {
	case "cmd:helm":
		f.OpenHelm()
	case "cmd:mx":
		f.OpenCommands()
	case "cmd:switch-buffer":
		f.OpenBufferSwitch()
	case "cmd:new-session":
		f.newSession()
	case "cmd:reload-theme":
		f.ReloadThemes()
	case "cmd:switch-theme":
		f.OpenThemeSwitch()
	case "cmd:cycle-theme":
		f.CycleTheme()
	case "cmd:zen-mode":
		f.ToggleZenMode()
	case "cmd:toggle-glass":
		f.ToggleGlass()
	case "cmd:cycle-glass-opacity":
		f.CycleGlassOpacity()
	case "cmd:init-ui-config":
		dir := appconfig.DefaultUIThemeDir()
		created, err := appconfig.InitUIConfigDir(dir)
		if err != nil {
			f.Echo("init-ui-config error: " + err.Error())
		} else if len(created) == 0 {
			f.Echo("UI config already exists at " + dir)
		} else {
			f.Echo("Initialized " + strings.Join(created, ", ") + " in " + dir)
		}
	case "cmd:toggle-motion":
		f.ToggleMotion()
	case "cmd:review-gate":
		f.OpenGateReview()
	case "cmd:insert":
		f.SelectBuffer(buffer.Conversation)
		f.EnterInsert()
	case "cmd:probe-targets":
		f.probeTargets()
	case "cmd:refresh-packages":
		f.RefreshPackagesAsync()
	default:
		f.Echo("Widget action: " + act)
	}
}

func (f *Frame) buildWidgetRuntime(mods widgets.Modules, pendingGateCount int, gateTitle, gateID string, pg *mygoshell.PromptGateController) *widgets.Runtime {
	var tasks []widgets.TaskItem
	if f.roomTasksFn != nil {
		tasks = f.roomTasksFn()
	} else {
		f.mu.Lock()
		tasks = append([]widgets.TaskItem(nil), f.localTasks...)
		f.mu.Unlock()
	}
	agentWorking := false
	lastReply := ""
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.Transcript != nil {
		tr := f.Adaptive.Desktop.Transcript
		agentWorking = tr.InFlight()
		for i := tr.Len() - 1; i >= 0; i-- {
			if e, ok := tr.Entry(i); ok && e.Role == mygoshell.RoleAssistant && strings.TrimSpace(e.Text) != "" {
				lastReply = strings.TrimSpace(e.Text)
				break
			}
		}
	}
	doneCount := 0
	for _, t := range tasks {
		if t.Status == "DONE" {
			doneCount++
		}
	}
	series := []float64{0.25, 0.42, 0.38, 0.60, 0.52, 0.76, float64(doneCount+1) / float64(len(tasks)+2)}
	return &widgets.Runtime{
		Modules:        mods,
		PromptDraft:    &f.dashPromptDraft,
		NewTaskDraft:   &f.dashTaskDraft,
		Tasks:          tasks,
		ActivitySeries: series,
		AgentWorking:   agentWorking,
		LastReply:      lastReply,
		OnSubmitPrompt: f.submitPrompt,
		OnAddTask: func(title string) {
			if f.onAddTaskFn != nil {
				_ = f.onAddTaskFn(title)
			} else {
				f.mu.Lock()
				f.localTasks = append(f.localTasks, widgets.TaskItem{
					ID:     fmt.Sprintf("lt-%d", len(f.localTasks)+1),
					Title:  title,
					Status: "OPEN",
					Owner:  "@you",
				})
				f.mu.Unlock()
			}
			f.Echo("Added task: " + title)
		},
		OnCycleTask: func(id string) {
			if f.onCycleTaskFn != nil {
				_ = f.onCycleTaskFn(id)
			} else {
				f.mu.Lock()
				for i := range f.localTasks {
					if f.localTasks[i].ID == id {
						switch f.localTasks[i].Status {
						case "OPEN":
							f.localTasks[i].Status = "IN_PROGRESS"
						case "IN_PROGRESS":
							f.localTasks[i].Status = "DONE"
						default:
							f.localTasks[i].Status = "OPEN"
						}
					}
				}
				f.mu.Unlock()
			}
			f.invalidate()
		},
		OnAction: f.dispatchWidgetAction,
		WebHost:  f.WebHost,
	}
}

// EffectiveSurface resolves the active glass, chrome, viewport opacities and
// OS vibrancy mode from the latest wait-free RCU snapshot.
func (f *Frame) EffectiveSurface(th theme.Theme) (glassOp, chromeOp, viewportOp float32, vibrancy string) {
	snap := f.loadSnapshot()
	snap.Theme = th
	return effectiveSurfaceFromSnap(snap)
}

// VibrancyMode returns the declarative TOML OS vibrancy mode ("auto", "hud",
// "under-window", "sidebar", "popover", or "none") for the active theme.
func (f *Frame) VibrancyMode() string {
	_, _, _, vib := effectiveSurfaceFromSnap(f.loadSnapshot())
	return vib
}

// EffectiveGlassOpacity resolves the live card/panel frosted glass alpha.
func (f *Frame) EffectiveGlassOpacity(th theme.Theme) float32 {
	op, _, _, _ := f.EffectiveSurface(th)
	return op
}

// ActivePaletteAndOpacity returns a wait-free snapshot of the active theme's
// kit.Palette and effective glass opacity for syncing external hosts (e.g. WebViews).
func (f *Frame) ActivePaletteAndOpacity() (kit.Palette, float32) {
	snap := f.loadSnapshot()
	op, _, _, _ := effectiveSurfaceFromSnap(snap)
	return snap.Theme.ToKitPalette(), op
}

// CycleGlassOpacity steps frosted glass opacity through VisionOS / Ghostty
// presets: Clear (22%) -> Balanced (34%) -> Frosted (54%) -> Deep (74%).
func (f *Frame) CycleGlassOpacity() {
	presets := []struct {
		name string
		val  float32
	}{
		{"Clear", 0.22},
		{"Balanced", 0.34},
		{"Frosted", 0.54},
		{"Deep", 0.74},
	}
	f.mu.Lock()
	cur := kit.ResolveGlassOpacity(kit.SurfaceOptions{GlassOpacity: f.uiModules.GlassOpacity}, f.Theme.Sheet)
	nextIdx := 0
	for i, p := range presets {
		if cur < p.val-0.02 {
			nextIdx = i
			break
		}
	}
	chosen := presets[nextIdx]
	f.uiModules.GlassOpacity = chosen.val
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if f.onGlassChanged != nil {
		f.onGlassChanged(f.UIModules().Glass)
	}
	f.Echo(fmt.Sprintf("Glass opacity: %s (%d%%)", chosen.name, int(chosen.val*100+0.5)))
}

// ToggleLeftRail pins or unpins the floating auto-hide left-edge glass sidebar.
func (f *Frame) ToggleLeftRail() {
	f.mu.Lock()
	f.railPinned = !f.railPinned
	pinned := f.railPinned
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.refocusFrame.Store(true)
	if pinned {
		f.Echo("Left sidebar pinned")
	} else {
		f.Echo("Left sidebar auto-hide enabled")
	}
}

func (f *Frame) renderEdgeSidebar(
	c *ui.Context,
	snap frameSnapshot,
	mods widgets.Modules,
	effOp float32,
	featBufs []tabs.FeatureTab,
	sig *widgets.Signals,
	wrt *widgets.Runtime,
) {
	th := snap.Theme
	pinned := snap.RailPinned
	customSB := snap.CustomSidebar
	activeBuf := snap.ActiveBuffer
	activeFeat := snap.ActiveFeatureID

	pal := th.ToKitPalette()
	if customSB != nil && customSB.Body != nil {
		kit.EdgeRail(c, kit.EdgeRailConfig{
			Key:      "marlin-left-rail",
			Width:    customSB.Width,
			Pinned:   pinned,
			OnToggle: f.ToggleLeftRail,
			Palette:  pal,
			Surface:  kit.SurfaceOptions{Glass: mods.Glass, GlassOpacity: effOp, Motion: mods.Motion},
			CustomContent: func(ctx *ui.Context) {
				kit.SafeCol(ctx, "custom-sb-body", 8).Fill().Padding(4, 10).AlignItems(ui.Stretch).Children(func() {
					widgets.Render(ctx, customSB.Body, sig, pal, th.Sheet, wrt)
				})
			},
		})
		return
	}

	primary := []kit.RailItem{
		{
			ID:      "dashboard",
			Icon:    "dashboard",
			Label:   "Dashboard",
			Tone:    th.Info,
			Active:  activeBuf == buffer.Dashboard && activeFeat == "",
			OnClick: func() { f.SelectBuffer(buffer.Dashboard) },
		},
		{
			ID:      "conversation",
			Icon:    "chat",
			Label:   "Conversation",
			Tone:    th.Accent,
			Active:  activeBuf == buffer.Conversation && activeFeat == "",
			OnClick: func() { f.SelectBuffer(buffer.Conversation) },
		},
	}
	for _, fb := range featBufs {
		switch fb.ID {
		case "feature:chatroom":
			fid := fb.ID
			primary = append(primary, kit.RailItem{
				ID:      "agent-room",
				Icon:    "agents",
				Label:   "Agent Room",
				Tone:    th.Success,
				Active:  activeFeat == fid,
				OnClick: func() { f.SelectFeature(fid) },
			})
		case "feature:webisland":
			fid := fb.ID
			primary = append(primary, kit.RailItem{
				ID:      "workflow-canvas",
				Icon:    "cube",
				Label:   "Workflow Canvas (BPMN / Hybrid WebView)",
				Tone:    th.Accent,
				Active:  activeFeat == fid,
				OnClick: func() { f.SelectFeature(fid) },
			})
		}
	}
	primary = append(primary,
		kit.RailItem{
			ID:      "sessions",
			Icon:    "sessions",
			Label:   "Sessions",
			Tone:    th.Warning,
			Active:  activeBuf == buffer.Sessions && activeFeat == "",
			OnClick: func() { f.SelectBuffer(buffer.Sessions) },
		},
		kit.RailItem{
			ID:      "targets",
			Icon:    "host",
			Label:   "Targets",
			Tone:    th.Info,
			Active:  activeBuf == buffer.Targets && activeFeat == "",
			OnClick: func() { f.SelectBuffer(buffer.Targets) },
		},
		kit.RailItem{
			ID:      "settings",
			Icon:    "settings",
			Label:   "Settings",
			Tone:    th.Accent,
			Active:  activeBuf == buffer.Settings && activeFeat == "",
			OnClick: func() { f.SelectBuffer(buffer.Settings) },
		},
	)

	utility := []kit.RailItem{
		{
			ID:      "helm",
			Icon:    "search",
			Label:   "Helm Search (SPC s)",
			Tone:    th.Warning,
			OnClick: f.OpenHelm,
		},
		{
			ID:      "glass",
			Icon:    "glass",
			Label:   "Toggle Frosted Glass (SPC t v)",
			Tone:    th.Info,
			Active:  mods.Glass,
			OnClick: f.ToggleGlass,
		},
		{
			ID:      "zen",
			Icon:    "zen",
			Label:   "Toggle Zen Mode (SPC t z)",
			Tone:    th.Success,
			Active:  mods.Zen,
			OnClick: f.ToggleZenMode,
		},
	}

	kit.EdgeRail(c, kit.EdgeRailConfig{
		Key:      "marlin-left-rail",
		Primary:  primary,
		Utility:  utility,
		Pinned:   pinned,
		OnToggle: f.ToggleLeftRail,
		Palette:  pal,
		Surface:  kit.SurfaceOptions{Glass: mods.Glass, GlassOpacity: effOp, Motion: mods.Motion},
	})
}

func majorModeName(id buffer.ID) string {
	switch id {
	case buffer.Dashboard:
		return "Bento-Dashboard"
	case buffer.Sessions, buffer.Targets, buffer.Processes, buffer.Models:
		return "Tabulated-List"
	case buffer.Help:
		return "Help"
	case buffer.Messages:
		return "Messages"
	case buffer.Settings:
		return "Customize"
	case buffer.Packages:
		return "Package-Menu"
	default:
		return "Marlin-Chat"
	}
}

func (f *Frame) newSession() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnNewSession != nil {
		f.Adaptive.Desktop.OnNewSession()
		f.SelectBuffer(buffer.Conversation)
		f.Echo("Session: " + f.Adaptive.Desktop.ActiveSessionID)
	}
}

func (f *Frame) probeTargets() {
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.FederationUI != nil && f.Adaptive.Desktop.FederationUI.OnProbeAll != nil {
		f.Adaptive.Desktop.FederationUI.OnProbeAll()
		f.Echo("Probing federation targets...")
	}
}

func (f *Frame) submitPrompt(prompt string) {
	f.SelectBuffer(buffer.Conversation)
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSendPrompt != nil {
		f.Adaptive.Desktop.OnSendPrompt(prompt)
	}
}

func (f *Frame) saveCustomizePrefs(sessionID, remoteHost string, realSession bool) {
	f.mu.Lock()
	if strings.TrimSpace(sessionID) != "" {
		f.Prefs.LastSessionID = strings.TrimSpace(sessionID)
	}
	f.Prefs.DefaultRemoteHost = strings.TrimSpace(remoteHost)
	rs := realSession
	f.Prefs.RealSession = &rs
	prefs := f.Prefs
	f.mu.Unlock()
	if f.Adaptive != nil && f.Adaptive.Desktop != nil && f.Adaptive.Desktop.OnSelectSession != nil && prefs.LastSessionID != "" {
		f.Adaptive.Desktop.OnSelectSession(prefs.LastSessionID)
	}
	if f.onPrefsChanged != nil {
		f.onPrefsChanged(prefs)
	}
}
