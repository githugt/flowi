// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// EnvRow is one variable of an environment. For a secret, Value is a new
// value for the vault (empty: keep the stored one); files never hold it.
type EnvRow struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Secret    bool   `json:"secret"`
	SecretSet bool   `json:"secretSet"`
	Override  bool   `json:"override"`
	// Kind is how Value is read: "" text, "number", "boolean" or "json" (an object or list).
	Kind string `json:"kind"`
}

// EnvView is an open environment as its screen shows it.
type EnvView struct {
	Node      string   `json:"node"`
	Name      string   `json:"name"`
	Rows      []EnvRow `json:"rows"`
	Colour    string   `json:"colour"` // "" for the default
	Protected bool     `json:"protected"`
	Colours   []string `json:"colours"` // what it can be set to
	Extends   string   `json:"extends"` // the environment this one builds on, or ""
	Bases     []string `json:"bases"`   // what it may build on
	Dirty     bool     `json:"dirty"`
	OnDisk    bool     `json:"onDisk"`
	NeedsDrop bool     `json:"needsDrop"`
	Error     string   `json:"error,omitempty"`
}

// EnvironmentService is the set of environments open in the window.
type EnvironmentService struct {
	app  *App
	envs map[string]*vm.Environment // by tree node id; used on the actor only
}

// Open opens the environment of a tree node, or returns the one already
// open. Every change publishes the "environment" event.
func (s *EnvironmentService) Open(node string) (EnvView, error) {
	var v EnvView
	var err error
	s.app.actor.Do(func() {
		if e, ok := s.envs[node]; ok {
			v = envViewOf(node, e)
			return
		}
		n := s.app.workspace.Node(node)
		if n.Kind != vm.NodeEnvironment {
			err = fmt.Errorf("%q is not an environment", node)
			return
		}
		e := vm.NewEnvironment(s.app.backend, s.app.worker, s.app.workspace.Environment(), n.File)
		e.OnChange(func() { s.app.emit("environment", envViewOf(node, e)) })
		e.OnSaved(s.app.workspace.Load) // nothing watches the environments directory
		s.envs[node] = e
		v = envViewOf(node, e)
	})
	return v, err
}

// Close forgets the open environment.
func (s *EnvironmentService) Close(node string) { s.app.actor.Do(func() { delete(s.envs, node) }) }

func (s *EnvironmentService) with(node string, f func(e *vm.Environment)) {
	s.app.actor.Do(func() {
		if e, ok := s.envs[node]; ok {
			f(e)
		}
	})
}

// SetColour marks the environment with a colour; "" is the default.
func (s *EnvironmentService) SetColour(node, colour string) {
	s.with(node, func(e *vm.Environment) { e.SetColour(colour) })
}

// SetProtected turns on or off the confirmation before a request that changes data is sent with this environment.
func (s *EnvironmentService) SetProtected(node string, on bool) {
	s.with(node, func(e *vm.Environment) { e.SetProtected(on) })
}

// SetExtends chooses the environment this one builds on; "" builds on none.
func (s *EnvironmentService) SetExtends(node, base string) {
	s.with(node, func(e *vm.Environment) { e.SetExtends(base) })
}

// SetRows replaces the variables.
func (s *EnvironmentService) SetRows(node string, rows []EnvRow) {
	s.with(node, func(e *vm.Environment) {
		out := make([]vm.EnvVarRow, len(rows))
		for i, r := range rows {
			out[i] = vm.EnvVarRow{Name: r.Name, Value: r.Value, Secret: r.Secret, SecretSet: r.SecretSet, Override: r.Override, Kind: model.VarKind(r.Kind)}
		}
		e.SetRows(out)
	})
}

// Save writes the file and stores new secret values in the vault.
func (s *EnvironmentService) Save(node string) {
	s.with(node, func(e *vm.Environment) { e.Save(false) })
}

// SaveDroppingComments saves although the file's comments are lost.
func (s *EnvironmentService) SaveDroppingComments(node string) {
	s.with(node, func(e *vm.Environment) { e.ConfirmDrop() })
}

// DeclineDrop gives up a save that would drop comments.
func (s *EnvironmentService) DeclineDrop(node string) {
	s.with(node, func(e *vm.Environment) { e.DeclineDrop() })
}

// Revert drops the edits and shows the file as on disk.
func (s *EnvironmentService) Revert(node string) {
	s.with(node, func(e *vm.Environment) { e.Revert() })
}

func envViewOf(node string, e *vm.Environment) EnvView {
	rows := make([]EnvRow, 0)
	for _, r := range e.Rows() {
		rows = append(rows, EnvRow{Name: r.Name, Value: r.Value, Secret: r.Secret, SecretSet: r.SecretSet, Override: r.Override, Kind: string(r.Kind)})
	}
	return EnvView{Node: node, Name: e.Name(), Rows: rows, Colour: e.Colour(), Protected: e.Protected(), Colours: model.EnvColours(), Extends: e.Extends(), Bases: bases(e), Dirty: e.Dirty(), OnDisk: e.ChangedOnDisk(), NeedsDrop: e.NeedsDropConfirm(), Error: e.Err()}
}

// bases is never nil, so the front end can loop over it.
func bases(e *vm.Environment) []string { return append([]string{}, e.Bases()...) }

// EnvIssueView is one unresolved thing in an environment, with what to do about it.
type EnvIssueView struct {
	Brief string `json:"brief"`
	Hint  string `json:"hint"`
}

// EnvStateView is what the sidebar shows beside an environment row.
type EnvStateView struct {
	File   string         `json:"file"` // the row's file, which keys the rows
	Name   string         `json:"name"`
	Active bool           `json:"active"` // requests are sent with it now
	Issues []EnvIssueView `json:"issues"`
}

// States says which environment is selected and what is unresolved in each (a secret without a value, a base that is
// not there). It reads the vault's names, so call it off the actor; the window asks again when the tree or the secrets
// change.
func (s *EnvironmentService) States() []EnvStateView {
	envs := s.app.backend.Environments()
	infos, err := s.app.backend.Secrets()
	var selected string
	s.app.actor.Do(func() { selected = s.app.workspace.SelectedEnv() })
	out := []EnvStateView{}
	for _, st := range vm.EnvironmentStates(envs, infos, selected, err == nil) {
		v := EnvStateView{File: st.File, Name: st.Name, Active: st.Active, Issues: []EnvIssueView{}}
		for _, i := range st.Issues {
			v.Issues = append(v.Issues, EnvIssueView{Brief: i.Brief, Hint: i.Hint})
		}
		out = append(out, v)
	}
	return out
}
