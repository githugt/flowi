// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/githugt/flowi/lib/diag"
)

// maxGraphQLMessages is how many error messages the diagnostic quotes.
const maxGraphQLMessages = 3

// isGraphQLRequest says whether a request body is a GraphQL query: a JSON object whose "query" is text.
func isGraphQLRequest(body string) bool {
	var m map[string]any
	if json.Unmarshal([]byte(body), &m) != nil {
		return false
	}
	q, ok := m["query"].(string)
	return ok && strings.TrimSpace(q) != ""
}

// graphQLErrors returns the messages of the errors list of a GraphQL response, and whether it has any.
func graphQLErrors(response any) ([]string, bool) {
	m, ok := response.(map[string]any)
	if !ok {
		return nil, false
	}
	list, ok := m["errors"].([]any)
	if !ok || len(list) == 0 {
		return nil, false
	}
	msgs := make([]string, 0, len(list))
	for _, e := range list {
		msg := "(no message)"
		if em, ok := e.(map[string]any); ok {
			if s, ok := em["message"].(string); ok && s != "" {
				msg = s
			}
		}
		msgs = append(msgs, msg)
	}
	return msgs, true
}

// checkGraphQLErrors reports the errors of a GraphQL response. A GraphQL server answers 200 when a query
// failed, so without this a step with no assert on the body passes while it did nothing. A request that
// asserts on response.body.errors handles them itself and is left alone: a test that expects an error must not warn.
func (x *execution) checkGraphQLErrors(requestBody string) {
	if x.cfg.graphqlErrors == "ignore" || x.record == nil || !isGraphQLRequest(requestBody) {
		return
	}
	for _, a := range x.version.Assert {
		if strings.Contains(a, "response.body.errors") {
			return
		}
	}
	msgs, ok := graphQLErrors(x.record.Body.JSON)
	if !ok {
		return
	}
	shown := msgs[:min(len(msgs), maxGraphQLMessages)]
	detail := strings.Join(shown, "; ")
	if len(msgs) > len(shown) {
		detail += fmt.Sprintf("; and %d more", len(msgs)-len(shown))
	}
	count := fmt.Sprintf("%d error(s)", len(msgs))
	args := map[string]string{"request": x.c.request.Path, "count": count, "detail": x.r.secrets.String(detail)}
	if x.cfg.graphqlErrors == "fail" {
		x.fail(StatusFailed, diag.List{diag.New(diag.CodeRunGraphQLError, diag.Location{}, args)})
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.diags = append(x.diags, diag.New(diag.CodeRunGraphQLWarning, diag.Location{}, args))
}
