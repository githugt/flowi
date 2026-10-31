// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/openapi"
	"github.com/githugt/flowi-lib/specoverlay"
	"github.com/githugt/flowi-lib/store"
)

// A request imported from a kept spec can save its edits to the spec, as an
// OpenAPI Overlay beside it (<name>.edits.overlay.yaml), so a re-import, a
// colleague's import or another tool reading the spec sees them too. Only
// what the importer reads back is written: the operation's description
// (the version's docs), the JSON request body's example, and query and
// header parameters added or removed. The rest stays in Flowi, which a
// re-import keeps (mergeReimport); OverlayPlan.NotCarried says why for each.

// ErrNotFromSpec is returned for a request no kept spec makes.
var ErrNotFromSpec = errors.New("the request was not imported from a spec kept in the collection")

// OverlayChange is one change a saved edit makes to the spec.
type OverlayChange struct {
	What   string // what changes, said plainly: "the docs", "query parameter page is added"
	Target string // the JSONPath the action points at
	Update any    // the action's update; nil for a removal
	Remove bool
	// Saved says the overlay already has it: saving again changes nothing
	// for it. Unsaved changes are the ones to offer first.
	Saved bool

	param map[string]any // an added parameter: grouped with the others when the operation has no parameters yet
}

// OverlayNote is an edit the overlay does not carry, and why.
type OverlayNote struct {
	What string
	Why  string
}

// OverlayPlan is what saving a request version to its spec's overlay would
// write. Changes are chosen by index in ApplySpecOverlay and YAML.
type OverlayPlan struct {
	Spec       KeptSpec
	Request    string // the request's path
	Label      string // the version
	Operation  string // the JSONPath of the operation
	Changes    []OverlayChange
	NotCarried []OverlayNote

	others    []overlayAction // the overlay's actions for other operations, kept as they are
	hasParams bool            // the operation already has a parameters list
}

// overlayDoc is an OpenAPI Overlay document.
type overlayDoc struct {
	Overlay string          `yaml:"overlay"`
	Info    overlayInfo     `yaml:"info"`
	Actions []overlayAction `yaml:"actions"`
}

type overlayInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type overlayAction struct {
	Target      string `yaml:"target"`
	Description string `yaml:"description,omitempty"`
	Update      any    `yaml:"update,omitempty"`
	Remove      bool   `yaml:"remove,omitempty"`
}

// specMatch is the kept spec a request comes from, read and overlaid.
type specMatch struct {
	kept     KeptSpec
	spec     []byte
	overlays [][]byte    // the kept overlays, without the edits
	edits    *overlayDoc // nil when there is no edits overlay yet
	op       openapi.OperationRef
}

// PreviewSpecOverlay works out what saving version v of the request to its
// spec's overlay would change. v is the version as it is being saved (the
// edit, when there is one).
func (w *Workspace) PreviewSpecOverlay(t RequestTarget, v model.RequestVersion) (OverlayPlan, error) {
	c, req, err := w.requestFor(t)
	if err != nil {
		return OverlayPlan{}, err
	}
	m, err := findSpec(c.Root, req.Path, v.Label)
	if err != nil {
		return OverlayPlan{}, err
	}
	target := operationTarget(m.op)
	others := m.otherActions(target)
	baseDoc, err := m.overlaid(others)
	if err != nil {
		return OverlayPlan{}, err
	}
	base, op, err := versionIn(baseDoc, m.kept.refFiles(), req.Path, v.Label)
	if err != nil {
		return OverlayPlan{}, err
	}
	p := OverlayPlan{Spec: m.kept, Request: req.Path, Label: v.Label, Operation: target, others: others}
	_, p.hasParams = op.Op["parameters"].([]any)
	p.Changes, p.NotCarried = diffForOverlay(base, v, op, target)
	markSaved(m, req.Path, v, target, p.Changes)
	return p, nil
}

// ApplySpecOverlay writes the chosen changes of p to the spec's edits
// overlay, in place of what it said about this operation before. The
// overlay is removed when it no longer says anything.
func (w *Workspace) ApplySpecOverlay(p OverlayPlan, chosen []int) error {
	doc := p.document(chosen)
	if len(doc.Actions) == 0 {
		if err := os.Remove(p.Spec.Edits); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", p.Spec.Edits, err)
		}
		return nil
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", p.Spec.Edits, err)
	}
	return store.Apply([]store.FileChange{{File: p.Spec.Edits, New: data}})
}

// YAML is the edits overlay as it would be written with the chosen changes.
func (p OverlayPlan) YAML(chosen []int) (string, error) {
	data, err := yaml.Marshal(p.document(chosen))
	if err != nil {
		return "", fmt.Errorf("encoding the overlay: %w", err)
	}
	return string(data), nil
}

// document is the edits overlay with this operation's chosen changes after
// the other operations' actions. Added parameters go in one action when the
// operation has no parameters list yet, which that action creates.
func (p OverlayPlan) document(chosen []int) overlayDoc {
	doc := overlayDoc{Overlay: "1.0.0", Info: editsInfo(), Actions: slices.Clone(p.others)}
	var added []any
	var addedWhat []string
	for _, i := range chosen {
		if i < 0 || i >= len(p.Changes) {
			continue
		}
		ch := p.Changes[i]
		if ch.param != nil && !p.hasParams {
			added, addedWhat = append(added, ch.param), append(addedWhat, ch.What)
			continue
		}
		doc.Actions = append(doc.Actions, overlayAction{Target: ch.Target, Description: ch.What, Update: ch.Update, Remove: ch.Remove})
	}
	if len(added) > 0 {
		doc.Actions = append(doc.Actions, overlayAction{Target: p.Operation, Description: strings.Join(addedWhat, "; "),
			Update: map[string]any{"parameters": added}})
	}
	return doc
}

// markSaved flags the changes the edits overlay already makes: they do
// not show up against the spec with the overlay as it is.
func markSaved(m specMatch, requestPath string, v model.RequestVersion, target string, changes []OverlayChange) {
	doc, err := m.overlaid(m.editActions())
	if err != nil {
		return // nothing can be told saved: all are offered
	}
	now, op, err := versionIn(doc, m.kept.refFiles(), requestPath, v.Label)
	if err != nil {
		return
	}
	pending, _ := diffForOverlay(now, v, op, target)
	for i := range changes {
		changes[i].Saved = !slices.ContainsFunc(pending, func(c OverlayChange) bool { return c.What == changes[i].What })
	}
}

// findSpec finds the kept spec whose import, with all of its overlays, has
// the request and version.
func findSpec(root, requestPath, label string) (specMatch, error) {
	specs, err := keptSpecs(root)
	if err != nil {
		return specMatch{}, err
	}
	for _, kept := range specs {
		m, err := readSpec(kept)
		if err != nil {
			continue // a kept spec that no longer reads is not the one this request comes from
		}
		full, err := m.overlaid(m.editActions())
		if err != nil {
			continue
		}
		ops, err := openapi.OperationsOf(full)
		if errors.Is(err, openapi.ErrSwagger2) {
			return specMatch{}, err
		}
		i := slices.IndexFunc(ops, func(o openapi.OperationRef) bool { return o.RequestPath == requestPath && o.Label == label })
		if err == nil && i >= 0 {
			m.op = ops[i]
			return m, nil
		}
	}
	return specMatch{}, ErrNotFromSpec
}

func readSpec(kept KeptSpec) (specMatch, error) {
	spec, err := os.ReadFile(kept.File)
	if err != nil {
		return specMatch{}, fmt.Errorf("reading %s: %w", kept.File, err)
	}
	overlays, edits, err := kept.readOverlays()
	if err != nil {
		return specMatch{}, err
	}
	m := specMatch{kept: kept, spec: spec, overlays: overlays}
	if edits != nil {
		var doc overlayDoc
		if err := yaml.Unmarshal(edits, &doc); err != nil {
			return specMatch{}, fmt.Errorf("reading %s: %w", kept.Edits, err)
		}
		m.edits = &doc
	}
	return m, nil
}

func (m specMatch) editActions() []overlayAction {
	if m.edits == nil {
		return nil
	}
	return m.edits.Actions
}

// otherActions are the edits overlay's actions that are not about the
// operation at target: saving replaces those that are.
func (m specMatch) otherActions(target string) []overlayAction {
	return slices.DeleteFunc(slices.Clone(m.editActions()), func(a overlayAction) bool { return strings.HasPrefix(a.Target, target) })
}

// overlaid is the spec with its kept overlays and then the given edits.
func (m specMatch) overlaid(edits []overlayAction) ([]byte, error) {
	overlays := slices.Clone(m.overlays)
	if len(edits) > 0 {
		data, err := encodeOverlay(edits)
		if err != nil {
			return nil, err
		}
		overlays = append(overlays, data)
	}
	out, diags := specoverlay.Apply(m.spec, overlays)
	if diags.HasErrors() {
		return nil, diags.Err()
	}
	return out, nil
}

// editsInfo heads the overlay Flowi writes.
func editsInfo() overlayInfo { return overlayInfo{Title: "Edits saved from Flowi", Version: "1.0.0"} }

func encodeOverlay(actions []overlayAction) ([]byte, error) {
	data, err := yaml.Marshal(overlayDoc{Overlay: "1.0.0", Info: editsInfo(), Actions: actions})
	if err != nil {
		return nil, fmt.Errorf("encoding the overlay: %w", err)
	}
	return data, nil
}

// versionIn imports doc and returns the request's version and the
// operation it comes from.
func versionIn(doc []byte, files fs.FS, requestPath, label string) (model.RequestVersion, openapi.OperationRef, error) {
	r, diags := openapi.Import(doc, openapi.Options{NewID: func() model.ID { return "" }, Files: files})
	if diags.HasErrors() {
		return model.RequestVersion{}, openapi.OperationRef{}, diags.Err()
	}
	ops, err := openapi.OperationsOf(doc)
	if err != nil {
		return model.RequestVersion{}, openapi.OperationRef{}, err
	}
	i := slices.IndexFunc(r.Requests, func(x model.Request) bool { return x.Path == requestPath })
	j := slices.IndexFunc(ops, func(o openapi.OperationRef) bool { return o.RequestPath == requestPath && o.Label == label })
	if i < 0 || j < 0 {
		return model.RequestVersion{}, openapi.OperationRef{}, ErrNotFromSpec
	}
	v, ok := versionByLabel(r.Requests[i], label)
	if !ok {
		return model.RequestVersion{}, openapi.OperationRef{}, ErrNotFromSpec
	}
	return v, ops[j], nil
}

// operationTarget points at an operation: by its operationId when it has
// one, which survives the path being renamed, else by path and method.
func operationTarget(op openapi.OperationRef) string {
	if op.OperationID != "" {
		return "$.paths.*[?(@.operationId == " + quoted(op.OperationID) + ")]"
	}
	return "$.paths[" + quoted(op.Path) + "]." + op.Method
}

// quoted is s as a single-quoted JSONPath string.
func quoted(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

const flowiOnly = "only Flowi fills in {{…}} values; Flowi keeps it"

// diffForOverlay compares the version being saved with what the spec
// makes of it and says what the overlay can carry.
func diffForOverlay(base, v model.RequestVersion, op openapi.OperationRef, target string) ([]OverlayChange, []OverlayNote) {
	var changes []OverlayChange
	var notes []OverlayNote
	note := func(what, why string) { notes = append(notes, OverlayNote{What: what, Why: why}) }
	if v.Method != base.Method || v.URL != base.URL {
		note("the method or URL", "that makes it another operation: change the spec itself; Flowi keeps it")
	}
	if v.Docs != base.Docs {
		if strings.Contains(v.Docs, "{{") {
			note("the docs", flowiOnly)
		} else {
			changes = append(changes, OverlayChange{What: "the docs", Target: target, Update: map[string]any{"description": v.Docs}})
		}
	}
	if !sameBody(base.Body, v.Body) {
		ch, why := bodyChange(v.Body, op, target)
		if why != "" {
			note("the body", why)
		} else {
			changes = append(changes, ch)
		}
	}
	for _, kind := range []struct {
		in        string
		ours, old []model.KeyValue
	}{{"query", v.Query, base.Query}, {"header", v.Headers, base.Headers}} {
		c, n := paramChanges(kind.in, kind.old, kind.ours, op, target)
		changes, notes = append(changes, c...), append(notes, n...)
	}
	return changes, append(notes, flowiOnlyParts(v)...)
}

// sameBody compares bodies; JSON by value, since the spec reformats it.
func sameBody(a, b model.Body) bool {
	if a.JSON != "" && b.JSON != "" {
		var x, y any
		if json.Unmarshal([]byte(a.JSON), &x) == nil && json.Unmarshal([]byte(b.JSON), &y) == nil {
			return reflect.DeepEqual(x, y)
		}
	}
	return reflect.DeepEqual(a, b)
}

// bodyChange is the request body example as an overlay change, or why the
// body cannot be one.
func bodyChange(b model.Body, op openapi.OperationRef, target string) (OverlayChange, string) {
	var parsed any
	switch {
	case b.JSON == "":
		return OverlayChange{}, "only a JSON body can be saved to the spec; Flowi keeps it"
	case strings.Contains(b.JSON, "{{"):
		return OverlayChange{}, flowiOnly
	case json.Unmarshal([]byte(b.JSON), &parsed) != nil:
		return OverlayChange{}, "it is not valid JSON; Flowi keeps it"
	}
	rb, _ := op.Op["requestBody"].(map[string]any)
	content, _ := rb["content"].(map[string]any)
	mt, hasJSON := content["application/json"].(map[string]any)
	switch {
	case rb["$ref"] != nil:
		return OverlayChange{}, "the spec shares this request body with other operations ($ref); Flowi keeps it"
	case !hasJSON:
		return OverlayChange{}, "the spec has no JSON request body for this operation; Flowi keeps it"
	case mt["examples"] != nil:
		return OverlayChange{}, "the spec gives named examples, and Flowi sets only one; Flowi keeps it"
	}
	return OverlayChange{What: "the body example", Target: target + ".requestBody.content['application/json']", Update: map[string]any{"example": parsed}}, ""
}

// ignoredHeaders are headers OpenAPI does not allow as parameters: the
// spec's media types and security say them.
func ignoredHeaders() []string { return []string{"accept", "content-type", "authorization"} }

// paramChanges are the parameters of one kind (query or header) added or
// removed since the spec's version.
func paramChanges(in string, old, ours []model.KeyValue, op openapi.OperationRef, target string) ([]OverlayChange, []OverlayNote) {
	var changes []OverlayChange
	var notes []OverlayNote
	for _, p := range ours {
		label := in + " parameter " + p.Key
		oldValue, had := pairValue(old, p.Key)
		switch {
		case had && oldValue != p.Value:
			notes = append(notes, OverlayNote{What: label, Why: "the spec says which parameters there are, not their values; Flowi keeps the value"})
		case had:
		case in == "header" && slices.Contains(ignoredHeaders(), strings.ToLower(p.Key)):
			notes = append(notes, OverlayNote{What: label, Why: "the spec says it through its media types or security; Flowi keeps it"})
		default:
			param := map[string]any{"name": p.Key, "in": in, "schema": map[string]any{"type": "string"}}
			if p.Value != "" && !strings.Contains(p.Value, "{{") {
				param["example"] = p.Value
			}
			changes = append(changes, OverlayChange{What: label + " is added", Target: target + ".parameters", Update: param, param: param})
		}
	}
	for _, p := range old {
		if _, kept := pairValue(ours, p.Key); kept {
			continue
		}
		label := in + " parameter " + p.Key
		if why := removable(op, p.Key, in); why != "" {
			notes = append(notes, OverlayNote{What: label + " is removed", Why: why})
			continue
		}
		sel := target + ".parameters[?(@.name == " + quoted(p.Key) + " && @.in == " + quoted(in) + ")]"
		changes = append(changes, OverlayChange{What: label + " is removed", Target: sel, Remove: true})
	}
	return changes, notes
}

// removable says why a parameter cannot be removed from just this
// operation, or "" when it can.
func removable(op openapi.OperationRef, name, in string) string {
	own, _ := op.Op["parameters"].([]any)
	if slices.ContainsFunc(own, func(p any) bool { return isParam(p, name, in) }) {
		return ""
	}
	if slices.ContainsFunc(op.PathParams, func(p any) bool { return isParam(p, name, in) }) {
		return "the spec gives it to every operation on the path; Flowi keeps it removed"
	}
	return "the spec shares it with other operations ($ref); Flowi keeps it removed"
}

func isParam(p any, name, in string) bool {
	m, _ := p.(map[string]any)
	n, _ := m["name"].(string)
	i, _ := m["in"].(string)
	return i == in && (n == name || (in == "header" && strings.EqualFold(n, name)))
}

// flowiOnlyParts are the parts of a version a spec has no place for.
func flowiOnlyParts(v model.RequestVersion) []OverlayNote {
	why := "the spec has no place for it; Flowi keeps it"
	parts := []struct {
		what string
		has  bool
	}{
		{"the version title", v.Name != ""},
		{"the checks", len(v.Assert) > 0},
		{"the extractions", len(v.Extract) > 0},
		{"the hooks", len(v.Hooks.Pre)+len(v.Hooks.Post) > 0},
		{"the settings", len(v.Settings) > 0},
		{"the variables", len(v.Variables) > 0},
	}
	var out []OverlayNote
	for _, p := range parts {
		if p.has {
			out = append(out, OverlayNote{What: p.what, Why: why})
		}
	}
	return out
}
