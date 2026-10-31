// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpact_exit_codes(t *testing.T) {
	e := newSendEnv(t)
	example := filepath.Join(e.dir, "example", "requests", "auth", "login.examples", "v2-200.json")
	tests := []struct {
		name   string
		args   []string
		edit   func(t *testing.T)
		code   int
		stderr []string
	}{
		{name: "all compatible", args: []string{"impact", "auth/login"}, code: 0},
		{name: "unknown request", args: []string{"impact", "nope"}, code: 2, stderr: []string{"FLW-REQ-001"}},
		{name: "unknown version", args: []string{"impact", "auth/login", "--version", "9"}, code: 2, stderr: []string{"FLW-VER-004"}},
		{name: "missing argument", args: []string{"impact"}, code: 2, stderr: []string{"a request or --against"}},
		{name: "broken consumer", args: []string{"impact", "auth/login"}, edit: func(t *testing.T) {
			data, err := os.ReadFile(example)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, example, strings.Replace(string(data), `"token"`, `"renamed"`, 1))
		}, code: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.edit != nil {
				tt.edit(t)
			}

			code, _, errOut := e.run(context.Background(), tt.args...)

			if code != tt.code {
				t.Fatalf("exit %d, want %d\n%s", code, tt.code, errOut)
			}
			for _, want := range tt.stderr {
				if !strings.Contains(errOut, want) {
					t.Errorf("stderr lacks %q:\n%s", want, errOut)
				}
			}
		})
	}
}

const petsSpec = `openapi: 3.0.3
info: {title: pets, version: 1.0.0}
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      parameters:
        - {name: petId, in: path, required: true, schema: {type: string}}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  FIELD: {type: string}
`

func TestImpact_Given_a_changed_spec_When_a_flow_reads_a_field_it_dropped_Then_exit_1_and_the_flow_and_step_are_named(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pets.yaml"), strings.Replace(petsSpec, "FIELD", "name", 1))
	writeFile(t, filepath.Join(dir, "pets-new.yaml"), strings.Replace(petsSpec, "FIELD", "label", 1))
	writeFile(t, filepath.Join(dir, "fetch.arazzo.yaml"), standaloneArazzo+"        outputs:\n          name: $response.body#/name\n")
	for _, args := range [][]string{
		{"import", "openapi", filepath.Join(dir, "pets.yaml"), "--collection", "Tools"},
		{"import", "arazzo", filepath.Join(dir, "fetch.arazzo.yaml"), "--collection", "Tools"},
	} {
		if code, _, errOut := e.run(context.Background(), args...); code != 0 {
			t.Fatalf("%v: exit %d\n%s", args, code, errOut)
		}
	}

	// Act
	same, _, _ := e.run(context.Background(), "impact", "--against", filepath.Join(dir, "pets.yaml"), "--collection", "Tools")
	code, out, errOut := e.run(context.Background(), "impact", "--against", filepath.Join(dir, "pets-new.yaml"), "--collection", "Tools")

	// Assert
	if same != 0 {
		t.Errorf("an unchanged spec: exit %d", same)
	}
	if code != 1 || !strings.Contains(out, "arazzo/fetchName") || !strings.Contains(out, "fetch") || !strings.Contains(out, "body.name") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
}

func TestImpact_Given_against_and_a_request_Then_it_is_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "impact", "auth/login", "--against", "x.yaml")

	if code != 2 || !strings.Contains(errOut, "not both") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestCoverage_Given_an_imported_spec_and_one_flow_Then_it_lists_used_and_unused_operations_and_can_fail_a_threshold(t *testing.T) {
	// Arrange
	e := newSendEnv(t)
	dir := t.TempDir()
	spec := strings.Replace(petsSpec, "FIELD", "name", 1) + "  /pets:\n    get:\n      operationId: listPets\n      responses:\n        \"200\":\n          description: ok\n          content:\n            application/json:\n              schema:\n                type: array\n                items: {type: object}\n"
	writeFile(t, filepath.Join(dir, "pets.yaml"), spec)
	writeFile(t, filepath.Join(dir, "fetch.arazzo.yaml"), standaloneArazzo+"        outputs:\n          name: $response.body#/name\n")
	for _, args := range [][]string{
		{"import", "openapi", filepath.Join(dir, "pets.yaml"), "--collection", "Tools"},
		{"import", "arazzo", filepath.Join(dir, "fetch.arazzo.yaml"), "--collection", "Tools"},
	} {
		if code, _, errOut := e.run(context.Background(), args...); code != 0 {
			t.Fatalf("%v: exit %d\n%s", args, code, errOut)
		}
	}

	// Act
	code, out, errOut := e.run(context.Background(), "coverage", "--collection", "Tools")
	failCode, _, _ := e.run(context.Background(), "coverage", "--collection", "Tools", "--fail-under", "100")

	// Assert
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, want := range []string{"getPet (version 1)", "arazzo/fetchName", "listPets (version 1)", "Not used by any flow", "1 of 2 operations"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}
	if failCode != 1 {
		t.Errorf("--fail-under 100 with a half-covered spec: exit %d", failCode)
	}
}
