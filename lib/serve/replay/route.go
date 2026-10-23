// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/model"
)

// matchKind ranks how a request path matched a route: an exact match
// beats one behind extra leading segments (a base URL with a path prefix).
type matchKind int

const (
	matchExact matchKind = iota
	matchSuffix
)

// segment is one path segment of a route: literal text, or a pattern when
// it contains {{…}} (any non-empty text fills each placeholder).
type segment struct {
	text    string
	pattern *regexp.Regexp // nil for a literal segment
}

func (s segment) matches(part string) bool {
	if s.pattern == nil {
		return s.text == part
	}
	return s.pattern.MatchString(part)
}

// route is one version of one saved request that the replayer can answer.
type route struct {
	coll      model.Collection
	req       model.Request
	ver       model.RequestVersion
	method    string
	segs      []segment
	query     []model.KeyValue // literal query params the call must carry
	literal   int              // literal segments, for ranking
	chars     int              // characters in literal segments, for ranking
	isDefault bool
	order     int
}

// pattern renders the route's path for humans: /v1/users/{id}.
func (r route) pattern() string {
	if len(r.segs) == 0 {
		return "/"
	}
	parts := make([]string, len(r.segs))
	for i, s := range r.segs {
		parts[i] = templateName.ReplaceAllString(s.text, "{$1}")
	}
	return "/" + strings.Join(parts, "/")
}

var templateName = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

// index is the route table. It is immutable once built.
type index struct {
	routes  []route
	skipped []string // requests left out because their ids are not plain names
}

func buildIndex(cs []model.Collection) *index {
	ix := &index{}
	for _, c := range cs {
		for _, req := range c.Requests {
			for _, v := range req.Versions {
				// Ids and labels become directory names in the data dir.
				if !names.AllValid(string(c.Manifest.ID), string(req.ID), v.Label) {
					ix.skipped = append(ix.skipped, req.Path)
					continue
				}
				ix.routes = append(ix.routes, newRoute(c, req, v, len(ix.routes)))
			}
		}
	}
	return ix
}

func newRoute(c model.Collection, req model.Request, v model.RequestVersion, order int) route {
	path, rawQuery := splitURL(v.URL)
	r := route{coll: c, req: req, ver: v, method: strings.ToUpper(cmpOr(v.Method, "GET")), order: order,
		isDefault: req.Default == v.Label || (req.Default == "" && len(req.Versions) > 0 && req.Versions[0].Label == v.Label)}
	for _, part := range splitPath(path) {
		seg := compileSegment(part)
		if seg.pattern == nil {
			r.literal++
			r.chars += len(part)
		}
		r.segs = append(r.segs, seg)
	}
	r.query = literalQuery(rawQuery, v.Query)
	return r
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// splitURL cuts a URL template into its path and raw query. The scheme and
// host (or a leading {{base_url}} variable) are dropped: the replayer
// answers on its own address.
func splitURL(raw string) (path, query string) {
	raw, _, _ = strings.Cut(raw, "#")
	raw, query, _ = strings.Cut(raw, "?")
	return dropAuthority(strings.TrimSpace(raw)), query
}

func dropAuthority(s string) string {
	switch {
	case strings.Contains(s, "://"):
		_, rest, _ := strings.Cut(s, "://")
		return afterFirstSlash(rest)
	case strings.HasPrefix(s, "{{"):
		end := strings.Index(s, "}}")
		if end < 0 {
			return ""
		}
		return s[end+2:]
	case strings.HasPrefix(s, "/"):
		return s
	}
	return afterFirstSlash(s)
}

func afterFirstSlash(s string) string {
	if i := strings.Index(s, "/"); i >= 0 {
		return s[i:]
	}
	return ""
}

func splitPath(p string) []string {
	var out []string
	for _, part := range strings.Split(p, "/") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

var placeholder = regexp.MustCompile(`\{\{.*?\}\}`)

func compileSegment(part string) segment {
	locs := placeholder.FindAllStringIndex(part, -1)
	if len(locs) == 0 {
		return segment{text: part}
	}
	var b strings.Builder
	b.WriteString("^")
	last := 0
	for _, l := range locs {
		b.WriteString(regexp.QuoteMeta(part[last:l[0]]))
		b.WriteString(".+")
		last = l[1]
	}
	b.WriteString(regexp.QuoteMeta(part[last:]))
	b.WriteString("$")
	return segment{text: part, pattern: regexp.MustCompile(b.String())}
}

// literalQuery collects the query params that have a fixed value, from the
// URL's own query string and the request's query list.
func literalQuery(rawQuery string, declared []model.KeyValue) []model.KeyValue {
	var out []model.KeyValue
	add := func(k, v string) {
		if k != "" && v != "" && !strings.Contains(k, "{{") && !strings.Contains(v, "{{") {
			out = append(out, model.KeyValue{Key: k, Value: v})
		}
	}
	for _, pair := range strings.Split(rawQuery, "&") {
		k, v, _ := strings.Cut(pair, "=")
		add(unescape(k), unescape(v))
	}
	for _, kv := range declared {
		add(kv.Key, kv.Value)
	}
	return out
}

func unescape(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return s
}

// match reports whether the call's path segments (and query) fit the route.
func (r route) match(parts []string, q url.Values) (matchKind, bool) {
	for _, kv := range r.query {
		if !slices.Contains(q[kv.Key], kv.Value) {
			return 0, false
		}
	}
	extra := len(parts) - len(r.segs)
	switch {
	case extra < 0, extra > 0 && len(r.segs) == 0:
		return 0, false
	case !r.segmentsFit(parts[extra:]):
		return 0, false
	case extra == 0:
		return matchExact, true
	}
	return matchSuffix, true
}

func (r route) segmentsFit(parts []string) bool {
	for i, s := range r.segs {
		if !s.matches(parts[i]) {
			return false
		}
	}
	return true
}

// candidate is a route that fits a call, with how well.
type candidate struct {
	route
	kind matchKind
}

// lookup is what find learned about a call.
type lookup struct {
	candidates []candidate // best first
	allow      []string    // methods the path answers, when no candidate fits the method
}

// find ranks the routes that answer method and path. Among matches the
// exact path wins, then the most literal segments, then the most literal
// characters, then the most literal query rules, then the default version,
// then file order.
func (ix *index) find(method string, parts []string, q url.Values) lookup {
	var l lookup
	allow := map[string]bool{}
	for _, r := range ix.routes {
		kind, ok := r.match(parts, q)
		if !ok {
			continue
		}
		allow[r.method] = true
		if r.method == method || (method == "HEAD" && r.method == "GET") {
			l.candidates = append(l.candidates, candidate{route: r, kind: kind})
		}
	}
	sort.SliceStable(l.candidates, func(i, j int) bool { return better(l.candidates[i], l.candidates[j]) })
	if len(l.candidates) == 0 {
		l.allow = sortedKeys(allow)
	}
	return l
}

func better(a, b candidate) bool {
	switch {
	case a.kind != b.kind:
		return a.kind < b.kind
	case a.literal != b.literal:
		return a.literal > b.literal
	case a.chars != b.chars:
		return a.chars > b.chars
	case len(a.query) != len(b.query):
		return len(a.query) > len(b.query)
	case a.isDefault != b.isDefault:
		return a.isDefault
	}
	return a.order < b.order
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// PathOf is the path the replayer answers a request's URL on: the URL without
// its scheme, host (or leading {{variable}}), query and fragment. A template
// segment such as {{id}} is kept as written; the replayer matches any value
// there.
func PathOf(rawURL string) string {
	path, _ := splitURL(rawURL)
	return path
}
