// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"errors"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// checkTemplates reports templates and expressions that do not compile,
// so typos surface on validate instead of on send.
func checkTemplates(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, s := range sites.Templates(c) {
			if d, bad := compileSite(s); bad {
				out = append(out, d)
				continue
			}
			out = append(out, undeclaredCollectionVars(c, s)...)
		}
	}
	return out
}

func compileSite(s sites.Site) (diag.Diagnostic, bool) {
	var err error
	if s.Expression {
		err = template.CheckSyntax(s.Src)
	} else {
		_, err = template.Parse(s.Src)
	}
	if err == nil {
		return diag.Diagnostic{}, false
	}
	detail := err.Error()
	var se *template.SyntaxError
	if errors.As(err, &se) {
		detail = se.Detail
	}
	return diag.New(diag.CodeTemplateSyntax, diag.Location{File: s.File, Path: s.Path}, map[string]string{"field": s.Field, "detail": detail}), true
}

// undeclaredCollectionVars reports {{collection_var.x}} where the collection's flowi.yaml declares no variable x:
// only declared variables exist (an environment can override one, never add one), so the read always fails.
//
// Before: a typo such as {{collection_var.regoin}} was found only when the request was sent.
func undeclaredCollectionVars(c model.Collection, s sites.Site) diag.List {
	refs, err := template.References(s.Src, s.Expression)
	if err != nil {
		return nil
	}
	declared := make([]string, 0, len(c.Manifest.Variables))
	for _, kv := range c.Manifest.Variables {
		declared = append(declared, kv.Key)
	}
	var out diag.List
	for _, r := range refs {
		if len(r.Path) < 2 || r.Path[0] != "collection_var" || r.Guarded || slices.Contains(declared, r.Path[1]) {
			continue
		}
		hint := " (declare it under variables: in flowi.yaml)"
		if len(declared) > 0 {
			hint = " (the collection declares: " + strings.Join(declared, ", ") + ")"
		}
		out = append(out, diag.New(diag.CodeTemplateUndefined, diag.Location{File: s.File, Path: s.Path},
			map[string]string{"name": "collection_var." + r.Path[1], "field": s.Field, "hint": hint}))
	}
	return out
}
