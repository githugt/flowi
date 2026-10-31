// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

func recordWith(status int, request, response any) engine.Record {
	rec := engine.Record{Version: "1", Status: status, Body: engine.Body{JSON: response}}
	if request != nil {
		rec.Request = engine.SentRequest{Method: "POST", Body: &engine.Body{JSON: request}}
	}
	return rec
}

func schemaPathOf(t *testing.T, w *Workspace, target RequestTarget, name string) string {
	t.Helper()
	_, r, err := w.requestFor(target)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(strings.TrimSuffix(r.File, ".req.yaml")+".schemas", name)
}

func TestLearnSchemas_should_create_request_and_response_schemas_from_the_first_send(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	changes, err := w.LearnSchemas(target, "1", recordWith(200, map[string]any{"email": "a@b.co"}, map[string]any{"token": "t", "expires": 3.0}))

	if err != nil || len(changes) != 0 {
		t.Fatalf("changes %v err %v", changes, err)
	}
	for _, name := range []string{"1-request.json", "1-200.json"} {
		if _, statErr := os.Stat(schemaPathOf(t, w, target, name)); statErr != nil {
			t.Errorf("%s: %v", name, statErr)
		}
	}
	got, source, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "response", Status: 200})
	if !ok || source != SchemaInferred || got["type"] != "object" {
		t.Fatalf("schema %v source %v ok %v", got, source, ok)
	}
}

func TestLearnSchemas_should_widen_the_schema_and_return_what_changed(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	if _, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"token": "t", "expires": 3.0})); err != nil {
		t.Fatal(err)
	}

	changes, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"token": "u"}))

	got, _, _ := w.SchemaFor(target, "1", SchemaPart{Kind: "response", Status: 200})
	required, _ := got["required"].([]any)
	if err != nil || len(changes) != 1 || changes[0].Change.Kind != schemainfer.OptionalField || !slices.Equal(required, []any{"token"}) {
		t.Fatalf("changes %+v required %v err %v", changes, required, err)
	}
}

func TestLearnSchemas_should_not_touch_a_schema_from_a_spec_or_the_user(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	for _, source := range []string{"openapi", "user"} {
		file := schemaPathOf(t, w, target, "1-200.json")
		content := `{"type":"object","x-flowi-source":"` + source + `","properties":{"token":{"type":"integer"}}}`
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		changes, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"token": "x"}))

		after, _ := os.ReadFile(file)
		if err != nil || len(changes) != 0 || string(after) != content {
			t.Errorf("%s: changes %v err %v file %s", source, changes, err, after)
		}
	}
}

func TestLearnSchemas_Given_a_body_that_is_not_JSON_Then_learns_nothing_and_reports_nothing(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	rec := engine.Record{Version: "1", Status: 502, Body: engine.Body{Text: "<html>bad gateway</html>"}}

	changes, err := w.LearnSchemas(target, "1", rec)

	_, statErr := os.Stat(schemaPathOf(t, w, target, "1-502.json"))
	if err != nil || len(changes) != 0 || !os.IsNotExist(statErr) {
		t.Fatalf("changes %v err %v stat %v", changes, err, statErr)
	}
}

func TestDeleteSchema_and_ReplaceSchema_should_remove_and_mark_a_hand_edit(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	part := SchemaPart{Kind: "response", Status: 200}
	if _, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"token": "t"})); err != nil {
		t.Fatal(err)
	}

	replaceErr := w.ReplaceSchema(target, "1", part, schemainfer.Schema{"type": "object"})
	_, source, _ := w.SchemaFor(target, "1", part)
	deleteErr := w.DeleteSchema(target, "1", part)
	_, _, still := w.SchemaFor(target, "1", part)

	if replaceErr != nil || source != SchemaUser || deleteErr != nil || still {
		t.Fatalf("replace %v source %v delete %v still %v", replaceErr, source, deleteErr, still)
	}
}

func sendLogin(t *testing.T, w *Workspace) engine.SendResult {
	t.Helper()
	target, _ := w.FindRequest("auth/login", "")
	s, err := w.SendRequest(t.Context(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	return drain(s)
}

func hasDiag(res engine.SendResult, code diag.Code) bool {
	return slices.ContainsFunc(res.Diags, func(d diag.Diagnostic) bool { return d.Code == code })
}

func TestSendRequest_should_learn_schemas_silently_on_the_first_send(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	res := sendLogin(t, w)

	_, _, response := w.SchemaFor(target, "1", SchemaPart{Kind: "response", Status: 200})
	_, _, request := w.SchemaFor(target, "1", SchemaPart{Kind: "request"})
	// Login's body uses {{secret.password}}, so the record holds it fully masked:
	// nothing may be learned from it (a secret must never shape a schema).
	if res.Status != engine.StatusPassed || !response || request || hasDiag(res, diag.CodeSchemaResponse) || hasDiag(res, diag.CodeSchemaUpdated) {
		t.Fatalf("status %s response %v request %v diags %v", res.Status, response, request, res.Diags)
	}
}

func TestSendRequest_Given_a_response_that_breaks_its_schema_Then_an_error_diag_and_the_send_still_passes(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	sendLogin(t, w)
	strict := schemainfer.Schema{"type": "object", "required": []any{"nope"}}
	if err := w.ReplaceSchema(target, "1", SchemaPart{Kind: "response", Status: 200}, strict); err != nil {
		t.Fatal(err)
	}

	res := sendLogin(t, w)

	if res.Status != engine.StatusPassed || !hasDiag(res, diag.CodeSchemaResponse) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSchemaChecker_Given_a_request_body_that_breaks_its_schema_Then_a_warning_that_does_not_stop_the_send(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	c, r, _ := w.requestFor(target)
	strict := schemainfer.Schema{"type": "object", "required": []any{"zzz"}}
	if err := w.ReplaceSchema(target, "1", SchemaPart{Kind: "request"}, strict); err != nil {
		t.Fatal(err)
	}

	diags := schemaChecker{w}.Check(c, r, "1", recordWith(200, map[string]any{"a": 1.0}, nil))

	if len(diags) != 1 || diags[0].Code != diag.CodeSchemaRequest || diags[0].Severity != diag.SeverityWarning {
		t.Fatalf("diags %v", diags)
	}
}

func TestSendRequest_Given_the_response_changed_since_the_schema_was_learned_Then_it_is_updated_and_reported(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	sendLogin(t, w)
	file := schemaPathOf(t, w, target, "1-200.json")
	data, _ := os.ReadFile(file)
	withExtra := strings.Replace(string(data), `"required": [`, `"required": [ "extra",`, 1) // a field this server never sends
	if err := os.WriteFile(file, []byte(withExtra), 0o644); err != nil {
		t.Fatal(err)
	}

	res := sendLogin(t, w)

	if !hasDiag(res, diag.CodeSchemaUpdated) {
		t.Fatalf("diags %v", res.Diags)
	}
}

func TestSendRequest_Given_schemas_infer_is_off_Then_nothing_is_learned_or_checked(t *testing.T) {
	w := demoWorkspaceWith(t, func(o *WorkspaceOptions) { o.Environ = append(o.Environ, "FLOWI_SCHEMAS_INFER=false") })
	target, _ := w.FindRequest("auth/login", "")

	res := sendLogin(t, w)

	_, _, found := w.SchemaFor(target, "1", SchemaPart{Kind: "response", Status: 200})
	if res.Status != engine.StatusPassed || found {
		t.Fatalf("status %s learned %v diags %v", res.Status, found, res.Diags)
	}
}

func TestListSchemas_should_list_the_parts_stored_for_a_version_in_a_stable_order(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	for _, status := range []int{404, 200} {
		if _, err := w.LearnSchemas(target, "1", recordWith(status, map[string]any{"a": 1.0}, map[string]any{"b": 1.0})); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.LearnSchemas(target, "2", recordWith(200, nil, map[string]any{"b": 1.0})); err != nil {
		t.Fatal(err)
	}

	got := w.ListSchemas(target, "1")

	want := []SchemaPart{{Kind: "request"}, {Kind: "response", Status: 200}, {Kind: "response", Status: 404}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Pins from the plan's review focus.
func TestSchemaChecker_Given_an_empty_response_Then_no_violation_even_with_a_stored_schema(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	c, r, _ := w.requestFor(target)
	if err := w.ReplaceSchema(target, "1", SchemaPart{Kind: "response", Status: 204}, schemainfer.Schema{"type": "object", "required": []any{"token"}}); err != nil {
		t.Fatal(err)
	}

	diags := schemaChecker{w}.Check(c, r, "1", engine.Record{Version: "1", Status: 204})

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestLearnSchemas_Given_a_stored_schema_that_does_not_parse_Then_leaves_it_and_reports_nothing(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	file := schemaPathOf(t, w, target, "1-200.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"a": 1.0}))

	after, _ := os.ReadFile(file)
	if err != nil || len(changes) != 0 || string(after) != "{not json" {
		t.Fatalf("changes %v err %v file %q", changes, err, after)
	}
}

func asyncRecord(sent any, received ...any) engine.Record {
	rec := engine.Record{Version: "1", Kind: engine.KindAsync, Outcome: engine.OutcomeOK}
	if sent != nil {
		rec.Messages = append(rec.Messages, engine.RecordMessage{Direction: "sent", Body: engine.Body{JSON: sent}})
		rec.Request = engine.SentRequest{Method: "MQTT", Body: &engine.Body{JSON: sent}}
	}
	for _, r := range received {
		rec.Messages = append(rec.Messages, engine.RecordMessage{Direction: "received", Body: engine.Body{JSON: r}})
	}
	return rec
}

func TestLearnSchemas_should_learn_the_sent_payload_and_every_received_message_of_an_async_request(t *testing.T) {
	w := demoWorkspace(t)
	target := saveAsync(t, w, "orders/placed", &model.Async{Protocol: "mqtt", Server: "mqtt://b", Channel: "c", Send: &model.AsyncSend{}, Receive: &model.AsyncReceive{}})
	rec := asyncRecord(map[string]any{"id": 1.0}, map[string]any{"ok": true, "n": 1.0}, map[string]any{"ok": false})

	changes, err := w.LearnSchemas(target, "1", rec)

	if err != nil {
		t.Fatal(err)
	}
	msg, source, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "message"})
	required, _ := msg["required"].([]any)
	if !ok || source != SchemaInferred || !slices.Equal(required, []any{"ok"}) {
		t.Fatalf("message schema %v required %v ok %v", msg, required, ok)
	}
	if len(changes) != 1 {
		t.Fatalf("changes %+v (the second message made n optional)", changes)
	}
	if _, _, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "request"}); !ok {
		t.Fatal("the sent payload learns a request schema")
	}
	parts := w.ListSchemas(target, "1")
	if len(parts) != 2 || parts[0].Kind != "request" || parts[1].Kind != "message" {
		t.Fatalf("parts %+v", parts)
	}
	data, _ := os.ReadFile(asyncFile(w))
	if !strings.Contains(string(data), "boolean") {
		t.Fatalf("the learned payload is in the AsyncAPI description:\n%s", data)
	}
}

func TestSchemaChecker_should_flag_a_received_message_that_breaks_its_schema(t *testing.T) {
	w := demoWorkspace(t)
	target := saveAsync(t, w, "orders/placed", &model.Async{Protocol: "mqtt", Server: "mqtt://b", Channel: "c", Receive: &model.AsyncReceive{}})
	if _, err := w.LearnSchemas(target, "1", asyncRecord(nil, map[string]any{"ok": true})); err != nil {
		t.Fatal(err)
	}
	c, _ := w.loadedCollection(target.Collection)
	_, r, _ := w.requestFor(target)

	notes := schemaChecker{w: w}.Check(c, r, "1", asyncRecord(nil, map[string]any{"ok": "yes"}))

	// As for HTTP: the mismatch is reported, then the schema widens to take the sample.
	if len(notes) != 2 || notes[0].Code != diag.CodeSchemaMessage || notes[1].Code != diag.CodeSchemaUpdated {
		t.Fatalf("notes %v", notes)
	}
}

func TestLearnSchemas_should_learn_the_request_and_each_response_message_of_a_grpc_call(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, "schemas/grpc")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://h:1", Method: "a.B/C"}}}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	rec := engine.Record{Kind: engine.KindGRPC, Request: engine.SentRequest{Body: &engine.Body{Text: `{"id":"7"}`}},
		Messages: []engine.RecordMessage{
			{Direction: "sent"},
			{Direction: "received", Body: engine.Body{JSON: map[string]any{"name": "order-7"}}},
		}}

	if _, err := w.LearnSchemas(target, "1", rec); err != nil {
		t.Fatal(err)
	}

	if _, _, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "request"}); !ok {
		t.Error("the request message was not learned")
	}
	if _, _, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "message"}); !ok {
		t.Error("the response message was not learned")
	}
	if _, _, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "response", Status: 200}); ok {
		t.Error("a gRPC call has no HTTP response schema")
	}
}
