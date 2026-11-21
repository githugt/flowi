// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	"github.com/githugt/flowi/lib/model"
)

func hintWorkspace(t *testing.T) *vm.Workspace {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Environments = []model.Environment{
		{Meta: model.Meta{Name: "base"}, File: "/c/environments/base.yaml", Vars: []model.EnvVar{{Name: "region", Value: "eu"}, {Name: "token", Secret: true, Value: "never-shown"}}},
		{Meta: model.Meta{Name: "staging"}, File: "/c/environments/staging.yaml", Extends: "base", Vars: []model.EnvVar{{Name: "base_url", Value: "https://api.example.com"}, {Name: "long", Value: strings.Repeat("x", 200)}, {Name: "empty"}}},
	}
	b := &vmtest.Backend{Cols: []model.Collection{c}, Values: map[string]any{"defaults.environment": "staging"}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	return ws
}

// Before: a variable in an address showed only as defined or not; its value was seen by opening the environment.
func TestWorkspaceVM_DescribeVariable_should_say_the_value_and_where_it_comes_from(t *testing.T) {
	ws := hintWorkspace(t)

	own := ws.DescribeVariable("env.base_url")
	inherited := ws.DescribeVariable(" region ")
	empty := ws.DescribeVariable("env.empty")
	long := ws.DescribeVariable("env.long")

	if own.Text != "base_url → https://api.example.com (from environment staging)" || !own.Known {
		t.Errorf("own %+v", own)
	}
	if inherited.Text != "region → eu (from environment base, which staging builds on)" || !inherited.Known {
		t.Errorf("inherited %+v", inherited)
	}
	if !strings.Contains(empty.Text, "(empty)") {
		t.Errorf("empty %+v", empty)
	}
	if !strings.HasSuffix(strings.Split(long.Text, " (from")[0], strings.Repeat("x", 80)+"…") || strings.Contains(long.Text, strings.Repeat("x", 81)) {
		t.Errorf("long %q", long.Text)
	}
}

func TestWorkspaceVM_DescribeVariable_should_never_show_a_secrets_value(t *testing.T) {
	ws := hintWorkspace(t)

	got := ws.DescribeVariable("secret.token")

	if got.Text != "token → •••• (secret of base, which staging builds on)" || !got.Known || strings.Contains(got.Text, "never-shown") {
		t.Fatalf("got %+v", got)
	}
}

func TestWorkspaceVM_DescribeVariable_Given_a_name_that_is_not_set_Then_it_is_unknown_and_names_the_environment_to_set_it_in(t *testing.T) {
	ws := hintWorkspace(t)

	plain := ws.DescribeVariable("env.nope")
	secret := ws.DescribeVariable("secret.missing")
	kindMismatch := ws.DescribeVariable("secret.region") // region is a plain variable: not a secret

	for _, h := range []vm.VariableHint{plain, secret, kindMismatch} {
		if h.Known || h.Environment != "staging" || !strings.Contains(h.Text, "not set in staging") {
			t.Errorf("hint %+v", h)
		}
	}
}

func TestWorkspaceVM_DescribeVariable_should_describe_generators_run_time_names_and_expressions(t *testing.T) {
	ws := hintWorkspace(t)

	gen := ws.DescribeVariable("$uuid-v4")
	bad := ws.DescribeVariable("$nope")
	run := ws.DescribeVariable("nodes.login.body.id")
	expr := ws.DescribeVariable("upper(env.region)")

	if !gen.Known || !strings.Contains(gen.Text, "Random UUID") || !strings.Contains(gen.Text, "every send") {
		t.Errorf("gen %+v", gen)
	}
	if bad.Known || bad.Environment != "" {
		t.Errorf("bad %+v", bad)
	}
	if !run.Known || !strings.HasPrefix(run.Text, "nodes:") || !expr.Known {
		t.Errorf("run %+v expr %+v", run, expr)
	}
}

func TestWorkspaceVM_DescribeVariable_Given_no_environment_is_chosen_Then_it_says_so_and_offers_no_place_to_set(t *testing.T) {
	ws := hintWorkspace(t)
	ws.SelectEnv("")

	got := ws.DescribeVariable("env.base_url")

	if got.Known || got.Environment != "" || !strings.Contains(got.Text, "no environment is chosen") {
		t.Fatalf("got %+v", got)
	}
}
