// SPDX-License-Identifier: Apache-2.0

// Package openrpc imports an OpenRPC document (the description format of
// JSON-RPC 2.0 services): each method becomes a request that POSTs the
// JSON-RPC envelope to the server, with the method's example parameters, and
// each example of a method becomes a pinned example of its response. The
// server of the document becomes the environment's base_url.
package openrpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/loose"
	"github.com/githugt/flowi/lib/internal/yamldoc"
	"github.com/githugt/flowi/lib/jsonrpc"
	"github.com/githugt/flowi/lib/model"
)

// Import reads an OpenRPC document (JSON or YAML). A file that does not parse
// is FLW-IMPORT-004; one that is not an OpenRPC document is FLW-IMPORT-002.
func Import(src []byte, file string) (importers.Result, diag.List) {
	root, err := yamldoc.Decode(src)
	if err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportSyntax, diag.Location{File: file}, map[string]string{"format": "OpenRPC", "detail": firstLine(err.Error())})}
	}
	methods := loose.List(root["methods"])
	if loose.Str(root["openrpc"]) == "" || len(methods) == 0 {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file},
			map[string]string{"detail": `an OpenRPC document has an "openrpc" version and a list of "methods"`})}
	}
	im := &importer{file: file}
	im.environment(root)
	for _, m := range methods {
		im.method(m)
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

// environment keeps the document's first server as base_url; without one,
// requests point at {{env.base_url}} for the person to fill in.
func (im *importer) environment(root map[string]any) {
	servers := loose.List(root["servers"])
	if len(servers) == 0 || loose.Str(servers[0]["url"]) == "" {
		return
	}
	it := importers.Item{Name: "environment", Source: im.at()}
	env := model.Environment{Meta: model.Meta{Name: "openrpc"}}
	env.Vars = append(env.Vars, importers.EnvVar(&it, "base_url", loose.Str(servers[0]["url"]), false))
	im.result.Environments = append(im.result.Environments, env)
	im.result.Items = append(im.result.Items, it)
}

func (im *importer) method(m map[string]any) {
	name := loose.Str(m["name"])
	it := importers.Item{Name: cmpOr(name, "method"), Source: im.at()}
	if name == "" {
		it.Skip("the method has no name")
		im.result.Items = append(im.result.Items, it)
		return
	}
	path := im.taken.Unique(cmpOr(importers.FileName(name), "method"))
	it.Name = path
	byName := loose.Str(m["paramStructure"]) == "by-name"
	params := paramsOf(loose.List(m["params"]), firstExampleValues(m), byName)
	call := jsonrpc.Call{Method: name, Params: params, ID: "1"}
	v := model.RequestVersion{
		Label: "1", Method: "POST", URL: "{{env.base_url}}",
		Headers: []model.KeyValue{{Key: "Content-Type", Value: "application/json"}},
		Body:    model.Body{JSON: jsonrpc.Envelope(call)},
		Docs:    docsOf(m),
	}
	im.result.Requests = append(im.result.Requests, model.Request{Meta: model.Meta{Name: name}, Path: path, Versions: []model.RequestVersion{v}})
	im.examples(path, m)
	im.result.Items = append(im.result.Items, it)
}

func docsOf(m map[string]any) string {
	return strings.TrimSpace(strings.Join(nonEmpty(loose.Str(m["summary"]), loose.Str(m["description"])), "\n\n"))
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return out
}

// paramsOf writes the parameters a request starts with: the schema's default
// or example, else the empty value of its type. By position, an array; by
// name, an object.
func paramsOf(params []map[string]any, given map[string]any, byName bool) string {
	if len(params) == 0 {
		return ""
	}
	value := func(p map[string]any) any {
		if v, ok := given[loose.Str(p["name"])]; ok {
			return v
		}
		return sample(loose.Map(p["schema"]))
	}
	if byName {
		obj := map[string]any{}
		for _, p := range params {
			obj[loose.Str(p["name"])] = value(p)
		}
		return marshal(obj)
	}
	list := make([]any, 0, len(params))
	for _, p := range params {
		list = append(list, value(p))
	}
	return marshal(list)
}

// firstExampleValues are the parameter values of the method's first example, by name.
func firstExampleValues(m map[string]any) map[string]any {
	examples := loose.List(m["examples"])
	if len(examples) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, p := range loose.List(examples[0]["params"]) {
		if v, ok := p["value"]; ok {
			out[loose.Str(p["name"])] = v
		}
	}
	return out
}

func marshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// sample is a value for a schema: its default, its first example, or the
// empty value of its type.
func sample(schema map[string]any) any {
	if v, ok := schema["default"]; ok {
		return v
	}
	if ex, _ := schema["examples"].([]any); len(ex) > 0 {
		return ex[0]
	}
	if v, ok := schema["example"]; ok {
		return v
	}
	switch loose.Str(schema["type"]) {
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	}
	return ""
}

// examples keeps each example of a method as a pinned response: the result in
// a JSON-RPC response envelope.
func (im *importer) examples(path string, m map[string]any) {
	for i, ex := range loose.List(m["examples"]) {
		result := loose.Map(ex["result"])
		if result == nil {
			continue
		}
		name := cmpOr(importers.FileName(loose.Str(ex["name"])), fmt.Sprintf("example-%d", i+1))
		body := map[string]any{"jsonrpc": jsonrpc.Version, "result": result["value"], "id": 1}
		im.result.Examples = append(im.result.Examples, importers.Example{RequestPath: path, Name: name, Version: "1", Status: 200,
			Headers: []importers.Header{{Name: "Content-Type", Value: "application/json"}}, Body: body})
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
