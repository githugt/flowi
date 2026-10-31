// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func setSpec() Spec {
	return Spec{
		Type: "set", Group: "Data",
		Summary: "Write variables. `values` is a list run in order, each `{name, from, value}` where `from` is `expr` (the default; `value` is a template), `literal` (`value` as written), `generate` (`value` uses generators such as `{{$uuid-v4}}`; optional `seed`) or `response` (post-hooks; `in` is `body`, `header`, `cookie` or `status`, with `path`, `header`, `cookie` or `regex`). Or `target` and `value` for one variable or, in pre-hooks, a request field.",
		Params: []Param{
			{Name: "target", Kind: ParamString, Doc: "Without `values`: `vars.<name>`; in pre-hooks also `request.method`, `request.url`, `request.headers.<Name>`, `request.query.<name>`, `request.body[.<path>]`, `cookies.<name>`."},
			{Name: "value", Kind: ParamAny, Doc: "Without `values`: what `target` is set to."},
		},
		Typed: []string{"values"},
		Run:   runSet,
	}
}

// runSet writes value to target: vars.<name>, or (before the request is
// sent) request.method, request.url, request.headers.<Name>,
// request.query.<name>, request.body, request.body.<dot.path>, or
// cookies.<name>.
func runSet(_ context.Context, n model.Node, s *State) Outcome {
	if len(n.Values) > 0 {
		return runValues(n, s)
	}
	target := stringParam(n, "target")
	raw, _ := param(n, "value")
	v, err := eval(raw, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "value", err)}
	}
	if detail := apply(target, v, s); detail != "" {
		return failed(n, diag.CodeRunSetTarget, map[string]string{"target": target, "detail": detail})
	}
	return Outcome{}
}

// apply writes v; it returns why it could not, or "".
func apply(target string, v any, s *State) string {
	if name, ok := strings.CutPrefix(target, "vars."); ok && name != "" {
		s.Vars.Set(name, v)
		return ""
	}
	if !strings.HasPrefix(target, "request.") && !strings.HasPrefix(target, "cookies.") {
		return "unknown target (use vars.<name>, request.<field> or cookies.<name>)"
	}
	if s.Request == nil {
		return requestGone(s.Phase)
	}
	s.Request.Mu.Lock()
	defer s.Request.Mu.Unlock()
	return applyRequest(target, v, s.Request)
}

func applyRequest(target string, v any, r *MutableRequest) string {
	text := template.String(v)
	rest, isRequest := strings.CutPrefix(target, "request.")
	switch {
	case !isRequest:
		name := strings.TrimPrefix(target, "cookies.")
		r.Cookies = setIn(r.Cookies, name, text)
	case rest == "method":
		r.Method = &text
	case rest == "url":
		r.URL = &text
	case rest == "body":
		r.Body = &v
	case strings.HasPrefix(rest, "headers."):
		r.setHeader(strings.TrimPrefix(rest, "headers."), text)
	case strings.HasPrefix(rest, "query."):
		r.Query = setIn(r.Query, strings.TrimPrefix(rest, "query."), text)
	case strings.HasPrefix(rest, "body."):
		if r.BodyPaths == nil {
			r.BodyPaths = map[string]any{}
		}
		r.BodyPaths[strings.TrimPrefix(rest, "body.")] = v
	default:
		return "unknown request field " + rest
	}
	return ""
}

func setIn(m map[string]string, k, v string) map[string]string {
	if m == nil {
		m = map[string]string{}
	}
	m[k] = v
	return m
}

func requestGone(p Phase) string {
	if p == PhaseFlow {
		return "flow nodes have no request (request.* and cookies.* work in pre-hooks)"
	}
	return "the request has already been sent"
}
