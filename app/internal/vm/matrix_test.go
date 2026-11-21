// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"context"
	"errors"
	"strconv"
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

func TestRunRequestMatrix_Given_env_and_a_value_Then_every_combination_is_sent_with_them_and_nothing_is_saved(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{SendFor: func(o flowi.SendOptions) (*vmtest.Sending, error) {
		rec := &engine.Record{Status: 200}
		if o.With["id"] == "404" {
			rec.Status = 404
			d := diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "status == 200", "message": ""})
			return &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusFailed, Record: rec, Diags: diag.List{d}}}, nil
		}
		return &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed, Record: rec}}, nil
	}}
	dims := []vm.MatrixDim{{Name: "env", Values: []string{"local", "staging"}}, {Name: "with.id", Values: []string{"1", "404"}}}

	// Act
	cells, err := vm.RunRequestMatrix(context.Background(), b, flowi.RequestTarget{}, flowi.SendOptions{Seed: 3, Version: "2"}, dims)

	// Assert
	if err != nil || len(cells) != 4 || len(b.SendOpts) != 4 {
		t.Fatalf("err %v cells %d sends %d", err, len(cells), len(b.SendOpts))
	}
	if strings.Join(cells[1].Labels, "|") != "local|404" || cells[1].Status != "failed" || cells[1].Detail != "404" || cells[1].Why == "" {
		t.Fatalf("cell 1 %+v", cells[1])
	}
	if cells[2].Status != "passed" || cells[2].Detail != "200" {
		t.Fatalf("cell 2 %+v", cells[2])
	}
	o := b.SendOpts[3]
	if o.Env != "staging" || o.With["id"] != "404" || o.Save != "none" || o.Seed != 3 || o.Version != "2" {
		t.Fatalf("options of the last cell %+v", o)
	}
}

func TestRunRequestMatrix_Given_a_flow_style_input_dimension_Then_it_is_refused(t *testing.T) {
	b := &vmtest.Backend{}

	_, err := vm.RunRequestMatrix(context.Background(), b, flowi.RequestTarget{}, flowi.SendOptions{}, []vm.MatrixDim{{Name: "input.email", Values: []string{"a"}}})

	if err == nil || !strings.Contains(err.Error(), "with.<name>") || len(b.SendOpts) != 0 {
		t.Fatalf("err %v", err)
	}
}

func TestRunMatrixProgress_Given_three_cells_Then_progress_is_told_before_the_first_and_after_each(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{NextRun: passing()}
	dims := []vm.MatrixDim{{Name: "env", Values: []string{"a", "b", "c"}}}
	var seen []string

	// Act
	_, err := vm.RunMatrixProgress(context.Background(), b, flowi.FlowTarget{}, flowi.RunOptions{}, dims, func(done, total int) {
		seen = append(seen, strconv.Itoa(done)+"/"+strconv.Itoa(total))
	})

	// Assert
	if err != nil || strings.Join(seen, " ") != "0/3 1/3 2/3 3/3" {
		t.Fatalf("progress %v err %v", seen, err)
	}
}

func TestRunMatrixProgress_Given_the_context_is_cancelled_after_a_cell_Then_the_cells_so_far_come_back_with_the_error(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &vmtest.Backend{RunFor: func(o flowi.RunOptions) (*vmtest.Running, error) {
		if o.Env == "b" {
			cancel()
		}
		return passing(), nil
	}}
	dims := []vm.MatrixDim{{Name: "env", Values: []string{"a", "b", "c"}}}

	// Act
	cells, err := vm.RunMatrixProgress(ctx, b, flowi.FlowTarget{}, flowi.RunOptions{}, dims, nil)

	// Assert
	if !errors.Is(err, context.Canceled) || len(cells) != 2 || len(b.RunOptionsLog) != 2 {
		t.Fatalf("cells %d runs %d err %v", len(cells), len(b.RunOptionsLog), err)
	}
}
