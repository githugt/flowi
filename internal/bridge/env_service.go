// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// EnvRow is one variable of an environment. For a secret, Value is a new
// value for the vault (empty: keep the stored one); files never hold it.
type EnvRow struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Secret    bool   `json:"secret"`
	SecretSet bool   `json:"secretSet"`
	Override  bool   `json:"override"`
}

// EnvView is an open environment as its screen shows it.
type EnvView struct {
	Node      string   `json:"node"`
	Name      string   `json:"name"`
	Rows      []EnvRow `json:"rows"`
	Colour    string   `json:"colour"`  // "" for the default
	Colours   []string `json:"colours"` // what it can be set to
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

// SetRows replaces the variables.
func (s *EnvironmentService) SetRows(node string, rows []EnvRow) {
	s.with(node, func(e *vm.Environment) {
		out := make([]vm.EnvVarRow, len(rows))
		for i, r := range rows {
			out[i] = vm.EnvVarRow{Name: r.Name, Value: r.Value, Secret: r.Secret, SecretSet: r.SecretSet, Override: r.Override}
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
		rows = append(rows, EnvRow{Name: r.Name, Value: r.Value, Secret: r.Secret, SecretSet: r.SecretSet, Override: r.Override})
	}
	return EnvView{Node: node, Name: e.Name(), Rows: rows, Colour: e.Colour(), Colours: model.EnvColours(), Dirty: e.Dirty(), OnDisk: e.ChangedOnDisk(), NeedsDrop: e.NeedsDropConfirm(), Error: e.Err()}
}
