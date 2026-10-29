// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"maps"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func httpParams(method string) []Param {
	return []Param{
		{Name: "url", Kind: ParamText, Required: true},
		{Name: "method", Kind: ParamString, Doc: "Default " + method + "."},
		{Name: "headers", Kind: ParamMap},
		{Name: "query", Kind: ParamMap},
	}
}

func fetchSpec() Spec {
	return Spec{
		Type: "fetch", Group: "Call", Summary: "An ad-hoc HTTP call (no saved request, no hooks), optionally transformed.",
		Params: append(httpParams("GET"),
			Param{Name: "body", Kind: ParamAny, Doc: "A string is sent as text; a mapping or list as JSON."},
			Param{Name: "transform", Kind: ParamText, Doc: "Rendered with response in scope; becomes `nodes.<id>.value`."}),
		Run: runFetch,
	}
}

func runFetch(ctx context.Context, n model.Node, s *State) Outcome {
	call, out := buildCall(n, s, "GET", "body")
	if out.Diags.HasErrors() {
		return out
	}
	res := s.Runtime.HTTP(ctx, call)
	if res.Response == nil {
		return Outcome{Status: res.Status, Diags: res.Diags}
	}
	outputs := responseOutputs(res.Response)
	if raw, ok := param(n, "transform"); ok {
		v, err := eval(raw, withResponse(s, res.Response))
		if err != nil {
			return Outcome{Outputs: outputs, Diags: templateDiags(n, "transform", err)}
		}
		outputs["value"] = v
	}
	return Outcome{Outputs: outputs, Status: res.Status, Diags: res.Diags}
}

// buildCall renders an HTTP call's url, method, headers, query and the
// body parameter named bodyKey.
func buildCall(n model.Node, s *State, defaultMethod, bodyKey string) (HTTPCall, Outcome) {
	var diags diag.List
	render := func(key string) any {
		raw, ok := param(n, key)
		if !ok {
			return nil
		}
		v, err := eval(raw, s)
		if err != nil {
			diags = append(diags, templateDiags(n, key, err)...)
		}
		return v
	}
	call := HTTPCall{Method: sentMethod(stringParam(n, "method")), URL: template.String(render("url")),
		Headers: textMap(render("headers")), Query: textMap(render("query")), Body: render(bodyKey)}
	if call.Method == "" {
		call.Method = defaultMethod
	}
	return call, Outcome{Diags: diags}
}

// sentMethod is the method as it goes on the wire: a known method is sent in its registered spelling, so "query" works;
// any other name is a case-sensitive token and is sent as written.
func sentMethod(name string) string {
	if spelling, known := model.KnownMethod(name); known {
		return spelling
	}
	return name
}

func textMap(v any) map[string]string {
	m, _ := v.(map[string]any)
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, value := range m {
		out[k] = template.String(value)
	}
	return out
}

// responseOutputs are the outputs of an ad-hoc call: status, headers (by
// lower-case name), body and time.
func responseOutputs(r *Response) map[string]any {
	values := r.Values()
	return map[string]any{"status": r.Status, "headers": values["headers"], "body": r.Body, "time": r.Time}
}

// withResponse is s with the response in scope (response, status).
func withResponse(s *State, r *Response) *State {
	seeded := *s
	inner := s.Scope
	seeded.Scope = func() template.Scope {
		sc := inner()
		values := maps.Clone(sc.Values)
		values["response"], values["status"] = r.Values(), r.Status
		sc.Values = values
		return sc
	}
	return &seeded
}
