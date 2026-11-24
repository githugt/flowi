// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func httpVersion() model.RequestVersion {
	return model.RequestVersion{
		Label: "1", Name: "Before tax", Docs: "notes", Method: "POST", URL: "{{env.base_url}}/orders",
		Headers: []model.KeyValue{{Key: "X-Trace", Value: "1"}},
		Query:   []model.KeyValue{{Key: "page", Value: "2"}},
		Body:    model.Body{JSON: `{"id": 1}`},
		Assert:  []string{"status == 200"}, Expect: "2xx",
		Extract: []model.Extraction{{Name: "id", From: "body", Path: "id"}},
	}
}

// Before: a request could not change type; it was made again from nothing.
func TestConvertVersion_Given_an_http_request_When_made_a_websocket_Then_the_address_message_and_checks_come_along(t *testing.T) {
	got, lost, err := vm.ConvertVersion(httpVersion(), "message")

	if err != nil || got.Async == nil || got.Async.Protocol != "ws" {
		t.Fatalf("err %v, version %+v", err, got)
	}
	if got.Async.Server != "{{env.base_url}}" || got.Async.Channel != "/orders" || got.Async.Send == nil || got.Async.Send.Body.JSON != `{"id": 1}` {
		t.Errorf("async %+v", got.Async)
	}
	if got.Label != "1" || got.Name != "Before tax" || got.Docs != "notes" || len(got.Assert) != 1 || len(got.Extract) != 1 || len(got.Headers) != 1 {
		t.Errorf("what the types share was lost: %+v", got)
	}
	if got.Method != "" || got.URL != "" || got.Body.JSON != "" {
		t.Errorf("the HTTP fields stayed: %+v", got)
	}
	want := []string{"the expected status", "the query parameters"}
	for _, w := range want {
		if !slices.Contains(lost, w) {
			t.Errorf("lost %v does not say %q", lost, w)
		}
	}
}

func TestConvertVersion_Given_a_put_and_a_post_When_made_a_websocket_Then_only_the_put_is_noted(t *testing.T) {
	put := model.RequestVersion{Label: "1", Method: "PUT", URL: "https://h.example.com/a"}
	post := model.RequestVersion{Label: "1", Method: "POST", URL: "https://h.example.com/a", Body: model.Body{JSON: "{}"}}

	_, lostPut, _ := vm.ConvertVersion(put, "message")
	_, lostPost, _ := vm.ConvertVersion(post, "message")

	if !slices.Contains(lostPut, "the method PUT") || len(lostPost) != 0 {
		t.Errorf("put %v post %v", lostPut, lostPost)
	}
}

func TestConvertVersion_Given_https_When_made_a_websocket_Then_the_scheme_follows(t *testing.T) {
	v := model.RequestVersion{Label: "1", Method: "GET", URL: "https://chat.example.com/socket"}

	got, _, err := vm.ConvertVersion(v, "message")

	if err != nil || got.Async.Server != "wss://chat.example.com" || got.Async.Channel != "/socket" {
		t.Fatalf("err %v, async %+v", err, got.Async)
	}
}

func TestConvertVersion_Given_an_http_request_When_made_grpc_Then_the_server_and_json_body_carry_and_the_path_is_reported(t *testing.T) {
	v := model.RequestVersion{Label: "1", Method: "POST", URL: "https://api.example.com:443/v1/orders", Body: model.Body{JSON: `{"a":1}`}, Headers: []model.KeyValue{{Key: "k", Value: "v"}}}

	got, lost, err := vm.ConvertVersion(v, "grpc")

	if err != nil || got.GRPC == nil || got.GRPC.Server != "grpcs://api.example.com:443" || got.GRPC.Body.JSON != `{"a":1}` || len(got.Headers) != 1 {
		t.Fatalf("err %v, grpc %+v", err, got.GRPC)
	}
	if !slices.Contains(lost, "the path /v1/orders") {
		t.Errorf("lost %v", lost)
	}
}

func TestConvertVersion_Given_a_websocket_request_When_made_http_Then_it_is_a_request_to_the_same_address(t *testing.T) {
	v := model.RequestVersion{Label: "1", Async: &model.Async{Protocol: "ws", Server: "wss://h.example.com", Channel: "chat", Send: &model.AsyncSend{Body: model.Body{Text: "hi"}}}}

	got, lost, err := vm.ConvertVersion(v, "http")

	if err != nil || got.Async != nil || got.Method != "POST" || got.URL != "https://h.example.com/chat" || got.Body.Text != "hi" || len(lost) != 0 {
		t.Fatalf("err %v lost %v version %+v", err, lost, got)
	}
}

func TestConvertVersion_Given_a_broker_request_When_made_http_Then_its_server_options_and_wait_are_reported(t *testing.T) {
	v := model.RequestVersion{Label: "1", Async: &model.Async{Protocol: "mqtt", Server: "mqtt://b:1883", Channel: "t/1",
		Binding: []model.KeyValue{{Key: "qos", Value: "1"}}, Receive: &model.AsyncReceive{Count: 3}, Send: &model.AsyncSend{Body: model.Body{JSON: "{}"}}}}

	got, lost, err := vm.ConvertVersion(v, "http")

	if err != nil || got.URL != "" || got.Body.JSON != "{}" {
		t.Fatalf("err %v version %+v", err, got)
	}
	for _, w := range []string{"the protocol's options", "what it waits for"} {
		if !slices.Contains(lost, w) {
			t.Errorf("lost %v does not say %q", lost, w)
		}
	}
}

func TestConvertVersion_Given_an_http_request_When_made_a_query_Then_it_starts_from_the_starter_and_says_what_it_left(t *testing.T) {
	got, lost, err := vm.ConvertVersion(httpVersion(), "db")

	if err != nil || got.Dataset == nil || got.Dataset.Driver != "postgres" || got.Dataset.Query == "" || got.Async != nil {
		t.Fatalf("err %v version %+v", err, got)
	}
	if len(got.Assert) != 1 || got.Headers != nil || got.Body.JSON != "" {
		t.Errorf("version %+v", got)
	}
	for _, w := range []string{"the address", "the body", "the headers", "the query parameters"} {
		if !slices.Contains(lost, w) {
			t.Errorf("lost %v does not say %q", lost, w)
		}
	}
}

func TestConvertVersion_Given_a_query_When_made_a_data_file_Then_the_query_is_reported_and_a_file_starter_is_made(t *testing.T) {
	v := model.RequestVersion{Label: "1", Dataset: &model.Dataset{Driver: "mysql", Server: "mysql://x", Query: "SELECT 1", Params: []string{"a"}}}

	got, lost, err := vm.ConvertVersion(v, "file")

	if err != nil || got.File == nil || got.File.Format != "csv" || got.Dataset != nil {
		t.Fatalf("err %v version %+v", err, got)
	}
	if !slices.Contains(lost, "the query") || !slices.Contains(lost, "the query's values") {
		t.Errorf("lost %v", lost)
	}
}

// The protocol, database and file format are the address row's pill, not part of the type: asking for the type a
// request already has changes nothing, whatever it is inside.
func TestConvertVersion_Given_the_type_it_already_is_Then_it_is_left_as_it_is(t *testing.T) {
	m := model.RequestVersion{Label: "1", Async: &model.Async{Protocol: "mqtt", Server: "mqtt://b", Channel: "t", Send: &model.AsyncSend{}}}
	d := model.RequestVersion{Label: "1", Dataset: &model.Dataset{Driver: "mysql", Server: "s", Query: "SELECT 1"}}
	f := model.RequestVersion{Label: "1", File: &model.DataFile{Format: "xlsx", Path: "a.xlsx", Query: "SELECT 1"}}

	mqtt, lostA, _ := vm.ConvertVersion(m, "message")
	mysql, lostD, _ := vm.ConvertVersion(d, "db")
	xlsx, lostF, _ := vm.ConvertVersion(f, "file")

	if mqtt.Async.Protocol != "mqtt" || mysql.Dataset.Driver != "mysql" || xlsx.File.Format != "xlsx" || len(lostA)+len(lostD)+len(lostF) != 0 {
		t.Errorf("%+v %+v %+v lost %v %v %v", mqtt.Async, mysql.Dataset, xlsx.File, lostA, lostD, lostF)
	}
}

func TestConvertVersion_Given_a_query_When_made_a_message_request_Then_it_starts_as_a_websocket(t *testing.T) {
	d := model.RequestVersion{Label: "1", Dataset: &model.Dataset{Driver: "mysql", Query: "SELECT 1"}}

	got, _, err := vm.ConvertVersion(d, "message")

	if err != nil || got.Async == nil || got.Async.Protocol != "ws" || got.Dataset != nil {
		t.Fatalf("err %v version %+v", err, got)
	}
}

func TestConvertVersion_Given_an_unknown_type_Then_it_is_refused(t *testing.T) {
	if _, _, err := vm.ConvertVersion(httpVersion(), "carrier-pigeon"); err == nil {
		t.Fatal("no error")
	}
}

func TestTypeOf_names_the_family_of_each_kind_and_every_listed_type_has_an_icon(t *testing.T) {
	cases := map[string]model.RequestVersion{
		"http": {}, "grpc": {GRPC: &model.GRPC{}}, "message": {Async: &model.Async{Protocol: "kafka"}},
		"db": {Dataset: &model.Dataset{Driver: "sqlite"}}, "file": {File: &model.DataFile{Format: "parquet"}},
	}
	for want, v := range cases {
		if got := vm.TypeOf(v); got != want {
			t.Errorf("TypeOf = %q, want %q", got, want)
		}
	}
	for _, id := range vm.TypeIDs() {
		if vm.TypeIcon(id) == "" {
			t.Errorf("%s has no icon", id)
		}
	}
	if got := vm.TypeIDs(); len(got) != 5 {
		t.Errorf("types %v: the menu is the five families", got)
	}
}

func TestRequestVM_SwitchType_is_an_edit_that_discard_undoes(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)
	before := r.Current()

	lost, err := r.SwitchType("message")

	if err != nil || !r.IsAsync() || !r.Dirty() || r.Current().Label != before.Label {
		t.Fatalf("err %v async %v dirty %v lost %v", err, r.IsAsync(), r.Dirty(), lost)
	}
	r.DiscardEdit()
	w.Wait()
	if r.IsAsync() || r.Current().URL != before.URL {
		t.Fatalf("discard did not bring the request back: %+v", r.Current())
	}
}

func TestRequestVM_SwitchType_to_the_type_it_is_changes_nothing(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	lost, err := r.SwitchType("http")

	if err != nil || len(lost) != 0 || r.Dirty() {
		t.Fatalf("err %v lost %v dirty %v", err, lost, r.Dirty())
	}
}
