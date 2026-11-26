// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// FilePreviewState stores the read-only source file or Helm hit displayed
// inside the governed *Preview* Doom Popup Drawer (right side by default,
// promotable into a main split window via Raise).
type FilePreviewState struct {
	Title    string
	Path     string
	Language string
	Line     int
	Content  string
}

func renderFilePreview(c *ui.Context, th theme.Theme, bc *Context) {
	if bc == nil || bc.FilePreview == nil || strings.TrimSpace(bc.FilePreview.Content) == "" {
		ui.Column(c.Key("buf-preview-empty")).Fill().Justify(ui.Center).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "No File Preview Loaded").Font(kit.FontDisplay).FontSize(14).FontWeight(700).TextColor(th.Ink)
			ui.Text(c, "Open any hit from Helm (SPC s) or click an @cite source pill to inspect it here.").
				Font(kit.FontDisplay).FontSize(12.5).TextColor(th.InkMuted)
			if bc != nil && bc.OnOpenHelm != nil {
				if theme.ActionButton(c, "preview-open-helm", "Open Helm (SPC s)", th.Info, true, th).Clicked() {
					bc.OnOpenHelm()
				}
			}
		})
		return
	}

	fp := bc.FilePreview
	title := strings.TrimSpace(fp.Title)
	if title == "" {
		title = previewBaseName(fp.Path)
	}
	if title == "" {
		title = "*Preview*"
	}
	lang := strings.TrimSpace(fp.Language)
	if lang == "" {
		lang = inferPreviewLanguage(fp.Path)
	}
	ref := strings.TrimSpace(fp.Path)
	if ref == "" {
		ref = title
	}
	if fp.Line > 0 && !strings.Contains(ref, ":") {
		ref = fmt.Sprintf("%s:%d", ref, fp.Line)
	}

	ui.Column(c.Key("buf-preview")).Fill().AlignItems(ui.Stretch).Children(func() {
		// Editorial file preview breadcrumb bar: path, language badge, @cite, Copy.
		ui.Row(c.Key("preview-hdr")).Height(36).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.Bar, th, 0.36, 0.60)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			ui.Row(c).Padding(2, 7).Radius(theme.RadiusPill).
				Background(kit.WithAlpha(th.Accent, 0.16)).Children(func() {
				ui.Text(c, strings.ToUpper(lang)).Font(theme.FontMono).FontSize(10).FontWeight(700).TextColor(th.Accent).NoWrap()
			})
			ui.Text(c, ref).Font(theme.FontMono).FontSize(12).FontWeight(600).TextColor(th.Ink).Grow(1).NoWrap().MaxLines(1)
			if bc.OnCiteInComposer != nil {
				if theme.ActionButton(c, "preview-cite-btn", "+ @cite", th.Success, false, th).Clicked() {
					bc.OnCiteInComposer(ref)
				}
			}
			if bc.Adaptive != nil && bc.Adaptive.Desktop != nil && bc.Adaptive.Desktop.Transcript != nil &&
				bc.Adaptive.Desktop.Transcript.OnCopyCode != nil {
				if theme.ActionLink(c, "preview-copy-btn", "Copy", th.InkMuted, th).Clicked() {
					bc.Adaptive.Desktop.Transcript.OnCopyCode(fp.Content)
				}
			}
		})

		lines := strings.Split(fp.Content, "\n")
		if len(lines) > 220 {
			lines = lines[:220]
		}

		ui.Scroll(c.Key("preview-scroll")).Grow(1).Padding(10, 14).Children(func() {
			ui.Column(c).Gap(2).AlignItems(ui.Stretch).Children(func() {
				for idx, line := range lines {
					lineNum := idx + 1
					highlighted := fp.Line > 0 && lineNum == fp.Line
					row := ui.Row(c.Key(fmt.Sprintf("prev-ln-%d", lineNum))).Gap(12).Padding(1, 6).Radius(4).AlignItems(ui.Start)
					if highlighted {
						row.Background(kit.WithAlpha(th.Accent, 0.18))
					}
					row.Children(func() {
						numCol := th.BannerInk
						if highlighted {
							numCol = th.Accent
						}
						ui.Text(c, fmt.Sprintf("%4d", lineNum)).Font(theme.FontMono).FontSize(12).TextColor(numCol).Width(36).NoWrap()
						ui.Text(c, line).Font(theme.FontMono).FontSize(12.5).LineHeight(1.42).TextColor(th.Ink).Grow(1)
					})
				}
			})
		})
	})
}

func previewBaseName(p string) string {
	clean := strings.Split(strings.TrimSpace(p), ":")[0]
	if idx := strings.LastIndexAny(clean, "/\\"); idx >= 0 && idx+1 < len(clean) {
		return clean[idx+1:]
	}
	return clean
}

func inferPreviewLanguage(path string) string {
	base := previewBaseName(path)
	ext := ""
	if dot := strings.LastIndex(base, "."); dot >= 0 && dot+1 < len(base) {
		ext = strings.ToLower(base[dot+1:])
	}
	switch ext {
	case "go":
		return "go"
	case "rs":
		return "rust"
	case "ss", "scm":
		return "scheme"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "toml":
		return "toml"
	case "json":
		return "json"
	case "md", "org":
		return "markdown"
	case "sysml":
		return "sysml"
	case "proto":
		return "proto"
	default:
		return "text"
	}
}
