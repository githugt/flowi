// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/model"
)

// Suffix ends the file name of a flowi overlay, kept under overlays/.
const Suffix = ".flowi-overlay.yaml"

// FormatVersion is the value of the file's flowi_overlay key.
const FormatVersion = 1

type fileDoc struct {
	Format      int                   `yaml:"flowi_overlay"`
	Name        string                `yaml:"name,omitempty"`
	Description string                `yaml:"description,omitempty"`
	Shared      []opDoc               `yaml:"shared,omitempty"`
	Requests    map[string]requestDoc `yaml:"requests,omitempty"`
}

type requestDoc struct {
	Path string  `yaml:"path,omitempty"`
	Ops  []opDoc `yaml:"ops"`
}

// opDoc is one operation as written: the verb is the key that carries the
// path (`set: headers.X-Tier`), so a file reads like a list of edits.
type opDoc struct {
	Set    string `yaml:"set,omitempty"`
	Remove string `yaml:"remove,omitempty"`
	Add    string `yaml:"add,omitempty"`
	Merge  string `yaml:"merge,omitempty"`
	Value  any    `yaml:"value,omitempty"`
	Was    any    `yaml:"was,omitempty"`
}

// Decode reads a *.flowi-overlay.yaml file. fallbackName names the overlay
// when the file does not (the file name without the suffix). Problems are
// FLW-OVL-001.
func Decode(file, fallbackName string, data []byte) (model.Overlay, diag.List) {
	o := model.Overlay{File: file, Name: fallbackName}
	var doc fileDoc
	if err := yaml.UnmarshalWithOptions(data, &doc, yaml.Strict()); err != nil {
		return o, invalid(file, firstLine(err.Error()))
	}
	if doc.Format != FormatVersion {
		return o, invalid(file, fmt.Sprintf("flowi_overlay must be %d", FormatVersion))
	}
	if doc.Name != "" {
		o.Name = doc.Name
	}
	o.Description = doc.Description
	var diags diag.List
	var ok bool
	if o.Shared, ok = decodeOps(doc.Shared, "shared", file, &diags); !ok {
		return o, diags
	}
	for _, id := range sortedRequestIDs(doc.Requests) {
		rd := doc.Requests[id]
		ops, ok := decodeOps(rd.Ops, "requests."+id, file, &diags)
		if !ok {
			return o, diags
		}
		o.Requests = append(o.Requests, model.RequestOverlay{Request: model.ID(id), Path: rd.Path, Ops: ops})
	}
	return o, diags
}

func sortedRequestIDs(m map[string]requestDoc) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func decodeOps(docs []opDoc, where, file string, diags *diag.List) ([]model.OverlayOp, bool) {
	ops := make([]model.OverlayOp, 0, len(docs))
	for i, d := range docs {
		op, err := d.operation()
		if err != nil {
			*diags = append(*diags, invalid(file, fmt.Sprintf("%s, item %d: %v", where, i+1, err))...)
			return nil, false
		}
		ops = append(ops, op)
	}
	return ops, true
}

func (d opDoc) operation() (model.OverlayOp, error) {
	verbs := map[string]string{model.OverlaySet: d.Set, model.OverlayRemove: d.Remove, model.OverlayAdd: d.Add, model.OverlayMerge: d.Merge}
	var op model.OverlayOp
	for verb, path := range verbs {
		if path == "" {
			continue
		}
		if op.Verb != "" {
			return op, fmt.Errorf("an operation has one of set, remove, add or merge")
		}
		op.Verb, op.Path = verb, path
	}
	if op.Verb == "" {
		return op, fmt.Errorf("an operation needs set, remove, add or merge")
	}
	op.Value, op.Was = normalise(d.Value), normalise(d.Was)
	return op, nil
}

func invalid(file, detail string) diag.List {
	return diag.List{diag.New(diag.CodeOverlayInvalid, diag.Location{File: file}, map[string]string{"detail": detail})}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// Encode writes an overlay as a file. Requests are sorted by id so the file
// changes only where the overlay does.
func Encode(o model.Overlay) ([]byte, error) {
	doc := fileDoc{Format: FormatVersion, Name: o.Name, Description: o.Description, Shared: opDocs(o.Shared)}
	if len(o.Requests) > 0 {
		doc.Requests = map[string]requestDoc{}
	}
	for _, r := range o.Requests {
		doc.Requests[string(r.Request)] = requestDoc{Path: r.Path, Ops: opDocs(r.Ops)}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf, yaml.Indent(2), yaml.IndentSequence(true))
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding flowi overlay %s: %w", o.Name, err)
	}
	return buf.Bytes(), nil
}

func opDocs(ops []model.OverlayOp) []opDoc {
	out := make([]opDoc, 0, len(ops))
	for _, op := range ops {
		d := opDoc{Value: op.Value, Was: op.Was}
		switch op.Verb {
		case model.OverlaySet:
			d.Set = op.Path
		case model.OverlayRemove:
			d.Remove = op.Path
		case model.OverlayAdd:
			d.Add = op.Path
		case model.OverlayMerge:
			d.Merge = op.Path
		}
		out = append(out, d)
	}
	return out
}

// CheckName reports why name cannot be a flowi overlay's name: it becomes a
// file name and appears in response folders and flags.
func CheckName(name string) error {
	if !names.Valid(name) {
		return fmt.Errorf("use letters, digits, '.', '_' and '-', not starting with '.'")
	}
	return nil
}

// ForRequest returns the operations that apply to request id: the shared
// part first, then the request's own.
func ForRequest(o model.Overlay, id model.ID) []model.OverlayOp {
	ops := slices.Clone(o.Shared)
	for _, r := range o.Requests {
		if r.Request == id {
			ops = append(ops, r.Ops...)
		}
	}
	return ops
}

// StackSeparator joins the names of overlays laid over one request, in order:
// "vip+slow". CheckName refuses it in a name, so a stack is never ambiguous
// and stays one string wherever an overlay is named (a flag, a flow step, a
// folder of saved responses).
const StackSeparator = "+"

// Stack splits a stack of overlay names, dropping blanks.
func Stack(stack string) []string {
	var out []string
	for _, name := range strings.Split(stack, StackSeparator) {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Resolve returns the operations the overlays of stack make to request id,
// each overlay's shared part and then its own, in the order named, so a later
// overlay wins where two set the same field. missing is the first name that is
// not among overlays ("" when all are).
func Resolve(overlays []model.Overlay, stack string, id model.ID) (ops []model.OverlayOp, missing string) {
	for _, name := range Stack(stack) {
		i := slices.IndexFunc(overlays, func(o model.Overlay) bool { return o.Name == name })
		if i < 0 {
			return nil, name
		}
		ops = append(ops, ForRequest(overlays[i], id)...)
	}
	return ops, ""
}
