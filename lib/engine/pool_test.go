// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi/lib/examples/demoserver"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
	"github.com/githugt/flowi/lib/vault"
)

// connectionsOfRun runs the onboarding-check flow (a poll of several requests) against a demo server and returns how
// many connections the server accepted, with collectionSettings applied to the collection.
func connectionsOfRun(t *testing.T, collectionSettings model.Settings) int32 {
	t.Helper()
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(demoserver.Handler(demoserver.Options{Password: "pw"}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	f := newFixture(t)
	c := exampleCollection(t, srv.URL)
	for k, v := range collectionSettings {
		c.Manifest.Settings[k] = v
	}
	f.lookup.collections = []model.Collection{c}
	f.environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=pw"}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "v.enc"), WorkspaceID: wsID, Keys: &memKeys{}, Environ: f.environ})

	_, res := runFlow(testCtx(t), t, f.deps(), RunInput{Collection: c, Flow: flowByPath(t, c, "onboarding-check"), Env: "local"})

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
	return conns.Load()
}

func TestRunFlow_Given_default_settings_When_a_flow_sends_several_requests_Then_they_share_a_connection(t *testing.T) {
	// Before: each request step built its own client, so a flow of N requests made N connections.
	got := connectionsOfRun(t, nil)

	if got != 1 {
		t.Fatalf("connections = %d, want 1 for the whole run", got)
	}
}

func TestRunFlow_Given_reuse_turned_off_When_a_flow_sends_several_requests_Then_each_starts_cold(t *testing.T) {
	// The cold-connection view: http.reuse_connections: false times every request from a fresh connection.
	got := connectionsOfRun(t, model.Settings{"http.reuse_connections": false})

	if got < 3 {
		t.Fatalf("connections = %d, want one per request (at least 3)", got)
	}
}

func TestBaseOptions_Given_a_flow_run_and_a_send_When_options_are_built_Then_only_the_run_has_a_pool(t *testing.T) {
	// A send has one request, so it starts cold; a flow run shares its transports unless the setting says otherwise.
	cfg := newConfig(settings.Resolve(settings.Schema()))
	run := &runner{pool: &httpx.Pool{}}
	send := &runner{}
	cold := cfg
	cold.reuse = false

	if run.baseOptions(cfg, nil, "").Pool == nil {
		t.Error("a flow run has no pool")
	}
	if send.baseOptions(cfg, nil, "").Pool != nil {
		t.Error("a send has a pool")
	}
	if run.baseOptions(cold, nil, "").Pool != nil {
		t.Error("reuse off still has a pool")
	}
}

func TestRunFlow_Given_run_keep_one_When_a_flow_runs_twice_Then_one_report_is_left(t *testing.T) {
	// Before: every run's report stayed for ever.
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "pw"}))
	t.Cleanup(srv.Close)
	f := newFixture(t)
	c := exampleCollection(t, srv.URL)
	c.Manifest.Settings["run.keep"] = int64(1)
	f.lookup.collections = []model.Collection{c}
	f.environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=pw"}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "v.enc"), WorkspaceID: wsID, Keys: &memKeys{}, Environ: f.environ})
	in := RunInput{Collection: c, Flow: flowByPath(t, c, "onboarding-check"), Env: "local"}

	_, first := runFlow(testCtx(t), t, f.deps(), in)
	_, second := runFlow(testCtx(t), t, f.deps(), in)

	files, err := filepath.Glob(filepath.Join(filepath.Dir(second.ReportFile), "*.json"))
	if err != nil || len(files) != 1 || files[0] != second.ReportFile {
		t.Fatalf("reports left %v (first %s, second %s), err %v", files, first.ReportFile, second.ReportFile, err)
	}
}
