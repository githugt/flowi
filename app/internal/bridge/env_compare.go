// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "github.com/githugt/flowi/app/internal/vm"

// EnvChangeView is a variable both environments have with different values.
type EnvChangeView struct {
	Name string `json:"name"`
	A    string `json:"a"`
	B    string `json:"b"`
}

// EnvComparisonView is how two environments differ, for the compare dialog.
type EnvComparisonView struct {
	A       string          `json:"a"`
	B       string          `json:"b"`
	OnlyA   []string        `json:"onlyA"`
	OnlyB   []string        `json:"onlyB"`
	Changed []EnvChangeView `json:"changed"`
	Same    int             `json:"same"`
	Secrets []string        `json:"secrets"` // names whose values are hidden
	Error   string          `json:"error,omitempty"`
}

// Compare says how the environments named a and b differ. Secret values are never part of it.
func (s *EnvironmentService) Compare(a, b string) EnvComparisonView {
	out := EnvComparisonView{A: a, B: b, OnlyA: []string{}, OnlyB: []string{}, Changed: []EnvChangeView{}, Secrets: []string{}}
	s.app.actor.Do(func() {
		c, err := s.app.workspace.CompareEnvironments(a, b)
		if err != nil {
			out.Error = vm.ErrorText(err)
			return
		}
		// Copies, so an empty list arrives as [] and not null.
		out.OnlyA, out.OnlyB, out.Secrets = append(out.OnlyA, c.OnlyA...), append(out.OnlyB, c.OnlyB...), append(out.Secrets, c.Secrets...)
		out.Same = c.Same
		for _, ch := range c.Changed {
			out.Changed = append(out.Changed, EnvChangeView{Name: ch.Name, A: ch.A, B: ch.B})
		}
	})
	return out
}

// EnvOverridesView is what an environment changes about the one it builds on.
type EnvOverridesView struct {
	Extends string `json:"extends"` // "" when the environment builds on nothing
	Count   int    `json:"count"`   // variables it sets differently, plus those it adds
}

// Overrides counts what the environment named name changes about the one it extends, for the context chip.
func (s *EnvironmentService) Overrides(name string) EnvOverridesView {
	var out EnvOverridesView
	s.app.actor.Do(func() {
		out.Extends = s.app.workspace.EnvironmentExtends(name)
		if out.Extends == "" {
			return
		}
		c, err := s.app.workspace.CompareEnvironments(out.Extends, name)
		if err != nil {
			return
		}
		out.Count = len(c.Changed) + len(c.OnlyB)
	})
	return out
}
