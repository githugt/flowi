// SPDX-License-Identifier: Apache-2.0

package synth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// Subject is a request Negatives breaks.
type Subject struct {
	Request model.ID
	Path    string // a readable hint at the request, kept in the overlay
	// Body is the version's JSON body when it is an object; nothing else has fields to break.
	Body map[string]any
	// Required are the fields the request's schema requires. Nil means no schema is kept, and every field of Body
	// is treated as required: the body is the only evidence of what the API wants.
	Required []string
	// Authorization says the version sends an Authorization header.
	Authorization bool
}

// staleToken is a bearer token that expired in 1970: a JWT with the claim {"exp":1}, unsigned.
func staleToken() string {
	return "Bearer " + strings.Join([]string{
		base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)),
		base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1}`)),
		"x",
	}, ".")
}

// Negatives makes one overlay for each way a request can be sent wrong: with a required field dropped (no-<field>),
// with a field of the wrong type (bad-<field>), and with an expired token (stale-token). Each is named so it can be
// run on its own, or all at once in a matrix of overlays; the expectation is the person's to add, since only they
// know whether the API should answer 400 or 422. Deterministic: the same subject gives the same overlays in the same
// order.
func Negatives(s Subject) []model.Overlay {
	var out []model.Overlay
	used := map[string]bool{}
	add := func(name, description string, ops []model.OverlayOp) {
		name = uniqueName(used, name)
		out = append(out, model.Overlay{
			Name: name, Description: description,
			Requests: []model.RequestOverlay{{Request: s.Request, Path: s.Path, Ops: ops}},
		})
	}
	for _, field := range sortedFields(s.Body) {
		add("bad-"+overlayWord(field), fmt.Sprintf("%s sent as the wrong type", field), mergeBody(field, wrongValue(s.Body[field])))
	}
	for _, field := range dropCandidates(s) {
		add("no-"+overlayWord(field), fmt.Sprintf("%s missing", field), mergeBody(field, nil))
	}
	if s.Authorization {
		add("stale-token", "an expired token", []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.Authorization", Value: staleToken()}})
	}
	slices.SortStableFunc(out, func(a, b model.Overlay) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// dropCandidates are the fields worth removing: the required ones that the body actually sends.
func dropCandidates(s Subject) []string {
	fields := sortedFields(s.Body)
	if s.Required == nil {
		return fields
	}
	return slices.DeleteFunc(fields, func(f string) bool { return !slices.Contains(s.Required, f) })
}

func sortedFields(body map[string]any) []string {
	fields := make([]string, 0, len(body))
	for f := range body {
		fields = append(fields, f)
	}
	slices.Sort(fields)
	return fields
}

// mergeBody is the merge patch that sets field to value (nil removes it).
func mergeBody(field string, value any) []model.OverlayOp {
	return []model.OverlayOp{{Verb: model.OverlayMerge, Path: "body.json", Value: map[string]any{field: value}}}
}

// wrongValue is a value of a different JSON type than v: a string becomes a number, a number a string, and so on.
func wrongValue(v any) any {
	switch v.(type) {
	case string:
		return 12345
	case json.Number, float64, int:
		return "not-a-number"
	case bool:
		return "yes"
	}
	return "oops"
}

// overlayWord keeps the characters an overlay name allows and turns the rest into '-'.
func overlayWord(field string) string {
	var b strings.Builder
	for _, r := range field {
		if r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

// uniqueName returns name, or name-2, name-3 … when two fields flattened to the same word.
func uniqueName(used map[string]bool, name string) string {
	candidate := name
	for n := 2; used[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", name, n)
	}
	used[candidate] = true
	return candidate
}
