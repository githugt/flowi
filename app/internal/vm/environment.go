// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
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
	// Kind is how Value is read: text (""), number, boolean or json (an object or list as JSON text).
	Kind model.VarKind
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
	extends  string
	protect  bool
	dirty    bool
	onDisk   bool
	confirm  bool
	err      string
	stored   map[string]bool // the secrets the vault holds a value for, by name
	losing   []string        // the stored values the last Save refused to remove until it is asked again
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
	e.extends = e.disk.Extends
	e.protect = e.disk.Protected
	e.rows = make([]EnvVarRow, len(e.disk.Vars))
	for j, v := range e.disk.Vars {
		e.rows[j] = EnvVarRow{Name: v.Name, Value: v.Value, Secret: v.Secret, Kind: v.Kind}
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
		stored := map[string]bool{}
		for _, info := range infos {
			if info.Environment == e.disk.Name && info.Set {
				stored[info.Name] = true
			}
		}
		if !maps.Equal(stored, e.stored) { // a write only when something changed: two answers in flight must not race
			e.stored = stored
		}
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

// Protected says the apps confirm before a request that changes data is sent with this environment.
func (e *Environment) Protected() bool { return e.protect }

// SetProtected turns the confirmation on or off. It is an edit like a variable's: Save writes it.
func (e *Environment) SetProtected(on bool) {
	e.protect = on
	e.dirty = true
	e.changed()
}

// Extends is the name of the environment this one builds on, or "".
func (e *Environment) Extends() string { return e.extends }

// SetExtends chooses the environment this one builds on; "" builds on none. A name that is not in Bases is
// ignored, because the file would hold a base that does not exist or that loops back to this one.
func (e *Environment) SetExtends(name string) {
	if name != "" && !slices.Contains(e.Bases(), name) {
		return
	}
	e.extends = name
	e.dirty = true
	e.changed()
}

// Bases lists the environments this one may build on, sorted: every other environment except those that
// already extend this one, directly or through others.
func (e *Environment) Bases() []string {
	self := e.Name()
	var out []string
	for _, c := range e.envs {
		if c.Name == self || c.Name == "" || extendsEnvironment(e.envs, c.Name, self) {
			continue
		}
		out = append(out, c.Name)
	}
	slices.Sort(out)
	return out
}

// extendsEnvironment says whether the environment called name builds on base somewhere along its chain.
func extendsEnvironment(envs []model.Environment, name, base string) bool {
	chain, err := model.EnvironmentChain(envs, name)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(chain[:len(chain)-1], func(e model.Environment) bool { return e.Name == base })
}

// SetRows replaces the variables.
func (e *Environment) SetRows(rows []EnvVarRow) {
	e.rows = slices.Clone(rows)
	e.losing = nil // the confirmation was for the earlier rows
	e.dirty = true
	e.changed()
}

// Save writes the file (secret values blanked), then stores new secret
// values in the vault and removes the vault values of variables that are
// no longer secret.
func (e *Environment) Save(dropComments bool) {
	env := e.disk
	// The groups are not edited here: they follow the file, so a move made on the Manage environments screen while this
	// editor holds unsaved edits is not undone by saving them.
	if i := slices.IndexFunc(e.envs, func(x model.Environment) bool { return x.File == e.file }); i >= 0 {
		env.Groups = e.envs[i].Groups
	}
	env.Colour = e.colour
	env.Extends = e.extends
	env.Protected = e.protect
	env.Vars = make([]model.EnvVar, 0, len(e.rows))
	for _, r := range e.rows {
		v := model.EnvVar{Name: r.Name, Value: r.Value, Secret: r.Secret, Kind: r.Kind}
		if r.Secret {
			v.Value, v.Kind = "", model.VarString
		}
		env.Vars = append(env.Vars, v)
	}
	if err := checkEnvNames(e.rows); err != nil {
		e.err = err.Error()
		e.changed()
		return
	}
	if err := checkKinds(env.Vars); err != nil {
		e.err = err.Error()
		e.changed()
		return
	}
	rows, wasSecret := slices.Clone(e.rows), e.secretNames()
	// Before: renaming a secret row, or clearing its Secret box, removed the stored value on Save and kept nothing
	// under the new name. The first Save now says what would be removed; the same Save again confirms it.
	if lost := e.storedLost(wasSecret, rows); len(lost) > 0 && !slices.Equal(lost, e.losing) {
		e.losing = lost
		e.err = fmt.Sprintf("Saving removes the stored value of %s. A renamed variable does not keep its value. Save again to remove it, or put the name back.", strings.Join(lost, ", "))
		e.changed()
		return
	}
	e.losing = nil
	Run(e.w, func() error { return e.write(env, rows, wasSecret, dropComments) }, func(err error) { e.saved(env, err) })
}

// storedLost names the secrets whose vault value this Save would remove: no longer secret rows that hold a value.
func (e *Environment) storedLost(wasSecret map[string]bool, rows []EnvVarRow) []string {
	var out []string
	for _, name := range noLongerSecret(wasSecret, rows) {
		if e.stored[name] {
			out = append(out, name)
		}
	}
	return out
}

var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// checkEnvNames refuses a blank, invalid or repeated variable name, naming the row. Before, such rows were written
// as they were, and a blank secret row called SetSecret with an empty name.
func checkEnvNames(rows []EnvVarRow) error {
	seen := map[string]int{}
	for i, r := range rows {
		switch {
		case strings.TrimSpace(r.Name) == "":
			return fmt.Errorf("row %d has no name: give it a name or remove it", i+1)
		case !envVarName.MatchString(r.Name):
			return fmt.Errorf("row %d: %q is not a variable name: use letters, digits and underscores, not starting with a digit", i+1, r.Name)
		case seen[r.Name] > 0:
			return fmt.Errorf("row %d: %q is already used by row %d", i+1, r.Name, seen[r.Name])
		}
		seen[r.Name] = i + 1
	}
	return nil
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

// checkKinds refuses to save a variable whose value does not fit its kind, naming the first one. The file
// would otherwise keep it as text and a number would go out as a string, the thing the kind is for.
func checkKinds(vars []model.EnvVar) error {
	for _, v := range vars {
		if _, err := v.Typed(); err != nil {
			return err
		}
	}
	return nil
}
