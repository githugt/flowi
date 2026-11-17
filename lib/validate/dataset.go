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
				base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
				if v.Dataset != nil {
					out = append(out, datasetDiags(r.File, yamlpath.Key(base, "dataset"), v, ix.allowWrites)...)
				}
				if v.File != nil {
					out = append(out, fileDiags(r.File, yamlpath.Key(base, "file"), v)...)
				}
			}
		}
	}
	return out
}

// datasetDiags checks one dataset block. path is the path of the block itself.
func datasetDiags(file, path string, v model.RequestVersion, allowWrites bool) diag.List {
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
	if detail := queryProblem(d.Query, allowWrites || d.Writes); detail != "" {
		problem(yamlpath.Key(path, "query"), detail)
	}
	if d.Limit < 0 {
		problem(yamlpath.Key(path, "limit"), "limit must not be negative")
	}
	return out
}

// fileDiags checks one file block. path is the path of the block itself. The files are only read, so no query may write.
func fileDiags(file, path string, v model.RequestVersion) diag.List {
	var out diag.List
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeDatasetRequest, diag.Location{File: file, Path: p}, map[string]string{"detail": detail}))
	}
	f := *v.File
	if v.Dataset != nil || v.GRPC != nil || v.Async != nil {
		problem(path, "a version is one kind of request: a file query cannot also be a database, gRPC or async request")
	}
	if !slices.Contains(model.DataFileFormats(), f.Format) {
		problem(yamlpath.Key(path, "format"), "the format is one of "+strings.Join(model.DataFileFormats(), ", "))
	}
	if strings.TrimSpace(f.Path) == "" {
		problem(yamlpath.Key(path, "path"), "the path is empty: give a file or a folder of the collection")
	}
	if detail := queryProblem(f.Query, false); detail != "" {
		problem(yamlpath.Key(path, "query"), detail)
	}
	if f.Limit < 0 {
		problem(yamlpath.Key(path, "limit"), "limit must not be negative")
	}
	return out
}

// queryProblem is what is wrong with a query, "" when it is fine.
func queryProblem(query string, allowWrites bool) string {
	if templated(query) {
		// A value written into the statement could change what it means. Params are sent apart from it.
		return "the query is not templated: put values in params and refer to them as $1, $2 … (postgres, sqlite) or ?"
	}
	if allowWrites {
		return ""
	}
	if err := readonly.Check(query); err != nil {
		return err.Error() + ". To run statements that change data, turn on 'Allow this query to change data' on the request, or the setting datasets.allow_writes for all (Settings > Requests)"
	}
	return ""
}
