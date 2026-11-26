// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build marlin_minimal || js

package featurekit

import (
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/async"
	"github.com/tao3k/marlin-mygo/runtime/kit"
)

// WebViewInstance is a stub when WebView islands are pruned (marlin_minimal / Web WASM).
type WebViewInstance struct {
	View       ui.NativeWebView
	Eval       func(js string)
	ReloadHTML func(html string)
	Destroy    func()
	Tier       async.TrustTier
}

// WebViewFactory is a no-op stub when WebView islands are pruned (marlin_minimal / Web WASM).
type WebViewFactory func(islandID, html, preloadJS string, tier async.TrustTier) (WebViewInstance, error)

// BuiltinWorkflowStudioIslandID is exported for build-tag parity when WebView islands are pruned.
const BuiltinWorkflowStudioIslandID = "workflow-graph"

// BuiltinArtifactStageIslandID is exported for build-tag parity when WebView islands are pruned.
const BuiltinArtifactStageIslandID = "artifact-stage"

func SetWebViewFactory(WebViewFactory)            {}
func SetWebViewInvalidate(func())                 {}
func RegisterUserIslands(map[string]string)       {}
func PrewarmWebIsland(string)                     {}
func SyncWebIslandTheme(kit.Palette, float32)     {}
func DispatchWebIslandAction(string, string) bool { return false }
func PushArtifactStagePayload(string, string, string) {}
func NotifyWebIslandVisibility(string)            {}
func CheckWorkflowBridgeRPC() error               { return async.ErrUntrustedRPCForbidden }
func RecoverWebIsland(string)                     {}
func ShutdownWebIslands()                         {}
func WebIslandHost() kit.WebIslandHost            { return nil }
