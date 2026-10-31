// SPDX-License-Identifier: Apache-2.0

// Package hoppscotch imports a Hoppscotch export: a collection file (one
// collection or a list, with nested folders) or an environment file.
// Hoppscotch's <<name>> becomes {{env.name}}; headers and auth set on a
// collection or folder are applied to the requests under it; scripts and
// unsupported auth are reported, and credentials are never written.
package hoppscotch

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/loose"
	"github.com/githugt/flowi/lib/model"
)

// Import reads a Hoppscotch collection or environment export. A file that
// does not parse is FLW-IMPORT-004; one that is neither is FLW-IMPORT-002.
func Import(src []byte, file string) (importers.Result, diag.List) {
	where := diag.Location{File: file}
	var raw any
	if err := json.Unmarshal(src, &raw); err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportSyntax, where, map[string]string{"format": "Hoppscotch", "detail": err.Error()})}
	}
	roots := roots(raw)
	im := &importer{file: file}
	imported := false
	for _, root := range roots {
		switch {
		case root["variables"] != nil && root["requests"] == nil && root["folders"] == nil:
			im.environment(root)
			imported = true
		case root["requests"] != nil || root["folders"] != nil:
			im.collection(root, nil, inherited{})
			imported = true
		}
	}
	if !imported {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, where,
			map[string]string{"detail": "a Hoppscotch export is a collection (requests and folders) or an environment (name and variables), or a list of them"})}
	}
	return im.result, nil
}

// roots is the export as a list of objects: one object or an array of them.
func roots(raw any) []map[string]any {
	if m, ok := raw.(map[string]any); ok {
		return []map[string]any{m}
	}
	items, _ := raw.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

type importer struct {
	file   string
	taken  importers.Names
	result importers.Result
}

func (im *importer) at() diag.Location { return diag.Location{File: im.file} }

// inherited is what a collection or folder hands down to its requests.
type inherited struct {
	headers []map[string]any
	auth    map[string]any
}

func (in inherited) with(node map[string]any) inherited {
	out := inherited{headers: slices.Concat(in.headers, loose.List(node["headers"])), auth: in.auth}
	if a := loose.Map(node["auth"]); a != nil && loose.Str(a["authType"]) != "inherit" && loose.Str(a["authType"]) != "" {
		out.auth = a
	}
	return out
}

// collection imports a collection's requests and then its folders.
func (im *importer) collection(node map[string]any, folders []string, parent inherited) {
	in := parent.with(node)
	for _, r := range loose.List(node["requests"]) {
		im.request(r, folders, in)
	}
	for _, f := range loose.List(node["folders"]) {
		im.collection(f, append(slices.Clone(folders), fileSegment(loose.Str(f["name"]), "folder")), in)
	}
}

func fileSegment(name, fallback string) string {
	if n := importers.FileName(name); n != "" {
		return n
	}
	return fallback
}

// environment imports an environment: a name and a list of variables, some
// marked secret (their value, if exported, is not written).
func (im *importer) environment(root map[string]any) {
	name := loose.Str(root["name"])
	it := importers.Item{Name: "environment " + name, Source: im.at()}
	env := model.Environment{Meta: model.Meta{Name: fileSegment(strings.ToLower(name), "hoppscotch")}}
	for _, v := range loose.List(root["variables"]) {
		key := loose.Str(v["key"])
		if key == "" {
			continue
		}
		env.Vars = append(env.Vars, importers.EnvVar(&it, key, loose.Str(v["value"]), loose.Flag(v["secret"])))
	}
	im.result.Environments = append(im.result.Environments, env)
	im.result.Items = append(im.result.Items, it)
}
