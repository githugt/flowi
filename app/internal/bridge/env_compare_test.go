// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func compareApp(t *testing.T, envs ...model.Environment) *App {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{{Environments: envs}}}
	app := NewApp(b, NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	return app
}

// env is an environment in one group; "" puts it in the default group.
func env(name, group string, vars ...model.EnvVar) model.Environment {
	e := model.Environment{Meta: model.Meta{Name: name}, File: "/e/" + name + ".yaml", Vars: vars}
	if group != "" {
		e.Groups = []string{group}
	}
	return e
}

func TestEnvironmentService_Compare_should_list_the_workspace_environments_with_a_row_per_variable(t *testing.T) {
	app, _, _ := twoCollections(t)

	v := app.Environments().Compare("")

	if len(v.Envs) == 0 || len(v.Rows) == 0 {
		t.Fatalf("view %+v", v)
	}
	for _, r := range v.Rows {
		if len(r.Cells) != len(v.Envs) {
			t.Fatalf("row %s has %d cells for %d environments", r.Name, len(r.Cells), len(v.Envs))
		}
		for _, c := range r.Cells {
			if c.State == "secret" && c.Value != "" {
				t.Fatalf("a secret's value is in the view: %+v", r)
			}
		}
	}
}

// Before: the cell view had no kind, so the screen could not tell 123 as text from 123 as a number.
func TestEnvironmentService_Compare_Given_the_same_text_as_text_and_as_a_number_Then_the_row_differs_and_each_cell_carries_its_kind(t *testing.T) {
	app := compareApp(t,
		env("env1", "", model.EnvVar{Name: "var3", Value: "123"}),
		env("env2", "", model.EnvVar{Name: "var3", Value: "123", Kind: model.VarNumber}),
	)

	v := app.Environments().Compare("")

	r := v.Rows[0]
	if !r.Differs || !r.KindsDiffer || r.Cells[0].Kind != "" || r.Cells[1].Kind != "number" {
		t.Fatalf("row %+v", r)
	}
}

func TestEnvironmentService_Compare_Given_groups_Then_one_group_is_compared_and_every_group_is_listed(t *testing.T) {
	app := compareApp(t, env("a", "checkout"), env("b", "checkout"), env("c", ""))

	v := app.Environments().Compare("checkout")

	if !slices.Equal(v.Envs, []string{"a", "b"}) || v.Group != "checkout" || len(v.Groups) != 2 || v.Groups[1].Name != "checkout" || v.Groups[1].Count != 2 {
		t.Fatalf("view %+v", v)
	}
	if v.Notes == nil {
		t.Fatal("notes is null in the JSON: the screen loops over it")
	}
}
