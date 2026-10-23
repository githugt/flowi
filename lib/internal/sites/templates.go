// SPDX-License-Identifier: Apache-2.0

package sites

import (
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// Site is one template or expression and where it is written.
type Site struct {
	File, Path, Field, Src string
	Expression             bool   // a bare expression (assert check, when), not a {{…}} template
	Node                   string // the node the site belongs to; empty for flow and version sites
}

// Templates lists every template and expression in a collection.
func Templates(c model.Collection) []Site {
	var sites []Site
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			sites = append(sites, VersionTemplates(r.File, v)...)
		}
	}
	for _, n := range AllNodes(c) {
		sites = append(sites, NodeTemplates(n)...)
	}
	for _, f := range c.Flows {
		sites = append(sites, FlowTemplates(f)...)
	}
	return sites
}

// FlowTemplates are a flow's own templates: input defaults, outputs and
// the run-till condition.
func FlowTemplates(f model.Flow) []Site {
	var sites []Site
	for _, in := range f.Inputs {
		at := yamlpath.Key(yamlpath.Key(yamlpath.Key("$", "inputs"), in.Name), "default")
		sites = append(sites, Site{File: f.File, Path: at, Field: "default of input " + in.Name, Src: in.Default})
	}
	sites = append(sites, keyValueSites(f.File, yamlpath.Key("$", "outputs"), "output ", f.Outputs)...)
	if f.Run != nil && f.Run.Until != "" {
		sites = append(sites, Site{File: f.File, Path: yamlpath.Key(yamlpath.Key("$", "run"), "until"), Field: "run.until", Src: f.Run.Until, Expression: true})
	}
	return sites
}

// VersionTemplates are a request version's templates: url, query,
// headers, body and asserts.
func VersionTemplates(file string, v model.RequestVersion) []Site {
	base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
	sites := []Site{{File: file, Path: yamlpath.Key(base, "url"), Field: "url", Src: v.URL}}
	sites = append(sites, keyValueSites(file, yamlpath.Key(base, "query"), "query ", v.Query)...)
	sites = append(sites, keyValueSites(file, yamlpath.Key(base, "headers"), "header ", v.Headers)...)
	body := yamlpath.Key(base, "body")
	sites = append(sites,
		Site{File: file, Path: yamlpath.Key(body, "json"), Field: "body", Src: v.Body.JSON},
		Site{File: file, Path: yamlpath.Key(body, "text"), Field: "body", Src: v.Body.Text},
		Site{File: file, Path: yamlpath.Key(body, "file"), Field: "body file", Src: v.Body.File})
	sites = append(sites, keyValueSites(file, yamlpath.Key(body, "form"), "form field ", v.Body.Form)...)
	for i, check := range v.Assert {
		sites = append(sites, Site{File: file, Path: yamlpath.Index(yamlpath.Key(base, "assert"), i), Field: "assert", Src: check, Expression: true})
	}
	if v.Async != nil {
		sites = append(sites, asyncTemplates(file, yamlpath.Key(base, "async"), *v.Async)...)
	}
	if v.GRPC != nil {
		sites = append(sites, grpcTemplates(file, yamlpath.Key(base, "grpc"), *v.GRPC)...)
	}
	return sites
}

// asyncTemplates are the templates of an async version: server, channel,
// binding values, the sent message and the receive filter.
func asyncTemplates(file, base string, a model.Async) []Site {
	sites := []Site{
		{File: file, Path: yamlpath.Key(base, "server"), Field: "server", Src: a.Server},
		{File: file, Path: yamlpath.Key(base, "channel"), Field: "channel", Src: a.Channel},
	}
	sites = append(sites, keyValueSites(file, yamlpath.Key(base, "binding"), "binding ", a.Binding)...)
	if s := a.Send; s != nil {
		send := yamlpath.Key(base, "send")
		body := yamlpath.Key(send, "body")
		sites = append(sites, keyValueSites(file, yamlpath.Key(send, "headers"), "message header ", s.Headers)...)
		sites = append(sites,
			Site{File: file, Path: yamlpath.Key(body, "json"), Field: "body", Src: s.Body.JSON},
			Site{File: file, Path: yamlpath.Key(body, "text"), Field: "body", Src: s.Body.Text},
			Site{File: file, Path: yamlpath.Key(body, "file"), Field: "body file", Src: s.Body.File})
		sites = append(sites, keyValueSites(file, yamlpath.Key(body, "form"), "form field ", s.Body.Form)...)
	}
	if r := a.Receive; r != nil && r.Match != "" {
		sites = append(sites, Site{File: file, Path: yamlpath.Key(yamlpath.Key(base, "receive"), "match"), Field: "match", Src: r.Match, Expression: true})
	}
	return sites
}

// grpcTemplates are the templates of a gRPC version: server, method, the
// request message and the receive filter.
func grpcTemplates(file, base string, g model.GRPC) []Site {
	body := yamlpath.Key(base, "body")
	sites := []Site{
		{File: file, Path: yamlpath.Key(base, "server"), Field: "server", Src: g.Server},
		{File: file, Path: yamlpath.Key(base, "method"), Field: "method", Src: g.Method},
		{File: file, Path: yamlpath.Key(body, "json"), Field: "body", Src: g.Body.JSON},
		{File: file, Path: yamlpath.Key(body, "text"), Field: "body", Src: g.Body.Text},
		{File: file, Path: yamlpath.Key(body, "file"), Field: "body file", Src: g.Body.File},
	}
	sites = append(sites, keyValueSites(file, yamlpath.Key(body, "form"), "form field ", g.Body.Form)...)
	if r := g.Receive; r != nil && r.Match != "" {
		sites = append(sites, Site{File: file, Path: yamlpath.Key(yamlpath.Key(base, "receive"), "match"), Field: "match", Src: r.Match, Expression: true})
	}
	return sites
}

func keyValueSites(file, parent, prefix string, kvs []model.KeyValue) []Site {
	out := make([]Site, len(kvs))
	for i, kv := range kvs {
		out[i] = Site{File: file, Path: yamlpath.Key(parent, kv.Key), Field: prefix + kv.Key, Src: kv.Value}
	}
	return out
}

// NodeTemplates covers when, a request node's with values, and the
// template parameters of inline node types.
func NodeTemplates(n Located[model.Node]) []Site {
	node := n.Value
	var sites []Site
	if node.When != "" {
		sites = append(sites, Site{File: n.File, Path: yamlpath.Key(n.Path, "when"), Field: "when of node " + node.ID, Src: node.When, Expression: true})
	}
	if node.Request != nil {
		sites = append(sites, keyValueSites(n.File, yamlpath.Key(n.Path, "with"), "with ", node.Request.With)...)
		sites = append(sites, keyValueSites(n.File, yamlpath.Key(n.Path, "headers"), "header ", node.Request.Headers)...)
	}
	if node.Subflow != nil {
		sites = append(sites, keyValueSites(n.File, yamlpath.Key(n.Path, "with"), "with ", node.Subflow.With)...)
	}
	for _, p := range node.Params {
		kind, isTemplate := templateParam(node, p.Key)
		if !isTemplate {
			continue
		}
		field := p.Key + " of node " + node.ID
		sites = append(sites, valueSites(n.File, yamlpath.Key(n.Path, p.Key), field, p.Value, kind == expressionParam)...)
	}
	for i := range sites {
		sites[i].Node = node.ID
	}
	return sites
}

// valueSites are the templates in a parameter value: the string itself,
// or every string inside a list or mapping.
func valueSites(file, path, field string, v any, expression bool) []Site {
	switch x := v.(type) {
	case string:
		return []Site{{File: file, Path: path, Field: field, Src: x, Expression: expression}}
	case []any:
		var out []Site
		for i, item := range x {
			out = append(out, valueSites(file, yamlpath.Index(path, i), field, item, expression)...)
		}
		return out
	case []model.Field:
		var out []Site
		for _, f := range x {
			out = append(out, valueSites(file, yamlpath.Key(path, f.Key), field, f.Value, expression)...)
		}
		return out
	}
	return nil
}

type paramKind int

const (
	textParam paramKind = iota
	expressionParam
)

// templateParam says which node parameters hold templates or
// expressions, from the node's spec. Map-form keys are templates.
func templateParam(n model.Node, key string) (paramKind, bool) {
	spec, ok := SpecsByType()[n.Type]
	if !ok {
		return 0, false
	}
	p, known := spec.Param(key)
	switch {
	case !known:
		return textParam, spec.MapForm && (n.Type != "extract" || !SingleExtract(n))
	case p.Kind == nodes.ParamExpr:
		return expressionParam, true
	case p.Kind == nodes.ParamText || p.Kind == nodes.ParamAny || p.Kind == nodes.ParamMap:
		return textParam, true
	}
	return 0, false
}

// SpecsByType indexes the node registry (the single source of parameter
// rules, plan Decision 17).
func SpecsByType() map[model.NodeType]nodes.Spec {
	out := map[model.NodeType]nodes.Spec{}
	for _, s := range nodes.Specs() {
		out[s.Type] = s
	}
	return out
}
