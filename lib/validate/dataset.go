// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/dataset/readonly"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkDataset reports what is wrong with the dataset versions of every request.
func checkDataset(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Dataset == nil {
					continue
				}
				base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
				out = append(out, datasetDiags(r.File, yamlpath.Key(base, "dataset"), v)...)
			}
		}
	}
	return out
}

// datasetDiags checks one dataset block. path is the path of the block itself.
func datasetDiags(file, path string, v model.RequestVersion) diag.List {
	var out diag.List
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeDatasetRequest, diag.Location{File: file, Path: p}, map[string]string{"detail": detail}))
	}
	d := *v.Dataset
	if v.GRPC != nil || v.Async != nil {
		problem(path, "a version is one kind of request: a dataset query cannot also be a gRPC call or an async request")
	}
	if !slices.Contains(model.DatasetDrivers(), d.Driver) {
		problem(yamlpath.Key(path, "driver"), "the driver is one of "+strings.Join(model.DatasetDrivers(), ", "))
	}
	if strings.TrimSpace(d.Server) == "" {
		problem(yamlpath.Key(path, "server"), "the server is empty: give a connection URL, or for sqlite the path of the file")
	}
	if detail := queryProblem(d.Query); detail != "" {
		problem(yamlpath.Key(path, "query"), detail)
	}
	if d.Limit < 0 {
		problem(yamlpath.Key(path, "limit"), "limit must not be negative")
	}
	return out
}

// queryProblem is what is wrong with a query, "" when it is fine.
func queryProblem(query string) string {
	if templated(query) {
		// A value written into the statement could change what it means. Params are sent apart from it.
		return "the query is not templated: put values in params and refer to them as $1, $2 … (postgres, sqlite) or ?"
	}
	if err := readonly.Check(query); err != nil {
		return err.Error()
	}
	return ""
}
