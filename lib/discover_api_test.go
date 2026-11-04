// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/discover"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/grpcx"
)

func discoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodies := map[string]string{"/": `{"pets":"/pets"}`, "/pets": `[{"id":1,"name":"Rex"}]`, "/openapi.json": `{"openapi":"3.0.0"}`}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSaveDiscovery_should_draft_a_request_per_json_answer_with_its_shape(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, err := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/", "/openapi.json", "/missing"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.SaveDiscovery(flow.Collection, srv.URL, rep)

	if err != nil || len(got.Created) != 2 {
		t.Fatalf("created %+v skipped %v err %v", got.Created, got.Skipped, err)
	}
	if len(got.Skipped) != 2 || !strings.Contains(strings.Join(got.Skipped, "|"), "import it instead") {
		t.Fatalf("skipped %v", got.Skipped)
	}
	schema, source, found := w.SchemaFor(got.Created[1], "1", SchemaPart{Kind: "response", Status: 200})
	if !found || source != SchemaInferred || schema["type"] != "array" {
		t.Fatalf("schema %v source %s found %v", schema, source, found)
	}
}

func TestSaveDiscovery_Given_it_ran_twice_Then_the_second_adds_nothing(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})
	_, _ = w.SaveDiscovery(flow.Collection, srv.URL, rep)

	got, err := w.SaveDiscovery(flow.Collection, srv.URL, rep)

	if err != nil || len(got.Created) != 0 || len(got.Skipped) != 1 {
		t.Fatalf("created %v skipped %v err %v", got.Created, got.Skipped, err)
	}
}

func TestSaveDiscovery_should_make_requests_that_send_and_pass(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})
	got, _ := w.SaveDiscovery(flow.Collection, srv.URL, rep)

	s, err := w.SendRequest(t.Context(), got.Created[0], SendOptions{Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	if res.Status != engine.StatusPassed {
		t.Fatalf("status %v", res.Status)
	}
}

func TestDiscoveredName_should_make_a_valid_flat_name(t *testing.T) {
	tests := map[string]string{"/": "discovered/root", "/v1/pets/1": "discovered/v1-pets-1", "/a b/ü": "discovered/a-b--"}
	for in, want := range tests {
		if got := discoveredName(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestSaveDiscovery_should_mark_the_shape_it_inferred_from_one_answer_as_shallow(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})

	got, _ := w.SaveDiscovery(flow.Collection, srv.URL, rep)

	schema, _, found := w.SchemaFor(got.Created[0], "1", SchemaPart{Kind: "response", Status: 200})
	if !found || !IsShallowSchema(schema) {
		t.Fatalf("schema %v found %v: want it marked shallow", schema, found)
	}
}

func TestLearnOne_Given_a_shallow_schema_When_a_send_agrees_with_it_Then_it_is_no_longer_shallow(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})
	got, _ := w.SaveDiscovery(flow.Collection, srv.URL, rep)
	part := SchemaPart{Kind: "response", Status: 200}
	_, r, _ := w.requestFor(got.Created[0])

	changes, err := learnOne(r.File, "1", part, rep.Findings[0].Sample)

	if err != nil || len(changes) != 0 {
		t.Fatalf("changes %v err %v", changes, err)
	}
	schema, _, _ := w.SchemaFor(got.Created[0], "1", part)
	if IsShallowSchema(schema) {
		t.Fatalf("a second sample that agrees should have ended shallow: %v", schema)
	}
}

func TestPromoteBaseline_should_freeze_inferred_schemas_so_later_sends_do_not_widen_them(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})
	got, _ := w.SaveDiscovery(flow.Collection, srv.URL, rep)
	part := SchemaPart{Kind: "response", Status: 200}

	n, err := w.PromoteBaseline(got.Created[0], "1")

	if err != nil || n != 1 {
		t.Fatalf("promoted %d, err %v", n, err)
	}
	schema, source, _ := w.SchemaFor(got.Created[0], "1", part)
	if source != SchemaUser || IsShallowSchema(schema) {
		t.Fatalf("source %s shallow %v", source, IsShallowSchema(schema))
	}
	_, r, _ := w.requestFor(got.Created[0])
	changes, _ := learnOne(r.File, "1", part, map[string]any{"surprise": true})
	if len(changes) != 0 {
		t.Fatalf("a promoted baseline was widened: %v", changes)
	}
}

func TestPromoteBaseline_Given_nothing_inferred_Then_it_promotes_nothing(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewGRPCRequest(w.Collections()[0].Manifest.ID, "plain/one")
	if err != nil {
		t.Fatal(err)
	}

	n, err := w.PromoteBaseline(target, "1")

	if err != nil || n != 0 {
		t.Fatalf("promoted %d, err %v", n, err)
	}
}

func TestDiscoverGRPC_should_list_what_the_server_offers_by_reflection(t *testing.T) {
	w := demoWorkspace(t)
	addr := startReflectingServer(t)

	methods, err := w.DiscoverGRPC(t.Context(), "grpc://"+addr)

	if err != nil || len(methods) != 1 || methods[0].Full() != "shop.Shop/Get" {
		t.Fatalf("methods %+v err %v", methods, err)
	}
}

func TestCallableMethods_should_leave_out_the_reflection_service_itself(t *testing.T) {
	in := []grpcx.Method{{Service: "grpc.reflection.v1.ServerReflection", Name: "ServerReflectionInfo"}, {Service: "shop.Shop", Name: "Get"}}

	got := callableMethods(in)

	if len(got) != 1 || got[0].Service != "shop.Shop" {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveGRPCDiscovery_should_draft_a_request_per_method_that_asks_the_server_by_reflection(t *testing.T) {
	w := demoWorkspace(t)
	addr := startReflectingServer(t)
	methods, _ := w.DiscoverGRPC(t.Context(), "grpc://"+addr)
	collection := w.Collections()[0].Manifest.ID

	got, err := w.SaveGRPCDiscovery(collection, "grpc://"+addr, methods)

	if err != nil || len(got.Created) != 1 {
		t.Fatalf("created %+v skipped %v err %v", got.Created, got.Skipped, err)
	}
	_, r, _ := w.requestFor(got.Created[0])
	g := r.Versions[0].GRPC
	if r.Path != "discovered/grpc-shop.Shop-Get" || g == nil || g.Method != "shop.Shop/Get" || g.Server != "grpc://"+addr || len(g.Proto) != 0 {
		t.Fatalf("request %s %+v", r.Path, g)
	}
}

func TestSaveGRPCDiscovery_Given_it_ran_twice_Then_the_second_adds_nothing(t *testing.T) {
	w := demoWorkspace(t)
	addr := startReflectingServer(t)
	methods, _ := w.DiscoverGRPC(t.Context(), "grpc://"+addr)
	collection := w.Collections()[0].Manifest.ID
	_, _ = w.SaveGRPCDiscovery(collection, "grpc://"+addr, methods)

	got, err := w.SaveGRPCDiscovery(collection, "grpc://"+addr, methods)

	if err != nil || len(got.Created) != 0 || len(got.Skipped) != 1 {
		t.Fatalf("created %v skipped %v err %v", got.Created, got.Skipped, err)
	}
}

func TestDiscoverGRPC_Given_a_server_without_reflection_Then_it_says_to_add_proto_files(t *testing.T) {
	w := demoWorkspace(t)
	srv := discoveryServer(t) // an HTTP server: not gRPC at all

	_, err := w.DiscoverGRPC(t.Context(), strings.TrimPrefix(srv.URL, "http://"))

	if err == nil {
		t.Fatal("want an error")
	}
}

func TestPromoteBaseline_Given_no_version_Then_every_version_is_promoted(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	srv := discoveryServer(t)
	rep, _ := w.Discover(t.Context(), srv.URL, discover.Auth{}, 1000, []string{"/pets"})
	got, _ := w.SaveDiscovery(flow.Collection, srv.URL, rep)

	n, err := w.PromoteBaseline(got.Created[0], "")

	if err != nil || n != 1 {
		t.Fatalf("promoted %d, err %v", n, err)
	}
}
