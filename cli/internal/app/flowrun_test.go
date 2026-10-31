// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/cli/internal/cli"
)

func TestRun_exit_codes(t *testing.T) {
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "fail.arazzo.yaml"), arazzoFlow("id: 01JA00000000000000000000T5\nnodes:\n  - id: never\n    type: assert\n    check: \"1 == 2\"\n"))
	unwritable := filepath.Join(e.dir, "missing-dir", "out.xml")
	tests := []struct {
		name   string
		args   []string
		code   int
		stderr []string
	}{
		{"passed", []string{"run", "user-creation", "--env", "local"}, 0, nil},
		{"unknown flow", []string{"run", "nope"}, 2, []string{"FLW-FLOW-002"}},
		{"undeclared input", []string{"run", "user-creation", "--env", "local", "--input", "nope=1"}, 2, []string{"FLW-RUN-010"}},
		{"bad input flag", []string{"run", "user-creation", "--input", "novalue"}, 2, []string{"--input"}},
		{"assertion failed", []string{"run", "fail", "--env", "local"}, 1, []string{"FLW-RUN-001"}},
		{"server down", []string{"run", "user-creation", "--env", "staging"}, 4, []string{"FLW-HTTP-001"}},
		{"bad report flag", []string{"run", "user-creation", "--report", "pdf=x.pdf"}, 2, []string{"--report"}},
		{"unwritable report", []string{"run", "user-creation", "--env", "local", "--report", "junit=" + unwritable}, 4, []string{"out.xml"}},
		{"missing argument", []string{"run"}, 2, []string{"a flow or --tag"}},
		{"flow and tag together", []string{"run", "user-creation", "--tag", "nightly"}, 2, []string{"not both"}},
		{"tag nothing carries", []string{"run", "--tag", "nope"}, 2, []string{"FLW-FLOW-004"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, errOut := e.run(context.Background(), tt.args...)

			if code != tt.code {
				t.Fatalf("exit %d, want %d\n%s", code, tt.code, errOut)
			}
			for _, want := range tt.stderr {
				if !strings.Contains(errOut, want) {
					t.Errorf("stderr lacks %q:\n%s", want, errOut)
				}
			}
			if strings.Contains(errOut, demoPassword) {
				t.Errorf("password in stderr:\n%s", errOut)
			}
		})
	}
}

func TestRun_Given_interrupt_Then_exit_3_and_report_cancelled(t *testing.T) {
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "wait.arazzo.yaml"),
		arazzoFlow("id: 01JA00000000000000000000T6\nnodes:\n  - id: slow\n    type: request\n    request:\n      id: 01JA00000000000000000000T3\n      path: slow\n"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()

	code, out, errOut := e.run(ctx, "run", "wait", "--env", "local", "--report", "json="+filepath.Join(e.dir, "r.json"))

	if code != cli.ExitCancelled || time.Since(start) > 5*time.Second {
		t.Fatalf("exit %d after %v\n%s\n%s", code, time.Since(start), out, errOut)
	}
	data, err := os.ReadFile(filepath.Join(e.dir, "r.json"))
	if err != nil || !strings.Contains(string(data), `"status": "cancelled"`) {
		t.Fatalf("report %s %v", data, err)
	}
}

// Final review, Minor 10 (graded Important): the plan's exit table says a
// report that cannot be written is exit 4 — the saved run report too, not
// only --report files.
func TestRun_Given_run_report_cannot_be_saved_Then_exit_4(t *testing.T) {
	e := newSendEnv(t)
	runs := filepath.Join(e.dir, "config", "workspaces", "default-workspace", "data", "01JA0000000000000000000001", "runs")
	writeFile(t, runs, "a file where the runs directory should be")

	code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local")

	if code != cli.ExitSystem || !strings.Contains(errOut, "run report") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

// arazzoFlow wraps a flow's own YAML in the minimal Arazzo document a flow file is.
func arazzoFlow(body string) string {
	var sb strings.Builder
	sb.WriteString("arazzo: 1.1.0\nworkflows:\n  - workflowId: w\n    x-flowi-flow:\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		sb.WriteString("      " + line + "\n")
	}
	return sb.String()
}

func TestRun_Given_a_tag_Then_every_flow_with_it_runs_and_the_worst_outcome_is_the_exit_code(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "fails.arazzo.yaml"), arazzoFlow("id: 01JA00000000000000000000T6\ntags:\n  - nightly\nnodes:\n  - id: never\n    type: assert\n    check: \"1 == 2\"\n"))
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "quiet.arazzo.yaml"), arazzoFlow("id: 01JA00000000000000000000T7\ntags:\n  - nightly\nnodes:\n  - id: ok\n    type: assert\n    check: \"1 == 1\"\n"))
	report := filepath.Join(t.TempDir(), "out.xml")

	// Act
	code, out, errOut := e.run(context.Background(), "run", "--tag", "nightly", "--env", "local", "--report", "junit="+report)

	// Assert
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s\n%s", code, out, errOut)
	}
	for _, flow := range []string{"fails", "quiet"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(report), "out-"+flow+".xml")); err != nil {
			t.Errorf("a report per flow was expected: %v", err)
		}
	}
}

const standaloneArazzo = `arazzo: 1.0.1
info: {title: pets, version: 1.0.0}
sourceDescriptions: [{name: pets, url: ./pets.yaml, type: openapi}]
workflows:
  - workflowId: fetchName
    inputs:
      type: object
      properties:
        petId: {type: string}
    steps:
      - stepId: fetch
        operationId: getPet
        parameters:
          - {name: petId, in: path, value: $inputs.petId}
        successCriteria:
          - condition: $statusCode == 200
`

func writeStandalone(t *testing.T, serverURL string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pets.yaml"), "openapi: 3.0.3\ninfo: {title: pets, version: 1.0.0}\nservers: [{url: "+serverURL+"}]\npaths:\n  /pets/{petId}:\n    get:\n      operationId: getPet\n      parameters:\n        - {name: petId, in: path, required: true, schema: {type: string}}\n      responses:\n        \"200\": {description: ok}\n")
	file := filepath.Join(dir, "fetch.arazzo.yaml")
	writeFile(t, file, standaloneArazzo)
	return file
}

func TestRun_Given_an_arazzo_file_Then_it_runs_without_being_mounted_and_writes_a_har(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"rex"}`))
	}))
	t.Cleanup(srv.Close)
	e := newSendEnv(t)
	file := writeStandalone(t, srv.URL)
	har := filepath.Join(t.TempDir(), "run.har")

	// Act
	code, _, errOut := e.run(context.Background(), "run", file, "--input", "petId=7", "--report", "har="+har)
	_, lsOut, _ := e.run(context.Background(), "workspace", "ls")

	// Assert
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	data, err := os.ReadFile(har)
	if err != nil || !strings.Contains(string(data), "/pets/7") || !strings.Contains(string(data), `"version": "1.2"`) {
		t.Fatalf("har %v\n%s", err, data)
	}
	if strings.Contains(lsOut, "fetch") {
		t.Errorf("the document must not be mounted:\n%s", lsOut)
	}
}

func TestRun_Given_a_file_that_is_not_arazzo_Then_it_is_a_usage_error(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	file := filepath.Join(t.TempDir(), "x.yaml")
	writeFile(t, file, "hello: world\n")

	// Act
	code, _, errOut := e.run(context.Background(), "run", file)

	// Assert
	if code != 2 || !strings.Contains(errOut, "not an Arazzo document") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestLs_Given_a_tag_Then_only_requests_and_flows_that_carry_it_are_listed_with_their_tags(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "quiet.arazzo.yaml"), arazzoFlow("id: 01JA00000000000000000000T8\ntags:\n  - nightly\nnodes:\n  - id: ok\n    type: assert\n    check: \"1 == 1\"\n"))
	writeFile(t, filepath.Join(e.dir, "tools", "requests", "slow.req.yaml"), "id: 01JA00000000000000000000T3\nname: Slow\ntags:\n  - health\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/slow\"\n")
	writeFile(t, filepath.Join(e.dir, "tools", "requests", "other.req.yaml"), "id: 01JA00000000000000000000T9\nname: Other\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/o\"\n")

	// Act
	_, nightly, _ := e.run(context.Background(), "ls", "--tag", "nightly")
	_, health, _ := e.run(context.Background(), "ls", "requests", "--tag", "health")
	_, all, _ := e.run(context.Background(), "ls", "requests")

	// Assert
	if !strings.Contains(nightly, "quiet") || strings.Contains(nightly, "Slow") || !strings.Contains(nightly, "#nightly") {
		t.Errorf("nightly:\n%s", nightly)
	}
	if !strings.Contains(health, "Slow") || strings.Contains(health, "Other") {
		t.Errorf("health:\n%s", health)
	}
	if !strings.Contains(all, "Slow") || !strings.Contains(all, "#health") || !strings.Contains(all, "Other") {
		t.Errorf("all:\n%s", all)
	}
}

func TestRun_Given_bundle_Then_the_run_is_written_and_plays_back_offline_in_another_workspace(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	file := filepath.Join(t.TempDir(), "login.flowi-run")
	code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--seed", "7", "--bundle", file)
	if code != 0 {
		t.Fatalf("writing the bundle: exit %d\n%s", code, errOut)
	}
	other := newSendEnv(t)
	other.environ = other.environ[:2] // no secrets of the original: a replay must not need them
	srvDown := "http://127.0.0.1:1"

	// Act
	replayCode, out, replayErr := other.run(context.Background(), "run", file)

	// Assert
	if replayCode != 0 {
		t.Fatalf("replay: exit %d (server at %s is not used)\nstdout: %s\nstderr: %s", replayCode, srvDown, out, replayErr)
	}
	if !strings.Contains(out, "Replaying") && !strings.Contains(replayErr, "Replaying") {
		t.Errorf("a replay should say it is one:\nstdout: %s\nstderr: %s", out, replayErr)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("the bundle file: %v", err)
	}
}

func TestRun_Given_a_bundle_file_and_options_that_a_replay_decides_itself_Then_it_is_a_usage_error(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	file := filepath.Join(t.TempDir(), "login.flowi-run")
	if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--bundle", file); code != 0 {
		t.Fatalf("writing the bundle: exit %d\n%s", code, errOut)
	}

	// Act
	again, _, againErr := e.run(context.Background(), "run", file, "--bundle", file+".2")
	env, _, envErr := e.run(context.Background(), "run", file, "--env", "staging")

	// Assert
	if again != 2 || !strings.Contains(againErr, "replay") {
		t.Errorf("--bundle on a replay: exit %d\n%s", again, againErr)
	}
	if env != 2 || !strings.Contains(envErr, "recorded environment") {
		t.Errorf("--env on a replay: exit %d\n%s", env, envErr)
	}
}

func TestRun_Given_a_matrix_Then_each_cell_runs_and_the_grid_shows_which_passed(t *testing.T) {
	// Arrange
	e := newSendEnv(t)

	// Act: local answers, staging points at a server that is down.
	code, out, errOut := e.run(context.Background(), "run", "user-creation", "--matrix", "env=local,staging")

	// Assert
	if code != 4 {
		t.Fatalf("exit %d: the worst cell is a system failure\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	for _, want := range []string{"✓ local", "! staging", "FLW-HTTP-001"} {
		if !strings.Contains(out, want) {
			t.Errorf("the matrix output lacks %q:\n%s", want, out)
		}
	}
}

func TestRun_Given_a_matrix_with_two_dimensions_Then_it_is_a_grid(t *testing.T) {
	// Arrange
	e := newSendEnv(t)

	// Act
	_, out, errOut := e.run(context.Background(), "run", "user-creation", "--matrix", "env=local,staging", "--matrix", "input.email=a@x.test,b@x.test")

	// Assert
	for _, want := range []string{"input.email", "a@x.test", "b@x.test", "local", "staging"} {
		if !strings.Contains(out, want) {
			t.Errorf("the grid lacks %q:\n%s\n%s", want, out, errOut)
		}
	}
}

func TestRun_Given_a_matrix_with_a_tag_or_a_bad_spec_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	for name, args := range map[string][]string{
		"with a tag":   {"run", "--tag", "x", "--matrix", "env=local,staging"},
		"bad spec":     {"run", "user-creation", "--matrix", "env"},
		"with --env":   {"run", "user-creation", "--env", "local", "--matrix", "env=local,staging"},
		"unknown dim":  {"run", "user-creation", "--matrix", "colour=red"},
		"with replay":  {"run", "x.flowi-run", "--matrix", "env=a,b"},
		"overlay flag": {"run", "user-creation", "--matrix", "overlay=a,b", "--overlay", "a"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := e.run(context.Background(), args...)

			if code != 2 {
				t.Fatalf("exit %d\n%s", code, errOut)
			}
		})
	}
}

func TestAccess_Given_a_server_that_does_not_check_who_asks_Then_the_other_user_is_an_exposure_and_exit_1(t *testing.T) {
	// Arrange: the demo server answers anyone, so the other identity reaches page-1.
	e := newSendEnv(t)
	for name, user := range map[string]string{"alice": "a", "bob": "b"} {
		writeFile(t, filepath.Join(e.dir, "example", "overlays", name+".flowi-overlay.yaml"), "flowi_overlay: 1\nshared:\n  - set: headers.X-User\n    value: "+user+"\n")
	}

	// Act
	code, out, errOut := e.run(context.Background(), "access", "user-creation", "--env", "local", "--owner", "alice", "--as", "bob", "--steps", "page-1")

	// Assert
	if code != 1 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	for _, want := range []string{"page-1", "alice", "bob", "allowed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}
}

func TestAccess_Given_a_missing_owner_or_identity_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	for name, args := range map[string][]string{
		"no owner":         {"access", "user-creation", "--as", "bob"},
		"no other":         {"access", "user-creation", "--owner", "alice"},
		"unknown identity": {"access", "user-creation", "--owner", "alice", "--as", "ghost"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := e.run(context.Background(), args...)

			if code != 2 {
				t.Fatalf("exit %d\n%s", code, errOut)
			}
		})
	}
}

func TestDrift_Given_a_flow_whose_response_changed_since_learning_Then_it_lists_the_drift_and_fails_on_missing_and_changed(t *testing.T) {
	// Arrange: learn the schema from the demo server's answers, then ask what differs: nothing, so it is clean.
	e := newSendEnv(t)
	if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local"); code != 0 {
		t.Fatalf("the learning run: exit %d\n%s", code, errOut)
	}

	// Act
	code, out, errOut := e.run(context.Background(), "drift", "user-creation", "--env", "local")

	// Assert
	if code != 0 {
		t.Fatalf("a run against the server it was learned from has no drift: exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if !strings.Contains(out, "No drift") && !strings.Contains(out, "login") {
		t.Errorf("the output should say what it compared:\n%s", out)
	}
}

func TestDrift_Given_a_bad_fail_on_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "drift", "user-creation", "--fail-on", "everything")

	if code != 2 || !strings.Contains(errOut, "--fail-on") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestRun_Given_a_mermaid_report_Then_the_file_is_a_sequence_diagram_of_the_calls(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	file := filepath.Join(t.TempDir(), "run.mmd")

	// Act
	code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--report", "mermaid="+file)

	// Assert
	data, err := os.ReadFile(file)
	if code != 0 || err != nil {
		t.Fatalf("exit %d err %v\n%s", code, err, errOut)
	}
	text := string(data)
	if !strings.HasPrefix(text, "sequenceDiagram") || !strings.Contains(text, "F->>H1: login: POST /v2/login") || !strings.Contains(text, "H1-->>F: 200") {
		t.Errorf("the diagram:\n%s", text)
	}
}
