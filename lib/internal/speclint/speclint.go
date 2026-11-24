// SPDX-License-Identifier: Apache-2.0

// Package speclint checks an API description itself, not the requests made from it: what a person reading it
// would want to find (what the API is for, what an operation does, how it is secured) and what has been left behind
// (components nothing refers to). It reads OpenAPI 3, Swagger 2, AsyncAPI 2 and 3 and OpenRPC, which share most of
// these questions under different field names, and .proto files (see LintProto). Each rule is a small function over the decoded document; the rules are data, so a
// collection can turn one off or make it an error (see ParseConfig).
//
// Findings are diagnostics (FLW-SPEC-*), so they reach the same Problems list as every other problem.
package speclint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/yamldoc"
	"github.com/githugt/flowi/lib/internal/yamlpath"
)

// Levels a rule can be set to.
const (
	LevelOff     = "off"
	LevelWarning = "warning"
	LevelError   = "error"
)

// Config says how seriously each rule is taken: a rule name to a level. A rule it does not name has its default.
type Config map[string]string

// finding is one thing a rule found: where, and the words for the message.
type finding struct {
	path string
	line int // 1-based, when the rule knows it (a .proto file has no YAML path); 0 otherwise
	args map[string]string
}

// rule is one check. defaultLevel is what it is without a Config entry.
type rule struct {
	name         string
	code         diag.Code
	defaultLevel string
	about        string // what the rule asks for, in a line
	run          func(d *document) []finding
}

// rules is every check, in the order their findings are reported. Naming rules are off until a collection wants them:
// they are a house style, not a mistake.
func rules() []rule {
	return []rule{
		{"info-description", diag.CodeSpecInfo, LevelWarning, "The description says what the API is for.", infoDescription},
		{"operation-description", diag.CodeSpecSummary, LevelWarning, "Every operation (or method, or channel operation) has a summary or a description.", operationDescription},
		{"operation-id", diag.CodeSpecOperation, LevelWarning, "Every operation has an operationId (OpenAPI, Swagger, AsyncAPI 2).", operationID},
		{"operation-tags", diag.CodeSpecTags, LevelWarning, "Every operation has a tag.", operationTags},
		{"operation-security", diag.CodeSpecSecurity, LevelWarning, "Every operation has a security requirement, or says it is public with security: [].", operationSecurity},
		{"success-response", diag.CodeSpecSuccess, LevelWarning, "Every operation documents a 2xx response.", successResponse},
		{"unused-component", diag.CodeSpecUnused, LevelWarning, "Every component is referred to somewhere.", unusedComponents},
		{"server-encrypted", diag.CodeSpecPlaintext, LevelWarning, "Servers are encrypted (https, wss, mqtts, amqps…); plain ones are for this machine only.", serverEncrypted},
		{"path-case", diag.CodeSpecPathCase, LevelOff, "Path segments are lowercase.", pathCase},
	}
}

// RuleNames lists the rules, for the message that names them and for a settings screen.
func RuleNames() []string {
	var out []string
	for _, r := range rules() {
		out = append(out, r.name)
	}
	return out
}

// RuleInfo describes a rule for a settings screen.
type RuleInfo struct {
	Name    string
	About   string
	Default string
}

// Info lists the rules with what each asks for and its default level.
func Info() []RuleInfo {
	var out []RuleInfo
	for _, r := range rules() {
		out = append(out, RuleInfo{Name: r.name, About: r.about, Default: r.defaultLevel})
	}
	return out
}

// Render writes cfg as a lint file, rules in their usual order. A level a rule has by default is left out, so the file
// says only what the collection chose.
func Render(cfg Config) []byte {
	var b strings.Builder
	b.WriteString("# How seriously each rule about the API description is taken: off, warning or error.\n")
	chosen := 0
	for _, r := range rules() {
		if l, ok := cfg[r.name]; ok && l != r.defaultLevel {
			if chosen == 0 {
				b.WriteString("rules:\n")
			}
			chosen++
			fmt.Fprintf(&b, "  %s: %s\n", r.name, l)
		}
	}
	if chosen == 0 {
		b.WriteString("rules: {}\n")
	}
	return []byte(b.String())
}

// DefaultLevel is the level of a rule with no Config entry; "" for a name that is not a rule.
func DefaultLevel(name string) string {
	for _, r := range rules() {
		if r.name == name {
			return r.defaultLevel
		}
	}
	return ""
}

// ParseConfig reads a lint file: `rules:` with a rule name and a level for each. A name or a level it does not know
// is reported and left out, so one typo does not switch off the rest.
func ParseConfig(data []byte, file string) (Config, diag.List) {
	root, err := yamldoc.Decode(data)
	if err != nil {
		return nil, diag.List{configProblem(file, "$", "it is not valid YAML: "+err.Error())}
	}
	given, _ := root["rules"].(map[string]any)
	cfg := Config{}
	var out diag.List
	for _, name := range sortedKeys(given) {
		path := yamlpath.Key("$.rules", name)
		level, _ := given[name].(string)
		level = normaliseLevel(level)
		switch {
		case DefaultLevel(name) == "":
			out = append(out, configProblem(file, path, fmt.Sprintf("%q is not a rule", name)))
		case level == "":
			out = append(out, configProblem(file, path, fmt.Sprintf("%v is not a level for %s", given[name], name)))
		default:
			cfg[name] = level
		}
	}
	return cfg, out
}

func normaliseLevel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "false", "none":
		return LevelOff
	case "warning", "warn":
		return LevelWarning
	case "error":
		return LevelError
	}
	return ""
}

func configProblem(file, path, detail string) diag.Diagnostic {
	return diag.New(diag.CodeSpecLintConfig, diag.Location{File: file, Path: path},
		map[string]string{"detail": detail, "known": strings.Join(RuleNames(), ", ")})
}

// Lint checks an API description: OpenAPI 3, Swagger 2, AsyncAPI 2 or 3, or OpenRPC (told apart by the field each
// starts with). spec names it in the messages; file is where its findings point (the file the description was kept
// in). A document of another kind is not linted and is no error.
func Lint(data []byte, spec, file string, cfg Config) (diag.List, error) {
	root, err := yamldoc.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", spec, err)
	}
	kind := kindOf(root)
	if kind == "" {
		return nil, nil
	}
	d := &document{root: root, spec: spec, kind: kind}
	var out diag.List
	for _, r := range rules() {
		level := levelOf(r, cfg)
		if level == LevelOff {
			continue
		}
		for _, f := range r.run(d) {
			out = append(out, finish(r, level, spec, file, f))
		}
	}
	return out, nil
}

func levelOf(r rule, cfg Config) string {
	if l, ok := cfg[r.name]; ok {
		return l
	}
	return r.defaultLevel
}

// finish turns what a rule found into a diagnostic at the level the collection gave the rule.
func finish(r rule, level, spec, file string, f finding) diag.Diagnostic {
	args := map[string]string{"spec": spec}
	for k, v := range f.args {
		args[k] = v
	}
	item := diag.New(r.code, diag.Location{File: file, Path: f.path, Pos: diag.Position{Line: f.line}}, args)
	if level == LevelError {
		item.Severity = diag.SeverityError
	}
	return item
}

// The kinds of description Lint reads.
const (
	kindOpenAPI  = "openapi"
	kindSwagger  = "swagger"
	kindAsyncAPI = "asyncapi"
	kindOpenRPC  = "openrpc"
)

// kindOf says what a decoded description is, by the field every version of it starts with; "" for none of them.
func kindOf(root map[string]any) string {
	for _, k := range []string{kindOpenAPI, kindSwagger, kindAsyncAPI, kindOpenRPC} {
		if _, ok := root[k]; ok {
			return k
		}
	}
	return ""
}

// document is the decoded description and what the rules share about it.
type document struct {
	root map[string]any
	spec string
	kind string // one of the kinds above
}

// http says whether the description is of an HTTP API with paths and responses, as opposed to messages or methods.
func (d *document) http() bool { return d.kind == kindOpenAPI || d.kind == kindSwagger }

// operation is one method of one path.
type operation struct {
	path   string // "/pets"
	method string // "get"
	op     map[string]any
	at     string // the YAML path of the operation
}

func (o operation) where() string {
	if o.method == "" {
		return o.path
	}
	return strings.ToUpper(o.method) + " " + o.path
}

func httpMethods() []string {
	return []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
}

// operations lists every operation: for an HTTP API in path order, then method order; for AsyncAPI each channel's
// publish and subscribe (version 2) or each entry of operations (version 3); for OpenRPC each method.
func (d *document) operations() []operation {
	switch d.kind {
	case kindAsyncAPI:
		return d.asyncOperations()
	case kindOpenRPC:
		return d.rpcMethods()
	}
	paths, _ := d.root["paths"].(map[string]any)
	var out []operation
	for _, p := range sortedKeys(paths) {
		item, _ := paths[p].(map[string]any)
		for _, m := range httpMethods() {
			op, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			at := yamlpath.Key(yamlpath.Key("$.paths", p), m)
			out = append(out, operation{path: p, method: m, op: op, at: at})
		}
	}
	return out
}

func (d *document) asyncOperations() []operation {
	var out []operation
	if ops, ok := d.root["operations"].(map[string]any); ok { // version 3: named operations
		for _, name := range sortedKeys(ops) {
			if op, ok := ops[name].(map[string]any); ok {
				out = append(out, operation{path: name, method: text(op["action"]), op: op, at: yamlpath.Key("$.operations", name)})
			}
		}
		return out
	}
	channels, _ := d.root["channels"].(map[string]any)
	for _, ch := range sortedKeys(channels) {
		item, _ := channels[ch].(map[string]any)
		for _, m := range []string{"publish", "subscribe"} {
			if op, ok := item[m].(map[string]any); ok {
				out = append(out, operation{path: ch, method: m, op: op, at: yamlpath.Key(yamlpath.Key("$.channels", ch), m)})
			}
		}
	}
	return out
}

func (d *document) rpcMethods() []operation {
	methods, _ := d.root["methods"].([]any)
	var out []operation
	for i, m := range methods {
		if op, ok := m.(map[string]any); ok {
			out = append(out, operation{path: text(op["name"]), op: op, at: yamlpath.Index("$.methods", i)})
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func text(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func onEach(d *document, bad func(operation) bool) []finding {
	var out []finding
	for _, o := range d.operations() {
		if bad(o) {
			out = append(out, finding{path: o.at, args: map[string]string{"where": o.where()}})
		}
	}
	return out
}

func infoDescription(d *document) []finding {
	info, _ := d.root["info"].(map[string]any)
	if text(info["description"]) != "" {
		return nil
	}
	return []finding{{path: "$.info"}}
}

func operationDescription(d *document) []finding {
	return onEach(d, func(o operation) bool { return text(o.op["summary"]) == "" && text(o.op["description"]) == "" })
}

// operationID: an AsyncAPI 3 operation is named by its key and an OpenRPC method by its name, so only the others can lack one.
func operationID(d *document) []finding {
	if d.kind == kindOpenRPC || (d.kind == kindAsyncAPI && d.root["operations"] != nil) {
		return nil
	}
	return onEach(d, func(o operation) bool { return text(o.op["operationId"]) == "" })
}

func operationTags(d *document) []finding {
	return onEach(d, func(o operation) bool {
		tags, _ := o.op["tags"].([]any)
		return len(tags) == 0
	})
}

// operationSecurity wants a requirement on the operation or one for the whole description. An empty list is a
// decision (a public operation), so it passes.
func operationSecurity(d *document) []finding {
	if !d.http() {
		return nil // a message API is secured on its servers
	}
	global, hasGlobal := d.root["security"].([]any)
	if hasGlobal && len(global) > 0 {
		return nil
	}
	return onEach(d, func(o operation) bool {
		_, own := o.op["security"].([]any)
		return !own
	})
}

func successResponse(d *document) []finding {
	if !d.http() {
		return nil
	}
	return onEach(d, func(o operation) bool {
		responses, _ := o.op["responses"].(map[string]any)
		for code := range responses {
			if strings.HasPrefix(code, "2") {
				return false
			}
		}
		return true
	})
}

func pathCase(d *document) []finding {
	if !d.http() {
		return nil
	}
	paths, _ := d.root["paths"].(map[string]any)
	var out []finding
	for _, p := range sortedKeys(paths) {
		if literalSegments(p) != strings.ToLower(literalSegments(p)) {
			out = append(out, finding{path: yamlpath.Key("$.paths", p), args: map[string]string{"path": p}})
		}
	}
	return out
}

// literalSegments is the path without its {parameters}, which are named by the code, not by the style of the URL.
func literalSegments(p string) string {
	var b strings.Builder
	depth := 0
	for _, r := range p {
		switch {
		case r == '{':
			depth++
		case r == '}':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func serverEncrypted(d *document) []finding {
	switch d.kind {
	case kindSwagger:
		return swaggerSchemes(d)
	case kindAsyncAPI:
		return asyncServers(d)
	}
	servers, _ := d.root["servers"].([]any)
	var out []finding
	for i, s := range servers {
		server, _ := s.(map[string]any)
		url := text(server["url"])
		if plainURL(url) {
			out = append(out, finding{path: yamlpath.Index("$.servers", i), args: map[string]string{"server": url}})
		}
	}
	return out
}

// plainURL is true for an http:// or ws:// address that leaves the machine.
func plainURL(url string) bool {
	lower := strings.ToLower(url)
	for _, scheme := range []string{"http://", "ws://"} {
		if strings.HasPrefix(lower, scheme) {
			return !isLocalHost(lower[len(scheme):])
		}
	}
	return false
}

// swaggerSchemes: a Swagger 2 description names its schemes once, for its host; plain ones without https beside them
// leave no encrypted way in.
func swaggerSchemes(d *document) []finding {
	schemes, _ := d.root["schemes"].([]any)
	plain, secure := false, false
	for _, s := range schemes {
		switch strings.ToLower(text(s)) {
		case "http", "ws":
			plain = true
		case "https", "wss":
			secure = true
		}
	}
	host := text(d.root["host"])
	if !plain || secure || (host != "" && isLocalHost(host)) {
		return nil
	}
	return []finding{{path: "$.schemes", args: map[string]string{"server": "http://" + host + text(d.root["basePath"])}}}
}

// plainProtocol says whether an AsyncAPI protocol carries nothing encrypted; the secure forms have a name of their own
// (mqtts, amqps, kafka-secure, wss, https).
func plainProtocol(protocol string) bool {
	switch protocol {
	case "http", "ws", "mqtt", "amqp", "kafka", "stomp", "nats", "redis":
		return true
	}
	return false
}

// asyncServers: version 2 servers have a url and version 3 a host, both with a protocol.
func asyncServers(d *document) []finding {
	servers, _ := d.root["servers"].(map[string]any)
	var out []finding
	for _, name := range sortedKeys(servers) {
		server, _ := servers[name].(map[string]any)
		protocol := strings.ToLower(text(server["protocol"]))
		address := firstText(text(server["url"]), text(server["host"]))
		host := address
		if _, rest, ok := strings.Cut(address, "://"); ok {
			host = rest
		}
		if plainProtocol(protocol) && !isLocalHost(host) {
			out = append(out, finding{path: yamlpath.Key("$.servers", name), args: map[string]string{"server": firstText(address, name) + " (" + protocol + ")"}})
		}
	}
	return out
}

func firstText(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// isLocalHost is true for an address (host, port and path, with no scheme) that does not leave the machine, where plain http is the usual choice.
func isLocalHost(hostAndPath string) bool {
	host, _, _ := strings.Cut(hostAndPath, "/")
	host = strings.TrimPrefix(host, "[")
	for _, local := range []string{"localhost", "127.0.0.1", "::1"} {
		if host == local || strings.HasPrefix(host, local+":") || strings.HasPrefix(host, local+"]") {
			return true
		}
	}
	return strings.HasSuffix(strings.SplitN(host, ":", 2)[0], ".localhost")
}

// componentKinds are the parts of a description's components that are referred to with $ref. Security schemes are
// named in security requirements instead, so they are not here.
func (d *document) componentKinds() (root string, kinds []string) {
	switch d.kind {
	case kindSwagger:
		return "", []string{"definitions", "parameters", "responses"}
	case kindAsyncAPI:
		return "components", []string{"schemas", "messages", "parameters", "correlationIds", "operationTraits", "messageTraits"}
	case kindOpenRPC:
		return "components", []string{"contentDescriptors", "schemas", "examples", "links", "errors", "examplePairingObjects", "tags"}
	}
	return "components", []string{"schemas", "parameters", "responses", "requestBodies", "headers", "examples", "links", "callbacks"}
}

func unusedComponents(d *document) []finding {
	root, kinds := d.componentKinds()
	holder := d.root
	prefix, at := "#/", "$"
	if root != "" {
		holder, _ = d.root[root].(map[string]any)
		prefix, at = "#/"+root+"/", yamlpath.Key("$", root)
	}
	if len(holder) == 0 {
		return nil
	}
	used := map[string]bool{}
	collectRefs(d.root, used)
	var out []finding
	for _, kind := range kinds {
		group, _ := holder[kind].(map[string]any)
		for _, name := range sortedKeys(group) {
			if used[prefix+kind+"/"+name] {
				continue
			}
			out = append(out, finding{path: yamlpath.Key(yamlpath.Key(at, kind), name), args: map[string]string{"component": kind + "/" + name}})
		}
	}
	return out
}

// collectRefs notes every "#/…" the document points at with $ref.
func collectRefs(v any, into map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if ref, ok := child.(string); ok && k == "$ref" {
				into[unescapePointer(ref)] = true
				continue
			}
			collectRefs(child, into)
		}
	case []any:
		for _, child := range x {
			collectRefs(child, into)
		}
	}
}

// unescapePointer undoes JSON Pointer's ~1 and ~0, so a name with a slash matches how it was defined.
func unescapePointer(ref string) string {
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(ref)
}
