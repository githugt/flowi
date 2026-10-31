// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	flowi "github.com/githugt/flowi/lib"
)

// maxMatrixCells keeps a typo from starting a thousand runs.
const maxMatrixCells = 100

// dimension is one axis of a matrix run: what varies, and the values it takes.
type dimension struct {
	name   string // env, overlay or input.<name>
	values []string
}

// parseMatrix reads --matrix specs such as env=local,staging, overlay=vip,guest or input.email=a@x,b@x.
func parseMatrix(specs []string) ([]dimension, error) {
	var dims []dimension
	for _, spec := range specs {
		name, list, ok := strings.Cut(spec, "=")
		if !ok || list == "" {
			return nil, &UsageError{Err: fmt.Errorf("--matrix wants name=value,value, got %q", spec)}
		}
		if !validDimension(name) {
			return nil, &UsageError{Err: fmt.Errorf("--matrix %q: the dimension is env, overlay or input.<name>", spec)}
		}
		if slices.ContainsFunc(dims, func(d dimension) bool { return d.name == name }) {
			return nil, &UsageError{Err: fmt.Errorf("--matrix names %s twice; list its values in one --matrix", name)}
		}
		values := strings.Split(list, ",")
		if hasDuplicates(values) || slices.Contains(values, "") {
			return nil, &UsageError{Err: fmt.Errorf("--matrix %q: values must be distinct and not empty", spec)}
		}
		dims = append(dims, dimension{name: name, values: values})
	}
	return dims, nil
}

func validDimension(name string) bool {
	if name == "env" || name == "overlay" {
		return true
	}
	input, ok := strings.CutPrefix(name, "input.")
	return ok && input != ""
}

func hasDuplicates(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			return true
		}
		seen[v] = true
	}
	return false
}

// checkMatrixSize refuses a matrix with more than maxMatrixCells cells.
func checkMatrixSize(dims []dimension) error {
	n := 1
	for _, d := range dims {
		n *= len(d.values)
	}
	if n > maxMatrixCells {
		return &UsageError{Err: fmt.Errorf("the matrix has %d cells; the most one command runs is %d", n, maxMatrixCells)}
	}
	return nil
}

// cell is one combination: the value taken on each dimension, in the order of the dimensions.
type cell struct{ values []string }

func (c cell) label() string { return strings.Join(c.values, " · ") }

// matrixCells lists every combination. The first dimension varies slowest, so cells of one row stay together.
func matrixCells(dims []dimension) []cell {
	cells := []cell{{}}
	for _, d := range dims {
		var next []cell
		for _, c := range cells {
			for _, v := range d.values {
				next = append(next, cell{values: append(slices.Clone(c.values), v)})
			}
		}
		cells = next
	}
	return cells
}

// options is base with this cell's values set. base is not changed.
func (c cell) options(base flowi.RunOptions, dims []dimension) flowi.RunOptions {
	out := base
	out.Inputs = make(map[string]string, len(base.Inputs)+1)
	for k, v := range base.Inputs {
		out.Inputs[k] = v
	}
	for i, d := range dims {
		switch d.name {
		case "env":
			out.Env = c.values[i]
		case "overlay":
			out.Overlay = c.values[i]
		default:
			out.Inputs[strings.TrimPrefix(d.name, "input.")] = c.values[i]
		}
	}
	return out
}

// outcome is how one cell ended.
type outcome int

const (
	outcomePassed  outcome = iota
	outcomeFailed          // a step or check failed
	outcomeError           // a system failure: the network, a report that could not be written
	outcomeStopped         // cancelled
)

func (o outcome) glyph() string { return [...]string{"✓", "✗", "!", "–"}[o] }

// cellResult is a cell and how its run ended; why is the first thing that went wrong.
type cellResult struct {
	cell    cell
	outcome outcome
	why     string
}

// renderGrid prints the results. Two dimensions make a grid (the first down, the second across); any other
// number is a list with a line for each cell. Every cell that did not pass is explained below.
func renderGrid(w io.Writer, dims []dimension, results []cellResult) {
	if len(dims) == 2 {
		renderTwoWay(w, dims, results)
	} else {
		for _, r := range results {
			fmt.Fprintf(w, "%s %s\n", r.outcome.glyph(), r.cell.label())
		}
	}
	for _, r := range results {
		if r.outcome != outcomePassed && r.why != "" {
			fmt.Fprintf(w, "  %s: %s\n", r.cell.label(), r.why)
		}
	}
}

func renderTwoWay(w io.Writer, dims []dimension, results []cellResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "\t%s\n", dims[1].name)
	fmt.Fprintf(tw, "%s\t%s\n", dims[0].name, strings.Join(dims[1].values, "\t"))
	i := 0
	for _, row := range dims[0].values {
		glyphs := make([]string, 0, len(dims[1].values))
		for range dims[1].values {
			glyphs = append(glyphs, results[i].outcome.glyph())
			i++
		}
		fmt.Fprintf(tw, "%s\t%s\n", row, strings.Join(glyphs, "\t"))
	}
	_ = tw.Flush() // a write to the command's own output
}
