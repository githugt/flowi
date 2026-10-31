// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"
	"time"

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
