// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestFormat_should_return_no_changes_for_canonical_tree(t *testing.T) {
	s := newStore()
	c, _, err := s.Load(copyTree(t, "basic"))
	if err != nil {
		t.Fatal(err)
	}

	changes, err := s.Format(c)

	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range changes {
		t.Errorf("unexpected change to %s:\n%s", ch.File, ch.New)
	}
}

func TestFormat_should_repair_stale_path_hint(t *testing.T) {
	root := copyTree(t, "basic")
	flow := filepath.Join(root, "flows", "journeys", "signup.arazzo.yaml")
	data, _ := os.ReadFile(flow)
	writeFile(t, flow, strings.Replace(string(data), "path: auth/login", "path: auth/old-login", 1))
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	changes, err := s.Format(c)

	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !strings.Contains(string(changes[0].New), "path: auth/login") {
		t.Fatalf("got %+v", changes)
	}
	if c.Flows[0].Nodes[0].Request.Request.Path != "auth/old-login" {
		t.Fatal("Format mutated the loaded collection")
	}
}

func TestRepairRefs_should_use_resolver_for_every_reference(t *testing.T) {
	c := model.Collection{
		Manifest: model.Manifest{Hooks: model.Hooks{Pre: []model.HookStep{{Flow: &model.Ref{ID: "F", Path: "old"}}}}},
		Flows: []model.Flow{{Nodes: []model.Node{{
			Type: "batch",
			Body: []model.Node{{Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: "F", Path: "old"}}}},
		}}}},
	}

	got := RepairRefs(c, func(r model.Ref) (string, bool) { return "new", r.ID == "F" })

	if got.Manifest.Hooks.Pre[0].Flow.Path != "new" || got.Flows[0].Nodes[0].Body[0].Subflow.Flow.Path != "new" {
		t.Fatalf("refs not repaired: %+v", got)
	}
	if c.Manifest.Hooks.Pre[0].Flow.Path != "old" || c.Flows[0].Nodes[0].Body[0].Subflow.Flow.Path != "old" {
		t.Fatal("RepairRefs mutated its input")
	}
}

func TestApply_should_write_atomically_and_preserve_mode(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.yaml")
	writeFile(t, existing, "old\n")
	if err := os.Chmod(existing, 0o600); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(dir, "sub", "b.yaml")

	err := Apply([]FileChange{
		{File: existing, Old: []byte("old\n"), New: []byte("new\n")},
		{File: created, New: []byte("fresh\n")},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(existing); string(got) != "new\n" {
		t.Fatalf("existing = %q", got)
	}
	if got, _ := os.ReadFile(created); string(got) != "fresh\n" {
		t.Fatalf("created = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".flowi-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(existing)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v, want 0600", info.Mode().Perm())
		}
		info, _ = os.Stat(created)
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("new file mode %v, want 0644", info.Mode().Perm())
		}
	}
}

func TestInit_should_create_layout_and_refuse_existing_collection(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-collection")
	s := newStore()
	m := model.Manifest{Format: 1, Meta: model.Meta{ID: "01J9Z8K3M4N5P6Q7R8S9T0VWXY", Name: "New"}}

	err := s.Init(dir, m)
	again := s.Init(dir, m)

	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"requests", "flows"} {
		if info, err := os.Stat(filepath.Join(dir, sub)); err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", sub, err)
		}
	}
	// Behaviour change: environments are global to the workspace, so a new
	// collection no longer gets an environments directory that nothing reads.
	if _, err := os.Stat(filepath.Join(dir, "environments")); err == nil {
		t.Fatal("Init created an environments directory")
	}
	c, diags, err := s.Load(dir)
	if err != nil || len(diags) != 0 || c.Manifest.Name != "New" {
		t.Fatalf("reload: %v %v %+v", err, codesOf(diags), c.Manifest)
	}
	if !errors.Is(again, ErrAlreadyCollection) {
		t.Fatalf("second Init: %v", again)
	}
}

func TestFormat_should_flag_changes_that_drop_comments(t *testing.T) {
	root := copyTree(t, "basic")
	login := filepath.Join(root, "requests", "auth", "login.req.yaml")
	data, _ := os.ReadFile(login)
	writeFile(t, login, "# owned by the auth team\n"+string(data))
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	changes, err := s.Format(c)

	if err != nil || len(changes) != 1 || !changes[0].DropsComments {
		t.Fatalf("err %v changes %+v", err, changes)
	}
}

func TestFormat_should_rewrite_a_blueprint_that_is_not_canonical(t *testing.T) {
	root := copyTree(t, "basic")
	file := filepath.Join(root, "blueprints", "standard.blueprint.yaml")
	writeFile(t, file, "name: Standard onboarding\nid: 01J9ZH3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  api: {description: The request to call, type: request-ref}\nnodes:\n  - {id: call, type: request, request: {param: api}}\n")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	changes, err := s.Format(c)

	if err != nil || len(changes) != 1 || filepath.Base(changes[0].File) != "standard.blueprint.yaml" {
		t.Fatalf("changes %d err %v", len(changes), err)
	}
	if !strings.Contains(string(changes[0].New), "request:\n      param: api") {
		t.Fatalf("new content:\n%s", changes[0].New)
	}
}
