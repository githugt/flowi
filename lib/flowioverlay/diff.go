// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// Diff works out the operations that turn base into edited, and the parts
// of the edit an overlay cannot hold (inline hooks, a changed form or file body),
// each as a short sentence. Applying the operations to base gives edited,
// except for those parts.
func Diff(base, edited model.RequestVersion) (ops []model.OverlayOp, unsupported []string) {
	ops = append(ops, diffScalar("method", base.Method, edited.Method)...)
	ops = append(ops, diffScalar("url", base.URL, edited.URL)...)
	ops = append(ops, diffScalar("name", base.Name, edited.Name)...)
	ops = append(ops, diffScalar("docs", base.Docs, edited.Docs)...)
	ops = append(ops, diffPairs("headers", base.Headers, edited.Headers, true)...)
	ops = append(ops, diffPairs("query", base.Query, edited.Query, false)...)
	ops = append(ops, diffPairs("variables", base.Variables, edited.Variables, false)...)
	ops = append(ops, diffSettings(base.Settings, edited.Settings)...)
	bodyOps, bodyNote := diffBody(base.Body, edited.Body)
	ops = append(ops, bodyOps...)
	ops = append(ops, diffAssert(base.Assert, edited.Assert)...)
	ops = append(ops, diffExtract(base.Extract, edited.Extract)...)
	if bodyNote != "" {
		unsupported = append(unsupported, bodyNote)
	}
	hookOps, hookNote := diffHooks(base.Hooks, edited.Hooks)
	ops = append(ops, hookOps...)
	if hookNote != "" {
		unsupported = append(unsupported, hookNote)
	}
	return ops, unsupported
}

func diffScalar(path, was, now string) []model.OverlayOp {
	switch {
	case was == now:
		return nil
	case now == "" && (path == "name" || path == "docs"):
		return []model.OverlayOp{{Verb: model.OverlayRemove, Path: path, Was: was}}
	}
	return []model.OverlayOp{{Verb: model.OverlaySet, Path: path, Value: now, Was: nonEmpty(was)}}
}

// nonEmpty keeps an empty base out of Was, so a field the base never had is
// not reported as drifted when the base still does not have it.
func nonEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func diffPairs(field string, base, edited []model.KeyValue, ignoreCase bool) []model.OverlayOp {
	var ops []model.OverlayOp
	for _, e := range edited {
		b, found := findPair(base, e.Key, ignoreCase)
		if !found {
			ops = append(ops, model.OverlayOp{Verb: model.OverlaySet, Path: field + "." + e.Key, Value: e.Value})
		} else if b.Value != e.Value {
			ops = append(ops, model.OverlayOp{Verb: model.OverlaySet, Path: field + "." + e.Key, Value: e.Value, Was: b.Value})
		}
	}
	for _, b := range base {
		if _, found := findPair(edited, b.Key, ignoreCase); !found {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayRemove, Path: field + "." + b.Key, Was: b.Value})
		}
	}
	return ops
}

func findPair(list []model.KeyValue, key string, ignoreCase bool) (model.KeyValue, bool) {
	i := slices.IndexFunc(list, func(kv model.KeyValue) bool { return sameKey(kv.Key, key, ignoreCase) })
	if i < 0 {
		return model.KeyValue{}, false
	}
	return list[i], true
}

func diffSettings(base, edited model.Settings) []model.OverlayOp {
	var ops []model.OverlayOp
	for _, k := range sortedKeys(edited) {
		b, had := base[k]
		switch {
		case !had:
			ops = append(ops, model.OverlayOp{Verb: model.OverlaySet, Path: "settings." + k, Value: edited[k]})
		case !reflect.DeepEqual(b, edited[k]):
			ops = append(ops, model.OverlayOp{Verb: model.OverlaySet, Path: "settings." + k, Value: edited[k], Was: b})
		}
	}
	for _, k := range sortedKeys(base) {
		if _, kept := edited[k]; !kept {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayRemove, Path: "settings." + k, Was: base[k]})
		}
	}
	return ops
}

func sortedKeys(s model.Settings) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// diffBody handles the two body kinds an overlay can hold: JSON (as a merge
// patch) and plain text (replaced). Any other change is reported, not held.
func diffBody(base, edited model.Body) ([]model.OverlayOp, string) {
	if reflect.DeepEqual(base, edited) {
		return nil, ""
	}
	if edited.JSON != "" && base.Text == "" && base.File == "" && len(base.Form) == 0 && len(base.Multipart) == 0 {
		return diffJSONBody(base.JSON, edited.JSON)
	}
	if edited.Text != "" && base.JSON == "" && base.File == "" && len(base.Form) == 0 && len(base.Multipart) == 0 {
		return []model.OverlayOp{{Verb: model.OverlaySet, Path: "body.text", Value: edited.Text, Was: nonEmpty(base.Text)}}, ""
	}
	return nil, "the body (an overlay holds a JSON or text body, not a form, file or a change of kind)"
}

func diffJSONBody(base, edited string) ([]model.OverlayOp, string) {
	var b, e any = map[string]any{}, nil
	if strings.TrimSpace(base) != "" && json.Unmarshal([]byte(base), &b) != nil {
		return nil, "the JSON body (the base does not parse as JSON, perhaps because it uses {{…}} outside a string)"
	}
	if json.Unmarshal([]byte(edited), &e) != nil {
		return nil, "the JSON body (it does not parse as JSON, perhaps because it uses {{…}} outside a string)"
	}
	bm, bok := b.(map[string]any)
	em, eok := e.(map[string]any)
	if !bok || !eok {
		return nil, "the JSON body (an overlay patches JSON objects)"
	}
	patch := patchBetween(bm, em)
	if len(patch) == 0 {
		return nil, ""
	}
	return []model.OverlayOp{{Verb: model.OverlayMerge, Path: "body.json", Value: patch}}, ""
}

// patchBetween is the RFC 7386 merge patch that turns a into b.
func patchBetween(a, b map[string]any) map[string]any {
	patch := map[string]any{}
	for k, bv := range b {
		av, had := a[k]
		am, aObj := av.(map[string]any)
		bm, bObj := bv.(map[string]any)
		switch {
		case had && aObj && bObj:
			if sub := patchBetween(am, bm); len(sub) > 0 {
				patch[k] = sub
			}
		case !had || !reflect.DeepEqual(av, bv):
			patch[k] = bv
		}
	}
	for k := range a {
		if _, kept := b[k]; !kept {
			patch[k] = nil
		}
	}
	return patch
}

func diffAssert(base, edited []string) []model.OverlayOp {
	var ops []model.OverlayOp
	for _, line := range edited {
		if !slices.Contains(base, line) {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayAdd, Path: "assert", Value: line})
		}
	}
	for _, line := range base {
		if !slices.Contains(edited, line) {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayRemove, Path: "assert", Value: line})
		}
	}
	return ops
}

func diffExtract(base, edited []model.Extraction) []model.OverlayOp {
	var ops []model.OverlayOp
	for _, e := range edited {
		i := slices.IndexFunc(base, func(b model.Extraction) bool { return b.Name == e.Name })
		if i < 0 || base[i] != e {
			ops = append(ops, model.OverlayOp{Verb: model.OverlaySet, Path: "extract." + e.Name, Value: extractionValue(e)})
		}
	}
	for _, b := range base {
		if !slices.ContainsFunc(edited, func(e model.Extraction) bool { return e.Name == b.Name }) {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayRemove, Path: "extract." + b.Name})
		}
	}
	return ops
}

func extractionValue(x model.Extraction) map[string]any {
	m := map[string]any{"from": x.From}
	for k, v := range map[string]string{"path": x.Path, "header": x.Header, "cookie": x.Cookie, "regex": x.Regex} {
		if v != "" {
			m[k] = v
		}
	}
	return m
}
