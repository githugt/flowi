// SPDX-License-Identifier: Apache-2.0

package har

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

// A recording shows which value came from where: the id a POST returned is the id the next GET asks for. wire
// finds those values and makes the flow read them from the earlier step, so the flow runs against a server
// that hands out new ids and not only against the one that was recorded.
//
// What counts as the same value is exact text. Path ids are wired whenever they match. A query value or a
// JSON body field must also look like something a response gave (four characters or more, or a field
// whose name ends in "id"), so that a page size of 2 is not tied to a count of 2. Query values and body
// fields live in the saved request, which the calls of one endpoint share, so they are wired only for an
// endpoint the recording calls once; a repeated endpoint keeps what was recorded.

// callRecord is what the importer keeps of one call to wire it later.
type callRecord struct {
	node  int // index into importer.nodes
	group *group
	ids   []string // the path ids as recorded, in order
	query []pair
	body  any // the JSON request body, nil when there is none
	resp  any // the JSON response body, nil when there is none
}

// place is where a response held a value.
type place struct {
	expr  string // nodes.<step>.body.<path>
	idKey bool   // the field's name says it is an id
}

var idKeyName = regexp.MustCompile(`(?i)(^|[_-])id$|[a-z]Id$`)

func (im *importer) remember(g *group, e entry, ids []string) {
	c := &callRecord{node: len(im.nodes) - 1, group: g, ids: ids, query: e.Request.QueryString}
	if pd := e.Request.PostData; pd != nil && strings.Contains(strings.ToLower(pd.MimeType), "json") {
		c.body = decodeJSON(pd.Text)
	}
	if r := e.Response; r != nil && strings.Contains(strings.ToLower(r.Content.MimeType), "json") && r.Content.Encoding != "base64" {
		c.resp = decodeJSON(r.Content.Text)
	}
	im.calls = append(im.calls, c)
}

// decodeJSON parses text keeping numbers as written, or returns nil when it is not JSON.
func decodeJSON(text string) any {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return v
}

// wire rewrites the flow's steps and the saved requests in place. It runs once every call is known.
func (im *importer) wire() {
	seen := map[string]place{}
	perGroup := map[*group]int{}
	for _, c := range im.calls {
		perGroup[c.group]++
	}
	for _, c := range im.calls {
		n := &im.nodes[c.node]
		wirePathIDs(n, c, seen)
		if perGroup[c.group] == 1 {
			wireQuery(n, c, seen)
			wireBody(n, c, seen)
		}
		collect(seen, n.ID, c.resp)
	}
}

func wirePathIDs(n *model.Node, c *callRecord, seen map[string]place) {
	for i, id := range c.ids {
		if p, ok := seen[id]; ok && i < len(n.Request.With) {
			n.Request.With[i].Value = "{{" + p.expr + "}}"
		}
	}
}

func wireQuery(n *model.Node, c *callRecord, seen map[string]place) {
	query := c.group.req.Versions[0].Query
	for i, q := range query {
		raw := recordedQuery(c.query, q.Key)
		p, ok := seen[raw]
		if !ok || !distinctive(raw, p) || importers.SensitiveHeader(q.Key) {
			continue
		}
		name := withName(n, q.Key)
		query[i].Value = "{{with." + name + "}}"
		n.Request.With = append(n.Request.With, model.KeyValue{Key: name, Value: "{{" + p.expr + "}}"})
	}
}

func recordedQuery(query []pair, name string) string {
	for _, q := range query {
		if q.Name == name {
			return q.Value
		}
	}
	return ""
}

// wireBody replaces the JSON body fields that came from earlier responses with with-parameters. The body is
// rebuilt from the redacted copy, so a credential never comes back.
func wireBody(n *model.Node, c *callRecord, seen map[string]place) {
	if c.body == nil {
		return
	}
	v := &c.group.req.Versions[0]
	clean, _ := importers.RedactJSON(c.body)
	sentinels := map[string]string{} // sentinel text → the placeholder to write in its place
	changed := false
	walkLeaves(&clean, nil, func(path []string, leaf *any) {
		text, ok := leafText(*leaf)
		p, found := seen[text]
		if !ok || !found || !distinctive(text, p) {
			return
		}
		name := withName(n, strings.Join(path, "_"))
		n.Request.With = append(n.Request.With, model.KeyValue{Key: name, Value: "{{" + p.expr + "}}"})
		sentinel := "__flowi_with_" + name + "__"
		_, isString := (*leaf).(string)
		sentinels[strconv.Quote(sentinel)] = withPlaceholder(name, isString)
		*leaf = sentinel
		changed = true
	})
	if !changed {
		return
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return
	}
	text := string(data)
	for _, sentinel := range slices.Sorted(maps.Keys(sentinels)) {
		text = strings.ReplaceAll(text, sentinel, sentinels[sentinel])
	}
	v.Body = model.Body{JSON: text}
}

// withPlaceholder is how the request holds the parameter: a string field keeps its quotes, anything else is
// written bare so the value keeps its type (a number stays a number).
func withPlaceholder(name string, isString bool) string {
	if isString {
		return `"{{with.` + name + `}}"`
	}
	return "{{with." + name + "}}"
}

// withName makes a parameter name from a key, unique among the step's parameters.
func withName(n *model.Node, key string) string {
	base := nonName.ReplaceAllString(key, "_")
	base = strings.Trim(base, "_")
	if base == "" || (base[0] >= '0' && base[0] <= '9') {
		base = "v_" + base
	}
	name := base
	for i := 2; slices.ContainsFunc(n.Request.With, func(kv model.KeyValue) bool { return kv.Key == name }); i++ {
		name = base + "_" + strconv.Itoa(i)
	}
	return name
}

var nonName = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// distinctive says whether a value is enough like an identifier to tie to the response it came from.
func distinctive(text string, p place) bool { return len(text) >= 4 || p.idKey }

// collect remembers every scalar of a response by its text. The first response to hold a value wins: that is
// where the value came from, and later steps that echo it back are not its source.
func collect(seen map[string]place, step string, resp any) {
	if resp == nil {
		return
	}
	walkLeaves(&resp, nil, func(path []string, leaf *any) {
		text, ok := leafText(*leaf)
		if !ok {
			return
		}
		expr := "nodes." + step + ".body"
		last := ""
		for _, seg := range path {
			expr += segment(seg)
			last = seg
		}
		if _, known := seen[text]; !known {
			seen[text] = place{expr: expr, idKey: idKeyName.MatchString(last)}
		}
	})
}

// segment writes one step of a path the way expressions read it: .key, ["odd key"] or [3].
func segment(seg string) string {
	if strings.HasPrefix(seg, "[") {
		return seg
	}
	if identifier.MatchString(seg) {
		return "." + seg
	}
	return "[" + strconv.Quote(seg) + "]"
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// leafText is the text of a string or number leaf. Booleans, nulls and empty strings tie nothing together.
func leafText(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, x != ""
	case json.Number:
		return x.String(), true
	}
	return "", false
}

// walkLeaves calls f for every scalar under *v, with its path ([2] for list items) and a pointer to replace it.
func walkLeaves(v *any, path []string, f func(path []string, leaf *any)) {
	switch x := (*v).(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			child := x[k]
			walkLeaves(&child, append(path, k), f)
			x[k] = child
		}
	case []any:
		for i := range x {
			walkLeaves(&x[i], append(path, fmt.Sprintf("[%d]", i)), f)
		}
	default:
		f(path, v)
	}
}
