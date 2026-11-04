// SPDX-License-Identifier: Apache-2.0

package discover_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/discover"
)

// fast is a policy with no pause, so a test does not wait.
func fast(start ...string) discover.Policy {
	p := discover.DefaultPolicy()
	p.Interval = 0
	p.Start = start
	return p
}

type recorder struct {
	mu      sync.Mutex
	methods []string
	agents  []string
	auth    []string
	paths   []string
}

func (r *recorder) note(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.methods = append(r.methods, req.Method)
	r.agents = append(r.agents, req.Header.Get("User-Agent"))
	r.auth = append(r.auth, req.Header.Get("Authorization"))
	r.paths = append(r.paths, req.URL.Path)
}

func jsonHandler(rec *recorder, routes map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.note(r)
		body, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

func TestRun_should_follow_links_on_the_same_host_and_only_ever_send_GET(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(jsonHandler(rec, map[string]string{
		"/":             `{"pets":"/pets","other":"https://elsewhere.example/x"}`,
		"/pets":         `[{"id":1,"self":"/pets/1"}]`,
		"/pets/1":       `{"id":1}`,
		"/openapi.json": `{"openapi":"3.0.0"}`,
	}))
	defer srv.Close()

	got, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{}, fast("/", "/openapi.json"), nil)

	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, f := range got.Findings {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"/", "/openapi.json", "/pets", "/pets/1"}) {
		t.Fatalf("paths %v", paths)
	}
	for _, m := range rec.methods {
		if m != http.MethodGet {
			t.Fatalf("a discovery sent %s", m)
		}
	}
	for _, a := range rec.agents {
		if a != "flowi" {
			t.Fatalf("user agent %q", a)
		}
	}
	if got.Findings[1].Kind != "spec" || got.Findings[2].Kind != "json" {
		t.Fatalf("kinds %+v", got.Findings)
	}
}

func TestRun_Given_a_link_that_logs_out_Then_it_is_not_followed(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(jsonHandler(rec, map[string]string{"/": `{"out":"/logout","users":"/users"}`, "/users": `[]`}))
	defer srv.Close()

	_, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{}, fast("/"), nil)

	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(rec.paths, "/logout") || !slices.Contains(rec.paths, "/users") {
		t.Fatalf("asked for %v", rec.paths)
	}
}

func TestRun_Given_the_request_cap_Then_it_stops_and_says_so(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(jsonHandler(rec, map[string]string{}))
	defer srv.Close()
	p := fast("/a", "/b", "/c", "/d")
	p.MaxRequests = 2

	got, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{}, p, nil)

	if err != nil || len(rec.paths) != 2 || got.Stopped == "" {
		t.Fatalf("sent %v stopped %q err %v", rec.paths, got.Stopped, err)
	}
}

func TestRun_Given_a_429_Then_it_stops_at_once(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	got, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{}, fast("/a", "/b"), nil)

	if err != nil || n != 1 || got.Stopped == "" {
		t.Fatalf("requests %d stopped %q err %v", n, got.Stopped, err)
	}
}

func TestRun_Given_a_redirect_to_another_host_Then_the_credential_does_not_travel(t *testing.T) {
	other := &recorder{}
	elsewhere := httptest.NewServer(jsonHandler(other, map[string]string{"/": `{}`}))
	defer elsewhere.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL, http.StatusFound)
	}))
	defer srv.Close()

	got, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{Header: "Authorization", Value: "Bearer s3cret"}, fast("/"), nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(other.paths) != 0 {
		t.Fatalf("the other host was asked: %v", other.paths)
	}
	if got.Findings[0].Status != 0 {
		t.Fatalf("finding %+v", got.Findings[0])
	}
}

func TestRun_should_send_the_credential_to_the_base_host(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(jsonHandler(rec, map[string]string{"/": `{}`}))
	defer srv.Close()

	_, err := discover.Run(context.Background(), srv.Client(), srv.URL, discover.Auth{Header: "Authorization", Value: "Bearer t"}, fast("/"), nil)

	if err != nil || rec.auth[0] != "Bearer t" {
		t.Fatalf("auth %v err %v", rec.auth, err)
	}
}

func TestPlan_should_list_the_start_paths_without_asking_anything(t *testing.T) {
	got, err := discover.Plan("https://api.example.com", discover.DefaultPolicy())

	if err != nil || len(got) == 0 || got[0].Path != "/" || got[0].Why == "" {
		t.Fatalf("plan %+v err %v", got, err)
	}
}

func TestPlan_Given_a_url_that_is_not_http_Then_it_is_refused(t *testing.T) {
	_, err := discover.Plan("ftp://host", discover.DefaultPolicy())

	if err == nil {
		t.Fatal("want an error")
	}
}
