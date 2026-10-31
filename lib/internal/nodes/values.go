// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"fmt"
	"math"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// runValues writes a set node's values in order, so a later value may read
// an earlier one. A value that fails is reported and the rest still run.
func runValues(n model.Node, s *State) Outcome {
	written := map[string]any{}
	var diags diag.List
	for _, v := range n.Values {
		got, d := readValue(n, v, s)
		if len(d) > 0 {
			diags = append(diags, d...)
			continue
		}
		s.Vars.Set(v.Name, got)
		written[v.Name] = got
	}
	return Outcome{Outputs: written, Diags: diags}
}

// readValue is the value v stands for, by its source, or why it has none.
func readValue(n model.Node, v model.SetValue, s *State) (any, diag.List) {
	switch v.From {
	case "literal":
		return v.Value, nil
	case "response":
		return fromResponse(n, v, s)
	case "generate":
		return evalValue(n, v, withValueSeed(s, v.Seed))
	case "", "expr":
		return evalValue(n, v, s)
	}
	return nil, diag.List{newDiag(n, diag.CodeRunExtractEmpty, map[string]string{"name": v.Name, "detail": "unknown source " + v.From + " (use expr, literal, response or generate)"})}
}

// evalValue renders the template and requires a finite number when the
// result is one: a division by zero is a mistake, not a value.
func evalValue(n model.Node, v model.SetValue, s *State) (any, diag.List) {
	got, err := eval(v.Value, s)
	if err != nil {
		return nil, templateDiags(n, "value "+v.Name, err)
	}
	if f, isFloat := got.(float64); isFloat && (math.IsNaN(f) || math.IsInf(f, 0)) {
		return nil, diag.List{newDiag(n, diag.CodeRunCalcNumber, map[string]string{"name": v.Name, "detail": fmt.Sprintf("got %v (division by zero?)", f)})}
	}
	return got, nil
}

func fromResponse(n model.Node, v model.SetValue, s *State) (any, diag.List) {
	if s.Response == nil {
		return nil, diag.List{newDiag(n, diag.CodeRunExtractNoResp, nil)}
	}
	x := model.Extraction{Name: v.Name, From: v.Where, Path: v.Path, Header: v.Header, Cookie: v.Cookie, Regex: v.Regex}
	got, err := Extract(x, s.Response, s.Cookies)
	if err != nil {
		return nil, diag.List{newDiag(n, diag.CodeRunExtractEmpty, map[string]string{"name": v.Name, "detail": err.Error()})}
	}
	return got, nil
}

// withValueSeed is s with generators seeded by seed, or s itself without one.
func withValueSeed(s *State, seed int64) *State {
	if seed == 0 {
		return s
	}
	return withSeed(s, seed)
}

// withSeed is s with generators seeded by seed.
func withSeed(s *State, seed int64) *State {
	gen := template.NewGeneration(uint64(seed), s.Runtime.Now).Func() //nolint:gosec // seeds are positive (validated); the bit pattern is all that matters
	seeded := *s
	inner := s.Scope
	seeded.Scope = func() template.Scope {
		sc := inner()
		sc.Gen = gen
		return sc
	}
	return &seeded
}

func describe(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%T %v", v, v)
}
