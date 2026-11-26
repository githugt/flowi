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

// graphQLErrorMode is what the response errors of this request do: fail, warn or ignore. With requests.graphql_errors
// at auto, a request with a GraphQL body fails, and an HTTP request whose JSON only looks like a query warns (it may
// not be GraphQL at all). ok is false when the request is not GraphQL, so there is nothing to check.
func (x *execution) graphQLErrorMode(requestBody string) (mode string, ok bool) {
	native := x.version.Body.GraphQL != nil
	if !native && !isGraphQLRequest(requestBody) {
		return "", false
	}
	mode = x.cfg.graphqlErrors
	if mode == "auto" || mode == "" {
		mode = "warn"
		if native {
			mode = "fail"
		}
	}
	return mode, mode != "ignore"
}

// checkGraphQLErrors reports the errors of a GraphQL response. A GraphQL server answers 200 when an operation
// failed, so without this a step with no assert on the body passes while it did nothing. A request that
// asserts on response.body.errors handles them itself and is left alone: a test that expects an error must not fail.
func (x *execution) checkGraphQLErrors(requestBody string) {
	if x.record == nil {
		return
	}
	x.reportGraphQLErrors([]any{x.record.Body.JSON}, requestBody)
}

// reportGraphQLErrors is checkGraphQLErrors for the answers it is given: the response of a query, or the events of a
// subscription. The errors of all of them are reported together.
func (x *execution) reportGraphQLErrors(responses []any, requestBody string) {
	mode, ok := x.graphQLErrorMode(requestBody)
	if !ok {
		return
	}
	for _, a := range x.version.Assert {
		if strings.Contains(a, "body.errors") { // response.body.errors for a response; body.errors or messages[n].body.errors for an event
			return
		}
	}
	var msgs []string
	for _, r := range responses {
		if m, ok := graphQLErrors(r); ok {
			msgs = append(msgs, m...)
		}
	}
	if len(msgs) == 0 {
		return
	}
	shown := msgs[:min(len(msgs), maxGraphQLMessages)]
	detail := strings.Join(shown, "; ")
	if len(msgs) > len(shown) {
		detail += fmt.Sprintf("; and %d more", len(msgs)-len(shown))
	}
	count := fmt.Sprintf("%d error(s)", len(msgs))
	args := map[string]string{"request": x.c.request.Path, "count": count, "detail": x.r.secrets.String(detail)}
	if mode == "fail" {
		x.fail(StatusFailed, diag.List{diag.New(diag.CodeRunGraphQLError, diag.Location{}, args)})
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.diags = append(x.diags, diag.New(diag.CodeRunGraphQLWarning, diag.Location{}, args))
}
