// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func flowFixture() (*vmtest.Backend, model.Flow) {
	c := vmtest.SampleCollection()
	f := model.Flow{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{
		{ID: "a", Type: "set", Params: []model.Field{{Key: "target", Value: "vars.x"}, {Key: "value", Value: "1"}}},
		{ID: "b", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}},
		{ID: "p", Type: "parallel", Branches: []model.Branch{
			{ID: "b1", Nodes: []model.Node{{ID: "x", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}}}},
			{ID: "b2", Nodes: []model.Node{{ID: "y", Type: "log"}}},
		}},
	}}
	c.Flows = []model.Flow{f}
	return &vmtest.Backend{Cols: []model.Collection{c}}, f
}

func newFlowDoc(t *testing.T) (*vm.FlowDoc, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b, f := flowFixture()
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()
	return d, b, w
}

func ids(list []model.Node) []string {
	var out []string
	for _, n := range list {
		out = append(out, n.ID)
	}
	return out
}

func TestFlowDoc_insert_with_unique_ids_then_undo_redo(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	first := d.Insert("log", "$.nodes", 1, nil)
	second := d.Insert("log", "$.nodes[3].branches[1].nodes", 0, nil) // p is now at 3
	afterInsert := ids(d.Draft().Nodes)
	d.Undo()
	d.Undo()
	afterUndo := ids(d.Draft().Nodes)
	d.Redo()

	if first != "log" || second != "log-2" || !slices.Equal(afterInsert, []string{"a", "log", "b", "p"}) {
		t.Fatalf("ids %q %q, nodes %v", first, second, afterInsert)
	}
	if !slices.Equal(afterUndo, []string{"a", "b", "p"}) || !slices.Equal(ids(d.Draft().Nodes), []string{"a", "log", "b", "p"}) || !d.CanRedo() { // one redo is left
		t.Fatalf("after undo %v, after redo %v, can redo %v", afterUndo, ids(d.Draft().Nodes), d.CanRedo())
	}
}

func TestFlowDoc_delete_move_and_needs_are_undoable(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	if err := d.Move("y", "$.nodes", 0); err != nil {
		t.Fatal(err)
	}
	moved := ids(d.Draft().Nodes)
	d.SetNeeds("b", []string{"y"})
	d.Delete("a")
	deleted := ids(d.Draft().Nodes)
	d.Undo()
	d.Undo()
	d.Undo()

	if !slices.Equal(moved, []string{"y", "a", "b", "p"}) || !slices.Equal(deleted, []string{"y", "b", "p"}) {
		t.Fatalf("moved %v deleted %v", moved, deleted)
	}
	if got := ids(d.Draft().Nodes); !slices.Equal(got, []string{"a", "b", "p"}) || len(d.Draft().Nodes[2].Branches[1].Nodes) != 1 {
		t.Fatalf("after undo %v", got)
	}
}

func TestFlowDoc_delete_clears_needs_pointing_at_the_deleted_node(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	d.SetNeeds("p", []string{"a"}) // a is not the node before p, so this is a real wait

	d.Delete("a")

	if needs := d.Draft().Nodes[1].Needs; len(needs) != 0 {
		t.Fatalf("needs %v still name the deleted node", needs)
	}
}

func TestFlowDoc_edit_node_and_flow_are_undoable(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	d.EditNode("b", func(n *model.Node) { n.Name = "Greeting" })
	d.EditFlow(func(f *model.Flow) { f.PreferVersion = "2" })
	d.Undo()

	if d.Draft().Nodes[1].Name != "Greeting" || d.Draft().PreferVersion != "" || !d.Dirty() {
		t.Fatalf("draft %+v", d.Draft())
	}
}

func TestFlowDoc_positions_are_not_undoable_and_relayout_clears_them(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	d.SetPosition("p", model.Point{X: 500, Y: 300})
	moved, _ := d.Diagram().Unit("p")
	canUndo := d.CanUndo()
	d.Relayout(nil)
	back, _ := d.Diagram().Unit("p")

	if moved.X != 500 || moved.Y != 300 || canUndo || back.X == 500 || !d.Dirty() {
		t.Fatalf("moved %+v back %+v canUndo %v", moved, back, canUndo)
	}
}

func TestFlowDoc_save_writes_flow_and_layout(t *testing.T) {
	d, b, w := newFlowDoc(t)
	d.Insert("note", "$.nodes", 0, nil)
	d.SetPosition("a", model.Point{X: 1, Y: 2})

	d.Save(false)
	w.Wait()

	if d.Dirty() || len(b.SavedFlows) != 1 || b.SavedFlows[0].Nodes[0].ID != "note" || b.Layout.Nodes["a"] != (model.Point{X: 1, Y: 2}) {
		t.Fatalf("dirty %v saved %+v layout %+v", d.Dirty(), b.SavedFlows, b.Layout)
	}
}

func TestFlowDoc_undo_after_save_still_works_and_marks_dirty(t *testing.T) {
	d, _, w := newFlowDoc(t)
	d.Delete("a")
	d.Save(false)
	w.Wait()

	d.Undo()

	if !d.Dirty() || ids(d.Draft().Nodes)[0] != "a" {
		t.Fatalf("dirty %v nodes %v", d.Dirty(), ids(d.Draft().Nodes))
	}
}

func TestFlowDoc_Given_comments_Then_asks_once(t *testing.T) {
	d, b, w := newFlowDoc(t)
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "x"})}.Err()
	asked := 0
	d.OnDropConfirm(func() { asked++ })
	d.Delete("a")

	d.Save(false)
	w.Wait()
	d.ConfirmDrop()
	w.Wait()

	if asked != 1 || d.Dirty() || len(b.SavedFlows) != 1 {
		t.Fatalf("asked %d dirty %v saved %d", asked, d.Dirty(), len(b.SavedFlows))
	}
}

func TestFlowDoc_reload_while_dirty_flags_and_clean_takes_it(t *testing.T) {
	d, b, _ := newFlowDoc(t)
	disk := b.Cols[0].Flows[0]
	disk.Description = "changed elsewhere"

	d.Delete("a")
	d.Reloaded(b.Cols[0], disk, true)
	flagged := d.ChangedOnDisk()
	d.Revert()

	if !flagged || d.Draft().Description != "changed elsewhere" || len(d.Draft().Nodes) != 3 || d.CanUndo() {
		t.Fatalf("flagged %v draft %+v canUndo %v", flagged, d.Draft(), d.CanUndo())
	}
}

func TestFlowDoc_list_of_node_and_node_path(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	list, i, ok := d.Locate("x")

	if !ok || list != "$.nodes[2].branches[0].nodes" || i != 0 || d.NodePath("x") != "$.nodes[2].branches[0].nodes[0]" {
		t.Fatalf("locate %q %d %v path %q", list, i, ok, d.NodePath("x"))
	}
}

func TestFlowDoc_merged_edits_are_one_undo_step(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	for _, s := range []string{"h", "he", "hel"} {
		d.EditNodeMerged("b/value", "b", func(n *model.Node) { n.Params = []model.Field{{Key: "value", Value: s}} })
	}
	d.EditNode("a", func(n *model.Node) { n.Name = "A" })
	d.Undo()
	d.Undo()

	n, _ := d.Node("b")
	if n.Params[0].Value != "hi" || d.CanUndo() {
		t.Fatalf("value %v canUndo %v", n.Params[0].Value, d.CanUndo())
	}
}

func TestFlowDoc_rename_updates_needs_and_refuses_duplicates(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	d.SetNeeds("p", []string{"a"})

	errDup := d.Rename("a", "b")
	errEmpty := d.Rename("a", "")
	err := d.Rename("a", "start")

	n, _ := d.Node("p")
	if errDup == nil || errEmpty == nil || err != nil || !slices.Equal(n.Needs, []string{"start"}) || ids(d.Draft().Nodes)[0] != "start" {
		t.Fatalf("errs %v %v %v needs %v", errDup, errEmpty, err, n.Needs)
	}
}

// Before: the destination path was resolved after removing the source, so
// a later sibling's container had moved up one index.
func TestFlowDoc_move_into_a_later_siblings_container(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	err := d.Move("a", "$.nodes[2].branches[1].nodes", 0) // p is at index 2 before the move

	p, _ := d.Node("p")
	if err != nil || len(d.Draft().Nodes) != 2 || ids(p.Branches[1].Nodes)[0] != "a" {
		t.Fatalf("err %v nodes %v branch %v", err, ids(d.Draft().Nodes), ids(p.Branches[1].Nodes))
	}
}

// Before: a save that finished after more typing marked the document
// clean, so the next reload threw the typing away.
func TestFlowDoc_edit_during_save_stays_dirty(t *testing.T) {
	b, f := flowFixture()
	var release chan struct{}
	w := vm.NewWorker(func(fn func()) { fn() })
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()
	release = make(chan struct{})
	b.SaveGate = release
	d.EditNode("a", func(n *model.Node) { n.Name = "one" })

	d.Save(false)
	d.EditNode("a", func(n *model.Node) { n.Name = "two" })
	close(release)
	w.Wait()

	if !d.Dirty() {
		t.Fatal("edits made during the save were marked saved")
	}
}

func TestFlowDoc_rename_keeps_position_and_collapsed_state(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	d.SetPosition("a", model.Point{X: 700, Y: 50})
	d.SetCollapsed("a", true)

	_ = d.Rename("a", "start")

	u, _ := d.Diagram().Unit("start")
	if u.X != 700 || !d.Collapsed("start") {
		t.Fatalf("unit %+v collapsed %v", u, d.Collapsed("start"))
	}
}

func TestFlowDoc_Mermaid_should_export_the_draft_with_unsaved_edits(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	d.Insert("delay", "$.nodes", 0, func(n *model.Node) { n.Params = []model.Field{{Key: "duration", Value: "3s"}} })

	got := d.Mermaid()

	if !strings.HasPrefix(got, "flowchart LR\n") || !strings.Contains(got, `["delay: 3s"]`) {
		t.Fatalf("got:\n%s", got)
	}
}

func TestFlowDoc_Arazzo_should_export_the_saved_flow_with_a_note_for_what_has_no_equivalent(t *testing.T) {
	d, b, w := newFlowDoc(t)
	b.ArazzoNotes = diag.List{diag.New(diag.CodeServeSchedule, diag.Location{}, map[string]string{"name": "x", "detail": "no equivalent"})}
	var text string
	var notes []string

	d.Arazzo(func(t string, n []string) { text, notes = t, n }, func(err error) { t.Fatal(err) })
	w.Wait()

	if text != "arazzo: 1.0.1\n" || len(notes) != 1 || notes[0] == "" || !b.Called("ExportArazzo F1") {
		t.Fatalf("text %q notes %v calls %v", text, notes, b.Calls)
	}
}

func TestFlowDoc_Arazzo_Given_the_export_fails_Then_reports_it(t *testing.T) {
	d, b, w := newFlowDoc(t)
	b.ArazzoErr = errors.New("no flow")
	var failed error

	d.Arazzo(func(string, []string) { t.Fatal("exported") }, func(err error) { failed = err })
	w.Wait()

	if failed == nil {
		t.Fatal("the failure was not reported")
	}
}

func TestFlowDoc_should_know_which_steps_only_Flow_I_can_run(t *testing.T) {
	b, f := flowFixture()
	b.Compat = diag.List{
		diag.New(diag.CodeArazzoCustom, diag.Location{}, map[string]string{"step": "a", "type": "set", "flow": "signup"}),
		diag.New(diag.CodeArazzoCustom, diag.Location{}, map[string]string{"step": "p", "type": "parallel", "flow": "signup"}),
	}
	w := vmtest.SyncWorker()

	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	if got := d.FlowIOnlySteps(); !slices.Equal(got, []string{"a (set)", "p (parallel)"}) {
		t.Fatalf("got %v", got)
	}
}

// Behaviour change: moving a node used to leave the needs of the old chain
// (each node needing the one before) pointing at nodes that were now later, so
// the canvas lost the arrows into and out of the moved node.
func TestFlowDoc_Move_should_keep_a_chain_of_needs_connected(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{
		{ID: "a", Type: "log"},
		{ID: "b", Type: "log", Needs: []string{"a"}},
		{ID: "c", Type: "log", Needs: []string{"b"}},
	}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	if err := d.Move("b", "$.nodes", 3); err != nil { // after c
		t.Fatal(err)
	}

	got := d.Draft().Nodes
	if !slices.Equal(ids(got), []string{"a", "c", "b"}) || len(got[0].Needs) != 0 || len(got[1].Needs) != 0 || len(got[2].Needs) != 0 {
		t.Fatalf("nodes %+v", got)
	}
}

func TestFlowDoc_Move_should_keep_needs_that_still_point_back(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{
		{ID: "a", Type: "log"},
		{ID: "b", Type: "log"},
		{ID: "join", Type: "log", Needs: []string{"a", "b"}},
	}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	if err := d.Move("b", "$.nodes", 1); err != nil { // where it already is
		t.Fatal(err)
	}

	if got := d.Draft().Nodes[2].Needs; !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("needs %v", got)
	}
}

func TestFlowDoc_RemoveCase_should_remove_an_empty_case_and_keep_one_with_steps(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "cond", Type: "condition", Cases: []model.Case{
		{When: "x == 1"}, {When: "x == 2", Nodes: []model.Node{{ID: "s", Type: "log"}}},
	}}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	d.RemoveCase("cond", 1) // has a step: kept, or the step would be deleted with it
	d.RemoveCase("cond", 0)

	cases := d.Draft().Nodes[0].Cases
	if len(cases) != 1 || cases[0].When != "x == 2" {
		t.Fatalf("cases %+v", cases)
	}
}

func TestFlowDoc_RemoveBranch_should_remove_only_an_empty_branch(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "par", Type: "parallel", Branches: []model.Branch{{ID: "b1"}, {ID: "b2", Nodes: []model.Node{{ID: "s", Type: "log"}}}}}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	d.RemoveBranch("par", 1)
	d.RemoveBranch("par", 0)

	branches := d.Draft().Nodes[0].Branches
	if len(branches) != 1 || branches[0].ID != "b2" {
		t.Fatalf("branches %+v", branches)
	}
}

func TestFlowDoc_Source_should_show_the_draft_with_unsaved_edits(t *testing.T) {
	d, b, w := newFlowDoc(t)
	d.Insert("log", "$.nodes", 0, nil)
	var flowText, fileText string

	d.Source(func(flow, file string) { flowText, fileText = flow, file }, func(err error) { t.Fatal(err) })
	w.Wait()

	if flowText != "nodes: 4\n" || fileText != "arazzo: 1.1.0\n" || !b.Called("FlowSource F1") {
		t.Fatalf("flow %q file %q calls %v", flowText, fileText, b.Calls)
	}
}

func TestFlowDoc_ApplySource_should_replace_the_draft_and_be_undoable(t *testing.T) {
	d, b, w := newFlowDoc(t)
	edited := cloneOf(d.Draft())
	edited.Nodes = edited.Nodes[:1]
	b.ParsedFlow = &edited
	var problems []string

	d.ApplySource("nodes: []\n", func(p, _ []string) { problems = p }, func(err error) { t.Fatal(err) })
	w.Wait()
	applied := len(d.Draft().Nodes)
	dirty := d.Dirty()
	d.Undo()

	if len(problems) != 0 || applied != 1 || !dirty || len(d.Draft().Nodes) != 3 {
		t.Fatalf("problems %v applied %d dirty %v after undo %d", problems, applied, dirty, len(d.Draft().Nodes))
	}
}

func TestFlowDoc_ApplySource_Given_problems_Then_lists_them_with_lines_and_keeps_the_draft(t *testing.T) {
	d, b, w := newFlowDoc(t)
	b.SourceDiags = diag.List{diag.New(diag.CodeServeSchedule, diag.Location{Pos: diag.Position{Line: 3, Column: 5}}, map[string]string{"name": "x", "detail": "bad"})}
	var problems []string

	d.ApplySource("nodes: [oops\n", func(p, _ []string) { problems = p }, func(err error) { t.Fatal(err) })
	w.Wait()

	if len(problems) != 1 || !strings.HasPrefix(problems[0], "line 3: ") || d.Dirty() || len(d.Draft().Nodes) != 3 {
		t.Fatalf("problems %v dirty %v", problems, d.Dirty())
	}
}

func TestFlowDoc_ApplySource_Given_no_change_Then_the_flow_stays_unedited(t *testing.T) {
	d, b, w := newFlowDoc(t)
	same := cloneOf(d.Draft())
	b.ParsedFlow = &same

	d.ApplySource("nodes: 3\n", func(_, _ []string) {}, func(err error) { t.Fatal(err) })
	w.Wait()

	if d.Dirty() || d.CanUndo() {
		t.Fatal("re-applying the text that is already shown should not mark an edit")
	}
}

// cloneOf copies f's node list so a test can change it apart from the draft.
func cloneOf(f model.Flow) model.Flow {
	f.Nodes = slices.Clone(f.Nodes)
	return f
}

func TestFlowDoc_AttachChoices_should_offer_the_nearest_request_before_and_after(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "r1", Type: "request"}, {ID: "a", Type: "log"}, {ID: "b", Type: "log"}, {ID: "r2", Type: "request"}, {ID: "r3", Type: "request"}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	got := d.AttachChoices("a")
	none := d.AttachChoices("nope")

	if len(got) != 2 || got[0] != "r1" || got[1] != "r2" || none != nil {
		t.Fatalf("choices %v, for an unknown node %v", got, none)
	}
}

func nodeIDs(d *vm.FlowDoc) []string {
	var ids []string
	for _, n := range d.Draft().Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func TestFlowDoc_MoveRun_should_move_a_run_of_nodes_together_in_either_direction(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "a", Type: "log"}, {ID: "b", Type: "log"}, {ID: "c", Type: "log"}, {ID: "d", Type: "log"}, {ID: "e", Type: "log"}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	errForward := d.MoveRun("b", 2, "$.nodes", 5) // after e
	forward := nodeIDs(d)
	errBack := d.MoveRun("e", 1, "$.nodes", 0)
	back := nodeIDs(d)

	if errForward != nil || errBack != nil || !slices.Equal(forward, []string{"a", "d", "e", "b", "c"}) || !slices.Equal(back, []string{"e", "a", "d", "b", "c"}) {
		t.Fatalf("errors %v %v, after moving forward %v, back %v", errForward, errBack, forward, back)
	}
}

func TestFlowDoc_Move_should_clear_attach_and_MoveAttached_should_set_it(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "a", Type: "log", Attach: "r"}, {ID: "r", Type: "request"}, {ID: "z", Type: "log"}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	_ = d.Move("a", "$.nodes", 3)
	moved, _ := d.Node("a")
	_ = d.MoveAttached("a", "$.nodes", 1, "r", "")
	attached, _ := d.Node("a")

	if moved.Attach != "" || attached.Attach != "r" || !slices.Equal(nodeIDs(d), []string{"r", "a", "z"}) {
		t.Fatalf("after a plain move %q, after MoveAttached %q, order %v", moved.Attach, attached.Attach, nodeIDs(d))
	}
}

// Behaviour change: a needs naming the node before was stored and drawn as a
// dependency, although a node follows its predecessor anyway.
func TestFlowDoc_SetNeeds_should_not_store_a_wait_for_the_node_before(t *testing.T) {
	d, _, _ := newFlowDoc(t) // a, b, p

	d.SetNeeds("b", []string{"a"})
	added := d.AddNeed("b", "a")
	d.SetNeeds("p", []string{"a"})
	again := d.AddNeed("p", "a")
	d.AddNeed("p", "b") // b is before p: the wait is redundant only when it is alone
	n, _ := d.Node("p")

	b, _ := d.Node("b")
	if len(b.Needs) != 0 || added || again || !slices.Equal(n.Needs, []string{"a", "b"}) {
		t.Fatalf("b needs %v (added %v), p needs %v (again %v)", b.Needs, added, n.Needs, again)
	}
	if got := d.EarlierIDs("p"); !slices.Equal(got, []string{"a", "b"}) || d.Previous("p") != "b" {
		t.Fatalf("earlier %v, previous %q", got, d.Previous("p"))
	}
	d.RemoveNeed("p", "a")
	n, _ = d.Node("p")
	if len(n.Needs) != 0 { // only b is left, which is the node before p: the default
		t.Fatalf("after removing a: %v", n.Needs)
	}
}

// Behaviour change: "Add case" on a plain if/else added an empty case next to
// the check, which the engine then ignores, so the condition's own steps stopped
// running. The check and the then-steps now become the first case.
func TestFlowDoc_AddCase_should_turn_a_plain_condition_into_a_chain_that_does_the_same(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{{ID: "c", Type: "condition", Params: []model.Field{{Key: "check", Value: "status == 200"}},
		Then: []model.Node{{ID: "a", Type: "log"}}, Else: []model.Node{{ID: "z", Type: "log"}}}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	d.AddCase("c")
	n, _ := d.Node("c")
	d.AddCase("c")
	again, _ := d.Node("c")

	if len(n.Cases) != 2 || n.Cases[0].When != "status == 200" || len(n.Cases[0].Nodes) != 1 || n.Cases[0].Nodes[0].ID != "a" || len(n.Then) != 0 || vm.ParamText(n, "check") != "" {
		t.Fatalf("after the first: %+v", n)
	}
	if len(again.Cases) != 3 || len(again.Else) != 1 {
		t.Fatalf("after the second: %d cases, else %v", len(again.Cases), again.Else)
	}
}

func TestFlowDoc_KeepEdits_then_RestoreKept_should_bring_the_edits_back_once(t *testing.T) {
	d, b, w := newFlowDoc(t)
	d.Insert("log", "$.nodes", 0, nil)
	edited := cloneOf(d.Draft())
	b.ParsedFlow = &edited
	if err := d.KeepEdits(); err != nil {
		t.Fatal(err)
	}
	w2 := vmtest.SyncWorker()
	reopened := vm.NewFlowDoc(b, w2, b.Cols[0], b.Cols[0].Flows[0]) // the same store, as after a restart
	w2.Wait()
	var restored, again bool

	reopened.RestoreKept(func(ok bool) { restored = ok })
	w2.Wait()
	reopened.RestoreKept(func(ok bool) { again = ok })
	w2.Wait()
	w.Wait()

	if !restored || again || !reopened.Dirty() || len(reopened.Draft().Nodes) != 4 {
		t.Fatalf("restored %v again %v dirty %v nodes %d", restored, again, reopened.Dirty(), len(reopened.Draft().Nodes))
	}
}

func TestFlowDoc_KeepEdits_Given_no_edits_Then_keeps_nothing(t *testing.T) {
	d, b, _ := newFlowDoc(t)

	err := d.KeepEdits()

	if err != nil || b.Called("KeepFlowEdits F1") {
		t.Fatalf("err %v calls %v", err, b.Calls)
	}
}

func TestFlowDoc_ApplySource_should_apply_the_text_and_note_what_is_wrong_with_the_flow_now(t *testing.T) {
	// Arrange
	d, b, w := newFlowDoc(t)
	b.DraftDiags = diag.List{diag.New(diag.CodeRefUnknownRequest, diag.Location{Path: "$.nodes[0].request.id"}, map[string]string{"id": "01X"})}
	var problems, notes []string

	// Act
	d.ApplySource("nodes: []\n", func(p, n []string) { problems, notes = p, n }, func(err error) { t.Fatal(err) })
	w.Wait()

	// Assert
	if len(problems) != 0 || len(notes) != 1 || !strings.Contains(notes[0], "FLW-REF") {
		t.Fatalf("problems %v notes %v: the text is applied, and the flow's own problems are told", problems, notes)
	}
}
