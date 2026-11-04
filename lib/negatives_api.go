// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/synth"
)

// SuggestNegatives makes the ways version of request t can be sent wrong, as flowi overlays that are drafts until
// they are accepted (AcceptNegatives): each required field dropped, each field of the wrong type, an expired token.
// A field is required when the request's stored schema says so; with no schema every field of the body counts. A body
// that is not a JSON object (or uses {{…}} where a value goes) has no fields to break.
func (w *Workspace) SuggestNegatives(t RequestTarget, version string) ([]model.Overlay, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return nil, err
	}
	if version == "" && len(r.Versions) > 0 {
		version = r.Versions[0].Label
	}
	v, ok := versionByLabel(r, version)
	if !ok {
		return nil, fmt.Errorf("request %s has no version %q", r.Path, version)
	}
	return synth.Negatives(synth.Subject{
		Request: r.ID, Path: r.Path, Body: jsonObjectBody(v),
		Required: w.requiredFields(r, version), Authorization: sendsAuthorization(v),
	}), nil
}

// jsonObjectBody is the version's body as a JSON object, or nil when it is not one.
func jsonObjectBody(v model.RequestVersion) map[string]any {
	if strings.TrimSpace(v.Body.JSON) == "" {
		return nil
	}
	d := json.NewDecoder(strings.NewReader(v.Body.JSON))
	d.UseNumber()
	var body map[string]any
	if d.Decode(&body) != nil {
		return nil
	}
	return body
}

// requiredFields reads the required list of the request-body schema: nil when no schema is kept, and empty when the
// schema says nothing is required.
func (w *Workspace) requiredFields(r model.Request, version string) []string {
	file, err := schemaFile(r.File, version, SchemaPart{Kind: "request"})
	if err != nil {
		return nil
	}
	schema, _, found := readSchema(file)
	if !found {
		return nil
	}
	required := []string{}
	list, _ := schema["required"].([]any)
	for _, item := range list {
		if name, ok := item.(string); ok {
			required = append(required, name)
		}
	}
	return required
}

func sendsAuthorization(v model.RequestVersion) bool {
	for _, h := range v.Headers {
		if strings.EqualFold(h.Key, "authorization") {
			return true
		}
	}
	return false
}

// AcceptNegatives saves the overlays in the collection and returns the names it saved. An overlay whose name is
// already taken is left as it is and its name returned in skipped: the person may have edited it.
func (w *Workspace) AcceptNegatives(collection model.ID, overlays []model.Overlay) (saved, skipped []string, err error) {
	existing, err := w.Overlays(collection)
	if err != nil {
		return nil, nil, err
	}
	taken := map[string]bool{}
	for _, ov := range existing {
		taken[ov.Name] = true
	}
	for _, ov := range overlays {
		if taken[ov.Name] {
			skipped = append(skipped, ov.Name)
			continue
		}
		if err := w.WriteOverlay(collection, ov, SaveOptions{}); err != nil {
			return saved, skipped, fmt.Errorf("saving overlay %s: %w", ov.Name, err)
		}
		saved = append(saved, ov.Name)
	}
	return saved, skipped, nil
}
