// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func envOf(name, extends string, vars ...model.EnvVar) model.Environment {
	return model.Environment{Meta: model.Meta{Name: name}, Vars: vars, Extends: extends}
}

func rowNamed(t *testing.T, m vm.EnvMatrix, name string) vm.EnvCompareRow {
	t.Helper()
	for _, r := range m.Rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no row %q in %+v", name, m.Rows)
	return vm.EnvCompareRow{}
}

// Before: finding why staging behaved unlike local meant opening each environment in turn and comparing by eye.
func TestCompareEnvironments_should_mark_differing_values_and_missing_variables(t *testing.T) {
	envs := []model.Environment{
		envOf("local", "", model.EnvVar{Name: "base_url", Value: "http://localhost"}, model.EnvVar{Name: "region", Value: "eu"}),
		envOf("staging", "", model.EnvVar{Name: "base_url", Value: "https://stg.example.com"}, model.EnvVar{Name: "region", Value: "eu"}, model.EnvVar{Name: "debug", Value: "1"}),
	}

	m := vm.CompareEnvironments(envs, nil)

	if len(m.Envs) != 2 || m.Envs[0] != "local" || len(m.Rows) != 3 {
		t.Fatalf("matrix %+v", m)
	}
	if r := rowNamed(t, m, "base_url"); !r.Differs || r.Gaps != 0 {
		t.Errorf("base_url %+v", r)
	}
	if r := rowNamed(t, m, "region"); r.Differs || r.Gaps != 0 {
		t.Errorf("region is the same everywhere: %+v", r)
	}
	if r := rowNamed(t, m, "debug"); r.Differs || r.Gaps != 1 || r.Cells[0].State != vm.CellMissing || r.Cells[1].State != vm.CellSet {
		t.Errorf("debug is missing from local: %+v", r)
	}
}

func TestCompareEnvironments_should_say_which_secrets_hold_a_value(t *testing.T) {
	envs := []model.Environment{
		envOf("local", "", model.EnvVar{Name: "token", Secret: true}),
		envOf("staging", "", model.EnvVar{Name: "token", Secret: true}),
		envOf("prod", "", model.EnvVar{Name: "token", Secret: true}),
	}
	infos := []flowi.SecretInfo{{Environment: "local", Name: "token", Set: true}, {Environment: "prod", Name: "token", Override: true}}

	r := rowNamed(t, vm.CompareEnvironments(envs, infos), "token")

	got := []string{r.Cells[0].State, r.Cells[1].State, r.Cells[2].State}
	want := []string{vm.CellSecret, vm.CellSecretUnset, vm.CellSecret}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || r.Gaps != 1 || r.Differs {
		t.Fatalf("states %v gaps %d differs %v, want %v", got, r.Gaps, r.Differs, want)
	}
	for _, c := range r.Cells {
		if c.Value != "" {
			t.Fatalf("a secret's value must never be in the comparison: %+v", c)
		}
	}
}

func TestCompareEnvironments_should_show_inherited_values_and_where_they_come_from(t *testing.T) {
	envs := []model.Environment{
		envOf("base", "", model.EnvVar{Name: "region", Value: "eu"}, model.EnvVar{Name: "key", Secret: true}),
		envOf("staging", "base", model.EnvVar{Name: "debug", Value: "1"}),
	}

	m := vm.CompareEnvironments(envs, nil)

	region := rowNamed(t, m, "region")
	if c := region.Cells[1]; c.State != vm.CellInherited || c.Value != "eu" || c.Source != "base" || region.Differs {
		t.Errorf("region in staging: %+v differs %v", c, region.Differs)
	}
	if c := rowNamed(t, m, "key").Cells[1]; c.State != vm.CellMissing {
		t.Errorf("secrets are not inherited: %+v", c)
	}
}

func TestCompareEnvironments_Given_an_extends_loop_Then_stops(t *testing.T) {
	envs := []model.Environment{
		envOf("a", "b", model.EnvVar{Name: "x", Value: "1"}),
		envOf("b", "a", model.EnvVar{Name: "y", Value: "2"}),
	}

	m := vm.CompareEnvironments(envs, nil)

	if len(m.Rows) != 2 {
		t.Fatalf("rows %+v", m.Rows)
	}
}

// Before: a variable that was the number 1 in one environment and the text "1" in another showed no difference, though a
// step that used it on its own got a number from one and a string from the other.
func TestCompareEnvironments_should_mark_a_variable_whose_type_differs_though_its_text_is_the_same(t *testing.T) {
	envs := []model.Environment{
		envOf("local", "", model.EnvVar{Name: "retries", Value: "3", Kind: model.VarNumber}, model.EnvVar{Name: "mode", Value: "fast"}),
		envOf("staging", "", model.EnvVar{Name: "retries", Value: "3"}, model.EnvVar{Name: "mode", Value: "fast"}),
	}

	m := vm.CompareEnvironments(envs, nil)

	if r := rowNamed(t, m, "retries"); !r.Differs || r.Cells[0].Kind != "number" || r.Cells[1].Kind != "" {
		t.Errorf("retries %+v", r)
	}
	if r := rowNamed(t, m, "mode"); r.Differs {
		t.Errorf("mode is the same everywhere: %+v", r)
	}
}

// Before: the screen's cells did not say what a value was read as, so 123 in one environment and 123 in another looked
// the same wherever the bridge and the screen dropped the type.
func TestCompareEnvironments_Given_the_same_text_as_a_number_and_as_text_and_a_json_value_Then_the_row_differs_and_says_the_kinds_differ(t *testing.T) {
	envs := []model.Environment{
		envOf("env1", "", model.EnvVar{Name: "var3", Value: "123"}, model.EnvVar{Name: "same", Value: "123", Kind: model.VarNumber}),
		envOf("env2", "", model.EnvVar{Name: "var3", Value: "123", Kind: model.VarNumber}, model.EnvVar{Name: "same", Value: "123", Kind: model.VarNumber}),
	}

	m := vm.CompareEnvironments(envs, nil)

	if r := rowNamed(t, m, "var3"); !r.Differs || !r.KindsDiffer {
		t.Errorf("var3 %+v", r)
	}
	if r := rowNamed(t, m, "same"); r.Differs || r.KindsDiffer {
		t.Errorf("same is a number in both: %+v", r)
	}
}

func TestCompareEnvironments_Given_different_text_of_one_kind_Then_the_row_differs_but_the_kinds_do_not(t *testing.T) {
	envs := []model.Environment{
		envOf("a", "", model.EnvVar{Name: "n", Value: "1", Kind: model.VarNumber}),
		envOf("b", "", model.EnvVar{Name: "n", Value: "2", Kind: model.VarNumber}),
	}

	m := vm.CompareEnvironments(envs, nil)

	if r := rowNamed(t, m, "n"); !r.Differs || r.KindsDiffer {
		t.Errorf("n %+v", r)
	}
}

func TestCompareEnvironments_Given_an_empty_text_a_secret_and_a_missing_variable_Then_each_is_told_apart(t *testing.T) {
	infos := []flowi.SecretInfo{{Environment: "b", Name: "k", Set: true}, {Environment: "c", Name: "k", Set: true}}
	envs := []model.Environment{
		envOf("a", "", model.EnvVar{Name: "k", Value: ""}, model.EnvVar{Name: "e", Value: ""}, model.EnvVar{Name: "s", Secret: true}),
		envOf("b", "", model.EnvVar{Name: "k", Secret: true}, model.EnvVar{Name: "e", Value: ""}, model.EnvVar{Name: "s", Secret: true}),
		envOf("c", "", model.EnvVar{Name: "k", Secret: true}),
	}

	m := vm.CompareEnvironments(envs, infos)

	// An empty text against a secret is a difference: one is plain, the other is not.
	if r := rowNamed(t, m, "k"); !r.Differs || r.KindsDiffer {
		t.Errorf("k %+v", r)
	}
	// Two empty texts are the same, and a variable the third environment lacks is a gap, not a difference.
	if r := rowNamed(t, m, "e"); r.Differs || r.Gaps != 1 {
		t.Errorf("e %+v", r)
	}
	// Two secrets are not compared: their values are never read here.
	if r := rowNamed(t, m, "s"); r.Differs {
		t.Errorf("s %+v", r)
	}
}

// Before: a comparison held every environment of the workspace, however many sets of services they served.
func TestCompareGroup_Given_environments_in_groups_Then_only_the_ones_of_that_group_are_columns(t *testing.T) {
	a := envOf("a", "", model.EnvVar{Name: "x", Value: "1"})
	b := envOf("b", "", model.EnvVar{Name: "x", Value: "1"})
	c := envOf("c", "", model.EnvVar{Name: "y", Value: "1"})
	a.Groups, b.Groups = []string{"checkout"}, []string{"checkout"}

	checkout := vm.CompareGroup([]model.Environment{a, b, c}, "checkout", nil)
	fallback := vm.CompareGroup([]model.Environment{a, b, c}, "", nil)

	if len(checkout.Envs) != 2 || len(checkout.Rows) != 1 || checkout.Rows[0].Name != "x" {
		t.Errorf("checkout %+v", checkout)
	}
	if len(fallback.Envs) != 1 || fallback.Envs[0] != "c" || len(fallback.Rows) != 1 {
		t.Errorf("default group %+v", fallback)
	}
}

func TestCompareGroup_Given_a_base_in_another_group_Then_its_values_are_inherited_and_a_note_says_so(t *testing.T) {
	base := envOf("base", "", model.EnvVar{Name: "region", Value: "eu"})
	prod := envOf("prod", "base", model.EnvVar{Name: "url", Value: "u"})
	stg := envOf("stg", "", model.EnvVar{Name: "region", Value: "us"})
	prod.Groups, stg.Groups = []string{"checkout"}, []string{"checkout"}

	m := vm.CompareGroup([]model.Environment{base, prod, stg}, "checkout", nil)

	r := rowNamed(t, m, "region")
	if r.Cells[0].State != vm.CellInherited || r.Cells[0].Source != "base" || !r.Differs {
		t.Errorf("region %+v", r)
	}
	if len(m.Notes) != 1 || m.Envs[0] != "prod" || len(m.Envs) != 2 {
		t.Errorf("notes %v envs %v", m.Notes, m.Envs)
	}
}

// Before: an environment had one group, so one shared by two sets was a column in only one comparison.
func TestCompareGroup_Given_an_environment_in_two_groups_Then_it_is_a_column_of_each(t *testing.T) {
	shared := envOf("shared", "", model.EnvVar{Name: "x", Value: "1"})
	a := envOf("a", "", model.EnvVar{Name: "x", Value: "2"})
	b := envOf("b", "", model.EnvVar{Name: "x", Value: "3"})
	shared.Groups, a.Groups, b.Groups = []string{"one", "two"}, []string{"one"}, []string{"two"}
	all := []model.Environment{shared, a, b}

	one := vm.CompareGroup(all, "one", nil)
	two := vm.CompareGroup(all, "two", nil)

	if len(one.Envs) != 2 || one.Envs[0] != "shared" || len(two.Envs) != 2 || two.Envs[0] != "shared" || two.Envs[1] != "b" {
		t.Fatalf("one %v two %v", one.Envs, two.Envs)
	}
}

func TestCompareGroup_Given_a_base_that_is_in_the_group_through_another_of_its_groups_Then_there_is_no_note(t *testing.T) {
	base := envOf("base", "", model.EnvVar{Name: "x", Value: "1"})
	prod := envOf("prod", "base")
	base.Groups, prod.Groups = []string{"a", "b"}, []string{"b"}

	m := vm.CompareGroup([]model.Environment{base, prod}, "b", nil)

	if len(m.Notes) != 0 {
		t.Fatalf("notes %v", m.Notes)
	}
}
