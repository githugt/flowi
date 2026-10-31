// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// collectionEnvSource reads the environment files a collection ships in its
// own folder. The workspace's environments are global, so these are only
// offered for copying in; the flowi-lib backend has it.
type collectionEnvSource interface {
	CollectionEnvironments(collection model.ID) ([]model.Environment, diag.List, error)
}

// EnvOffer is the environments a mounted collection ships that the
// workspace does not have yet.
type EnvOffer struct {
	Collection   string   `json:"collection"` // the collection's id
	Name         string   `json:"name"`       // the collection's name
	Environments []string `json:"environments"`
}

// EnvironmentOffers lists, per collection, the environments it ships that
// the workspace lacks. A collection whose files cannot be read offers none.
func (s *WorkspaceService) EnvironmentOffers() []EnvOffer {
	out := []EnvOffer{}
	src, ok := s.app.backend.(collectionEnvSource)
	if !ok {
		return out
	}
	cols, have := s.collectionsAndEnvNames()
	for _, c := range cols {
		envs, _, err := src.CollectionEnvironments(c.Manifest.ID)
		if err != nil {
			continue // an unreadable folder is reported by the Problems panel, not offered
		}
		if names := newEnvNames(envs, have); len(names) > 0 {
			out = append(out, EnvOffer{Collection: string(c.Manifest.ID), Name: c.Manifest.Name, Environments: names})
		}
	}
	return out
}

// ImportEnvironments copies the environments collection ships that the
// workspace lacks into the workspace, and returns their names. Secret
// variables come over as declarations: their values are set in the
// environment's tab, into the vault.
func (s *WorkspaceService) ImportEnvironments(collection string) ([]string, error) {
	src, ok := s.app.backend.(collectionEnvSource)
	if !ok {
		return nil, fmt.Errorf("this window cannot read a collection's environments")
	}
	envs, _, err := src.CollectionEnvironments(model.ID(collection))
	if err != nil {
		return nil, fmt.Errorf("reading the collection's environments: %w", err)
	}
	_, have := s.collectionsAndEnvNames()
	added := []string{}
	for _, e := range envs {
		if slices.Contains(have, e.Name) {
			continue
		}
		if err := s.copyEnvironment(e); err != nil {
			s.app.actor.Do(s.app.workspace.Load)
			return added, fmt.Errorf("adding environment %q: %w", e.Name, err)
		}
		added = append(added, e.Name)
	}
	s.app.actor.Do(s.app.workspace.Load) // nothing watches the environments directory
	return added, nil
}

// copyEnvironment creates e in the workspace with its variables. It calls the
// backend, so it runs off the actor.
func (s *WorkspaceService) copyEnvironment(e model.Environment) error {
	file, err := s.app.backend.NewEnvironment(e.Name)
	if err != nil {
		return err
	}
	created := model.Environment{Meta: model.Meta{Name: e.Name}, File: file}
	if i := slices.IndexFunc(s.app.backend.Environments(), func(x model.Environment) bool { return x.File == file }); i >= 0 {
		created = s.app.backend.Environments()[i] // keeps the fresh id NewEnvironment gave it
	}
	created.Vars = make([]model.EnvVar, len(e.Vars))
	for i, v := range e.Vars {
		created.Vars[i] = model.EnvVar{Name: v.Name, Value: v.Value, Secret: v.Secret}
	}
	return s.app.backend.SaveEnvironment(created, false)
}

// collectionsAndEnvNames is the mounted collections (not the drafts) and the
// workspace's environment names, read on the actor.
func (s *WorkspaceService) collectionsAndEnvNames() ([]model.Collection, []string) {
	var cols []model.Collection
	var names []string
	s.app.actor.Do(func() {
		ws := s.app.workspace
		for _, c := range ws.Collections() {
			if !ws.IsDraftCollection(c.Manifest.ID) {
				cols = append(cols, c)
			}
		}
		names = slices.Clone(ws.Environments())
	})
	return cols, names
}

func newEnvNames(envs []model.Environment, have []string) []string {
	var out []string
	for _, e := range envs {
		if !slices.Contains(have, e.Name) {
			out = append(out, e.Name)
		}
	}
	return out
}

// Unset lists the env.* and secret.* variables the workspace's requests and
// flows use that environment node (with its unsaved edits) does not define,
// so its tab can offer them as rows to fill in.
func (s *EnvironmentService) Unset(node string) []MissingVar {
	out := []MissingVar{}
	s.with(node, func(e *vm.Environment) {
		var vars, secrets []string
		for _, r := range e.Rows() {
			if r.Secret {
				secrets = append(secrets, r.Name)
			} else {
				vars = append(vars, r.Name)
			}
		}
		for _, m := range missingVars(workspaceTexts(s.app.workspace.Collections()), vm.NewNameChecker(vars, secrets)) {
			if strings.HasPrefix(m.Expr, "env.") || strings.HasPrefix(m.Expr, "secret.") {
				out = append(out, m) // a bare name is mostly a request's or flow's input, not the environment's
			}
		}
	})
	return out
}

// workspaceTexts is every templated text of every request version and flow step.
func workspaceTexts(cols []model.Collection) []string {
	var texts []string
	for _, c := range cols {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				texts = append(texts, templatedTexts(v)...)
			}
		}
		for _, f := range c.Flows {
			texts = appendNodeTexts(texts, f.Nodes)
		}
	}
	return texts
}

func appendNodeTexts(texts []string, nodes []model.Node) []string {
	for _, n := range nodes {
		texts = appendFieldTexts(texts, n.Params)
		if n.Request != nil {
			for _, kv := range n.Request.With {
				texts = append(texts, kv.Value)
			}
		}
		for _, list := range n.Children() {
			texts = appendNodeTexts(texts, list)
		}
	}
	return texts
}

// appendFieldTexts adds the strings anywhere in a step's free-form options.
func appendFieldTexts(texts []string, fields []model.Field) []string {
	for _, f := range fields {
		texts = appendValueTexts(texts, f.Value)
	}
	return texts
}

func appendValueTexts(texts []string, v any) []string {
	switch x := v.(type) {
	case string:
		return append(texts, x)
	case []any:
		for _, e := range x {
			texts = appendValueTexts(texts, e)
		}
	case []model.Field:
		return appendFieldTexts(texts, x)
	}
	return texts
}
