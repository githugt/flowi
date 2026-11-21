// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
	"github.com/githugt/flowi/lib/workspace"
)

const sharedEnv = "id: 01JA0000000000000000000009\nvars:\n  base_url: http://shared.example\n  token:\n    secret: true\n"

func writeGlobalEnv(t *testing.T, e env, name, content string) {
	t.Helper()
	dir := workspace.EnvironmentsDir(e.opts.Paths, workspace.DefaultName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func envNames(w *Workspace) []string {
	var out []string
	for _, e := range w.Collections()[0].Environments {
		out = append(out, e.Name)
	}
	return out
}

func TestOpenWorkspace_should_give_every_collection_the_global_environments(t *testing.T) {
	e := newEnv(t)
	writeGlobalEnv(t, e, "shared", sharedEnv)
	w := e.open(t)
	mustMount(t, w, copyDir(t, filepath.Join("examples", "user-creation")))

	again := e.open(t)

	// Before: the collection's own files (the example ships local and staging) were dropped, so only the global
	// one was left. They follow the global ones now, so a clone's environments work without being copied in.
	if got := envNames(again); !slices.Equal(got, []string{"shared", "local", "staging"}) {
		t.Fatalf("environments = %v, want the global one first, then the ones the collection ships", got)
	}
	if got := environmentNames(again.Environments()); !slices.Equal(got, []string{"shared"}) {
		t.Fatalf("workspace environments = %v, want only the global one", got)
	}
}

func TestSecrets_should_scope_global_environment_secrets_to_the_workspace(t *testing.T) {
	e := newEnv(t)
	e.opts.Keys = &memKeys{}
	writeGlobalEnv(t, e, "shared", sharedEnv)
	w := e.open(t)
	mustMount(t, w, copyDir(t, filepath.Join("examples", "user-creation")))
	w = e.open(t)

	if err := w.SetSecret("shared", "token", "s3cret"); err != nil {
		t.Fatal(err)
	}
	stored, err := w.secrets().Names(vault.WorkspaceScope)

	if err != nil || len(stored) != 1 || stored[0].Name != "token" {
		t.Fatalf("workspace-scope secrets = %+v %v", stored, err)
	}
}

// rewriteCollectionIdentity makes a copy of an example a different collection.
func rewriteCollectionIdentity(t *testing.T, dir, name string, id model.ID) {
	t.Helper()
	file := filepath.Join(dir, "flowi.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(data), "01JA0000000000000000000001", string(id), 1)
	s = strings.Replace(s, "name: User creation", "name: "+name, 1)
	if err := os.WriteFile(file, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Before: every collection carried the same global environments, so one
// environment file looked like a duplicate of itself.
func TestValidate_should_not_flag_a_global_environment_shared_by_collections_as_duplicate(t *testing.T) {
	e := newEnv(t)
	writeGlobalEnv(t, e, "shared", sharedEnv)
	w := e.open(t)
	mustMount(t, w, copyDir(t, filepath.Join("examples", "user-creation")))
	other := copyDir(t, filepath.Join("examples", "user-creation"))
	rewriteCollectionIdentity(t, other, "Tools", "01JA0000000000000000000T01")
	mustMount(t, w, other)
	w = e.open(t)

	got := w.Validate()

	for _, d := range got {
		if d.Code == diag.CodeIDDuplicate && strings.Contains(d.File, "shared.yaml") {
			t.Fatalf("shared environment reported as duplicate: %+v", d)
		}
	}
}

func TestCollectionEnvironments_should_read_the_collections_own_environment_files(t *testing.T) {
	w := demoWorkspace(t) // its environments were moved to the workspace
	c := w.Collections()[0]
	dir := filepath.Join(c.Root, "environments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shared.yaml"), []byte(sharedEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	got, diags, err := w.CollectionEnvironments(c.Manifest.ID)
	_, _, unknown := w.CollectionEnvironments("nope")

	if err != nil || len(diags) != 0 || len(got) != 1 || got[0].Name != "shared" {
		t.Fatalf("got %+v, diags %v, err %v", got, diags, err)
	}
	if !slices.Equal(got[0].Vars, []model.EnvVar{{Name: "base_url", Value: "http://shared.example"}, {Name: "token", Secret: true}}) {
		t.Fatalf("vars %+v", got[0].Vars)
	}
	if codes := codesOfErr(unknown); !slices.Equal(codes, []diag.Code{diag.CodeWSUnknownCollection}) {
		t.Fatalf("unknown collection: %v", unknown)
	}
}

func TestCollectionEnvironments_Given_no_environments_directory_Then_none(t *testing.T) {
	w := demoWorkspace(t)

	got, diags, err := w.CollectionEnvironments(w.Collections()[0].Manifest.ID)

	if err != nil || len(diags) != 0 || len(got) != 0 {
		t.Fatalf("got %+v, diags %v, err %v", got, diags, err)
	}
}

func TestShareEnvironment_Given_a_workspace_environment_When_shared_Then_the_collection_holds_it_without_secret_values(t *testing.T) {
	// Before: environments could be imported from a collection but never exported to one.
	w := demoWorkspace(t)
	c := w.Collections()[0]
	file, err := w.NewEnvironment("partner")
	if err != nil {
		t.Fatal(err)
	}
	envs := w.Environments()
	env := envs[slices.IndexFunc(envs, func(e model.Environment) bool { return e.File == file })]
	env.Colour = "blue"
	env.Vars = []model.EnvVar{{Name: "base_url", Value: "http://partner.example"}, {Name: "token", Secret: true}, {Name: "n", Value: "5", Kind: model.VarKind("number")}}
	if err := w.SaveEnvironment(env, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	shared, existed, notes, err := w.ShareEnvironment(c.Manifest.ID, "partner", false)
	got, _, readErr := w.CollectionEnvironments(c.Manifest.ID)

	if err != nil || existed || len(notes) != 0 || readErr != nil {
		t.Fatalf("err %v, existed %t, notes %v, read %v", err, existed, notes, readErr)
	}
	if shared != filepath.Join(c.Root, "environments", "partner.yaml") || len(got) != 1 || got[0].Name != "partner" || got[0].Colour != "blue" {
		t.Fatalf("file %s, got %+v", shared, got)
	}
	if got[0].ID == env.ID || got[0].Scope != "" {
		t.Errorf("the copy keeps the workspace id or scope: %+v", got[0])
	}
	if !slices.Equal(got[0].Vars, env.Vars) {
		t.Errorf("vars %+v, want %+v", got[0].Vars, env.Vars)
	}
}

func TestShareEnvironment_Given_a_file_already_there_When_shared_Then_it_is_kept_until_overwrite_is_set(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	if _, err := w.NewEnvironment("partner"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(c.Root, "environments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(dir, "partner.yaml")
	if err := os.WriteFile(mine, []byte(sharedEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	_, existed, _, err := w.ShareEnvironment(c.Manifest.ID, "partner", false)
	kept, _ := os.ReadFile(mine)
	_, existedAgain, _, errAgain := w.ShareEnvironment(c.Manifest.ID, "partner", true)
	replaced, _ := os.ReadFile(mine)

	if err != nil || !existed || string(kept) != sharedEnv {
		t.Fatalf("err %v, existed %t, file changed: %q", err, existed, kept)
	}
	if errAgain != nil || existedAgain || string(replaced) == sharedEnv {
		t.Fatalf("err %v, existed %t, file not replaced", errAgain, existedAgain)
	}
}

func TestShareEnvironment_Given_a_base_the_collection_lacks_When_shared_Then_a_note_names_it(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	for _, n := range []string{"base", "child"} {
		if _, err := w.NewEnvironment(n); err != nil {
			t.Fatal(err)
		}
	}
	envs := w.Environments()
	child := envs[slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == "child" })]
	child.Extends = "base"
	if err := w.SaveEnvironment(child, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	_, _, notes, err := w.ShareEnvironment(c.Manifest.ID, "child", false)

	if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "builds on base") {
		t.Fatalf("err %v, notes %v", err, notes)
	}
}

func TestShareEnvironment_Given_an_unknown_environment_or_collection_or_a_bad_name_When_shared_Then_it_is_refused(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	_, _, _, noEnv := w.ShareEnvironment(c.Manifest.ID, "nope", false)
	_, _, _, noColl := w.ShareEnvironment("nope", "x", false)
	_, _, _, badName := w.ShareEnvironment(c.Manifest.ID, "../x", false)

	if noEnv == nil || noColl == nil || badName == nil {
		t.Fatalf("refusals: %v / %v / %v", noEnv, noColl, badName)
	}
}
