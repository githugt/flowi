// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/settings"

	"github.com/githugt/flowi/app/internal/palette"
	"github.com/githugt/flowi/app/internal/vm"
)

// SettingView is one setting on the settings page.
type SettingView struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"` // string, bool, int, duration, enum, list
	Enum        []string `json:"enum,omitempty"`
	Value       string   `json:"value"`
	Source      string   `json:"source"`
	UserSet     bool     `json:"userSet"` // the user layer sets it: Reset removes it
	Problem     string   `json:"problem,omitempty"`
}

// SettingGroup is a group of settings, in schema order.
type SettingGroup struct {
	Name   string        `json:"name"`
	Fields []SettingView `json:"fields"`
}

// SettingsView is the settings page.
type SettingsView struct {
	Groups []SettingGroup `json:"groups"`
	// Appearance is what the window itself needs: the effective theme, accent,
	// font and size.
	Appearance Appearance `json:"appearance"`
}

// Appearance is the effective look of the window.
type Appearance struct {
	Theme    string `json:"theme"`
	Accent   string `json:"accent"`
	Font     string `json:"font"`
	FontSize int    `json:"fontSize"`
	// Custom is the palette of the "custom" theme when its file is read; CustomProblem says why it
	// is not (the window then keeps the theme it has).
	Custom        *CustomPalette `json:"custom,omitempty"`
	CustomProblem string         `json:"customProblem,omitempty"`

	paletteFile string
}

// CustomPalette is the palette file of the custom theme, checked.
type CustomPalette struct {
	Base    string            `json:"base"` // dark or light: the theme the colours not named come from
	Vars    map[string]string `json:"vars"` // CSS variable → #rrggbb
	Rainbow []string          `json:"rainbow"`
}

func kindNamesOfSetting() map[settings.Kind]string {
	return map[settings.Kind]string{
		settings.KindString: "string", settings.KindBool: "bool", settings.KindInt: "int",
		settings.KindDuration: "duration", settings.KindEnum: "enum", settings.KindStringList: "list",
	}
}

// SettingsService is the user's settings page.
type SettingsService struct {
	app   *App
	vm    *vm.Settings
	first *firstLoad
}

// newSettingsService builds the service for the settings page.
func newSettingsService(a *App) *SettingsService {
	s := &SettingsService{app: a, vm: vm.NewSettings(a.backend, a.worker), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("settings", s.view())
	})
	return s
}

// State loads the settings and returns them; later changes arrive as the
// "settings" event.
func (s *SettingsService) State() SettingsView {
	var v SettingsView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Set stores a value at the user layer; a value that does not parse is kept
// off disk and shown as the field's problem.
func (s *SettingsService) Set(key, raw string) { s.app.actor.Do(func() { s.vm.Set(key, raw) }) }

// Reset removes the user layer's value of a setting.
func (s *SettingsService) Reset(key string) { s.app.actor.Do(func() { s.vm.Reset(key) }) }

// ResetGroup removes the user layer's value of every setting in a group.
func (s *SettingsService) ResetGroup(group string) { s.app.actor.Do(func() { s.vm.ResetGroup(group) }) }

func (s *SettingsService) view() SettingsView {
	v := SettingsView{Groups: []SettingGroup{}}
	for _, g := range s.vm.Groups() {
		group := SettingGroup{Name: g, Fields: []SettingView{}}
		for _, f := range s.vm.Fields(g) {
			group.Fields = append(group.Fields, SettingView{
				Name: f.Key.Name, Description: f.Key.Description, Kind: kindNamesOfSetting()[f.Key.Kind], Enum: f.Key.Enum,
				Value: f.Value, Source: f.Source, UserSet: f.UserSet, Problem: f.Problem,
			})
			s.fillAppearance(&v.Appearance, f)
		}
		v.Groups = append(v.Groups, group)
	}
	loadCustom(&v.Appearance)
	return v
}

// loadCustom reads the palette file when the custom theme is chosen.
func loadCustom(a *Appearance) {
	if a.Theme != "custom" {
		return
	}
	if a.paletteFile == "" {
		a.CustomProblem = "The custom theme needs a palette file: set appearance.palette_file."
		return
	}
	p, err := palette.Load(a.paletteFile)
	if err != nil {
		a.CustomProblem = err.Error()
		return
	}
	a.Custom = &CustomPalette{Base: p.Base, Vars: p.Vars, Rainbow: nonNil(p.Rainbow)}
}

func (s *SettingsService) fillAppearance(a *Appearance, f vm.SettingField) {
	switch f.Key.Name {
	case "appearance.theme":
		a.Theme = f.Value
	case "appearance.palette_file":
		a.paletteFile = f.Value
	case "appearance.accent":
		a.Accent = f.Value
	case "appearance.font":
		a.Font = f.Value
	case "appearance.font_size":
		for _, c := range f.Value {
			a.FontSize = a.FontSize*10 + int(c-'0')
		}
	}
}
