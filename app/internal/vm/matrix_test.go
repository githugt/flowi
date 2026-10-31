// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func passing() *vmtest.Running {
	return &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}
}

func TestRunMatrix_Given_two_dimensions_Then_every_combination_runs_with_its_options_and_the_first_varies_slowest(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{NextRun: passing()}
	dims := []vm.MatrixDim{{Name: "env", Values: []string{"local", "staging"}}, {Name: "overlay", Values: []string{"vip", "guest"}}, {Name: "input.email", Values: []string{"a@x.test"}}}

	// Act
	cells, err := vm.RunMatrix(context.Background(), b, flowi.FlowTarget{}, flowi.RunOptions{Seed: 9}, dims)

	// Assert
	if err != nil || len(cells) != 4 || len(b.RunOptionsLog) != 4 {
		t.Fatalf("err %v cells %d runs %d", err, len(cells), len(b.RunOptionsLog))
	}
	if strings.Join(cells[1].Labels, "|") != "local|guest|a@x.test" || cells[3].Status != "passed" {
		t.Fatalf("cell 1 %+v cell 3 %+v", cells[1], cells[3])
	}
	o := b.RunOptionsLog[3]
	if o.Env != "staging" || o.Overlay != "guest" || o.Inputs["email"] != "a@x.test" || o.Seed != 9 {
		t.Fatalf("options of the last cell %+v", o)
	}
}

func TestRunMatrix_Given_a_cell_that_fails_or_cannot_start_Then_it_says_why_and_the_others_still_run(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{RunFor: func(o flowi.RunOptions) (*vmtest.Running, error) {
		switch o.Env {
		case "staging":
			d := diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "status == 200", "message": ""})
			return &vmtest.Running{Result: engine.RunResult{Status: engine.StatusFailed, Diags: diag.List{d}}}, nil
		case "gone":
			return nil, errors.New("environment \"gone\" not found")
		}
		return passing(), nil
	}}
	dims := []vm.MatrixDim{{Name: "env", Values: []string{"local", "staging", "gone"}}}

	// Act
	cells, err := vm.RunMatrix(context.Background(), b, flowi.FlowTarget{}, flowi.RunOptions{}, dims)

	// Assert
	if err != nil || len(cells) != 3 {
		t.Fatalf("err %v cells %+v", err, cells)
	}
	if cells[0].Status != "passed" || cells[1].Status != "failed" || !strings.Contains(cells[1].Why, "status == 200") || cells[2].Status != "error" || !strings.Contains(cells[2].Why, "gone") {
		t.Fatalf("cells %+v", cells)
	}
}

func TestRunMatrix_Given_a_bad_matrix_Then_nothing_runs(t *testing.T) {
	big := make([]string, 11)
	for i := range big {
		big[i] = string(rune('a' + i))
	}
	for name, dims := range map[string][]vm.MatrixDim{
		"none":              nil,
		"empty values":      {{Name: "env"}},
		"unknown dimension": {{Name: "colour", Values: []string{"red"}}},
		"twice":             {{Name: "env", Values: []string{"a"}}, {Name: "env", Values: []string{"b"}}},
		"repeated value":    {{Name: "env", Values: []string{"a", "a"}}},
		"too many cells":    {{Name: "env", Values: big}, {Name: "overlay", Values: big}},
	} {
		t.Run(name, func(t *testing.T) {
			b := &vmtest.Backend{NextRun: passing()}

			_, err := vm.RunMatrix(context.Background(), b, flowi.FlowTarget{}, flowi.RunOptions{}, dims)

			if err == nil || len(b.RunOptionsLog) != 0 {
				t.Fatalf("err %v runs %d", err, len(b.RunOptionsLog))
			}
		})
	}
}
