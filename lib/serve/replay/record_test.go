// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/serve/replay"
)

type upstream struct {
	srv  *httptest.Server
	mu   sync.Mutex
	seen []*http.Request
	body []string
}

func newUpstream(t *testing.T, h http.HandlerFunc) *upstream {
	t.Helper()
	u := &upstream{}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		u.mu.Lock()
		u.seen = append(u.seen, r)
		u.body = append(u.body, string(b))
		u.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func recorderFor(t *testing.T, u *upstream) *replay.Recorder {
	t.Helper()
	base, err := url.Parse(u.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &replay.Recorder{Upstream: base}
}

func recordingFixture(t *testing.T, u *upstream) *fixture {
	f := newFixture(t,
		spec{path: "auth/login", versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/v1/login"}}},
		spec{path: "users/get", versions: get("{{env.base_url}}/v1/users/{{id}}")},
	).materialise()
	f.options.Record = recorderFor(t, u)
	f.options.Log = replay.NewLog(50)
	return f.start()
}

func (f *fixture) savedFile(req, version, name string) string {
	return filepath.Join(f.dir, "data", "C1", "responses", req, version, name+".json")
}

func TestRecord_should_forward_matched_calls_answer_with_the_real_response_and_save_it(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "sid=secret-session; HttpOnly")
		w.Header().Set("X-Request-Id", "abc")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data": {"token": "t-9"}}`)
	})
	f := recordingFixture(t, up)

	got := f.do("POST", "/v1/login?x=1", "Authorization", "Bearer abc", "X-Flowi-Status", "500", "Accept-Encoding", "br")

	if got.status != 201 || got.json(t)["data"].(map[string]any)["token"] != "t-9" || got.header.Get("X-Request-Id") != "abc" {
		t.Fatalf("answer: %d %s %v", got.status, got.body, got.header)
	}
	if got.header.Get("X-Flowi-Match") != "recorded" || got.header.Get("X-Flowi-Request") != "auth/login" {
		t.Errorf("headers %v", got.header)
	}
	up.mu.Lock()
	defer up.mu.Unlock()
	seen := up.seen[0]
	if seen.Method != "POST" || seen.URL.RawQuery != "x=1" || seen.Header.Get("Authorization") != "Bearer abc" {
		t.Errorf("upstream saw %s %s auth=%q", seen.Method, seen.URL, seen.Header.Get("Authorization"))
	}
	if seen.Header.Get("X-Flowi-Status") != "" || seen.Header.Get("Accept-Encoding") == "br" {
		t.Errorf("steering/encoding headers were forwarded: %v", seen.Header)
	}
	for _, name := range []string{"201", "last"} {
		data, err := os.ReadFile(f.savedFile("RA", "1", name))
		if err != nil {
			t.Fatalf("%s.json not saved: %v", name, err)
		}
		if !strings.Contains(string(data), `"t-9"`) {
			t.Errorf("%s.json lacks the body: %s", name, data)
		}
		// Behaviour change: response headers used to be redacted by name, which hid
		// a Set-Cookie the user wanted to see and was inconsistent with a normal
		// send. They are kept as received (the data dir is not committed); a
		// request credential is saved as a secret placeholder, never as a value.
		if !strings.Contains(string(data), "secret-session") {
			t.Errorf("%s.json lost the response header: %s", name, data)
		}
		if strings.Contains(string(data), "Bearer abc") || !strings.Contains(string(data), "{{secret.authorization}}") {
			t.Errorf("%s.json holds the request credential, or not its placeholder: %s", name, data)
		}
		if strings.Contains(string(data), "X-Flowi-Status") {
			t.Errorf("%s.json holds the replayer's own header: %s", name, data)
		}
	}
}

func TestRecord_round_trip_replays_what_was_recorded(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 7}`)
	})
	f := recordingFixture(t, up)
	f.do("GET", "/v1/users/7")
	replayer := replay.New(replay.Options{Collections: f.options.Collections, DataDir: f.options.DataDir})

	req := httptest.NewRequestWithContext(t.Context(), "GET", "/v1/users/7", nil)
	rec := httptest.NewRecorder()
	replayer.ServeHTTP(rec, req)

	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"id":7}` || rec.Header().Get("X-Flowi-Source") != "saved" {
		t.Fatalf("%d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
}

func TestRecord_should_forward_unmatched_calls_without_saving(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hello") })
	f := recordingFixture(t, up)

	got := f.do("GET", "/somewhere/else")

	if got.status != 200 || got.body != "hello" || got.header.Get("X-Flowi-Match") != "forwarded" {
		t.Fatalf("%d %q %v", got.status, got.body, got.header)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "data")); !os.IsNotExist(err) {
		t.Errorf("something was saved for an unmatched call: %v", err)
	}
}

func TestRecord_should_decode_gzip_and_keep_binary_bodies_intact(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users/1" {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/json")
			zw := gzip.NewWriter(w)
			_, _ = io.WriteString(zw, `{"zipped": true}`)
			_ = zw.Close()
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0xff, 0xfe, 0x00, 0x01})
	})
	f := recordingFixture(t, up)

	zipped := f.do("GET", "/v1/users/1")
	binary := f.do("GET", "/v1/users/2")
	replayed := replay.New(replay.Options{Collections: f.options.Collections, DataDir: f.options.DataDir})
	rec := httptest.NewRecorder()
	replayed.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/v1/users/2", nil))

	if zipped.json(t)["zipped"] != true || zipped.header.Get("Content-Encoding") != "" {
		t.Errorf("zipped: %q %v", zipped.body, zipped.header)
	}
	if binary.body != "\xff\xfe\x00\x01" || rec.Body.String() != "\xff\xfe\x00\x01" {
		t.Errorf("binary: live %q replayed %q", binary.body, rec.Body.String())
	}
}

func TestRecord_Given_an_unreachable_upstream_Then_502_that_says_so(t *testing.T) {
	up := newUpstream(t, func(http.ResponseWriter, *http.Request) {})
	f := recordingFixture(t, up)
	up.srv.Close()

	got := f.do("GET", "/v1/users/1")

	if got.status != 502 || got.header.Get("X-Flowi-Match") != "upstream-error" || !strings.Contains(got.body, "upstream") {
		t.Fatalf("%d %s %v", got.status, got.body, got.header)
	}
}

func TestRecord_should_not_follow_redirects_and_should_cap_big_bodies(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/users/1" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("x", 100))
	})
	f := newFixture(t, spec{path: "users/get", versions: get("{{env.base_url}}/v1/users/{{id}}")}).materialise()
	rec := recorderFor(t, up)
	rec.MaxBody = 50
	f.options.Record = rec
	f.start()

	redirect := f.do("GET", "/v1/users/1")
	big := f.do("GET", "/v1/users/2")

	if redirect.status != 302 || redirect.header.Get("Location") != "/elsewhere" {
		t.Errorf("redirect: %d %v", redirect.status, redirect.header)
	}
	if big.status != 502 || !strings.Contains(big.body, "larger than") {
		t.Errorf("big: %d %s", big.status, big.body)
	}
}

func TestRecord_should_log_recorded_and_forwarded_calls(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "{}") })
	f := recordingFixture(t, up)

	f.do("GET", "/v1/users/1")
	f.do("GET", "/other")

	entries := f.options.Log.Entries()
	if len(entries) != 2 || entries[0].Match != "recorded" || entries[0].Request != "users/get" || entries[1].Match != "forwarded" {
		t.Fatalf("%+v", entries)
	}
}

// Before: path.Join cleaned the joined path, so /../admin left the
// upstream's /api prefix.
func TestRecord_should_keep_calls_inside_the_upstream_path_prefix(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "{}") })
	f := newFixture(t, spec{path: "users/get", versions: get("{{env.base_url}}/v1/users/{{id}}")}).materialise()
	base, _ := url.Parse(up.srv.URL + "/api")
	f.options.Record = &replay.Recorder{Upstream: base}
	f.start()

	f.do("GET", "/v1/users/1")
	f.do("GET", "/../../admin")
	f.do("GET", "/v1/users/../../../admin")

	up.mu.Lock()
	defer up.mu.Unlock()
	var paths []string
	for _, r := range up.seen {
		paths = append(paths, r.URL.Path)
	}
	if strings.Join(paths, " ") != "/api/v1/users/1 /api/admin /api/admin" {
		t.Fatalf("upstream saw %v", paths)
	}
}

// Before: a request id or version label from a cloned collection was joined
// into the data dir path, so "../.." made record mode write (and the
// replayer read) outside it.
func TestRecord_and_replay_should_ignore_requests_whose_ids_are_not_plain_names(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"ok":true}`) })
	f := newFixture(t, spec{path: "evil", versions: get("{{env.base_url}}/evil")}).materialise()
	f.specs[0].id = "../../../escaped"
	f.options.Record = recorderFor(t, up)
	f.start()
	outside := filepath.Join(f.dir, "escaped")

	recorded := f.do("GET", "/evil")

	if recorded.header.Get("X-Flowi-Match") != "forwarded" {
		t.Errorf("a request with an unsafe id was treated as matched: %v", recorded.header)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("record mode wrote outside the data dir: %v", err)
	}
	f.write(filepath.Join(outside, "1", "200.json"), `{"status": 200, "body": {"json": {"leaked": true}}}`)
	f.options.Record = nil
	f.start()

	got := f.do("GET", "/evil")

	if got.status != 404 || strings.Contains(got.body, "leaked") {
		t.Fatalf("the replayer read outside the data dir: %d %s", got.status, got.body)
	}
}

func TestRecord_should_save_json_numbers_exactly(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 9007199254740993}`)
	})
	f := recordingFixture(t, up)
	f.do("GET", "/v1/users/1")

	data, err := os.ReadFile(f.savedFile("RB", "1", "last"))

	if err != nil || !strings.Contains(string(data), "9007199254740993") {
		t.Fatalf("%v: %s", err, data)
	}
}

// captureInto makes Capture add a request for each new call to the fixture's
// collection, the way the workspace does (the id segment as {{id}}, the query
// not part of the URL), and records the calls it captured.
func captureInto(f *fixture, calls *[]replay.Call) replay.Capture {
	return func(c replay.Call) (replay.Captured, error) {
		*calls = append(*calls, c)
		url := strings.Replace(strings.SplitN(c.URL, "?", 2)[0], "/7", "/{{id}}", 1)
		f.specs = append(f.specs, spec{path: "recorded/" + strings.ToLower(c.Method) + "-things", versions: []ver{{label: "1", method: c.Method, url: url}}})
		coll := f.collections()[0]
		req := coll.Requests[len(coll.Requests)-1]
		return replay.Captured{Collection: coll, Request: req, Version: req.Versions[0]}, nil
	}
}

func TestRecord_Given_Capture_Then_a_call_no_request_matches_becomes_a_request_and_is_saved_under_it(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 7}`)
	})
	f := newFixture(t, spec{path: "auth/login", versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/v1/login"}}}).materialise()
	f.options.Record = recorderFor(t, up)
	var calls []replay.Call
	f.options.Record.Capture = captureInto(f, &calls)
	f.start()

	first := f.do("GET", "/v1/things/7?x=1", "Authorization", "Bearer abc", "X-Flowi-Status", "500")
	second := f.do("GET", "/v1/things/7")

	if first.status != 200 || first.header.Get("X-Flowi-Match") != "recorded" || second.header.Get("X-Flowi-Match") != "recorded" {
		t.Fatalf("answers: %v / %v", first.header, second.header)
	}
	if len(calls) != 1 || !strings.HasSuffix(calls[0].URL, "/v1/things/7?x=1") || calls[0].Header.Get("Authorization") != "Bearer abc" {
		t.Fatalf("captured %d calls: %+v", len(calls), calls)
	}
	data, err := os.ReadFile(f.savedFile("RB", "1", "last"))
	if err != nil || !strings.Contains(string(data), `"id": 7`) {
		t.Fatalf("saved under the new request: %v %s", err, data)
	}
}

func TestRecord_Given_Capture_fails_Then_the_call_is_still_forwarded_and_nothing_is_saved(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hello") })
	f := newFixture(t).materialise()
	f.options.Record = recorderFor(t, up)
	f.options.Record.Capture = func(replay.Call) (replay.Captured, error) {
		return replay.Captured{}, errors.New("read-only collection")
	}
	f.start()

	got := f.do("GET", "/anything")

	if got.status != 200 || got.body != "hello" || got.header.Get("X-Flowi-Match") != "forwarded" {
		t.Fatalf("%d %q %v", got.status, got.body, got.header)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "data")); !os.IsNotExist(err) {
		t.Errorf("something was saved: %v", err)
	}
}
