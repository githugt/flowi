// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/examples"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"

	"github.com/githugt/flowi/app/internal/backend"
	"github.com/githugt/flowi/app/internal/uithread"
	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

var idLine = regexp.MustCompile(`(?m)(x-flowi-flow:\n +)id: .*$`)

// The roadmap's G2 acceptance: the example flow built from scratch with
// canvas commands saves to the same YAML as the hand-written file (ids of
// new files are generated, so the flow's own id line is set aside).
func TestFlowDoc_golden_rebuild_of_user_creation(t *testing.T) {
	want, err := fs.ReadFile(examples.UserCreation(), "flows/user-creation.arazzo.yaml")
	if err != nil {
		t.Fatal(err)
	}
	b, cid, flowFile := workspaceWithoutUserCreation(t)
	w := vmtest.SyncWorker()
	target, err := b.NewFlow(cid, "user-creation")
	if err != nil {
		t.Fatal(err)
	}
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	c, _ := ws.Collection(cid)
	var f model.Flow
	for _, x := range c.Flows {
		if x.ID == target.Flow {
			f = x
		}
	}
	d := vm.NewFlowDoc(b, w, c, f)
	w.Wait()

	buildUserCreation(d)
	d.Save(false)
	w.Wait()

	got, _ := os.ReadFile(flowFile)
	if d.Err() != "" || idLine.ReplaceAllString(string(got), "${1}id: X") != idLine.ReplaceAllString(string(want), "${1}id: X") {
		t.Fatalf("err %q; got:\n%s\nwant:\n%s", d.Err(), got, want)
	}
}

// buildUserCreation makes the edits a user makes on the canvas: flow
// properties in the inspector, nodes from the palette and the tree, and
// their fields in the inspector.
func buildUserCreation(d *vm.FlowDoc) {
	d.EditFlow(func(f *model.Flow) {
		f.Name, f.Description, f.PreferVersion = "User creation journey", "Login, verify, then fill both onboarding pages.", "2"
		f.Inputs = []model.Input{{Name: "email", Default: "{{$email}}"}}
	})
	request := func(id string, req model.Ref, with ...model.KeyValue) {
		got := d.Insert("request", "$.nodes", 99, func(n *model.Node) { n.Request = &model.RequestCall{Request: req} })
		d.EditNode(got, func(n *model.Node) { n.ID = id; n.Request.With = with })
	}
	action := func(typ model.NodeType, id string, params ...model.Field) {
		got := d.Insert(typ, "$.nodes", 99, nil)
		d.EditNode(got, func(n *model.Node) { n.ID = id; n.Params = params })
	}
	values := func(id string, vs ...model.SetValue) {
		got := d.Insert("set", "$.nodes", 99, nil)
		d.EditNode(got, func(n *model.Node) { n.ID = id; n.Values = vs })
	}
	token, subject := model.KeyValue{Key: "token", Value: "{{vars.token}}"}, model.KeyValue{Key: "subject", Value: "{{vars.subject}}"}
	request("login", model.Ref{ID: "01JA0000000000000000000010", Path: "auth/login"}, model.KeyValue{Key: "email", Value: "{{inputs.email}}"})
	values("session", model.SetValue{Name: "token", Value: "{{nodes.login.token}}"}, model.SetValue{Name: "subject", Value: "{{nodes.login.subject}}"})
	action("delay", "wait-for-code", model.Field{Key: "duration", Value: "{{env.verify_wait}}"})
	request("verify", model.Ref{ID: "01JA0000000000000000000011", Path: "auth/verify"}, token, subject, model.KeyValue{Key: "code", Value: "123456"})
	values("profile", model.SetValue{Name: "first_name", From: "generate", Value: "{{$first-name}}"}, model.SetValue{Name: "last_name", From: "generate", Value: "{{$last-name}}"},
		model.SetValue{Name: "city", From: "generate", Value: "{{$city-name}}"})
	request("page-1", model.Ref{ID: "01JA0000000000000000000012", Path: "onboarding/form-page-1"}, token, subject,
		model.KeyValue{Key: "first_name", Value: "{{vars.first_name}}"}, model.KeyValue{Key: "last_name", Value: "{{vars.last_name}}"})
	request("page-2", model.Ref{ID: "01JA0000000000000000000013", Path: "onboarding/form-page-2"}, token, subject,
		model.KeyValue{Key: "city", Value: "{{vars.city}}"})
}

// workspaceWithoutUserCreation opens the example collection minus its
// user-creation flow, and returns where that flow's file belongs.
func workspaceWithoutUserCreation(t *testing.T) (vm.Backend, model.ID, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "example")
	if err := os.CopyFS(root, examples.UserCreation()); err != nil {
		t.Fatal(err)
	}
	flowFile := filepath.Join(root, "flows", "user-creation.arazzo.yaml")
	if err := os.Remove(flowFile); err != nil {
		t.Fatal(err)
	}
	environ := []string{"FLOWI_CONFIG_DIR=" + filepath.Join(dir, "config"), "FLOWI_VAULT_KEY=test-only"}
	paths, _ := settings.DiscoverPaths(environ)
	ws, _, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{Paths: paths, Environ: environ})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ws.Mount(root)
	if err != nil {
		t.Fatal(err)
	}
	return backend.New(ws, uithread.NewGuard(true)), m.ID, flowFile
}
