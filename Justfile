# SPDX-FileCopyrightText: 2026 tao3k team and Contributors
#
# SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

version := `git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev`
commit := `git rev-parse --short HEAD 2>/dev/null || echo local`
build_date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X github.com/tao3k/marlin-ui/internal/appconfig.Version=" + version + " -X github.com/tao3k/marlin-ui/internal/appconfig.Commit=" + commit + " -X github.com/tao3k/marlin-ui/internal/appconfig.BuildDate=" + build_date

default:
    @just --list

# Run pure-Go test suite and race detector.
test:
    CGO_ENABLED=0 go test ./internal/... ./cmd/marlin-desktop ./cmd/marlin-mobile
    go test -race ./internal/...

# Build native Desktop release binary (default: core + chatroom; override tags with marlin_minimal or marlin_full).
build-desktop tags="" out="./dist/marlin-desktop":
    mkdir -p $(dirname {{out}})
    CGO_ENABLED=0 go build -trimpath -ldflags="{{ldflags}}" -tags "{{tags}}" -o {{out}} ./cmd/marlin-desktop

# Package a self-contained macOS Marlin.app bundle (optionally co-locating a signed marlin-app-server sidecar via app_server_bin=...).
package-macos-app tags="marlin_full" out="./dist/Marlin.app" app_server_bin="":
    rm -rf "{{out}}"
    mkdir -p "{{out}}/Contents/MacOS" "{{out}}/Contents/Resources"
    CGO_ENABLED=0 go build -trimpath -ldflags="{{ldflags}}" -tags "{{tags}}" -o "{{out}}/Contents/MacOS/marlin-desktop" ./cmd/marlin-desktop
    sed "s/__VERSION__/{{version}}/g" packaging/macos/Info.plist.tmpl > "{{out}}/Contents/Info.plist"
    @if [ -n "{{app_server_bin}}" ]; then \
        cp "{{app_server_bin}}" "{{out}}/Contents/MacOS/marlin-app-server"; \
        chmod 0755 "{{out}}/Contents/MacOS/marlin-app-server"; \
        echo "Bundled sidecar marlin-app-server into {{out}}/Contents/MacOS/"; \
    fi
    @echo "Packaged macOS app bundle at {{out}}"

# Build Mobile Companion target binary (iOS HIG / Android Material 3).
build-mobile tags="" out="./dist/marlin-mobile":
    mkdir -p $(dirname {{out}})
    CGO_ENABLED=0 go build -trimpath -ldflags="{{ldflags}}" -tags "{{tags}}" -o {{out}} ./cmd/marlin-mobile

# Build WebAssembly static bundle (dist/web/marlin-web.wasm + index.html + wasm_exec.js).
build-web tags="" dir="./dist/web":
    mkdir -p {{dir}}
    GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags="{{ldflags}}" -tags "{{tags}}" -o {{dir}}/marlin-web.wasm ./cmd/marlin-web
    cp cmd/marlin-web/index.html {{dir}}/index.html
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" {{dir}}/wasm_exec.js 2>/dev/null || cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" {{dir}}/wasm_exec.js

# Build all 3 targets (Desktop + Mobile + Web WASM) with full features.
build-all tags="marlin_full":
    just build-desktop "{{tags}}"
    just build-mobile "{{tags}}"
    just build-web "{{tags}}"

# Verify Rust-Cargo-style compile-time feature pruning across all 3 release targets.
check-feature-pruning:
    @if go list -tags marlin_minimal -deps ./cmd/marlin-desktop | grep -q "marlin-mygo/features/"; then echo "FAIL: marlin_minimal linked features into marlin-desktop"; exit 1; fi
    @if go list -tags marlin_minimal -deps ./cmd/marlin-mobile | grep -q "marlin-mygo/features/"; then echo "FAIL: marlin_minimal linked features into marlin-mobile"; exit 1; fi
    @if GOOS=js GOARCH=wasm go list -tags marlin_minimal -deps ./cmd/marlin-web | grep -q "marlin-mygo/features/"; then echo "FAIL: marlin_minimal linked features into marlin-web"; exit 1; fi
    @go list -tags marlin_full -deps ./cmd/marlin-desktop | grep -q "marlin-mygo/features/chatroom"
    @go list -tags marlin_full -deps ./cmd/marlin-desktop | grep -q "marlin-mygo/features/compare"
    @go list -tags marlin_full -deps ./cmd/marlin-desktop | grep -q "marlin-mygo/features/canvas"
    @CGO_ENABLED=0 go build -tags marlin_minimal -o /dev/null ./cmd/marlin-desktop
    @CGO_ENABLED=0 go build -tags marlin_full -o /dev/null ./cmd/marlin-desktop
    @CGO_ENABLED=0 go build -tags marlin_minimal -o /dev/null ./cmd/marlin-mobile
    @CGO_ENABLED=0 go build -tags marlin_full -o /dev/null ./cmd/marlin-mobile
    @GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -tags marlin_minimal -o /dev/null ./cmd/marlin-web
    @GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -tags marlin_full -o /dev/null ./cmd/marlin-web
    @echo "OK: compile-time feature pruning verified across Desktop, Mobile, and Web WASM"

# Enforce Go modularity hygiene: zero .go files exceeding 1,000 lines.
check-file-size max_lines="1000":
    @OVER=$(find ./cmd ./internal -name '*.go' -exec wc -l {} + | awk -v max={{max_lines}} '$1 > max && $2 != "total" {print}'); if [ -n "$OVER" ]; then echo "Go files exceeding {{max_lines}} lines:"; echo "$OVER"; exit 1; else echo "OK: all Go files are <= {{max_lines}} lines"; fi

# --- Local DEV-ONLY helpers (never baked into Go release binaries) ---

# DEV: Launch Desktop client connected to a sibling marlin-agent-core Rust binary.
# Accepts any marlin-desktop flags directly (e.g. `just dev-desktop -config ./example.toml`).
# Override core root or build tags via CORE=... / TAGS=... environment variables.
dev-desktop *args:
    #!/usr/bin/env bash
    set -euo pipefail
    core="${CORE:-../..}"
    tags="${TAGS:-marlin_full}"
    for envf in "$core/.env" "./.env"; do
        if [ -f "$envf" ]; then
            set -a
            # shellcheck disable=SC1090
            source "$envf"
            set +a
        fi
    done
    bin=""
    for cand in \
        "$core/.bin/marlin-app-server" \
        "$core/target/debug/marlin-mygo-host" \
        "$core/target/release/marlin-app-server"; do
        if [ -x "$cand" ]; then
            bin="$cand"
            break
        fi
    done
    if [ -z "${GOWORK:-}" ] && [ -f "../marlin-dev.work" ]; then
        export GOWORK="$(cd .. && pwd)/marlin-dev.work"
    fi
    has_host_or_bin=0
    for arg in {{args}}; do
        case "$arg" in
            -app-server|-app-server=*|-host|-host=*|-standalone|-standalone=*) has_host_or_bin=1 ;;
        esac
    done
    extra=()
    if [ "$has_host_or_bin" -eq 0 ] && [ -n "$bin" ]; then
        extra+=(-app-server "$bin")
    fi
    exec env CGO_ENABLED=0 go run -tags "$tags" ./cmd/marlin-desktop ${extra[@]+"${extra[@]}"} {{args}}

# DEV: Launch full live Agent Room stack: marlin-app-server (LLM) <-> marlin-cove (gRPC HTTP/2) <-> marlin-desktop.
dev-room *args:
    #!/usr/bin/env bash
    set -euo pipefail
    core="${CORE:-../..}"
    cove_bin="${MARLIN_COVE_BIN:-../marlin-cove/target/debug/marlin-cove}"
    tags="${TAGS:-marlin_full}"
    for envf in "$core/.env" "./.env"; do
        if [ -f "$envf" ]; then
            set -a
            # shellcheck disable=SC1090
            source "$envf"
            set +a
        fi
    done
    app_bin=""
    for cand in \
        "$core/.bin/marlin-app-server" \
        "$core/target/debug/marlin-mygo-host" \
        "$core/target/release/marlin-app-server"; do
        if [ -x "$cand" ]; then
            app_bin="$cand"
            break
        fi
    done
    if [ -z "$app_bin" ]; then
        echo "error: marlin-app-server binary not found" >&2
        exit 1
    fi
    if [ ! -x "$cove_bin" ]; then
        echo "error: marlin-cove binary not found at $cove_bin (build .data/marlin-cove first)" >&2
        exit 1
    fi
    if [ -z "${GOWORK:-}" ] && [ -f "../marlin-dev.work" ]; then
        export GOWORK="$(cd .. && pwd)/marlin-dev.work"
    fi
    cfg_file="$(pwd)/example.toml"
    tmp_dir="$(mktemp -d)"
    app_log="$tmp_dir/app-server.log"
    cove_log="$tmp_dir/marlin-cove.log"
    cleanup() {
        kill "${COVE_PID:-}" "${APP_PID:-}" 2>/dev/null || true
        rm -rf "$tmp_dir"
    }
    trap cleanup EXIT INT TERM
    bridge_token="${MARLIN_BRIDGE_TOKEN:-marlin-cove-dev-$(date +%s)}"
    cove_token="${MARLIN_COVE_TOKEN:-marlin-cove-local-dev}"
    MARLIN_BRIDGE_REAL_SESSION=1 MARLIN_BRIDGE_TOKEN="$bridge_token" MARLIN_CONFIG="$cfg_file" MARLIN_BRIDGE_CONFIG="$cfg_file" "$app_bin" >"$app_log" 2>&1 &
    APP_PID=$!
    app_addr=""
    for _ in $(seq 1 60); do
        if grep -q "^READY " "$app_log" 2>/dev/null; then
            app_addr="$(awk '/^READY / {print $2; exit}' "$app_log")"
            break
        fi
        sleep 0.1
    done
    if [ -z "$app_addr" ]; then
        echo "error: marlin-app-server failed to start:" >&2
        cat "$app_log" >&2
        exit 1
    fi
    cove_bind="${MARLIN_COVE_RELAY_GRPC_BIND:-127.0.0.1:18090}"
    cove_host="${cove_bind%:*}"
    cove_port="${cove_bind##*:}"
    if (exec 3<>"/dev/tcp/$cove_host/$cove_port") 2>/dev/null; then
        exec 3>&- 3<&-
        echo "[dev-room] reusing live marlin-cove at $cove_bind (shared room across Desktop & Mobile)"
    else
        MARLIN_COVE_RELAY_GRPC_BIND="$cove_bind" \
        MARLIN_COVE_RELAY_AUTH_TOKEN="$cove_token" \
        MARLIN_COVE_MARLIN_GATEWAY_ENDPOINT="http://$app_addr" \
        MARLIN_COVE_MARLIN_GATEWAY_TOKEN="$bridge_token" \
            "$cove_bin" serve >"$cove_log" 2>&1 &
        COVE_PID=$!
        cove_ready=0
        for _ in $(seq 1 50); do
            if ! kill -0 "$COVE_PID" 2>/dev/null; then
                echo "error: marlin-cove exited early:" >&2
                cat "$cove_log" >&2
                exit 1
            fi
            if (exec 3<>"/dev/tcp/$cove_host/$cove_port") 2>/dev/null; then
                exec 3>&- 3<&-
                cove_ready=1
                break
            fi
            sleep 0.1
        done
        if [ "$cove_ready" -ne 1 ]; then
            echo "warning: marlin-cove did not open $cove_bind within 5s (logs follow); UI will keep local fallback live:" >&2
            cat "$cove_log" >&2 || true
        fi
    fi
    echo "[dev-room] marlin-app-server=$app_addr  marlin-cove=$cove_bind"
    exec env CGO_ENABLED=0 go run -tags "$tags" ./cmd/marlin-desktop \
        -config "$cfg_file" -host "$app_addr" -token "$bridge_token" \
        -cove "$cove_bind" -cove-token "$cove_token" {{args}}

# DEV: Launch full live Agent Room stack on iOS Companion shell (393x852).
dev-room-ios *args:
    #!/usr/bin/env bash
    set -euo pipefail
    core="${CORE:-../..}"
    cove_bin="${MARLIN_COVE_BIN:-../marlin-cove/target/debug/marlin-cove}"
    tags="${TAGS:-marlin_full}"
    for envf in "$core/.env" "./.env"; do
        if [ -f "$envf" ]; then
            set -a
            # shellcheck disable=SC1090
            source "$envf"
            set +a
        fi
    done
    app_bin=""
    for cand in \
        "$core/.bin/marlin-app-server" \
        "$core/target/debug/marlin-mygo-host" \
        "$core/target/release/marlin-app-server"; do
        if [ -x "$cand" ]; then
            app_bin="$cand"
            break
        fi
    done
    if [ -z "$app_bin" ] || [ ! -x "$cove_bin" ]; then
        echo "error: require marlin-app-server and marlin-cove binaries" >&2
        exit 1
    fi
    if [ -z "${GOWORK:-}" ] && [ -f "../marlin-dev.work" ]; then
        export GOWORK="$(cd .. && pwd)/marlin-dev.work"
    fi
    cfg_file="$(pwd)/example.toml"
    tmp_dir="$(mktemp -d)"
    app_log="$tmp_dir/app-server.log"
    cove_log="$tmp_dir/marlin-cove.log"
    cleanup() {
        kill "${COVE_PID:-}" "${APP_PID:-}" 2>/dev/null || true
        rm -rf "$tmp_dir"
    }
    trap cleanup EXIT INT TERM
    bridge_token="${MARLIN_BRIDGE_TOKEN:-marlin-cove-dev-$(date +%s)}"
    cove_token="${MARLIN_COVE_TOKEN:-cove-room-dev-$(date +%s)}"
    MARLIN_BRIDGE_REAL_SESSION=1 MARLIN_BRIDGE_TOKEN="$bridge_token" MARLIN_CONFIG="$cfg_file" MARLIN_BRIDGE_CONFIG="$cfg_file" "$app_bin" >"$app_log" 2>&1 &
    APP_PID=$!
    app_addr=""
    for _ in $(seq 1 60); do
        if grep -q "^READY " "$app_log" 2>/dev/null; then
            app_addr="$(awk '/^READY / {print $2; exit}' "$app_log")"
            break
        fi
        sleep 0.1
    done
    if [ -z "$app_addr" ]; then
        cat "$app_log" >&2
        exit 1
    fi
    cove_bind="${MARLIN_COVE_RELAY_GRPC_BIND:-127.0.0.1:18090}"
    MARLIN_COVE_RELAY_GRPC_BIND="$cove_bind" \
    MARLIN_COVE_RELAY_AUTH_TOKEN="$cove_token" \
    MARLIN_COVE_MARLIN_GATEWAY_ENDPOINT="http://$app_addr" \
    MARLIN_COVE_MARLIN_GATEWAY_TOKEN="$bridge_token" \
        "$cove_bin" serve >"$cove_log" 2>&1 &
    COVE_PID=$!
    cove_host="${cove_bind%:*}"
    cove_port="${cove_bind##*:}"
    for _ in $(seq 1 50); do
        if ! kill -0 "$COVE_PID" 2>/dev/null; then
            echo "error: marlin-cove exited early:" >&2
            cat "$cove_log" >&2
            exit 1
        fi
        if (exec 3<>"/dev/tcp/$cove_host/$cove_port") 2>/dev/null; then
            exec 3>&- 3<&-
            break
        fi
        sleep 0.1
    done
    echo "[dev-room-ios] marlin-app-server=$app_addr  marlin-cove=$cove_bind"
    exec env CGO_ENABLED=0 go run -tags "$tags" ./cmd/marlin-mobile -platform ios \
        -config "$cfg_file" -host "$app_addr" -token "$bridge_token" \
        -cove "$cove_bind" -cove-token "$cove_token" {{args}}

# DEV: Launch iOS Companion client in Apple HIG (393x852) dev shell connected to sibling Rust host.
# Accepts any marlin-mobile flags directly (e.g. `just dev-ios -config ./example.toml -device iphone16`).
dev-ios *args:
    #!/usr/bin/env bash
    set -euo pipefail
    core="${CORE:-../..}"
    tags="${TAGS:-marlin_full}"
    for envf in "$core/.env" "./.env"; do
        if [ -f "$envf" ]; then
            set -a
            # shellcheck disable=SC1090
            source "$envf"
            set +a
        fi
    done
    bin=""
    for cand in \
        "$core/.bin/marlin-app-server" \
        "$core/target/debug/marlin-mygo-host" \
        "$core/target/release/marlin-app-server"; do
        if [ -x "$cand" ]; then
            bin="$cand"
            break
        fi
    done
    if [ -z "${GOWORK:-}" ] && [ -f "../marlin-dev.work" ]; then
        export GOWORK="$(cd .. && pwd)/marlin-dev.work"
    fi
    has_host_or_bin=0
    for arg in {{args}}; do
        case "$arg" in
            -app-server|-app-server=*|-host|-host=*|-standalone|-standalone=*) has_host_or_bin=1 ;;
        esac
    done
    extra=(-platform ios)
    if [ "$has_host_or_bin" -eq 0 ] && [ -n "$bin" ]; then
        extra+=(-app-server "$bin")
    fi
    exec env CGO_ENABLED=0 go run -tags "$tags" ./cmd/marlin-mobile ${extra[@]+"${extra[@]}"} {{args}}

# Build native iOS .app bundle via MyGo Swift/Metal host (`ios/simulator` or `ios` for physical device).
build-ios platform="ios/simulator" out="./dist/ios" tags="marlin_full":
    #!/usr/bin/env bash
    set -euo pipefail
    unset NIX_CFLAGS_COMPILE NIX_LDFLAGS SDKROOT
    sdk_name="iphonesimulator"
    target="arm64-apple-ios17.0-simulator"
    plat_name="iPhoneSimulator"
    if [ "{{platform}}" = "ios" ] || [ "{{platform}}" = "iphoneos" ]; then
        sdk_name="iphoneos"
        target="arm64-apple-ios17.0"
        plat_name="iPhoneOS"
    fi
    sdk_path="$(xcrun --sdk "$sdk_name" --show-sdk-path)"
    clang_bin="$(xcrun --sdk "$sdk_name" --find clang)"
    mygo_dir="$(go list -m -f '{{{{.Dir}}' github.com/egoist/mygo)"
    host_dir="$mygo_dir/internal/mobile/ios"
    work_dir=".mygo/ios-$sdk_name"
    bundle="{{out}}/Marlin.app"
    mkdir -p "$work_dir" "{{out}}"
    rm -rf "$bundle"
    mkdir -p "$bundle"
    echo "[ios] compiling Go c-archive ($target, tags={{tags}})..."
    GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
        CC="$clang_bin -target $target -isysroot $sdk_path" \
        go build -trimpath -tags "{{tags}}" -ldflags="{{ldflags}}" \
        -buildmode=c-archive -o "$work_dir/libapp.a" ./cmd/marlin-mobile
    echo "[ios] compiling MyGo Swift UIKit/Metal host..."
    xcrun --sdk "$sdk_name" swiftc -parse-as-library -module-name MyGoHost \
        -target "$target" -sdk "$sdk_path" \
        -import-objc-header "$host_dir/MyGo.h" \
        -wmo -c -o "$work_dir/host.o" \
        "$host_dir"/Sources/MyGoHost/*.swift
    echo "[ios] linking native $bundle/Marlin..."
    xcrun --sdk "$sdk_name" swiftc -target "$target" -sdk "$sdk_path" \
        "$work_dir/host.o" "$work_dir/libapp.a" \
        -o "$bundle/Marlin" \
        -Xlinker -dead_strip -lresolv \
        -framework UIKit -framework WebKit -framework Metal -framework QuartzCore \
        -framework CoreText -framework CoreGraphics -framework Security -framework CoreFoundation \
        -framework UserNotifications -framework UniformTypeIdentifiers -framework Network -framework AVFoundation
    sed -e "s/__VERSION__/{{version}}/g" -e "s/__PLATFORM__/$plat_name/g" \
        packaging/ios/Info.plist.tmpl > "$bundle/Info.plist"
    if [ -f "resources/icon.png" ]; then
        cp "resources/icon.png" "$bundle/AppIcon.png"
    fi
    codesign --force --sign - "$bundle"
    echo "[ios] OK: built native $plat_name bundle at $bundle"

# DEV: Build and launch native Marlin.app inside Xcode iOS Simulator (`just dev-ios-sim [udid_or_booted]`).
dev-ios-sim device="":
    #!/usr/bin/env bash
    set -euo pipefail
    just build-ios ios/simulator ./dist/ios
    dev="{{device}}"
    if [ -z "$dev" ]; then
        dev="$(xcrun simctl list devices booted -j | python3 -c 'import json,sys; d=json.load(sys.stdin).get("devices",{}); print(next((x["udid"] for arr in d.values() for x in arr if x.get("state")=="Booted"), ""))' 2>/dev/null || true)"
    fi
    if [ -z "$dev" ]; then
        dev="$(xcrun simctl list devices available -j | python3 -c 'import json,sys; d=json.load(sys.stdin).get("devices",{}); print(next((x["udid"] for arr in d.values() for x in arr if x.get("isAvailable") and "iPhone" in x.get("name","")), ""))' 2>/dev/null || true)"
        if [ -z "$dev" ]; then
            echo "No iOS Simulator runtime installed yet. Install one with:"
            echo "  xcodebuild -downloadPlatform iOS"
            echo "Meanwhile, your native iOS Simulator bundle is ready at: dist/ios/Marlin.app"
            echo "Or launch the instant macOS Apple HIG iOS dev shell right now with:"
            echo "  just dev-ios -config ./example.toml"
            exit 1
        fi
        xcrun simctl boot "$dev"
    fi
    open -a Simulator
    xcrun simctl install "$dev" ./dist/ios/Marlin.app
    exec xcrun simctl launch --console "$dev" org.tao3k.marlin

# DEV: Run live cross-process hostlink E2E test against a sibling marlin-agent-core Rust binary.
dev-test-rust core="../..":
    @BIN="$(cd {{core}} && pwd)/target/debug/marlin-mygo-host"; \
    if [ ! -x "$BIN" ]; then BIN="$(cd {{core}} && pwd)/target/release/marlin-app-server"; fi; \
    MARLIN_APP_SERVER_BIN="$BIN" go test -v -race -run "TestHostLinkLiveRustAppServerGRPCConnection" ./internal/hostlink

# Validate project.sysml and dev/sysml OpenSysML v2 architecture models.
sysml-check:
    sysml -strict -validate -self-check -no-record-cache project.sysml dev/sysml

# Enforce hard Go pure-frontend constraint (MUI-ARCH-R04) and Emacs package hierarchy (MUI-EMACS-R01).
check-go-boundary:
    CGO_ENABLED=0 go test -v ./internal/archguard

# Full validation gate.
check: test check-go-boundary check-feature-pruning check-file-size
    @if command -v sysml >/dev/null 2>&1; then just sysml-check; else echo "SKIP: sysml CLI not installed"; fi
    CGO_ENABLED=0 go vet ./internal/... ./cmd/marlin-desktop ./cmd/marlin-mobile
    GOOS=js GOARCH=wasm CGO_ENABLED=0 go vet ./cmd/marlin-web
