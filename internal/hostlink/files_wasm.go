// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build js

package hostlink

import (
	"context"
	"fmt"
)

// FileEntry describes one real directory or file entry returned for Vertico.
type FileEntry struct {
	Name    string
	Path    string
	RelPath string
	IsDir   bool
	Size    int64
	Summary string
}

// DirListing is the live directory snapshot for Vertico directory navigation.
type DirListing struct {
	Dir           string
	Parent        string
	Home          string
	WorkspaceRoot string
	Entries       []FileEntry
}

// FileContent holds file content read from a remote host.
type FileContent struct {
	Title     string
	Path      string
	RelPath   string
	Size      int64
	Lines     int
	Truncated bool
	Binary    bool
	Content   string
}

// GrepHit describes one live consult-ripgrep match projected by the Rust host.
type GrepHit struct {
	RelPath string
	Path    string
	Line    int
	Col     int
	Snippet string
}

// ProjectInfo describes one real Git/jj repository discovered by the Rust host.
type ProjectInfo struct {
	Name string
	Path string
	VCS  string
}

// SymbolHit describes one live ASP AST symbol (`consult-imenu`) projected by Rust.
type SymbolHit struct {
	Symbol   string
	Kind     string
	RelPath  string
	Path     string
	Line     int
	Language string
}

// DefaultWorkspaceRoot returns the browser WASM default virtual root.
func DefaultWorkspaceRoot() string { return "/" }

func (l *Link) ListDirectory(_ context.Context, _ string) (DirListing, error) {
	return DirListing{}, fmt.Errorf("connect a remote Marlin host to browse files in WebAssembly")
}

func (l *Link) ListProjectFiles(_ context.Context, _ string) (string, []FileEntry, error) {
	return "", nil, fmt.Errorf("connect a remote Marlin host to list project files in WebAssembly")
}

func (l *Link) ReadFileContent(_ context.Context, _ string) (FileContent, error) {
	return FileContent{}, fmt.Errorf("connect a remote Marlin host to read files in WebAssembly")
}

func (l *Link) SearchProjectGrep(_ context.Context, _, _ string) ([]GrepHit, error) {
	return nil, fmt.Errorf("connect a remote Marlin host for consult-ripgrep in WebAssembly")
}

func (l *Link) ListKnownProjects(_ context.Context, _ string) ([]ProjectInfo, error) {
	return nil, fmt.Errorf("connect a remote Marlin host to list projects in WebAssembly")
}

func (l *Link) SearchProjectSymbols(_ context.Context, _, _, _ string) ([]SymbolHit, error) {
	return nil, fmt.Errorf("connect a remote Marlin host for consult-imenu in WebAssembly")
}
