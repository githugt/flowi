// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/formats"
	"github.com/githugt/flowi/lib/internal/jsonschema"
	"github.com/githugt/flowi/lib/internal/textenc"
	"github.com/githugt/flowi/lib/internal/yamldoc"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/version"
)

// Options configure Import.
type Options struct {
	NewID func() model.ID // ids for new requests and flows
	// Files is where references to other files ("$ref: ./schemas/pet.yaml")
	// are read from, rooted at the folder of the document. Nil: they are
	// reported and left out.
	Files fs.FS
}

// Result is what an OpenAPI document becomes.
type Result struct {
	Requests []model.Request // Path is set (relative to requests/); File is not
	// Schemas holds response schemas: request path → version label →
	// status code → JSON schema with references inlined.
	Schemas map[string]map[string]map[string]any
	Flows   []model.Flow // suggested from links; Path is relative to flows/
	// Files are the files the document's references were followed into: path
	// (relative to the folder of the document, slash-separated) → content. They
	// are what a later import of a kept copy needs beside it.
	Files map[string][]byte
}

// methods in the order operations are read.
func methods() []string {
	return []string{"get", "put", "post", "delete", "options", "head", "patch", "trace", "query"}
}

// verb is the HTTP method as sent: the fixed fields of a path item are written in lower case, so they are upper-cased;
// a name from additionalOperations is a case-sensitive token and is kept as written.
func (o operation) verb() string {
	if slices.Contains(methods(), o.method) {
		return strings.ToUpper(o.method)
	}
	return o.method
}

var versionPrefix = regexp.MustCompile(`^/v(\d+)(/.*)?$`)

// operation is one OpenAPI operation with what grouping needs.
type operation struct {
	method, path, stripped, label string
	op                            map[string]any
	pathParams                    []any
	tagParents                    map[string]string // 3.2 nested tags: a tag's name to its parent's
}

// Import maps an OpenAPI 3.0/3.1 or Swagger 2.0 document (YAML or JSON; Swagger is converted to 3.0 first) to requests,
// response schemas and suggested flows.
func Import(data []byte, o Options) (Result, diag.List) {
	data = textenc.ToUTF8(data)
	if isSwagger2(data) {
		upgraded, err := upgradeSwagger2(data)
		if err != nil {
			return Result{}, invalid(err.Error())
		}
		data = upgraded
	}
	if detail := checkOpenAPI(data); detail != "" {
		return Result{}, invalid(detail)
	}
	root, err := yamldoc.Decode(data)
	if err != nil {
		return Result{}, invalid(err.Error())
	}
	res := newResolver(root, o.Files)
	res.expandPathItems()
	groups := groupOperations(operations(root))
	out := Result{Schemas: map[string]map[string]map[string]any{}}
	byOperation := map[string]model.Request{}
	var diags diag.List
	paths := requestPaths(groups)
	for i, g := range groups {
		r, schemas, d := requestOf(g, res, o.NewID())
		r.Path = paths[i]
		diags = append(diags, d...)
		out.Requests = append(out.Requests, r)
		out.Schemas[r.Path] = schemas
		for _, op := range g {
			if id, _ := op.op["operationId"].(string); id != "" {
				byOperation[id] = r
			}
		}
	}
	flows, linkDiags := suggestedFlows(groups, byOperation, o.NewID)
	out.Flows = flows
	out.Files = res.fileContents()
	return out, slices.Concat(diags, linkDiags, externalRefs(res), missingOperationIDs(groups, paths), unregisteredFormats(root))
}

// missingOperationIDs warns about each operation without an operationId: its
// request is named from the method and path, which is fine for Flowi, but an
// Arazzo step refers to an operation only by its operationId.
func missingOperationIDs(groups [][]operation, paths []string) diag.List {
	var out diag.List
	for i, g := range groups {
		for _, op := range g {
			if id, _ := op.op["operationId"].(string); id == "" {
				out = append(out, diag.New(diag.CodeImportNoOpID, diag.Location{}, map[string]string{
					"operation": op.verb() + " " + op.path, "name": paths[i]}))
			}
		}
	}
	return out
}

// requestPaths names the request of each group, unique in group order: the
// names Import gives, which OperationsOf repeats.
func requestPaths(groups [][]operation) []string {
	var taken importers.Names
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = taken.Unique(requestPath(g[0]))
	}
	return out
}

// checkOpenAPI uses libopenapi to confirm an OpenAPI 3 document.
func checkOpenAPI(data []byte) string {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return err.Error()
	}
	if v := doc.GetVersion(); !strings.HasPrefix(v, "3.") {
		return fmt.Sprintf("OpenAPI %q; only 3.0 and 3.1 are supported", v)
	}
	return ""
}

func invalid(detail string) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": detail})}
}

func externalRefs(r *resolver) diag.List {
	var out diag.List
	for _, ref := range r.order {
		out = append(out, diag.New(diag.CodeImportUnmapped, diag.Location{},
			map[string]string{"what": "external reference " + ref, "how": "left out", "detail": r.external[ref]}))
	}
	return out
}

// expandPathItems replaces each path item that is a reference to another file
// (`/pets: {$ref: ./paths/pets.yaml}`) with the item it names, so that its
// operations are imported like any other.
func (r *resolver) expandPathItems() {
	paths, _ := r.main.root["paths"].(map[string]any)
	for p, item := range paths {
		if m, ok := item.(map[string]any); ok {
			if ref, _ := m["$ref"].(string); ref != "" && !strings.HasPrefix(ref, "#") {
				paths[p] = r.resolve(m)
			}
		}
	}
}

// operations lists every operation, paths sorted.
func operations(root map[string]any) []operation {
	paths, _ := root["paths"].(map[string]any)
	major := infoMajor(root)
	parents := tagParents(root)
	var out []operation
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		item, _ := paths[p].(map[string]any)
		shared, _ := item["parameters"].([]any)
		label, stripped := major, p
		if m := versionPrefix.FindStringSubmatch(p); m != nil {
			label, stripped = m[1], m[2]
			if stripped == "" {
				stripped = "/"
			}
		}
		for _, method := range methods() {
			if op, ok := item[method].(map[string]any); ok {
				out = append(out, operation{method: method, path: p, stripped: stripped, label: label, op: op, pathParams: shared, tagParents: parents})
			}
		}
		extra, _ := item["additionalOperations"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(extra)) {
			op, ok := extra[name].(map[string]any)
			if !ok || slices.Contains(methods(), strings.ToLower(name)) {
				continue // OpenAPI keeps the fixed fields for the fixed methods
			}
			out = append(out, operation{method: name, path: p, stripped: stripped, label: label, op: op, pathParams: shared, tagParents: parents})
		}
	}
	return out
}

var firstNumber = regexp.MustCompile(`\d+`)

// infoMajor is the major number of info.version ("1.4.0" → "1"; "v02" →
// "2"); without one, "1".
func infoMajor(root map[string]any) string {
	info, _ := root["info"].(map[string]any)
	n, err := strconv.Atoi(firstNumber.FindString(fmt.Sprint(info["version"])))
	if err != nil {
		return "1"
	}
	return strconv.Itoa(n)
}

// groupOperations groups the versions of one operation: the same method
// on the same path once a /vN/ prefix is removed. Groups keep first-seen
// order; versions are sorted.
func groupOperations(ops []operation) [][]operation {
	var keys []string
	groups := map[string][]operation{}
	for _, op := range ops {
		k := op.method + " " + op.stripped
		if _, seen := groups[k]; !seen {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], op)
	}
	out := make([][]operation, 0, len(keys))
	for _, k := range keys {
		g := groups[k]
		slices.SortStableFunc(g, func(a, b operation) int { return compareLabels(a.label, b.label) })
		out = append(out, g)
	}
	return out
}

func compareLabels(a, b string) int {
	la, errA := version.Parse(a)
	lb, errB := version.Parse(b)
	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}
	return la.Compare(lb)
}

// requestPath names a request: <first tag>/<operationId>, or
// <method>-<path slug> without an operationId. Names come from the
// document, so they are made into valid file names here; Import makes
// them unique.
func requestPath(op operation) string {
	id, _ := op.op["operationId"].(string)
	name := importers.FileName(id)
	if name == "" {
		name = importers.FileName(op.method + "-" + slug(op.stripped))
	}
	if tags := stringList(op.op["tags"]); len(tags) > 0 && slug(tags[0]) != "" {
		return tagFolder(tags[0], op.tagParents) + "/" + name
	}
	return name
}

// tagParents maps each tag of the document to its parent (OpenAPI 3.2 nests tags with "parent").
func tagParents(root map[string]any) map[string]string {
	out := map[string]string{}
	tags, _ := root["tags"].([]any)
	for _, t := range tags {
		tag, _ := t.(map[string]any)
		name, _ := tag["name"].(string)
		parent, _ := tag["parent"].(string)
		if name != "" && parent != "" {
			out[name] = parent
		}
	}
	return out
}

// tagFolder is the folder a tag stands for: the names of its parents, outermost first, then its own. A tag that
// names itself again through its parents ends the chain there: the spec forbids a loop, and a loop must not hang the import.
func tagFolder(tag string, parents map[string]string) string {
	chain := []string{tag}
	seen := map[string]bool{tag: true}
	for p, ok := parents[tag]; ok && !seen[p]; p, ok = parents[p] {
		seen[p] = true
		chain = append(chain, p)
	}
	var folders []string
	for i := len(chain) - 1; i >= 0; i-- {
		if s := slug(chain[i]); s != "" {
			folders = append(folders, importers.FileName(s))
		}
	}
	return strings.Join(folders, "/")
}

var nonSlug = regexp.MustCompile(`[^A-Za-z0-9]+`)

func slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// ParamKey is the template name a parameter is filled through ({{X_Trace}}
// for header X-Trace): request nodes pass it in with. Arazzo import and
// export use the same rule.
func ParamKey(name string) string { return unsafe.ReplaceAllString(name, "_") }

func requestOf(g []operation, res *resolver, id model.ID) (model.Request, map[string]map[string]any, diag.List) {
	first := g[0]
	r := model.Request{Meta: model.Meta{ID: id, Name: nameOf(first)}, Path: requestPath(first)}
	r.Operation, _ = first.op["operationId"].(string)
	if summary, _ := first.op["summary"].(string); summary != "" {
		r.Description = summary
	}
	schemas := map[string]map[string]any{}
	var diags diag.List
	for _, op := range g {
		r.Versions = append(r.Versions, versionOf(op, res))
		s, d := responseSchemas(op, res)
		diags = append(diags, d...)
		if len(s) > 0 {
			schemas[op.label] = s
		}
	}
	return r, schemas, diags
}

func nameOf(op operation) string {
	if s, _ := op.op["summary"].(string); s != "" {
		return s
	}
	if id, _ := op.op["operationId"].(string); id != "" {
		return id
	}
	return op.verb() + " " + op.stripped
}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

func versionOf(op operation, res *resolver) model.RequestVersion {
	url := pathParam.ReplaceAllStringFunc(op.path, func(m string) string { return "{{" + ParamKey(m[1:len(m)-1]) + "}}" })
	v := model.RequestVersion{Label: op.label, Method: op.verb(), URL: "{{env.base_url}}" + url}
	v.Docs, _ = op.op["description"].(string) // the operation's description is the version's documentation
	for _, p := range append(slices.Clone(op.pathParams), paramsOf(op.op)...) {
		pm, _ := res.resolve(p).(map[string]any)
		name, _ := pm["name"].(string)
		kv := model.KeyValue{Key: name, Value: "{{" + ParamKey(name) + "}}"}
		switch pm["in"] {
		case "query":
			v.Query = append(v.Query, kv)
		case "header":
			v.Headers = append(v.Headers, kv)
		case "querystring":
			// OpenAPI 3.2: the whole query string is one value (the media type in "content" says how it is written).
			if !strings.Contains(v.URL, "?") {
				v.URL += "?" + kv.Value
			}
		}
	}
	if isEventStream(op.op, res) {
		return serverSentEvents(v)
	}
	v.Body = bodyOf(op.op, res)
	return v
}

// isEventStream says whether an operation answers with text/event-stream and nothing JSON: a stream to listen to, not
// a response to read (OpenAPI 3.2 describes each event with itemSchema).
func isEventStream(op map[string]any, res *resolver) bool {
	responses, _ := op["responses"].(map[string]any)
	for code, r := range responses {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		rm, _ := res.resolve(r).(map[string]any)
		content, _ := rm["content"].(map[string]any)
		_, stream := content["text/event-stream"]
		if stream && jsonSchemaOf(rm) == nil {
			return true
		}
	}
	return false
}

// serverSentEvents turns v into a message request that listens to the stream at v's address.
func serverSentEvents(v model.RequestVersion) model.RequestVersion {
	return model.RequestVersion{
		Label: v.Label, Docs: v.Docs, Headers: v.Headers,
		Async: &model.Async{Protocol: "sse", Server: v.URL, Receive: &model.AsyncReceive{Count: 1, Timeout: 30 * time.Second}},
	}
}

func paramsOf(op map[string]any) []any {
	l, _ := op["parameters"].([]any)
	return l
}

// bodyOf generates a JSON body (or form fields) from the request body
// schema.
func bodyOf(op map[string]any, res *resolver) model.Body {
	rb, _ := res.resolve(op["requestBody"]).(map[string]any)
	content, _ := rb["content"].(map[string]any)
	if mt, ok := content["application/json"].(map[string]any); ok {
		// The media type's example wins over the schema's, as OpenAPI says; a
		// spec overlay saved from Flowi puts the edited body there.
		if ex, ok := mt["example"]; ok {
			return model.Body{JSON: jsonValue{raw: ex}.text()}
		}
		if text, ok := namedExample(mt, res); ok {
			return model.Body{JSON: text}
		}
		return model.Body{JSON: skeleton(mt["schema"], "body", 0).text()}
	}
	if mt, ok := content["application/x-www-form-urlencoded"].(map[string]any); ok {
		s, _ := mt["schema"].(map[string]any)
		props, _ := jsonschema.MergeAllOf(s)["properties"].(map[string]any)
		var form []model.KeyValue
		for _, k := range slices.Sorted(maps.Keys(props)) {
			form = append(form, model.KeyValue{Key: k, Value: "{{" + ParamKey(k) + "}}"})
		}
		return model.Body{Form: form}
	}
	return model.Body{}
}

// namedExample is the JSON of the first example (by name) of a media type that has a usable value: 3.2's dataValue,
// 3.1's value, or a serializedValue that is JSON text. dataValue and value come before serializedValue.
func namedExample(mt map[string]any, res *resolver) (string, bool) {
	examples, _ := mt["examples"].(map[string]any)
	for _, name := range slices.Sorted(maps.Keys(examples)) {
		ex, _ := res.resolve(examples[name]).(map[string]any)
		for _, key := range []string{"dataValue", "value"} {
			if v, ok := ex[key]; ok {
				return jsonValue{raw: v}.text(), true
			}
		}
		if text, _ := ex["serializedValue"].(string); json.Valid([]byte(text)) {
			return text, true
		}
	}
	return "", false
}

// statusCode is a response key kept as a contract. It becomes part of a
// file name, so nothing else from the document may reach it; ranges
// (2XX) and default are reported instead.
var statusCode = regexp.MustCompile(`^[1-5][0-9]{2}$`)

// responseSchemas are the JSON response schemas by status code.
func responseSchemas(op operation, res *resolver) (map[string]any, diag.List) {
	responses, _ := op.op["responses"].(map[string]any)
	out := map[string]any{}
	var diags diag.List
	for _, code := range slices.Sorted(maps.Keys(responses)) {
		rm, _ := res.resolve(responses[code]).(map[string]any)
		schema := jsonSchemaOf(rm)
		switch {
		case schema == nil:
		case !statusCode.MatchString(code):
			diags = append(diags, diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
				"what": fmt.Sprintf("response %q of %s %s", code, op.verb(), op.path), "how": "left out",
				"detail": "only exact status codes are kept as contracts"}))
		default:
			out[code] = schema
		}
	}
	return out, diags
}

// jsonSchemaOf is a response's schema for application/json or any
// +json media type.
func jsonSchemaOf(response map[string]any) any {
	content, _ := response["content"].(map[string]any)
	for _, mediaType := range slices.Sorted(maps.Keys(content)) {
		base, _, _ := strings.Cut(mediaType, ";")
		base = strings.TrimSpace(strings.ToLower(base))
		if base != "application/json" && !strings.HasSuffix(base, "+json") {
			continue
		}
		if mt, ok := content[mediaType].(map[string]any); ok && mt["schema"] != nil {
			return mt["schema"]
		}
	}
	return nil
}

// suggestedFlows turns each link (A's response → B) into a two-node flow.
// Link values that do not map are reported and left out.
func suggestedFlows(groups [][]operation, byOperation map[string]model.Request, newID func() model.ID) ([]model.Flow, diag.List) {
	var out []model.Flow
	var diags diag.List
	var taken importers.Names
	for _, g := range groups {
		for _, op := range g {
			from, _ := op.op["operationId"].(string)
			for _, link := range links(op.op) {
				target, _ := link["operationId"].(string)
				to, ok := byOperation[target]
				if from == "" || !ok {
					continue
				}
				f, d := linkFlow(byOperation[from], to, link, newID())
				f.Path = taken.Unique(f.Path)
				out, diags = append(out, f), append(diags, d...)
			}
		}
	}
	return out, diags
}

func links(op map[string]any) []map[string]any {
	responses, _ := op["responses"].(map[string]any)
	var out []map[string]any
	for _, code := range slices.Sorted(maps.Keys(responses)) {
		r, _ := responses[code].(map[string]any)
		ls, _ := r["links"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(ls)) {
			if l, ok := ls[name].(map[string]any); ok {
				out = append(out, l)
			}
		}
	}
	return out
}

func linkFlow(from, to model.Request, link map[string]any, id model.ID) (model.Flow, diag.List) {
	fromID, toID := path.Base(from.Path), path.Base(to.Path)
	params, _ := link["parameters"].(map[string]any)
	var with []model.KeyValue
	var diags diag.List
	for _, k := range slices.Sorted(maps.Keys(params)) {
		value, ok := linkValue(fmt.Sprint(params[k]), fromID)
		if !ok {
			diags = append(diags, diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
				"what": fmt.Sprintf("link parameter %s from %s to %s", k, fromID, toID), "how": "left out",
				"detail": fmt.Sprintf("expression %q has no Flowi equivalent", fmt.Sprint(params[k]))}))
			continue
		}
		with = append(with, model.KeyValue{Key: ParamKey(k), Value: value})
	}
	return model.Flow{
		Meta: model.Meta{ID: id, Name: fromID + " then " + toID}, Path: "suggested/" + importers.FileName(fromID+"-to-"+toID),
		Nodes: []model.Node{
			{ID: fromID, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: from.ID, Path: from.Path}}},
			{ID: toID, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: to.ID, Path: to.Path}, With: with}},
		},
	}, diags
}

// linkValue maps a link runtime expression to a template, or reports
// false: $response.body#/a/b → {{nodes.<from>.body.a.b}};
// $response.header.X → {{nodes.<from>.headers.x}}; $statusCode;
// $request.path|query|header.x → {{inputs.x}}. Text without "$" is a
// literal unless it would read as a template.
func linkValue(expr, from string) (string, bool) {
	node := "nodes" + refSegment(from)
	switch {
	case !strings.Contains(expr, "$"):
		return expr, !strings.Contains(expr, "{{")
	case expr == "$statusCode":
		return "{{" + node + ".status}}", true
	case expr == "$response.body":
		return "{{" + node + ".body}}", true
	case strings.HasPrefix(expr, "$response.body#/"):
		out := node + ".body"
		for _, p := range strings.Split(strings.TrimPrefix(expr, "$response.body#/"), "/") {
			out += refSegment(strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~"))
		}
		return "{{" + out + "}}", true
	case strings.HasPrefix(expr, "$response.header."):
		return "{{" + node + ".headers" + refSegment(strings.ToLower(strings.TrimPrefix(expr, "$response.header."))) + "}}", true
	}
	for _, prefix := range []string{"$request.path.", "$request.query.", "$request.header."} {
		if name, ok := strings.CutPrefix(expr, prefix); ok && name != "" {
			return "{{inputs." + ParamKey(name) + "}}", true
		}
	}
	return "", false
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// refSegment renders one reference step: .name, [0] or ["odd-name"].
func refSegment(s string) string {
	switch {
	case identifier.MatchString(s):
		return "." + s
	case s != "" && strings.Trim(s, "0123456789") == "":
		return "[" + s + "]"
	}
	return "[" + strconv.Quote(s) + "]"
}

// unregisteredFormats warns once about the schema formats of the document that the OpenAPI Format Registry does not
// list. A format that is not registered is legal: it annotates and nothing checks it, so a value of that format is not
// validated (here or by most tools). What is walked are the places schemas live; examples and constants are data and may
// hold a word "format" that is not one.
func unregisteredFormats(root map[string]any) diag.List {
	found := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for key, child := range x {
				switch key {
				case "example", "examples", "default", "enum", "const", "dataValue", "value", "serializedValue", "externalValue":
					continue
				case "format":
					if name, isString := child.(string); isString {
						if _, registered := formats.Lookup(name); !registered {
							found[name] = true
						}
						continue
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(root)
	if len(found) == 0 {
		return nil
	}
	names := slices.Sorted(maps.Keys(found))
	return diag.List{diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
		"what": "schema format " + strings.Join(names, ", "), "how": "kept as an annotation",
		"detail": "not in the OpenAPI Format Registry (" + formats.RegistryDate + "): values of this format are not checked",
	})}
}
