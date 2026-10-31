// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"fmt"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
)

// MaxMatrixCells keeps a mistake from starting a thousand runs.
const MaxMatrixCells = 100

// MatrixDim is one axis of a matrix run: env, overlay or input.<name>, and the values it takes.
type MatrixDim struct {
	Name   string
	Values []string
}

// MatrixCell is one combination and how its run ended: Status is passed, failed, error or cancelled, and Why the first
// thing that went wrong.
type MatrixCell struct {
	Labels []string // the value taken on each dimension, in the order of the dimensions
	Status string
	Why    string
}

// RunMatrix runs the flow once for every combination of the dimensions' values (the first dimension varies slowest)
// and reports each. Cells run one after the other; a cell that cannot start (an unknown environment) is an error cell
// and the others still run. It must be called off the UI goroutine, since it waits for every run.
func RunMatrix(ctx context.Context, b Backend, t flowi.FlowTarget, base flowi.RunOptions, dims []MatrixDim) ([]MatrixCell, error) {
	if err := checkMatrix(dims); err != nil {
		return nil, err
	}
	var out []MatrixCell
	for _, labels := range combinations(dims) {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out = append(out, runCell(ctx, b, t, cellOptions(base, dims, labels), labels))
	}
	return out, nil
}

func checkMatrix(dims []MatrixDim) error {
	if len(dims) == 0 {
		return fmt.Errorf("choose at least one thing to vary")
	}
	cells := 1
	for i, d := range dims {
		input, isInput := strings.CutPrefix(d.Name, "input.")
		if d.Name != "env" && d.Name != "overlay" && (!isInput || input == "") {
			return fmt.Errorf("a matrix varies env, overlay or input.<name>, not %q", d.Name)
		}
		if slices.ContainsFunc(dims[:i], func(o MatrixDim) bool { return o.Name == d.Name }) {
			return fmt.Errorf("%s is listed twice: give its values once", d.Name)
		}
		if len(d.Values) == 0 || slices.Contains(d.Values, "") {
			return fmt.Errorf("%s needs at least one value, and none may be empty", d.Name)
		}
		if len(slices.Compact(slices.Sorted(slices.Values(d.Values)))) != len(d.Values) {
			return fmt.Errorf("the values of %s must be different from each other", d.Name)
		}
		cells *= len(d.Values)
	}
	if cells > MaxMatrixCells {
		return fmt.Errorf("the matrix has %d cells; the most one run does is %d", cells, MaxMatrixCells)
	}
	return nil
}

func combinations(dims []MatrixDim) [][]string {
	out := [][]string{{}}
	for _, d := range dims {
		var next [][]string
		for _, prefix := range out {
			for _, v := range d.Values {
				next = append(next, append(slices.Clone(prefix), v))
			}
		}
		out = next
	}
	return out
}

// cellOptions is base with one combination's values set; base is not changed.
func cellOptions(base flowi.RunOptions, dims []MatrixDim, labels []string) flowi.RunOptions {
	o := base
	o.Inputs = make(map[string]string, len(base.Inputs)+1)
	for k, v := range base.Inputs {
		o.Inputs[k] = v
	}
	for i, d := range dims {
		switch d.Name {
		case "env":
			o.Env = labels[i]
		case "overlay":
			o.Overlay = labels[i]
		default:
			o.Inputs[strings.TrimPrefix(d.Name, "input.")] = labels[i]
		}
	}
	return o
}

func runCell(ctx context.Context, b Backend, t flowi.FlowTarget, o flowi.RunOptions, labels []string) MatrixCell {
	cell := MatrixCell{Labels: labels}
	running, err := b.RunFlow(ctx, t, o)
	if err != nil {
		cell.Status, cell.Why = "error", ErrorText(err)
		return cell
	}
	for range running.Events() { //nolint:revive // a cell shows nothing of its own: draining lets the run end
	}
	res := running.Wait()
	cell.Status = cellStatus(res.Status)
	if cell.Status != "passed" {
		cell.Why = firstProblem(res)
	}
	return cell
}

func cellStatus(s engine.Status) string {
	switch s {
	case engine.StatusPassed:
		return "passed"
	case engine.StatusFailed:
		return "failed"
	case engine.StatusCancelled, engine.StatusAborted:
		return "cancelled"
	}
	return "error"
}

// firstProblem is the first error of a run, as a sentence with its code, or the run's own status.
func firstProblem(res engine.RunResult) string {
	for _, d := range res.Diags {
		if d.Severity == diag.SeverityError {
			msg, _ := diag.English().Message(d)
			return string(d.Code) + " " + msg
		}
	}
	return "the run " + string(res.Status)
}
