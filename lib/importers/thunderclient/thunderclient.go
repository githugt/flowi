// SPDX-License-Identifier: Apache-2.0

// Package thunderclient imports a Thunder Client export: a collection
// (folders, requests, auth, bodies, response tests) or an environment file.
// Thunder Client's {{name}} becomes {{env.name}}; credentials are never
// written, and what has no Flowi equivalent is reported.
package thunderclient

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/internal/loose"
	"github.com/githugt/flowi-lib/internal/yamldoc"
	"github.com/githugt/flowi-lib/model"
)

// Import reads a Thunder Client collection or environment. A file that does
// not parse is FLW-IMPORT-004; one that is neither is FLW-IMPORT-002.
func Import(src []byte, file string) (importers.Result, diag.List) {
	where := diag.Location{File: file}
	root, err := yamldoc.Decode(src)
	if err != nil {
		first, _, _ := strings.Cut(err.Error(), "\n")
		return importers.Result{}, diag.List{diag.New(diag.CodeImportSyntax, where, map[string]string{"format": "Thunder Client", "detail": first})}
	}
	im := &importer{file: file}
	switch {
	case root["requests"] != nil && (root["clientName"] != nil || root["collectionName"] != nil || root["folders"] != nil):
		im.collection(root)
	case root["data"] != nil && root["name"] != nil:
		im.environment(root)
	default:
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, where,
			map[string]string{"detail": "a Thunder Client export has a list of requests (a collection) or a name and data (an environment)"})}
	}
	return im.result, nil
}

type importer struct {
	file   string
	taken  importers.Names
	result importers.Result
}

func (im *importer) at() diag.Location { return diag.Location{File: im.file} }

// collection imports the requests under their folders; Thunder Client keeps
// folders flat, each pointing at its parent by containerId.
func (im *importer) collection(root map[string]any) {
	folders := map[string]map[string]any{}
	for _, f := range loose.List(root["folders"]) {
		folders[loose.Str(f["_id"])] = f
	}
	requests := loose.List(root["requests"])
	slices.SortStableFunc(requests, func(a, b map[string]any) int { return sortNum(a) - sortNum(b) })
	for _, r := range requests {
		im.request(r, folderPath(folders, loose.Str(r["containerId"])))
	}
}

func sortNum(m map[string]any) int {
	n, _ := m["sortNum"].(float64)
	return int(n)
}

func folderPath(folders map[string]map[string]any, id string) []string {
	var out []string
	for depth := 0; id != "" && depth < 64; depth++ {
		f, ok := folders[id]
		if !ok {
			break
		}
		out = append(out, fileSegment(loose.Str(f["name"]), "folder"))
		id = loose.Str(f["containerId"])
	}
	slices.Reverse(out)
	return out
}

func fileSegment(name, fallback string) string {
	if n := importers.FileName(name); n != "" {
		return n
	}
	return fallback
}

// environment imports a Thunder Client environment: a list of name/value.
func (im *importer) environment(root map[string]any) {
	it := importers.Item{Name: "environment " + loose.Str(root["name"]), Source: im.at()}
	env := model.Environment{Meta: model.Meta{Name: fileSegment(strings.ToLower(loose.Str(root["name"])), "thunder-client")}}
	for _, v := range loose.List(root["data"]) {
		if name := loose.Str(v["name"]); name != "" {
			env.Vars = append(env.Vars, importers.EnvVar(&it, name, loose.Str(v["value"]), false))
		}
	}
	im.result.Environments = append(im.result.Environments, env)
	im.result.Items = append(im.result.Items, it)
}
