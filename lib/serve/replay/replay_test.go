// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/problem"
)

func TestHandler_should_answer_a_matching_request_with_its_default_example(t *testing.T) {
	f := newFixture(t, spec{
		path:     "auth/login",
		versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/v1/login"}},
		examples: map[string]string{
			"ok":      ex(200, `{"data": {"token": "t-1"}}`),
			"invalid": ex(401, `{"error": "bad password"}`),
		},
	}).materialise().start()

	got := f.do("POST", "/v1/login")

	if got.status != 200 || got.json(t)["data"].(map[string]any)["token"] != "t-1" {
		t.Fatalf("got %d %s", got.status, got.body)
	}
	for header, want := range map[string]string{"X-Flowi-Request": "auth/login", "X-Flowi-Version": "1", "X-Flowi-Example": "ok", "X-Flowi-Source": "example"} {
		if got.header.Get(header) != want {
			t.Errorf("%s = %q, want %q", header, got.header.Get(header), want)
		}
	}
	if ct := got.header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type %q", ct)
	}
}

func TestHandler_should_pick_the_response_asked_for_in_headers(t *testing.T) {
	f := newFixture(t, spec{
		path: "auth/login", versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/v1/login"}},
		examples: map[string]string{"ok": ex(200, `{"ok": true}`), "invalid": ex(401, `{"error": "bad"}`), "locked": ex(401, `{"error": "locked"}`)},
	}).materialise().start()

	byStatus := f.do("POST", "/v1/login", "X-Flowi-Status", "401")
	byName := f.do("POST", "/v1/login", "X-Flowi-Example", "locked")
	missingStatus := f.do("POST", "/v1/login", "X-Flowi-Status", "500")
	missingName := f.do("POST", "/v1/login", "X-Flowi-Example", "nope")
	badStatus := f.do("POST", "/v1/login", "X-Flowi-Status", "abc")

	if byStatus.status != 401 || byStatus.header.Get("X-Flowi-Example") != "invalid" {
		t.Errorf("by status: %d %s", byStatus.status, byStatus.header.Get("X-Flowi-Example"))
	}
	if byName.json(t)["error"] != "locked" {
		t.Errorf("by name: %s", byName.body)
	}
	for name, r := range map[string]reply{"missing status": missingStatus, "missing name": missingName} {
		if r.status != 404 || r.header.Get("X-Flowi-Match") != "no-response" {
			t.Errorf("%s: %d match=%q", name, r.status, r.header.Get("X-Flowi-Match"))
		}
	}
	if badStatus.status != 400 {
		t.Errorf("bad X-Flowi-Status gave %d, want 400", badStatus.status)
	}
}

// Before: a name like ../secret could be joined onto the examples path.
func TestHandler_should_never_read_outside_the_examples_for_a_picked_name(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise()
	f.write(f.dir+"/coll/requests/secret.json", ex(200, `{"leak": true}`))
	f.start()

	for _, name := range []string{"../secret", "../../coll/requests/secret", "/etc/passwd", "ok/../ok"} {
		got := f.do("GET", "/a", "X-Flowi-Example", name)
		if got.status != 404 || strings.Contains(got.body, "leak") {
			t.Errorf("%q: %d %s", name, got.status, got.body)
		}
	}
}

func TestHandler_Given_no_matching_path_Then_404_that_says_so(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise().start()

	got := f.do("GET", "/nothing/here?x=1")

	body := got.json(t)
	if got.status != 404 || got.header.Get("X-Flowi-Match") != "none" || body["method"] != "GET" || body["path"] != "/nothing/here" {
		t.Fatalf("%d %v %v", got.status, got.header, body)
	}
}

func TestHandler_Given_a_path_with_other_methods_Then_405_with_Allow(t *testing.T) {
	f := newFixture(t,
		spec{path: "a/get", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}},
		spec{path: "a/post", versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/a"}}, examples: map[string]string{"ok": ex(201, `{}`)}},
	).materialise().start()

	got := f.do("DELETE", "/a")

	if got.status != 405 || got.header.Get("Allow") != "GET, POST" || got.header.Get("X-Flowi-Match") != "none" {
		t.Fatalf("%d allow=%q", got.status, got.header.Get("Allow"))
	}
}

func TestHandler_should_match_path_variables_and_prefer_the_more_literal_path(t *testing.T) {
	f := newFixture(t,
		spec{path: "users/get", versions: get("{{env.base_url}}/v1/users/{{id}}"), examples: map[string]string{"ok": ex(200, `{"which": "by id"}`)}},
		spec{path: "users/me", versions: get("{{env.base_url}}/v1/users/me"), examples: map[string]string{"ok": ex(200, `{"which": "me"}`)}},
		spec{path: "users/files", versions: get("https://{{host}}/v1/users/{{id}}/files/{{name}}.txt"), examples: map[string]string{"ok": ex(200, `{"which": "file"}`)}},
	).materialise().start()

	byID := f.do("GET", "/v1/users/42").json(t)["which"]
	me := f.do("GET", "/v1/users/me").json(t)["which"]
	file := f.do("GET", "/v1/users/42/files/report.txt").json(t)["which"]
	slash := f.do("GET", "/v1/users/42/").json(t)["which"]
	other := f.do("GET", "/v1/users/42/files/report.csv")

	if byID != "by id" || me != "me" || file != "file" || slash != "by id" || other.status != 404 {
		t.Fatalf("by id %v, me %v, file %v, slash %v, other %d", byID, me, file, slash, other.status)
	}
}

func TestHandler_should_require_literal_query_params_declared_on_the_request(t *testing.T) {
	f := newFixture(t,
		spec{path: "list/active", versions: []ver{{label: "1", method: "GET", url: "{{env.base_url}}/items?state=active", query: []model.KeyValue{{Key: "page", Value: "{{page}}"}}}},
			examples: map[string]string{"ok": ex(200, `{"which": "active"}`)}},
		spec{path: "list/all", versions: get("{{env.base_url}}/items"), examples: map[string]string{"ok": ex(200, `{"which": "all"}`)}},
	).materialise().start()

	active := f.do("GET", "/items?state=active&page=2").json(t)["which"]
	all := f.do("GET", "/items?state=closed").json(t)["which"]

	if active != "active" || all != "all" {
		t.Fatalf("active %v all %v", active, all)
	}
}

// A base URL variable may carry a path prefix (https://x/api) that the
// request's own URL does not repeat.
func TestHandler_should_match_behind_a_prefix_but_prefer_an_exact_path(t *testing.T) {
	f := newFixture(t,
		spec{path: "health", versions: get("{{env.base_url}}/health"), examples: map[string]string{"ok": ex(200, `{"which": "health"}`)}},
		spec{path: "api-health", versions: get("{{env.base_url}}/api/health"), examples: map[string]string{"ok": ex(200, `{"which": "api health"}`)}},
	).materialise().start()

	prefixed := f.do("GET", "/v2/health").json(t)["which"]
	exact := f.do("GET", "/api/health").json(t)["which"]

	if prefixed != "health" || exact != "api health" {
		t.Fatalf("prefixed %v exact %v", prefixed, exact)
	}
}

func TestHandler_should_choose_versions_by_header_then_default(t *testing.T) {
	f := newFixture(t, spec{
		path: "auth/login", def: "2",
		versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/login"}, {label: "2", method: "POST", url: "{{env.base_url}}/login"}},
		examples: map[string]string{
			"v1":  `{"version": "1", "status": 200, "body": {"v": 1}}`,
			"v2":  `{"version": "2", "status": 200, "body": {"v": 2}}`,
			"any": `{"status": 202, "body": {"v": "any"}}`,
		},
	}).materialise().start()

	byDefault := f.do("POST", "/login")
	v1 := f.do("POST", "/login", "X-Flowi-Version", "1")
	unknown := f.do("POST", "/login", "X-Flowi-Version", "9")

	if byDefault.header.Get("X-Flowi-Version") != "2" || byDefault.json(t)["v"] != float64(2) {
		t.Errorf("default: %s %s", byDefault.header.Get("X-Flowi-Version"), byDefault.body)
	}
	if v1.header.Get("X-Flowi-Version") != "1" || v1.json(t)["v"] != float64(1) {
		t.Errorf("v1: %s", v1.body)
	}
	if unknown.status != 404 || unknown.header.Get("X-Flowi-Match") != "none" {
		t.Errorf("unknown version: %d", unknown.status)
	}
}

func TestHandler_should_pick_a_request_by_header_when_paths_are_ambiguous(t *testing.T) {
	f := newFixture(t,
		spec{path: "a/first", versions: get("{{env.base_url}}/same"), examples: map[string]string{"ok": ex(200, `{"which": "first"}`)}},
		spec{path: "b/second", versions: get("{{env.base_url}}/same"), examples: map[string]string{"ok": ex(200, `{"which": "second"}`)}},
	).materialise().start()

	byDefault := f.do("GET", "/same").json(t)["which"]
	picked := f.do("GET", "/same", "X-Flowi-Request", "b/second").json(t)["which"]

	if byDefault != "first" || picked != "second" {
		t.Fatalf("default %v picked %v", byDefault, picked)
	}
}

func TestHandler_should_serve_saved_responses_when_there_are_no_examples(t *testing.T) {
	f := newFixture(t,
		spec{path: "a", versions: get("{{env.base_url}}/a"),
			saved: map[string]map[string]string{"1": {
				"404":  `{"status": 404, "headers": [{"name": "X-Where", "value": "saved"}], "body": {"json": {"gone": true}}}`,
				"200":  `{"status": 200, "body": {"text": "plain words"}}`,
				"last": `{"status": 200, "body": {"text": "plain words"}}`,
			}}},
		spec{path: "b", versions: get("{{env.base_url}}/b"),
			examples: map[string]string{"pinned": ex(200, `{"from": "example"}`)},
			saved:    map[string]map[string]string{"1": {"last": `{"status": 200, "body": {"json": {"from": "saved"}}}`}}},
	).materialise().start()

	text := f.do("GET", "/a")
	gone := f.do("GET", "/a", "X-Flowi-Status", "404")
	both := f.do("GET", "/b")

	if text.status != 200 || text.body != "plain words" || text.header.Get("X-Flowi-Source") != "saved" {
		t.Errorf("text: %d %q %s", text.status, text.body, text.header.Get("X-Flowi-Source"))
	}
	if gone.status != 404 || gone.header.Get("X-Where") != "saved" || gone.json(t)["gone"] != true {
		t.Errorf("gone: %d %v %s", gone.status, gone.header, gone.body)
	}
	if both.json(t)["from"] != "example" {
		t.Errorf("examples must win over saved responses: %s", both.body)
	}
}

func TestHandler_Given_a_request_with_no_responses_Then_404_that_explains(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a")}).materialise().start()

	got := f.do("GET", "/a")

	if got.status != 404 || got.header.Get("X-Flowi-Match") != "no-response" || !strings.Contains(got.json(t)["error"].(string), "pin") {
		t.Fatalf("%d %s", got.status, got.body)
	}
}

func TestHandler_should_simulate_latency_from_header_or_default_and_cap_it(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise()
	f.options.Latency = 40 * time.Millisecond
	f.start()

	f.do("GET", "/a")
	f.do("GET", "/a", "X-Flowi-Latency", "250ms")
	f.do("GET", "/a", "X-Flowi-Latency", "0s")
	f.do("GET", "/a", "X-Flowi-Latency", "10m")
	bad := f.do("GET", "/a", "X-Flowi-Latency", "soon")
	negative := f.do("GET", "/a", "X-Flowi-Latency", "-1s")

	want := []time.Duration{40 * time.Millisecond, 250 * time.Millisecond, 30 * time.Second}
	if len(f.sleeps) != len(want) {
		t.Fatalf("sleeps %v, want %v", f.sleeps, want)
	}
	for i := range want {
		if f.sleeps[i] != want[i] {
			t.Errorf("sleep %d = %v, want %v", i, f.sleeps[i], want[i])
		}
	}
	if bad.status != 400 || negative.status != 400 {
		t.Errorf("bad latency gave %d / %d, want 400", bad.status, negative.status)
	}
}

// Before: header values from a file were copied verbatim.
func TestHandler_should_not_copy_unsafe_or_hop_by_hop_headers_from_files(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": `{
		"status": 200,
		"headers": [
			{"name": "X-Fine", "value": "yes"},
			{"name": "X-Split", "value": "a\r\nSet-Cookie: pwned=1"},
			{"name": "Bad Name", "value": "x"},
			{"name": "Transfer-Encoding", "value": "chunked"},
			{"name": "Content-Length", "value": "9999"},
			{"name": "Content-Encoding", "value": "gzip"},
			{"name": "Connection", "value": "close"},
			{"name": "X-Flowi-Match", "value": "forged"},
			{"name": "Set-Cookie", "value": "sid=1"}
		],
		"body": {"a": 1}}`}}).materialise().start()

	got := f.do("GET", "/a")

	if got.status != 200 || got.body != `{"a":1}` {
		t.Fatalf("%d %q", got.status, got.body)
	}
	if got.header.Get("X-Fine") != "yes" || got.header.Get("Set-Cookie") != "sid=1" {
		t.Errorf("good headers lost: %v", got.header)
	}
	if got.header.Get("X-Flowi-Match") != "ok" {
		t.Errorf("a file forged X-Flowi-Match: %q", got.header.Get("X-Flowi-Match"))
	}
	for _, name := range []string{"X-Split", "Bad Name", "Transfer-Encoding", "Content-Encoding", "Connection"} {
		if v := got.header.Get(name); v != "" {
			t.Errorf("%s copied: %q", name, v)
		}
	}
	if strings.Contains(strings.Join(got.header.Values("Set-Cookie"), "|"), "pwned") {
		t.Error("header splitting got through")
	}
	if got.header.Get("Content-Length") == "9999" {
		t.Error("Content-Length copied")
	}
}

func TestHandler_should_answer_HEAD_and_bodiless_statuses_without_a_body(t *testing.T) {
	f := newFixture(t,
		spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{"big": "body"}`)}},
		spec{path: "b", versions: []ver{{label: "1", method: "DELETE", url: "{{env.base_url}}/b"}}, examples: map[string]string{"gone": ex(204, `{"ignored": true}`)}},
	).materialise().start()

	head := f.do("HEAD", "/a")
	noContent := f.do("DELETE", "/b")

	if head.status != 200 || head.body != "" {
		t.Errorf("HEAD: %d %q", head.status, head.body)
	}
	if noContent.status != 204 || noContent.body != "" {
		t.Errorf("204: %d %q", noContent.status, noContent.body)
	}
}

func TestHandler_Refresh_should_see_new_requests(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise().start()
	before := f.do("GET", "/b").status
	f.specs = append(f.specs, spec{path: "b", versions: get("{{env.base_url}}/b"), examples: map[string]string{"ok": ex(200, `{}`)}})
	f.materialise()

	stale := f.do("GET", "/b").status
	f.h.Refresh()
	fresh := f.do("GET", "/b").status

	if before != 404 || stale != 404 || fresh != 200 {
		t.Fatalf("before %d stale %d fresh %d", before, stale, fresh)
	}
}

func TestHandler_should_add_CORS_headers_when_asked(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise()
	f.options.CORS = true
	f.start()

	preflight := f.do("OPTIONS", "/a", "Origin", "http://app.local", "Access-Control-Request-Method", "GET", "Access-Control-Request-Headers", "x-flowi-status")
	plain := f.do("GET", "/a", "Origin", "http://app.local")

	if preflight.status != 204 || preflight.header.Get("Access-Control-Allow-Origin") != "*" ||
		!strings.Contains(strings.ToLower(preflight.header.Get("Access-Control-Allow-Headers")), "x-flowi-status") {
		t.Errorf("preflight: %d %v", preflight.status, preflight.header)
	}
	for _, method := range []string{"QUERY", "PROPFIND", "MKCALENDAR", "VERSION-CONTROL"} {
		if !slices.Contains(strings.Split(preflight.header.Get("Access-Control-Allow-Methods"), ", "), method) {
			t.Errorf("preflight does not allow %s: %q", method, preflight.header.Get("Access-Control-Allow-Methods"))
		}
	}
	if plain.header.Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(plain.header.Get("Access-Control-Expose-Headers"), "X-Flowi-Example") {
		t.Errorf("plain: %v", plain.header)
	}
}

func TestHandler_should_survive_concurrent_requests(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a/{{id}}"), examples: map[string]string{"ok": ex(200, `{"n": 1}`)}}).materialise()
	f.options.Sequence = true
	f.start()
	done := make(chan int, 40)

	for i := range 40 {
		go func() {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/a/"+itoa(i), nil)
			done <- f.doRequest(req).status
		}()
	}

	for range 40 {
		if status := <-done; status != 200 {
			t.Fatalf("status %d", status)
		}
	}
}

// Before: bodies went through map[string]any, so big integers became
// floats (…456800) and keys were re-sorted.
func TestHandler_should_replay_json_bodies_exactly(t *testing.T) {
	f := newFixture(t,
		spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": `{"status": 200, "body": {"z": 1, "id": 1234567890123456789, "a": 1.50}}`}},
		spec{path: "b", versions: get("{{env.base_url}}/b"), saved: map[string]map[string]string{"1": {"last": `{"status": 200, "body": {"json": {"id": 9007199254740993}}}`}}},
	).materialise().start()

	example := f.do("GET", "/a")
	saved := f.do("GET", "/b")

	if example.body != `{"z":1,"id":1234567890123456789,"a":1.50}` {
		t.Errorf("example body %s", example.body)
	}
	if saved.body != `{"id":9007199254740993}` {
		t.Errorf("saved body %s", saved.body)
	}
}

// Before: any Host header was served, so a web page on a name that DNS
// rebinding points at 127.0.0.1 could read the replayer.
func TestHandler_should_refuse_hosts_that_are_not_allowed(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise()
	f.options.Hosts = []string{"localhost", "127.0.0.1", "::1"}
	f.start()
	cases := map[string]int{"localhost:8525": 200, "127.0.0.1:8525": 200, "LOCALHOST": 200, "[::1]:8525": 200,
		"evil.example": 403, "evil.example:8525": 403, "127.0.0.1.evil.example": 403, "": 403}

	for host, want := range cases {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/a", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		f.h.ServeHTTP(rec, req)

		if rec.Code != want {
			t.Errorf("Host %q: %d, want %d", host, rec.Code, want)
		}
	}
	control := httptest.NewRequestWithContext(context.Background(), "GET", "/__flowi/traffic", nil)
	control.Host = "evil.example"
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, control)
	if rec.Code != 403 {
		t.Errorf("the control endpoints must check the Host too: %d", rec.Code)
	}
}

// Before: the replayer's own errors (no route, no response, an injected fault) were a private JSON shape with the
// content type application/json, which a client built on RFC 9457 could not tell from any other error.
func TestHandler_Given_its_own_errors_Then_they_are_problem_details_documents(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a")}).materialise().start()
	noRoute := f.do("GET", "/nowhere")
	noResponse := f.do("GET", "/a")
	wrongMethod := f.do("DELETE", "/a")

	for name, got := range map[string]struct {
		status int
		header http.Header
		doc    map[string]any
	}{
		"no route":     {noRoute.status, noRoute.header, noRoute.json(t)},
		"no response":  {noResponse.status, noResponse.header, noResponse.json(t)},
		"wrong method": {wrongMethod.status, wrongMethod.header, wrongMethod.json(t)},
	} {
		if violations := problem.Check(got.status, got.header.Get("Content-Type"), got.doc); len(violations) != 0 {
			t.Errorf("%s (%d, %q): %v in %v", name, got.status, got.header.Get("Content-Type"), violations, got.doc)
		}
	}
	if noRoute.json(t)["flowi"] != true {
		t.Errorf("the replayer's own marker must stay: %s", noRoute.body)
	}
}
