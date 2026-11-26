// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame

import (
	"embed"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-ui/internal/emacs/hydra"
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
	Heads  []declarativeHeadDef
}

type declarativeHeadDef struct {
	Key      string
	KeyCode  ui.Key
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

func loadDefaultKeymaps() map[string]declarativeHydraDef {
	raw, err := defaultTOMLFS.ReadFile("defaults/keymaps.toml")
	if err != nil {
		panic(fmt.Sprintf("frame: missing embedded defaults/keymaps.toml: %v", err))
	}
	km, err := parseKeymapsTOML(string(raw))
	if err != nil {
		panic(fmt.Sprintf("frame: invalid embedded defaults/keymaps.toml: %v", err))
	}
	return km
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
			code, ok := parseUIKey(key)
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
	heads := make([]hydra.Head, 0, len(def.Heads))
	for _, h := range def.Heads {
		act := h.Action
		var run func()
		if act != "" {
			run = func() { f.dispatchWidgetAction(act) }
		}
		heads = append(heads, hydra.Head{
			Key:      h.Key,
			KeyCode:  h.KeyCode,
			Label:    h.Label,
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
		Heads:  heads,
	}
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
