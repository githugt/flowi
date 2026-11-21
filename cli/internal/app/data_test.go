// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

const dataFlow = "id: 01JA00000000000000000000E1\ninputs:\n  email:\n    default: nobody@x.test\nnodes:\n  - id: allowed\n    type: assert\n    check: \"inputs.email != 'bad@x.test'\"\n"

func TestRun_Given_a_data_file_Then_the_flow_runs_once_per_row_and_the_exit_code_is_the_worst(t *testing.T) {
	// Before: flowi run --data did not exist, and a failure in a data-driven run was only a number.
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "datadriven.arazzo.yaml"), arazzoFlow(dataFlow))
	data := filepath.Join(e.dir, "users.csv")
	writeFile(t, data, "email\nada@x.test\nbad@x.test\n")

	code, out, errOut := e.run(context.Background(), "run", "datadriven", "--data", data)

	if code != 1 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, want := range []string{"✓ ada@x.test", "✗ bad@x.test"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRun_Given_a_data_file_where_every_row_passes_Then_exit_0(t *testing.T) {
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "datadriven.arazzo.yaml"), arazzoFlow(dataFlow))
	data := filepath.Join(e.dir, "users.json")
	writeFile(t, data, `[{"email":"ada@x.test"},{"email":"bob@x.test"}]`)

	code, out, errOut := e.run(context.Background(), "run", "datadriven", "--data", data)

	if code != 0 || strings.Count(out, "✓") != 2 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestRun_Given_bad_data_usage_Then_exit_2(t *testing.T) {
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "datadriven.arazzo.yaml"), arazzoFlow(dataFlow))
	data := filepath.Join(e.dir, "users.csv")
	writeFile(t, data, "email\nada@x.test\n")
	tests := map[string][]string{
		"missing file":  {"run", "datadriven", "--data", filepath.Join(e.dir, "nope.csv")},
		"with a tag":    {"run", "--tag", "x", "--data", data},
		"with a matrix": {"run", "datadriven", "--data", data, "--matrix", "env=local"},
		"with all":      {"run", "--all", "--data", data},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := e.run(context.Background(), args...)

			if code != 2 {
				t.Fatalf("exit %d\n%s", code, errOut)
			}
		})
	}
}
