// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"embed"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/pkg/schema"

	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
	"github.com/tao3k/marlin-ui/internal/uiframework"
)

//go:embed defaults/*.toml
var defaultTOMLFS embed.FS

// declarativeCommand is one M-x entry loaded from commands.toml.
type declarativeCommand struct {
	Name   string
	Desc   string
	Action string
}

// declarativeHydraDef is one SPC Hydra body loaded from keymaps.toml.
type declarativeHydraDef struct {
	ID     string
	Prefix string
	Title  string
	Hint   string
	Heads  []declarativeHeadDef
}

type declarativeHeadDef struct {
	Key      string
	KeyCode  ui.Key
	KeyMods  ui.Modifiers
	Label    string
	Column   string
	Exit     bool
	IsPrefix bool
	Action   string
}

func loadDefaultCommands() []declarativeCommand {
	raw, err := defaultTOMLFS.ReadFile("defaults/commands.toml")
	if err != nil {
		panic(fmt.Sprintf("frame: missing embedded defaults/commands.toml: %v", err))
	}
	cmds, err := parseCommandsTOML(string(raw))
	if err != nil {
		panic(fmt.Sprintf("frame: invalid embedded defaults/commands.toml: %v", err))
	}
	return cmds
}

func loadDesktopTOMLKeymaps() map[string]declarativeHydraDef {
	raw, err := defaultTOMLFS.ReadFile("defaults/keymaps.toml")
	if err != nil {
		panic(fmt.Sprintf("frame: missing embedded defaults/keymaps.toml: %v", err))
	}
	overlay, err := parseKeymapsTOML(string(raw))
	if err != nil {
		panic(fmt.Sprintf("frame: invalid embedded defaults/keymaps.toml: %v", err))
	}
	return overlay
}

func loadDefaultKeymaps() map[string]declarativeHydraDef {
	return mergeHydraOverlay(loadStandaloneBootstrapKeymaps(), loadDesktopTOMLKeymaps())
}

// projectLiveSchemeKeymaps converts live `marlin.ui.v1` Scheme keybindings and
// Hydra transients into declarativeHydraDef entries ready to layer beneath
// Desktop-special and user TOML overlays.
func projectLiveSchemeKeymaps(
	keymap []schema.UIKeyBindingSpec,
	transients []schema.UITransient,
) map[string]declarativeHydraDef {
	return projectMappedHydras(uiframework.ProjectKeymapAndTransients(keymap, transients))
}

func projectMappedHydras(mapped map[string]uiframework.MappedHydra) map[string]declarativeHydraDef {
	out := make(map[string]declarativeHydraDef, len(mapped))
	for pref, mh := range mapped {
		def := declarativeHydraDef{
			ID:     mh.ID,
			Prefix: mh.Prefix,
			Title:  mh.Title,
			Hint:   mh.Hint,
			Heads:  make([]declarativeHeadDef, 0, len(mh.Heads)),
		}
		for _, hd := range mh.Heads {
			code, mods, ok := parseUIKeyWithMods(hd.Key)
			if !ok {
				continue
			}
			def.Heads = append(def.Heads, declarativeHeadDef{
				Key:      hd.Key,
				KeyCode:  code,
				KeyMods:  mods,
				Label:    hd.Label,
				Column:   hd.Column,
				Exit:     hd.Exit,
				IsPrefix: hd.IsPrefix,
				Action:   hd.Action,
			})
		}
		if len(def.Heads) > 0 {
			out[pref] = def
		}
	}
	return out
}

// mergeHydraOverlay layers TOML desktop extensions or downstream user overrides
// onto the authoritative Scheme Doom keymap base: matching (prefix, key) heads
// override in-place while new heads extend the prefix cleanly.
func mergeHydraOverlay(base, overlay map[string]declarativeHydraDef) map[string]declarativeHydraDef {
	out := make(map[string]declarativeHydraDef, len(base)+len(overlay))
	for pref, def := range base {
		cp := def
		cp.Heads = append([]declarativeHeadDef(nil), def.Heads...)
		out[pref] = cp
	}
	for pref, over := range overlay {
		cur, exists := out[pref]
		if !exists {
			cp := over
			cp.Heads = append([]declarativeHeadDef(nil), over.Heads...)
			out[pref] = cp
			continue
		}
		if over.ID != "" && over.ID != pref {
			cur.ID = over.ID
		}
		if over.Title != "" && over.Title != pref {
			cur.Title = over.Title
		}
		for _, oh := range over.Heads {
			replaced := false
			for i := range cur.Heads {
				if cur.Heads[i].Key == oh.Key {
					cur.Heads[i] = oh
					replaced = true
					break
				}
			}
			if !replaced {
				cur.Heads = append(cur.Heads, oh)
			}
		}
		out[pref] = cur
	}
	return out
}

func parseCommandsTOML(raw string) ([]declarativeCommand, error) {
	sections, err := parseTOMLArraySections(raw)
	if err != nil {
		return nil, err
	}
	var out []declarativeCommand
	for _, sec := range sections {
		if sec.name != "commands" {
			continue
		}
		name := strings.TrimSpace(sec.kv["name"])
		if name == "" {
			return nil, fmt.Errorf("commands.toml: [[commands]] entry missing name")
		}
		out = append(out, declarativeCommand{
			Name:   name,
			Desc:   strings.TrimSpace(sec.kv["desc"]),
			Action: strings.TrimSpace(sec.kv["action"]),
		})
	}
	return out, nil
}

func parseKeymapsTOML(raw string) (map[string]declarativeHydraDef, error) {
	sections, err := parseTOMLArraySections(raw)
	if err != nil {
		return nil, err
	}
	out := make(map[string]declarativeHydraDef)
	for _, sec := range sections {
		switch sec.name {
		case "hydras":
			pref := strings.TrimSpace(sec.kv["prefix"])
			if pref == "" {
				return nil, fmt.Errorf("keymaps.toml: [[hydras]] missing prefix")
			}
			def := out[pref]
			def.Prefix = pref
			def.ID = strings.TrimSpace(sec.kv["id"])
			if def.ID == "" {
				def.ID = pref
			}
			def.Title = strings.TrimSpace(sec.kv["title"])
			out[pref] = def
		case "heads":
			pref := strings.TrimSpace(sec.kv["prefix"])
			key := strings.TrimSpace(sec.kv["key"])
			if pref == "" || key == "" {
				return nil, fmt.Errorf("keymaps.toml: [[heads]] requires prefix and key")
			}
			code, mods, ok := parseUIKeyWithMods(key)
			if !ok {
				return nil, fmt.Errorf("keymaps.toml: unsupported key %q", key)
			}
			def := out[pref]
			if def.Prefix == "" {
				def.Prefix = pref
				def.ID = pref
				def.Title = pref
			}
			def.Heads = append(def.Heads, declarativeHeadDef{
				Key:      key,
				KeyCode:  code,
				KeyMods:  mods,
				Label:    strings.TrimSpace(sec.kv["label"]),
				Column:   strings.TrimSpace(sec.kv["column"]),
				Exit:     strings.EqualFold(sec.kv["exit"], "true"),
				IsPrefix: strings.EqualFold(sec.kv["is_prefix"], "true"),
				Action:   strings.TrimSpace(sec.kv["action"]),
			})
			out[pref] = def
		}
	}
	return out, nil
}

func (f *Frame) materializeHydra(def declarativeHydraDef) hydra.Body {
	f.mu.Lock()
	allHydras := f.tomlHydras
	f.mu.Unlock()

	heads := make([]hydra.Head, 0, len(def.Heads))
	for _, h := range def.Heads {
		act := h.Action
		var run func()
		if act != "" {
			run = func() { f.dispatchWidgetAction(act) }
		}
		alt := crossHintForAction(allHydras, def.Prefix, act)
		heads = append(heads, hydra.Head{
			Key:      h.Key,
			KeyCode:  h.KeyCode,
			KeyMods:  h.KeyMods,
			Label:    h.Label,
			AltHint:  alt,
			Column:   h.Column,
			Exit:     h.Exit,
			IsPrefix: h.IsPrefix,
			Run:      run,
		})
	}
	return hydra.Body{
		ID:     def.ID,
		Prefix: def.Prefix,
		Title:  def.Title,
		Hint:   def.Hint,
		Heads:  heads,
	}
}

// crossHintForAction derives the paired shortcut from the opposite keymap family
// (native Emacs C-x/C-c/C-h when viewing a Doom SPC menu, or Doom SPC when
// viewing a native Emacs menu) directly from the live effective keymaps.
func crossHintForAction(all map[string]declarativeHydraDef, currentPrefix, action string) string {
	if action == "" || strings.HasPrefix(action, "hydra:") {
		return ""
	}
	wantNative := strings.HasPrefix(currentPrefix, "SPC")
	wantDoom := strings.HasPrefix(currentPrefix, "C-")
	if !wantNative && !wantDoom {
		return ""
	}
	// Deterministic check order for common prefixes.
	prefOrder := []string{"C-x", "C-x t", "C-x w", "C-c", "C-h", "SPC", "SPC w", "SPC b", "SPC TAB", "SPC t", "SPC m", "SPC q"}
	seen := make(map[string]bool, len(prefOrder))
	check := func(pref string, def declarativeHydraDef) string {
		if pref == currentPrefix {
			return ""
		}
		if wantNative && !strings.HasPrefix(pref, "C-") {
			return ""
		}
		if wantDoom && !strings.HasPrefix(pref, "SPC") {
			return ""
		}
		for _, hd := range def.Heads {
			if hd.Action == action {
				return pref + " " + hd.Key
			}
		}
		return ""
	}
	for _, pref := range prefOrder {
		seen[pref] = true
		if def, ok := all[pref]; ok {
			if hit := check(pref, def); hit != "" {
				return hit
			}
		}
	}
	for pref, def := range all {
		if seen[pref] {
			continue
		}
		if hit := check(pref, def); hit != "" {
			return hit
		}
	}
	// Direct global Emacs shortcuts not living under a prefix hydra.
	if wantNative {
		switch action {
		case "cmd:mx":
			return "M-x"
		case "cmd:helm":
			return "C-s"
		default:
			if strings.HasPrefix(action, "cmd:win-") && len(action) == len("cmd:win-1") {
				d := action[len(action)-1]
				if d >= '1' && d <= '9' {
					return "M-" + string(d)
				}
			}
		}
	}
	return ""
}

type tomlArraySection struct {
	name string
	kv   map[string]string
}

func parseTOMLArraySections(raw string) ([]tomlArraySection, error) {
	var sections []tomlArraySection
	for idx, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]") {
			name := strings.ToLower(strings.TrimSpace(trimmed[2 : len(trimmed)-2]))
			sections = append(sections, tomlArraySection{name: name, kv: make(map[string]string)})
			continue
		}
		if len(sections) == 0 {
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = \"value\"", idx+1)
		}
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if strings.HasPrefix(val, `"`) {
			end := strings.Index(val[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("line %d: unclosed string quote", idx+1)
			}
			val = val[1 : 1+end]
		} else if c := strings.IndexByte(val, '#'); c >= 0 {
			val = strings.TrimSpace(val[:c])
		}
		sections[len(sections)-1].kv[key] = val
	}
	return sections, nil
}

func parseUIKeyWithMods(raw string) (ui.Key, ui.Modifiers, bool) {
	trimmed := strings.TrimSpace(raw)
	var extraMods ui.Modifiers
	for {
		if strings.HasPrefix(trimmed, "C-") && len(trimmed) > 2 {
			extraMods |= ui.Ctrl
			trimmed = trimmed[2:]
			continue
		}
		if strings.HasPrefix(trimmed, "M-") && len(trimmed) > 2 {
			extraMods |= ui.Alt
			trimmed = trimmed[2:]
			continue
		}
		break
	}
	switch trimmed {
	case "~":
		return ui.KeyBackquote, extraMods | ui.Shift, true
	case "`":
		return ui.KeyBackquote, extraMods, true
	case "[":
		return ui.KeyBracketLeft, extraMods, true
	case "]":
		return ui.KeyBracketRight, extraMods, true
	case "=":
		return ui.KeyEqual, extraMods, true
	case "+":
		return ui.KeyEqual, extraMods | ui.Shift, true
	case "-":
		return ui.KeyMinus, extraMods, true
	case ".":
		return ui.KeyPeriod, extraMods, true
	case "/":
		return ui.KeySlash, extraMods, true
	case "?":
		return ui.KeySlash, extraMods | ui.Shift, true
	}
	lower := strings.ToLower(trimmed)
	if lower == "tab" {
		return ui.KeyTab, extraMods, true
	}
	mods := extraMods
	if len(trimmed) == 1 && trimmed[0] >= 'A' && trimmed[0] <= 'Z' {
		mods |= ui.Shift
	}
	code, ok := parseUIKey(lower)
	return code, mods, ok
}

func parseUIKey(k string) (ui.Key, bool) {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "spc", "space":
		return ui.KeySpace, true
	case ",":
		return ui.KeyComma, true
	case "0":
		return ui.Key0, true
	case "1":
		return ui.Key1, true
	case "2":
		return ui.Key2, true
	case "3":
		return ui.Key3, true
	case "4":
		return ui.Key4, true
	case "5":
		return ui.Key5, true
	case "6":
		return ui.Key6, true
	case "7":
		return ui.Key7, true
	case "8":
		return ui.Key8, true
	case "9":
		return ui.Key9, true
	case "a":
		return ui.KeyA, true
	case "b":
		return ui.KeyB, true
	case "c":
		return ui.KeyC, true
	case "d":
		return ui.KeyD, true
	case "e":
		return ui.KeyE, true
	case "f":
		return ui.KeyF, true
	case "g":
		return ui.KeyG, true
	case "h":
		return ui.KeyH, true
	case "i":
		return ui.KeyI, true
	case "j":
		return ui.KeyJ, true
	case "k":
		return ui.KeyK, true
	case "l":
		return ui.KeyL, true
	case "m":
		return ui.KeyM, true
	case "n":
		return ui.KeyN, true
	case "o":
		return ui.KeyO, true
	case "p":
		return ui.KeyP, true
	case "q":
		return ui.KeyQ, true
	case "r":
		return ui.KeyR, true
	case "s":
		return ui.KeyS, true
	case "t":
		return ui.KeyT, true
	case "u":
		return ui.KeyU, true
	case "v":
		return ui.KeyV, true
	case "w":
		return ui.KeyW, true
	case "x":
		return ui.KeyX, true
	case "y":
		return ui.KeyY, true
	case "z":
		return ui.KeyZ, true
	default:
		return 0, false
	}
}
