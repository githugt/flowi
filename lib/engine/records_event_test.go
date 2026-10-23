// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
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
