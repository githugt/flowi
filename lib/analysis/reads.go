// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// Read kinds: what part of the response (or which input) a read relies on.
const (
	ReadBody     = "body"     // a body path from a template, e.g. nodes.login.body.data.token
	ReadJSONPath = "jsonpath" // an extraction's JSONPath
	ReadHeader   = "header"
	ReadCookie   = "cookie"
	ReadRegex    = "regex"   // an extraction's regex over the body text
	ReadExtract  = "extract" // an extract name the version does not define
	ReadInput    = "input"   // an input the request needs and the node does not fill
)

// Read is one thing a consumer relies on.
type Read struct {
	Kind   string `json:"kind"`
	Expr   string `json:"expr"`             // as written: nodes.login.token, response.body.id, extract token, with.tenant
	Target string `json:"target"`           // what is checked: $.data.token, a header, cookie or input name
	Via    string `json:"via,omitempty"`    // the extract name a node read goes through
	Reader string `json:"reader,omitempty"` // the node whose template reads it; empty for flow-level templates
	File   string `json:"file"`
	Path   string `json:"path"` // YAML path of the reading template
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`

	steps []string // body path steps, for ReadBody
}

// at fills in where a read is written.
func (r Read) at(s sites.Site, expr string) Read {
	r.Expr, r.Reader, r.File, r.Path = expr, s.Node, s.File, s.Path
	return r
}

// parseRefs lists what each site relies on: its unguarded reads (a
// guarded miss is nil by design, design §6.2). Sites that do not compile
// are skipped: validate reports them.
func parseRefs(siteList []sites.Site) []siteRef {
	var out []siteRef
	for _, s := range siteList {
		refs, err := template.References(s.Src, s.Expression)
		if err != nil {
			continue
		}
		for _, ref := range refs {
			if !ref.Guarded {
				out = append(out, siteRef{site: s, path: ref.Path})
			}
		}
	}
	return out
}

// outputReads are the reads of nodes.<id>… in a root's templates.
func outputReads(refs []siteRef, id string, v model.RequestVersion) []Read {
	var out []Read
	for _, ref := range refs {
		if len(ref.path) < 3 || ref.path[0] != "nodes" || ref.path[1] != id {
			continue
		}
		if r, ok := outputRead(ref.path[2:], v); ok {
			out = append(out, r.at(ref.site, strings.Join(ref.path, ".")))
		}
	}
	return out
}

// outputRead maps a read of a request node's output to what it needs from
// the response. status, time, vars and a whole body or header map rely on
// nothing a contract can lack.
func outputRead(rest []string, v model.RequestVersion) (Read, bool) {
	switch rest[0] {
	case "status", "time", "vars":
		return Read{}, false
	case "body":
		return bodyRead(rest[1:])
	case "headers":
		if len(rest) < 2 {
			return Read{}, false
		}
		return Read{Kind: ReadHeader, Target: rest[1]}, true
	}
	return extractionRead(rest[0], v)
}

func extractionRead(name string, v model.RequestVersion) (Read, bool) {
	for _, e := range v.Extract {
		if e.Name == name {
			r, ok := extractRead(e)
			r.Via = name
			return r, ok
		}
	}
	return Read{Kind: ReadExtract, Target: name, Status: Broken,
		Reason: "version " + v.Label + " does not extract " + name}, true
}

// extractRead is what an extraction needs (nodes.Extract semantics: a
// body regex wins over the path). A status extraction needs nothing.
func extractRead(e model.Extraction) (Read, bool) {
	switch {
	case e.From == "body" && e.Regex != "":
		return Read{Kind: ReadRegex, Target: e.Regex}, true
	case e.From == "body":
		return Read{Kind: ReadJSONPath, Target: e.Path}, true
	case e.From == "header":
		return Read{Kind: ReadHeader, Target: e.Header}, true
	case e.From == "cookie":
		return Read{Kind: ReadCookie, Target: e.Cookie}, true
	}
	return Read{}, false
}

func bodyRead(steps []string) (Read, bool) {
	if len(steps) == 0 {
		return Read{}, false
	}
	return Read{Kind: ReadBody, Target: jsonPathOf(steps), steps: steps}, true
}

// identifier is a step that needs no quoting in a JSONPath.
var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsonPathOf renders steps as a JSONPath for display: $.data.items[0]['x-y'].
func jsonPathOf(steps []string) string {
	var b strings.Builder
	b.WriteString("$")
	for _, s := range steps {
		switch {
		case isIndex(s):
			b.WriteString("[" + s + "]")
		case identifier.MatchString(s):
			b.WriteString("." + s)
		default:
			b.WriteString("['" + strings.ReplaceAll(s, "'", `\'`) + "']")
		}
	}
	return b.String()
}

func isIndex(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0
}

// responseRead maps a read of response.… (hooks, asserts).
func responseRead(path []string) (Read, bool) {
	if len(path) < 3 || path[0] != "response" {
		return Read{}, false
	}
	switch path[1] {
	case "body":
		return bodyRead(path[2:])
	case "headers":
		return Read{Kind: ReadHeader, Target: path[2]}, true
	case "cookies":
		return Read{Kind: ReadCookie, Target: path[2]}, true
	}
	return Read{}, false
}

// hookReads are a hook step's reads: response.… in its templates, and
// set values read from the response. They do not depend on the hooked request, so
// a collection hook's are worked out once.
func (ix *index) hookReads(rt root) []Read {
	if reads, ok := ix.hookMemo[rt.key()]; ok {
		return reads
	}
	out := responseReads(ix.refsOf(rt))
	for _, n := range rt.allNodes() {
		for i, v := range n.Value.Values {
			if v.From != "response" {
				continue
			}
			e := extractionOf(v)
			if r, ok := extractRead(e); ok {
				at := yamlpath.Index(yamlpath.Key(n.Path, "values"), i)
				out = append(out, r.at(sites.Site{File: n.File, Path: at, Node: n.Value.ID}, "value "+e.Name))
			}
		}
	}
	ix.hookMemo[rt.key()] = out
	return out
}

func extractionOf(v model.SetValue) model.Extraction {
	return model.Extraction{Name: v.Name, From: v.Where, Path: v.Path, Header: v.Header, Cookie: v.Cookie, Regex: v.Regex}
}

// ownReads are a version's own extract entries and the response reads of
// its asserts.
func ownReads(file string, v model.RequestVersion) []Read {
	base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
	var out []Read
	for _, e := range v.Extract {
		if r, ok := extractRead(e); ok {
			r.Via = e.Name
			out = append(out, r.at(sites.Site{File: file, Path: yamlpath.Key(yamlpath.Key(base, "extract"), e.Name)}, "extract "+e.Name))
		}
	}
	var asserts []sites.Site
	for i, check := range v.Assert {
		asserts = append(asserts, sites.Site{File: file, Path: yamlpath.Index(yamlpath.Key(base, "assert"), i), Src: check, Expression: true})
	}
	return append(out, responseReads(parseRefs(asserts))...)
}

// responseReads are the reads of response.… among refs.
func responseReads(refs []siteRef) []Read {
	var out []Read
	for _, ref := range refs {
		if r, ok := responseRead(ref.path); ok {
			out = append(out, r.at(ref.site, strings.Join(ref.path, ".")))
		}
	}
	return out
}
