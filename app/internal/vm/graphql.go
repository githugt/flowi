// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/githugt/flowi/lib/graphqlx"
	"github.com/githugt/flowi/lib/model"
)

// GraphQLVerb is what a GraphQL request shows where an HTTP request shows its method.
const GraphQLVerb = "GQL"

// GraphQL is a copy of the selected version's GraphQL body; the zero value for any other body.
func (r *Request) GraphQL() model.GraphQL {
	if g := r.Current().Body.GraphQL; g != nil {
		return *g
	}
	return model.GraphQL{}
}

// IsGraphQL reports whether the selected version sends a GraphQL operation.
func (r *Request) IsGraphQL() bool { return r.Current().Body.GraphQL != nil }

// GraphQLField names a part of a GraphQL body the editor changes.
type GraphQLField string

// The parts of a GraphQL body.
const (
	GraphQLQuery     GraphQLField = "query"
	GraphQLVariables GraphQLField = "variables"
	GraphQLOperation GraphQLField = "operation"
)

// SetGraphQL changes one part of the selected version's GraphQL body; it does nothing for another body kind.
func (r *Request) SetGraphQL(field GraphQLField, text string) {
	if !r.IsGraphQL() {
		return
	}
	r.Edit(func(v *model.RequestVersion) {
		g := *v.Body.GraphQL // a copy: the version this edit started from must not change
		switch field {
		case GraphQLQuery:
			g.Query = text
		case GraphQLVariables:
			g.Variables = text
		case GraphQLOperation:
			g.Operation = strings.TrimSpace(text)
		}
		v.Body.GraphQL = &g
	})
}

// SetGraphQLReceive sets what a subscription waits for: how many events, for how long, which ones count. nil goes
// back to the default (the first event, within 30 seconds). It does nothing for another body kind.
func (r *Request) SetGraphQLReceive(rc *model.AsyncReceive) {
	if !r.IsGraphQL() {
		return
	}
	r.Edit(func(v *model.RequestVersion) {
		g := *v.Body.GraphQL
		if rc != nil {
			c := *rc
			rc = &c
		}
		g.Receive = rc
		v.Body.GraphQL = &g
	})
}

// GraphQLOp is one operation of the query the editor lists.
type GraphQLOp struct {
	Kind string `json:"kind"` // query, mutation or subscription
	Name string `json:"name"` // empty for an anonymous operation
}

// GraphQLInfo is what the editor shows about the selected GraphQL body besides its text.
type GraphQLInfo struct {
	// Operations are the operations the query defines, in order.
	Operations []GraphQLOp `json:"operations"`
	// Kind is the kind of the operation that will run ("" when it cannot be told: no operation, or several and none
	// chosen). It is what the send button and the sidebar call the request.
	Kind string `json:"kind"`
	// Problem says in a sentence why the body cannot be sent as it is ("" when it can).
	Problem string `json:"problem"`
	// VariablesProblem says why the variables are not a JSON object ("" when they are, or hold a template).
	VariablesProblem string `json:"variablesProblem"`
	// GetOK reports whether the operation may be sent with GET: only a query may.
	GetOK bool `json:"getOK"`
}

// GraphQLInfo reads the selected version's GraphQL body; nil for any other body.
func (r *Request) GraphQLInfo() *GraphQLInfo {
	g := r.Current().Body.GraphQL
	if g == nil {
		return nil
	}
	return graphQLInfoOf(*g)
}

func graphQLInfoOf(g model.GraphQL) *GraphQLInfo {
	info := &GraphQLInfo{Operations: []GraphQLOp{}}
	for _, op := range graphqlx.Operations(g.Query) {
		info.Operations = append(info.Operations, GraphQLOp{Kind: string(op.Kind), Name: op.Name})
	}
	switch {
	case strings.TrimSpace(g.Query) == "":
		info.Problem = "Write a query, mutation or subscription."
	case strings.Contains(g.Query, "{{"):
		info.Problem = "The query is sent as written: put a value in the variables and use $name."
	default:
		op, err := graphqlx.Select(g.Query, g.Operation)
		switch {
		case err == nil:
			info.Kind = string(op.Kind)
		case errors.Is(err, graphqlx.ErrAmbiguous):
			info.Problem = "The query has several operations: choose the one to run."
		default:
			info.Problem = capital(err.Error()) + "."
		}
	}
	info.GetOK = info.Kind == string(graphqlx.Query)
	info.VariablesProblem = variablesProblem(g.Variables)
	return info
}

// variablesProblem says why text is not a JSON object; "" when it is, is empty, or holds a template (which is
// only JSON once rendered).
func variablesProblem(text string) string {
	if strings.TrimSpace(text) == "" || strings.Contains(text, "{{") {
		return ""
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return "Not valid JSON: " + err.Error() + "."
		}
		return "The variables are a JSON object, such as {\"id\": 1}."
	}
	return ""
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// graphQLFacts is the kind and the name of the operation a GraphQL version runs; empty when they cannot be told.
func graphQLFacts(v model.RequestVersion) (kind, target string) {
	g := v.Body.GraphQL
	if op, err := graphqlx.Select(g.Query, g.Operation); err == nil {
		kind, target = string(op.Kind), op.Name
	}
	return kind, target
}

// cloneGraphQL copies a GraphQL body, so an edit never reaches the copy it came from.
func cloneGraphQL(g *model.GraphQL) *model.GraphQL {
	if g == nil {
		return nil
	}
	out := *g
	if g.Receive != nil {
		rc := *g.Receive
		out.Receive = &rc
	}
	return &out
}

// graphQLPayloadJSON is the JSON body a GraphQL operation is sent as, for a request that stops being GraphQL.
func graphQLPayloadJSON(g model.GraphQL) string {
	payload := struct {
		Query         string          `json:"query"`
		OperationName string          `json:"operationName,omitempty"`
		Variables     json.RawMessage `json:"variables,omitempty"`
	}{Query: g.Query, OperationName: g.Operation}
	if strings.TrimSpace(g.Variables) != "" && json.Valid([]byte(g.Variables)) {
		payload.Variables = json.RawMessage(g.Variables)
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	return string(out)
}
