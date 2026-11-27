// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package archguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// mygoPreRunForbiddenCalls enumerates MyGo APIs guarded by needsApp(...) or
// ready-state assertions that panic if invoked on the main thread before
// mygo.App.Run() enters App.WhenReady.
//
// Prefix entries ending with "." match any method under that namespace
// (e.g. "Clipboard." matches Clipboard.ReadText, Clipboard.WriteText, etc.).
var mygoPreRunForbiddenExact = map[string]bool{
	"App.Focus":                true,
	"App.IsActive":             true,
	"App.Hide":                 true,
	"App.Show":                 true,
	"App.IsHidden":             true,
	"App.SetBadgeCount":        true,
	"App.ShowAboutPanel":       true,
	"App.ClearBrowsingData":    true,
	"App.AddRecentDocument":    true,
	"App.ClearRecentDocuments": true,
	"App.RestoredState":        true,
	"App.Dock.SetBadge":        true,
	"App.Dock.Badge":           true,
	"App.Dock.Bounce":          true,
	"App.Dock.CancelBounce":    true,
	"App.Dock.SetIcon":         true,
	"App.JumpList.SetTasks":    true,
	"NewWindow":                true,
	"NewTray":                  true,
	"NewTabs":                  true,
	"NewPluginView":            true,
	"WatchFiles":               true,
	"CompileContentRules":      true,
	"Share":                    true,
	"NotificationsSupported":   true,
	"Theme.IsDark":             true,
	"Theme.Preferences":        true,
	"Theme.WindowButtons":      true,
	"GlobalShortcut.Register":  true,
	"Power.KeepAwake":          true,
	"Power.KeepRunning":        true,
	"Power.IdleTime":           true,
	"Power.Battery":            true,
}

var mygoPreRunForbiddenPrefixes = []string{
	"Dialog.",
	"Shell.",
	"Clipboard.",
	"Screen.",
	"Cookies.",
	"Documents.",
	"Network.",
}

// mygoDeferredCallbackRegistrations are MyGo APIs whose function literal
// arguments execute after the runtime event loop is initialized, not during
// synchronous pre-Run startup.
var mygoDeferredCallbackRegistrations = map[string]bool{
	"App.WhenReady":         true,
	"App.OnSecondInstance":  true,
	"App.OnOpenFile":        true,
	"App.OnBeforeQuit":      true,
	"App.OnWillQuit":        true,
	"App.OnQuit":            true,
	"App.OnWindowAllClosed": true,
	"App.OnActivate":        true,
	"App.OnOpenURL":         true,
	"Bind":                  true,
}

func isForbiddenMyGoPreRunCall(mygoTarget string) bool {
	if mygoPreRunForbiddenExact[mygoTarget] {
		return true
	}
	for _, prefix := range mygoPreRunForbiddenPrefixes {
		if strings.HasPrefix(mygoTarget, prefix) {
			return true
		}
	}
	return false
}

// selectorChain flattens a nested SelectorExpr rooted at an Ident into dot form
// (e.g. mygo.App.Dock.SetIcon -> "mygo.App.Dock.SetIcon").
func selectorChain(expr ast.Expr) (string, bool) {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name, true
	case *ast.SelectorExpr:
		base, ok := selectorChain(x.X)
		if !ok {
			return "", false
		}
		return base + "." + x.Sel.Name, true
	default:
		return "", false
	}
}

type mygoPreRunViolation struct {
	File string
	Line int
	Func string
	Call string
}

// checkMyGoPreRunPackage inspects all parsed files in a cmd/* package and
// verifies that neither init() nor main() — nor any same-package helper called
// synchronously before mygo.App.Run() — invokes a needsApp-gated MyGo API
// outside a deferred lifecycle callback (such as mygo.App.WhenReady).
func checkMyGoPreRunPackage(fset *token.FileSet, files map[string]*ast.File) []mygoPreRunViolation {
	funcs := make(map[string]*ast.FuncDecl)
	fileForFunc := make(map[string]string)
	mygoAliases := make(map[string]map[string]bool)

	for relPath, file := range files {
		aliases := map[string]bool{}
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, `"`) == "github.com/egoist/mygo" {
				if imp.Name != nil {
					aliases[imp.Name.Name] = true
				} else {
					aliases["mygo"] = true
				}
			}
		}
		mygoAliases[relPath] = aliases

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			funcs[fn.Name.Name] = fn
			fileForFunc[fn.Name.Name] = relPath
		}
	}

	var violations []mygoPreRunViolation
	visited := make(map[string]bool)

	var scanNode func(fnName, relPath string, aliases map[string]bool, node ast.Node)
	scanNode = func(fnName, relPath string, aliases map[string]bool, node ast.Node) {
		if node == nil {
			return
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			// Function literals passed as callbacks (e.g. OnReady: func(...) {...}
			// or mygo.App.WhenReady(func() {...})) do not execute inline unless
			// immediately invoked as an IIFE.
			if _, isLit := n.(*ast.FuncLit); isLit {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// Immediately-invoked function literal: func() { ... }().
			if iife, ok := call.Fun.(*ast.FuncLit); ok {
				scanNode(fnName, relPath, aliases, iife.Body)
				return false
			}
			chain, ok := selectorChain(call.Fun)
			if ok {
				if dot := strings.IndexByte(chain, '.'); dot > 0 {
					pkgIdent := chain[:dot]
					member := chain[dot+1:]
					if aliases[pkgIdent] {
						if isForbiddenMyGoPreRunCall(member) {
							pos := fset.Position(call.Pos())
							violations = append(violations, mygoPreRunViolation{
								File: relPath,
								Line: pos.Line,
								Func: fnName,
								Call: "mygo." + member,
							})
						}
						if mygoDeferredCallbackRegistrations[member] {
							return false
						}
					}
				} else if callee, exists := funcs[chain]; exists && !visited[chain] {
					visited[chain] = true
					calleeFile := fileForFunc[chain]
					scanNode(chain, calleeFile, mygoAliases[calleeFile], callee.Body)
				}
			}
			return true
		})
	}

	// Scan all init() functions and main() synchronously.
	for relPath, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			if fn.Name.Name == "init" {
				scanNode("init", relPath, mygoAliases[relPath], fn.Body)
			}
		}
	}
	if mainFn, ok := funcs["main"]; ok {
		visited["main"] = true
		mainFile := fileForFunc["main"]
		scanNode("main", mainFile, mygoAliases[mainFile], mainFn.Body)
	}
	return violations
}

// TestMyGoPreRunLifecycleInvariantsAST enforces MUI-LIFECYCLE-R01 across every
// cmd/* entrypoint: no needsApp-gated MyGo API (such as mygo.App.Dock.SetIcon,
// mygo.NewWindow, mygo.NewTray, or mygo.Clipboard.*) may be called before
// mygo.App.Run() / outside mygo.App.WhenReady.
func TestMyGoPreRunLifecycleInvariantsAST(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	cmdRoot := filepath.Join(repoRoot, "cmd")
	entries, err := filepath.Glob(filepath.Join(cmdRoot, "*"))
	if err != nil {
		t.Fatalf("glob cmd/*: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one cmd/* package under %s", cmdRoot)
	}

	checkedPkgs := 0
	for _, pkgDir := range entries {
		fset := token.NewFileSet()
		files := make(map[string]*ast.File)
		walkErr := filepath.WalkDir(pkgDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, _ := filepath.Rel(repoRoot, path)
			parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				t.Errorf("parse %s: %v", relPath, parseErr)
				return nil
			}
			files[filepath.ToSlash(relPath)] = parsed
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", pkgDir, walkErr)
		}
		if len(files) == 0 {
			continue
		}
		checkedPkgs++
		for _, v := range checkMyGoPreRunPackage(fset, files) {
			t.Errorf("MUI-LIFECYCLE-R01 violation in %s:%d (%s): %s called before mygo.App.Run; move inside mygo.App.WhenReady",
				v.File, v.Line, v.Func, v.Call)
		}
	}
	if checkedPkgs == 0 {
		t.Fatalf("expected to inspect cmd/* packages, found 0")
	}
}

// TestMyGoPreRunLifecycleGuardCatchesViolations verifies that the AST guard
// reliably catches direct and transitive pre-Run MyGo violations while allowing
// properly gated App.WhenReady and pre-Run configuration calls.
func TestMyGoPreRunLifecycleGuardCatchesViolations(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantCall string
	}{
		{
			name: "direct App.Dock.SetIcon in main fails",
			src: `package main
import "github.com/egoist/mygo"
func main() {
	mygo.App.SetName("Marlin")
	_ = mygo.App.Dock.SetIcon(nil)
	mygo.App.Run()
}`,
			wantCall: "mygo.App.Dock.SetIcon",
		},
		{
			name: "transitive helper calling App.Dock.SetIcon before Run fails",
			src: `package main
import "github.com/egoist/mygo"
func initIcon() { _ = mygo.App.Dock.SetIcon(nil) }
func main() {
	initIcon()
	mygo.App.Run()
}`,
			wantCall: "mygo.App.Dock.SetIcon",
		},
		{
			name: "direct NewWindow before WhenReady fails",
			src: `package main
import "github.com/egoist/mygo"
func main() {
	_ = mygo.NewWindow(mygo.WindowOptions{})
	mygo.App.Run()
}`,
			wantCall: "mygo.NewWindow",
		},
		{
			name: "aliased mygo import calling NewTray before WhenReady fails",
			src: `package main
import mg "github.com/egoist/mygo"
func main() {
	_, _ = mg.NewTray(mg.TrayOptions{})
	mg.App.Run()
}`,
			wantCall: "mygo.NewTray",
		},
		{
			name: "WhenReady and OnBeforeQuit callbacks pass cleanly",
			src: `package main
import "github.com/egoist/mygo"
func setupReady() {
	_ = mygo.App.Dock.SetIcon(nil)
	_ = mygo.NewWindow(mygo.WindowOptions{})
}
func main() {
	mygo.App.SetName("Marlin")
	mygo.App.OnSecondInstance(func(_ []string, _ string) {
		mygo.App.Focus()
	})
	mygo.App.WhenReady(func() {
		setupReady()
	})
	mygo.App.Run()
}`,
			wantCall: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "main.go", tc.src, 0)
			if err != nil {
				t.Fatalf("parse snippet: %v", err)
			}
			got := checkMyGoPreRunPackage(fset, map[string]*ast.File{"cmd/synthetic/main.go": file})
			if tc.wantCall == "" {
				if len(got) != 0 {
					t.Fatalf("expected 0 violations, got %+v", got)
				}
				return
			}
			found := false
			for _, v := range got {
				if v.Call == tc.wantCall {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected violation %q, got %s", tc.wantCall, fmt.Sprintf("%+v", got))
			}
		})
	}
}
