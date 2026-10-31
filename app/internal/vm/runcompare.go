// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
)

// Line kinds of a comparison.
const (
	LineSame = "same"
	LineAdd  = "add" // only in the later run
	LineDel  = "del" // only in the earlier run
)

// BodyLine is one line of a node's outputs in a comparison of two runs.
type BodyLine struct {
	Kind string
	Text string
}

// VarChange is a variable that differs between two runs: what it held in
// each ("" when the run did not have it; Had* says which).
type VarChange struct {
	Name                string
	Before, After       string
	HadBefore, HadAfter bool
}

// RunComparison is what two saved runs did at each top-level node, line by
// line, and which variables ended differently.
type RunComparison struct {
	Nodes map[string][]BodyLine // by node id
	Vars  []VarChange
}

// maxDiffLines bounds each side of a line diff: the work grows with their
// product, and a thousand lines of body is a lot to read as a diff anyway.
const maxDiffLines = 2000

// volatileOutputs change on every run and say nothing about what the node did.
func volatileOutputs() []string { return []string{"time"} }

// OutputLines shows a node's outputs as pretty JSON, one line each, without
// the outputs that differ on every run.
func OutputLines(outputs map[string]any) []string {
	shown := maps.Clone(outputs)
	for _, name := range volatileOutputs() {
		delete(shown, name)
	}
	if len(shown) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(shown, "", "  ")
	if err != nil {
		return []string{fmt.Sprint(shown)}
	}
	return strings.Split(string(data), "\n")
}

// LineDiff lines up before and after: the longest common run of lines is
// kept as it is, the rest is deleted from before or added by after. Past
// maxDiffLines on either side the lines are shown as wholly replaced, so a
// huge body cannot stall the window.
func LineDiff(before, after []string) []BodyLine {
	if len(before) > maxDiffLines || len(after) > maxDiffLines {
		return replaced(before, after)
	}
	lcs := make([][]int, len(before)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []BodyLine
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		switch {
		case before[i] == after[j]:
			out = append(out, BodyLine{LineSame, before[i]})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, BodyLine{LineDel, before[i]})
			i++
		default:
			out = append(out, BodyLine{LineAdd, after[j]})
			j++
		}
	}
	return append(append(out, tagged(LineDel, before[i:])...), tagged(LineAdd, after[j:])...)
}

func replaced(before, after []string) []BodyLine {
	return append(tagged(LineDel, before), tagged(LineAdd, after)...)
}

func tagged(kind string, lines []string) []BodyLine {
	out := make([]BodyLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, BodyLine{kind, l})
	}
	return out
}

// CompareDetails compares what two runs did at each node they have, and the
// variables they ended with. Only nodes whose outputs differ are listed.
func CompareDetails(before, after flowi.RunDetail) RunComparison {
	out := RunComparison{Nodes: map[string][]BodyLine{}}
	for id, outputs := range after.Outputs {
		lines := LineDiff(OutputLines(before.Outputs[id]), OutputLines(outputs))
		if slices.ContainsFunc(lines, func(l BodyLine) bool { return l.Kind != LineSame }) {
			out.Nodes[id] = lines
		}
	}
	for id, outputs := range before.Outputs {
		if _, kept := after.Outputs[id]; !kept {
			out.Nodes[id] = LineDiff(OutputLines(outputs), nil)
		}
	}
	out.Vars = varChanges(before.Vars, after.Vars)
	return out
}

func varChanges(before, after map[string]any) []VarChange {
	names := slices.Sorted(maps.Keys(before))
	for name := range after {
		if _, had := before[name]; !had {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var out []VarChange
	for _, name := range names {
		b, hadB := before[name]
		a, hadA := after[name]
		bs, as := oneLineValue(b), oneLineValue(a)
		if hadB == hadA && bs == as {
			continue
		}
		out = append(out, VarChange{Name: name, Before: bs, After: as, HadBefore: hadB, HadAfter: hadA})
	}
	return out
}

func oneLineValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}
