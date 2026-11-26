// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build js && wasm

// Command marlin-web compiles Marlin UI's AdaptiveShell (Desktop Emacs
// WindowTree + Mobile Phone Companion + 120Hz Helm Search + compiled featurekit)
// into a static WebAssembly bundle (`GOOS=js GOARCH=wasm`).
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/featurekit"
)

func main() {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle:     "Marlin",
		BrandSubtitle:  "WebAssembly · Desktop & Phone Adaptive",
		InitialThemeID: "midnight",
		FormFactor:     mygoshell.FormFactorAuto,
	})
	adaptive.Desktop.ActiveSessionID = "marlin-web-main"
	adaptive.Desktop.Sessions = []mygoshell.SessionSummaryView{
		{SessionID: "marlin-web-main", Title: "Web Session", TurnCount: 0},
	}
	_ = adaptive.Desktop.EnableEmacsWindowKeymap()
	mygoshell.BindDefaultHelmSources(adaptive.Desktop, nil)

	continuity := initWebContinuity(adaptive)
	var winRef *mygo.Window
	invalidate := func() {
		if winRef != nil {
			winRef.Invalidate()
		}
	}
	featurekit.Mount("all", adaptive, continuity.onDraftChanged, invalidate)

	adaptive.Desktop.Transcript.OnSubmitTurn = func(sub composer.TurnSubmission) {
		refs := make([]composer.AttachmentRef, 0, len(sub.Attachments))
		for _, att := range sub.Attachments {
			refs = append(refs, att.Ref)
		}
		adaptive.Desktop.Transcript.AppendUserWithAttachments(sub.Prompt, refs)
		invalidate()
	}

	mygo.App.WhenReady(func() {
		winRef = mygo.NewWindow(mygo.WindowOptions{
			Title:   "Marlin UI — Web",
			Width:   1280,
			Height:  820,
			Content: ui.View(adaptive.View),
		})
		continuity.attachWindow(winRef)
		winRef.Invalidate()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
