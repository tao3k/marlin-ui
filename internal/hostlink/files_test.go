// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFilesystemBrowseProjectFilesAndRead(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	srcFile := filepath.Join(tmp, "src", "main.rs")
	if err := os.WriteFile(srcFile, []byte("fn main() {\n    println!(\"hello\");\n}\n"), 0o644); err != nil {
		t.Fatalf("write main.rs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "node_modules", "pkg", "ignored.js"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ignored.js: %v", err)
	}

	var nilLink *Link
	listing, err := nilLink.ListDirectory(context.Background(), tmp)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	foundSrc := false
	for _, e := range listing.Entries {
		if e.Name == "src/" && e.IsDir {
			foundSrc = true
		}
	}
	if !foundSrc {
		t.Fatalf("expected src/ directory in listing, got %+v", listing.Entries)
	}

	root, projFiles, err := nilLink.ListProjectFiles(context.Background(), filepath.Join(tmp, "src"))
	if err != nil {
		t.Fatalf("ListProjectFiles: %v", err)
	}
	if root != tmp {
		t.Fatalf("expected discovered repo root %q, got %q", tmp, root)
	}
	foundMain := false
	for _, pf := range projFiles {
		if pf.RelPath == "src/main.rs" {
			foundMain = true
		}
		if strings.Contains(pf.RelPath, "node_modules") {
			t.Fatalf("expected node_modules ignored, found %q", pf.RelPath)
		}
	}
	if !foundMain {
		t.Fatalf("expected src/main.rs in project files, got %+v", projFiles)
	}

	fc, err := nilLink.ReadFileContent(context.Background(), srcFile)
	if err != nil {
		t.Fatalf("ReadFileContent: %v", err)
	}
	if fc.Title != "main.rs" || !strings.Contains(fc.Content, "fn main()") {
		t.Fatalf("unexpected file content: %+v", fc)
	}
}
