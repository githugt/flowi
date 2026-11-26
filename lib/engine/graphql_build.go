// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/graphqlx"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/template"
)

// graphQLAccept asks for the GraphQL-over-HTTP media type first and plain JSON second; every server answers JSON.
const graphQLAccept = "application/graphql-response+json, application/json;q=0.9"

// graphQLPayload is the request of the GraphQL-over-HTTP specification. Fields keep this order on the wire.
type graphQLPayload struct {
	Query         string          `json:"query"`
	OperationName string          `json:"operationName,omitempty"`
	Variables     json.RawMessage `json:"variables,omitempty"`
}

// graphQLRequest renders the GraphQL body of the version: the document as written (it is never templated, so a
// value cannot change what it means), and the variables, which are. ok is false when a diagnostic was added.
func (b *builder) graphQLRequest() (graphQLPayload, bool) {
	g := *b.in.version.Body.GraphQL
	if strings.Contains(g.Query, "{{") && !b.in.verbatim {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "the query of " + b.in.label + " contains {{ : a GraphQL document is sent as written, so put the value in variables and refer to it as $name"})
		return graphQLPayload{}, false
	}
	p := graphQLPayload{Query: g.Query, OperationName: g.Operation}
	if strings.TrimSpace(g.Variables) == "" {
		return p, true
	}
	r, err := template.RenderJSON(g.Variables, b.scope("graphql variables"))
	if err != nil {
		if b.in.verbatim {
			p.Variables = json.RawMessage(g.Variables) // a preview shows the text as written; it may not be JSON yet
			return p, true
		}
		b.templateError("graphql variables", err)
		return graphQLPayload{}, false
	}
	b.tainted["body"] = b.tainted["body"] || r.Tainted
	b.tainted["graphql variables"] = r.Tainted
	b.derived(r)
	text := r.Value.(string)
	if err := checkVariables(text); err != nil && !b.in.verbatim {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "the variables of " + b.in.label + " " + err.Error()})
		return graphQLPayload{}, false
	}
	p.Variables = json.RawMessage(text)
	return p, true
}

// checkVariables says why text is not a JSON object, which is all variables may be.
func checkVariables(text string) error {
	var v map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			return errors.New("are not valid JSON: " + err.Error())
		}
		return errors.New("must be a JSON object, such as {\"id\": 1}")
	}
	return nil
}

// isGET reports whether the version is sent with the GET method: its operation then travels in the URL.
func (b *builder) isGraphQLGET() bool {
	return b.in.version.Body.GraphQL != nil && strings.EqualFold(b.in.version.Method, http.MethodGet)
}

// graphQLBody is the JSON body of a GraphQL POST.
func (b *builder) graphQLBody() (httpx.BodySource, *Body) {
	if b.isGraphQLGET() {
		return nil, nil // a GET carries the operation in the URL (graphQLQuery)
	}
	if isSubscription(b.in.version) && !b.in.verbatim {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "a subscription is not sent as an HTTP request (" + b.in.label + ")"})
		return nil, nil
	}
	p, ok := b.graphQLRequest()
	if !ok {
		return nil, nil
	}
	text, err := marshalGraphQL(p)
	if err != nil {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "the operation of " + b.in.label + " cannot be sent: " + err.Error()})
		return nil, nil
	}
	return b.inMemory(text, "application/json")
}

// graphQLQuery is the query string of a GraphQL GET: query, operationName and variables (as JSON), after the
// pairs the version's query parameters make. Only a query may be sent this way: a GET must not change anything.
func (b *builder) graphQLQuery() []string {
	if !b.isGraphQLGET() {
		return nil
	}
	if op, err := graphqlx.Select(b.in.version.Body.GraphQL.Query, b.in.version.Body.GraphQL.Operation); err == nil && op.Kind != graphqlx.Query && !b.in.verbatim {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "a " + string(op.Kind) + " cannot be sent with GET (the GraphQL over HTTP specification forbids it): use POST for " + b.in.label})
		return nil
	}
	p, ok := b.graphQLRequest()
	if !ok {
		return nil
	}
	pairs := []string{"query=" + url.QueryEscape(p.Query)}
	if p.OperationName != "" {
		pairs = append(pairs, "operationName="+url.QueryEscape(p.OperationName))
	}
	if len(p.Variables) > 0 {
		compact := new(bytes.Buffer)
		if json.Compact(compact, p.Variables) != nil {
			compact.Reset()
			compact.Write(p.Variables)
		}
		pairs = append(pairs, "variables="+url.QueryEscape(compact.String()))
	}
	return pairs
}

// marshalGraphQL is the JSON of the payload, without escaping <, > and & (a query is full of them).
func marshalGraphQL(p graphQLPayload) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// addGraphQLHeaders sets the Accept header a GraphQL request has by default, unless the version names it. It runs
// after the default headers, whose Accept it replaces (Content-Type needs nothing: the JSON body has set it).
func (b *builder) addGraphQLHeaders(h http.Header, shown []Header) []Header {
	if b.in.version.Body.GraphQL == nil || b.namesHeader("Accept") {
		return shown
	}
	h.Set("Accept", graphQLAccept)
	for i := range shown {
		if strings.EqualFold(shown[i].Name, "Accept") {
			shown[i].Value = graphQLAccept
			return shown
		}
	}
	return append(shown, Header{Name: "Accept", Value: graphQLAccept})
}

func (b *builder) namesHeader(name string) bool {
	for _, kv := range b.in.version.Headers {
		if !kv.Disabled && strings.EqualFold(kv.Key, name) {
			return true
		}
	}
	return false
}
