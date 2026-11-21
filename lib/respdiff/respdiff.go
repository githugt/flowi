// SPDX-License-Identifier: Apache-2.0

// Package respdiff compares two responses of a request: the status, the headers and the body, the body by path when
// both are JSON ($.items[3].price 4.5 → 5) and by line when they are text.
//
// Before: to see what a response changed the person copied both bodies into another tool; the pane had no
// comparison, and a line diff of a large body is quadratic.
//
// Values that look like credentials are not compared (a token that changes on every send is not news) and never
// shown: headers and JSON fields named like a credential read (redacted) on both sides, and the number of them is
// reported.
package respdiff

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/importers"
)

// Kind says how a value differs.
type Kind string

// The kinds of change.
const (
	Added   Kind = "added"
	Removed Kind = "removed"
	Changed Kind = "changed"
)

// Change is one difference. Path is a JSON path for a JSON body ($.items[3].price), "line 12" for a text body and a
// header's name for a header. Old and New are the values as short text; the one that is absent is "".
type Change struct {
	Path string
	Kind Kind
	Old  string
	New  string
}

// Diff is the comparison of an older response with a newer one.
type Diff struct {
	OldStatus, NewStatus int
	Headers              []Change
	Body                 []Change
	// BodyMode is "json", "text", "binary" (compared by digest and size only) or "" when both have no body.
	BodyMode string
	// Truncated counts changes found beyond the limit and not listed.
	Truncated int
	// Hidden counts credential-named values left out of the comparison, and Volatile the headers that change on
	// every send (Date, a request id) and were skipped.
	Hidden, Volatile int
}

// Same reports whether the two responses do not differ in anything compared.
func (d Diff) Same() bool {
	return d.OldStatus == d.NewStatus && len(d.Headers) == 0 && len(d.Body) == 0 && d.Truncated == 0
}

// Options sizes the comparison.
type Options struct {
	// MaxChanges is how many changes of the body are listed; 0 means 200.
	MaxChanges int
}

const (
	defaultMaxChanges = 200
	maxShown          = 80
	// lcsCells bounds the line comparison: a body of two lists whose product is bigger is compared as one changed
	// block between its common start and end.
	lcsCells = 1_000_000
)

// volatileHeader reports whether a (lower-case) header changes on every response, so a difference in it means nothing.
func volatileHeader(k string) bool {
	switch k {
	case "date", "age", "x-request-id", "x-correlation-id", "x-trace-id", "traceparent",
		"server-timing", "cf-ray", "x-amzn-requestid", "x-amz-request-id", "x-runtime":
		return true
	}
	return false
}

// Compare compares older with newer.
func Compare(older, newer engine.Record, o Options) Diff {
	if o.MaxChanges <= 0 {
		o.MaxChanges = defaultMaxChanges
	}
	d := Diff{OldStatus: older.Status, NewStatus: newer.Status}
	d.Headers = compareHeaders(older.Header, newer.Header, &d)
	compareBodies(older.Body, newer.Body, o.MaxChanges, &d)
	return d
}

func compareHeaders(a, b []engine.Header, d *Diff) []Change {
	index := func(hs []engine.Header) (map[string]string, []string) {
		m, order := map[string]string{}, []string{}
		for _, h := range hs {
			k := strings.ToLower(h.Name)
			if volatileHeader(k) {
				d.Volatile++
				continue
			}
			if importers.SensitiveHeader(h.Name) {
				d.Hidden++
				continue
			}
			if _, seen := m[k]; seen {
				m[k] += ", " + h.Value
				continue
			}
			m[k] = h.Value
			order = append(order, k)
		}
		return m, order
	}
	am, aOrder := index(a)
	bm, bOrder := index(b)
	var out []Change
	for _, k := range aOrder {
		switch nv, ok := bm[k]; {
		case !ok:
			out = append(out, Change{Path: k, Kind: Removed, Old: short(am[k])})
		case nv != am[k]:
			out = append(out, Change{Path: k, Kind: Changed, Old: short(am[k]), New: short(nv)})
		}
	}
	for _, k := range bOrder {
		if _, ok := am[k]; !ok {
			out = append(out, Change{Path: k, Kind: Added, New: short(bm[k])})
		}
	}
	return out
}

func compareBodies(a, b engine.Body, limit int, d *Diff) {
	switch {
	case a.JSON != nil && b.JSON != nil:
		d.BodyMode = "json"
		ra, na := importers.RedactJSON(a.JSON)
		rb, nb := importers.RedactJSON(b.JSON)
		d.Hidden += max(na, nb)
		var c collector
		c.max = limit
		walk(&c, "$", ra, rb)
		d.Body, d.Truncated = c.out, c.over
	case textOf(a) != "" || textOf(b) != "":
		d.BodyMode = "text"
		d.Body, d.Truncated = lineChanges(textOf(a), textOf(b), limit)
	case a.SHA256 != "" || b.SHA256 != "" || a.Size > 0 || b.Size > 0:
		d.BodyMode = "binary"
		if a.SHA256 != b.SHA256 || a.Size != b.Size {
			d.Body = []Change{{Path: "body", Kind: Changed, Old: fmt.Sprintf("%d bytes", a.Size), New: fmt.Sprintf("%d bytes", b.Size)}}
		}
	}
}

func textOf(b engine.Body) string {
	if b.Text != "" {
		return b.Text
	}
	if b.JSON != nil {
		if p, err := json.MarshalIndent(b.JSON, "", "  "); err == nil {
			return string(p)
		}
	}
	return ""
}

type collector struct {
	out  []Change
	over int
	max  int
}

func (c *collector) add(ch Change) {
	if len(c.out) >= c.max {
		c.over++
		return
	}
	c.out = append(c.out, ch)
}

// walk lists where a and b differ. Arrays are compared by position: a change of order shows as the values that
// moved, and an item added in the middle shows as every item after it changing.
func walk(c *collector, path string, a, b any) {
	switch x := a.(type) {
	case map[string]any:
		if y, ok := b.(map[string]any); ok {
			for _, k := range sortedKeys(x) {
				p := path + keyPath(k)
				if yv, has := y[k]; has {
					walk(c, p, x[k], yv)
				} else {
					c.add(Change{Path: p, Kind: Removed, Old: show(x[k])})
				}
			}
			for _, k := range sortedKeys(y) {
				if _, has := x[k]; !has {
					c.add(Change{Path: path + keyPath(k), Kind: Added, New: show(y[k])})
				}
			}
			return
		}
	case []any:
		if y, ok := b.([]any); ok {
			for i := 0; i < len(x) || i < len(y); i++ {
				p := path + "[" + strconv.Itoa(i) + "]"
				switch {
				case i >= len(y):
					c.add(Change{Path: p, Kind: Removed, Old: show(x[i])})
				case i >= len(x):
					c.add(Change{Path: p, Kind: Added, New: show(y[i])})
				default:
					walk(c, p, x[i], y[i])
				}
			}
			return
		}
	}
	if show(a) != show(b) || kindOf(a) != kindOf(b) {
		c.add(Change{Path: path, Kind: Changed, Old: show(a), New: show(b)})
	}
}

func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return "number"
}

func sortedKeys(m map[string]any) []string { return slices.Sorted(maps.Keys(m)) }

// keyPath writes a key as .name, or as ["any key"] when it is not a plain name.
func keyPath(k string) string {
	if k != "" && strings.IndexFunc(k, func(r rune) bool {
		return r != '_' && r != '-' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}) < 0 {
		return "." + k
	}
	q, _ := json.Marshal(k)
	return "[" + string(q) + "]"
}

// show is a value as short text: a string as written, an object or array as JSON, cut at 80 characters.
func show(v any) string {
	if s, ok := v.(string); ok {
		return short(strconv.Quote(s))
	}
	p, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return short(string(p))
}

func short(s string) string {
	if utf8.RuneCountInString(s) <= maxShown {
		return s
	}
	return string([]rune(s)[:maxShown-1]) + "…"
}

// lineChanges compares two texts by line. The common start and end are set aside; what is between is compared
// with a longest-common-subsequence table when it is small and reported as one changed block when it is not, so a
// large body is never refused and never takes quadratic time.
func lineChanges(a, b string, limit int) ([]Change, int) {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	start := 0
	for start < len(al) && start < len(bl) && al[start] == bl[start] {
		start++
	}
	endA, endB := len(al), len(bl)
	for endA > start && endB > start && al[endA-1] == bl[endB-1] {
		endA--
		endB--
	}
	am, bm := al[start:endA], bl[start:endB]
	c := collector{max: limit}
	if len(am) == 0 && len(bm) == 0 {
		return nil, 0
	}
	if len(am)*len(bm) > lcsCells {
		c.add(Change{Path: fmt.Sprintf("lines %d–%d", start+1, max(endA, endB)), Kind: Changed, Old: fmt.Sprintf("%d lines", len(am)), New: fmt.Sprintf("%d lines", len(bm))})
		return c.out, 0
	}
	// lcs[i][j] is the length of the common subsequence of am[i:] and bm[j:].
	lcs := make([][]int32, len(am)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(bm)+1)
	}
	for i := len(am) - 1; i >= 0; i-- {
		for j := len(bm) - 1; j >= 0; j-- {
			if am[i] == bm[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(am) || j < len(bm) {
		switch {
		case i < len(am) && j < len(bm) && am[i] == bm[j]:
			i++
			j++
		case j < len(bm) && (i == len(am) || lcs[i][j+1] >= lcs[i+1][j]):
			c.add(Change{Path: fmt.Sprintf("line %d", start+j+1), Kind: Added, New: short(bm[j])})
			j++
		default:
			c.add(Change{Path: fmt.Sprintf("line %d", start+i+1), Kind: Removed, Old: short(am[i])})
			i++
		}
	}
	return c.out, c.over
}

// Values lists where two decoded JSON values differ, by path, at most limit of them; more is how many further changes
// were found and not listed. It is the comparison a body gets, for callers that compare something else.
func Values(a, b any, limit int) (changes []Change, more int) {
	c := &collector{max: limit}
	walk(c, "$", a, b)
	return c.out, c.over
}
