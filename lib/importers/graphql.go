// SPDX-License-Identifier: Apache-2.0

package importers

import (
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// GraphQLBody makes the GraphQL body of an imported request: the query as written, the variables as a template
// (text translates the other tool's variable syntax, redact keeps credentials out). ok is false when the query
// itself holds {{ : Flowi sends a document as written, so the caller falls back to a templated JSON body, which
// keeps the values working, and says so in the report.
func GraphQLBody(query, variables, operation string, text, redact func(string) string) (body model.Body, ok bool) {
	if strings.Contains(query, "{{") {
		return model.Body{}, false
	}
	g := model.GraphQL{Query: query, Operation: operation}
	if vars := strings.TrimSpace(variables); vars != "" {
		g.Variables = redact(text(vars))
	}
	return model.Body{GraphQL: &g}, true
}
