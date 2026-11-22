// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkStatuses reports a status definition in flowi.yaml that cannot be used (FLW-STATUS-002) and a request, flow or
// folder whose `status:` names a status the collection does not have (FLW-STATUS-001).
func checkStatuses(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		out = append(out, checkStatusDefinitions(c)...)
		known := statusNames(c)
		unknown := func(file, where, status string) {
			if status == "" || known[status] {
				return
			}
			out = append(out, diag.New(diag.CodeStatusUnknown, diag.Location{File: file, Path: "$.status"}, map[string]string{
				"status": status, "where": where, "known": knownList(c),
			}))
		}
		for _, r := range c.Requests {
			unknown(r.File, "request "+r.Path, r.Status)
		}
		for _, f := range c.Flows {
			unknown(f.File, "flow "+f.Path, f.Status)
		}
		for _, f := range c.Folders {
			unknown(f.File, "folder "+f.Dir, f.Status)
		}
	}
	return out
}

// checkStatusDefinitions checks each status of the manifest, and that none is defined twice (the second would be
// ignored, which is not what its author meant).
func checkStatusDefinitions(c model.Collection) diag.List {
	var out diag.List
	seen := map[string]bool{}
	for i, s := range c.Manifest.Statuses {
		loc := diag.Location{File: c.Manifest.File, Path: yamlpath.Index("$.statuses", i)}
		if err := model.CheckStatus(s); err != nil {
			out = append(out, diag.New(diag.CodeStatusInvalid, loc, map[string]string{"name": s.Name, "detail": err.Error()}))
		} else if seen[s.Name] {
			out = append(out, diag.New(diag.CodeStatusInvalid, loc, map[string]string{"name": s.Name, "detail": "it is defined twice, and only the first counts"}))
		}
		seen[s.Name] = true
	}
	return out
}

func statusNames(c model.Collection) map[string]bool {
	out := map[string]bool{}
	for _, s := range c.Manifest.StatusList() {
		out[s.Name] = true
	}
	return out
}

func knownList(c model.Collection) string {
	var names []string
	for _, s := range c.Manifest.StatusList() {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}
