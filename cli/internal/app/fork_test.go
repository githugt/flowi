// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func recordUserCreation(t *testing.T, e *sendEnv) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "run.flowi-run")
	if code, _, errOut := e.run(context.Background(), "run", "user-creation", "--env", "local", "--seed", "7", "--bundle", file); code != 0 {
		t.Fatalf("recording: exit %d\n%s", code, errOut)
	}
	return file
}

func TestRunFork_should_run_again_from_the_step_and_not_send_anything(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	other := newSendEnv(t) // another workspace, its server never used
	other.environ = other.environ[:2]

	code, out, errOut := other.run(context.Background(), "run", file, "--from", "page-1", "--set", "first_name=Zed")

	if code != 0 || !strings.Contains(errOut, "No request is sent") || !strings.Contains(out, "Against the recording") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if strings.Contains(out, "login") {
		t.Fatalf("a step before the fork ran again:\n%s", out)
	}
}

func TestRunFork_Given_an_unknown_step_Then_a_usage_error_lists_the_steps(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)

	code, _, errOut := e.run(context.Background(), "run", file, "--from", "nope")

	if code != 2 || !strings.Contains(errOut, "page-1") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestRunFork_Given_set_without_from_or_from_without_a_bundle_Then_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)

	noFrom, _, _ := e.run(context.Background(), "run", file, "--set", "a=1")
	noBundle, _, _ := e.run(context.Background(), "run", "user-creation", "--from", "verify")

	if noFrom != 2 || noBundle != 2 {
		t.Fatalf("exits %d and %d", noFrom, noBundle)
	}
}

func TestRunFork_Given_only_Then_the_steps_after_the_fork_step_do_not_run(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)

	code, out, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--only")

	if code != 0 || !strings.Contains(out, "page-1") || strings.Contains(out, "page-2") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestRunFork_Given_respond_with_a_status_Then_the_step_is_answered_with_it(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)

	code, out, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--only", "--respond", "page-1=503")

	if !strings.Contains(out+errOut, "503") {
		t.Fatalf("the 503 never shows (exit %d)\n%s\n%s", code, out, errOut)
	}
}

func TestRunFork_Given_respond_with_a_body_file_Then_the_body_is_read_from_it(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	body := filepath.Join(t.TempDir(), "page.json")
	writeFile(t, body, `{"saved": {"firstName": "Yan"}}`)

	code, out, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--only", "--respond", "page-1=418@"+body, "--output", "json")

	if code == 2 || !strings.Contains(out, "418") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestRunFork_Given_a_bad_respond_Then_a_usage_error_says_what_to_write(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	cases := map[string][]string{
		"no equals":       {"--respond", "page-1"},
		"status too low":  {"--respond", "page-1=99"},
		"status too high": {"--respond", "page-1=600"},
		"missing file":    {"--respond", "page-1=@/no/such/file.json"},
		"nothing given":   {"--respond", "page-1="},
		"not a step":      {"--respond", "profile=500"},
	}

	for name, args := range cases {
		code, _, errOut := e.run(context.Background(), append([]string{"run", file, "--from", "page-1"}, args...)...)

		if code != 2 || !strings.Contains(errOut, "respond") && !strings.Contains(errOut, "profile") {
			t.Errorf("%s: exit %d\n%s", name, code, errOut)
		}
	}
}

func TestRunFork_Given_only_or_respond_without_from_Then_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)

	only, _, _ := e.run(context.Background(), "run", file, "--only")
	respond, _, _ := e.run(context.Background(), "run", file, "--respond", "page-1=500")

	if only != 2 || respond != 2 {
		t.Fatalf("exits %d and %d", only, respond)
	}
}

func TestRunFork_Given_respond_with_a_non_json_file_Then_the_step_is_answered_with_its_text(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	page := filepath.Join(t.TempDir(), "down.xml")
	writeFile(t, page, `<error>down</error>`)

	code, out, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--only", "--respond", "page-1=503@"+page)

	if code == 2 || !strings.Contains(out+errOut, "call body") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestRunFork_Given_a_json_file_that_is_not_json_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	page := filepath.Join(t.TempDir(), "page.json")
	writeFile(t, page, `<error>down</error>`)

	code, _, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--respond", "page-1=@"+page)

	// A file that says it is JSON and is not has a typo in it; answering with its text would hide that.
	if code != 2 || !strings.Contains(errOut, "not valid JSON") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestRunFork_Given_respond_with_a_binary_file_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)
	file := recordUserCreation(t, e)
	page := filepath.Join(t.TempDir(), "image.png")
	writeFile(t, page, "\xff\xfe\x00\x01")

	code, _, errOut := e.run(context.Background(), "run", file, "--from", "page-1", "--respond", "page-1=@"+page)

	if code != 2 || !strings.Contains(errOut, "text") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}
