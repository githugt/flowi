// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/graphqlx"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkGraphQL reports what is wrong with the GraphQL bodies of every request.
func checkGraphQL(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Body.GraphQL == nil {
					continue
				}
				base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
				out = append(out, graphQLDiags(r.File, base, v)...)
			}
		}
	}
	return out
}

// graphQLDiags checks one GraphQL body. base is the path of the version.
func graphQLDiags(file, base string, v model.RequestVersion) diag.List {
	var out diag.List
	path := yamlpath.Key(yamlpath.Key(base, "body"), "graphql")
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeGraphQLRequest, diag.Location{File: file, Path: p}, map[string]string{"detail": detail}))
	}
	g := *v.Body.GraphQL
	if v.GRPC != nil || v.Async != nil || v.Dataset != nil || v.File != nil {
		problem(path, "a version is one kind of request: a GraphQL body belongs to an HTTP request")
		return out
	}
	opKind, field, detail := graphQLOperation(g)
	if detail != "" {
		problem(yamlpath.Key(path, field), detail)
	}
	if r := g.Receive; r != nil {
		recv := yamlpath.Key(path, "receive")
		switch {
		case opKind != "" && opKind != graphqlx.Subscription:
			problem(recv, "receive says how many events a subscription waits for; a "+string(opKind)+" has one answer")
		case r.Count < 0:
			problem(yamlpath.Key(recv, "count"), "count must not be negative")
		case r.Timeout < 0:
			problem(yamlpath.Key(recv, "timeout"), "timeout must not be negative")
		}
	}
	if opKind == graphqlx.Subscription && v.Auth != nil {
		problem(yamlpath.Key(base, "auth"), "a subscription cannot use an auth block yet: put the credential in a header (it is sent with the WebSocket handshake)")
	}
	if strings.Contains(g.Query, "{{") {
		problem(yamlpath.Key(path, "query"), "the query contains {{ : a GraphQL document is sent as written, so put the value in variables and refer to it as $name")
	}
	if detail := variablesProblem(g.Variables); detail != "" {
		problem(yamlpath.Key(path, "variables"), detail)
	}
	method := strings.ToUpper(v.Method)
	switch method {
	case "", http.MethodPost:
	case http.MethodGet:
		if opKind == graphqlx.Mutation || opKind == graphqlx.Subscription {
			problem(yamlpath.Key(base, "method"), "a "+string(opKind)+" cannot be sent with GET: use POST")
		}
	default:
		problem(yamlpath.Key(base, "method"), "a GraphQL operation is sent with POST (or GET for a query), not "+method)
	}
	return out
}

// graphQLOperation finds the operation the body selects. When it cannot, detail says why and field is the field
// at fault: the query, or the operation name that matches none.
func graphQLOperation(g model.GraphQL) (kind graphqlx.Kind, field, detail string) {
	if strings.TrimSpace(g.Query) == "" {
		return "", "query", "the query is empty: write a query, mutation or subscription"
	}
	op, err := graphqlx.Select(g.Query, g.Operation)
	switch {
	case err == nil:
		return op.Kind, "", ""
	case errors.Is(err, graphqlx.ErrAmbiguous):
		return "", "query", "the document defines several operations: name the one to run in operation"
	case errors.Is(err, graphqlx.ErrNoOperation):
		return "", "query", err.Error()
	}
	return "", "operation", err.Error()
}

// variablesProblem says why variables are not a JSON object; "" when they are, or when they hold a template whose
// value is only known when the request is sent.
func variablesProblem(text string) string {
	if strings.TrimSpace(text) == "" || strings.Contains(text, "{{") {
		return ""
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			return "the variables are not valid JSON: " + err.Error()
		}
		return "the variables must be a JSON object, such as {\"id\": 1}"
	}
	return ""
}
