// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/settings"
)

// SettingField is one user setting on the settings page.
type SettingField struct {
	Key     settings.Key
	Group   string // first segment of the name; "general" for plain names
	Value   string // effective value as text (lists comma-separated)
	Source  string // "default", "user (settings.yaml)", "workspace (workspace.yaml)", …
	UserSet bool   // the user layer sets it (Reset removes it)
	Problem string // the last Set was refused
}

// Settings is the settings page (design §8.1): every setting the user
// layer may set, generated from the schema, edited at the user layer.
type Settings struct {
	b            Backend
	w            *Worker
	fields       []SettingField
	notes        map[string]string // advice shown under a field that is not a refused value
	onChange     []func()
	onAppearance []func()
}

// NewSettings returns the page's view-model; call Load.
func NewSettings(b Backend, w *Worker) *Settings {
	return &Settings{b: b, w: w, notes: map[string]string{}}
}

// OnChange registers f to run after values change.
func (s *Settings) OnChange(f func()) { s.onChange = append(s.onChange, f) }

// OnAppearanceChanged registers f to run after an appearance.* or ui.*
// setting changes, so the theme and the screens' preferences are re-read.
func (s *Settings) OnAppearanceChanged(f func()) { s.onAppearance = append(s.onAppearance, f) }

// Load reads every setting in the background.
func (s *Settings) Load() {
	Run(s.w, func() []SettingField { return settingFields(s.b.Settings("")) }, func(fields []SettingField) {
		for i := range fields {
			fields[i].Problem = s.problemOf(fields[i].Key.Name)
			if fields[i].Problem == "" {
				fields[i].Problem = s.notes[fields[i].Key.Name]
			}
		}
		s.fields = fields
		s.changed()
	})
}

func (s *Settings) problemOf(key string) string {
	i := slices.IndexFunc(s.fields, func(f SettingField) bool { return f.Key.Name == key })
	if i < 0 {
		return ""
	}
	return s.fields[i].Problem
}

func (s *Settings) changed() {
	for _, f := range s.onChange {
		f()
	}
}

func settingFields(r settings.Resolved) []SettingField {
	var out []SettingField
	for _, k := range settings.Schema() {
		if !slices.Contains(k.FileLayers, settings.LayerUser) {
			continue
		}
		v, _ := r.Get(k.Name)
		f := SettingField{Key: k, Group: groupOf(k.Name), Value: valueText(v.Value), Source: v.Layer.String(), UserSet: v.Layer == settings.LayerUser}
		if v.Source != "" {
			f.Source += " (" + filepath.Base(v.Source) + ")"
		}
		out = append(out, f)
	}
	return out
}

// urlColourGroup is the settings group of the URL bar colours.
const urlColourGroup = "appearance.url"

func groupOf(name string) string {
	if strings.HasPrefix(name, urlColourGroup+".") {
		return urlColourGroup
	}
	if group, _, ok := strings.Cut(name, "."); ok {
		return group
	}
	return "general"
}

// valueText shows a value the way it is typed: lists comma-separated.
func valueText(v any) string {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ", ")
	case []any:
		parts := make([]string, len(x))
		for i, p := range x {
			parts[i] = fmt.Sprint(p)
		}
		return strings.Join(parts, ", ")
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// Groups lists the groups in schema order.
func (s *Settings) Groups() []string {
	var out []string
	for _, f := range s.fields {
		if !slices.Contains(out, f.Group) {
			out = append(out, f.Group)
		}
	}
	return out
}

// Fields lists a group's settings.
func (s *Settings) Fields(group string) []SettingField {
	var out []SettingField
	for _, f := range s.fields {
		if f.Group == group {
			out = append(out, f)
		}
	}
	return out
}

// Set stores raw (as typed; lists comma-separated) at the user layer. A
// value that does not parse is kept off disk and shown as the field's
// problem.
func (s *Settings) Set(key, raw string) {
	if isColourKey(key) {
		raw = strings.TrimSpace(raw)
	}
	if err := validateColour(key, raw); err != nil {
		s.done(key, err)
		return
	}
	Run(s.w, func() error { return s.b.SetUserSetting(key, raw) }, func(err error) { s.done(key, err) })
}

// SetNote shows advice under a setting (for example why a palette file
// cannot be used); "" clears it. It does not change the value.
func (s *Settings) SetNote(key, text string) {
	if text == "" {
		delete(s.notes, key)
	} else {
		s.notes[key] = text
	}
	if i := slices.IndexFunc(s.fields, func(f SettingField) bool { return f.Key.Name == key }); i >= 0 {
		s.fields[i].Problem = text
		s.changed()
	}
}

// ResetGroup removes the user layer's value of every setting in group.
func (s *Settings) ResetGroup(group string) {
	var keys []string
	for _, f := range s.Fields(group) {
		if f.UserSet {
			keys = append(keys, f.Key.Name)
		}
	}
	type outcome struct {
		key string
		err error
	}
	Run(s.w, func() outcome {
		for _, k := range keys {
			if err := s.b.ResetUserSetting(k); err != nil {
				return outcome{k, err} // the problem is shown on the field that failed
			}
		}
		return outcome{key: group}
	}, func(o outcome) { s.done(o.key, o.err) })
}

// Reset removes the user layer's value.
func (s *Settings) Reset(key string) {
	Run(s.w, func() error { return s.b.ResetUserSetting(key) }, func(err error) { s.done(key, err) })
}

func (s *Settings) done(key string, err error) {
	problem := ""
	if err != nil {
		problem = ErrorText(err)
	}
	if i := slices.IndexFunc(s.fields, func(f SettingField) bool { return f.Key.Name == key }); i >= 0 {
		s.fields[i].Problem = problem
	}
	s.changed()
	if err == nil && (strings.HasPrefix(key, "appearance") || strings.HasPrefix(key, "ui.")) {
		for _, f := range s.onAppearance {
			f()
		}
	}
	s.Load()
}

// colourKeys are the settings holding colours (one, or a list for the pair
// cycle).
func isColourKey(key string) bool {
	switch key {
	case "appearance.accent", "appearance.url.pair_color", "appearance.url.pair_colors":
		return true
	}
	return strings.HasPrefix(key, urlColourGroup+".") && !strings.HasSuffix(key, ".pairs")
}

// validateColour refuses a colour setting that is not #rrggbb ("" is the
// default; the pair cycle is a comma-separated list of them).
func validateColour(key, raw string) error {
	if !isColourKey(key) {
		return nil
	}
	parts := strings.Split(raw, ",")
	if key != "appearance.url.pair_colors" && len(parts) > 1 {
		return fmt.Errorf("this setting takes one colour, not a list")
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) != 7 || part[0] != '#' || strings.Trim(part[1:], "0123456789abcdefABCDEF") != "" {
			return fmt.Errorf("%q is not a colour: use #rrggbb (for example #89b4fa)", part)
		}
	}
	return nil
}
