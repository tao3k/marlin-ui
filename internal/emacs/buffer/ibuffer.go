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
// Tabulated-List) with live Polymode inner-mode badges, winum indicators, and
// 1-click switch / kill actions.
func renderIbuffer(c *ui.Context, th theme.Theme, bc *Context) {
	var rows []IbufferEntry
	if bc != nil {
		rows = bc.IbufferRows
	}
	groups := make(map[string][]IbufferEntry)
	var groupNames []string
	for _, r := range rows {
		grp := r.MajorMode
		if grp == "" {
			grp = "Fundamental"
		}
		if _, ok := groups[grp]; !ok {
			groupNames = append(groupNames, grp)
		}
		groups[grp] = append(groups[grp], r)
	}
	sort.Strings(groupNames)

	ui.Column(c.Key("buf-ibuffer")).Fill().AlignItems(ui.Stretch).Children(func() {
		theme.TabulatedHeader(c, []theme.TableColumn{
			{Label: "BUFFER", Width: 240},
			{Label: "MODE / POLYMODE", Width: 210},
			{Label: "WORKSPACE / WIN", Width: 160},
			{Label: "FLAGS"},
		}, th)
		ui.Scroll(c.Key("ibuffer-scroll")).Grow(1).Padding(6, 12).Children(func() {
			ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
				for _, grp := range groupNames {
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
						ui.Row(c.Key("ibuf-row-"+string(it.BufferID))).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Grow(1).MinWidth(0).Children(func() {
								if theme.TabulatedRow(c, "ibuf-btn-"+string(it.BufferID), []theme.TableColumn{
									{Label: string(it.BufferID), Width: 240, Color: th.Info, Bold: it.Active, Mono: true},
									{Label: modeLbl, Width: 210, Color: th.Accent, Mono: true},
									{Label: wsLbl, Width: 160, Color: th.InkMuted, Mono: true},
									{Label: flags, Color: th.InkMuted, Mono: true},
								}, it.Active, th).Clicked() && bc != nil && bc.OnSelectBuffer != nil {
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
