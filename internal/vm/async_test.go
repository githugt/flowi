// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func asyncBackend() *vmtest.Backend {
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{
		Meta: model.Meta{ID: "A1", Name: "Placed"}, Path: "orders/placed", File: "/c/requests/orders/placed.req.yaml",
		Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{
			Protocol: "mqtt", Server: "mqtt://b", Channel: "orders/placed",
			Send:    &model.AsyncSend{Body: model.Body{JSON: `{"id":1}`}},
			Binding: []model.KeyValue{{Key: "qos", Value: "1"}}}}},
	})
	return &vmtest.Backend{Cols: []model.Collection{c}}
}

func asyncRequestVM(t *testing.T, b *vmtest.Backend) (*vm.Request, *vm.Worker) {
	t.Helper()
	w := vmtest.SyncWorker()
	c := b.Cols[0]
	return vm.NewRequest(b, w, c, c.Requests[3]), w
}

func TestVerb_should_be_the_protocol_for_an_async_version_and_the_method_otherwise(t *testing.T) {
	tests := []struct {
		name string
		v    model.RequestVersion
		want string
	}{
		{"http", model.RequestVersion{Method: "POST"}, "POST"},
		{"mqtt", model.RequestVersion{Async: &model.Async{Protocol: "mqtt"}}, "MQTT"},
		{"ws", model.RequestVersion{Async: &model.Async{Protocol: "ws"}}, "WS"},
	}
	for _, tt := range tests {
		if got := vm.Verb(tt.v); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRequestVM_should_edit_the_async_block_without_touching_what_is_on_disk(t *testing.T) {
	b := asyncBackend()
	r, w := asyncRequestVM(t, b)

	r.EditAsync(func(a *model.Async) {
		a.Channel = "orders/shipped"
		a.Binding = append(a.Binding, model.KeyValue{Key: "retain", Value: "true"})
		a.Send.Body.JSON = `{"id":2}`
	})
	edited := r.Dirty() && r.Async().Channel == "orders/shipped"
	r.Revert()
	back := r.Async()
	r.EditAsync(func(a *model.Async) { a.Channel = "again" })
	r.Save(false)
	w.Wait()

	if !r.IsAsync() || !edited {
		t.Fatalf("async %v edited %v", r.IsAsync(), edited)
	}
	if back.Channel != "orders/placed" || len(back.Binding) != 1 || back.Send.Body.JSON != `{"id":1}` {
		t.Fatalf("revert did not restore the async block: %+v", back)
	}
	if len(b.Saved) != 1 || b.Saved[0].Versions[0].Async.Channel != "again" {
		t.Fatalf("saved %+v", b.Saved)
	}
}

func TestRequestVM_EditAsync_Given_an_http_request_Then_nothing_changes(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.EditAsync(func(a *model.Async) { a.Channel = "x" })

	if r.IsAsync() || r.Dirty() {
		t.Fatalf("async %v dirty %v", r.IsAsync(), r.Dirty())
	}
}

func TestRequestVM_SetProtocol_should_drop_binding_names_the_new_protocol_does_not_know(t *testing.T) {
	r, _ := asyncRequestVM(t, asyncBackend())
	r.EditAsync(func(a *model.Async) {
		a.Binding = []model.KeyValue{{Key: "qos", Value: "1"}, {Key: "username", Value: "u"}}
	})

	r.SetProtocol("nats")

	a := r.Async()
	if a.Protocol != "nats" || len(a.Binding) != 1 || a.Binding[0].Key != "username" {
		t.Fatalf("after switching to nats: %+v", a)
	}
}

func TestRequestVM_Configured_should_mark_the_async_sections_that_have_content(t *testing.T) {
	r, _ := asyncRequestVM(t, asyncBackend())

	got := map[vm.Section]bool{
		vm.SectionMessage: r.Configured(vm.SectionMessage), vm.SectionReceive: r.Configured(vm.SectionReceive), vm.SectionBinding: r.Configured(vm.SectionBinding),
	}

	if !got[vm.SectionMessage] || got[vm.SectionReceive] || !got[vm.SectionBinding] {
		t.Fatalf("configured %v", got)
	}
}

func TestTree_should_show_the_protocol_where_an_http_request_shows_its_verb(t *testing.T) {
	ws, _ := loadedWorkspace(t, asyncBackend())

	got := ws.Node("r:C1:A1").Method

	if got != "MQTT" {
		t.Fatalf("tree verb %q", got)
	}
}

func TestWorkspace_CreateAsyncRequest_should_create_through_the_backend_and_open_it(t *testing.T) {
	b := sampleBackend()
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateAsyncRequest("C1", "orders/placed", "mqtt", func(id string) { opened = id }, func(err error) { t.Fatal(err) })
	w.Wait()

	if !slices.Contains(b.Calls, "NewAsyncRequest orders/placed mqtt") || opened == "" {
		t.Fatalf("calls %v opened %q", b.Calls, opened)
	}
}

func TestTrace_should_pass_the_request_row_of_an_async_send_with_its_outcome(t *testing.T) {
	trace := vm.NewTrace()

	trace.Apply(engine.RequestSent{Method: "NATS", URL: "nats://localhost:4222/orders"})
	trace.Apply(engine.ResponseReceived{Outcome: engine.OutcomeOK, Duration: 12})
	trace.Finish()

	rows := pipelineOf(t, trace)
	var api vm.PipelineRow
	for _, r := range rows {
		if r.Kind == vm.RowAPI {
			api = r
		}
	}
	if api.State != vm.StepPassed || api.Detail != "ok" {
		t.Fatalf("api row %+v", api)
	}
}

func TestPipeline_should_show_the_scheme_in_the_tooltip_of_an_async_request(t *testing.T) {
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = &model.Async{Protocol: "nats", Server: "localhost:4222", Channel: "orders", Receive: &model.AsyncReceive{}}
	r, _ := asyncRequestVM(t, b)

	var tip string
	for _, row := range r.Pipeline(nil) {
		if row.Kind == vm.RowAPI {
			tip = row.Tip
		}
	}

	if tip != "nats://localhost:4222/orders" {
		t.Fatalf("tooltip %q", tip)
	}
}

func TestWorkspace_CreateAsyncRequest_should_seed_the_starting_hooks(t *testing.T) {
	b := sampleBackend()
	ws, w := loadedWorkspace(t, b)

	ws.CreateAsyncRequest("C1", "orders/placed", "nats", func(string) {}, func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("SeedRequestHooks new") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestRequestVM_SetProtocol_to_sse_should_drop_the_message_and_keep_it_receiving(t *testing.T) {
	r, _ := asyncRequestVM(t, asyncBackend())

	r.SetProtocol("sse")

	if a := r.Async(); a.Send != nil || a.Receive == nil {
		t.Fatalf("async %+v", a)
	}
}

func TestTrace_should_collect_the_messages_an_async_request_has_received_so_far(t *testing.T) {
	trace := vm.NewTrace()

	trace.Apply(engine.RequestSent{Method: "MQTT"})
	trace.Apply(engine.MessageReceived{Seq: 1, Message: engine.RecordMessage{Direction: "received", Body: engine.Body{Text: "a"}}})
	trace.Apply(engine.MessageReceived{Seq: 2, Message: engine.RecordMessage{Direction: "received", Body: engine.Body{Text: "b"}}})
	trace.Apply(engine.MessageReceived{Seq: 1, Depth: 1}) // inside a hook flow: not this request's

	got := trace.Messages()
	if len(got) != 2 || got[1].Body.Text != "b" {
		t.Fatalf("messages %+v", got)
	}
	trace.Apply(engine.RequestSent{Method: "MQTT"}) // a new attempt starts over
	if len(trace.Messages()) != 0 {
		t.Fatal("a new send starts with no messages")
	}
}

func TestPipeline_should_not_put_an_sse_event_name_in_the_url(t *testing.T) {
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = &model.Async{Protocol: "sse", Server: "http://localhost:4000/stream", Channel: "custom-trigger", Receive: &model.AsyncReceive{}}
	r, _ := asyncRequestVM(t, b)

	var tip string
	for _, row := range r.Pipeline(nil) {
		if row.Kind == vm.RowAPI {
			tip = row.Tip
		}
	}

	if tip != "http://localhost:4000/stream" {
		t.Fatalf("tooltip %q", tip)
	}
}

func cloudRequest(t *testing.T, binding ...model.KeyValue) *vm.Request {
	t.Helper()
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = &model.Async{Protocol: "sqs", Channel: "jobs", Send: &model.AsyncSend{}, Binding: binding}
	r, _ := asyncRequestVM(t, b)
	return r
}

func TestRequestVM_should_keep_credentials_apart_from_the_other_binding_options(t *testing.T) {
	r := cloudRequest(t,
		model.KeyValue{Key: "region", Value: "eu-west-1"}, model.KeyValue{Key: "auth", Value: "explicit"},
		model.KeyValue{Key: "access_key_id", Value: "{{secret.id}}"}, model.KeyValue{Key: "wait", Value: "5"})

	opts := r.BindingOptions()

	if len(opts) != 2 || opts[0].Key != "region" || opts[1].Key != "wait" {
		t.Fatalf("options %+v, want region and wait only", opts)
	}
	if r.Credential("access_key_id") != "{{secret.id}}" || r.AuthMode() != "explicit" {
		t.Fatalf("credential %q mode %q", r.Credential("access_key_id"), r.AuthMode())
	}
}

func TestRequestVM_SetBindingOptions_should_leave_the_credentials_and_mode_alone(t *testing.T) {
	r := cloudRequest(t, model.KeyValue{Key: "auth", Value: "explicit"}, model.KeyValue{Key: "secret_access_key", Value: "{{secret.k}}"})

	r.SetBindingOptions([]model.KeyValue{{Key: "region", Value: "us-east-1"}})

	got := r.Async().Binding
	want := []model.KeyValue{{Key: "region", Value: "us-east-1"}, {Key: "auth", Value: "explicit"}, {Key: "secret_access_key", Value: "{{secret.k}}"}}
	if !slices.Equal(got, want) {
		t.Fatalf("binding %+v, want %+v", got, want)
	}
}

func TestRequestVM_SetCredential_should_write_the_entry_and_remove_it_when_cleared(t *testing.T) {
	r := cloudRequest(t)

	r.SetCredential("access_key_id", "AKID")
	r.SetCredential("secret_access_key", "{{secret.k}}")
	r.SetCredential("access_key_id", "")

	got := r.Async().Binding
	if len(got) != 1 || got[0].Key != "secret_access_key" {
		t.Fatalf("binding %+v, want only secret_access_key", got)
	}
	r.SetCredential("not_a_credential", "x")
	if len(r.Async().Binding) != 1 {
		t.Fatalf("a name the protocol does not take as a credential was written: %+v", r.Async().Binding)
	}
}

func TestRequestVM_SetAuthMode_should_store_explicit_and_drop_the_entry_for_the_default(t *testing.T) {
	r := cloudRequest(t)
	if r.AuthMode() != "default" {
		t.Fatalf("mode %q, want default when nothing is set", r.AuthMode())
	}

	r.SetAuthMode("explicit")
	explicit := slices.Clone(r.Async().Binding)
	r.SetAuthMode("default")

	if len(explicit) != 1 || explicit[0] != (model.KeyValue{Key: "auth", Value: "explicit"}) || len(r.Async().Binding) != 0 {
		t.Fatalf("explicit %+v, then default %+v", explicit, r.Async().Binding)
	}
}

func TestRequestVM_SetProtocol_should_drop_the_credentials_the_new_protocol_does_not_take(t *testing.T) {
	r := cloudRequest(t, model.KeyValue{Key: "auth", Value: "explicit"}, model.KeyValue{Key: "access_key_id", Value: "AKID"}, model.KeyValue{Key: "region", Value: "eu-west-1"})

	r.SetProtocol("sns") // takes the same credentials
	kept := slices.Clone(r.Async().Binding)
	r.SetProtocol("mqtt") // takes none of them

	if len(kept) != 3 {
		t.Fatalf("sqs to sns lost credentials: %+v", kept)
	}
	if got := r.Async().Binding; len(got) != 0 {
		t.Fatalf("after mqtt the binding is %+v, want empty", got)
	}
}

func TestRequestVM_SetProtocol_to_sns_should_stop_receiving_and_keep_a_message_to_send(t *testing.T) {
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = &model.Async{Protocol: "sqs", Channel: "jobs", Receive: &model.AsyncReceive{Count: 2}}
	r, _ := asyncRequestVM(t, b)

	r.SetProtocol("sns")

	if a := r.Async(); a.Receive != nil || a.Send == nil {
		t.Fatalf("after sns: send %v receive %v", a.Send, a.Receive)
	}
}

func TestRequestVM_Configured_should_mark_Authentication_when_a_credential_or_explicit_mode_is_set(t *testing.T) {
	plain := cloudRequest(t, model.KeyValue{Key: "region", Value: "eu-west-1"})
	explicit := cloudRequest(t, model.KeyValue{Key: "auth", Value: "explicit"})
	withKey := cloudRequest(t, model.KeyValue{Key: "access_key_id", Value: "AKID"})

	if plain.Configured(vm.SectionCredentials) || !explicit.Configured(vm.SectionCredentials) || !withKey.Configured(vm.SectionCredentials) {
		t.Fatalf("plain %v explicit %v key %v", plain.Configured(vm.SectionCredentials), explicit.Configured(vm.SectionCredentials), withKey.Configured(vm.SectionCredentials))
	}
}

func TestRequestVM_SetMode_should_choose_between_sending_receiving_and_both(t *testing.T) {
	r := cloudRequest(t) // sqs, sends
	modes := []struct {
		set        vm.AsyncMode
		send, recv bool
	}{{vm.ModeReceive, false, true}, {vm.ModeSend, true, false}, {vm.ModeSendReceive, true, true}}

	for _, m := range modes {
		r.SetMode(m.set)

		a := r.Async()
		if (a.Send != nil) != m.send || (a.Receive != nil) != m.recv || r.Mode() != m.set {
			t.Fatalf("after %s: send %v receive %v mode %s", m.set, a.Send != nil, a.Receive != nil, r.Mode())
		}
	}
}

func TestAsyncModesFor_should_offer_no_choice_to_one_way_protocols(t *testing.T) {
	if got := vm.AsyncModesFor("sse"); got != nil {
		t.Errorf("sse listens only: %v", got)
	}
	if got := vm.AsyncModesFor("sns"); got != nil {
		t.Errorf("sns publishes only: %v", got)
	}
	if got := vm.AsyncModesFor("pubsub"); len(got) != 3 {
		t.Errorf("pubsub modes %v", got)
	}
}
