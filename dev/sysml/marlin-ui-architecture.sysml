// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Top-level system boundary and target release architecture for marlin-ui.
// Follows the deterministic-engineering architecture content contract:
// actual describes source-inspected implementation; target describes planned
// extensions; dependencies point from consumer to supplier.
package MarlinUIArchitecture {
    doc /* System boundary separating Marlin UI product targets, headless engine
           library (marlin-mygo), gRPC transport (hostlink), compile-time feature
           pruning (featurekit), and typed user configuration (appconfig). */

    package actual {
        part <'MUI-ARCH-A01'> releaseTargets {
            doc /* Three pure-Go (CGO_ENABLED=0) release executables:
                   cmd/marlin-desktop (Doom Emacs desktop workbench),
                   cmd/marlin-mobile (iOS/Android companion), and
                   cmd/marlin-web (GOOS=js GOARCH=wasm static bundle). */
            part desktopApp {
                doc /* cmd/marlin-desktop: owns native window, OS menu bar,
                       Retina tray icon, file drop, voice capture, and binds
                       internal/emacs/frame.Frame.View directly to mygo/ui. */
            }
            part mobileApp {
                doc /* cmd/marlin-mobile: phone companion built over marlin-mygo
                       mobile adaptive controllers. */
            }
            part webApp {
                doc /* cmd/marlin-web: static WebAssembly bundle with browser
                       localStorage preferences and clipboard continuity. */
            }
        }

        part <'MUI-ARCH-A02'> hostlinkTransport {
            doc /* internal/hostlink: owns supervised local marlin-app-server
                   sidecar lifecycle, remote TLS 1.3 / mTLS gRPC federation,
                   OS Keychain Vault OAuth 2.1 PKCE / OIDC auth wiring, Helm
                   multi-source dispatch, and atomic crash recovery. */
        }

        part <'MUI-ARCH-A03'> marlinMyGoEngine {
            doc /* External supplier github.com/tao3k/marlin-mygo: provides
                   headless TranscriptController, 120Hz HelmSearch engine,
                   PromptGate queue, ProjectionController, AuthManager,
                   WorkbenchController (marlin.ui.v1), declarative Scheme
                   UIElement mapper (runtime/mapper), governed pure-Go CSS-subset
                   stylesheet engine (runtime/style), framework-level GPU design
                   kit (runtime/kit: overflow-safe primitives, iOS 26 Liquid Glass,
                   SF Symbols, Kage GPU shaders, ui/chart), declarative Bento
                   widget framework (runtime/widgets), and gRPC clients. */
        }

        part <'MUI-ARCH-A04'> appConfigBoundary {
            doc /* internal/appconfig: typed CLI config, build version ldflags,
                   atomic 0600 UserPrefs persistence, and governed ~/.config/marlin/ui/
                   stylesheet bundle loader (manifest.toml, themes/<id>/, theme.css).
                   Zero dev token fallbacks and zero hardcoded sibling repository paths. */
        }

        part <'MUI-ARCH-A05'> featureKitBoundary {
            doc /* internal/featurekit: compile-time build-tag feature pruning
                   (marlin_minimal, default, marlin_full) matching Rust Cargo
                   feature boundaries across all three release targets. */
        }

        part <'MUI-ARCH-A06'> goBoundaryGuard {
            doc /* internal/archguard/boundary_test.go + Justfile check-go-boundary:
                   mechanical Go AST and import-graph gate enforcing that Go
                   never implements Runtime backend logic (forbidding os/exec,
                   database/sql, net/rpc, grpc.NewServer, http.ListenAndServe,
                   net.Listen in production code; forbidding os/net/grpc I/O
                   inside internal/emacs/*; and enforcing the strict acyclic
                   dependency DAG of internal/emacs/*). */
        }

        dependency desktopUsesHostlink from releaseTargets::desktopApp to hostlinkTransport;
        dependency desktopUsesConfig from releaseTargets::desktopApp to appConfigBoundary;
        dependency desktopUsesFeatures from releaseTargets::desktopApp to featureKitBoundary;
        dependency mobileUsesHostlink from releaseTargets::mobileApp to hostlinkTransport;
        dependency webUsesFeatures from releaseTargets::webApp to featureKitBoundary;
        dependency hostlinkUsesEngine from hostlinkTransport to marlinMyGoEngine;
        dependency featuresUseEngine from featureKitBoundary to marlinMyGoEngine;
        dependency guardChecksTargets from goBoundaryGuard to releaseTargets;
        dependency guardChecksHostlink from goBoundaryGuard to hostlinkTransport;
    }

    package target {
        part <'MUI-ARCH-T01'> unifiedEmacsWebTarget {
            doc /* Target: share internal/emacs/* pure-Go views with WebAssembly
                   builds by removing !js build constraints from platform-neutral
                   Emacs subpackages while keeping OS Keychain/sidecar native-only. */
        }

        part <'MUI-ARCH-T02'> signedReleaseDistribution {
            doc /* Target: reproducible signed macOS Marlin.app DMG, Linux
                   AppImage/deb, and Windows MSI bundles with bundled or
                   discovered marlin-app-server sidecar verification. */
        }
    }

    package requirements {
        requirement <'MUI-ARCH-R01'> headlessEngineSeparation {
            doc /* marlin-mygo remains a headless engine and protocol library;
                   marlin-ui owns its desktop visual design, Doom Emacs
                   hierarchy, typography, and layout directly over mygo/ui. */
        }

        requirement <'MUI-ARCH-R02'> pureGoReleasePortability {
            doc /* All three release targets compile with CGO_ENABLED=0, zero
                   local go.mod replace directives, zero sibling workspace paths,
                   and verified compile-time feature pruning. */
        }

        requirement <'MUI-ARCH-R03'> failClosedAuthAndTransport {
            doc /* No hardcoded dev bearer tokens, no placeholder OAuth client
                   IDs without explicit operator configuration, and encrypted OS
                   Keychain Vault storage for credentials. */
        }

        requirement <'MUI-ARCH-R04'> goPureFrontendNoRuntimeBackend {
            doc /* Hard constraint: Rust (marlin-agent-core) owns 100% of the
                   agent runtime backend (LLM dispatch, tool execution, database
                   storage, gRPC/HTTP servers, process orchestration). Go in
                   marlin-ui is strictly a pure native UI shell and thin gRPC
                   projection client: zero backend servers, zero SQL/database
                   drivers, zero os/exec process execution in production Go, and
                   zero OS/network/gRPC I/O inside internal/emacs/*. */
        }
    }

    allocation ownEngineSeparation allocate requirements::headlessEngineSeparation to actual::releaseTargets.desktopApp;
    allocation ownPortability allocate requirements::pureGoReleasePortability to actual::featureKitBoundary;
    allocation ownAuthSafety allocate requirements::failClosedAuthAndTransport to actual::hostlinkTransport;
    allocation ownGoFrontendBoundary allocate requirements::goPureFrontendNoRuntimeBackend to actual::goBoundaryGuard;
}
