// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/serve/replay"
)

func captureFixture(t *testing.T, capture bool) (*fixture, *replay.Log) {
	t.Helper()
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{"pets":2}`)}}).materialise()
	log := replay.NewLog(10)
	f.options.Log = log
	f.options.Capture = capture
	return f.start(), log
}

func postTo(f *fixture, body string, headers ...string) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/a", strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	f.doRequest(req)
}

func TestHandler_Given_capture_on_When_called_Then_the_entry_keeps_what_was_sent_and_answered(t *testing.T) {
	f, log := captureFixture(t, true)

	postTo(f, `{"name":"rex"}`, "Content-Type", "application/json", "X-Trace", "abc")

	got := log.Entries()[0].Exchange
	if got == nil {
		t.Fatal("no capture")
	}
	if got.Sent.Body != `{"name":"rex"}` || got.Sent.Header.Get("X-Trace") != "abc" {
		t.Errorf("sent = %+v", got.Sent)
	}
	if !strings.Contains(got.Got.Body, `"pets"`) || got.Got.Header.Get("X-Flowi-Match") != "ok" {
		t.Errorf("got = %+v", got.Got)
	}
}

func TestHandler_Given_capture_on_When_the_call_carries_credentials_Then_their_values_are_masked(t *testing.T) {
	f, log := captureFixture(t, true)

	postTo(f, ``, "Authorization", "Bearer s3cret", "Cookie", "sid=1", "X-Api-Key", "k", "Accept", "*/*")

	sent := log.Entries()[0].Exchange.Sent.Header
	for _, name := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		if v := sent.Get(name); v != replay.Masked {
			t.Errorf("%s = %q, want masked", name, v)
		}
	}
	if sent.Get("Accept") != "*/*" {
		t.Errorf("Accept = %q, want it kept", sent.Get("Accept"))
	}
}

func TestHandler_Given_capture_off_When_called_Then_nothing_is_kept(t *testing.T) {
	f, log := captureFixture(t, false)

	postTo(f, `{"a":1}`)

	if log.Entries()[0].Exchange != nil {
		t.Fatal("captured with Capture off")
	}
}

func TestHandler_Given_capture_on_When_the_body_is_binary_or_large_Then_it_is_described_or_cut(t *testing.T) {
	f, log := captureFixture(t, true)

	postTo(f, "\x00\x01\x02")
	postTo(f, strings.Repeat("x", replay.CaptureLimit+10))

	entries := log.Entries()
	if b := entries[0].Exchange.Sent.Body; b != "(binary, 3 bytes)" {
		t.Errorf("binary body = %q", b)
	}
	if m := entries[1].Exchange.Sent; len(m.Body) != replay.CaptureLimit || !m.Truncated {
		t.Errorf("large body: len %d truncated %v", len(m.Body), m.Truncated)
	}
}

func TestHandler_Given_capture_on_When_the_traffic_endpoint_is_read_Then_it_leaves_the_capture_out(t *testing.T) {
	// The endpoint answers anyone who can reach the port; what callers sent stays inside the app.
	f, _ := captureFixture(t, true)
	postTo(f, `{"secret":"x"}`, "X-Trace", "abc")

	served := f.do("GET", "/__flowi/traffic")

	if strings.Contains(served.body, "secret") || strings.Contains(served.body, "abc") {
		t.Fatalf("traffic endpoint leaked the capture: %s", served.body)
	}
}
