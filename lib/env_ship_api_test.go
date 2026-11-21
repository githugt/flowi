// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"net/http/httptest"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/examples/demoserver"
	"github.com/githugt/flowi/lib/model"
)

// shippedWorkspace mounts a copy of the example collection and keeps its environments in the collection's own folder,
// as a repository clone has them: the workspace has none. local points at a demo server, and its password secret comes
// from the process environment, as it does in CI.
func shippedWorkspace(t *testing.T) *Workspace {
	t.Helper()
	return shippedWorkspaceWith(t, "FLOWI_SECRET_LOCAL_PASSWORD=demo")
}

func shippedWorkspaceWith(t *testing.T, environ ...string) *Workspace {
	t.Helper()
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "demo"}))
	t.Cleanup(srv.Close)
	dir := copyDir(t, filepath.Join("examples", "user-creation"))
	local := filepath.Join(dir, "environments", "local.yaml")
	data, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte(strings.Replace(string(data), "http://localhost:8080", srv.URL, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, environ...)
	e.opts.Keys = &memKeys{}
	w := e.open(t)
	mustMount(t, w, dir)
	return w
}

// Before: the environment files a collection carries were never used, so a clone of the repository on a CI machine
// answered `--env local` with "environment not found" although local.yaml was committed.
func TestSendRequest_Given_an_environment_only_the_collection_ships_When_sent_Then_it_is_used(t *testing.T) {
	w := shippedWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	if res.Status != engine.StatusPassed || res.Record.Status != 200 {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	if len(w.Environments()) != 0 {
		t.Fatalf("the workspace must not have gained environments: %v", w.Environments())
	}
}

func TestShippedEnvironments_should_list_what_the_collections_ship_and_the_workspace_lacks(t *testing.T) {
	w := shippedWorkspace(t)
	if _, err := w.NewEnvironment("local"); err != nil {
		t.Fatal(err) // the person's own local: it wins over the shipped one
	}

	got := w.ShippedEnvironments()

	var names []string
	for _, s := range got {
		names = append(names, s.Environment.Name)
		if s.Name != "User creation" {
			t.Errorf("collection %q", s.Name)
		}
	}
	if !slices.Equal(names, []string{"staging"}) {
		t.Fatalf("shipped = %v, want only staging: the workspace has its own local", names)
	}
}

func TestOpenWorkspace_Given_a_workspace_environment_of_the_same_name_Then_the_workspace_one_wins(t *testing.T) {
	w := shippedWorkspace(t)
	if _, err := w.NewEnvironment("local"); err != nil {
		t.Fatal(err)
	}

	envs := w.Collections()[0].Environments

	var local []model.Environment
	for _, e := range envs {
		if e.Name == "local" {
			local = append(local, e)
		}
	}
	if len(local) != 1 || len(local[0].Vars) != 0 {
		t.Fatalf("local = %+v, want only the workspace's empty one", local)
	}
}

func TestImportEnvironments_should_copy_what_the_collection_ships_with_fresh_ids_and_no_secret_values(t *testing.T) {
	w := shippedWorkspace(t)
	c := w.Collections()[0]

	added, err := w.ImportEnvironments(c.Manifest.ID)
	again, errAgain := w.ImportEnvironments(c.Manifest.ID)

	if err != nil || errAgain != nil {
		t.Fatal(err, errAgain)
	}
	slices.Sort(added)
	if !slices.Equal(added, []string{"local", "staging"}) || len(again) != 0 {
		t.Fatalf("added %v, again %v", added, again)
	}
	var local model.Environment
	for _, e := range w.Environments() {
		if e.Name == "local" {
			local = e
		}
	}
	shipped := filepath.Join(c.Root, "environments", "local.yaml")
	if local.ID == "01JA0000000000000000000002" || local.File == shipped {
		t.Fatalf("the copy must be its own file with its own id: %+v", local)
	}
	if i := slices.IndexFunc(local.Vars, func(v model.EnvVar) bool { return v.Name == "password" }); i < 0 || !local.Vars[i].Secret || local.Vars[i].Value != "" {
		t.Fatalf("vars %+v", local.Vars)
	}
	if got := w.ShippedEnvironments(); len(got) != 0 {
		t.Fatalf("nothing is left to import: %v", got)
	}
}

func TestImportEnvironments_Given_names_Then_only_those_come_and_an_unknown_one_is_refused(t *testing.T) {
	w := shippedWorkspace(t)
	id := w.Collections()[0].Manifest.ID

	_, unknown := w.ImportEnvironments(id, "nope")
	added, err := w.ImportEnvironments(id, "staging")

	if codes := codesOfErr(unknown); !slices.Equal(codes, []diag.Code{diag.CodeRequestEnvUnknown}) {
		t.Fatalf("unknown: %v", unknown)
	}
	if err != nil || !slices.Equal(added, []string{"staging"}) || len(w.Environments()) != 1 {
		t.Fatalf("added %v err %v envs %v", added, err, w.Environments())
	}
}

func TestInBaseOrder_should_put_a_base_before_the_environments_that_build_on_it(t *testing.T) {
	envs := []model.Environment{
		{Meta: model.Meta{Name: "leaf"}, Extends: "mid"},
		{Meta: model.Meta{Name: "mid"}, Extends: "base"},
		{Meta: model.Meta{Name: "base"}},
		{Meta: model.Meta{Name: "other"}, Extends: "held"},
	}

	got := environmentNames(inBaseOrder(envs, []string{"held"}))

	if !slices.Equal(got, []string{"base", "other", "mid", "leaf"}) {
		t.Fatalf("order %v", got)
	}
}

func TestSetEnvironmentVar_should_add_replace_and_keep_the_kind(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := w.NewEnvironment("qa"); err != nil {
		t.Fatal(err)
	}

	added, err := w.SetEnvironmentVar("qa", "retries", "3", model.VarNumber)
	replaced, errAgain := w.SetEnvironmentVar("qa", "retries", "5", model.VarNumber)

	if err != nil || errAgain != nil || !added || replaced {
		t.Fatalf("added %v/%v errs %v %v", added, replaced, err, errAgain)
	}
	got := workspaceEnv(w, "qa").Vars
	if !slices.Equal(got, []model.EnvVar{{Name: "retries", Value: "5", Kind: model.VarNumber}}) {
		t.Fatalf("vars %+v", got)
	}
}

func TestSetEnvironmentVar_Given_a_bad_request_Then_it_is_refused_and_the_file_is_untouched(t *testing.T) {
	w := demoWorkspace(t)
	before := workspaceEnv(w, "local")

	_, notNumber := w.SetEnvironmentVar("local", "n", "five", model.VarNumber)
	_, secret := w.SetEnvironmentVar("local", "password", "x", model.VarString)
	_, unknown := w.SetEnvironmentVar("nope", "a", "b", model.VarString)
	_, nameless := w.SetEnvironmentVar("local", "", "b", model.VarString)

	for name, err := range map[string]error{"not a number": notNumber, "secret": secret, "nameless": nameless} {
		if err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if secret != nil && !strings.Contains(secret.Error(), "flowi secret set") {
		t.Errorf("a secret must point at the vault: %v", secret)
	}
	if codes := codesOfErr(unknown); !slices.Equal(codes, []diag.Code{diag.CodeRequestEnvUnknown}) {
		t.Errorf("unknown env: %v", unknown)
	}
	if after := workspaceEnv(w, "local"); !slices.Equal(after.Vars, before.Vars) {
		t.Fatalf("vars changed: %+v -> %+v", before.Vars, after.Vars)
	}
}

func TestUnsetEnvironmentVar_should_remove_a_plain_variable_and_refuse_a_secret(t *testing.T) {
	w := demoWorkspace(t)

	removed, err := w.UnsetEnvironmentVar("local", "verify_wait")
	again, errAgain := w.UnsetEnvironmentVar("local", "verify_wait")
	_, secret := w.UnsetEnvironmentVar("local", "password")

	if err != nil || errAgain != nil || !removed || again {
		t.Fatalf("removed %v/%v errs %v %v", removed, again, err, errAgain)
	}
	if secret == nil {
		t.Fatal("a secret declaration must not be removed without its stored value")
	}
	for _, v := range workspaceEnv(w, "local").Vars {
		if v.Name == "verify_wait" {
			t.Fatalf("still there: %+v", v)
		}
	}
}

func workspaceEnv(w *Workspace, name string) model.Environment {
	for _, e := range w.Environments() {
		if e.Name == name {
			return e
		}
	}
	return model.Environment{}
}

func TestSetEnvironmentVar_should_mark_a_caller_mistake_so_a_command_can_call_it_usage(t *testing.T) {
	w := demoWorkspace(t)

	_, notNumber := w.SetEnvironmentVar("local", "n", "five", model.VarNumber)
	_, secret := w.SetEnvironmentVar("local", "password", "x", model.VarString)
	_, unknown := w.SetEnvironmentVar("nope", "a", "b", model.VarString)

	if !errors.Is(notNumber, ErrEnvironmentVar) || !errors.Is(secret, ErrEnvironmentVar) {
		t.Fatalf("not marked: %v / %v", notNumber, secret)
	}
	if strings.Contains(notNumber.Error(), "refused") || errors.Is(unknown, ErrEnvironmentVar) {
		t.Fatalf("the message must be the plain reason, and an unknown environment has its own code: %v / %v", notNumber, unknown)
	}
}

// Before: a run of a shipped environment whose secret was not provided said "set it with `flowi secret set`", which
// then answered "environment not found": there is no vault entry until the environment is imported.
func TestSendRequest_Given_a_shipped_environment_with_no_secret_provided_Then_the_message_names_the_variable_to_set(t *testing.T) {
	w := shippedWorkspaceWith(t)
	target, _ := w.FindRequest("auth/login", "")

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	var found bool
	for _, d := range res.Diags {
		if d.Code == diag.CodeSecretShipped && d.Args["var"] == "FLOWI_SECRET_LOCAL_PASSWORD" {
			found = true
		}
	}
	if res.Status != engine.StatusError || !found {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSetSecret_Given_a_shipped_environment_Then_it_says_to_import_it_first_and_import_makes_it_work(t *testing.T) {
	w := shippedWorkspaceWith(t)

	err := w.SetSecret("local", "password", "x")
	_, envErr := w.SetEnvironmentVar("local", "retries", "3", model.VarNumber)
	_, unknown := w.SetEnvironmentVar("nope", "a", "b", model.VarString)
	if !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeRequestEnvShipped}) || !slices.Equal(codesOfErr(envErr), []diag.Code{diag.CodeRequestEnvShipped}) {
		t.Fatalf("secret %v, env %v", err, envErr)
	}
	if !slices.Equal(codesOfErr(unknown), []diag.Code{diag.CodeRequestEnvUnknown}) {
		t.Fatalf("a name nobody ships stays unknown: %v", unknown)
	}

	if _, err := w.ImportEnvironments(w.Collections()[0].Manifest.ID, "local"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSecret("local", "password", "x"); err != nil {
		t.Fatalf("after the import: %v", err)
	}
}
