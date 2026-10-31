// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

func recordEvents(events []Event) []RecordCaptured {
	var out []RecordCaptured
	for _, ev := range events {
		if rc, ok := ev.(RecordCaptured); ok {
			out = append(out, rc)
		}
	}
	return out
}

func TestRecordCaptured_should_be_emitted_for_a_request_node_with_the_masked_record(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("auth/login", model.RequestVersion{
		Method: "POST", URL: "{{env.base_url}}/login",
		Body: model.Body{JSON: `{"password":"{{secret.password}}"}`},
	})
	fl := f.addFlow("record", requestNode("login", r))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	got := recordEvents(events)
	if res.Status != StatusPassed || len(got) != 1 || got[0].Node != "login" || got[0].Path != "login" || got[0].Record.Status != 200 {
		t.Fatalf("status %s, events %+v", res.Status, got)
	}
	data, _ := json.Marshal(got[0].Record)
	if strings.Contains(string(data), secretV) || !strings.Contains(got[0].Record.Request.Body.Text, vault.Mask) {
		t.Fatalf("record is not masked: %s", data)
	}
}

func TestRecordCaptured_should_not_be_emitted_for_a_node_that_sent_nothing(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("none", requestNode("call", model.Request{Meta: model.Meta{ID: "01JZ0000000000000000000000"}}))

	events, _ := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if got := recordEvents(events); len(got) != 0 {
		t.Fatalf("events %+v", got)
	}
}

func TestRunResult_Given_a_request_node_Then_the_result_keeps_the_masked_record_of_each_call(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("auth/login", model.RequestVersion{
		Method: "POST", URL: "{{env.base_url}}/login",
		Body: model.Body{JSON: `{"password":"{{secret.password}}"}`},
	})
	fl := f.addFlow("record", requestNode("login", r))

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	data, _ := json.Marshal(res.Captured)
	if len(res.Captured) != 1 || res.Captured[0].Status != 200 || strings.Contains(string(data), secretV) {
		t.Fatalf("captured %s", data)
	}
}

type cannedTransport struct{ urls []string }

func (c *cannedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.urls = append(c.urls, req.URL.String())
	body := `{"from":"transport"}`
	return &http.Response{
		StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req,
	}, nil
}

func TestRun_Given_a_transport_in_the_deps_Then_calls_go_through_it_and_not_to_the_server(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("auth/login", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/v1/login"})
	fl := f.addFlow("canned", requestNode("login", r))
	canned := &cannedTransport{}
	deps := f.deps()
	deps.Transport = canned

	// Act
	_, res := runFlow(testCtx(t), t, deps, f.runInput(fl))

	// Assert
	if res.Status != StatusPassed || len(canned.urls) != 1 || len(res.Captured) != 1 || res.Captured[0].Body.JSON == nil {
		t.Fatalf("status %s urls %v captured %+v diags %v", res.Status, canned.urls, res.Captured, res.Diags)
	}
}
