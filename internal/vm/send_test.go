// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

type sendFixture struct {
	b    *vmtest.Backend
	w    *vm.Worker
	ws   *vm.Workspace
	req  *vm.Request
	send *vm.Send
	log  *vm.RunLog
	hist *vm.History
}

func newSendFixture(t *testing.T) *sendFixture {
	t.Helper()
	b := sampleBackend()
	b.Values = map[string]any{"defaults.environment": "staging"}
	rec := &engine.Record{Status: 200, Version: "1", Body: engine.Body{JSON: map[string]any{"ok": true}}}
	b.NextSend = &vmtest.Sending{
		EventList: []engine.Event{
			engine.RequestSent{Method: "POST", URL: "https://s/login", RequestPath: "auth/login", Version: "1"},
			engine.ResponseReceived{Status: 200, Duration: 12 * time.Millisecond},
			engine.RunFinished{Status: engine.StatusPassed},
		},
		Result: engine.SendResult{Status: engine.StatusPassed, Seed: 7, Record: rec},
	}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	log, hist := vm.NewRunLog(), vm.NewHistory()
	c := b.Cols[0]
	return &sendFixture{b: b, w: w, ws: ws, req: vm.NewRequest(b, w, c, c.Requests[0]), send: vm.NewSend(b, w, ws, log, hist), log: log, hist: hist}
}

func TestSendVM_should_send_with_options_and_record_history(t *testing.T) {
	f := newSendFixture(t)
	opts := vm.SendOptions{With: []model.KeyValue{{Key: "email", Value: "a@b.c"}}, Hooks: vm.HooksSkipPost, Seed: "7", Save: "none", PreferVersion: "2"}

	f.send.Send(f.req, opts)
	f.w.Wait()

	got := f.b.SendOpts[0]
	want := flowi.SendOptions{Env: "staging", Version: "1", PreferVersion: "2", Save: "none", With: map[string]string{"email": "a@b.c"},
		Hooks: model.HookControl{Mode: "all", Skip: []string{"post"}}, Seed: 7}
	if got.Env != want.Env || got.Version != want.Version || got.PreferVersion != want.PreferVersion || got.Save != want.Save ||
		got.With["email"] != "a@b.c" || got.Hooks.Mode != "all" || !slices.Equal(got.Hooks.Skip, want.Hooks.Skip) || got.Seed != 7 {
		t.Fatalf("options %+v", got)
	}
	last, ok := f.send.Last()
	if !ok || last.Result.Status != engine.StatusPassed || len(f.hist.Items()) != 1 || f.send.Running() {
		t.Fatalf("last %+v ok %v history %d running %v", last, ok, len(f.hist.Items()), f.send.Running())
	}
	lines := strings.Join(f.log.Lines(), "\n")
	if !strings.Contains(lines, "→ POST https://s/login") || !strings.Contains(lines, "← 200") || !strings.Contains(lines, "passed") {
		t.Fatalf("log:\n%s", lines)
	}
}

func TestSendVM_Given_bad_seed_Then_error_and_no_send(t *testing.T) {
	f := newSendFixture(t)

	f.send.Send(f.req, vm.SendOptions{Seed: "abc"})
	f.w.Wait()

	if f.send.Err() == "" || f.b.Called("Send") {
		t.Fatalf("err %q, sent %v", f.send.Err(), f.b.Called("Send"))
	}
}

// Behaviour change (dirty-edit slot): a send used to save the request's
// file first, silently changing the user's saved version. The edit now goes
// to the one edit slot and is sent from there; the saved version stays as it
// was until the user saves.
func TestSendVM_Given_dirty_request_Then_stores_the_edit_instead_of_saving(t *testing.T) {
	f := newSendFixture(t)
	f.req.Edit(func(v *model.RequestVersion) { v.Method = "PUT" })

	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	if len(f.b.Saved) != 0 || !f.b.Called("StoreEdit") || !f.b.Called("Send") || !f.b.SendOpts[0].UseEdit || !f.req.Dirty() {
		t.Fatalf("saved %d stored %v sent %v dirty %v", len(f.b.Saved), f.b.Called("StoreEdit"), f.b.Called("Send"), f.req.Dirty())
	}
}

func TestSendVM_Given_unsaved_structural_changes_Then_saves_before_sending(t *testing.T) {
	f := newSendFixture(t)
	_ = f.req.NewVersionFromCurrent("2") // does not fit the one-version edit slot

	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	if len(f.b.Saved) != 1 || !f.b.Called("Send") || f.b.SendOpts[0].UseEdit || f.req.Dirty() {
		t.Fatalf("saved %d sent %v opts %+v dirty %v", len(f.b.Saved), f.b.Called("Send"), f.b.SendOpts, f.req.Dirty())
	}
}

func TestSendVM_Given_send_error_Then_shown(t *testing.T) {
	f := newSendFixture(t)
	f.b.SendErr = errors.New("vault is locked")

	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	if f.send.Err() != "vault is locked" || f.send.Running() {
		t.Fatalf("err %q running %v", f.send.Err(), f.send.Running())
	}
}

func TestSendVM_pin_example(t *testing.T) {
	f := newSendFixture(t)
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()
	var got string

	last, _ := f.send.Last()
	f.send.Pin(*last.Result.Record, "ok", func(file string, _ error) { got = file })
	f.w.Wait()

	if !f.b.Called("PinExample ok") || !strings.HasSuffix(got, "ok.json") {
		t.Fatalf("got %q calls %v", got, f.b.Calls)
	}
}

func TestHistory_should_list_newest_first_and_reopen(t *testing.T) {
	f := newSendFixture(t)
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	items := f.hist.Items()
	f.hist.Open(items[1].ID)

	if len(items) != 2 || items[0].ID < items[1].ID || f.hist.Selected().ID != items[1].ID {
		t.Fatalf("items %+v selected %+v", items, f.hist.Selected())
	}
}

func TestEventLine_should_render_hook_nodes_and_skip_their_start(t *testing.T) {
	finished := vm.EventLine(engine.NodeFinished{Node: "logged-in", Type: "assert", Status: engine.StatusPassed, Depth: 1})
	started := vm.EventLine(engine.NodeStarted{Node: "logged-in"})

	if finished != "  · logged-in (assert) passed" || started != "" {
		t.Fatalf("finished %q started %q", finished, started)
	}
}

// Review finding 7: one shared Send served every tab, so every tab showed
// the running state of whichever request was sending.
func TestSendVM_RunningFor_should_name_only_the_request_that_is_sending(t *testing.T) {
	b := sampleBackend()
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed}}
	q := newTimerQueue()
	w := vm.NewWorker(q.post)
	ws := vm.NewWorkspace(b, w)
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	c := b.Cols[0]
	login, verify := vm.NewRequest(b, w, c, c.Requests[0]), vm.NewRequest(b, w, c, c.Requests[1])

	send.Send(login, vm.SendOptions{})
	forLogin, forVerify, anySending := send.RunningFor(login.Target()), send.RunningFor(verify.Target()), send.Running()
	q.drainAll(w)

	if !forLogin || forVerify || !anySending || send.Running() || send.RunningFor(login.Target()) {
		t.Fatalf("login %v verify %v any %v; after: running %v", forLogin, forVerify, anySending, send.Running())
	}
}

func TestSendVM_should_trace_the_events_of_the_last_send_for_its_request(t *testing.T) {
	f := newSendFixture(t)
	other := flowi.RequestTarget{Collection: "other", Request: "other"}

	before := f.send.TraceFor(f.req.Target())
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()
	trace := f.send.TraceFor(f.req.Target())

	rows := f.req.Pipeline(trace)
	api := rows[len(rows)-1]
	if before != nil || f.send.TraceFor(other) != nil {
		t.Fatalf("a trace before any send, or for another request")
	}
	if api.Kind != vm.RowAPI || api.State != vm.StepPassed || api.Detail != "200" || api.Tip != "https://s/login" {
		t.Fatalf("api row %+v", api)
	}
}

// Before: the trace kept hook results by place, so after the hooks were edited
// a tick from the old send showed on whatever step now sat there.
func TestSendVM_Given_the_hooks_changed_after_a_send_Then_its_trace_is_not_shown(t *testing.T) {
	f := newSendFixture(t)
	f.req.Edit(func(v *model.RequestVersion) { v.Hooks.Post = []model.HookStep{inline("a")} })
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()
	before := f.req.Pipeline(f.send.TraceFor(f.req.Target()))

	f.req.Edit(func(v *model.RequestVersion) { v.Hooks.Post = []model.HookStep{inline("z", "a")} })
	after := f.req.Pipeline(f.send.TraceFor(f.req.Target()))

	if before[len(before)-2].State == vm.StepIdle && before[len(before)-1].State == vm.StepIdle {
		t.Fatalf("the send's own pipeline shows no results: %+v", before)
	}
	for _, row := range after {
		if row.State != vm.StepIdle {
			t.Fatalf("%q shows %v from a send of other hooks", row.Title, row.State)
		}
	}
}

func TestPipeline_Given_a_hook_flow_of_another_collection_Then_it_is_shown_as_not_looked_up_here(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.Edit(func(v *model.RequestVersion) {
		v.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{Collection: "elsewhere", Path: "hooks/sign"}}}
	})

	rows := r.Pipeline(nil)

	if len(rows) < 3 || rows[1].Title != "flow hooks/sign" || rows[1].Detail != "version hook, not shown here" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestSend_should_not_panic_and_should_show_the_warnings_when_a_send_has_only_warnings(t *testing.T) {
	b := sampleBackend()
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed,
		Diags: diag.List{diag.New(diag.CodeGRPCField, diag.Location{}, map[string]string{"detail": "extra is not a field of shop.Q: it was left out"})}}}
	ws, w := loadedWorkspace(t, b)
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	r := vm.NewRequest(b, w, b.Cols[0], b.Cols[0].Requests[0])

	send.Send(r, vm.SendOptions{})
	w.Wait()

	if got := send.Err(); !strings.Contains(got, "FLW-GRPC-006") || !strings.Contains(got, "extra") {
		t.Fatalf("status text %q, want the warning with its code", got)
	}
}

func TestErrorText_should_return_nothing_for_no_error(t *testing.T) {
	if got := vm.ErrorText(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestHistory_and_RunLog_Clear_should_empty_them_and_tell_the_listeners(t *testing.T) {
	f := newSendFixture(t)
	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()
	changed := 0
	f.hist.OnChange(func() { changed++ })
	f.log.OnChange(func() { changed++ })

	f.hist.Clear()
	f.log.Clear()

	if len(f.hist.Items()) != 0 || len(f.log.Lines()) != 0 || changed != 2 {
		t.Fatalf("history %d log %d changed %d", len(f.hist.Items()), len(f.log.Lines()), changed)
	}
}

func TestHistory_Remove_should_forget_one_exchange_and_leave_the_others(t *testing.T) {
	h := vm.NewHistory()
	target := flowi.RequestTarget{Collection: "C1", Request: "R1"}
	first := h.Add(target, "1", engine.SendResult{})
	second := h.Add(target, "1", engine.SendResult{})
	changes := 0
	h.OnChange(func() { changes++ })

	h.Remove(first.ID)
	h.Remove(first.ID) // already gone: nothing changes

	if items := h.Items(); len(items) != 1 || items[0].ID != second.ID || changes != 1 {
		t.Fatalf("items %+v, changes %d", items, changes)
	}
}
