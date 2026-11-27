// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package uiframework

import (
	"strings"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-mygo/runtime/mapper"

	"github.com/tao3k/marlin-ui/internal/shellmanifest"
)

// SchemeExtBufferPrefix namespaces Scheme-authored UIExtensionRegion buffers
// so they behave as first-class Emacs buffers inside multi-window splits,
// centaur tabs, SPC b b buffer switching, and per-window Doom modelines.
const SchemeExtBufferPrefix = "scheme-ext:"

// ProjectedWindow is a Scheme `marlin.ui.v1.UiWindow` resolved to a concrete
// native or Scheme-extension buffer ID ready for workspace window-tree sync.
type ProjectedWindow struct {
	ID          string
	BufferID    string
	IsExtension bool
	ExtensionID string
}

// MappedCommand is a Scheme `marlin.ui.v1.UiCommand` resolved onto either a
// native Doom action (`cmd:*` / `buffer:*`) or a live Scheme RPC (`scheme:*`).
type MappedCommand struct {
	Name        string
	Description string
	Action      string
}

// WorkbenchProjection is the validated, surface-adapted result of projecting
// one `marlin.ui.v1.UIWorkbenchSnapshot` for Desktop, iOS, or Android.
type WorkbenchProjection struct {
	Surface            AssembledSurface
	Hydras             map[string]MappedHydra
	Windows            []ProjectedWindow
	SelectedWindowID   string
	StatusItems        []mapper.ResolvedStatusItem
	Extensions         []schema.UIExtensionRegion
	RejectedExtensions int
	Commands           []MappedCommand
}

// DefaultStatusWidthCols returns the responsive status-item column budget for target.
func DefaultStatusWidthCols(target shellmanifest.TargetKind) uint32 {
	if target == shellmanifest.TargetMobileIOS || target == shellmanifest.TargetMobileAndroid {
		return 42
	}
	return 120
}

// ProjectWorkbench validates and projects a live `marlin.ui.v1` workbench
// snapshot for any Marlin surface (Desktop, iOS, or Android), enforcing:
//   - Modular Component Registry assembly + fail-closed slot trust policies
//   - MAR-UI-R03 bounded UIElement tree validation on all Scheme extensions
//   - Width-aware Scheme status item projection (matching Rust TUI modeline)
//   - Canonical Doom + live custom Scheme keymap & transient projection
//   - Scheme UIFrame window topology resolution (native + extension buffers)
func ProjectWorkbench(
	reg *Registry,
	target shellmanifest.TargetKind,
	snap schema.UIWorkbenchSnapshot,
) (WorkbenchProjection, error) {
	if reg == nil {
		reg = DefaultRegistry()
	}
	surf, err := reg.AssembleSurface(target, snap.Slots)
	if err != nil {
		return WorkbenchProjection{}, err
	}

	validExts := make([]schema.UIExtensionRegion, 0, len(snap.Extensions))
	rejected := 0
	for _, ext := range snap.Extensions {
		if strings.TrimSpace(ext.ID) == "" {
			rejected++
			continue
		}
		if err := ext.Content.ValidateBounds(0, 0); err != nil {
			rejected++
			continue
		}
		validExts = append(validExts, ext)
	}

	var hydras map[string]MappedHydra
	if len(snap.Keymap) > 0 || len(snap.Transients) > 0 {
		hydras = ProjectKeymapAndTransients(snap.Keymap, snap.Transients)
	}

	cmds := make([]MappedCommand, 0, len(snap.Commands))
	for _, sc := range snap.Commands {
		name := strings.TrimSpace(sc.Name)
		if name == "" {
			continue
		}
		desc := strings.TrimSpace(sc.Description)
		if desc == "" {
			desc = "Scheme marlin.ui.v1 command"
		}
		act, _, _ := ResolveSchemeAction(name)
		cmds = append(cmds, MappedCommand{
			Name:        name,
			Description: desc,
			Action:      act,
		})
	}

	wins := ProjectFrameWindows(snap.Frame, validExts)
	statusItems := mapper.ProjectStatusItems(snap.StatusItems, DefaultStatusWidthCols(target), nil)

	return WorkbenchProjection{
		Surface:            surf,
		Hydras:             hydras,
		Windows:            wins,
		SelectedWindowID:   strings.TrimSpace(snap.Frame.SelectedWindow),
		StatusItems:        statusItems,
		Extensions:         validExts,
		RejectedExtensions: rejected,
		Commands:           cmds,
	}, nil
}

// ProjectFrameWindows resolves Scheme `UIFrame.Windows` into concrete native
// or `scheme-ext:<id>` window buffer bindings in reading order.
func ProjectFrameWindows(frame schema.UIFrame, exts []schema.UIExtensionRegion) []ProjectedWindow {
	if len(frame.Windows) == 0 {
		return nil
	}
	out := make([]ProjectedWindow, 0, len(frame.Windows))
	for _, w := range frame.Windows {
		wid := strings.TrimSpace(w.ID)
		bufID, isExt, extID := ResolveSchemeBufferID(w.BufferID, exts)
		if bufID == "" {
			continue
		}
		out = append(out, ProjectedWindow{
			ID:          wid,
			BufferID:    bufID,
			IsExtension: isExt,
			ExtensionID: extID,
		})
	}
	return out
}

// ResolveSchemeBufferID maps a Scheme buffer name or extension ID onto Marlin
// UI's canonical buffer ID namespace (`*conversation*`, `*Dashboard*`, or
// `scheme-ext:<id>` for live Scheme extension buffers).
func ResolveSchemeBufferID(raw string, exts []schema.UIExtensionRegion) (bufID string, isExt bool, extID string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, ""
	}
	if strings.HasPrefix(trimmed, SchemeExtBufferPrefix) {
		eid := strings.TrimSpace(strings.TrimPrefix(trimmed, SchemeExtBufferPrefix))
		if eid != "" {
			return SchemeExtBufferPrefix + eid, true, eid
		}
	}

	// Match live Scheme UIExtensionRegion by ID or display Label (case-insensitive).
	norm := strings.ToLower(strings.Trim(trimmed, "* "))
	for _, ext := range exts {
		if strings.EqualFold(ext.ID, trimmed) || strings.EqualFold(ext.ID, norm) {
			return SchemeExtBufferPrefix + ext.ID, true, ext.ID
		}
		extLblNorm := strings.ToLower(strings.Trim(ext.Label, "* "))
		if extLblNorm != "" && extLblNorm == norm {
			return SchemeExtBufferPrefix + ext.ID, true, ext.ID
		}
	}

	switch norm {
	case "dashboard", "home":
		return "*Dashboard*", false, ""
	case "conversation", "chat", "agent":
		return "*conversation*", false, ""
	case "sessions":
		return "*Sessions*", false, ""
	case "targets":
		return "*Targets*", false, ""
	case "processes":
		return "*Processes*", false, ""
	case "models":
		return "*Models*", false, ""
	case "messages":
		return "*Messages*", false, ""
	case "help":
		return "*Help*", false, ""
	case "settings", "customize":
		return "*Settings*", false, ""
	case "packages", "marketplace":
		return "*Packages*", false, ""
	case "preview":
		return "*Preview*", false, ""
	case "agent room", "agent-room", "room", "chatroom":
		return "*Agent Room*", false, ""
	case "workflow canvas", "workflow-canvas", "workflow":
		return "*Workflow Canvas*", false, ""
	default:
		return trimmed, false, ""
	}
}
