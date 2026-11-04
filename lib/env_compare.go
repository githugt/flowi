// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"maps"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// EnvChange is a variable both environments have, with different values.
type EnvChange struct {
	Name string `json:"name"`
	A    string `json:"a"`
	B    string `json:"b"`
}

// EnvComparison says how two environments differ, as a run would see them: what each inherits from the
// environment it extends counts as its own.
type EnvComparison struct {
	A string `json:"a"`
	B string `json:"b"`
	// OnlyA and OnlyB are the variables only one has.
	OnlyA []string `json:"onlyA"`
	OnlyB []string `json:"onlyB"`
	// Changed are the variables both have with different values. Secrets are never in it: their values live in
	// the vault, not in the environment, so there is nothing here to compare.
	Changed []EnvChange `json:"changed"`
	// Same counts the variables both have with the same value (a secret both declare is the same).
	Same int `json:"same"`
	// Secrets names the variables, in either environment, whose values are hidden.
	Secrets []string `json:"secrets"`
}

// CompareEnvironments compares the environments named a and b. An unknown name is FLW-REQ-004.
func (w *Workspace) CompareEnvironments(a, b string) (EnvComparison, error) {
	envs := w.Environments()
	left, err := effectiveVars(envs, a)
	if err != nil {
		return EnvComparison{}, err
	}
	right, err := effectiveVars(envs, b)
	if err != nil {
		return EnvComparison{}, err
	}
	return compareVars(a, b, left, right), nil
}

// effectiveVars are the variables of the named environment with those it inherits, its own winning.
func effectiveVars(envs []model.Environment, name string) (map[string]model.EnvVar, error) {
	out := map[string]model.EnvVar{}
	seen := map[string]bool{}
	for current := name; current != "" && !seen[current]; {
		seen[current] = true // a cycle of extends ends here rather than looping
		i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == current })
		if i < 0 {
			if current == name {
				return nil, diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": name})
			}
			break // an unknown parent: what is known of the chain is used
		}
		for _, v := range envs[i].Vars {
			if _, own := out[v.Name]; !own {
				out[v.Name] = v
			}
		}
		current = envs[i].Extends
	}
	return out, nil
}

func compareVars(a, b string, left, right map[string]model.EnvVar) EnvComparison {
	c := EnvComparison{A: a, B: b, OnlyA: []string{}, OnlyB: []string{}, Changed: []EnvChange{}, Secrets: []string{}}
	for _, name := range slices.Sorted(maps.Keys(left)) {
		l := left[name]
		r, both := right[name]
		switch {
		case !both:
			c.OnlyA = append(c.OnlyA, name)
		case l.Secret || r.Secret || l.Value == r.Value:
			c.Same++
		default:
			c.Changed = append(c.Changed, EnvChange{Name: name, A: l.Value, B: r.Value})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(right)) {
		if _, both := left[name]; !both {
			c.OnlyB = append(c.OnlyB, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(maps.Collect(unionNames(left, right)))) {
		if left[name].Secret || right[name].Secret {
			c.Secrets = append(c.Secrets, name)
		}
	}
	return c
}

// unionNames yields every variable name of both maps, as a set.
func unionNames(a, b map[string]model.EnvVar) func(yield func(string, bool) bool) {
	return func(yield func(string, bool) bool) {
		for name := range a {
			if !yield(name, true) {
				return
			}
		}
		for name := range b {
			if !yield(name, true) {
				return
			}
		}
	}
}
