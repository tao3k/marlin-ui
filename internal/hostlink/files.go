// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
)

// FileEntry describes one real directory or file entry returned for Vertico
// `find-file` (`C-x C-f` / `SPC f f`) and `project-find-file` (`SPC p f`).
type FileEntry struct {
	Name    string // display name (trailing "/" for directories, or relative path in project-find-file)
	Path    string // absolute filesystem path
	RelPath string // project-relative path (populated for project-find-file)
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

// FileContent holds real UTF-8 (or binary-notice) file content read from disk or remote gRPC host.
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

var ignoredProjectDirs = map[string]bool{
	".git":         true,
	".jj":          true,
	"target":       true,
	"node_modules": true,
	".gerbil":      true,
	".cache":       true,
	".devenv":      true,
	".direnv":      true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
}

// DefaultWorkspaceRoot resolves the active Git/jj workspace root on the local host:
// 1. MARLIN_BRIDGE_WORKSPACE_ROOT environment variable (set by `just dev-desktop`)
// 2. Nearest ancestor containing `.git` or `.jj` from `os.Getwd()`
// 3. `os.Getwd()` fallback
func DefaultWorkspaceRoot() string {
	if envRoot := strings.TrimSpace(os.Getenv("MARLIN_BRIDGE_WORKSPACE_ROOT")); envRoot != "" {
		if abs, err := filepath.Abs(envRoot); err == nil {
			if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
				return abs
			}
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return discoverLocalRepoRoot(cwd)
}

func discoverLocalRepoRoot(start string) string {
	abs, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	cur := abs
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		if info, err := os.Stat(filepath.Join(cur, ".jj")); err == nil && info.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		cur = parent
	}
}

func expandLocalPath(raw, base string) string {
	trimmed := strings.TrimSpace(raw)
	home, _ := os.UserHomeDir()
	if trimmed == "" {
		if base != "" {
			return base
		}
		return DefaultWorkspaceRoot()
	}
	if trimmed == "~" && home != "" {
		return home
	}
	if strings.HasPrefix(trimmed, "~/") && home != "" {
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	if base == "" {
		base = DefaultWorkspaceRoot()
	}
	return filepath.Clean(filepath.Join(base, trimmed))
}

// ListDirectory fetches real directory entries over live Rust HTTP/2 gRPC
// (`fs:ls@<dir>`, serving both local sidecar and remote Marlin Authority Hosts
// uniformly without legacy TRAMP/SSH hacks), transparently falling back to
// direct local disk reading when standalone.
func (l *Link) ListDirectory(ctx context.Context, dir string) (DirListing, error) {
	if l != nil && l.Switcher != nil {
		if target := l.Switcher.CurrentTarget(); target != nil && target.Projection() != nil {
			q := "fs:ls"
			if strings.TrimSpace(dir) != "" {
				q = "fs:ls@" + strings.TrimSpace(dir)
			}
			qctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			resp, err := target.Projection().QueryProjectGraph(qctx, &projectionv1.QueryProjectGraphRequest{
				ProjectId:        "default",
				Query:            q,
				SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_UNSPECIFIED,
			})
			cancel()
			if err == nil {
				if listing, ok := decodeDirListingRecords(resp.GetRecords()); ok {
					return listing, nil
				}
			}
		}
	}
	return listDirectoryLocal(dir)
}

// ListProjectFiles fetches all real non-ignored files in the current Git/jj project
// via Rust gRPC (`fs:project-files@<root>`), falling back to local `filepath.WalkDir`.
func (l *Link) ListProjectFiles(ctx context.Context, root string) (string, []FileEntry, error) {
	if l != nil && l.Switcher != nil {
		if target := l.Switcher.CurrentTarget(); target != nil && target.Projection() != nil {
			q := "fs:project-files"
			if strings.TrimSpace(root) != "" {
				q = "fs:project-files@" + strings.TrimSpace(root)
			}
			qctx, cancel := context.WithTimeout(ctx, 6*time.Second)
			resp, err := target.Projection().QueryProjectGraph(qctx, &projectionv1.QueryProjectGraphRequest{
				ProjectId:        "default",
				Query:            q,
				SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_UNSPECIFIED,
			})
			cancel()
			if err == nil {
				if r, files, ok := decodeProjectFileRecords(resp.GetRecords()); ok {
					return r, files, nil
				}
			}
		}
	}
	return listProjectFilesLocal(root)
}

// ReadFileContent reads a real file via Rust gRPC (`fs:read@<path>`), falling back
// to direct local `os.ReadFile` when standalone.
func (l *Link) ReadFileContent(ctx context.Context, path string) (FileContent, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return FileContent{}, fmt.Errorf("empty file path")
	}
	if l != nil && l.Switcher != nil {
		if target := l.Switcher.CurrentTarget(); target != nil && target.Projection() != nil {
			qctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			resp, err := target.Projection().QueryProjectGraph(qctx, &projectionv1.QueryProjectGraphRequest{
				ProjectId:        "default",
				Query:            "fs:read@" + clean,
				SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_UNSPECIFIED,
			})
			cancel()
			if err == nil {
				if fc, ok := decodeFileContentRecords(resp.GetRecords()); ok {
					return fc, nil
				}
			}
		}
	}
	return readFileContentLocal(clean)
}

func decodeDirListingRecords(records []*projectionv1.GraphRecordView) (DirListing, bool) {
	var out DirListing
	foundMeta := false
	for _, rec := range records {
		if rec == nil {
			continue
		}
		f := rec.GetFacets()
		switch rec.GetKind() {
		case "fs.meta":
			foundMeta = true
			out.Dir = f["dir"]
			out.Parent = f["parent"]
			out.Home = f["home"]
			out.WorkspaceRoot = f["workspace_root"]
		case "fs.dir":
			out.Entries = append(out.Entries, FileEntry{
				Name:    f["name"],
				Path:    f["path"],
				IsDir:   true,
				Summary: "dir",
			})
		case "fs.file":
			sz, _ := strconv.ParseInt(f["size"], 10, 64)
			out.Entries = append(out.Entries, FileEntry{
				Name:    f["name"],
				Path:    f["path"],
				IsDir:   false,
				Size:    sz,
				Summary: rec.GetSummary(),
			})
		}
	}
	return out, foundMeta
}

func decodeProjectFileRecords(records []*projectionv1.GraphRecordView) (string, []FileEntry, bool) {
	root := ""
	foundMeta := false
	var files []FileEntry
	for _, rec := range records {
		if rec == nil {
			continue
		}
		f := rec.GetFacets()
		switch rec.GetKind() {
		case "fs.project_meta":
			foundMeta = true
			root = f["root"]
		case "fs.project_file":
			sz, _ := strconv.ParseInt(f["size"], 10, 64)
			files = append(files, FileEntry{
				Name:    f["rel_path"],
				RelPath: f["rel_path"],
				Path:    f["path"],
				Size:    sz,
				Summary: rec.GetSummary(),
			})
		}
	}
	return root, files, foundMeta
}

func decodeFileContentRecords(records []*projectionv1.GraphRecordView) (FileContent, bool) {
	for _, rec := range records {
		if rec == nil || rec.GetKind() != "fs.content" {
			continue
		}
		f := rec.GetFacets()
		sz, _ := strconv.ParseInt(f["size"], 10, 64)
		lines, _ := strconv.Atoi(f["lines"])
		return FileContent{
			Title:     rec.GetTitle(),
			Path:      f["path"],
			RelPath:   f["rel_path"],
			Size:      sz,
			Lines:     lines,
			Truncated: f["truncated"] == "true",
			Binary:    f["binary"] == "true",
			Content:   f["content"],
		}, true
	}
	return FileContent{}, false
}

func listDirectoryLocal(dir string) (DirListing, error) {
	wsRoot := DefaultWorkspaceRoot()
	resolved := expandLocalPath(dir, wsRoot)
	items, err := os.ReadDir(resolved)
	if err != nil {
		return DirListing{}, err
	}
	home, _ := os.UserHomeDir()
	var dirs, files []FileEntry
	for _, it := range items {
		name := it.Name()
		full := filepath.Join(resolved, name)
		info, _ := it.Info()
		if it.IsDir() {
			dirs = append(dirs, FileEntry{
				Name:    name + "/",
				Path:    full,
				IsDir:   true,
				Summary: "dir",
			})
		} else {
			var sz int64
			if info != nil {
				sz = info.Size()
			}
			files = append(files, FileEntry{
				Name:    name,
				Path:    full,
				IsDir:   false,
				Size:    sz,
				Summary: formatFileSize(sz),
			})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return lessFoldASCII(dirs[i].Name, dirs[j].Name) })
	sort.Slice(files, func(i, j int) bool { return lessFoldASCII(files[i].Name, files[j].Name) })
	entries := append(dirs, files...)
	if len(entries) > 512 {
		entries = entries[:512]
	}
	return DirListing{
		Dir:           resolved,
		Parent:        filepath.Dir(resolved),
		Home:          home,
		WorkspaceRoot: wsRoot,
		Entries:       entries,
	}, nil
}

func listProjectFilesLocal(root string) (string, []FileEntry, error) {
	base := expandLocalPath(root, DefaultWorkspaceRoot())
	repoRoot := discoverLocalRepoRoot(base)
	const maxFiles = 4000
	var out []FileEntry
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != repoRoot && ignoredProjectDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= maxFiles {
			return filepath.SkipAll
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		var sz int64
		if info, err := d.Info(); err == nil {
			sz = info.Size()
		}
		out = append(out, FileEntry{
			Name:    rel,
			RelPath: rel,
			Path:    path,
			Size:    sz,
			Summary: formatFileSize(sz),
		})
		return nil
	})
	if err != nil {
		return repoRoot, nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return repoRoot, out, nil
}

func readFileContentLocal(rawPath string) (FileContent, error) {
	wsRoot := DefaultWorkspaceRoot()
	abs := expandLocalPath(rawPath, wsRoot)
	data, err := os.ReadFile(abs)
	if err != nil {
		return FileContent{}, err
	}
	const maxBytes = 256 * 1024
	total := int64(len(data))
	truncated := len(data) > maxBytes
	slice := data
	if truncated {
		slice = data[:maxBytes]
	}
	isBinary := false
	checkLen := min(len(slice), 4096)
	for i := 0; i < checkLen; i++ {
		if slice[i] == 0 {
			isBinary = true
			break
		}
	}
	content := string(slice)
	if isBinary {
		content = fmt.Sprintf("[Binary file: %s (%s) — cannot render raw bytes as text]", abs, formatFileSize(total))
	}
	rel, err := filepath.Rel(wsRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = abs
	} else {
		rel = filepath.ToSlash(rel)
	}
	lines := strings.Count(content, "\n") + 1
	return FileContent{
		Title:     filepath.Base(abs),
		Path:      abs,
		RelPath:   rel,
		Size:      total,
		Lines:     lines,
		Truncated: truncated,
		Binary:    isBinary,
		Content:   content,
	}, nil
}

func lessFoldASCII(a, b string) bool {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return ca < cb
		}
	}
	return len(a) < len(b)
}

func formatFileSize(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
