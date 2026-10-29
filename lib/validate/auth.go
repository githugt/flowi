// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkAuth reports auth blocks that can never work: an unknown scheme, a
// parameter the scheme does not have, or an auth block on a request that is not
// HTTP (messages and gRPC calls carry their credentials in their bindings and
// metadata). Missing parameters are reported when the request is sent, because
// a value may come from a template.
func checkAuth(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Auth == nil {
					continue
				}
				path := yamlpath.Key(yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), "auth")
				for _, detail := range authProblems(v) {
					out = append(out, diag.New(diag.CodeAuthInvalid, diag.Location{File: r.File, Path: path}, map[string]string{"detail": detail}))
				}
			}
		}
	}
	return out
}

func authProblems(v model.RequestVersion) []string {
	a := v.Auth
	var out []string
	if v.Async != nil {
		return []string{"only an HTTP or gRPC request has an auth block; a message request sets its credentials in its binding"}
	}
	if v.GRPC != nil && v.Auth.Type != model.AuthMutualTLS {
		return []string{"a gRPC call sets its credentials in its metadata; its auth block can only be mtls (a client certificate)"}
	}
	if !slices.Contains(model.AuthTypes(), a.Type) {
		return []string{fmt.Sprintf("type %q is not one of %s", a.Type, strings.Join(model.AuthTypes(), ", "))}
	}
	known := model.AuthParams(a.Type)
	for _, kv := range a.Params {
		if !slices.Contains(known, kv.Key) {
			out = append(out, fmt.Sprintf("%s has no parameter %q (it has %s)", a.Type, kv.Key, strings.Join(known, ", ")))
		}
	}
	return out
}
