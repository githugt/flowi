// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// editorFor returns a hook editor over a request whose own hooks are the
// given ones.
var errNotCalled = errors.New("save callback not called")

func editorFor(t *testing.T, hooks model.Hooks) (*vm.HookEditor, *vm.Request, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := sampleBackend()
	b.Cols[0].Requests[0].Hooks = hooks
	r, w := newRequestVM(t, b)
	return vm.NewHookEditor(b, w, r), r, b, w
}

func inline(ids ...string) model.HookStep {
	var nodes []model.Node
	for _, id := range ids {
		nodes = append(nodes, logNode(id))
	}
	return model.HookStep{Inline: nodes}
}

func TestHookEditor_should_start_with_the_requests_own_inline_nodes_and_not_be_dirty(t *testing.T) {
	e, _, _, _ := editorFor(t, model.Hooks{Pre: []model.HookStep{inline("a")}, Post: []model.HookStep{inline("b", "c"), inline("d")}})

	if got := ids(e.Doc(vm.PhasePre).Draft().Nodes); len(got) != 1 || got[0] != "a" {
		t.Fatalf("pre %v", got)
	}
	if got := ids(e.Doc(vm.PhasePost).Draft().Nodes); len(got) != 3 || got[2] != "d" {
		t.Fatalf("post %v", got)
	}
	if e.Dirty() {
		t.Fatal("dirty before any edit")
	}
}

func TestHookEditor_Given_an_edit_Then_is_dirty_and_save_writes_the_requests_hooks(t *testing.T) {
	e, r, b, w := editorFor(t, model.Hooks{Post: []model.HookStep{inline("b")}})
	e.Doc(vm.PhasePost).Insert("assert", "$.nodes", 1, nil)
	e.Doc(vm.PhasePre).Insert("set", "$.nodes", 0, nil)
	dirty := e.Dirty()

	saved := errNotCalled
	e.Save(false, func(err error) { saved = err })
	w.Wait()

	if !dirty || saved != nil || e.Dirty() {
		t.Fatalf("dirty %v, save %v, dirty after %v", dirty, saved, e.Dirty())
	}
	got := b.Saved[len(b.Saved)-1].Hooks
	if len(got.Pre) != 1 || len(got.Pre[0].Inline) != 1 || got.Pre[0].Inline[0].Type != "set" {
		t.Fatalf("pre %+v", got.Pre)
	}
	if len(got.Post) != 1 || len(got.Post[0].Inline) != 2 || got.Post[0].Inline[1].Type != "assert" {
		t.Fatalf("post %+v", got.Post)
	}
	if r.Dirty() {
		t.Fatal("the request is still dirty after the save")
	}
}

func TestHookEditor_Given_flow_steps_Then_they_are_kept_where_they_were(t *testing.T) {
	flow := model.HookStep{Flow: &model.Ref{Path: "hooks/sign"}}
	e, _, b, w := editorFor(t, model.Hooks{Pre: []model.HookStep{flow, inline("a"), flow}})
	e.Doc(vm.PhasePre).Insert("log", "$.nodes", 1, nil)

	e.Save(false, func(error) {})
	w.Wait()

	pre := b.Saved[len(b.Saved)-1].Hooks.Pre
	if len(pre) != 3 || pre[0].Flow == nil || len(pre[1].Inline) != 2 || pre[2].Flow == nil {
		t.Fatalf("pre %+v", pre)
	}
}

func TestHookEditor_Given_every_node_deleted_Then_the_inline_step_goes(t *testing.T) {
	e, _, b, w := editorFor(t, model.Hooks{Post: []model.HookStep{inline("b")}})
	e.Doc(vm.PhasePost).Delete("b")

	e.Save(false, func(error) {})
	w.Wait()

	if post := b.Saved[len(b.Saved)-1].Hooks.Post; len(post) != 0 {
		t.Fatalf("post %+v", post)
	}
}

func TestHookEditor_should_count_the_hooks_it_does_not_edit(t *testing.T) {
	flow := model.HookStep{Flow: &model.Ref{Path: "hooks/sign"}}
	e, _, _, _ := editorFor(t, model.Hooks{Pre: []model.HookStep{flow, inline("a")}})

	if got := e.Others(vm.PhasePre); got != 1 {
		t.Fatalf("others %d, want the one flow step", got)
	}
}

func TestHookEditor_should_move_a_node_between_phases(t *testing.T) {
	e, _, _, _ := editorFor(t, model.Hooks{Pre: []model.HookStep{inline("a")}, Post: []model.HookStep{inline("b")}})

	e.MoveNode(vm.PhasePre, "a", vm.PhasePost, 0)

	pre, post := ids(e.Doc(vm.PhasePre).Draft().Nodes), ids(e.Doc(vm.PhasePost).Draft().Nodes)
	if len(pre) != 0 || len(post) != 2 || post[1] != "b" || !e.Dirty() {
		t.Fatalf("pre %v post %v", pre, post)
	}
}

func hookBlueprints() []model.Blueprint {
	return []model.Blueprint{
		{Meta: model.Meta{ID: "BP-POST", Name: "Checks"}, Path: "checks", Body: model.Flow{Nodes: []model.Node{logNode("c1"), logNode("c2")}}},
		{Meta: model.Meta{ID: "BP-PRE", Name: "Stamp"}, Path: "stamp", Phase: "pre", Body: model.Flow{Nodes: []model.Node{logNode("s1")}}},
		{Meta: model.Meta{ID: "BP-ARGS", Name: "Needs a request"}, Path: "needs", Params: []model.Param{{Name: "r", Type: model.ParamRequestRef}}},
	}
}

func editorWithBlueprints(t *testing.T, hooks model.Hooks) (*vm.HookEditor, *vm.Workspace, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := sampleBackend()
	b.Cols[0].Blueprints = hookBlueprints()
	b.Cols[0].Requests[0].Hooks = hooks
	ws, w := loadedWorkspace(t, b)
	r := vm.NewRequest(b, w, b.Cols[0], b.Cols[0].Requests[0])
	return vm.NewHookEditor(b, w, r), ws, b, w
}

func TestHookEditor_Blueprints_should_offer_only_those_without_parameters_with_their_phase(t *testing.T) {
	e, ws, _, _ := editorWithBlueprints(t, model.Hooks{})

	got := e.Blueprints(ws)

	if len(got) != 2 || got[0].Phase != "post" || got[1].Phase != "pre" {
		t.Fatalf("got %+v", got)
	}
}

func TestHookEditor_InsertBlueprint_should_append_its_nodes_to_its_phase_with_free_ids(t *testing.T) {
	e, ws, _, _ := editorWithBlueprints(t, model.Hooks{Post: []model.HookStep{inline("c1")}})

	e.InsertBlueprint(e.Blueprints(ws)[0])
	e.InsertBlueprint(e.Blueprints(ws)[1])

	post, pre := ids(e.Doc(vm.PhasePost).Draft().Nodes), ids(e.Doc(vm.PhasePre).Draft().Nodes)
	if len(post) != 3 || post[0] != "c1" || post[1] == "c1" || post[1] == post[2] || len(pre) != 1 || !e.Dirty() {
		t.Fatalf("pre %v post %v", pre, post)
	}
}

func TestHookEditor_SaveAsBlueprint_should_store_one_phase_and_optionally_make_it_the_default(t *testing.T) {
	e, ws, b, w := editorWithBlueprints(t, model.Hooks{Pre: []model.HookStep{inline("a")}, Post: []model.HookStep{inline("b")}})
	var done bool

	e.SaveAsBlueprint(ws, vm.PhasePre, "hooks/mine", "Mine", true, func(string) { done = true }, func(err error) { t.Fatal(err) })
	w.Wait()

	if !done || !b.Called("CreateHookBlueprint C1 hooks/mine Mine pre 1") {
		t.Fatalf("done %v calls %v", done, b.Calls)
	}
	// The built-in default keeps the post phase; the new one takes pre.
	if !b.Called("SetUserSetting requests.seed_blueprint=default, hooks/mine") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestHookEditor_SaveAsBlueprint_Given_a_phase_with_no_nodes_Then_refuses_without_calling_the_backend(t *testing.T) {
	e, ws, b, w := editorWithBlueprints(t, model.Hooks{})
	var failed error

	e.SaveAsBlueprint(ws, vm.PhasePost, "x", "X", false, nil, func(err error) { failed = err })
	w.Wait()

	if failed == nil || b.Called("CreateHookBlueprint C1 x X post 0") {
		t.Fatalf("failed %v calls %v", failed, b.Calls)
	}
}

func TestSeedAfterAdding_should_replace_the_entry_of_the_same_phase_and_keep_the_other(t *testing.T) {
	known := []vm.BlueprintChoice{{Path: "checks", Phase: "post"}, {Path: "stamp", Phase: "pre"}}

	cases := []struct{ current, add, phase, want string }{
		{"default", "stamp", "pre", "default, stamp"},
		{"default", "checks", "post", "checks"},
		{"checks, stamp", "mine", "post", "stamp, mine"},
		{"off", "stamp", "pre", "default, stamp"},
		{"", "mine", "post", "mine"},
	}
	for _, c := range cases {
		if got := vm.SeedAfterAdding(c.current, c.add, c.phase, known); got != c.want {
			t.Errorf("SeedAfterAdding(%q, %q, %q) = %q, want %q", c.current, c.add, c.phase, got, c.want)
		}
	}
}

func TestHookEditor_Given_repeated_node_ids_across_inline_steps_Then_they_are_made_unique(t *testing.T) {
	e, _, b, w := editorFor(t, model.Hooks{Post: []model.HookStep{inline("response"), inline("response")}})

	got := ids(e.Doc(vm.PhasePost).Draft().Nodes)
	e.Save(false, func(error) {})
	w.Wait()

	if len(got) != 2 || got[0] == got[1] {
		t.Fatalf("ids %v", got)
	}
	saved := b.Saved[len(b.Saved)-1].Hooks.Post
	if len(saved) != 1 || saved[0].Inline[0].ID == saved[0].Inline[1].ID {
		t.Fatalf("saved %+v", saved)
	}
}

func TestHookEditor_InsertBlueprint_Given_the_same_blueprint_twice_Then_ids_and_references_stay_unique_and_consistent(t *testing.T) {
	bp := vm.BlueprintChoice{Phase: "post", Nodes: []model.Node{
		logNode("first"),
		{ID: "second", Type: "log", Needs: []string{"first"}, Params: []model.Field{{Key: "value", Value: "{{nodes.first.x}}"}}},
	}}
	e, _, _, _ := editorFor(t, model.Hooks{})

	e.InsertBlueprint(bp)
	e.InsertBlueprint(bp)

	nodes := e.Doc(vm.PhasePost).Draft().Nodes
	seen := map[string]bool{}
	for _, n := range nodes {
		if seen[n.ID] {
			t.Fatalf("duplicate id %q in %v", n.ID, ids(nodes))
		}
		seen[n.ID] = true
	}
	copyOfSecond := nodes[3]
	if copyOfSecond.Needs[0] != nodes[2].ID || copyOfSecond.Params[0].Value != "{{nodes."+nodes[2].ID+".x}}" {
		t.Fatalf("the second copy refers to %v / %v, want %s", copyOfSecond.Needs, copyOfSecond.Params, nodes[2].ID)
	}
	if bp.Nodes[1].Needs[0] != "first" {
		t.Fatal("the blueprint itself was changed")
	}
}

// Before: the hooks went into the request draft before the write, so a failed
// save left them there while the editor said the changes were unsaved.
func TestHookEditor_Given_a_failed_save_Then_the_request_keeps_its_hooks_and_the_editor_stays_dirty(t *testing.T) {
	e, r, b, w := editorFor(t, model.Hooks{Post: []model.HookStep{inline("b")}})
	e.Doc(vm.PhasePost).Insert("assert", "$.nodes", 1, nil)
	b.SaveErr = errBoom
	var got error

	e.Save(false, func(err error) { got = err })
	w.Wait()

	if got == nil || !e.Dirty() || len(r.Hooks().Post[0].Inline) != 1 {
		t.Fatalf("err %v dirty %v request hooks %+v", got, e.Dirty(), r.Hooks().Post)
	}
}

func TestHookEditor_Given_an_edit_made_while_saving_Then_it_stays_unsaved(t *testing.T) {
	e, _, _, _ := editorFor(t, model.Hooks{Post: []model.HookStep{inline("b")}})
	doc := e.Doc(vm.PhasePost)
	doc.Insert("assert", "$.nodes", 1, nil)
	snapshot, rev := doc.Snapshot()
	doc.Insert("set", "$.nodes", 2, nil) // typed while the write was running

	doc.MarkSavedAt(snapshot, rev)

	if !doc.Dirty() {
		t.Fatal("an edit made during the save was marked saved")
	}
}

func TestHookEditor_Given_edits_after_a_save_Then_the_requests_hooks_do_not_change(t *testing.T) {
	e, r, _, w := editorFor(t, model.Hooks{})
	e.Doc(vm.PhasePost).Insert("log", "$.nodes", 0, nil)
	e.Save(false, func(error) {})
	w.Wait()

	e.Doc(vm.PhasePost).EditNode(e.Doc(vm.PhasePost).Draft().Nodes[0].ID, func(n *model.Node) { n.Name = "changed" })

	if got := r.Hooks().Post[0].Inline[0].Name; got != "" {
		t.Fatalf("the request's hooks changed to %q without a save", got)
	}
	if r.Dirty() {
		t.Fatal("the request is dirty from an edit in the overlay")
	}
}

func TestSeedAfterAdding_should_recognise_an_entry_written_as_a_file_path(t *testing.T) {
	known := []vm.BlueprintChoice{{Path: "stamp", Phase: "pre"}}

	got := vm.SeedAfterAdding("default, blueprints/stamp.blueprint.yaml", "newer", "pre", known)

	if got != "default, newer" {
		t.Fatalf("got %q", got)
	}
}
