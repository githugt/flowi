// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
)

// WireChild is one thing inside the value a wire would take, to click: Path is what to take to read it.
type WireChild struct {
	Name   string
	Path   string
	Sample string // the value shortened, or what a container holds ("3 keys")
	Branch bool   // it holds more to click into
}

// WirePreview is what a wire taking Path of a step would pass, by the step's last run.
type WirePreview struct {
	Ran      bool // the step has run: without a run there is nothing to show
	Found    bool // the last run's values have something at Path
	Text     string
	Children []WireChild
}

const (
	wireTextRunes = 200 // how much of the value shows whole
	wireSample    = 40  // how much of a child shows beside its name
	wireChildren  = 30
)

// PreviewWire reads Path (body.token, headers.etag, items[0].id; empty for everything) in the values step detail d made
// in the last run, so the dialog can show "→ eyJhbGc…" before the wire is made and a tree to click through.
// Before: "Take" was free text with no sign of what it would pass.
//
// The values are the step's outputs plus, for a call, its status, its body (parsed when it is JSON) and its headers;
// a credential header's value is hidden like everywhere else.
func PreviewWire(d NodeDetail, ran bool, path string) WirePreview {
	if !ran {
		return WirePreview{}
	}
	value, found := wireWalk(stepValues(d), path)
	out := WirePreview{Ran: true, Found: found}
	if !found {
		return out
	}
	out.Text = wireText(value, wireTextRunes)
	out.Children = wireChildrenOf(value, path)
	return out
}

// stepValues are what a later step can read of this step, from the last run.
func stepValues(d NodeDetail) map[string]any {
	out := map[string]any{}
	for k, v := range d.Outputs {
		out[k] = v
	}
	rec := d.Record
	if rec == nil {
		return out
	}
	if _, ok := out["status"]; !ok && rec.Status > 0 {
		out["status"] = float64(rec.Status)
	}
	if _, ok := out["body"]; !ok {
		out["body"] = bodyValue(rec.Body)
	}
	if _, ok := out["headers"]; !ok && len(rec.Header) > 0 {
		headers := map[string]any{}
		for _, row := range maskedHeaderRows(rec.Header) {
			name := strings.ToLower(row.Key)
			if old, ok := headers[name].(string); ok {
				row.Value = old + ", " + row.Value
			}
			headers[name] = row.Value
		}
		out["headers"] = headers
	}
	return out
}

// bodyValue is the parsed JSON of a body, else its text.
func bodyValue(b engine.Body) any {
	if b.JSON != nil {
		return b.JSON
	}
	return b.Text
}

// walkValue follows path (names joined by dots, [n] or .n for a list's item) into v.
func wireWalk(v any, path string) (any, bool) {
	for _, seg := range pathSegments(path) {
		switch cur := v.(type) {
		case map[string]any:
			next, ok := cur[seg]
			if !ok {
				next, ok = foldedKey(cur, seg)
			}
			if !ok {
				return nil, false
			}
			v = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(cur) {
				return nil, false
			}
			v = cur[i]
		default:
			return nil, false
		}
	}
	return v, true
}

// foldedKey finds key in m ignoring case (header names are).
func foldedKey(m map[string]any, key string) (any, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// pathSegments splits "items[0].id" into items, 0, id.
func pathSegments(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	path = strings.NewReplacer("[", ".", "]", "").Replace(path)
	var out []string
	for _, s := range strings.Split(path, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// wireText is v as it would be written into a field: text as it is, anything else as JSON; cut to max runes.
func wireText(v any, max int) string {
	var text string
	switch t := v.(type) {
	case string:
		text = t
	case nil:
		text = "null"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			text = fmt.Sprint(t)
		} else {
			text = string(b)
		}
	}
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	return string([]rune(text)[:max]) + "…"
}

// childrenOf lists what is inside v to click: a map's keys (sorted), a list's first items.
func wireChildrenOf(v any, path string) []WireChild {
	var out []WireChild
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys[:min(len(keys), wireChildren)] {
			out = append(out, wireChild(k, wireJoin(path, k), t[k]))
		}
	case []any:
		for i := range t[:min(len(t), wireChildren)] {
			out = append(out, wireChild("["+strconv.Itoa(i)+"]", path+"["+strconv.Itoa(i)+"]", t[i]))
		}
	}
	return out
}

func wireJoin(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func wireChild(name, path string, v any) WireChild {
	c := WireChild{Name: name, Path: path}
	switch t := v.(type) {
	case map[string]any:
		c.Sample, c.Branch = plural(len(t), "key"), len(t) > 0
	case []any:
		c.Sample, c.Branch = plural(len(t), "item"), len(t) > 0
	default:
		c.Sample = wireText(v, wireSample)
	}
	return c
}
