// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrace_should_say_which_step_makes_a_variable_and_which_read_it(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "trace", "user-creation", "token")

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"vars.token", "made by", "session", "read by", "verify"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestTrace_should_print_json_when_asked(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "trace", "user-creation", "token", "--output", "json")

	var got struct {
		Name      string   `json:"name"`
		Producers []string `json:"producers"`
	}
	if err := json.Unmarshal([]byte(out), &got); code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s\n%s", code, err, out, errOut)
	}
	if got.Name != "vars.token" || len(got.Producers) != 1 || got.Producers[0] != "session" {
		t.Fatalf("got %+v", got)
	}
}

func TestTrace_Given_a_name_nothing_makes_Then_exits_1_so_a_typo_fails_a_script(t *testing.T) {
	e := newSendEnv(t)

	code, out, _ := e.run(context.Background(), "trace", "user-creation", "tokn")

	if code != 1 || !strings.Contains(out, "nothing makes") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestTrace_Given_a_flow_input_Then_exits_0_although_no_step_makes_it(t *testing.T) {
	e := newSendEnv(t)

	code, out, _ := e.run(context.Background(), "trace", "user-creation", "inputs.email")

	if code != 0 || !strings.Contains(out, "input") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestTrace_Given_a_run_bundle_Then_shows_the_value_each_step_had(t *testing.T) {
	e := newSendEnv(t)
	bundle := filepath.Join(t.TempDir(), "run.flowi-run")
	if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--bundle", bundle); code != 0 {
		t.Fatalf("run: %d\n%s", code, errOut)
	}

	code, out, errOut := e.run(context.Background(), "trace", "user-creation", "subject", "--run", bundle)

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"in the run", "made  session", "read  verify"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "(no value in the run)") {
		t.Errorf("every step ran, so every value is known:\n%s", out)
	}
}

func TestRun_Given_timeline_Then_draws_each_step_on_a_time_axis(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--timeline")

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"login", "page-2", "█", "total "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDiff_should_name_what_two_runs_did_differently(t *testing.T) {
	e := newSendEnv(t)
	dir := t.TempDir()
	first, second := filepath.Join(dir, "a.flowi-run"), filepath.Join(dir, "b.flowi-run")
	for _, bundle := range []string{first, second} {
		if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--bundle", bundle); code != 0 {
			t.Fatalf("run: %d\n%s", code, errOut)
		}
	}

	code, out, errOut := e.run(context.Background(), "diff", first, second)
	same, _, _ := e.run(context.Background(), "diff", first, first)

	if code != 1 || !strings.Contains(out, "first differs at") || !strings.Contains(out, "wrote token") {
		t.Fatalf("exit %d (the demo server hands out a new token each run)\n%s\n%s", code, out, errOut)
	}
	if same != 0 {
		t.Errorf("a run compared with itself exits %d", same)
	}
}

func TestDiff_Given_timing_Then_it_adds_which_steps_took_much_longer_or_shorter(t *testing.T) {
	e := newSendEnv(t)
	file := filepath.Join(t.TempDir(), "a.flowi-run")
	if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--bundle", file); code != 0 {
		t.Fatalf("run: %d\n%s", code, errOut)
	}

	_, withTiming, _ := e.run(context.Background(), "diff", file, file, "--timing")
	_, without, _ := e.run(context.Background(), "diff", file, file)

	if !strings.Contains(withTiming, "Timing, against the first run") || !strings.Contains(withTiming, "no step took much longer") {
		t.Fatalf("with --timing:\n%s", withTiming)
	}
	if strings.Contains(without, "Timing") {
		t.Fatalf("without --timing:\n%s", without)
	}
}
