// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// Problem is an operation that did not fit the version it was applied to,
// or that applied although the base changed since the overlay was saved.
type Problem struct {
	Op     model.OverlayOp
	Reason string
	// Drift marks an operation that was applied: the base no longer holds
	// what it held when the overlay was saved. Otherwise the operation was
	// left out.
	Drift bool
}

// Describe names the operation the way a user reads it (see Describe).
func (p Problem) Describe() string { return Describe(p.Op) }

// Apply returns v with ops applied in order. An operation that does not fit
// (it removes a header the version no longer has) is left out and reported;
// the rest still apply. v is not changed.
func Apply(v model.RequestVersion, ops []model.OverlayOp) (model.RequestVersion, []Problem) {
	v = detach(v)
	var problems []Problem
	for _, op := range ops {
		problems = append(problems, applyOne(&v, op)...)
	}
	return v, problems
}

// detach copies the slices and maps Apply edits, so the caller's version
// stays as it was.
func detach(v model.RequestVersion) model.RequestVersion {
	v.Headers = slices.Clone(v.Headers)
	v.Query = slices.Clone(v.Query)
	v.Variables = slices.Clone(v.Variables)
	v.Assert = slices.Clone(v.Assert)
	v.Extract = slices.Clone(v.Extract)
	v.Hooks = cloneHooks(v.Hooks)
	if v.Settings != nil {
		s := make(model.Settings, len(v.Settings))
		for k, x := range v.Settings {
			s[k] = x
		}
		v.Settings = s
	}
	return v
}

func applyOne(v *model.RequestVersion, op model.OverlayOp) []Problem {
	field, rest, _ := strings.Cut(op.Path, ".")
	h, ok := handlerFor(field)
	if !ok {
		return []Problem{{Op: op, Reason: fmt.Sprintf("%q is not a field overlays can change", field)}}
	}
	if reason := notUsed(*v, field); reason != "" {
		return []Problem{{Op: op, Reason: reason}}
	}
	reason, drift := h(v, op, rest)
	switch {
	case reason != "":
		return []Problem{{Op: op, Reason: reason}}
	case drift != "":
		return []Problem{{Op: op, Reason: drift, Drift: true}}
	}
	return nil
}

// notUsed explains why a field means nothing to the kind of request v is.
func notUsed(v model.RequestVersion, field string) string {
	switch field {
	case "method", "url", "query", "body":
		if v.Family() != model.FamilyHTTP {
			return "this kind of request does not use " + field
		}
	}
	return ""
}

// handler applies op to one field. rest is the path after the field name.
// It returns why the operation did not fit, or a drift note when it applied.
type handler func(v *model.RequestVersion, op model.OverlayOp, rest string) (reason, drift string)

// handlerFor returns the handler of an overlay field.
func handlerFor(field string) (handler, bool) {
	switch field {
	case "method":
		return scalar(func(v *model.RequestVersion) *string { return &v.Method }, false), true
	case "url":
		return scalar(func(v *model.RequestVersion) *string { return &v.URL }, false), true
	case "name":
		return scalar(func(v *model.RequestVersion) *string { return &v.Name }, true), true
	case "docs":
		return scalar(func(v *model.RequestVersion) *string { return &v.Docs }, true), true
	case "headers":
		return pairs(func(v *model.RequestVersion) *[]model.KeyValue { return &v.Headers }, true), true
	case "query":
		return pairs(func(v *model.RequestVersion) *[]model.KeyValue { return &v.Query }, false), true
	case "variables":
		return pairs(func(v *model.RequestVersion) *[]model.KeyValue { return &v.Variables }, false), true
	case "settings":
		return applySetting, true
	case "body":
		return applyBody, true
	case "assert":
		return applyAssert, true
	case "extract":
		return applyExtract, true
	case "hooks":
		return applyHooks, true
	}
	return nil, false
}

func text(x any) string {
	if x == nil {
		return ""
	}
	if s, ok := x.(string); ok {
		return s
	}
	return fmt.Sprint(x)
}

func driftNote(op model.OverlayOp, now string, present bool) string {
	if op.Was == nil {
		return ""
	}
	if !present {
		return "the base no longer has it (it had " + quote(text(op.Was)) + ")"
	}
	if text(op.Was) != now {
		return "the base changed from " + quote(text(op.Was)) + " to " + quote(now)
	}
	return ""
}

func quote(s string) string { return `"` + s + `"` }

func scalar(field func(*model.RequestVersion) *string, removable bool) handler {
	return func(v *model.RequestVersion, op model.OverlayOp, rest string) (string, string) {
		if rest != "" {
			return "has no part named " + rest, ""
		}
		p := field(v)
		switch op.Verb {
		case model.OverlaySet:
			drift := driftNote(op, *p, true)
			*p = text(op.Value)
			return "", drift
		case model.OverlayRemove:
			if !removable {
				return "cannot be removed", ""
			}
			*p = ""
			return "", ""
		}
		return "does not take " + op.Verb, ""
	}
}

func pairs(list func(*model.RequestVersion) *[]model.KeyValue, ignoreCase bool) handler {
	return func(v *model.RequestVersion, op model.OverlayOp, key string) (string, string) {
		if key == "" {
			return "needs a name after the dot", ""
		}
		l := list(v)
		i := slices.IndexFunc(*l, func(kv model.KeyValue) bool { return sameKey(kv.Key, key, ignoreCase) })
		switch op.Verb {
		case model.OverlaySet:
			if i < 0 {
				*l = append(*l, model.KeyValue{Key: key, Value: text(op.Value)})
				return "", driftNote(op, "", false)
			}
			drift := driftNote(op, (*l)[i].Value, true)
			(*l)[i].Value = text(op.Value)
			return "", drift
		case model.OverlayRemove:
			if i < 0 {
				return key + " is not in this version", ""
			}
			*l = slices.Delete(*l, i, i+1)
			return "", ""
		}
		return "does not take " + op.Verb, ""
	}
}

func sameKey(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func applySetting(v *model.RequestVersion, op model.OverlayOp, key string) (string, string) {
	if key == "" {
		return "needs a setting name after the dot", ""
	}
	switch op.Verb {
	case model.OverlaySet:
		if v.Settings == nil {
			v.Settings = model.Settings{}
		}
		cur, present := v.Settings[key]
		drift := ""
		if present {
			drift = driftNote(op, text(cur), true)
		}
		v.Settings[key] = op.Value
		return "", drift
	case model.OverlayRemove:
		if _, ok := v.Settings[key]; !ok {
			return "setting " + key + " is not set in this version", ""
		}
		delete(v.Settings, key)
		return "", ""
	}
	return "does not take " + op.Verb, ""
}

func applyAssert(v *model.RequestVersion, op model.OverlayOp, rest string) (string, string) {
	if rest != "" {
		return "has no part named " + rest, ""
	}
	line := text(op.Value)
	i := slices.Index(v.Assert, line)
	switch op.Verb {
	case model.OverlayAdd:
		if i < 0 {
			v.Assert = append(v.Assert, line)
		}
		return "", ""
	case model.OverlayRemove:
		if i < 0 {
			return "the check " + quote(line) + " is not in this version", ""
		}
		v.Assert = slices.Delete(v.Assert, i, i+1)
		return "", ""
	}
	return "does not take " + op.Verb, ""
}

func applyExtract(v *model.RequestVersion, op model.OverlayOp, name string) (string, string) {
	if name == "" {
		return "needs a name after the dot", ""
	}
	i := slices.IndexFunc(v.Extract, func(x model.Extraction) bool { return x.Name == name })
	switch op.Verb {
	case model.OverlaySet:
		x, err := extractionOf(name, op.Value)
		if err != nil {
			return err.Error(), ""
		}
		if i < 0 {
			v.Extract = append(v.Extract, x)
		} else {
			v.Extract[i] = x
		}
		return "", ""
	case model.OverlayRemove:
		if i < 0 {
			return "extraction " + name + " is not in this version", ""
		}
		v.Extract = slices.Delete(v.Extract, i, i+1)
		return "", ""
	}
	return "does not take " + op.Verb, ""
}

func applyBody(v *model.RequestVersion, op model.OverlayOp, kind string) (string, string) {
	switch {
	case kind == "json" && op.Verb == model.OverlayMerge:
		return mergeJSONBody(v, op.Value)
	case kind == "text" && op.Verb == model.OverlaySet:
		if v.Body.JSON != "" || v.Body.File != "" || len(v.Body.Form) > 0 || len(v.Body.Multipart) > 0 {
			return "the version's body is not plain text", ""
		}
		v.Body.Text = text(op.Value)
		return "", ""
	}
	return "does not take " + op.Verb + " on body." + kind, ""
}

// mergeJSONBody applies an RFC 7386 merge patch to the version's JSON body.
// Only a JSON object can be patched; any other body is a problem, because
// replacing it whole would be a different kind of change.
func mergeJSONBody(v *model.RequestVersion, patch any) (string, string) {
	if v.Body.Text != "" || v.Body.File != "" || len(v.Body.Form) > 0 || len(v.Body.Multipart) > 0 {
		return "the version's body is not JSON", ""
	}
	var base any = map[string]any{}
	if strings.TrimSpace(v.Body.JSON) != "" {
		if err := json.Unmarshal([]byte(v.Body.JSON), &base); err != nil {
			return "the version's JSON body does not parse (it may use {{…}} outside a string)", ""
		}
	}
	if _, ok := base.(map[string]any); !ok {
		return "the version's JSON body is not an object", ""
	}
	out, err := json.MarshalIndent(MergePatch(base, normalise(patch)), "", "  ")
	if err != nil {
		return "the merged body cannot be written: " + err.Error(), ""
	}
	v.Body.JSON = string(out)
	return "", ""
}

// MergePatch applies an RFC 7386 JSON merge patch: objects merge key by
// key, null removes a key, anything else replaces.
func MergePatch(base, patch any) any {
	p, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	b, ok := base.(map[string]any)
	if !ok {
		b = map[string]any{}
	}
	out := make(map[string]any, len(b))
	for k, x := range b {
		out[k] = x
	}
	for k, x := range p {
		if x == nil {
			delete(out, k)
			continue
		}
		out[k] = MergePatch(out[k], x)
	}
	return out
}

// normalise turns the map[any]any shapes some YAML decoders produce into
// map[string]any, so a patch read from a file merges like decoded JSON.
func normalise(x any) any {
	switch t := x.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v := range t {
			out[k] = normalise(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, v := range t {
			out[fmt.Sprint(k)] = normalise(v)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			out[i] = normalise(v)
		}
		return out
	}
	return x
}

func extractionOf(name string, value any) (model.Extraction, error) {
	m, ok := normalise(value).(map[string]any)
	if !ok {
		return model.Extraction{}, fmt.Errorf("an extraction needs from, and path, header, cookie or regex")
	}
	x := model.Extraction{Name: name}
	for k, v := range m {
		s := text(v)
		switch k {
		case "from":
			x.From = s
		case "path":
			x.Path = s
		case "header":
			x.Header = s
		case "cookie":
			x.Cookie = s
		case "regex":
			x.Regex = s
		default:
			return model.Extraction{}, fmt.Errorf("an extraction has no %q", k)
		}
	}
	if x.From == "" {
		return model.Extraction{}, fmt.Errorf("an extraction needs from")
	}
	return x, nil
}

// verbsOf lists the verbs a field takes (nil for an unknown field).
func verbsOf(field string) []string {
	switch field {
	case "method", "url":
		return []string{model.OverlaySet}
	case "name", "docs", "headers", "query", "variables", "settings", "extract":
		return []string{model.OverlaySet, model.OverlayRemove}
	case "body":
		return []string{model.OverlayMerge, model.OverlaySet}
	case "assert", "hooks":
		return []string{model.OverlayAdd, model.OverlayRemove}
	}
	return nil
}

// CheckOp reports why op can never apply to any version: an unknown field,
// a verb the field does not take, or a missing name after the dot. It does
// not look at any version, so it cannot tell a header that is absent.
func CheckOp(op model.OverlayOp) error {
	field, rest, _ := strings.Cut(op.Path, ".")
	verbs := verbsOf(field)
	if verbs == nil {
		return fmt.Errorf("%q is not a field overlays can change", field)
	}
	if !slices.Contains(verbs, op.Verb) {
		return fmt.Errorf("%s does not take %s", field, op.Verb)
	}
	needsName := map[string]bool{"headers": true, "query": true, "variables": true, "settings": true, "extract": true}
	if needsName[field] && rest == "" {
		return fmt.Errorf("%s needs a name after the dot", field)
	}
	if _, ok := hookPhase(rest); field == "hooks" && !ok {
		return fmt.Errorf("hooks is hooks.pre, hooks.post or hooks.message")
	}
	if field == "body" && rest != "json" && rest != "text" {
		return fmt.Errorf("body is body.json or body.text")
	}
	return nil
}
