// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"errors"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/template"
)

// checkTemplates reports templates and expressions that do not compile,
// so typos surface on validate instead of on send.
func checkTemplates(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, s := range sites.Templates(c) {
			if d, bad := compileSite(s); bad {
				out = append(out, d)
			}
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
