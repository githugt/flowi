// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkExpect reports an expect: that is not a status, a range or a class, and one on a version that has no HTTP
// status to compare (a message, a gRPC call or a query).
func checkExpect(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Expect == "" {
					continue
				}
				path := yamlpath.Key(yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), "expect")
				problem := func(detail string) {
					out = append(out, diag.New(diag.CodeExpectInvalid, diag.Location{File: r.File, Path: path}, map[string]string{"detail": detail}))
				}
				if v.Async != nil || v.GRPC != nil || v.SQL() != nil {
					problem("only an HTTP request has a status to expect")
					continue
				}
				if _, err := model.ParseExpect(v.Expect); err != nil {
					problem(err.Error())
				}
			}
		}
	}
	return out
}
