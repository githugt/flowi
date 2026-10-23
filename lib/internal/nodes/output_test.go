// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/report"
)

func digestState() (*State, *recorder, *fakeRuntime) {
	s, rec, rt := flowState()
	rt.reports["run"] = report.Run{ID: "R1", FlowName: "Leaky hunter2 flow", Status: "passed", Nodes: []report.Node{{Path: "a", ID: "a", Type: "log", Status: "passed"}}}
	rt.reports["a"] = report.Run{ID: "R1", FlowName: "only a", Status: "passed"}
	rt.httpOut = okResponse()
	return s, rec, rt
}

func TestDigest_should_send_masked_markdown_to_each_sink(t *testing.T) {
	tests := []struct {
		sink   string
		params []any
		check  func(t *testing.T, rec *recorder, rt *fakeRuntime, out Outcome)
	}{
		{sink: "stdout", check: func(t *testing.T, rec *recorder, _ *fakeRuntime, _ Outcome) {
			if len(rec.digests) != 1 || !strings.HasPrefix(rec.digests[0], "d markdown: # Leaky ") {
				t.Fatalf("digests %v", rec.digests)
			}
		}},
		{sink: "file", params: []any{"file", "summary.md"}, check: func(t *testing.T, _ *recorder, rt *fakeRuntime, out Outcome) {
			if out.Outputs["file"] != "digests/summary.md" || !strings.Contains(string(rt.written["digests/summary.md"]), "# Leaky") {
				t.Fatalf("outcome %+v written %v", out, rt.written)
			}
		}},
		{sink: "webhook", params: []any{"url", "http://hooks.test/d"}, check: func(t *testing.T, _ *recorder, rt *fakeRuntime, _ Outcome) {
			body, _ := json.Marshal(rt.https[0].Body)
			if rt.https[0].Method != "POST" || rt.https[0].URL != "http://hooks.test/d" || !strings.HasPrefix(string(body), `{"text":"# Leaky`) {
				t.Fatalf("call %+v body %s", rt.https[0], body)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.sink, func(t *testing.T) {
			s, rec, rt := digestState()
			n := node("digest", append([]any{"sink", tt.sink}, tt.params...)...)
			n.ID = "d"

			out := runOutcome(t, n, s)

			if len(out.Diags) != 0 {
				t.Fatalf("diags %v", out.Diags)
			}
			tt.check(t, rec, rt, out)
			everything := strings.Join(rec.digests, "") + string(rt.written["digests/summary.md"])
			if len(rt.https) > 0 {
				b, _ := json.Marshal(rt.https[0].Body)
				everything += string(b)
			}
			if strings.Contains(everything, "hunter2") {
				t.Fatalf("secret leaked: %s", everything)
			}
		})
	}
}

func TestDigest_Given_format_json_and_of_node_Then_report_of_that_node(t *testing.T) {
	s, rec, _ := digestState()

	runOutcome(t, node("digest", "of", "a", "format", "json"), s)

	if len(rec.digests) != 1 || !strings.Contains(rec.digests[0], `"flow_name": "only a"`) {
		t.Fatalf("digests %v", rec.digests)
	}
}

func TestDigest_Given_unknown_target_Then_FLW_RUN_014(t *testing.T) {
	s, _, _ := digestState()

	out := runOutcome(t, node("digest", "of", "later"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunDigestTarget {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestSave_should_write_each_mode_masked(t *testing.T) {
	tests := []struct {
		mode     string
		wantFile string
		wantData string
	}{
		{mode: "", wantFile: "saved/resp.json", wantData: "{\n  \"pw\": \"••••••\",\n  \"status\": 201\n}\n"},
		{mode: "latest-by-status", wantFile: "saved/resp/201.json", wantData: "{\n  \"pw\": \"••••••\",\n  \"status\": 201\n}\n"},
		{mode: "append", wantFile: "saved/resp.ndjson", wantData: "{\"pw\":\"••••••\",\"status\":201}\n{\"pw\":\"••••••\",\"status\":201}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			s, _, rt := flowState()
			s.Vars.Set("resp", map[string]any{"status": 201, "pw": "hunter2"})
			params := []any{"key", "resp", "value", "{{vars.resp}}"}
			if tt.mode != "" {
				params = append(params, "mode", tt.mode)
			}
			n := node("save", params...)

			out := runOutcome(t, n, s)
			if tt.mode == "append" {
				out = runOutcome(t, n, s)
			}

			if len(out.Diags) != 0 || out.Outputs["file"] != tt.wantFile || string(rt.written[tt.wantFile]) != tt.wantData {
				t.Fatalf("outcome %+v written %q", out, rt.written[tt.wantFile])
			}
		})
	}
}

func TestSave_Given_latest_by_status_without_status_Then_FLW_RUN_018(t *testing.T) {
	s, _, rt := flowState()

	out := runOutcome(t, node("save", "key", "x", "value", "{{'text'}}", "mode", "latest-by-status"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunSaveStatus || len(rt.files) != 0 {
		t.Fatalf("outcome %+v", out)
	}
}

func TestSave_Given_refused_name_Then_FLW_RUN_017(t *testing.T) {
	s, _, rt := flowState()
	rt.fileErr = errors.Join(ErrFileName, errors.New("../x"))

	out := runOutcome(t, node("save", "key", "../x", "value", "1"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunFileName {
		t.Fatalf("outcome %+v", out)
	}
}

func TestSave_Given_write_failure_Then_error_status(t *testing.T) {
	s, _, rt := flowState()
	rt.fileErr = errors.New("disk full")

	out := runOutcome(t, node("save", "key", "x", "value", "1"), s)

	if out.Status != StatusError || len(out.Diags) != 1 {
		t.Fatalf("outcome %+v", out)
	}
}

func TestDigest_webhook_should_shape_the_payload_for_each_style(t *testing.T) {
	tests := []struct {
		style  string
		params []any
		want   string
	}{
		{style: "text", want: `{"text":"# Leaky `},
		{style: "slack", want: `{"text":"# Leaky `},
		{style: "discord", want: `{"content":"# Leaky `},
		{style: "teams", want: `{"@context":"https://schema.org/extensions","@type":"MessageCard","summary":"Flow-I digest","text":"# Leaky `},
		{style: "email", params: []any{"to", "ops@example.com", "subject", "Nightly"}, want: `{"subject":"Nightly","text":"# Leaky `},
	}
	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			s, _, rt := digestState()
			n := node("digest", append([]any{"sink", "webhook", "url", "http://hooks.test/d", "style", tt.style}, tt.params...)...)
			n.ID = "d"

			out := runOutcome(t, n, s)

			if len(out.Diags) != 0 {
				t.Fatalf("diags %v", out.Diags)
			}
			body, _ := json.Marshal(rt.https[0].Body)
			if !strings.Contains(string(body), tt.want) || strings.Contains(string(body), "hunter2") {
				t.Fatalf("body %s want %s", body, tt.want)
			}
			if tt.style == "email" && !strings.Contains(string(body), `"to":"ops@example.com"`) {
				t.Fatalf("no recipient: %s", body)
			}
		})
	}
}

func TestDigest_Given_unknown_style_Then_FLW_NODE_005(t *testing.T) {
	s, _, _ := digestState()
	n := node("digest", "sink", "webhook", "url", "http://hooks.test/d", "style", "pager")
	n.ID = "d"

	out := runOutcome(t, n, s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeNodeParam {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestDigest_Given_email_style_without_recipient_Then_FLW_NODE_005(t *testing.T) {
	s, _, rt := digestState()
	n := node("digest", "sink", "webhook", "url", "http://hooks.test/d", "style", "email")
	n.ID = "d"

	out := runOutcome(t, n, s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeNodeParam || len(rt.https) != 0 {
		t.Fatalf("diags %v calls %d", out.Diags, len(rt.https))
	}
}

func TestDigest_webhook_should_keep_each_style_within_what_its_receiver_accepts(t *testing.T) {
	limits := map[string]int{"discord": 2000, "slack": 39000, "teams": 20000}
	for style, limit := range limits {
		t.Run(style, func(t *testing.T) {
			s, _, rt := digestState()
			rt.reports["run"] = report.Run{ID: "R1", FlowName: strings.Repeat("long ", 20000), Status: "passed"}
			n := node("digest", "sink", "webhook", "url", "http://hooks.test/d", "style", style)
			n.ID = "d"

			out := runOutcome(t, n, s)

			body, _ := json.Marshal(rt.https[0].Body)
			var sent map[string]any
			_ = json.Unmarshal(body, &sent)
			text, _ := sent["content"].(string)
			if text == "" {
				text, _ = sent["text"].(string)
			}
			if len(out.Diags) != 0 || len([]rune(text)) > limit || !strings.HasSuffix(text, "(truncated)") {
				t.Fatalf("diags %v, %d runes (limit %d), ends %q", out.Diags, len([]rune(text)), limit, text[max(0, len(text)-20):])
			}
		})
	}
}

func TestDigest_webhook_Given_the_receiver_refuses_Then_the_node_fails(t *testing.T) {
	s, _, rt := digestState()
	rt.httpOut = HTTPResult{Status: StatusPassed, Response: &Response{Status: 400}}
	n := node("digest", "sink", "webhook", "url", "http://hooks.test/d")
	n.ID = "d"

	out := runOutcome(t, n, s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeHTTPFailed {
		t.Fatalf("a refused digest was reported as delivered: %+v", out)
	}
}
