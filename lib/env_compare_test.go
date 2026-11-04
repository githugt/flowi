// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// saveEnv makes an environment with these variables.
func saveEnv(t *testing.T, w *Workspace, name, extends string, vars ...model.EnvVar) {
	t.Helper()
	file, err := w.NewEnvironment(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range w.Environments() {
		if e.File == file {
			e.Vars, e.Extends = vars, extends
			if err := w.SaveEnvironment(e, SaveOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCompareEnvironments_should_list_what_only_one_has_and_what_differs(t *testing.T) {
	w := newEnv(t).open(t)
	saveEnv(t, w, "dev", "",
		model.EnvVar{Name: "base_url", Value: "http://localhost"},
		model.EnvVar{Name: "region", Value: "eu"},
		model.EnvVar{Name: "debug", Value: "1"},
		model.EnvVar{Name: "same", Value: "x"})
	saveEnv(t, w, "prod", "",
		model.EnvVar{Name: "base_url", Value: "https://api.example.com"},
		model.EnvVar{Name: "region", Value: "eu"},
		model.EnvVar{Name: "tenant", Value: "acme"},
		model.EnvVar{Name: "same", Value: "x"})

	got, err := w.CompareEnvironments("dev", "prod")

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.OnlyA, []string{"debug"}) || !slices.Equal(got.OnlyB, []string{"tenant"}) {
		t.Fatalf("only-in lists: %+v", got)
	}
	want := []EnvChange{{Name: "base_url", A: "http://localhost", B: "https://api.example.com"}}
	if !slices.Equal(got.Changed, want) || got.Same != 2 {
		t.Fatalf("changed %+v, same %d", got.Changed, got.Same)
	}
}

func TestCompareEnvironments_should_never_show_a_secret_value_only_that_it_is_one(t *testing.T) {
	w := newEnv(t).open(t)
	saveEnv(t, w, "dev", "", model.EnvVar{Name: "password", Secret: true})
	saveEnv(t, w, "prod", "", model.EnvVar{Name: "password", Secret: true}, model.EnvVar{Name: "token", Secret: true})

	got, err := w.CompareEnvironments("dev", "prod")

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.OnlyB, []string{"token"}) || len(got.Changed) != 0 || got.Same != 1 {
		t.Fatalf("got %+v: a secret in both is the same, whatever the vault holds", got)
	}
	if !slices.Equal(got.Secrets, []string{"password", "token"}) {
		t.Fatalf("secrets %v: the caller needs to know which names to show as hidden", got.Secrets)
	}
}

func TestCompareEnvironments_should_count_what_an_environment_inherits_from_the_one_it_extends(t *testing.T) {
	w := newEnv(t).open(t)
	saveEnv(t, w, "base", "", model.EnvVar{Name: "region", Value: "eu"})
	saveEnv(t, w, "dev", "base", model.EnvVar{Name: "debug", Value: "1"})
	saveEnv(t, w, "prod", "", model.EnvVar{Name: "region", Value: "eu"})

	got, err := w.CompareEnvironments("dev", "prod")

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.OnlyA, []string{"debug"}) || len(got.OnlyB) != 0 || got.Same != 1 {
		t.Fatalf("got %+v: dev has region from base", got)
	}
}

func TestCompareEnvironments_Given_an_unknown_name_Then_says_which(t *testing.T) {
	w := newEnv(t).open(t)
	saveEnv(t, w, "dev", "")

	_, err := w.CompareEnvironments("dev", "nope")

	if err == nil {
		t.Fatal("want an error")
	}
}
