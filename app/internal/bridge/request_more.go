// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/vm"
)

// SettingRowView is a setting a version sets for itself, as typed.
type SettingRowView struct {
	Key string `json:"key"`
	Raw string `json:"raw"`
}

// AvailableSettingView is a setting a request version may set.
type AvailableSettingView struct {
	Key         string   `json:"key"`
	Description string   `json:"description"`
	Default     string   `json:"default"`
	Choices     []string `json:"choices,omitempty"`
	Set         bool     `json:"set"`
}

// RequestSettingsView is the Settings section.
type RequestSettingsView struct {
	Rows      []SettingRowView       `json:"rows"`
	Available []AvailableSettingView `json:"available"`
	Problems  []string               `json:"problems"`
}

// SchemaView is a schema stored for the selected version.
type SchemaView struct {
	Title  string `json:"title"`
	Source string `json:"source"`
	Text   string `json:"text"`
	Kind   string `json:"kind"`   // request, response or message
	Status int    `json:"status"` // for a response
}

// Settings is the version's own settings, and the ones it could set.
func (s *RequestService) Settings(node string) RequestSettingsView {
	v := RequestSettingsView{Rows: []SettingRowView{}, Available: []AvailableSettingView{}, Problems: []string{}}
	s.with(node, func(o *openRequest) {
		for _, r := range o.req.SettingRows() {
			v.Rows = append(v.Rows, SettingRowView{Key: r.Key, Raw: r.Raw})
		}
		for _, a := range o.req.AvailableSettings() {
			v.Available = append(v.Available, AvailableSettingView{Key: a.Key, Description: a.Description, Default: a.Default, Choices: a.Choices, Set: a.Set})
		}
	})
	return v
}

// SetSettings takes the rows as typed. Rows that parse become the version's
// settings; the others come back as problems (and block saving).
func (s *RequestService) SetSettings(node string, rows []SettingRowView) []string {
	problems := []string{}
	s.with(node, func(o *openRequest) {
		in := make([]vm.SettingRow, len(rows))
		for i, r := range rows {
			in[i] = vm.SettingRow{Key: r.Key, Raw: r.Raw}
		}
		problems = append(problems, o.req.SetSettings(in)...)
	})
	return problems
}

// AddSetting adds a setting with its default value.
func (s *RequestService) AddSetting(node, key string) {
	s.with(node, func(o *openRequest) { o.req.AddSetting(key) })
}

// Schemas lists the schemas learned from (or imported for) the version.
func (s *RequestService) Schemas(node string) []SchemaView {
	ch := make(chan *vm.Schemas, 1)
	s.with(node, func(o *openRequest) {
		sc := vm.NewSchemas(s.app.backend, s.app.worker, o.req)
		sc.OnChange(func() {
			select {
			case ch <- sc:
			default:
			}
		})
		sc.Load()
	})
	sc := <-ch
	out := []SchemaView{}
	s.app.actor.Do(func() {
		for _, it := range sc.Items() {
			out = append(out, SchemaView{Title: it.Title, Source: string(it.Source), Text: it.Text, Kind: it.Part.Kind, Status: it.Part.Status})
		}
	})
	return out
}

// DeleteSchema removes a schema; the next send learns it again.
func (s *RequestService) DeleteSchema(node, kind string, status int) {
	part := flowi.SchemaPart{Kind: kind, Status: status}
	s.with(node, func(o *openRequest) { _ = s.app.backend.DeleteSchema(o.req.Target(), o.req.SelectedLabel(), part) })
}
