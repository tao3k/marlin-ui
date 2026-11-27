// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme

import (
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"
)

// Semantic Apple SF Symbol tokens shared across Desktop (macOS 26), iOS 26,
// and Web WASM so every action control uses consistent optical iconography
// instead of raw ASCII characters.
const (
	IconSend       = "arrow.up"
	IconStop       = "stop"
	IconSparkles   = "sparkles"
	IconCite       = "at"
	IconAttach     = "paperclip"
	IconSearch     = "magnifyingglass"
	IconApprove    = "checkmark"
	IconReject     = "xmark"
	IconClose      = "xmark"
	IconCopy       = "doc.on.doc"
	IconRetry      = "arrow.counterclockwise"
	IconFork       = "arrow.triangle.branch"
	IconReload     = "arrow.clockwise"
	IconModels     = "cpu"
	IconSave       = "checkmark.circle"
	IconShield     = "checkmark.shield"
	IconWorkspace  = "square.stack.3d.up"
	IconThemeDark  = "moon.stars"
	IconThemeLight = "sun.max"
	IconExternal   = "arrow.up.right"
	IconCode       = "chevron.left.forwardslash.chevron.right"
	IconWarning    = "exclamationmark.triangle"
	IconTool       = "wrench.and.screwdriver"
	IconNetwork    = "network"
	IconKey        = "key"
	IconPlus       = "plus"
	IconSwap       = "arrow.left.arrow.right"
	IconLock       = "lock"
	IconUnlock     = "lock.open"
	IconSignOut    = "rectangle.portrait.and.arrow.right"
)

// SFIcon renders an Apple CoreGraphics SF Symbol with SwiftUI optical sizing
// (.Natural) and optional filled weight across Desktop, iOS, and Web.
func SFIcon(c *ui.Context, name string, size float32, col ui.Color, filled bool) ui.Element {
	return SFIconWeighted(c, name, size, col, sfsymbols.Medium, filled, true)
}

// SFIconWeighted renders an Apple SF Symbol with explicit stroke weight and
// optional natural text-alignment sizing. Standalone glyphs inside circular
// buttons should pass natural=false with Semibold/Bold weight so the glyph
// fills its optical square dead-center like Apple Messages and iOS 26 toolbars.
func SFIconWeighted(c *ui.Context, name string, size float32, col ui.Color, weight sfsymbols.Weight, filled bool, natural bool) ui.Element {
	el := sfsymbols.Symbol(c, name).Size(size).Color(col).Weight(weight)
	if natural {
		el = el.Natural()
	}
	if filled {
		el = el.Fill()
	}
	return el.Element
}
