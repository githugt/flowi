// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
)

// fuzzServer validates names like a careless service: it answers 400 for anything it checks, and crashes (500)
// on a name longer than 100 characters.
func fuzzServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if json.Unmarshal(raw, &body) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name, ok := body["name"].(string)
		switch {
		case !ok || name == "":
			w.WriteHeader(http.StatusBadRequest)
		case len(name) > 100:
			w.WriteHeader(http.StatusInternalServerError) // the bug the fuzz step must find
		default:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fuzzWorkspace(t *testing.T, srv *httptest.Server, extra ...model.Field) (*Workspace, FlowTarget) {
	t.Helper()
	w := newEnv(t).open(t)
	m, err := w.InitCollection(t.TempDir(), "fuzz", true)
	if err != nil {
		t.Fatal(err)
	}
	reqTarget, err := w.NewRequest(m.ID, "create")
	if err != nil {
		t.Fatal(err)
	}
	r := requestOf(t, w, reqTarget)
	r.Versions = []model.RequestVersion{{Label: "1", Method: "POST", URL: "{{env.base_url}}/pets", Body: model.Body{JSON: `{"name": "rex", "age": 3}`}}}
	if err := w.SaveRequest(reqTarget, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	envFile, err := w.NewEnvironment("local")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range w.Environments() {
		if e.File == envFile {
			e.Vars = []model.EnvVar{{Name: "base_url", Value: srv.URL}}
			if err := w.SaveEnvironment(e, SaveOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	target, err := w.NewFlow(m.ID, "fuzz")
	if err != nil {
		t.Fatal(err)
	}
	_, flow, _ := w.flowFor(target)
	flow.Nodes = []model.Node{{ID: "fuzz-create", Type: "fuzz", Params: extra, Request: &model.RequestCall{Request: model.Ref{ID: r.ID, Path: "create"}}}}
	if err := w.SaveFlow(target, flow, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, target
}

func TestFuzzStep_Given_a_server_that_crashes_on_a_long_name_Then_the_flow_fails_and_names_the_case(t *testing.T) {
	// Arrange
	w, target := fuzzWorkspace(t, fuzzServer(t), model.Field{Key: "count", Value: int64(60)})

	// Act
	res := runToEnd(t, w, target, RunOptions{Env: "local"})

	// Assert
	if res.Status != engine.StatusFailed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	node := res.Report.Nodes[0]
	failures, _ := node.Outputs["failures"].([]any)
	if len(failures) != 1 || !strings.Contains(failures[0].(map[string]any)["case"].(string), "name: very long text") {
		t.Fatalf("failures %v", node.Outputs["failures"])
	}
	if len(res.Captured) < 10 {
		t.Errorf("every case is a real call that is recorded: %d", len(res.Captured))
	}
}

func TestFuzzStep_Given_a_server_that_checks_everything_Then_the_flow_passes(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	t.Cleanup(srv.Close)
	w, target := fuzzWorkspace(t, srv)

	// Act
	res := runToEnd(t, w, target, RunOptions{Env: "local"})

	// Assert
	cases, _ := strconv.Atoi(fmt.Sprint(res.Report.Nodes[0].Outputs["cases"]))
	if res.Status != engine.StatusPassed || cases < 10 || cases > 20 {
		t.Fatalf("status %s outputs %v diags %v", res.Status, res.Report.Nodes[0].Outputs, res.Diags)
	}
}

func TestFuzzStep_Given_a_request_without_a_json_body_Then_it_says_so(t *testing.T) {
	// Arrange
	srv := fuzzServer(t)
	w, target := fuzzWorkspace(t, srv)
	_, flow, _ := w.flowFor(target)
	reqTarget, _ := w.FindRequest("create", "")
	r := requestOf(t, w, reqTarget)
	r.Versions[0].Body = model.Body{}
	if err := w.SaveRequest(reqTarget, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	_ = flow

	// Act
	res := runToEnd(t, w, target, RunOptions{Env: "local"})

	// Assert
	if res.Status == engine.StatusPassed || !strings.Contains(fuzzText(res), "no JSON body") {
		t.Fatalf("status %s text %q", res.Status, fuzzText(res))
	}
}

func fuzzText(r engine.RunResult) string {
	var sb strings.Builder
	for _, n := range r.Report.Nodes {
		for _, d := range n.Diagnostics {
			sb.WriteString(d.Message + "\n")
		}
	}
	return sb.String()
}
