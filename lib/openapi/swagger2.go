// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/internal/yamldoc"
)

// isSwagger2 reports whether data is a Swagger 2.0 document.
func isSwagger2(data []byte) bool {
	root, err := yamldoc.Decode(data)
	if err != nil {
		return false
	}
	v, _ := root["swagger"].(string)
	return strings.HasPrefix(v, "2.")
}

// upgradeSwagger2 turns a Swagger 2.0 document into the OpenAPI 3.0 document
// with the same operations, so one importer serves both. It carries what the
// importer reads: servers (from schemes, host and basePath), parameters, the
// request body (body and formData parameters), response schemas, the
// definitions and the security schemes. Vendor extensions and OAuth flows are
// dropped.
func upgradeSwagger2(data []byte) ([]byte, error) {
	root, err := yamldoc.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("reading the Swagger document: %w", err)
	}
	out := map[string]any{"openapi": "3.0.3", "info": root["info"], "paths": map[string]any{}}
	if servers := swaggerServers(root); len(servers) > 0 {
		out["servers"] = servers
	}
	components := map[string]any{}
	if defs, ok := root["definitions"].(map[string]any); ok {
		components["schemas"] = defs
	}
	if sec := swaggerSecurity(root); len(sec) > 0 {
		components["securitySchemes"] = sec
	}
	if len(components) > 0 {
		out["components"] = components
	}
	paths, _ := root["paths"].(map[string]any)
	consumes, produces := stringList(root["consumes"]), stringList(root["produces"])
	converted := map[string]any{}
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		item, _ := paths[p].(map[string]any)
		converted[p] = swaggerPath(item, consumes, produces)
	}
	out["paths"] = converted
	text, err := json.Marshal(rewriteRefs(out))
	if err != nil {
		return nil, fmt.Errorf("writing the converted document: %w", err)
	}
	return text, nil
}

func swaggerServers(root map[string]any) []any {
	host, _ := root["host"].(string)
	if host == "" {
		return nil
	}
	base, _ := root["basePath"].(string)
	schemes := stringList(root["schemes"])
	if len(schemes) == 0 {
		schemes = []string{"https"}
	}
	var out []any
	for _, s := range schemes {
		out = append(out, map[string]any{"url": s + "://" + host + strings.TrimRight(base, "/")})
	}
	return out
}

func swaggerSecurity(root map[string]any) map[string]any {
	defs, _ := root["securityDefinitions"].(map[string]any)
	out := map[string]any{}
	for name, v := range defs {
		d, _ := v.(map[string]any)
		switch d["type"] {
		case "basic":
			out[name] = map[string]any{"type": "http", "scheme": "basic"}
		case "apiKey":
			out[name] = map[string]any{"type": "apiKey", "in": d["in"], "name": d["name"]}
		}
	}
	return out
}

// swaggerPath converts one path item: its own parameters apply to every
// operation, as in OpenAPI 3.
func swaggerPath(item map[string]any, consumes, produces []string) map[string]any {
	out := map[string]any{}
	if params, ok := item["parameters"].([]any); ok {
		out["parameters"] = nonBodyParams(params)
	}
	for _, m := range methods() {
		op, ok := item[m].(map[string]any)
		if !ok {
			continue
		}
		out[m] = swaggerOperation(op, item, firstNonEmpty(stringList(op["consumes"]), consumes), firstNonEmpty(stringList(op["produces"]), produces))
	}
	return out
}

func firstNonEmpty(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

func swaggerOperation(op, item map[string]any, consumes, produces []string) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"operationId", "summary", "description", "tags", "deprecated"} {
		if v, ok := op[k]; ok {
			out[k] = v
		}
	}
	params, _ := op["parameters"].([]any)
	inherited, _ := item["parameters"].([]any)
	all := slices.Concat(inherited, params)
	if converted := nonBodyParams(params); len(converted) > 0 {
		out["parameters"] = converted
	}
	if body := swaggerBody(all, consumes); body != nil {
		out["requestBody"] = body
	}
	responses := map[string]any{}
	rs, _ := op["responses"].(map[string]any)
	for code, v := range rs {
		r, _ := v.(map[string]any)
		resp := map[string]any{"description": r["description"]}
		if resp["description"] == nil {
			resp["description"] = ""
		}
		if schema, ok := r["schema"]; ok {
			resp["content"] = contentOf(schema, produces, "application/json")
		}
		responses[code] = resp
	}
	out["responses"] = responses
	return out
}

// nonBodyParams are the path, query and header parameters, each with its
// type moved under schema.
func nonBodyParams(params []any) []any {
	var out []any
	for _, p := range params {
		pm, _ := p.(map[string]any)
		if _, isRef := pm["$ref"]; isRef {
			out = append(out, pm)
			continue
		}
		if in := pm["in"]; in == "body" || in == "formData" {
			continue
		}
		conv := map[string]any{}
		schema := map[string]any{}
		for k, v := range pm {
			switch k {
			case "name", "in", "required", "description":
				conv[k] = v
			case "type", "format", "enum", "default", "items", "minimum", "maximum":
				schema[k] = v
			}
		}
		conv["schema"] = schema
		out = append(out, conv)
	}
	return out
}

// swaggerBody builds the request body from a body parameter, or from the
// formData parameters as a form schema.
func swaggerBody(params []any, consumes []string) map[string]any {
	props := map[string]any{}
	var required []any
	multipart := false
	for _, p := range params {
		pm, _ := p.(map[string]any)
		switch pm["in"] {
		case "body":
			return map[string]any{"content": contentOf(pm["schema"], consumes, "application/json")}
		case "formData":
			name, _ := pm["name"].(string)
			schema := map[string]any{}
			for _, k := range []string{"type", "format", "enum", "default", "items"} {
				if v, ok := pm[k]; ok {
					schema[k] = v
				}
			}
			if schema["type"] == "file" {
				schema["type"], schema["format"] = "string", "binary"
				multipart = true
			}
			props[name] = schema
			if req, _ := pm["required"].(bool); req {
				required = append(required, name)
			}
		}
	}
	if len(props) == 0 {
		return nil
	}
	mime := "application/x-www-form-urlencoded"
	if multipart || slices.Contains(consumes, "multipart/form-data") && !slices.Contains(consumes, mime) {
		mime = "multipart/form-data"
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]any{"content": map[string]any{mime: map[string]any{"schema": schema}}}
}

// contentOf is the media types an operation accepts or returns, each with the
// schema (the first one, or fallback, when none are declared).
func contentOf(schema any, mimes []string, fallback string) map[string]any {
	if len(mimes) == 0 {
		mimes = []string{fallback}
	}
	out := map[string]any{}
	for _, m := range mimes {
		out[m] = map[string]any{"schema": schema}
	}
	return out
}

// rewriteRefs points #/definitions/X at #/components/schemas/X everywhere.
func rewriteRefs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			if s, ok := item.(string); ok && k == "$ref" {
				out[k] = strings.Replace(s, "#/definitions/", "#/components/schemas/", 1)
				continue
			}
			out[k] = rewriteRefs(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = rewriteRefs(item)
		}
		return out
	}
	return v
}
