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
dev-desktop core="../.." tags="marlin_full":
    @BIN="{{core}}/target/debug/marlin-mygo-host"; \
    if [ ! -x "$BIN" ]; then BIN="{{core}}/target/release/marlin-app-server"; fi; \
    if [ -z "${GOWORK:-}" ] && [ -f "../marlin-dev.work" ]; then export GOWORK="$(cd .. && pwd)/marlin-dev.work"; fi; \
    CGO_ENABLED=0 go run -tags "{{tags}}" ./cmd/marlin-desktop -app-server "$BIN"

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
