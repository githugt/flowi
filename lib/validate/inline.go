// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

func missingParam(key string) requiredField {
	return requiredField{key, func(n model.Node) bool { return !sites.HasParam(n, key) }}
}

// paramRequired is what node types need beyond their typed fields: every
// spec parameter marked required, plus rules that depend on other
// parameters.
func paramRequired() map[model.NodeType][]requiredField {
	out := map[model.NodeType][]requiredField{}
	for _, s := range nodes.Specs() {
		for _, p := range s.Params {
			if p.Required {
				out[s.Type] = append(out[s.Type], missingParam(p.Name))
			}
		}
	}
	out["set"] = append(out["set"],
		requiredField{"values", func(n model.Node) bool { return len(n.Values) == 0 && !sites.HasParam(n, "target") }},
		requiredField{"value", func(n model.Node) bool { return sites.HasParam(n, "target") && !sites.HasParam(n, "value") }})
	out["assert"] = append(out["assert"], requiredField{"check", func(n model.Node) bool { return !sites.HasParam(n, "check") && !sites.HasParam(n, "schema") }})
	out["digest"] = append(out["digest"], missingForSink("file", "file"), missingForSink("webhook", "url"))
	out["condition"] = append(out["condition"], requiredField{"check", func(n model.Node) bool { return len(n.Cases) == 0 && !sites.HasParam(n, "check") }})
	return out
}

func missingForSink(sink, key string) requiredField {
	return requiredField{key, func(n model.Node) bool { return sites.ParamValue(n, "sink") == sink && !sites.HasParam(n, key) }}
}

// checkSetTargets warns about set targets the runner will refuse; at run
// time they are errors.
func checkSetTargets(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			if n.Value.Type != "set" {
				continue
			}
			target, _ := sites.ParamValue(n.Value, "target").(string)
			if target == "" || validTarget(target) {
				continue
			}
			d := diag.New(diag.CodeRunSetTarget, diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "target")},
				map[string]string{"target": target, "detail": "unknown target (use vars.<name>, request.<field> or cookies.<name>)"})
			d.Severity = diag.SeverityWarning
			out = append(out, d)
		}
	}
	return out
}

func validTarget(t string) bool {
	for _, prefix := range []string{"vars.", "request.", "cookies."} {
		if strings.HasPrefix(t, prefix) && len(t) > len(prefix) {
			return true
		}
	}
	return false
}
