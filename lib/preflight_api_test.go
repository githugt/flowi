// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"

	"github.com/githugt/flowi/lib/model"
)

func preflightEnvs() []model.Environment {
	return []model.Environment{
		{Name: "base", Vars: []model.EnvVar{{Name: "host", Value: "x"}, {Name: "key", Secret: true}}},
		{Name: "staging", Extends: "base", Vars: []model.EnvVar{{Name: "token", Secret: true}, {Name: "user", Secret: true}}},
	}
}

// Before: a missing variable or secret was found in the middle of a run, after the earlier steps had made their requests.
func TestPreflightNeeds_should_list_what_the_environment_lacks(t *testing.T) {
	texts := []string{
		`{"url":"{{env.host}}/a?u={{env.region}}","auth":"{{secret.token}}"}`, // region: no such variable; token: no value
		`{"x":"{{secret.user}} {{secret.key}} {{secret.other}}"}`,             // user has a value; key is not inherited; other is not declared
	}
	infos := []SecretInfo{{Environment: "staging", Name: "user", Set: true}, {Environment: "staging", Name: "token"}}

	got := preflightNeeds(texts, preflightEnvs(), "staging", infos)

	want := []Need{{NeedVariable, "region"}, {NeedSecret, "key"}, {NeedSecret, "other"}, {NeedSecretValue, "token"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPreflightNeeds_should_take_an_override_as_a_value_and_ignore_look_alikes(t *testing.T) {
	texts := []string{`{"a":"{{secret.token}}","b":"api.env.local and environment.name and nosecret.x"}`}
	infos := []SecretInfo{{Environment: "staging", Name: "token", Override: true}}

	got := preflightNeeds(texts, preflightEnvs(), "staging", infos)

	if len(got) != 0 {
		t.Fatalf("got %v, want nothing", got)
	}
}

func TestPreflightNeeds_Given_an_unknown_environment_Then_it_is_left_to_the_run(t *testing.T) {
	got := preflightNeeds([]string{`{{env.x}}`}, preflightEnvs(), "nowhere", nil)

	if got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestPreflightFlow_Given_no_environment_Then_there_is_nothing_to_check(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	got, err := w.PreflightFlow(target, "", nil)

	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPreflightFlow_should_read_the_draft_with_its_unsaved_edits(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	envs := w.Environments()
	if len(envs) == 0 {
		t.Fatal("the demo workspace has no environment")
	}
	saved, err := w.PreflightFlow(target, envs[0].Name, nil)
	f.Nodes = append(f.Nodes, model.Node{ID: "peek", Type: "log", Params: []model.Field{{Key: "value", Value: "{{secret.not_declared_anywhere}}"}}})

	draft, draftErr := w.PreflightFlow(target, envs[0].Name, &f)

	if err != nil || draftErr != nil || len(saved) != 0 || !slices.Contains(draft, Need{NeedSecret, "not_declared_anywhere"}) {
		t.Fatalf("saved %v (%v), draft %v (%v)", saved, err, draft, draftErr)
	}
}

// Before: `flowi validate` could not say whether a flow could run in an environment: a teammate found out halfway through a run.
func TestNeedDiagnostic_should_name_the_flow_the_thing_and_the_environment(t *testing.T) {
	f := model.Flow{Meta: model.Meta{Name: "Signup"}, File: "/c/flows/signup.arazzo.yaml"}

	d := needDiagnostic(f, "staging", Need{NeedSecretValue, "token"})

	want := `flow "Signup" reads {{secret.token}}, but environment "staging" has no value for that secret`
	if d.Code != diag.CodeRunEnvMissing || d.File != f.File || !strings.HasPrefix(diag.Text(d), want) {
		t.Fatalf("got %s %q", d.Code, diag.Text(d))
	}
}

func TestPreflightAll_Given_an_environment_with_everything_Then_it_is_quiet(t *testing.T) {
	w := demoWorkspace(t)
	envs := w.Environments()

	got, err := w.PreflightAll(envs[0].Name)

	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}
