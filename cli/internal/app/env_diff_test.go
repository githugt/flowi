// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"testing"
)

func TestEnvDiff_should_show_what_differs_and_exit_1(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "env", "diff", "local", "staging")

	if code != 1 {
		t.Fatalf("exit %d, want 1 for environments that differ\n%s\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "~ base_url: http://127.0.0.1:") || !strings.Contains(out, "-> http://127.0.0.1:1\n") || !strings.Contains(out, "~ verify_wait: 10ms -> 2s") {
		t.Fatalf("output lacks the changed base_url:\n%s", out)
	}
}

func TestEnvDiff_Given_the_same_environment_twice_Then_exits_0(t *testing.T) {
	e := newSendEnv(t)

	code, out, _ := e.run(context.Background(), "env", "diff", "local", "local")

	if code != 0 || !strings.Contains(out, "are the same") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestEnvDiff_Given_an_unknown_environment_Then_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "env", "diff", "local", "nope")

	if code != 2 || !strings.Contains(errOut, "nope") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}
