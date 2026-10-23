// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkParams checks every node's parameters against its spec: kinds,
// enums, ranges, durations, file names; unknown keys are warnings.
func checkParams(ix *index) diag.List {
	specs := sites.SpecsByType()
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			if spec, ok := specs[n.Value.Type]; ok {
				out = append(out, nodeParamProblems(n, spec)...)
			}
		}
	}
	return out
}

func nodeParamProblems(n sites.Located[model.Node], spec nodes.Spec) diag.List {
	var out diag.List
	for _, f := range n.Value.Params {
		at := diag.Location{File: n.File, Path: yamlpath.Key(n.Path, f.Key)}
		p, known := spec.Param(f.Key)
		if !known {
			if !spec.MapForm {
				out = append(out, paramDiag(n.Value, f.Key, "unknown parameter", at, diag.SeverityWarning))
			}
			continue
		}
		if detail := paramProblem(n.Value, p, f.Value); detail != "" {
			out = append(out, paramDiag(n.Value, f.Key, detail, at, diag.SeverityError))
		}
	}
	return append(out, exclusiveProblems(n)...)
}

func paramDiag(n model.Node, field, detail string, at diag.Location, sev diag.Severity) diag.Diagnostic {
	d := diag.New(diag.CodeNodeParam, at, map[string]string{"node": n.ID, "field": field, "detail": detail})
	d.Severity = sev
	return d
}

// paramProblem says what is wrong with one value, or "".
func paramProblem(n model.Node, p nodes.Param, v any) string {
	text, isText := v.(string)
	switch p.Kind {
	case nodes.ParamString, nodes.ParamText, nodes.ParamExpr:
		if !isText {
			return "must be a string"
		}
		if n.Type == "delay" && p.Name == "duration" && !strings.Contains(text, "{{") {
			if _, _, err := nodes.ParseDelay(text); err != nil {
				return err.Error()
			}
		}
	case nodes.ParamDuration:
		if _, err := time.ParseDuration(text); !isText || err != nil {
			return fmt.Sprintf("%v is not a duration (e.g. 500ms, 5s)", v)
		}
	case nodes.ParamInt:
		return intProblem(p, v)
	case nodes.ParamBool:
		if _, ok := v.(bool); !ok {
			return "must be true or false"
		}
	case nodes.ParamEnum:
		if !isText || !slices.Contains(p.Enum, text) {
			return "must be one of " + strings.Join(p.Enum, ", ")
		}
	case nodes.ParamName:
		if !isText || !names.Valid(text) {
			return "must be a plain file name (letters, digits, '.', '_', '-'; no '/', not starting with '.')"
		}
	case nodes.ParamMap:
		if _, ok := v.([]model.Field); !ok {
			return "must be a mapping"
		}
	}
	return ""
}

func intProblem(p nodes.Param, v any) string {
	i, ok := v.(int64)
	switch {
	case !ok:
		return "must be a whole number"
	case i < p.Min:
		return fmt.Sprintf("must be at least %d", p.Min)
	case p.Max != 0 && i > p.Max:
		return fmt.Sprintf("must be at most %d", p.Max)
	}
	return ""
}

// exclusiveProblems are rules between parameters: batch needs exactly
// one of over and times.
func exclusiveProblems(n sites.Located[model.Node]) diag.List {
	if n.Value.Type != "batch" {
		return nil
	}
	if sites.HasParam(n.Value, "over") == sites.HasParam(n.Value, "times") {
		at := diag.Location{File: n.File, Path: n.Path}
		return diag.List{paramDiag(n.Value, "over/times", "use exactly one of over and times", at, diag.SeverityError)}
	}
	return nil
}
