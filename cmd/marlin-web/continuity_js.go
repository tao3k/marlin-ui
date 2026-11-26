// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	"github.com/egoist/mygo"

	"github.com/tao3k/marlin-mygo/engine/composer"
	"github.com/tao3k/marlin-mygo/engine/resilience"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"
)

const (
	webStorageKey = "marlin.ui.recovery.v1"
	webChatScope  = "chat:marlin-web-main"
)

type webContinuity struct {
	saveNow        func()
	onDraftChanged func(scopeID, text string)
	attachWindow   func(win *mygo.Window)
}

func initWebContinuity(adaptive *mygoshell.AdaptiveShell) *webContinuity {
	tr := adaptive.Desktop.Transcript
	res := tr.Resilience
	var winRef *mygo.Window
	invalidate := func() {
		if winRef != nil {
			winRef.Invalidate()
		}
	}

	if ls := js.Global().Get("localStorage"); ls.Truthy() {
		raw := ls.Call("getItem", webStorageKey)
		if raw.Type() == js.TypeString && strings.TrimSpace(raw.String()) != "" {
			if snap, err := resilience.DecodeRecoverySnapshot([]byte(raw.String())); err == nil {
				res.ImportSnapshot(snap)
			}
		}
	}
	tr.RestoreDraftFromResilience(webChatScope)
	if adaptive.Mobile != nil {
		adaptive.Mobile.RestoreDraftsFromResilience()
	}

	saveNow := func() {
		ls := js.Global().Get("localStorage")
		if !ls.Truthy() {
			return
		}
		if data, err := resilience.EncodeRecoverySnapshot(res.ExportSnapshot()); err == nil {
			ls.Call("setItem", webStorageKey, string(data))
		}
	}
	onDraftChanged := func(_, _ string) { saveNow() }
	tr.OnDraftChanged = onDraftChanged
	tr.OnDiscardOutbox = func(_ string) {
		saveNow()
		invalidate()
	}

	flushOutbox := func() {
		res.Outbox.Flush(webChatScope, func(it resilience.OutboxItem) error {
			if tr.OnSubmitTurn != nil {
				tr.OnSubmitTurn(composer.TurnSubmission{Prompt: it.Text})
			} else {
				tr.AppendUser(it.Text)
			}
			return nil
		})
		res.MarkOnline()
		saveNow()
		invalidate()
	}
	tr.OnRetryOutbox = flushOutbox

	return &webContinuity{
		saveNow:        saveNow,
		onDraftChanged: onDraftChanged,
		attachWindow: func(w *mygo.Window) {
			winRef = w
			winJS := js.Global().Get("window")
			if !winJS.Truthy() || !winJS.Get("addEventListener").Truthy() {
				return
			}
			winJS.Call("addEventListener", "offline", js.FuncOf(func(_ js.Value, _ []js.Value) any {
				res.MarkOffline("browser offline")
				saveNow()
				invalidate()
				return nil
			}))
			winJS.Call("addEventListener", "online", js.FuncOf(func(_ js.Value, _ []js.Value) any {
				flushOutbox()
				return nil
			}))
			onHide := js.FuncOf(func(_ js.Value, _ []js.Value) any {
				saveNow()
				return nil
			})
			winJS.Call("addEventListener", "pagehide", onHide)
			winJS.Call("addEventListener", "beforeunload", onHide)
			winJS.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
				if len(args) == 0 {
					return nil
				}
				ev := args[0]
				if !(ev.Get("metaKey").Bool() || ev.Get("ctrlKey").Bool()) {
					return nil
				}
				switch strings.ToLower(ev.Get("key").String()) {
				case "k":
					ev.Call("preventDefault")
					adaptive.Desktop.OpenCommandPalette()
					invalidate()
				case "p":
					ev.Call("preventDefault")
					adaptive.Desktop.OpenHelmSearch()
					invalidate()
				case "f":
					ev.Call("preventDefault")
					tr.OpenSearch()
					invalidate()
				case "g":
					if ev.Get("shiftKey").Bool() {
						ev.Call("preventDefault")
						adaptive.Desktop.OpenGovernance()
						invalidate()
					}
				}
				return nil
			}))
		},
	}
}
