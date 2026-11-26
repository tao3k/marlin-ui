// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package minibuffer

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func renderHelm(c *ui.Context, s *State, th theme.Theme, helm *mygoshell.HelmMinibufferController, onEcho func(string)) {
	if helm == nil || helm.Engine == nil {
		return
	}
	eng := helm.Engine
	eng.SetViewportRows(9)
	rows, _, _, _, _, _ := eng.VisibleWindow(s.helmRowsBuf[:0])
	s.helmRowsBuf = rows

	ui.Column(c.Key("helm-vertico-list")).Background(th.Bar).Padding(6, 8).Gap(2).
		BorderWidth(1, 0, 0, 0).BorderColor(th.BorderSubtle).AlignItems(ui.Stretch).Children(func() {
		for _, r := range rows {
			if r.IsHeader {
				ui.Row(c.Key("helm-hdr-"+r.SourceID)).Height(24).Padding(0, 12).
					AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%s (%d)", r.SourceLabel, r.SourceCount)).
						Font(theme.FontMono).FontSize(12).FontWeight(700).TextColor(th.Accent)
				})
				continue
			}
			cand := r.Candidate
			if theme.VerticoRow(c, "hrow-"+cand.ID, cand.Title, cand.Subtitle, 250, r.Selected, th).Clicked() {
				if helm.OnExecuteAction != nil {
					helm.OnExecuteAction(search.ActionOpenInFocused, []search.Candidate{cand})
				}
				s.Close()
				if onEcho != nil {
					onEcho("Opened " + cand.Title)
				}
			}
		}
	})
}
