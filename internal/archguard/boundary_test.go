// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Package archguard mechanically enforces MUI-ARCH-R04 (Go Pure-Frontend
// Constraint: Rust marlin-agent-core owns 100% of Runtime backend logic; Go in
// marlin-ui is strictly a native UI shell and thin gRPC projection client) and
// MUI-EMACS-R01 (strict acyclic internal/emacs/* package hierarchy).
package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/tao3k/marlin-ui/"

// forbiddenAnywhereImports are backend/runtime packages that production Go code
// in marlin-ui must NEVER import (Rust marlin-agent-core owns the entire backend).
var forbiddenAnywhereImports = map[string]string{
	"database/sql":         "Rust owns all durable storage (Turso/SQLite/Org); Go UI must not access SQL databases",
	"database/sql/driver":  "Rust owns all durable storage; Go UI must not register SQL drivers",
	"os/exec":              "Rust owns process execution; Go UI must not spawn arbitrary processes directly",
	"net/rpc":              "Rust owns backend RPC services; Go UI must not expose RPC servers",
	"net/rpc/jsonrpc":      "Rust owns backend RPC services",
	"net/smtp":             "Go UI must not implement backend mail/network services",
	"plugin":               "Go runtime plugins are forbidden; Rust/Scheme owns runtime extensions",
	"unsafe":               "CGO_ENABLED=0 pure-Go UI forbids unsafe pointer manipulation",
}

// forbiddenEmacsUIImports ensure internal/emacs/* stays 100% pure UI presentation
// and interaction state: zero filesystem I/O, zero network, zero gRPC, zero syscall.
var forbiddenEmacsUIImports = map[string]string{
	"os":                           "internal/emacs/* is pure UI: no direct OS/filesystem I/O",
	"io/fs":                        "internal/emacs/* is pure UI: no direct filesystem I/O",
	"path/filepath":                "internal/emacs/* is pure UI: no filesystem path manipulation",
	"net":                          "internal/emacs/* is pure UI: no network sockets",
	"net/http":                     "internal/emacs/* is pure UI: no HTTP clients or servers",
	"syscall":                      "internal/emacs/* is pure UI: no syscalls",
	"google.golang.org/grpc":       "internal/emacs/* consumes projected state, never raw gRPC",
	"google.golang.org/grpc/codes": "internal/emacs/* consumes projected state, never raw gRPC",
}

// forbiddenServerCalls forbid starting backend listeners or RPC servers in Go production code.
var forbiddenServerCalls = map[string]string{
	"http.ListenAndServe":    "Go UI must never run an HTTP backend server",
	"http.ListenAndServeTLS": "Go UI must never run an HTTP backend server",
	"http.Serve":             "Go UI must never run an HTTP backend server",
	"net.Listen":             "Go UI must never open listening backend sockets",
	"net.ListenPacket":       "Go UI must never open listening backend sockets",
	"grpc.NewServer":         "Rust owns gRPC servers; Go UI is strictly a client",
	"exec.Command":           "Rust owns process execution; Go UI must not spawn commands",
	"exec.CommandContext":    "Rust owns process execution; Go UI must not spawn commands",
}

// allowedEmacsInternalDeps enforces the strict acyclic internal/emacs/* hierarchy
// declared in dev/sysml/emacs-workbench.sysml (MUI-EMACS-R01).
var allowedEmacsInternalDeps = map[string]map[string]bool{
	"internal/emacs/theme": {},
	"internal/emacs/meow":  {},
	"internal/emacs/hydra": {
		"internal/emacs/theme": true,
	},
	"internal/emacs/tabs": {
		"internal/emacs/theme":  true,
		"internal/emacs/buffer": true,
	},
	"internal/emacs/minibuffer": {
		"internal/emacs/theme": true,
		"internal/emacs/meow":  true,
		"internal/emacs/hydra": true,
	},
	"internal/emacs/modeline": {
		"internal/emacs/theme":      true,
		"internal/emacs/meow":       true,
		"internal/emacs/minibuffer": true,
	},
	"internal/emacs/buffer": {
		"internal/emacs/theme": true,
		"internal/appconfig":   true,
		"internal/hostlink":    true,
	},
	"internal/emacs/window": {
		"internal/emacs/theme":     true,
		"internal/emacs/buffer":    true,
		"internal/emacs/workspace": true,
	},
	"internal/emacs/workspace": {
		"internal/emacs/buffer": true,
	},
	"internal/emacs/frame": {
		"internal/emacs/theme":      true,
		"internal/emacs/meow":       true,
		"internal/emacs/hydra":      true,
		"internal/emacs/tabs":       true,
		"internal/emacs/window":     true,
		"internal/emacs/workspace":  true,
		"internal/emacs/buffer":     true,
		"internal/emacs/modeline":   true,
		"internal/emacs/minibuffer": true,
		"internal/appconfig":        true,
		"internal/hostlink":         true,
		"internal/shellmanifest":    true,
		"internal/uiframework":      true,
	},
}

func TestGoPureFrontendAndEmacsHierarchyConstraints(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	fset := token.NewFileSet()
	checkedFiles := 0

	for _, top := range []string{"cmd", "internal"} {
		rootDir := filepath.Join(repoRoot, top)
		err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			relPkg := filepath.ToSlash(filepath.Dir(relPath))
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Errorf("parse %s: %v", relPath, err)
				return nil
			}
			checkedFiles++

			isEmacsPkg := strings.HasPrefix(relPkg, "internal/emacs/")
			allowedDeps, knownEmacsPkg := allowedEmacsInternalDeps[relPkg]
			if isEmacsPkg && !knownEmacsPkg {
				t.Errorf("MUI-EMACS-R01 violation: undeclared internal/emacs package %q (must be declared in dev/sysml/emacs-workbench.sysml and allowedEmacsInternalDeps)", relPkg)
			}

			for _, imp := range file.Imports {
				impPath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				if reason, bad := forbiddenAnywhereImports[impPath]; bad {
					t.Errorf("MUI-ARCH-R04 violation in %s: forbidden backend/runtime import %q (%s)", relPath, impPath, reason)
				}
				if isEmacsPkg {
					if reason, bad := forbiddenEmacsUIImports[impPath]; bad {
						t.Errorf("MUI-ARCH-R04 violation in %s: forbidden I/O import %q in pure Emacs UI package (%s)", relPath, impPath, reason)
					}
					if strings.HasPrefix(impPath, modulePrefix) {
						targetPkg := strings.TrimPrefix(impPath, modulePrefix)
						if knownEmacsPkg && !allowedDeps[targetPkg] {
							t.Errorf("MUI-EMACS-R01 violation in %s: package %q may not import %q (violates declared acyclic hierarchy)", relPath, relPkg, targetPkg)
						}
					}
				}
			}

			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				key := ident.Name + "." + sel.Sel.Name
				if reason, bad := forbiddenServerCalls[key]; bad {
					t.Errorf("MUI-ARCH-R04 violation in %s: forbidden call %s (%s)", relPath, key, reason)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}

	if checkedFiles == 0 {
		t.Fatalf("expected to inspect production Go files, found 0")
	}
}

// TestQuintConcurrencyInvariantsAST mechanically enforces our Deterministic
// Engineering Quint concurrency laws against production Go AST:
//  1. StructuredCancellation.qnt (child-quiescence-barrier): zero raw `go ...`
//     goroutine statements in cmd/marlin-desktop, internal/featurekit,
//     internal/appconfig, and internal/emacs/* (must use StructuredScope / FencedLane).
//  2. UiMainThreadRcuSnapshot.qnt (off-thread-io-lock-free): zero `f.mu.Lock()`
//     acquisitions inside Frame.View, hydraBodyFor, or buildWidgetRuntime.
func TestQuintConcurrencyInvariantsAST(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	fset := token.NewFileSet()

	structuredDirs := []string{
		"cmd/marlin-desktop",
		"internal/featurekit",
		"internal/appconfig",
		"internal/emacs",
	}
	for _, sub := range structuredDirs {
		root := filepath.Join(repoRoot, sub)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, _ := filepath.Rel(repoRoot, path)
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				t.Errorf("parse %s: %v", relPath, parseErr)
				return nil
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if goStmt, ok := n.(*ast.GoStmt); ok {
					pos := fset.Position(goStmt.Pos())
					t.Errorf("Quint StructuredCancellation.qnt violation in %s:%d: raw fire-and-forget `go` statement forbidden (use async.StructuredScope.Go or async.FencedLane)", relPath, pos.Line)
				}
				if fn, ok := n.(*ast.FuncDecl); ok {
					switch fn.Name.Name {
					case "View", "hydraBodyFor", "buildWidgetRuntime", "syncFromGRPCWorkbench":
						if fn.Recv != nil && fn.Body != nil {
							ast.Inspect(fn.Body, func(inner ast.Node) bool {
								if lit, isLit := inner.(*ast.FuncLit); isLit {
									// Ignore deferred UI click callbacks (OnAddTask/OnCycleTask) that run on user click, not during render.
									_ = lit
									return false
								}
								call, ok := inner.(*ast.CallExpr)
								if !ok {
									return true
								}
								sel, ok := call.Fun.(*ast.SelectorExpr)
								if !ok || sel.Sel.Name != "Lock" {
									return true
								}
								if xSel, ok := sel.X.(*ast.SelectorExpr); ok && xSel.Sel.Name == "mu" {
									pos := fset.Position(call.Pos())
									t.Errorf("Quint UiMainThreadRcuSnapshot.qnt violation in %s:%d (%s): render-thread f.mu.Lock() forbidden (read wait-free RCU frameSnapshot)", relPath, pos.Line, fn.Name.Name)
								}
								return true
							})
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
}

// TestFrameAntiFlickerAndOverflowInvariantsAST mechanically enforces Frame-wide
// visual stability guarantees across EVERY Emacs Window, Buffer, Popup, Split,
// Modeline, Tab Bar, and Minibuffer under internal/emacs/*:
//  1. MUI-UI-R01 (Zero Text-Overflow Collision): chaining .NoWrap() with
//     .MaxLines(...) is forbidden because MyGo's NoWrap explicitly allows text
//     to paint outside its bounding box onto adjacent columns. Single-line
//     cells MUST use .SingleLine().Ellipsis("…").MinWidth(0).Clip().
//  2. MUI-UI-R02 (Zero Synchronous Invalidation Storms): Frame.View,
//     OpenMinibuffer, OpenHydra, and EnterInsert must route redraw requests
//     through f.requestFrame() (inView-guarded + FramePacer-paced) instead of
//     calling raw f.invalidate() directly.
func TestFrameAntiFlickerAndOverflowInvariantsAST(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	fset := token.NewFileSet()
	root := filepath.Join(repoRoot, "internal", "emacs")

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relPath, _ := filepath.Rel(repoRoot, path)
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Errorf("parse %s: %v", relPath, parseErr)
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					// MUI-UI-R01: forbid NoWrap().MaxLines(...) or MaxLines(...).NoWrap().
					if sel.Sel.Name == "MaxLines" || sel.Sel.Name == "NoWrap" {
						if prevCall, ok := sel.X.(*ast.CallExpr); ok {
							if prevSel, ok := prevCall.Fun.(*ast.SelectorExpr); ok {
								if (sel.Sel.Name == "MaxLines" && prevSel.Sel.Name == "NoWrap") ||
									(sel.Sel.Name == "NoWrap" && prevSel.Sel.Name == "MaxLines") {
									pos := fset.Position(call.Pos())
									t.Errorf("MUI-UI-R01 violation in %s:%d: .%s().%s() overflows adjacent columns in MyGo (use .SingleLine().Ellipsis(\"…\").MinWidth(0).Clip())",
										relPath, pos.Line, prevSel.Sel.Name, sel.Sel.Name)
								}
							}
						}
					}
				}
			}
			// MUI-UI-R02: forbid raw f.invalidate() inside View / OpenMinibuffer / OpenHydra / EnterInsert.
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Body != nil {
				switch fn.Name.Name {
				case "View", "OpenMinibuffer", "OpenHydra", "EnterInsert":
					ast.Inspect(fn.Body, func(inner ast.Node) bool {
						if _, isLit := inner.(*ast.FuncLit); isLit {
							return false
						}
						c, ok := inner.(*ast.CallExpr)
						if !ok {
							return true
						}
						sel, ok := c.Fun.(*ast.SelectorExpr)
						if ok && sel.Sel.Name == "invalidate" {
							pos := fset.Position(c.Pos())
							t.Errorf("MUI-UI-R02 violation in %s:%d (%s): direct f.invalidate() causes redundant double-frame flicker; use f.requestFrame()",
								relPath, pos.Line, fn.Name.Name)
						}
						return true
					})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/emacs: %v", err)
	}
}
