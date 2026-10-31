// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/schemainfer"
)

// driftWorkspace has a flow that calls GET /pet once. What the server answers is the test's to change.
func driftWorkspace(t *testing.T, answer *atomic.Value) (*Workspace, FlowTarget, RequestTarget) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answer.Load().(string)))
	}))
	t.Cleanup(srv.Close)
	w := newEnv(t).open(t)
	m, err := w.InitCollection(t.TempDir(), "drift", true)
	if err != nil {
		t.Fatal(err)
	}
	reqTarget, err := w.NewRequest(m.ID, "pet")
	if err != nil {
		t.Fatal(err)
	}
	r := requestOf(t, w, reqTarget)
	r.Versions = []model.RequestVersion{{Label: "1", Method: "GET", URL: "{{env.base_url}}/pet"}}
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
	target, err := w.NewFlow(m.ID, "watch")
	if err != nil {
		t.Fatal(err)
	}
	_, flow, _ := w.flowFor(target)
	flow.Nodes = []model.Node{{ID: "pet", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: r.ID, Path: "pet"}}}}
	if err := w.SaveFlow(target, flow, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, target, reqTarget
}

func TestCheckDrift_Given_a_response_that_changed_since_the_schema_was_learned_Then_new_missing_and_changed_are_reported(t *testing.T) {
	// Arrange: learn from the first response, then let the API change.
	var answer atomic.Value
	answer.Store(`{"id": 7, "name": "rex"}`)
	w, target, reqTarget := driftWorkspace(t, &answer)
	if res := runToEnd(t, w, target, RunOptions{Env: "local"}); res.Report.Status != "passed" {
		t.Fatalf("the learning run: %s %v", res.Report.Status, res.Diags)
	}
	answer.Store(`{"id": "seven", "region": "eu"}`)
	file, _ := schemaFile(requestOf(t, w, reqTarget).File, "1", SchemaPart{Kind: "response", Status: 200})
	before, _ := os.ReadFile(file)

	// Act
	rep, err := w.CheckDrift(context.Background(), target, DriftOptions{Env: "local"})

	// Assert
	if err != nil || len(rep.Requests) != 1 {
		t.Fatalf("err %v report %+v", err, rep)
	}
	got := map[string]string{}
	for _, d := range rep.Requests[0].Differences {
		got[string(d.Kind)+" "+d.Path] = d.Detail
	}
	for _, want := range []string{"changed $.id", "missing $.name", "new $.region"} {
		if _, ok := got[want]; !ok {
			t.Errorf("no %q in %v", want, got)
		}
	}
	if !rep.Has(schemainfer.DriftMissing, schemainfer.DriftChanged) || rep.Requests[0].Status != 200 {
		t.Errorf("report %+v", rep)
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Errorf("a drift check must not widen the schema:\n%s\n%s", before, after)
	}
}

func TestCheckDrift_Given_a_response_like_the_learned_one_Then_the_request_is_clean(t *testing.T) {
	// Arrange
	var answer atomic.Value
	answer.Store(`{"id": 7, "name": "rex"}`)
	w, target, _ := driftWorkspace(t, &answer)
	runToEnd(t, w, target, RunOptions{Env: "local"})
	answer.Store(`{"id": 8, "name": "fido"}`)

	// Act
	rep, err := w.CheckDrift(context.Background(), target, DriftOptions{Env: "local"})

	// Assert
	if err != nil || rep.Has(schemainfer.DriftNew, schemainfer.DriftMissing, schemainfer.DriftChanged) || len(rep.Requests[0].Differences) != 0 {
		t.Fatalf("err %v report %+v", err, rep)
	}
}

func TestCheckDrift_Given_no_schema_was_learned_yet_Then_it_says_so_and_does_not_learn_one(t *testing.T) {
	// Arrange
	var answer atomic.Value
	answer.Store(`{"id": 7}`)
	w, target, reqTarget := driftWorkspace(t, &answer)

	// Act
	rep, err := w.CheckDrift(context.Background(), target, DriftOptions{Env: "local"})

	// Assert
	if err != nil || len(rep.Requests) != 1 || !rep.Requests[0].NoSchema {
		t.Fatalf("err %v report %+v", err, rep)
	}
	file, _ := schemaFile(requestOf(t, w, reqTarget).File, "1", SchemaPart{Kind: "response", Status: 200})
	if _, statErr := os.Stat(file); statErr == nil {
		t.Errorf("a drift check must not learn a schema: %s exists", file)
	}
	if strings.Contains(rep.Requests[0].Request, "..") {
		t.Errorf("request %q", rep.Requests[0].Request)
	}
}
