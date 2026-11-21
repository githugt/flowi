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
