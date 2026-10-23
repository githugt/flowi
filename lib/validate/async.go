// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkAsync reports what is wrong with the async versions of every request.
func checkAsync(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
				if v.Async != nil && v.GRPC == nil { // with both set, only the clash is reported
					out = append(out, asyncDiags(r.File, yamlpath.Key(base, "async"), *v.Async)...)
				}
				if v.GRPC != nil {
					out = append(out, grpcDiags(r.File, yamlpath.Key(base, "grpc"), v.Async != nil, *v.GRPC)...)
				}
			}
		}
	}
	return out
}

// asyncDiags checks one async block. path is the path of the block itself.
func asyncDiags(file, path string, a model.Async) diag.List {
	at := func(p string) diag.Location { return diag.Location{File: file, Path: p} }
	if !model.KnownProtocol(a.Protocol) {
		return diag.List{diag.New(diag.CodeAsyncProtocol, at(yamlpath.Key(path, "protocol")), map[string]string{"protocol": a.Protocol})}
	}
	var out diag.List
	out = append(out, actionDiags(at, path, a)...)
	out = append(out, bindingDiags(at, path, a)...)
	out = append(out, managedDiags(at, path, a)...)
	if d, bad := serverDiag(at(yamlpath.Key(path, "server")), a); bad {
		out = append(out, d)
	}
	return out
}

func actionDiags(at func(string) diag.Location, path string, a model.Async) diag.List {
	var out diag.List
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeAsyncAction, at(p), map[string]string{"detail": detail}))
	}
	if a.Action() == "" {
		problem(path, "neither send nor receive is set")
	}
	if a.Send != nil && len(a.Send.Body.Multipart) > 0 {
		problem(yamlpath.Key(yamlpath.Key(path, "send"), "body"), "a multipart body cannot be sent as a message")
	}
	if a.Protocol == model.ProtocolSSE && a.Send != nil {
		problem(yamlpath.Key(path, "send"), "sse is a one-way stream and cannot send")
	}
	if r := a.Receive; r != nil {
		recv := yamlpath.Key(path, "receive")
		if r.Count < 0 {
			problem(yamlpath.Key(recv, "count"), "count must not be negative")
		}
		if r.Timeout < 0 {
			problem(yamlpath.Key(recv, "timeout"), "timeout must not be negative")
		}
	}
	return out
}

func bindingDiags(at func(string) diag.Location, path string, a model.Async) diag.List {
	var out diag.List
	allowed := model.AsyncBindings(a.Protocol)
	for _, kv := range a.Binding {
		if !slices.Contains(allowed, kv.Key) {
			loc := at(yamlpath.Key(yamlpath.Key(path, "binding"), kv.Key))
			out = append(out, diag.New(diag.CodeAsyncBinding, loc, map[string]string{"name": kv.Key, "protocol": a.Protocol}))
		}
	}
	return out
}

// serverDiag checks the scheme of a server that is a plain URL; a server
// that starts with a template ({{env.x}}) is only known at run time.
func serverDiag(loc diag.Location, a model.Async) (diag.Diagnostic, bool) {
	if a.Server == "" || strings.HasPrefix(strings.TrimSpace(a.Server), "{{") {
		return diag.Diagnostic{}, false
	}
	schemes := model.AsyncSchemes(a.Protocol)
	// Only the scheme is checked, and by hand: the rest may hold templates
	// ({{env.host}}), which url.Parse rejects in a host.
	scheme, _, found := strings.Cut(a.Server, "://")
	if found && slices.Contains(schemes, strings.ToLower(scheme)) {
		return diag.Diagnostic{}, false
	}
	// NATS, Kafka and Service Bus servers are often written without a scheme;
	// SQS and SNS may leave the server out to reach AWS itself.
	if !found && schemeOptional(a.Protocol) {
		return diag.Diagnostic{}, false
	}
	return diag.New(diag.CodeAsyncServer, loc, map[string]string{
		"server": a.Server, "protocol": a.Protocol, "schemes": strings.Join(schemes, ", ")}), true
}

// schemeOptional reports whether servers of protocol p may leave the scheme off.
func schemeOptional(protocol string) bool {
	switch protocol {
	case model.ProtocolNATS, model.ProtocolKafka, model.ProtocolServiceBus:
		return true
	}
	return false
}

// managedDiags checks the rules of Redis and the managed messaging services:
// SNS only publishes, Redis pub/sub cannot tell its own message from a reply,
// and `auth` is one of the two modes.
func managedDiags(at func(string) diag.Location, path string, a model.Async) diag.List {
	var out diag.List
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeAsyncAction, at(p), map[string]string{"detail": detail}))
	}
	if a.Protocol == model.ProtocolSNS && a.Receive != nil {
		problem(yamlpath.Key(path, "receive"), "sns only publishes; subscribe a queue to the topic and receive from the queue")
	}
	if a.Protocol == model.ProtocolRedis && a.Send != nil && a.Receive != nil && bindingValue(a, "mode") != "stream" {
		problem(path, "a redis pub/sub channel cannot tell its own message from a reply; use mode: stream or two requests")
	}
	if mode := bindingValue(a, "auth"); mode != "" && model.CloudProtocol(a.Protocol) && !templated(mode) &&
		mode != model.AuthDefault && mode != model.AuthExplicit {
		problem(yamlpath.Key(yamlpath.Key(path, "binding"), "auth"), "auth must be default or explicit")
	}
	return out
}

// bindingValue is the value of the binding called name ("" when absent).
func bindingValue(a model.Async, name string) string {
	for _, kv := range a.Binding {
		if kv.Key == name {
			return kv.Value
		}
	}
	return ""
}

func templated(s string) bool { return strings.Contains(s, "{{") }
