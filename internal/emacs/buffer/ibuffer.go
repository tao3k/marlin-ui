// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package buffer

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/runtime/kit"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

// renderIbuffer paints an authentic Emacs `*Ibuffer*` manager grouping all open
// buffers by their Major Mode (e.g. Marlin-Chat, Magit-Status, Bento-Dashboard,
// Tabulated-List) with live Polymode inner-mode badges, winum indicators,
// Major-Mode filter pills, per-row [D] delete marks, and batch kill execution.
func renderIbuffer(c *ui.Context, th theme.Theme, bc *Context) {
	var rows []IbufferEntry
	activeFilter := ""
	markedCount := 0
	if bc != nil {
		rows = bc.IbufferRows
		activeFilter = bc.IbufferFilter
		markedCount = bc.IbufferMarkedCount
	}
	groups := make(map[string][]IbufferEntry)
	var allGroupNames []string
	for _, r := range rows {
		grp := r.MajorMode
		if grp == "" {
			grp = "Fundamental"
		}
		if _, ok := groups[grp]; !ok {
			allGroupNames = append(allGroupNames, grp)
		}
		groups[grp] = append(groups[grp], r)
	}
	sort.Strings(allGroupNames)

	ui.Column(c.Key("buf-ibuffer")).Fill().AlignItems(ui.Stretch).Children(func() {
		// Ibuffer toolbar: Major-Mode filter pills + batch mark actions + New Chat.
		ui.Row(c.Key("ibuffer-toolbar")).Padding(6, 12).Gap(6).AlignItems(ui.Center).
			Background(theme.GlassTone(c, th.Bar, th, 0.36, 0.62)).
			BorderWidth(0, 0, 1, 0).BorderColor(th.BorderSubtle).Children(func() {
			if theme.ActionButton(c, "ibuf-flt-all", fmt.Sprintf("All (%d)", len(rows)), th.Info, activeFilter == "", th).Clicked() &&
				bc != nil && bc.OnFilterIbuffer != nil {
				bc.OnFilterIbuffer("")
			}
			for _, grp := range allGroupNames {
				g := grp
				lbl := fmt.Sprintf("%s (%d)", g, len(groups[g]))
				if theme.ActionButton(c, "ibuf-flt-"+g, lbl, th.Accent, activeFilter == g, th).Clicked() &&
					bc != nil && bc.OnFilterIbuffer != nil {
					bc.OnFilterIbuffer(g)
				}
			}
			ui.Box(c).Grow(1)
			if markedCount > 0 && bc != nil {
				if bc.OnUnmarkAllIbuffer != nil &&
					theme.ActionButton(c, "ibuf-unmark-all", "Unmark (U)", th.InkMuted, false, th).Clicked() {
					bc.OnUnmarkAllIbuffer()
				}
				if bc.OnKillMarkedIbuffer != nil &&
					theme.ActionButton(c, "ibuf-exec-kill", fmt.Sprintf("Kill Marked (%d) [x]", markedCount), th.Danger, true, th).Clicked() {
					bc.OnKillMarkedIbuffer()
				}
			}
			if bc != nil && bc.OnNewChatBuffer != nil &&
				theme.ActionButton(c, "ibuf-new-chat", "+ New Chat", th.Success, false, th).Clicked() {
				bc.OnNewChatBuffer()
			}
		})

		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "M", Width: 34},
			{Label: "BUFFER", Width: 230},
			{Label: "MODE / POLYMODE", Width: 200},
			{Label: "WORKSPACE / WIN", Width: 150},
			{Label: "FLAGS"},
		}, th)
		ui.Scroll(c.Key("ibuffer-scroll")).Grow(1).Padding(6, 12).Children(func() {
			ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
				for _, grp := range allGroupNames {
					if activeFilter != "" && activeFilter != grp {
						continue
					}
					items := groups[grp]
					ui.Row(c.Key("ibuf-grp-"+grp)).Padding(4, 6).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("[ %s ] (%d)", grp, len(items))).
							Font(kit.FontDisplay).FontSize(12).FontWeight(700).TextColor(th.Accent).NoWrap()
					})
					for _, item := range items {
						it := item
						modeLbl := it.MajorMode
						if it.InnerMode != "" {
							modeLbl += "[" + it.InnerMode + "]"
						}
						wsLbl := it.Workspace
						if it.WindowNum > 0 {
							wsLbl += fmt.Sprintf(" · win #%d", it.WindowNum)
						}
						flags := "rw"
						if it.ReadOnly {
							flags = "ro"
						}
						if it.Active {
							flags += " · active"
						}
						markLbl := "·"
						markCol := th.InkMuted
						if it.Marked {
							markLbl = "D"
							markCol = th.Danger
						}
						ui.Row(c.Key("ibuf-row-"+string(it.BufferID))).Gap(6).AlignItems(ui.Center).Children(func() {
							if bc != nil && bc.OnToggleIbufferMark != nil && len(rows) > 1 {
								if theme.ActionButton(c, "ibuf-mark-"+string(it.BufferID), markLbl, markCol, it.Marked, th).Clicked() {
									bc.OnToggleIbufferMark(it.BufferID)
								}
							}
							ui.Box(c).Grow(1).MinWidth(0).Children(func() {
								if theme.TabulatedRow(c, "ibuf-btn-"+string(it.BufferID), []theme.TableColumn{
									{Label: string(it.BufferID), Width: 230, Color: th.Info, Bold: it.Active, Mono: true},
									{Label: modeLbl, Width: 200, Color: th.Accent, Mono: true},
									{Label: wsLbl, Width: 150, Color: th.InkMuted, Mono: true},
									{Label: flags, Color: th.InkMuted, Mono: true},
								}, it.Active || it.Marked, th).Clicked() && bc != nil && bc.OnSelectBuffer != nil {
									bc.OnSelectBuffer(it.BufferID)
								}
							})
							if bc != nil && bc.OnKillBuffer != nil && len(rows) > 1 {
								if theme.ActionLink(c, "ibuf-kill-"+string(it.BufferID), "Kill", th.Danger, th).Clicked() {
									bc.OnKillBuffer(it.BufferID)
								}
							}
						})
					}
				}
			})
		})
	})
}
