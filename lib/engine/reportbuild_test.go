// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files") //nolint:gochecknoglobals // the standard golden-file flag; flags must be package-level to register before the test runs

func TestReportBuilder_should_build_tree_with_hooks_lanes_attempts_versions_notes(t *testing.T) {
	at := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	b := newReportBuilder(func() time.Time { return at }, "Journey", "users")
	assertFail := diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "x", "message": "", "node": "a"})
	events := []Event{
		RunStarted{RunID: "R1", Kind: "flow", FlowID: "F1", FlowPath: "journey", Env: "local", Seed: 7, At: at},
		NodeStarted{RunID: "R1", Path: "login", Node: "login", Type: "request"},
		HookStarted{RunID: "R1", Path: "login", Level: "collection", Phase: "pre", Step: "hooks/sign", Index: 0},
		NodeStarted{RunID: "R1", Path: "login/pre:collection:0/sign", Parent: "login", Lane: "pre:collection:0", Node: "sign", Type: "set", Depth: 1},
		NodeFinished{RunID: "R1", Path: "login/pre:collection:0/sign", Parent: "login", Lane: "pre:collection:0", Node: "sign", Type: "set", Status: StatusPassed, Duration: time.Millisecond, Depth: 1},
		HookFinished{RunID: "R1", Path: "login", Level: "collection", Phase: "pre", Step: "hooks/sign", Status: StatusPassed, Duration: 2 * time.Millisecond},
		RequestSent{RunID: "R1", Path: "login", RequestID: "Q1", RequestPath: "auth/login", Version: "2", VersionSource: "preferred", Method: "POST", URL: "http://api/login", Attempt: 1},
		ResponseReceived{RunID: "R1", Path: "login", RequestID: "Q1", Status: 503, Duration: 10 * time.Millisecond, Attempt: 1},
		RequestSent{RunID: "R1", Path: "login", RequestID: "Q1", RequestPath: "auth/login", Version: "2", VersionSource: "preferred", Method: "POST", URL: "http://api/login", Attempt: 2},
		ResponseReceived{RunID: "R1", Path: "login", RequestID: "Q1", Status: 200, Duration: 20 * time.Millisecond, Attempt: 2},
		NodeFinished{RunID: "R1", Path: "login", Node: "login", Type: "request", Status: StatusPassed, Duration: 40 * time.Millisecond, Outputs: map[string]any{"status": 200}},
		NodeStarted{RunID: "R1", Path: "each", Node: "each", Type: "batch"},
		NodeStarted{RunID: "R1", Path: "each[0]/n", Parent: "each", Lane: "[0]", Node: "n", Type: "note"},
		NodeFinished{RunID: "R1", Path: "each[0]/n", Parent: "each", Lane: "[0]", Node: "n", Type: "note", Status: StatusPassed, Outputs: map[string]any{"text": "first"}},
		NodeStarted{RunID: "R1", Path: "each[1]/n", Parent: "each", Lane: "[1]", Node: "n", Type: "note"},
		NodeFinished{RunID: "R1", Path: "each[1]/n", Parent: "each", Lane: "[1]", Node: "n", Type: "note", Status: StatusPassed, Outputs: map[string]any{"text": "second"}},
		NodeFinished{RunID: "R1", Path: "each", Node: "each", Type: "batch", Status: StatusPassed, Duration: 5 * time.Millisecond},
		NodeStarted{RunID: "R1", Path: "a", Node: "a", Type: "assert"},
		NodeFinished{RunID: "R1", Path: "a", Node: "a", Type: "assert", Status: StatusFailed, Handled: true, Diags: diag.List{assertFail}},
		Warning{RunID: "R1", Diag: diag.New(diag.CodeRunHandled, diag.Location{}, map[string]string{"node": "a", "on_error": "continue"})},
		RunFinished{RunID: "R1", Status: StatusPassed, Duration: time.Second},
	}

	for _, ev := range events {
		b.add(ev)
	}
	var got bytes.Buffer
	if err := report.WriteJSON(&got, b.snapshot()); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join("testdata", "report.golden.json")
	if *updateGolden {
		if err := os.WriteFile(file, got.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("report differs (-update to accept):\n%s", got.String())
	}
}

func TestRun_should_save_report_json_for_passed_failed_and_cancelled_runs(t *testing.T) {
	tests := []struct {
		name   string
		nodes  []model.Node
		cancel bool
		want   Status
	}{
		{name: "passed", nodes: []model.Node{flowNode("a", "note", "text", "x")}, want: StatusPassed},
		{name: "failed", nodes: []model.Node{flowNode("a", "assert", "check", "false")}, want: StatusFailed},
		{name: "cancelled", nodes: []model.Node{flowNode("a", "delay", "duration", "1h")}, cancel: true, want: StatusCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			fl := f.addFlow("save-"+tt.name, tt.nodes...)
			clock := newFakeClock()
			d := f.deps()
			d.After = clock.After
			ctx, cancel := context.WithCancel(testCtx(t))
			defer cancel()

			run := StartRun(ctx, d, f.runInput(fl))
			if tt.cancel {
				clock.WaitForTimers(t, 1)
				cancel()
			}
			_, res := drain(run)

			want := filepath.Join(f.dataDir, string(collID), "runs", string(fl.ID), res.RunID+".json")
			if res.Status != tt.want || res.ReportFile != want || res.Report.Status != string(tt.want) {
				t.Fatalf("result %+v", res)
			}
			var saved report.Run
			data, err := os.ReadFile(want)
			if err != nil || json.Unmarshal(data, &saved) != nil || saved.Status != string(tt.want) || saved.FlowPath != fl.Path || len(saved.Nodes) != 1 {
				t.Fatalf("saved %s: %v", data, err)
			}
		})
	}
}

func TestDigest_of_batch_should_summarise_only_its_subtree(t *testing.T) {
	f := newFixture(t)
	batch := batchOf("each", []model.Field{{Key: "times", Value: int64(2)}}, flowNode("n", "note", "text", "i{{index}}"))
	fl := f.addFlow("digest", flowNode("before", "note", "text", "x"), batch,
		flowNode("d", "digest", "of", "each", "format", "text"))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	digest, ok := lastEvent[Digest](events)
	if res.Status != StatusPassed || !ok {
		t.Fatalf("result %+v", res)
	}
	if !strings.Contains(digest.Text, "each") || !strings.Contains(digest.Text, "[1]") || strings.Contains(digest.Text, "before") {
		t.Fatalf("digest:\n%s", digest.Text)
	}
}

func TestDigest_of_unfinished_or_unknown_node_Then_FLW_RUN_014(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("digest-bad", flowNode("d", "digest", "of", "later"), flowNode("later", "note", "text", "x"))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	d, _ := finishedNode(events, "d")
	if res.Status != StatusFailed || !hasCode(d.Diags, diag.CodeRunDigestTarget) {
		t.Fatalf("result %+v d %+v", res, d)
	}
}

func TestReportBuilder_should_order_iteration_lanes_by_index(t *testing.T) {
	b := newReportBuilder(time.Now, "F", "c")
	for _, ev := range []Event{
		NodeStarted{Path: "each", Node: "each", Type: "batch"},
		NodeStarted{Path: "each[10]/n", Parent: "each", Lane: "[10]", Node: "n"},
		NodeStarted{Path: "each[2]/n", Parent: "each", Lane: "[2]", Node: "n"},
		NodeStarted{Path: "each[0]/n", Parent: "each", Lane: "[0]", Node: "n"},
	} {
		b.add(ev)
	}

	lanes := b.snapshot().Nodes[0].Lanes

	if len(lanes) != 3 || lanes[0].Label != "[0]" || lanes[1].Label != "[2]" || lanes[2].Label != "[10]" {
		t.Fatalf("lanes %+v", lanes)
	}
}
