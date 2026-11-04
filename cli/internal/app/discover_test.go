// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func countingServer(t *testing.T, bodies map[string]string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func TestDiscover_Given_no_yes_Then_it_lists_the_requests_and_sends_nothing(t *testing.T) {
	e := newSendEnv(t)
	srv, n := countingServer(t, map[string]string{"/": `{}`})

	code, out, errOut := e.run(context.Background(), "discover", srv.URL)

	if code != 0 || !strings.Contains(out, "Dry run") || !strings.Contains(out, "/health") || n.Load() != 0 {
		t.Fatalf("exit %d, %d requests\n%s\n%s", code, n.Load(), out, errOut)
	}
}

func TestDiscover_Given_yes_and_a_collection_Then_json_answers_become_requests(t *testing.T) {
	e := newSendEnv(t)
	srv, _ := countingServer(t, map[string]string{"/": `{"pets":"/pets"}`, "/pets": `[{"id":1}]`})

	code, out, errOut := e.run(context.Background(), "discover", srv.URL, "--yes", "--rate", "1000", "--path", "/", "--collection", "Tools")

	if code != 0 || !strings.Contains(out, "Drafted 2 requests") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	_, list, _ := e.run(context.Background(), "ls", "requests")
	if !strings.Contains(list, "discovered/pets") {
		t.Fatalf("the drafted request is missing:\n%s", list)
	}
}

func TestDiscover_Given_nothing_answers_Then_exit_1(t *testing.T) {
	e := newSendEnv(t)
	srv, _ := countingServer(t, map[string]string{})

	code, out, _ := e.run(context.Background(), "discover", srv.URL, "--yes", "--rate", "1000", "--path", "/")

	if code != 1 || !strings.Contains(out, "Nothing answered") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestDiscover_Given_a_collection_without_yes_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "discover", "http://127.0.0.1:1", "--collection", "Tools")

	if code != 2 || !strings.Contains(errOut, "--yes") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestDiscover_Given_a_token_variable_that_is_not_set_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "discover", "http://127.0.0.1:1", "--yes", "--token-env", "NO_SUCH_TOKEN")

	if code != 2 || !strings.Contains(errOut, "NO_SUCH_TOKEN") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestDiscover_Given_a_grpc_address_without_yes_Then_it_says_it_would_ask_by_reflection_and_connects_to_nothing(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "discover", "grpc://127.0.0.1:1")

	if code != 0 || !strings.Contains(out, "Dry run") || !strings.Contains(out, "reflection") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestDiscover_Given_a_grpc_address_with_yes_and_a_collection_Then_each_method_becomes_a_request(t *testing.T) {
	e := newSendEnv(t)
	addr := startShopServer(t)

	code, out, errOut := e.run(context.Background(), "discover", "grpc://"+addr, "--yes", "--collection", "Tools")

	if code != 0 || !strings.Contains(out, "shop.Shop/Get") || !strings.Contains(out, "Drafted 2 requests") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	_, list, _ := e.run(context.Background(), "ls", "requests")
	if !strings.Contains(list, "discovered/grpc-shop.Shop-Fail") {
		t.Fatalf("the drafted request is missing:\n%s", list)
	}
}

func TestDiscover_Given_a_grpc_server_without_reflection_Then_exit_1_with_the_reason(t *testing.T) {
	e := newSendEnv(t)
	srv, _ := countingServer(t, map[string]string{})

	code, _, errOut := e.run(context.Background(), "discover", "grpc://"+strings.TrimPrefix(srv.URL, "http://"), "--yes")

	if code == 0 || errOut == "" {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestBaseline_Given_a_discovered_request_Then_its_schemas_are_kept_as_the_drift_baseline(t *testing.T) {
	e := newSendEnv(t)
	srv, _ := countingServer(t, map[string]string{"/pets": `[{"id":1}]`})
	e.run(context.Background(), "discover", srv.URL, "--yes", "--rate", "1000", "--path", "/pets", "--collection", "Tools")

	code, out, errOut := e.run(context.Background(), "baseline", "discovered/pets", "--collection", "Tools")

	if code != 0 || !strings.Contains(out, "Kept 1 schema") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	_, again, _ := e.run(context.Background(), "baseline", "discovered/pets", "--collection", "Tools")
	if !strings.Contains(again, "Nothing to keep") {
		t.Fatalf("second run:\n%s", again)
	}
}
