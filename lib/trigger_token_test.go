// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"strings"
	"testing"
)

func tokenWorkspace(t *testing.T, environ ...string) (*Workspace, env) {
	t.Helper()
	e := newEnv(t, environ...)
	e.opts.Keys = &memKeys{}
	return e.open(t), e
}

func TestTriggerToken_Given_none_set_Then_not_ok(t *testing.T) {
	w, _ := tokenWorkspace(t)

	got, ok, err := w.TriggerToken()

	if err != nil || ok || got != "" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
}

func TestNewTriggerToken_should_generate_store_and_rotate(t *testing.T) {
	w, e := tokenWorkspace(t)

	first, err := w.NewTriggerToken()
	if err != nil {
		t.Fatal(err)
	}
	stored, ok, err := w.TriggerToken()
	second, err2 := w.NewTriggerToken()

	if err != nil || err2 != nil || !ok || stored != first {
		t.Fatalf("stored %q (ok %v, %v), first %q", stored, ok, err, first)
	}
	if len(first) < 40 || first == second {
		t.Fatalf("tokens %q then %q: want long, distinct tokens", first, second)
	}
	data, _ := os.ReadFile(workspaceFile(e))
	if strings.Contains(string(data), first) {
		t.Fatal("the token is in workspace.yaml")
	}
}

func TestTriggerToken_should_prefer_the_environment_override(t *testing.T) {
	w, _ := tokenWorkspace(t, "FLOWI_SECRET_SERVE_TRIGGER_TOKEN=from-env")
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}

	got, ok, err := w.TriggerToken()

	if err != nil || !ok || got != "from-env" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
}

func TestHasTriggerToken_should_say_whether_a_token_exists_without_reading_it(t *testing.T) {
	w, _ := tokenWorkspace(t)
	before, errBefore := w.HasTriggerToken()
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}

	after, errAfter := w.HasTriggerToken()

	if errBefore != nil || errAfter != nil || before || !after {
		t.Fatalf("before %v (%v), after %v (%v)", before, errBefore, after, errAfter)
	}
}
