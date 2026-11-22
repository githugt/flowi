// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"unicode"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// maxBatchTimes bounds times: so a typo cannot start a billion iterations.
const maxBatchTimes = 100000

func batchSpec() Spec {
	return Spec{
		Type: "batch", Group: "Control", Summary: "Run the body for each item of a list, or N times, with bounded concurrency.",
		Params: []Param{
			{Name: "over", Kind: ParamText, Doc: "The list to repeat over, usually a list an earlier step produced or an input. The steps inside run once per item; there `item` is the current one and `index` its position. Set over or times, not both."},
			{Name: "times", Kind: ParamInt, Min: 1, Max: maxBatchTimes, Doc: "Repeat a fixed number of times instead of over a list; `index` counts from 0 (and `item` is the same number)."},
			{Name: "concurrency", Kind: ParamInt, Min: 1, Doc: "Iterations in flight (default 1); capped by the run-wide limit."},
			{Name: "label", Kind: ParamText, Doc: "Names each iteration in the run tree, reports and failures instead of its number (item, index in scope): `{{item.email}}` gives users[ada@example.com]/create. Slashes and brackets become _, long names are cut, repeats get #2."},
			loopLayoutParam(),
		},
		Typed: []string{"body"},
		Run:   runBatch,
	}
}

// runBatch runs one isolated lane per item and gathers the results in
// input order.
func runBatch(ctx context.Context, n model.Node, s *State) Outcome {
	items, out := batchItems(n, s)
	if out.Diags.HasErrors() {
		return out
	}
	labels, out := batchLabels(n, s, items)
	if out.Diags.HasErrors() {
		return out
	}
	lanes := make([]Lane, len(items))
	for i, item := range items {
		lanes[i] = Lane{Label: labels[i], Nodes: n.Body, OwnVars: true, OwnNodes: true, Item: item, Index: i, HasItem: true}
	}
	results := s.Runtime.Lanes(ctx, lanes, LaneOptions{Concurrency: intParam(n, "concurrency", 1), LaneJars: true, Loop: true})
	return Outcome{Outputs: batchOutputs(items, results), Status: combine(results), Diags: diagsOf(results)}
}

func batchItems(n model.Node, s *State) ([]any, Outcome) {
	if _, hasOver := param(n, "over"); !hasOver {
		count := intParam(n, "times", 0)
		if count < 1 || count > maxBatchTimes {
			// validate refuses this; the engine is public, so refuse it here too
			return nil, failed(n, diag.CodeNodeParam, map[string]string{"field": "times", "detail": "must be between 1 and " + strconv.Itoa(maxBatchTimes)})
		}
		items := make([]any, count)
		for i := range items {
			items[i] = i
		}
		return items, Outcome{}
	}
	raw, _ := param(n, "over")
	v, err := eval(raw, s)
	if err != nil {
		return nil, Outcome{Diags: templateDiags(n, "over", err)}
	}
	list, ok := v.([]any)
	if !ok {
		return nil, failed(n, diag.CodeRunBatchOver, map[string]string{"detail": "got " + describe(v) + " from " + template.String(raw)})
	}
	return list, Outcome{}
}

// batchOutputs are the results per item; left says a loop_control left the loop early, so the items after it
// (status skipped) never started.
func batchOutputs(items []any, results []LaneResult) map[string]any {
	out := make([]any, len(results))
	passed, failedCount, left := 0, 0, false
	for i, r := range results {
		left = left || r.Left
		switch r.Status {
		case StatusPassed:
			passed++
		case StatusFailed, StatusError:
			failedCount++
		}
		out[i] = map[string]any{"index": i, "item": items[i], "status": string(r.Status), "vars": r.Vars, "nodes": r.Nodes}
	}
	return map[string]any{"results": out, "count": len(results), "passed": passed, "failed": failedCount, "left": left}
}

// maxLabelRunes cuts a label that came from data, so a long cell cannot make a path nobody can read.
const maxLabelRunes = 40

// batchLabels are the lane labels: "[0]", "[1]"... or, with a label template, "[<what it renders>]" for each item.
// The label is part of the step's path, so it is cleaned (see cleanLabel) and kept unique.
func batchLabels(n model.Node, s *State, items []any) ([]string, Outcome) {
	labels := make([]string, len(items))
	raw, has := param(n, "label")
	text, isText := raw.(string)
	if !has || !isText || strings.TrimSpace(text) == "" {
		for i := range items {
			labels[i] = "[" + strconv.Itoa(i) + "]"
		}
		return labels, Outcome{}
	}
	tpl, err := template.Parse(text)
	if err != nil {
		return nil, Outcome{Diags: templateDiags(n, "label", err)}
	}
	seen := map[string]int{}
	for i, item := range items {
		sc := s.Scope()
		values := maps.Clone(sc.Values)
		if values == nil {
			values = map[string]any{}
		}
		values["item"], values["index"] = item, i
		sc.Values = values
		r, err := tpl.Eval(sc)
		if err != nil {
			return nil, Outcome{Diags: templateDiags(n, "label", err)}
		}
		s.derived(r)
		name := cleanLabel(fmt.Sprint(r.Value))
		if name == "" {
			name = strconv.Itoa(i)
		}
		if seen[name]++; seen[name] > 1 {
			name += "#" + strconv.Itoa(seen[name])
		}
		labels[i] = "[" + name + "]"
	}
	return labels, Outcome{}
}

// cleanLabel makes text safe as part of a step path: no slash or bracket (they split and close it), no control
// character, trimmed, and at most maxLabelRunes long.
func cleanLabel(text string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '/' || r == '[' || r == ']' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(text))
	if r := []rune(clean); len(r) > maxLabelRunes {
		clean = string(r[:maxLabelRunes])
	}
	return clean
}
