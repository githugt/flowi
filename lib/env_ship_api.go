// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

// ErrEnvironmentVar is what SetEnvironmentVar and UnsetEnvironmentVar wrap when the request cannot be done as asked: a
// nameless variable, a value that does not fit its kind, or a secret. The caller made a mistake; nothing was written.
var ErrEnvironmentVar = errors.New("environment variable request refused")

// refusedVar is an error that reads as its own message and matches ErrEnvironmentVar.
type refusedVar struct{ err error }

func (e refusedVar) Error() string               { return e.err.Error() }
func (e refusedVar) Unwrap() error               { return e.err }
func (refusedVar) Is(target error) bool          { return target == ErrEnvironmentVar }
func refuseVar(format string, args ...any) error { return refusedVar{fmt.Errorf(format, args...)} }

// withShipped returns the workspace's environments followed by the ones a collection ships in its own environments
// folder (loaded without a workspace scope) that the workspace does not have by name. current is what the collection
// holds now: its shipped environments, and possibly an older copy of the workspace's, which is dropped.
//
// Before: a collection's own environment files were never used, so CI and a colleague who had cloned the repository
// had no `staging` and `flowi run --env staging` failed with "environment not found" although the file was committed.
// The workspace's environment of the same name still wins: a person's own copy holds their values.
func withShipped(global, current []model.Environment) []model.Environment {
	out := slices.Clone(global)
	for _, e := range current {
		if e.Scope == vault.WorkspaceScope || slices.ContainsFunc(global, func(g model.Environment) bool { return g.Name == e.Name }) {
			continue
		}
		e.Shipped = true
		out = append(out, e)
	}
	return out
}

// ShippedEnvironment is an environment a mounted collection carries in its own folder and that the workspace does not
// have, so runs and sends use it as it is: its plain variables from the file, its secrets from FLOWI_SECRET_<ENV>_<NAME>
// (there is no vault entry for it until it is imported).
type ShippedEnvironment struct {
	Collection  model.ID
	Name        string // the collection's name
	Environment model.Environment
}

// ShippedEnvironments lists the environments the mounted collections ship that the workspace lacks, in mount order.
func (w *Workspace) ShippedEnvironments() []ShippedEnvironment {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []ShippedEnvironment
	for _, c := range w.collections {
		for _, e := range c.Environments {
			if e.Scope != vault.WorkspaceScope {
				out = append(out, ShippedEnvironment{Collection: c.Manifest.ID, Name: c.Manifest.Name, Environment: e})
			}
		}
	}
	return out
}

// ImportEnvironments copies the environments a collection ships that the workspace lacks into the workspace, and
// returns their names. only limits it to those names; a name the collection does not ship is FLW-REQ-004. A secret
// comes over as a declaration: its value is set with SetSecret. A base environment is created before the one that
// builds on it, and a base the workspace will not have is left out, because the file would name an environment that
// does not exist. Colour, protection and each variable's kind are kept.
//
// Before: only the app could do this; a CI job or a colleague working in a terminal copied the files by hand.
func (w *Workspace) ImportEnvironments(collection model.ID, only ...string) ([]string, error) {
	shipped, _, err := w.CollectionEnvironments(collection)
	if err != nil {
		return nil, err
	}
	for _, name := range only {
		if !slices.ContainsFunc(shipped, func(e model.Environment) bool { return e.Name == name }) {
			return nil, diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": name})
		}
	}
	have := environmentNames(w.Environments())
	var added []string
	for _, e := range inBaseOrder(shipped, have) {
		if slices.Contains(have, e.Name) || (len(only) > 0 && !slices.Contains(only, e.Name)) {
			continue
		}
		if err := w.copyEnvironment(e, have); err != nil {
			return added, fmt.Errorf("adding environment %q: %w", e.Name, err)
		}
		have = append(have, e.Name)
		added = append(added, e.Name)
	}
	return added, nil
}

// copyEnvironment creates e in the workspace under a fresh id.
func (w *Workspace) copyEnvironment(e model.Environment, have []string) error {
	file, err := w.NewEnvironment(e.Name)
	if err != nil {
		return err
	}
	created, ok := w.environmentByFile(file)
	if !ok {
		return fmt.Errorf("%s was written but is not loaded", file)
	}
	created.Colour, created.Protected = e.Colour, e.Protected
	if slices.Contains(have, e.Extends) {
		created.Extends = e.Extends
	}
	created.Vars = make([]model.EnvVar, len(e.Vars))
	for i, v := range e.Vars {
		created.Vars[i] = model.EnvVar{Name: v.Name, Value: v.Value, Secret: v.Secret, Kind: v.Kind}
	}
	return w.SaveEnvironment(created, SaveOptions{DropComments: true}) // the file was created a moment ago: it has none
}

func (w *Workspace) environmentByFile(file string) (model.Environment, bool) {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.File == file })
	if i < 0 {
		return model.Environment{}, false
	}
	return envs[i], true
}

func environmentNames(envs []model.Environment) []string {
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	return names
}

// inBaseOrder puts every environment after the one it builds on, when both are in envs. Environments that build on
// nothing, or on one that is already held, keep their order. A loop stays as it is: the engine reports it.
func inBaseOrder(envs []model.Environment, have []string) []model.Environment {
	out := make([]model.Environment, 0, len(envs))
	placed := map[string]bool{}
	for _, n := range have {
		placed[n] = true
	}
	pending := slices.Clone(envs)
	for len(pending) > 0 {
		progress := false
		var rest []model.Environment
		for _, e := range pending {
			waits := e.Extends != "" && !placed[e.Extends] && slices.ContainsFunc(pending, func(x model.Environment) bool { return x.Name == e.Extends })
			if waits {
				rest = append(rest, e)
				continue
			}
			out, placed[e.Name], progress = append(out, e), true, true
		}
		if !progress {
			return append(out, rest...)
		}
		pending = rest
	}
	return out
}

// SetEnvironmentVar sets the plain variable name of the workspace's environment env, adding it when it is new. kind
// says what the value is read as (model.VarString for text); a value that does not fit its kind is refused. A secret
// variable is refused: its value belongs in the vault (SetSecret, DeclareSecret). It reports whether the variable
// was added (false: its value was replaced). An unknown environment is FLW-REQ-004.
//
// Before: a variable could be set only by editing the file or in the app, so a script or CI job could not prepare an
// environment.
func (w *Workspace) SetEnvironmentVar(env, name, value string, kind model.VarKind) (added bool, err error) {
	v := model.EnvVar{Name: name, Value: value, Kind: kind}
	if name == "" {
		return false, refuseVar("a variable needs a name")
	}
	if _, err := v.Typed(); err != nil {
		return false, refusedVar{err}
	}
	e, err := w.environmentNamed(env)
	if err != nil {
		return false, err
	}
	e.Vars = slices.Clone(e.Vars)
	switch i := slices.IndexFunc(e.Vars, func(x model.EnvVar) bool { return x.Name == name }); {
	case i < 0:
		e.Vars, added = append(e.Vars, v), true
	case e.Vars[i].Secret:
		return false, refuseVar("%q is a secret of the environment %q: its value is stored with `flowi secret set`, not in the file", name, env)
	default:
		e.Vars[i] = v
	}
	return added, w.SaveEnvironment(e, SaveOptions{})
}

// UnsetEnvironmentVar removes the plain variable name from the workspace's environment env. A secret is refused (the
// declaration and its stored value are removed together by the app). It reports whether the variable was there.
func (w *Workspace) UnsetEnvironmentVar(env, name string) (removed bool, err error) {
	e, err := w.environmentNamed(env)
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(e.Vars, func(x model.EnvVar) bool { return x.Name == name })
	switch {
	case i < 0:
		return false, nil
	case e.Vars[i].Secret:
		return false, refuseVar("%q is a secret of the environment %q: remove it in the app so its stored value goes with it", name, env)
	}
	e.Vars = slices.Delete(slices.Clone(e.Vars), i, i+1)
	return true, w.SaveEnvironment(e, SaveOptions{})
}

func (w *Workspace) environmentNamed(name string) (model.Environment, error) {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == name })
	if i < 0 {
		return model.Environment{}, w.envNotInWorkspace(name)
	}
	return envs[i], nil
}

// envNotInWorkspace is the error for an environment the workspace does not have: when a collection ships one of that
// name it says so and names the way to get it (`flowi env import`), instead of "not found" for a file the person can
// see in the collection and in `flowi env ls`.
//
// Before: `flowi secret set local password` answered "environment not found" for an environment `env ls` listed.
func (w *Workspace) envNotInWorkspace(name string) error {
	for _, s := range w.ShippedEnvironments() {
		if s.Environment.Name == name {
			return diagErr(diag.CodeRequestEnvShipped, map[string]string{"env": name, "collection": s.Name})
		}
	}
	return diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": name})
}
