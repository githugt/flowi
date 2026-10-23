// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"errors"
	"path"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// ErrNoName is returned by FromFlow for an empty blueprint name.
var ErrNoName = errors.New("a blueprint needs a name")

// FromFlow makes a blueprint from an existing flow: every distinct request
// or subflow reference in the same collection becomes a reference parameter
// whose default is the original target, and every input becomes a string
// parameter with its default. References into other collections stay as
// they are, since a parameter could not say which collection to look in.
// Using the result with its defaults rebuilds the original flow.
func FromFlow(f model.Flow, newID func() model.ID, name string) (model.Blueprint, error) {
	if strings.TrimSpace(name) == "" {
		return model.Blueprint{}, ErrNoName
	}
	d := &deriver{used: map[string]bool{}, byRef: map[refKey]string{}}
	bp := model.Blueprint{Meta: model.Meta{ID: newID(), Name: name, Description: f.Description}, Body: f}
	bp.Body.Meta, bp.Body.Path, bp.Body.File = model.Meta{}, "", ""
	for _, in := range f.Inputs {
		d.used[in.Name] = true
		bp.Params = append(bp.Params, model.Param{Name: in.Name, Type: model.ParamString, Default: in.Default})
	}
	bp.Body.Inputs = nil
	bp.Body.Outputs = append([]model.KeyValue(nil), f.Outputs...)
	bp.Body.Nodes = cloneNodes(f.Nodes, func(n *model.Node) {
		if n.Request != nil {
			n.Request.Request = d.param(&bp, n.Request.Request, model.ParamRequestRef)
		}
		if n.Subflow != nil {
			n.Subflow.Flow = d.param(&bp, n.Subflow.Flow, model.ParamFlowRef)
		}
	})
	return bp, nil
}

type refKey struct {
	kind model.ParamType
	ref  model.Ref
}

type deriver struct {
	used  map[string]bool
	byRef map[refKey]string
}

// param returns the {param: name} reference standing for ref, declaring the
// parameter on first use. Foreign references are returned unchanged.
func (d *deriver) param(bp *model.Blueprint, ref model.Ref, kind model.ParamType) model.Ref {
	if ref.Collection != "" || ref.Param != "" {
		return ref
	}
	key := refKey{kind: kind, ref: ref}
	if name, ok := d.byRef[key]; ok {
		return model.Ref{Param: name}
	}
	target := ref.Path
	if target == "" {
		target = string(ref.ID)
	}
	name := d.freeName(baseName(target, kind))
	d.byRef[key] = name
	label := "Request"
	if kind == model.ParamFlowRef {
		label = "Flow"
	}
	bp.Params = append(bp.Params, model.Param{Name: name, Type: kind, Description: label + " " + target, Default: target})
	return model.Ref{Param: name}
}

// baseName derives a parameter name from the last segment of a path.
func baseName(target string, kind model.ParamType) string {
	last := path.Base(target)
	var b strings.Builder
	for _, r := range last {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" || name[0] >= '0' && name[0] <= '9' {
		return string(kind[:strings.IndexByte(string(kind), '-')]) + "_" + name // request_1abc, flow_
	}
	return name
}

// freeName appends _2, _3, … until the name is unused.
func (d *deriver) freeName(base string) string {
	name := base
	for n := 2; d.used[name]; n++ {
		name = base + "_" + strconv.Itoa(n)
	}
	d.used[name] = true
	return name
}
