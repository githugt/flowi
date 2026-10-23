// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
	"github.com/githugt/flowi-lib/workspace"
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

	if got := envNames(again); !slices.Equal(got, []string{"shared"}) {
		t.Fatalf("environments = %v, want only the global one", got)
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
