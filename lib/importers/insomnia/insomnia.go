// SPDX-License-Identifier: Apache-2.0

// Package insomnia imports an Insomnia export: the JSON export (format 4) and
// the YAML collection of Insomnia 10 and later. Folders become request
// folders, requests become requests (one version "1"), and environments
// become environments. Insomnia's {{ _.name }} becomes {{env.name}}; scripts,
// template tags and unsupported auth are reported, and credentials never
// written.
package insomnia

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/internal/yamldoc"
	"github.com/githugt/flowi-lib/model"
)

// Import reads an Insomnia export. A file that does not parse is
// FLW-IMPORT-004; one that is not an export is FLW-IMPORT-002.
func Import(src []byte, file string) (importers.Result, diag.List) {
	root, err := yamldoc.Decode(src)
	if err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportSyntax, diag.Location{File: file}, map[string]string{"format": "Insomnia", "detail": firstLine(err.Error())})}
	}
	im := &importer{file: file}
	switch {
	case root["_type"] == "export":
		im.exportV4(root)
	case strings.HasPrefix(str(root["type"]), "collection.insomnia.rest/"):
		im.collectionV5(root)
	default:
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file},
			map[string]string{"detail": `an Insomnia export has "_type": "export" (format 4) or "type": "collection.insomnia.rest/5.0"`})}
	}
	return im.result, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

type importer struct {
	file   string
	taken  importers.Names
	result importers.Result
}

func (im *importer) at() diag.Location { return diag.Location{File: im.file} }

// exportV4 reads {"_type": "export", "resources": [...]}: a flat list whose
// entries point at their folder (or environment) by parentId.
func (im *importer) exportV4(root map[string]any) {
	resources := list(root["resources"])
	byID := map[string]map[string]any{}
	for _, r := range resources {
		byID[str(r["_id"])] = r
	}
	workspace := firstOf(resources, "workspace")
	for _, r := range resources {
		switch str(r["_type"]) {
		case "request":
			im.request(r, folderPath(byID, str(r["parentId"])))
		case "grpc_request", "websocket_request":
			im.skip(str(r["name"]), "this kind of request is not imported ("+str(r["_type"])+")")
		}
	}
	im.environmentsV4(resources, str(workspace["name"]))
}

func firstOf(resources []map[string]any, typ string) map[string]any {
	for _, r := range resources {
		if str(r["_type"]) == typ {
			return r
		}
	}
	return nil
}

// folderPath is the names of the folders from the top down to parentID.
func folderPath(byID map[string]map[string]any, parentID string) []string {
	var out []string
	for depth := 0; parentID != "" && depth < 64; depth++ {
		p, ok := byID[parentID]
		if !ok || str(p["_type"]) != "request_group" {
			break
		}
		out = append(out, folderName(str(p["name"])))
		parentID = str(p["parentId"])
	}
	slices.Reverse(out)
	return out
}

func folderName(name string) string {
	if n := importers.FileName(name); n != "" {
		return n
	}
	return "folder"
}

// environmentsV4 imports the base environment (the one whose parent is the
// workspace) and each sub-environment with the base's variables under its
// own, because a Flowi environment stands alone.
func (im *importer) environmentsV4(resources []map[string]any, workspaceName string) {
	var base map[string]any
	var subs []map[string]any
	for _, r := range resources {
		if str(r["_type"]) != "environment" {
			continue
		}
		if p := str(r["parentId"]); strings.HasPrefix(p, "wrk_") || p == "" {
			base = r
		} else {
			subs = append(subs, r)
		}
	}
	im.environments(base, subs, workspaceName)
}

// collectionV5 reads Insomnia 10's YAML collection: nested items, where a
// folder has children.
func (im *importer) collectionV5(root map[string]any) {
	im.walkV5(list(root["collection"]), nil)
	env := asMap(root["environments"])
	var subs []map[string]any
	if env != nil {
		subs = list(env["subEnvironments"])
	}
	im.environments(env, subs, str(root["name"]))
}

func (im *importer) walkV5(items []map[string]any, folders []string) {
	for _, it := range items {
		if children, ok := it["children"]; ok {
			im.walkV5(list(children), append(slices.Clone(folders), folderName(str(it["name"]))))
			continue
		}
		if str(it["url"]) != "" || str(it["method"]) != "" {
			im.request(it, folders)
		}
	}
}

func (im *importer) skip(name, detail string) {
	it := importers.Item{Name: name, Source: im.at()}
	it.Skip(detail)
	im.result.Items = append(im.result.Items, it)
}

func (im *importer) environments(base map[string]any, subs []map[string]any, workspaceName string) {
	baseVars := dataOf(base)
	if len(baseVars) > 0 {
		im.addEnvironment(workspaceName, "base environment", nil, baseVars)
	}
	for _, s := range subs {
		im.addEnvironment(str(s["name"]), "environment "+str(s["name"]), baseVars, dataOf(s))
	}
}

func (im *importer) addEnvironment(name, label string, base, own []variable) {
	it := importers.Item{Name: label, Source: im.at()}
	env := model.Environment{Meta: model.Meta{Name: environmentName(name)}}
	seen := map[string]int{}
	for _, v := range slices.Concat(base, own) {
		if v.nested {
			continue
		}
		if i, ok := seen[v.name]; ok {
			env.Vars[i] = importers.EnvVar(&it, v.name, v.value, false)
			continue
		}
		seen[v.name] = len(env.Vars)
		env.Vars = append(env.Vars, importers.EnvVar(&it, v.name, v.value, false))
	}
	for _, name := range nestedNames(slices.Concat(base, own)) {
		it.Unmapped("variable "+name, "left out", "a nested object has no Flowi equivalent; flatten it into separate variables")
	}
	im.result.Environments = append(im.result.Environments, env)
	im.result.Items = append(im.result.Items, it)
}

func environmentName(name string) string {
	if n := importers.FileName(strings.ToLower(name)); n != "" {
		return n
	}
	return "insomnia"
}

type variable struct {
	name, value string
	nested      bool
}

// dataOf lists an environment's variables: scalars as text, objects marked.
func dataOf(env map[string]any) []variable {
	data := asMap(env["data"])
	var out []variable
	for _, k := range sortedKeys(data) {
		switch v := data[k].(type) {
		case map[string]any, []any:
			out = append(out, variable{name: k, nested: true})
		default:
			out = append(out, variable{name: k, value: str(v)})
		}
	}
	return out
}

func nestedNames(vars []variable) []string {
	var out []string
	for _, v := range vars {
		if v.nested {
			out = append(out, v.name)
		}
	}
	return slices.Compact(out)
}
