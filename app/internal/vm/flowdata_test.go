// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func sampleRecord() *engine.Record {
	return &engine.Record{
		Status: 200,
		Request: engine.SentRequest{Method: "POST", URL: "https://x.test/login",
			Header: []engine.Header{{Name: "X-A", Value: "1"}},
			Body:   &engine.Body{Text: `{"u":1}`, JSON: map[string]any{"u": 1}}},
		Header:  []engine.Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    engine.Body{Text: `{"id":7}`, JSON: map[string]any{"id": float64(7)}, ContentType: "application/json", Size: 8},
		Timings: engine.Timings{TTFB: 20 * time.Millisecond, Total: 84 * time.Millisecond},
		Outputs: map[string]any{"token": "t-1"},
	}
}

func partTexts(parts []vm.Part) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Heading + "|" + strings.Join(p.Lines, ";") + "|" + p.Code + "|")
		for _, r := range p.Rows {
			b.WriteString(r.Key + "=" + r.Value + ";")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestFlowRun_should_keep_the_last_record_of_a_node_and_count_instances(t *testing.T) {
	events := []engine.Event{
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.RecordCaptured{Path: "a", Node: "a", Record: engine.Record{Status: 200}},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed},
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.RecordCaptured{Path: "a", Node: "a", Record: engine.Record{Status: 404}},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusFailed},
		engine.RunFinished{Status: engine.StatusFailed},
	}
	r, _, w, _ := runFixture(t, events)

	r.Run("staging", nil)
	w.Wait()

	d, ok := r.Detail("a")
	if !ok || d.Record == nil || d.Record.Status != 404 || d.Instances != 2 {
		t.Fatalf("detail %+v", d)
	}
}

func TestFlowRun_should_keep_the_tries_and_outcomes_of_the_last_instance(t *testing.T) {
	events := []engine.Event{
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusFailed, Tries: 3, Outcomes: []string{"failure"}},
		engine.RunFinished{Status: engine.StatusFailed},
	}
	r, _, w, _ := runFixture(t, events)

	r.Run("staging", nil)
	w.Wait()

	if d, _ := r.Detail("a"); d.Tries != 3 || len(d.Outcomes) != 1 || d.Outcomes[0] != "failure" {
		t.Fatalf("detail %+v", d)
	}
}

func TestFlowRun_should_ignore_records_of_hook_steps_and_subflow_nodes(t *testing.T) {
	events := []engine.Event{
		engine.RecordCaptured{Path: "x/pre:request:0/a", Node: "a", Depth: 1, Record: engine.Record{Status: 500}},
		engine.RecordCaptured{Path: "sub/a", Node: "a", Record: engine.Record{Status: 501}},
		engine.RunFinished{Status: engine.StatusPassed},
	}
	r, _, w, _ := runFixture(t, events)

	r.Run("staging", nil)
	w.Wait()

	if d, ok := r.Detail("a"); ok && d.Record != nil {
		t.Fatalf("record kept: %+v", d.Record)
	}
}

func TestFlowRun_ClearResults_should_forget_details_and_statuses(t *testing.T) {
	events := []engine.Event{
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed},
		engine.RunFinished{Status: engine.StatusPassed},
	}
	r, _, w, seen := runFixture(t, events)
	r.Run("staging", nil)
	w.Wait()

	r.ClearResults()

	if _, ok := r.Detail("a"); ok || seen["a"] != "" {
		t.Fatalf("detail kept, status %q", seen["a"])
	}
}

func TestRenderBody_should_format_json_text_and_raw(t *testing.T) {
	jsonBody := engine.Body{Text: `{"a":1}`, JSON: map[string]any{"a": float64(1)}}
	textBody := engine.Body{Text: "hello"}
	binary := engine.Body{Base64: "AAEC", Size: 3, ContentType: "image/png"}

	pretty, jl, jok := vm.RenderBody(jsonBody, vm.ViewJSON)
	_, _, jsonOfText := vm.RenderBody(textBody, vm.ViewJSON)
	text, _, tok := vm.RenderBody(textBody, vm.ViewText)
	_, _, textOfBinary := vm.RenderBody(binary, vm.ViewText)
	raw, _, rok := vm.RenderBody(binary, vm.ViewRaw)

	if !jok || jl != vm.LangJSON || pretty != "{\n  \"a\": 1\n}" || jsonOfText || !tok || text != "hello" || textOfBinary || !rok || strings.Contains(raw, "AAEC") || !strings.Contains(raw, "image/png") {
		t.Fatalf("pretty %q (%s %v), json of text %v, text %q %v, text of binary %v, raw %q %v", pretty, jl, jok, jsonOfText, text, tok, textOfBinary, raw, rok)
	}
}

func TestRenderBody_should_cut_a_huge_body_and_say_so(t *testing.T) {
	big := engine.Body{Text: strings.Repeat("x", 300*1024)}

	text, _, ok := vm.RenderBody(big, vm.ViewText)

	if !ok || len(text) > 260*1024 || !strings.Contains(text, "left out") {
		t.Fatalf("len %d ok %v", len(text), ok)
	}
}

func TestInputParts_should_describe_the_request_as_sent_and_other_nodes_by_their_parameters(t *testing.T) {
	request := model.Node{ID: "login", Type: "request", Request: &model.RequestCall{}}
	delay := model.Node{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "2s"}}}

	sent := partTexts(vm.InputParts(request, vm.NodeDetail{Status: "passed", Record: sampleRecord()}))
	written := partTexts(vm.InputParts(delay, vm.NodeDetail{Status: "passed"}))
	never := vm.InputParts(delay, vm.NodeDetail{})

	if !strings.Contains(sent, "POST https://x.test/login") || !strings.Contains(sent, "X-A=1") || !strings.Contains(sent, `"u": 1`) {
		t.Fatalf("sent:\n%s", sent)
	}
	if !strings.Contains(written, "duration=2s") || never != nil {
		t.Fatalf("written:\n%s\nnever %v", written, never)
	}
}

func TestOutputParts_should_show_status_headers_body_and_timings_or_the_outputs(t *testing.T) {
	request := model.Node{ID: "login", Type: "request", Request: &model.RequestCall{}}
	other := model.Node{ID: "pick", Type: "set"}

	got := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "passed", Record: sampleRecord()}, vm.ViewJSON))
	outs := partTexts(vm.OutputParts(other, vm.NodeDetail{Status: "passed", Outputs: map[string]any{"token": "t-1"}}, vm.ViewJSON))

	for _, want := range []string{"200", "84ms", "Content-Type=application/json", "\"id\": 7", "token=t-1", "ttfb"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(outs, `"token": "t-1"`) || vm.OutputParts(other, vm.NodeDetail{}, vm.ViewJSON) != nil {
		t.Fatalf("outputs:\n%s", outs)
	}
}

// Before: the Data tab showed what a step produced and not how it ended or on which try.
func TestOutputParts_should_say_which_outcome_the_step_ended_as_and_on_which_try(t *testing.T) {
	request := model.Node{ID: "login", Type: "request", Retries: 2, Request: &model.RequestCall{}}
	other := model.Node{ID: "pick", Type: "set"}

	retried := vm.OutputParts(request, vm.NodeDetail{Status: "failed", Record: sampleRecord(), Tries: 3, Outcomes: []string{"expired-token", "failure"}}, vm.ViewJSON)
	plain := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "passed", Record: sampleRecord(), Tries: 1, Outcomes: []string{"success"}}, vm.ViewJSON))
	quiet := partTexts(vm.OutputParts(other, vm.NodeDetail{Status: "passed", Outputs: map[string]any{"a": "b"}, Tries: 1, Outcomes: []string{"success"}}, vm.ViewJSON))
	skipped := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "skipped"}, vm.ViewJSON))

	if len(retried) < 2 || retried[1].Heading != "Outcome" || retried[1].Lines[0] != "Ended as expired-token, failure · try 3 of 3" {
		t.Fatalf("retried: %+v", retried)
	}
	if !strings.Contains(plain, "Outcome|Ended as success|") || strings.Contains(plain, "try 1") {
		t.Fatalf("a request that ran once names its outcome and no try: %s", plain)
	}
	if strings.Contains(quiet, "Outcome") || strings.Contains(skipped, "Outcome") {
		t.Fatalf("a set step's lone success and a skipped step say nothing: %q %q", quiet, skipped)
	}
}

func TestTestRows_should_list_an_assert_node_and_a_request_nodes_failed_assertions(t *testing.T) {
	assert := model.Node{ID: "chk", Type: "assert", Params: []model.Field{{Key: "check", Value: "status == 200"}}}
	request := model.Node{ID: "login", Type: "request", Request: &model.RequestCall{}}

	passed := vm.TestRows(assert, vm.NodeDetail{Status: "passed"})
	failed := vm.TestRows(assert, vm.NodeDetail{Status: "failed", Diags: []string{"FLW-RUN-001: assertion failed: status == 200"}})
	onRequest := vm.TestRows(request, vm.NodeDetail{Status: "failed", Diags: []string{"FLW-RUN-001: assertion failed: status == 201", "FLW-HTTP-001: other"}})
	none := vm.TestRows(model.Node{Type: "log"}, vm.NodeDetail{Status: "passed"})

	if len(passed) != 1 || !passed[0].Passed || passed[0].Name != "status == 200" {
		t.Fatalf("passed %+v", passed)
	}
	if len(failed) != 1 || failed[0].Passed || !strings.Contains(failed[0].Detail, "assertion failed") {
		t.Fatalf("failed %+v", failed)
	}
	if len(onRequest) != 1 || onRequest[0].Passed || !strings.Contains(onRequest[0].Name, "status == 201") || len(none) != 0 {
		t.Fatalf("on request %+v, none %+v", onRequest, none)
	}
}

func TestMessageRows_Given_a_message_request_Then_messages_are_listed_with_direction_in_order(t *testing.T) {
	rec := engine.Record{Kind: engine.KindAsync, Outcome: "received", Messages: []engine.RecordMessage{
		{Direction: "sent", Body: engine.Body{Text: `{"q":1}`, JSON: map[string]any{"q": float64(1)}, ContentType: "application/json"}},
		{Direction: "received", Body: engine.Body{Text: `hello`, ContentType: "text/plain"}},
		{Direction: "received", Body: engine.Body{Base64: "AAEC", Size: 3, ContentType: "application/octet-stream"}},
	}}

	got := vm.MessageRows(vm.NodeDetail{Status: "passed", Record: &rec})

	if len(got) != 3 || got[0].Direction != "sent" || got[1].Direction != "received" || got[2].Direction != "received" {
		t.Fatalf("rows %+v", got)
	}
	if !strings.Contains(got[0].Text, `"q": 1`) || got[1].Text != "hello" || !strings.Contains(got[2].Text, "3 bytes") {
		t.Fatalf("texts %q %q %q", got[0].Text, got[1].Text, got[2].Text)
	}
}

func TestMessageRows_Given_a_request_that_is_not_message_based_Then_there_are_none(t *testing.T) {
	got := vm.MessageRows(vm.NodeDetail{Status: "passed", Record: sampleRecord()})

	if len(got) != 0 {
		t.Fatalf("rows %+v", got)
	}
}

func TestInputParts_Given_an_http_request_Then_secrets_in_the_input_view_are_masked(t *testing.T) {
	rec := sampleRecord()
	rec.Request.Header = []engine.Header{
		{Name: "Authorization", Value: "Bearer abc.def.ghi"},
		{Name: "Cookie", Value: "sid=1234"},
		{Name: "X-Api-Key", Value: "k-999"},
		{Name: "X-A", Value: "plain"},
	}

	got := partTexts(vm.InputParts(model.Node{ID: "login", Type: "request", Request: &model.RequestCall{}}, vm.NodeDetail{Status: "passed", Record: rec}))

	for _, leaked := range []string{"abc.def.ghi", "sid=1234", "k-999"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("input view shows %q:\n%s", leaked, got)
		}
	}
	if !strings.Contains(got, "Authorization=Bearer ••••") || !strings.Contains(got, "X-A=plain") {
		t.Fatalf("input view:\n%s", got)
	}
}

func TestOutputParts_Given_a_dataset_step_Then_the_status_line_says_the_outcome_and_the_rows(t *testing.T) {
	request := model.Node{ID: "read", Type: "request", Request: &model.RequestCall{}}
	rec := engine.Record{Kind: engine.KindDataset, Outcome: engine.OutcomeOK, Timings: engine.Timings{Total: 12 * time.Millisecond},
		Body: engine.Body{JSON: map[string]any{"row_count": 3, "truncated": false}}}

	got := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "passed", Record: &rec}, vm.ViewJSON))

	if !strings.Contains(got, "ok · 3 rows · 12ms") || strings.Contains(got, "0 ·") {
		t.Fatalf("status line missing in:\n%s", got)
	}
}

func TestOutputParts_Given_a_dataset_step_that_failed_Then_the_status_line_has_no_row_count(t *testing.T) {
	request := model.Node{ID: "read", Type: "request", Request: &model.RequestCall{}}
	rec := engine.Record{Kind: engine.KindDataset, Outcome: engine.OutcomeError, Timings: engine.Timings{Total: 2 * time.Millisecond}}

	got := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "failed", Record: &rec}, vm.ViewJSON))

	if !strings.Contains(got, "error · 2ms") || strings.Contains(got, "rows") {
		t.Fatalf("status line wrong in:\n%s", got)
	}
}

func TestOutputParts_Given_a_failed_grpc_call_Then_the_status_line_has_the_server_message_and_the_trailers_show(t *testing.T) {
	// Before: the call showed "NOT_FOUND · 12ms", and the message and trailers the engine recorded were never shown.
	request := model.Node{ID: "get", Type: "request", Request: &model.RequestCall{}}
	rec := engine.Record{Kind: engine.KindGRPC, GRPCCode: "NOT_FOUND", GRPCMessage: "order 17\nnot found", Timings: engine.Timings{Total: 12 * time.Millisecond},
		Trailers: []engine.Header{{Name: "x-request-id", Value: "abc"}}}

	got := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "failed", Record: &rec}, vm.ViewJSON))

	if !strings.Contains(got, "NOT_FOUND · order 17 not found · 12ms") {
		t.Fatalf("message missing in:\n%s", got)
	}
	if !strings.Contains(got, "Trailers") || !strings.Contains(got, "x-request-id") {
		t.Fatalf("trailers missing in:\n%s", got)
	}
}

func TestOutputParts_Given_a_grpc_call_with_no_message_Then_the_status_line_is_as_before(t *testing.T) {
	request := model.Node{ID: "get", Type: "request", Request: &model.RequestCall{}}
	rec := engine.Record{Kind: engine.KindGRPC, GRPCCode: "OK", Timings: engine.Timings{Total: 3 * time.Millisecond}}

	got := partTexts(vm.OutputParts(request, vm.NodeDetail{Status: "passed", Record: &rec}, vm.ViewJSON))

	if !strings.Contains(got, "OK · 3ms") || strings.Contains(got, "Trailers") {
		t.Fatalf("wrong output:\n%s", got)
	}
}

func TestRenderBody_Given_json_text_Then_the_json_view_keeps_the_servers_key_order_and_characters(t *testing.T) {
	// Before: the view was a re-serialisation of the parsed map: keys were sorted and & and < were written as & and <.
	text := `{"next":"https://api/x?page=2&per=50","b":1,"a":"<ok>"}`
	body := engine.Body{Text: text, JSON: map[string]any{"next": "https://api/x?page=2&per=50", "b": 1.0, "a": "<ok>"}}

	got, lang, ok := vm.RenderBody(body, vm.ViewJSON)

	if !ok || lang != vm.LangJSON {
		t.Fatalf("ok %v lang %q", ok, lang)
	}
	if strings.Contains(got, "\\u0026") || strings.Contains(got, "\\u003c") {
		t.Fatalf("characters were escaped:\n%s", got)
	}
	if strings.Index(got, `"next"`) > strings.Index(got, `"b"`) || strings.Index(got, `"b"`) > strings.Index(got, `"a"`) {
		t.Fatalf("the key order changed:\n%s", got)
	}
}

func TestRenderBody_Given_only_a_parsed_value_Then_it_is_indented_without_escaping(t *testing.T) {
	body := engine.Body{JSON: map[string]any{"url": "a?b=1&c=2"}}

	got, _, ok := vm.RenderBody(body, vm.ViewJSON)

	if !ok || !strings.Contains(got, `"a?b=1&c=2"`) {
		t.Fatalf("got %q (ok %v)", got, ok)
	}
}

func TestRenderBody_Given_a_long_text_with_multibyte_characters_Then_the_cut_does_not_split_one(t *testing.T) {
	// Before: s[:bodyShowLimit] could end in the middle of a UTF-8 character.
	line := strings.Repeat("é", 49) + "\n" // 99 bytes per line
	text := strings.Repeat(line, 4000)     // about 396 KB, over the 256 KiB limit

	got, _, _ := vm.RenderBody(engine.Body{Text: text}, vm.ViewText)

	if !utf8.ValidString(got) {
		t.Fatal("the cut text is not valid UTF-8")
	}
	if !strings.Contains(got, "more bytes left out") {
		t.Fatal("the cut is not announced")
	}
	body := got[:strings.LastIndex(got, "\n…")]
	if !strings.HasSuffix(text[:len(body)+1], "\n") && !strings.HasSuffix(body, "é") {
		t.Fatalf("the cut did not fall on a line end: ...%q", body[len(body)-5:])
	}
}

func TestRecordSize_Given_a_thousand_bytes_Then_it_is_counted_in_the_same_units_as_the_response_pane(t *testing.T) {
	// Before: the dock counted in 1024 and wrote "1000 B" where the response pane wrote "1.0 KB" for the same body.
	rec := engine.Record{Body: engine.Body{Size: 1000}}

	got := vm.RecordSize(vm.NodeDetail{Record: &rec})

	if got != "1.0 KB" || got != vm.SizeText(1000) {
		t.Fatalf("size %q", got)
	}
}
