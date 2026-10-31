// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/report"
)

var update = flag.Bool("update", false, "rewrite golden files") //nolint:gochecknoglobals // the standard golden-file flag; flags must be package-level to register before the test runs

// fixtureRun has one of everything the writers render.
func fixtureRun() report.Run {
	start := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	ms := time.Millisecond
	return report.Run{
		ID: "01JRUN0000000000000000000A", Kind: "flow", FlowID: "01JA0000000000000000000020", FlowPath: "user-creation",
		FlowName: "User creation journey", Collection: "user-creation", Env: "local", Status: "aborted",
		Seed: 42, Started: start, Duration: 61500 * ms,
		Nodes: []report.Node{
			{
				Path: "login", ID: "login", Type: "request", Status: "passed", Started: start, Duration: 120 * ms,
				Outputs: map[string]any{"status": 200, "token": "abc"},
				Attempts: []report.Attempt{
					{N: 1, Method: "POST", URL: "http://api.test/v2/login", Status: 503, Duration: 20 * ms},
					{N: 2, Method: "POST", URL: "http://api.test/v2/login", Status: 200, Duration: 30 * ms},
				},
				Hooks: []report.Hook{{
					Phase: "pre", Level: "collection", Step: "hooks/sign", Index: 0, Status: "passed", Duration: 5 * ms,
					Nodes: []report.Node{{Path: "login/pre:collection:0/sign", ID: "sign", Type: "set", Status: "passed", Duration: ms}},
				}},
			},
			{
				Path: "flaky", ID: "flaky", Type: "assert", Status: "failed", Handled: true, Duration: ms,
				Diagnostics: []report.Diagnostic{{Code: "FLW-RUN-001", Severity: "error", Message: "assertion failed: vars.ready"}},
			},
			{
				Path: "users", ID: "users", Type: "batch", Status: "passed", Duration: 300 * ms,
				Lanes: []report.Lane{
					{Label: "[0]", Nodes: []report.Node{{Path: "users[0]/create", ID: "create", Type: "fetch", Status: "passed", Duration: 100 * ms}}},
					{Label: "[1]", Nodes: []report.Node{{Path: "users[1]/create", ID: "create", Type: "fetch", Status: "passed", Duration: 90 * ms}}},
				},
			},
			{
				Path: "both", ID: "both", Type: "parallel", Status: "aborted", Duration: 60 * time.Second,
				Lanes: []report.Lane{
					{Label: "b1", Nodes: []report.Node{{Path: "both/b1/verify", ID: "verify", Type: "request", Status: "aborted", Duration: 60 * time.Second}}},
					{Label: "b2", Nodes: []report.Node{{Path: "both/b2/note", ID: "note", Type: "note", Status: "passed", Outputs: map[string]any{"text": "hello"}}}},
				},
			},
		},
		Warnings: []report.Diagnostic{{Code: "FLW-RUN-015", Severity: "warning", Message: `node "flaky" failed; continuing (on_error: continue)`}},
		Notes:    []report.Note{{Path: "both/b2/note", Text: "hello"}},
		Versions: []report.VersionUse{{Path: "login", RequestID: "01JA0000000000000000000010", RequestPath: "auth/login", Version: "2", Source: "flow preference"}},
		Aborted:  &report.Abort{Reason: "grace_period_exceeded", Grace: 60 * time.Second, Inflight: []string{"both/b1/verify"}},
	}
}

// golden compares got with testdata/name, rewriting it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	file := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s differs (run with -update to accept):\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}
