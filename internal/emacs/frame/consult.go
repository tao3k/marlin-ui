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

type grepReq struct {
	root  string
	query string
}

type grepResult struct {
	query  string
	hits   []hostlink.GrepHit
	errMsg string
}

type projectsResult struct {
	projects []hostlink.ProjectInfo
	errMsg   string
}

type imenuReq struct {
	root    string
	docPath string
	query   string
}

type imenuResult struct {
	query  string
	hits   []hostlink.SymbolHit
	errMsg string
}

// EmacsBookmark records a saved file:line anchor (`C-x r m` / `C-x r b`).
type EmacsBookmark struct {
	Name    string
	Path    string
	RelPath string
	Line    int
}

func (f *Frame) initConsultLanes() {
	f.grepLane = async.NewFencedLane[grepReq, grepResult](
		func(ctx context.Context, req grepReq) (grepResult, error) {
			hits, err := f.Link.SearchProjectGrep(ctx, req.root, req.query)
			if err != nil {
				return grepResult{query: req.query, errMsg: err.Error()}, nil
			}
			return grepResult{query: req.query, hits: hits}, nil
		},
		func(_ uint64, res grepResult) {
			f.mu.Lock()
			f.grepLoading = false
			f.grepQuery = res.query
			f.grepErr = res.errMsg
			if res.errMsg == "" {
				f.grepHits = res.hits
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		},
	)
	f.projectsLane = async.NewFencedLane[string, projectsResult](
		func(ctx context.Context, root string) (projectsResult, error) {
			projs, err := f.Link.ListKnownProjects(ctx, root)
			if err != nil {
				return projectsResult{errMsg: err.Error()}, nil
			}
			return projectsResult{projects: projs}, nil
		},
		func(_ uint64, res projectsResult) {
			f.mu.Lock()
			f.projectsLoading = false
			if res.errMsg == "" {
				f.knownProjects = res.projects
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		},
	)
	f.imenuLane = async.NewFencedLane[imenuReq, imenuResult](
		func(ctx context.Context, req imenuReq) (imenuResult, error) {
			syms, err := f.Link.SearchProjectSymbols(ctx, req.root, req.docPath, req.query)
			if err != nil {
				return imenuResult{query: req.query, errMsg: err.Error()}, nil
			}
			return imenuResult{query: req.query, hits: syms}, nil
		},
		func(_ uint64, res imenuResult) {
			f.mu.Lock()
			f.imenuLoading = false
			f.imenuErr = res.errMsg
			if res.errMsg == "" {
				f.imenuHits = res.hits
			}
			f.publishSnapshotLocked()
			f.mu.Unlock()
			f.requestFrame()
		},
	)
}

// OpenProjectGrep opens Doom Emacs `consult-ripgrep` (`SPC /` / `C-c /`),
// executing live queries against ASP's resident Trigram + Tantivy BM25 engine.
func (f *Frame) OpenProjectGrep() {
	f.mu.Lock()
	f.grepLoading = false
	f.grepErr = ""
	f.grepQuery = ""
	f.grepHits = nil
	f.mu.Unlock()
	f.OpenMinibuffer(minibuffer.ModeProjectGrep)
}

// SetProjectGrepQuery submits an incremental live query to the Rust ASP+Tantivy host.
func (f *Frame) SetProjectGrepQuery(query string) {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 2 {
		f.mu.Lock()
		f.grepLoading = false
		f.grepQuery = trimmed
		f.grepHits = nil
		f.grepErr = ""
		f.mu.Unlock()
		return
	}
	root := f.activeWorkspaceProjectRoot()
	f.mu.Lock()
	f.grepLoading = true
	f.grepQuery = trimmed
	f.grepErr = ""
	f.mu.Unlock()
	if f.grepLane != nil {
		f.grepLane.Submit(grepReq{root: root, query: trimmed})
		return
	}
	hits, err := f.Link.SearchProjectGrep(context.Background(), root, trimmed)
	f.mu.Lock()
	f.grepLoading = false
	if err != nil {
		f.grepErr = err.Error()
	} else {
		f.grepHits = hits
	}
	f.mu.Unlock()
}

func (f *Frame) projectGrepCandidates(query string) []minibuffer.Candidate {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 2 {
		return []minibuffer.Candidate{{
			ID:   "grep-hint",
			Name: "Type at least 2 characters (ASP Trigram + Tantivy BM25)...",
			Desc: f.activeWorkspaceProjectRoot(),
		}}
	}
	f.mu.Lock()
	hits := append([]hostlink.GrepHit(nil), f.grepHits...)
	loading := f.grepLoading
	errMsg := f.grepErr
	f.mu.Unlock()

	if errMsg != "" && len(hits) == 0 {
		return []minibuffer.Candidate{{
			ID:   "grep-err",
			Name: "consult-ripgrep unavailable",
			Desc: errMsg,
		}}
	}
	if loading && len(hits) == 0 {
		return []minibuffer.Candidate{{
			ID:   "grep-loading",
			Name: "Searching project via ASP Trigram + Tantivy...",
			Desc: trimmed,
		}}
	}
	all := make([]minibuffer.Candidate, 0, len(hits))
	for i, h := range hits {
		hit := h
		all = append(all, minibuffer.Candidate{
			ID:         fmt.Sprintf("grep-%d-%s:%d", i, hit.RelPath, hit.Line),
			Name:       fmt.Sprintf("%s:%d", hit.RelPath, hit.Line),
			Desc:       hit.Snippet,
			Annotation: fmt.Sprintf("L%d", hit.Line),
			Run:        func() { f.OpenDiskFileAtLine(hit.Path, hit.Line) },
		})
	}
	return all
}

// OpenProjectSwitch opens Doom Emacs `project-switch-project` (`SPC p p` / `C-c p p`)
// listing real Git/jj repositories discovered by the Rust host.
func (f *Frame) OpenProjectSwitch() {
	root := f.activeWorkspaceProjectRoot()
	f.mu.Lock()
	f.projectsLoading = true
	f.mu.Unlock()
	if f.projectsLane != nil {
		f.projectsLane.Submit(root)
	} else if projs, err := f.Link.ListKnownProjects(context.Background(), root); err == nil {
		f.mu.Lock()
		f.projectsLoading = false
		f.knownProjects = projs
		f.mu.Unlock()
	}
	f.OpenMinibuffer(minibuffer.ModeProjectSwitch)
}

// SwitchActiveProjectRoot rebinds the active workspace to targetRoot, immediately
// updating find-file, project-find-file, consult-ripgrep, consult-imenu, and live *magit*.
func (f *Frame) SwitchActiveProjectRoot(targetRoot string) {
	clean := strings.TrimSpace(targetRoot)
	if clean == "" {
		return
	}
	if f.wsMgr != nil {
		f.wsMgr.SetActiveProjectRoot(clean)
	}
	f.mu.Lock()
	f.findFileDir = clean
	f.findFileEntries = nil
	f.findFileCandCache = nil
	f.projFilesRoot = clean
	f.projFilesEntries = nil
	f.projFileCandCache = nil
	f.grepHits = nil
	f.imenuHits = nil
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.RefreshMagitAsync()
	f.requestFrame()
	f.Echo("Switched project -> " + clean)
}

func (f *Frame) projectSwitchCandidates(query string) []minibuffer.Candidate {
	active := f.activeWorkspaceProjectRoot()
	f.mu.Lock()
	projs := append([]hostlink.ProjectInfo(nil), f.knownProjects...)
	loading := f.projectsLoading
	f.mu.Unlock()

	if loading && len(projs) == 0 {
		if strings.TrimSpace(query) != "" {
			return nil
		}
		return []minibuffer.Candidate{{
			ID:   "proj-loading",
			Name: "Discovering Git/jj repositories...",
			Desc: active,
		}}
	}
	all := make([]minibuffer.Candidate, 0, len(projs))
	for _, p := range projs {
		proj := p
		badge := proj.VCS
		if proj.Path == active {
			badge = "★ active · " + badge
		}
		all = append(all, minibuffer.Candidate{
			ID:         "proj-" + proj.Path,
			Name:       proj.Name,
			Desc:       proj.Path,
			Annotation: badge,
			Run:        func() { f.SwitchActiveProjectRoot(proj.Path) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

// OpenConsultLine opens Doom Emacs `consult-line` (`SPC l` / `C-c l`) for instant
// Orderless line search and jump inside the active buffer (*Preview* or *Messages*).
func (f *Frame) OpenConsultLine() {
	f.OpenMinibuffer(minibuffer.ModeConsultLine)
}

func (f *Frame) consultLineCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	fp := f.filePreview
	msgs := append([]string(nil), f.messagesLog...)
	activeBuf := f.activeBuffer
	f.mu.Unlock()

	if activeBuf == buffer.Messages || (fp == nil || strings.TrimSpace(fp.Content) == "") {
		all := make([]minibuffer.Candidate, 0, len(msgs))
		for i, line := range msgs {
			ln := i + 1
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			all = append(all, minibuffer.Candidate{
				ID:         fmt.Sprintf("cline-msg-%d", ln),
				Name:       text,
				Desc:       fmt.Sprintf("*Messages*:%d", ln),
				Annotation: fmt.Sprintf("L%d", ln),
				Run:        func() { f.SelectBuffer(buffer.Messages) },
			})
		}
		return minibuffer.FilterCandidates(all, query)
	}

	rawLines := strings.Split(fp.Content, "\n")
	all := make([]minibuffer.Candidate, 0, len(rawLines))
	for i, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		ln := i + 1
		all = append(all, minibuffer.Candidate{
			ID:         fmt.Sprintf("cline-%d", ln),
			Name:       trimmed,
			Desc:       fmt.Sprintf("%s:%d", fp.Path, ln),
			Annotation: fmt.Sprintf("L%d", ln),
			Run: func() {
				f.mu.Lock()
				if f.filePreview != nil {
					cp := *f.filePreview
					cp.Line = ln
					f.filePreview = &cp
				}
				f.publishSnapshotLocked()
				f.mu.Unlock()
				f.SelectBuffer(buffer.Preview)
				f.Echo(fmt.Sprintf("Jumped to %s:%d", fp.Path, ln))
			},
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

// OpenConsultImenu opens Doom Emacs `consult-imenu` (`SPC i` / `C-c i`) backed
// by ASP's resident AST Symbol Index over HTTP/2 gRPC (`fs:imenu`).
func (f *Frame) OpenConsultImenu() {
	root := f.activeWorkspaceProjectRoot()
	docPath := ""
	f.mu.Lock()
	if f.activeBuffer == buffer.Preview && f.filePreview != nil {
		docPath = f.filePreview.Path
	}
	f.imenuLoading = true
	f.imenuErr = ""
	f.imenuHits = nil
	f.mu.Unlock()
	if f.imenuLane != nil {
		f.imenuLane.Submit(imenuReq{root: root, docPath: docPath, query: ""})
	}
	f.OpenMinibuffer(minibuffer.ModeConsultImenu)
}

// SetConsultImenuQuery queries ASP's AST Symbol Index incrementally as the user types.
func (f *Frame) SetConsultImenuQuery(query string) {
	root := f.activeWorkspaceProjectRoot()
	docPath := ""
	trimmed := strings.TrimSpace(query)
	f.mu.Lock()
	if trimmed == "" && f.activeBuffer == buffer.Preview && f.filePreview != nil {
		docPath = f.filePreview.Path
	}
	f.imenuLoading = true
	f.imenuErr = ""
	f.mu.Unlock()
	if f.imenuLane != nil {
		f.imenuLane.Submit(imenuReq{root: root, docPath: docPath, query: trimmed})
	}
}

func (f *Frame) consultImenuCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	hits := append([]hostlink.SymbolHit(nil), f.imenuHits...)
	loading := f.imenuLoading
	errMsg := f.imenuErr
	f.mu.Unlock()

	if errMsg != "" && len(hits) == 0 {
		return []minibuffer.Candidate{{
			ID:   "imenu-err",
			Name: "consult-imenu unavailable",
			Desc: errMsg,
		}}
	}
	if loading && len(hits) == 0 {
		return []minibuffer.Candidate{{
			ID:   "imenu-loading",
			Name: "Querying ASP AST Symbol Index...",
			Desc: query,
		}}
	}
	all := make([]minibuffer.Candidate, 0, len(hits))
	for i, s := range hits {
		sym := s
		all = append(all, minibuffer.Candidate{
			ID:         fmt.Sprintf("imenu-%d-%s:%d", i, sym.RelPath, sym.Line),
			Name:       sym.Symbol,
			Desc:       fmt.Sprintf("%s:%d", sym.RelPath, sym.Line),
			Annotation: fmt.Sprintf("%s · L%d", sym.Kind, sym.Line),
			Run:        func() { f.OpenDiskFileAtLine(sym.Path, sym.Line) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}

// OpenBookmarkSet opens `bookmark-set` (`C-x r m` / `SPC b M`) to bookmark the
// active file:line location.
func (f *Frame) OpenBookmarkSet() {
	f.OpenMinibuffer(minibuffer.ModeBookmarkSet)
}

// SaveBookmark records or overwrites an Emacs bookmark at the active file:line.
func (f *Frame) SaveBookmark(name string) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return
	}
	f.mu.Lock()
	path := f.activeWorkspaceProjectRoot()
	rel := path
	line := 1
	if f.filePreview != nil && strings.TrimSpace(f.filePreview.Path) != "" {
		rel = f.filePreview.Path
		path = f.filePreview.Path
		if !strings.HasPrefix(path, "/") {
			path = strings.TrimRight(f.activeWorkspaceProjectRoot(), "/") + "/" + path
		}
		if f.filePreview.Line > 0 {
			line = f.filePreview.Line
		}
	}
	next := make([]EmacsBookmark, 0, len(f.bookmarks)+1)
	next = append(next, EmacsBookmark{Name: clean, Path: path, RelPath: rel, Line: line})
	for _, old := range f.bookmarks {
		if old.Name != clean {
			next = append(next, old)
		}
	}
	f.bookmarks = next
	f.publishSnapshotLocked()
	f.mu.Unlock()
	f.Echo(fmt.Sprintf("Bookmark '%s' saved -> %s:%d", clean, rel, line))
}

// OpenBookmarkJump opens `bookmark-jump` (`C-x r b` / `SPC b m` / `SPC RET`).
func (f *Frame) OpenBookmarkJump() {
	f.OpenMinibuffer(minibuffer.ModeBookmarkJump)
}

func (f *Frame) bookmarkJumpCandidates(query string) []minibuffer.Candidate {
	f.mu.Lock()
	bms := append([]EmacsBookmark(nil), f.bookmarks...)
	f.mu.Unlock()
	if len(bms) == 0 {
		return []minibuffer.Candidate{{
			ID:   "bm-empty",
			Name: "No bookmarks yet — press C-x r m / SPC b M to set one",
			Desc: "Emacs bookmark ring is empty",
			Run:  f.OpenBookmarkSet,
		}}
	}
	all := make([]minibuffer.Candidate, 0, len(bms))
	for _, b := range bms {
		bm := b
		all = append(all, minibuffer.Candidate{
			ID:         "bm-" + bm.Name,
			Name:       bm.Name,
			Desc:       fmt.Sprintf("%s:%d", bm.RelPath, bm.Line),
			Annotation: fmt.Sprintf("bookmark · L%d", bm.Line),
			Run:        func() { f.OpenDiskFileAtLine(bm.Path, bm.Line) },
		})
	}
	return minibuffer.FilterCandidates(all, query)
}
