// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"slices"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// EnvVarRow is one environment variable as edited. For a secret, Value is
// a new value to store in the vault (empty: keep the stored one); it is
// never written to the file.
type EnvVarRow struct {
	Name      string
	Value     string
	Secret    bool
	SecretSet bool // the vault holds a value
	Override  bool // FLOWI_SECRET_<ENV>_<NAME> overrides it
}

// Environment edits one environment file and its secrets (design §10.4).
type Environment struct {
	b        Backend
	w        *Worker
	envs     []model.Environment
	file     string
	disk     model.Environment
	rows     []EnvVarRow
	colour   string
	dirty    bool
	onDisk   bool
	confirm  bool
	err      string
	onChange []func()
	onDrop   []func()
	onSaved  []func()
}

// NewEnvironment returns the editor for the environment in file, one of
// envs (the workspace's environments), and loads its secret states.
func NewEnvironment(b Backend, w *Worker, envs []model.Environment, file string) *Environment {
	e := &Environment{b: b, w: w, envs: envs, file: file}
	e.load(envs)
	e.loadSecrets()
	return e
}

func (e *Environment) load(envs []model.Environment) {
	e.envs = envs
	i := slices.IndexFunc(envs, func(x model.Environment) bool { return x.File == e.file })
	if i < 0 {
		e.err = "this environment file was removed or no longer loads (see Problems)"
		return
	}
	e.disk = envs[i]
	e.colour = e.disk.Colour
	e.rows = make([]EnvVarRow, len(e.disk.Vars))
	for j, v := range e.disk.Vars {
		e.rows[j] = EnvVarRow{Name: v.Name, Value: v.Value, Secret: v.Secret}
	}
	e.dirty, e.onDisk, e.confirm = false, false, false
}

func (e *Environment) loadSecrets() {
	Run(e.w, func() []flowi.SecretInfo {
		infos, err := e.b.Secrets()
		if err != nil {
			return nil // secret states are a hint; the editor still works without them
		}
		return infos
	}, func(infos []flowi.SecretInfo) {
		for i, r := range e.rows {
			j := slices.IndexFunc(infos, func(s flowi.SecretInfo) bool { return s.Environment == e.disk.Name && s.Name == r.Name })
			if j >= 0 {
				e.rows[i].SecretSet, e.rows[i].Override = infos[j].Set, infos[j].Override
			}
		}
		e.changed()
	})
}

// OnChange registers f to run after any state change.
func (e *Environment) OnChange(f func()) { e.onChange = append(e.onChange, f) }

func (e *Environment) changed() {
	for _, f := range e.onChange {
		f()
	}
}

// OnSaved registers f to run after each successful save, so the workspace
// can reload: nothing watches the environments directory.
func (e *Environment) OnSaved(f func()) { e.onSaved = append(e.onSaved, f) }

// OnDropConfirm registers f to run once each time a save stops because
// it would drop the file's comments; answer with ConfirmDrop or
// DeclineDrop.
func (e *Environment) OnDropConfirm(f func()) { e.onDrop = append(e.onDrop, f) }

// ConfirmDrop saves again, dropping the comments.
func (e *Environment) ConfirmDrop() { e.Save(true) }

// DeclineDrop gives up the save that would drop comments.
func (e *Environment) DeclineDrop() {
	e.confirm = false
	e.changed()
}

// File is the environment file.
func (e *Environment) File() string { return e.file }

// Name is the environment's name.
func (e *Environment) Name() string { return e.disk.Name }

// Rows are the variables as edited.
func (e *Environment) Rows() []EnvVarRow { return slices.Clone(e.rows) }

// Dirty reports unsaved edits.
func (e *Environment) Dirty() bool { return e.dirty }

// ChangedOnDisk reports a change on disk while there were edits.
func (e *Environment) ChangedOnDisk() bool { return e.onDisk }

// NeedsDropConfirm reports that saving would drop the file's comments.
func (e *Environment) NeedsDropConfirm() bool { return e.confirm }

// Err is the last problem, or "".
func (e *Environment) Err() string { return e.err }

// Colour is the colour the environment is marked with: one of model.EnvColours, or "" for the default.
func (e *Environment) Colour() string { return e.colour }

// SetColour marks the environment with a colour of model.EnvColours, or "" for the default. It is an
// edit like a variable's: Save writes both. Any other name is ignored: the file would hold a colour
// nothing can draw.
func (e *Environment) SetColour(colour string) {
	if colour != "" && !slices.Contains(model.EnvColours(), colour) {
		return
	}
	e.colour = colour
	e.dirty = true
	e.changed()
}

// SetRows replaces the variables.
func (e *Environment) SetRows(rows []EnvVarRow) {
	e.rows = slices.Clone(rows)
	e.dirty = true
	e.changed()
}

// Save writes the file (secret values blanked), then stores new secret
// values in the vault and removes the vault values of variables that are
// no longer secret.
func (e *Environment) Save(dropComments bool) {
	env := e.disk
	env.Colour = e.colour
	env.Vars = make([]model.EnvVar, 0, len(e.rows))
	for _, r := range e.rows {
		v := model.EnvVar{Name: r.Name, Value: r.Value, Secret: r.Secret}
		if r.Secret {
			v.Value = ""
		}
		env.Vars = append(env.Vars, v)
	}
	rows, wasSecret := slices.Clone(e.rows), e.secretNames()
	Run(e.w, func() error { return e.write(env, rows, wasSecret, dropComments) }, func(err error) { e.saved(env, err) })
}

func (e *Environment) secretNames() map[string]bool {
	out := map[string]bool{}
	for _, v := range e.disk.Vars {
		if v.Secret {
			out[v.Name] = true
		}
	}
	return out
}

// write runs off the UI goroutine: the file first, so new secrets are
// declared before their values are stored.
func (e *Environment) write(env model.Environment, rows []EnvVarRow, wasSecret map[string]bool, dropComments bool) error {
	if err := e.b.SaveEnvironment(env, dropComments); err != nil {
		return err
	}
	var errs []error
	for _, r := range rows {
		if r.Secret && r.Value != "" {
			errs = append(errs, e.b.SetSecret(env.Name, r.Name, r.Value))
		}
	}
	for _, name := range noLongerSecret(wasSecret, rows) {
		errs = append(errs, e.b.DiscardSecret(env.Name, name))
	}
	return errors.Join(errs...)
}

// noLongerSecret names the variables that were secrets on disk and are not now: removed, or made plain.
// Their vault values go once the file no longer declares them.
func noLongerSecret(wasSecret map[string]bool, rows []EnvVarRow) []string {
	var out []string
	for name := range wasSecret {
		if !slices.ContainsFunc(rows, func(r EnvVarRow) bool { return r.Name == name && r.Secret }) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func (e *Environment) saved(env model.Environment, err error) {
	e.confirm, e.err = false, ""
	switch {
	case hasCode(err, diag.CodeWriteDropsComments):
		e.confirm = true
		e.changed()
		for _, f := range e.onDrop {
			f()
		}
		return
	case err != nil:
		// Keep the edits and typed secret values so the user can retry;
		// the file or some vault values may not have been written.
		e.err = ErrorText(err)
		e.changed()
		return
	}
	e.disk = env
	for i := range e.rows {
		if e.rows[i].Secret && e.rows[i].Value != "" {
			e.rows[i].Value, e.rows[i].SecretSet = "", true // values never stay in memory after saving
		}
	}
	e.dirty = false
	e.changed()
	e.loadSecrets()
	for _, f := range e.onSaved {
		f()
	}
}

// Reloaded tells the editor its file was reloaded.
func (e *Environment) Reloaded(envs []model.Environment) {
	if e.dirty {
		e.envs, e.onDisk = envs, true
		e.changed()
		return
	}
	e.load(envs)
	e.changed()
	e.loadSecrets()
}

// Revert drops the edits and shows the file as on disk.
func (e *Environment) Revert() {
	e.dirty = false
	e.load(e.envs)
	e.changed()
	e.loadSecrets()
}
