// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkAuth reports auth blocks that can never work: an unknown scheme, a
// parameter the scheme does not have, a named authentication the collection
// does not have, or an auth block on a request that cannot use it (a message
// request other than WebSocket and SSE carries its credentials in its bindings,
// a gRPC call in its metadata). The
// collection's named authentications are checked the same way. Missing
// parameters are reported when the request is sent, because a value may come
// from a template.
func checkAuth(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		out = append(out, namedAuthProblems(c)...)
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				out = append(out, versionAuthProblems(c, r, v)...)
			}
		}
	}
	return out
}

// versionAuthProblems checks the auth block of one request version: a named authentication is looked up in c and
// checked as the block it stands for.
func versionAuthProblems(c model.Collection, r model.Request, v model.RequestVersion) diag.List {
	if v.Auth == nil {
		return nil
	}
	loc := diag.Location{File: r.File, Path: yamlpath.Key(yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), "auth")}
	if v.Auth.Use != "" {
		if v.Auth.Type != "" || len(v.Auth.Params) > 0 {
			return diag.List{diag.New(diag.CodeAuthInvalid, loc, map[string]string{"detail": "an auth block that uses a named authentication has no type or parameters of its own"})}
		}
		named, ok := c.NamedAuth(v.Auth.Use)
		if !ok {
			return diag.List{diag.New(diag.CodeAuthUnknown, loc, map[string]string{"name": v.Auth.Use})}
		}
		v.Auth = &named.Auth
	}
	var out diag.List
	for _, detail := range authProblems(v) {
		out = append(out, diag.New(diag.CodeAuthInvalid, loc, map[string]string{"detail": detail}))
	}
	return out
}

// namedAuthProblems checks each named authentication of c as an auth block of
// an HTTP request, and that no two share a name: requests use them by name.
func namedAuthProblems(c model.Collection) diag.List {
	var out diag.List
	seen := map[string]bool{}
	for _, a := range c.Auths {
		loc := diag.Location{File: a.File, Path: yamlpath.Key("$", "auth")}
		problems := authProblems(model.RequestVersion{Auth: &a.Auth})
		if a.Auth.Use != "" {
			problems = []string{"a named authentication cannot use another one"}
		}
		if seen[a.Name] {
			problems = append(problems, fmt.Sprintf("another authentication is also named %q", a.Name))
		}
		seen[a.Name] = true
		for _, detail := range problems {
			out = append(out, diag.New(diag.CodeAuthInvalid, loc, map[string]string{"detail": detail}))
		}
	}
	return out
}

func authProblems(v model.RequestVersion) []string {
	a := v.Auth
	var out []string
	if v.Async != nil && len(model.AsyncAuthTypes(v.Async.Protocol)) == 0 {
		return []string{"only an HTTP, gRPC, WebSocket or SSE request has an auth block; this message request sets its credentials in its binding"}
	}
	if v.GRPC != nil && v.Auth.Type != model.AuthMutualTLS {
		return []string{"a gRPC call sets its credentials in its metadata; its auth block can only be mtls (a client certificate)"}
	}
	if !slices.Contains(model.AuthTypes(), a.Type) {
		return []string{fmt.Sprintf("type %q is not one of %s", a.Type, strings.Join(model.AuthTypes(), ", "))}
	}
	if allowed := asyncAuthTypes(v); allowed != nil && !slices.Contains(allowed, a.Type) {
		return []string{fmt.Sprintf("a %s request can use %s in its auth block, not %q: a signature or a challenge needs a plain HTTP request", v.Async.Protocol, strings.Join(allowed, ", "), a.Type)}
	}
	known := model.AuthParams(a.Type)
	for _, kv := range a.Params {
		if !slices.Contains(known, kv.Key) {
			out = append(out, fmt.Sprintf("%s has no parameter %q (it has %s)", a.Type, kv.Key, strings.Join(known, ", ")))
		}
	}
	return out
}

// asyncAuthTypes are the schemes the auth block of v can have when v is a message request; nil for any other request.
func asyncAuthTypes(v model.RequestVersion) []string {
	if v.Async == nil {
		return nil
	}
	return model.AsyncAuthTypes(v.Async.Protocol)
}
