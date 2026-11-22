// SPDX-License-Identifier: Apache-2.0

package asyncapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/yamldoc"
	"github.com/githugt/flowi/lib/model"
)

// Options configure Import.
type Options struct {
	NewID func() model.ID // ids for new requests
}

// Result is what an AsyncAPI document becomes.
type Result struct {
	Requests []model.Request // Path is set (relative to requests/); File is not
}

// Import maps the operations of an AsyncAPI 3.x (or 2.x) document to async
// requests. An operation whose server speaks a protocol Flowi has no client
// for is left out and reported (FLW-IMPORT-001); nothing else stops the import.
func Import(data []byte, o Options) (Result, diag.List) {
	root, err := yamldoc.Decode(data)
	if err != nil {
		return Result{}, invalid(err.Error())
	}
	version, _ := root["asyncapi"].(string)
	var ops []operation
	switch {
	case strings.HasPrefix(version, "3."):
		ops = operationsV3(root)
	case strings.HasPrefix(version, "2."):
		ops = operationsV2(root)
	default:
		return Result{}, invalid("no asyncapi 2.x or 3.x version field")
	}
	var out Result
	var diags diag.List
	var taken importers.Names
	for _, op := range ops {
		req, d := requestOf(op, o.NewID())
		diags = append(diags, d...)
		if req == nil {
			continue
		}
		req.Path = taken.Unique(req.Path)
		out.Requests = append(out.Requests, *req)
	}
	if len(out.Requests) == 0 && !diags.HasErrors() && len(diags) == 0 {
		return Result{}, invalid("the document has no operations")
	}
	return out, diags
}

func invalid(detail string) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": detail})}
}

// operation is one AsyncAPI operation, read from either version.
type operation struct {
	id, title, summary string
	send, receive      bool
	channel            channel
	server             server
	message            map[string]any // the message this operation sends or receives
	bindings           bindingSources
	flowiBinding       map[string]any // x-flowi-binding of the operation: Flowi binding names the protocol bindings cannot carry
}

type channel struct {
	address string
}

type server struct {
	protocol, host, pathname, extension string
	scheme                              string // x-flowi-scheme: the URL scheme to write, where the protocol's usual one is not wanted
}

// ref follows a local "#/a/b" reference; anything else comes back as given.
func ref(root map[string]any, v any) map[string]any {
	m, _ := v.(map[string]any)
	for range 8 { // a bounded chain: a reference cycle must not loop
		r, ok := m["$ref"].(string)
		if !ok || !strings.HasPrefix(r, "#/") {
			break
		}
		var cur any = root
		for _, seg := range strings.Split(strings.TrimPrefix(r, "#/"), "/") {
			seg = strings.NewReplacer("~1", "/", "~0", "~").Replace(seg)
			next, _ := cur.(map[string]any)
			cur = next[seg]
		}
		m, _ = cur.(map[string]any)
	}
	return m
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func operationsV3(root map[string]any) []operation {
	servers, _ := root["servers"].(map[string]any)
	channels, _ := root["channels"].(map[string]any)
	opsDoc, _ := root["operations"].(map[string]any)
	var out []operation
	for _, id := range slices.Sorted(maps.Keys(opsDoc)) {
		od := ref(root, opsDoc[id])
		ch := ref(root, od["channel"])
		op := operation{id: id, title: str(od, "title"), summary: str(od, "summary"),
			send: str(od, "action") == "send", receive: str(od, "action") == "receive"}
		op.channel = channel{address: str(ch, "address")}
		op.server = firstServerV3(root, ch, servers)
		op.message = firstMessage(root, od["messages"], ch["messages"], channels)
		op.bindings = bindingSources{server: bindingsMap(root, firstServerRawV3(root, ch, servers)), channel: bindingsMap(root, ch), operation: bindingsMap(root, od), message: bindingsMap(root, op.message)}
		op.flowiBinding, _ = od["x-flowi-binding"].(map[string]any)
		if od["reply"] != nil {
			op.send, op.receive = true, true
		}
		out = append(out, op)
	}
	return out
}

// bindingsMap is the `bindings` object of a document node (nil for none).
func bindingsMap(root, node map[string]any) map[string]any {
	if node == nil {
		return nil
	}
	return ref(root, node["bindings"])
}

// firstServerRawV3 is firstServerV3's pick as the document wrote it.
func firstServerRawV3(root, ch, servers map[string]any) map[string]any {
	if list, _ := ch["servers"].([]any); len(list) > 0 {
		return ref(root, list[0])
	}
	if names := slices.Sorted(maps.Keys(servers)); len(names) > 0 {
		return ref(root, servers[names[0]])
	}
	return nil
}

// firstServerV3 is the first server the channel lists, else the first of the
// document.
func firstServerV3(root, ch, servers map[string]any) server {
	pick := func(v any) server {
		s := ref(root, v)
		return server{protocol: str(s, "protocol"), host: str(s, "host"), pathname: str(s, "pathname"), extension: str(s, "x-flowi-protocol"), scheme: str(s, "x-flowi-scheme")}
	}
	if list, _ := ch["servers"].([]any); len(list) > 0 {
		return pick(list[0])
	}
	if names := slices.Sorted(maps.Keys(servers)); len(names) > 0 {
		return pick(servers[names[0]])
	}
	return server{}
}

// firstMessage is the first message of the operation, else of its channel.
func firstMessage(root map[string]any, opMessages, chMessages any, _ map[string]any) map[string]any {
	for _, v := range []any{opMessages, chMessages} {
		switch list := v.(type) {
		case []any:
			if len(list) > 0 {
				return ref(root, list[0])
			}
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(list)) {
				return ref(root, list[k])
			}
		}
	}
	return nil
}

// operationsV2 reads publish and subscribe as a client of the API would use
// them: it sends on what the API accepts (publish) and receives what the API
// offers (subscribe).
func operationsV2(root map[string]any) []operation {
	servers, _ := root["servers"].(map[string]any)
	channels, _ := root["channels"].(map[string]any)
	var first server
	var firstRaw map[string]any
	if names := slices.Sorted(maps.Keys(servers)); len(names) > 0 {
		s := ref(root, servers[names[0]])
		firstRaw = s
		host, path, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(str(s, "url"), str(s, "protocol")+"://"), "//"), "/")
		first = server{protocol: str(s, "protocol"), host: host, extension: str(s, "x-flowi-protocol"), scheme: str(s, "x-flowi-scheme")}
		if path != "" {
			first.pathname = "/" + path
		}
	}
	var out []operation
	for _, address := range slices.Sorted(maps.Keys(channels)) {
		ch := ref(root, channels[address])
		for _, kind := range []string{"publish", "subscribe"} {
			od := ref(root, ch[kind])
			if od == nil {
				continue
			}
			id := str(od, "operationId")
			if id == "" {
				id = kind + "-" + address
			}
			op := operation{id: id, title: str(od, "summary"), summary: str(od, "description"), send: kind == "publish", receive: kind == "subscribe",
				channel: channel{address: address}, server: first}
			op.bindings = bindingSources{server: bindingsMap(root, firstRaw), channel: bindingsMap(root, ch), operation: bindingsMap(root, od)}
			op.flowiBinding, _ = od["x-flowi-binding"].(map[string]any)
			if m := ref(root, od["message"]); m != nil {
				op.message = m
				if one, _ := m["oneOf"].([]any); len(one) > 0 {
					op.message = ref(root, one[0])
				}
				op.bindings.message = bindingsMap(root, op.message)
			}
			out = append(out, op)
		}
	}
	return out
}

// protocolOf maps an AsyncAPI server protocol to Flowi's protocol and the
// URL scheme its server is written with.
func protocolOf(s server) (protocol, scheme string, ok bool) {
	if s.extension == model.ProtocolServiceBus {
		return model.ProtocolServiceBus, "sb", true
	}
	if s.extension == model.ProtocolAMQP10 {
		return model.ProtocolAMQP10, map[bool]string{true: "amqps", false: "amqp"}[strings.ToLower(s.protocol) == "amqps"], true
	}
	switch strings.ToLower(s.protocol) {
	case "ws", "wss":
		return model.ProtocolWS, strings.ToLower(s.protocol), true
	case "mqtt", "mqtts", "secure-mqtt":
		return model.ProtocolMQTT, map[bool]string{true: "mqtts", false: "mqtt"}[strings.ToLower(s.protocol) != "mqtt"], true
	case "kafka":
		return model.ProtocolKafka, "kafka", true
	case "kafka-secure":
		return model.ProtocolKafka, "kafkas", true
	case "amqp", "amqps":
		return model.ProtocolAMQP, strings.ToLower(s.protocol), true
	case "nats":
		return model.ProtocolNATS, "nats", true
	case "redis":
		return model.ProtocolRedis, "redis", true
	case "sqs":
		return model.ProtocolSQS, "https", true
	case "sns":
		return model.ProtocolSNS, "https", true
	case "googlepubsub":
		return model.ProtocolPubSub, "pubsub", true
	}
	return "", "", false
}

var (
	braced     = regexp.MustCompile(`\{([A-Za-z0-9_.-]+)\}`)
	nonKeyChar = regexp.MustCompile(`[^A-Za-z0-9_]`)
)

// templated turns {name} (a server variable or a channel parameter) into
// the {{env.name}} or {{name}} a request fills in.
func templated(s, prefix string) string {
	return braced.ReplaceAllStringFunc(s, func(m string) string {
		return "{{" + prefix + nonKeyChar.ReplaceAllString(m[1:len(m)-1], "_") + "}}"
	})
}

func requestOf(op operation, id model.ID) (*model.Request, diag.List) {
	protocol, scheme, ok := protocolOf(op.server)
	if !ok {
		what := "operation " + op.id
		detail := fmt.Sprintf("its server protocol is %q, which Flowi has no client for", op.server.protocol)
		return nil, diag.List{diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{"what": what, "how": "skipped", "detail": detail})}
	}
	var diags diag.List
	if want := op.server.scheme; want != "" {
		if slices.Contains(model.AsyncSchemes(protocol), want) {
			scheme = want
		} else {
			diags = append(diags, unmappedExtension(op.id, "x-flowi-scheme "+want, protocol+" has no such URL scheme"))
		}
	}
	server := ""
	if op.server.host != "" {
		server = scheme + "://" + templated(op.server.host, "env.") + op.server.pathname
	}
	a := model.Async{Protocol: protocol, Server: server, Channel: templated(op.channel.address, "")}
	binding, topic, left := bindingsOf(protocol, op.bindings)
	binding, extDiags := flowiBindings(op, protocol, binding)
	a.Binding = binding
	diags = append(diags, extDiags...)
	if topic != "" {
		a.Channel = templated(topic, "")
	}
	if op.send {
		a.Send = &model.AsyncSend{Body: model.Body{JSON: exampleOf(op.message)}}
	}
	if op.receive {
		a.Receive = &model.AsyncReceive{}
	}
	name := op.title
	if name == "" {
		name = op.id
	}
	r := model.Request{Meta: model.Meta{ID: id, Name: name, Description: op.summary}, Path: importers.FileName(op.id), Operation: op.id,
		Versions: []model.RequestVersion{{Label: "1", Async: &a}}}
	return &r, append(diags, unmappedBindings(op.id, left)...)
}

// flowiBindings applies x-flowi-binding of the operation: Flowi binding names (model.AsyncBindings) the AsyncAPI
// protocol bindings have no field for, such as the mode of a Redis request or the endpoint of a Pub/Sub emulator.
// A name the protocol does not have is reported. A credential is reported and never imported: the extension is
// for options, and credentials stay out of files.
func flowiBindings(op operation, protocol string, binding []model.KeyValue) ([]model.KeyValue, diag.List) {
	var diags diag.List
	for _, name := range slices.Sorted(maps.Keys(op.flowiBinding)) {
		value := scalar(op.flowiBinding[name])
		switch {
		case slices.Contains(model.AsyncCredentials(protocol), name):
			diags = append(diags, unmappedExtension(op.id, "x-flowi-binding "+name, "credentials are never imported: set it in the request's Authentication"))
			continue
		case !slices.Contains(model.AsyncBindings(protocol), name):
			diags = append(diags, unmappedExtension(op.id, "x-flowi-binding "+name, protocol+" has no such binding"))
			continue
		case value == "":
			continue
		}
		if i := slices.IndexFunc(binding, func(kv model.KeyValue) bool { return kv.Key == name }); i >= 0 {
			binding[i].Value = value
		} else {
			binding = append(binding, model.KeyValue{Key: name, Value: value})
		}
	}
	return binding, diags
}

func unmappedExtension(operation, what, detail string) diag.Diagnostic {
	return diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
		"what": what + " of operation " + operation, "how": "left out", "detail": detail})
}

// exampleOf is the first example payload of a message, else a skeleton of its
// payload schema (one empty value per property), pretty-printed JSON.
func exampleOf(msg map[string]any) string {
	if msg == nil {
		return ""
	}
	if list, _ := msg["examples"].([]any); len(list) > 0 {
		if ex, _ := list[0].(map[string]any); ex != nil && ex["payload"] != nil {
			return marshal(ex["payload"])
		}
	}
	schema, _ := msg["payload"].(map[string]any)
	if props, _ := schema["properties"].(map[string]any); len(props) > 0 {
		skeleton := map[string]any{}
		for _, k := range slices.Sorted(maps.Keys(props)) {
			skeleton[k] = emptyOf(props[k])
		}
		return marshal(skeleton)
	}
	return ""
}

func emptyOf(schema any) any {
	m, _ := schema.(map[string]any)
	switch str(m, "type") {
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

func marshal(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}
