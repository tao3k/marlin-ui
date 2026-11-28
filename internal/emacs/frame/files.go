// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"context"
	"fmt"
	"strings"

	"github.com/tao3k/marlin-mygo/runtime/async"

	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/minibuffer"
	"github.com/tao3k/marlin-ui/internal/hostlink"
)

type dirLoadResult struct {
	listing hostlink.DirListing
	errMsg  string
}

type projectFilesResult struct {
	root   string
	files  []hostlink.FileEntry
	errMsg string
}

type fileReadReq struct {
	path string
	line int
}

type fileReadResult struct {
	fc     hostlink.FileContent
	line   int
	errMsg string
}

func (f *Frame) initFileLanes() {
	f.dirListLane = async.NewFencedLane[string, dirLoadResult](
		func(ctx context.Context, dir string) (dirLoadResult, error) {
			listing, err := f.Link.ListDirectory(ctx, dir)
			if err != nil {
				return dirLoadResult{errMsg: err.Error()}, nil
			}
			return dirLoadResult{listing: listing}, nil
		},
		func(_ uint64, res dirLoadResult) {
			f.mu.Lock()
			f.findFileLoading = false
			if res.errMsg != "" {
				f.findFileErr = res.errMsg
			} else {
				f.findFileErr = ""
				f.findFileDir = res.listing.Dir
				f.findFileParent = res.listing.Parent
				f.findFileHome = res.listing.Home
				f.findFileEntries = res.listing.Entries
				f.rebuildFindFileCandCacheLocked()
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		},
	)
	f.projFilesLane = async.NewFencedLane[string, projectFilesResult](
		func(ctx context.Context, root string) (projectFilesResult, error) {
			r, files, err := f.Link.ListProjectFiles(ctx, root)
			if err != nil {
				return projectFilesResult{errMsg: err.Error()}, nil
			}
			return projectFilesResult{root: r, files: files}, nil
		},
		func(_ uint64, res projectFilesResult) {
			f.mu.Lock()
			f.projFilesLoading = false
			if res.errMsg != "" {
				f.projFilesErr = res.errMsg
			} else {
				f.projFilesErr = ""
				f.projFilesRoot = res.root
				f.projFilesEntries = res.files
				f.rebuildProjFileCandCacheLocked()
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		},
	)
	f.fileReadLane = async.NewFencedLane[fileReadReq, fileReadResult](
		func(ctx context.Context, req fileReadReq) (fileReadResult, error) {
			fc, err := f.Link.ReadFileContent(ctx, req.path)
			if err != nil {
				return fileReadResult{errMsg: err.Error()}, nil
			}
			return fileReadResult{fc: fc, line: req.line}, nil
		},
		func(_ uint64, res fileReadResult) {
			if res.errMsg != "" {
				f.Echo("Find file error: " + res.errMsg)
				return
			}
			f.openLoadedFile(res.fc, res.line)
		},
	)
	f.initConsultLanes()
}

func (f *Frame) activeWorkspaceProjectRoot() string {
	if f.wsMgr != nil {
		if pr := strings.TrimSpace(f.wsMgr.Snapshot().Active.ProjectRoot); pr != "" {
			return pr
		}
	}
	return hostlink.DefaultWorkspaceRoot()
}

// OpenFindFile opens authentic Emacs Vertico `find-file` (`C-x C-f` / `SPC f f`)
// starting in the active workspace's real directory.
func (f *Frame) OpenFindFile() {
	f.mu.Lock()
	dir := f.findFileDir
	f.mu.Unlock()
	if strings.TrimSpace(dir) == "" {
		dir = f.activeWorkspaceProjectRoot()
	}
	f.loadFindFileDir(dir)
	f.OpenMinibuffer(minibuffer.ModeFindFile)
}

func (f *Frame) loadFindFileDir(dir string) {
	f.mu.Lock()
	f.findFileDir = dir
	f.findFileLoading = true
	f.findFileErr = ""
	f.mu.Unlock()
	if f.dirListLane != nil {
		f.dirListLane.Submit(dir)
		return
	}
	if listing, err := f.Link.ListDirectory(context.Background(), dir); err == nil {
		f.mu.Lock()
		f.findFileLoading = false
		f.findFileDir = listing.Dir
		f.findFileParent = listing.Parent
		f.findFileHome = listing.Home
		f.findFileEntries = listing.Entries
		f.rebuildFindFileCandCacheLocked()
		f.mu.Unlock()
	}
}

func (f *Frame) rebuildFindFileCandCacheLocked() {
	all := make([]minibuffer.Candidate, 0, len(f.findFileEntries)+1)
	if f.findFileParent != "" && f.findFileParent != f.findFileDir {
		p := f.findFileParent
		all = append(all, minibuffer.Candidate{
			ID:         "ff-parent",
			Name:       "../",
			Desc:       "Parent directory (" + p + ")",
			Annotation: "dir · DEL",
			KeepOpen:   true,
			Run:        func() { f.FindFileEnterDir(p) },
		})
	}
	for _, e := range f.findFileEntries {
		ent := e
		if ent.IsDir {
			all = append(all, minibuffer.Candidate{
				ID:         "ff-dir-" + ent.Path,
				Name:       ent.Name,
				Desc:       ent.Path,
				Annotation: "dir",
				KeepOpen:   true,
				Run:        func() { f.FindFileEnterDir(ent.Path) },
			})
		} else {
			all = append(all, minibuffer.Candidate{
				ID:         "ff-file-" + ent.Path,
				Name:       ent.Name,
				Desc:       ent.Path,
				Annotation: ent.Summary,
				KeepOpen:   false,
				Run:        func() { f.OpenDiskFile(ent.Path) },
			})
		}
	}
	f.findFileCandCache = all
}

func (f *Frame) rebuildProjFileCandCacheLocked() {
	all := make([]minibuffer.Candidate, 0, len(f.projFilesEntries))
	for _, e := range f.projFilesEntries {
		ent := e
		all = append(all, minibuffer.Candidate{
			ID:         "pf-" + ent.Path,
			Name:       ent.RelPath,
			Desc:       ent.Path,
			Annotation: ent.Summary,
			Run:        func() { f.OpenDiskFile(ent.Path) },
		})
	}
	f.projFileCandCache = all
}

// FindFileEnterDir descends into subDir while keeping Vertico open (`vertico-directory-enter`).
func (f *Frame) FindFileEnterDir(subDir string) {
	f.loadFindFileDir(subDir)
	f.requestFrame()
}

// FindFileUp ascends one parent directory (`vertico-directory-up` on Backspace / C-l / ../).
func (f *Frame) FindFileUp() {
	f.mu.Lock()
	parent := f.findFileParent
	cur := f.findFileDir
	f.mu.Unlock()
	if parent != "" && parent != cur {
		f.loadFindFileDir(parent)
		f.requestFrame()
	}
}

// FindFileJump handles `vertico-directory-tidy` shortcuts (`~/`, `/`, or `subdir/`).
func (f *Frame) FindFileJump(spec string) {
	clean := strings.TrimSpace(spec)
	if clean == "" {
		return
	}
	f.mu.Lock()
	cur := f.findFileDir
	home := f.findFileHome
	f.mu.Unlock()
	target := clean
	switch {
	case clean == "~/" && home != "":
		target = home
	case clean == "/":
		target = "/"
	case strings.HasPrefix(clean, "/"):
		target = strings.TrimRight(clean, "/")
		if target == "" {
			target = "/"
		}
	default:
		seg := strings.TrimRight(clean, "/")
		if cur != "" {
			target = strings.TrimRight(cur, "/") + "/" + seg
		} else {
			target = seg
		}
	}
	f.loadFindFileDir(target)
	f.requestFrame()
}

func (f *Frame) currentFindFileDirDisplay() string {
	f.mu.Lock()
	dir := f.findFileDir
	home := f.findFileHome
	f.mu.Unlock()
	if home != "" && strings.HasPrefix(dir, home) {
		return "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}

func (f *Frame) findFileCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	curDir := f.findFileDir
	cached := f.findFileCandCache
	loading := f.findFileLoading
	errMsg := f.findFileErr
	f.mu.Unlock()

	if errMsg != "" && len(cached) == 0 {
		return []minibuffer.Candidate{{
			ID:         "ff-err",
			Name:       "../",
			Desc:       "Error: " + errMsg,
			Annotation: "go up",
			KeepOpen:   true,
			Run:        f.FindFileUp,
		}}
	}
	if loading && len(cached) == 0 {
		if strings.TrimSpace(query) != "" {
			return nil
		}
		return []minibuffer.Candidate{{
			ID:   "ff-loading",
			Name: "Loading directory...",
			Desc: curDir,
		}}
	}
	return minibuffer.FilterCandidates(cached, query)
}

// OpenProjectFindFile opens authentic Emacs `project-find-file` / `projectile-find-file`
// (`C-c p f` / `SPC p f` / `SPC f p`), indexing real files in the active Git/jj repo.
func (f *Frame) OpenProjectFindFile() {
	root := f.activeWorkspaceProjectRoot()
	f.mu.Lock()
	f.projFilesLoading = true
	f.projFilesErr = ""
	if f.projFilesRoot == "" {
		f.projFilesRoot = root
	}
	f.mu.Unlock()
	if f.projFilesLane != nil {
		f.projFilesLane.Submit(root)
	} else if r, files, err := f.Link.ListProjectFiles(context.Background(), root); err == nil {
		f.mu.Lock()
		f.projFilesLoading = false
		f.projFilesRoot = r
		f.projFilesEntries = files
		f.rebuildProjFileCandCacheLocked()
		f.mu.Unlock()
	}
	f.OpenMinibuffer(minibuffer.ModeProjectFindFile)
}

func (f *Frame) currentProjectRootLabel() string {
	f.mu.Lock()
	r := f.projFilesRoot
	f.mu.Unlock()
	if r == "" {
		r = f.activeWorkspaceProjectRoot()
	}
	if idx := strings.LastIndex(strings.TrimRight(r, "/"), "/"); idx >= 0 {
		return r[idx+1:]
	}
	return r
}

func (f *Frame) projectFileCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	cached := f.projFileCandCache
	loading := f.projFilesLoading
	errMsg := f.projFilesErr
	root := f.projFilesRoot
	f.mu.Unlock()

	if errMsg != "" && len(cached) == 0 {
		return []minibuffer.Candidate{{
			ID:   "pf-err",
			Name: "Error scanning project",
			Desc: errMsg,
		}}
	}
	if loading && len(cached) == 0 {
		if strings.TrimSpace(query) != "" {
			return nil
		}
		return []minibuffer.Candidate{{
			ID:   "pf-loading",
			Name: "Indexing project files...",
			Desc: root,
		}}
	}
	return minibuffer.FilterCandidates(cached, query)
}

// OpenRecentFiles opens Emacs `recentf-open-files` (`C-x C-r` / `SPC f r`).
func (f *Frame) OpenRecentFiles() {
	f.OpenMinibuffer(minibuffer.ModeRecentFiles)
}

func (f *Frame) recentFileCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	recents := append([]string(nil), f.recentFiles...)
	f.mu.Unlock()
	if len(recents) == 0 {
		return []minibuffer.Candidate{{
			ID:   "rf-empty",
			Name: "No recent files yet — press C-x C-f / SPC f f to open a file",
			Desc: "recentf ring is empty",
			Run:  f.OpenFindFile,
		}}
	}
	all := make([]minibuffer.Candidate, 0, len(recents))
	for _, p := range recents {
		abs := p
		base := abs
		if idx := strings.LastIndex(abs, "/"); idx >= 0 && idx+1 < len(abs) {
			base = abs[idx+1:]
		}
		all = append(all, minibuffer.Candidate{
			ID:         "rf-" + abs,
			Name:       base,
			Desc:       abs,
			Annotation: "recentf",
			Run:        func() { f.OpenDiskFile(abs) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

// OpenDiskFile reads a real file from the active host (via Rust gRPC `fs:read@<path>`
// or local disk when standalone) and displays it cleanly in the workbench.
func (f *Frame) OpenDiskFile(path string) {
	f.OpenDiskFileAtLine(path, 1)
}

// OpenDiskFileAtLine opens a real file from the host and highlights targetLine
// inside *Preview* (used by consult-ripgrep and consult-line jumps).
func (f *Frame) OpenDiskFileAtLine(path string, targetLine int) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return
	}
	if targetLine < 1 {
		targetLine = 1
	}
	if f.fileReadLane != nil {
		f.fileReadLane.Submit(fileReadReq{path: clean, line: targetLine})
		return
	}
	fc, err := f.Link.ReadFileContent(context.Background(), clean)
	if err != nil {
		f.Echo("Open file error: " + err.Error())
		return
	}
	f.openLoadedFile(fc, targetLine)
}

func (f *Frame) openLoadedFile(fc hostlink.FileContent, targetLine int) {
	f.recordRecentFile(fc.Path)
	if targetLine < 1 {
		targetLine = 1
	}
	title := fc.Title
	if title == "" {
		title = fc.RelPath
	}
	pathLabel := fc.RelPath
	if pathLabel == "" {
		pathLabel = fc.Path
	}
	f.mu.Lock()
	f.filePreview = &buffer.FilePreviewState{
		Title:   title,
		Path:    pathLabel,
		Line:    targetLine,
		Content: fc.Content,
	}
	f.publishSnapshotLocked()
	f.mu.Unlock()
	// Promote *Preview* directly into the active main window so real files
	// opened via C-x C-f / SPC f f / SPC p f get full editorial readability.
	if f.wsMgr != nil {
		_, wsSnap := f.wsMgr.OpenBufferWithAction(buffer.Preview, "reuse-window")
		if wsSnap.Active.PopupVisible && wsSnap.Active.PopupBuffer == buffer.Preview {
			_, wsSnap = f.wsMgr.RaisePopup()
		}
		f.mu.Lock()
		f.activeBuffer = wsSnap.Active.FocusedBuffer()
		f.activeFeatureID = ""
		f.activeExtID = ""
		f.activePluginID = ""
		f.openTabs = append([]buffer.ID(nil), wsSnap.Active.Buffers...)
		f.publishSnapshotLocked()
		f.mu.Unlock()
		f.requestFrame()
	} else {
		f.SelectBuffer(buffer.Preview)
	}
	f.Echo(fmt.Sprintf("Opened %s (%d lines)", pathLabel, fc.Lines))
}

func (f *Frame) recordRecentFile(path string) {
	p := strings.TrimSpace(path)
	if p == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	next := make([]string, 0, len(f.recentFiles)+1)
	next = append(next, p)
	for _, old := range f.recentFiles {
		if old != p && len(next) < 40 {
			next = append(next, old)
		}
	}
	f.recentFiles = next
}
