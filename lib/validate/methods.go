// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkMethods reports an HTTP method that cannot be sent (anything that is not a token), and a known method written
// in the wrong case. Any other token is allowed: a method of your own (PURGE, X-CUSTOM) is a normal thing to send.
// gRPC and async versions keep their method elsewhere, so they are not judged here.
func checkMethods(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Method == "" || v.GRPC != nil || v.Async != nil {
					continue
				}
				out = append(out, methodDiags(r, v)...)
			}
		}
	}
	return out
}

func methodDiags(r model.Request, v model.RequestVersion) diag.List {
	loc := diag.Location{File: r.File, Path: yamlpath.Key(yamlpath.Key("$", "versions"), v.Label) + ".method"}
	if !model.ValidMethod(v.Method) {
		return diag.List{diag.New(diag.CodeRequestMethod, loc, map[string]string{"method": v.Method})}
	}
	if spelling, known := model.KnownMethod(v.Method); known && spelling != v.Method {
		return diag.List{diag.New(diag.CodeRequestMethodCase, loc, map[string]string{"method": v.Method, "spelling": spelling})}
	}
	return nil
}
