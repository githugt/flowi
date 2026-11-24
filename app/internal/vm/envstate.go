// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"slices"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// EnvIssue is something an environment cannot do yet: a value it lacks or a base it cannot find.
type EnvIssue struct {
	Brief string
	Hint  string
}

// EnvState is what the sidebar shows beside an environment: whether requests are sent with it now, and what is
// unresolved in it.
type EnvState struct {
	File   string
	Name   string
	Active bool
	Issues []EnvIssue
}

// EnvironmentStates says, for each environment, whether it is the selected one and what is unresolved: a secret that
// holds no value (neither in the vault nor from the process environment) and a base it builds on that does not exist
// or that loops back. infos says which secrets hold a value; pass nil when that could not be read, and secrets are
// then not judged.
//
// Before: the sidebar listed environments by name only; a secret nobody had filled in showed up when a request using
// it was sent.
func EnvironmentStates(envs []model.Environment, infos []flowi.SecretInfo, selected string, secretsKnown bool) []EnvState {
	out := make([]EnvState, 0, len(envs))
	for _, e := range envs {
		st := EnvState{File: e.File, Name: e.Name, Active: e.Name == selected}
		if secretsKnown {
			for _, v := range e.Vars {
				if v.Secret && !slices.ContainsFunc(infos, func(s flowi.SecretInfo) bool {
					return s.Environment == e.Name && s.Name == v.Name && (s.Set || s.Override)
				}) {
					st.Issues = append(st.Issues, EnvIssue{Brief: fmt.Sprintf("The secret %s has no value.", v.Name), Hint: "Give it one on the Secrets screen, or set FLOWI_SECRET_<ENV>_<NAME>."})
				}
			}
		}
		if e.Extends != "" {
			if _, err := model.EnvironmentChain(envs, e.Name); err != nil {
				st.Issues = append(st.Issues, baseIssue(e, err))
			}
		}
		out = append(out, st)
	}
	return out
}

func baseIssue(e model.Environment, err error) EnvIssue {
	if errors.Is(err, model.ErrEnvironmentLoop) {
		return EnvIssue{Brief: "It builds on environments that build on it in turn.", Hint: "Change 'extends' in one of them so the chain ends."}
	}
	return EnvIssue{Brief: fmt.Sprintf("It builds on %q, which does not exist.", e.Extends), Hint: "Fix 'extends' in the environment's file, or add that environment."}
}
