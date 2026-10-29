// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// ConnectData makes step toStep read a value that step fromStep makes: it writes
// the template that reads it into a field of toStep, as one change that Undo
// takes back. outPath is what to read of the step's outputs (status, body.token);
// an empty fromStep means the flow's own input outPath, as the Start card
// offers it. field says where it goes: "with.<name>" or "headers.<name>" of a
// request or flow step, "param.<name>" of any step, "values.<n>" for the value
// row n of a set step. It is refused, with the reason, when toStep cannot read
// what fromStep makes (it runs before it, or in another lane); nothing changes
// then.
func (s *FlowService) ConnectData(node, fromStep, outPath, toStep, field string) error {
	return s.edit(node, func(o *openFlow) error {
		if fromStep != "" {
			if ok, why := o.doc.CanRead(fromStep, toStep); !ok {
				return fmt.Errorf("%s", why)
			}
		}
		target, ok := o.doc.Node(toStep)
		if !ok {
			return fmt.Errorf("there is no step %s", toStep)
		}
		place, err := placeFor(target, field)
		if err != nil {
			return err
		}
		ref := readRef(fromStep, outPath)
		o.doc.EditNode(toStep, func(n *model.Node) { place(n, ref) })
		return nil
	})
}

// readRef is what reads a value: nodes.<step>.<path>, or inputs.<name>.
func readRef(step, path string) string {
	if step == "" {
		return "inputs." + path
	}
	if path == "" {
		return "nodes." + step
	}
	return "nodes." + step + "." + path
}

// placer writes a reference into a node.
type placer func(n *model.Node, ref string)

// placeFor is how a reference is written into field of n; an error says why the
// step has no such field.
func placeFor(n model.Node, field string) (placer, error) {
	kind, name, _ := strings.Cut(field, ".")
	switch {
	case (kind == "with" || kind == "headers") && name != "":
		return placeInCall(n, kind, name)
	case kind == "param" && name != "":
		return placeInParam(n, name)
	case kind == "values":
		return placeInValue(n, name)
	}
	return nil, fmt.Errorf("%q is not a place a value can go (use with.<name>, headers.<name>, param.<name> or values.<n>)", field)
}

func placeInCall(n model.Node, kind, name string) (placer, error) {
	call := n.Request != nil || n.Subflow != nil
	if !call || (kind == "headers" && n.Request == nil) {
		return nil, fmt.Errorf("step %s has no %s to pass a value into", n.ID, kind)
	}
	return func(n *model.Node, ref string) {
		var pairs *[]model.KeyValue
		switch {
		case n.Request != nil && kind == "headers":
			pairs = &n.Request.Headers
		case n.Request != nil:
			pairs = &n.Request.With
		default:
			pairs = &n.Subflow.With
		}
		*pairs = setPair(*pairs, name, "{{"+ref+"}}")
	}, nil
}

// setPair sets key's value, keeping its place, or appends it.
func setPair(pairs []model.KeyValue, key, value string) []model.KeyValue {
	out := append([]model.KeyValue(nil), pairs...)
	for i := range out {
		if out[i].Key == key {
			out[i].Value = value
			return out
		}
	}
	return append(out, model.KeyValue{Key: key, Value: value})
}

func placeInParam(n model.Node, name string) (placer, error) {
	var param *flowParam
	for _, p := range catalog().byType[string(n.Type)].Params {
		if p.Name == name {
			param = &flowParam{kind: p.Kind}
		}
	}
	if param == nil {
		return nil, fmt.Errorf("step %s has no field %s", n.ID, name)
	}
	if param.kind == "bool" || param.kind == "int" || param.kind == "enum" || param.kind == "duration" {
		return nil, fmt.Errorf("%s of step %s is a choice, not something that reads a value", name, n.ID)
	}
	expr := param.kind == "expr"
	if expr && vm.ParamText(n, name) != "" {
		return nil, fmt.Errorf("%s of step %s already has a test: add the value to it there", name, n.ID)
	}
	return func(n *model.Node, ref string) {
		text := "{{" + ref + "}}"
		if expr {
			text = ref
		}
		if old := vm.ParamText(*n, name); old != "" {
			text = old + " " + text
		}
		n.Params = vm.SetParam(n.Params, name, text)
	}, nil
}

type flowParam struct{ kind string }

func placeInValue(n model.Node, index string) (placer, error) {
	i, err := strconv.Atoi(index)
	if n.Type != "set" || err != nil || i < 0 || i >= len(n.Values) {
		return nil, fmt.Errorf("step %s has no value %s", n.ID, index)
	}
	return func(n *model.Node, ref string) {
		n.Values[i].Value = "{{" + ref + "}}"
		if n.Values[i].From == "literal" {
			n.Values[i].From = ""
		}
	}, nil
}

// WireChoices is what a value wire from step `from` to step `to` can say: which
// part of what `from` makes to take, and where in `to` to put it. When `to`
// cannot read `from`, Reason says why and nothing else is offered.
type WireChoices struct {
	Reason  string      `json:"reason,omitempty"`
	Outputs []string    `json:"outputs"`
	Fields  []WireField `json:"fields"`
}

// WireField is a place a value can go. NeedsName means ID ends in a dot and the
// person types the name of the new entry (a with value, a header).
type WireField struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	NeedsName bool   `json:"needsName,omitempty"`
}

// WireChoices lists the choices for a wire from `from` to `to`.
func (s *FlowService) WireChoices(node, from, to string) WireChoices {
	out := WireChoices{Outputs: []string{}, Fields: []WireField{}}
	_ = s.edit(node, func(o *openFlow) error {
		if ok, why := o.doc.CanRead(from, to); !ok {
			out.Reason = why
			return nil
		}
		source, _ := o.doc.Node(from)
		target, _ := o.doc.Node(to)
		out.Outputs = outputsOfStep(source)
		out.Fields = fieldsOfStep(target)
		return nil
	})
	return out
}

// outputsOfStep is what a step makes that a later step can read, by name.
func outputsOfStep(n model.Node) []string {
	switch {
	case n.Request != nil:
		return []string{"status", "body", "headers", "time"}
	case n.Type == "set":
		var names []string
		for _, row := range valueRowsOf(n) {
			names = append(names, row.Name)
		}
		return names
	case n.Type == "fetch" || n.Type == "webhook":
		return []string{"status", "body", "headers", "value"}
	}
	return []string{}
}

// fieldsOfStep is where a step takes a value in.
func fieldsOfStep(n model.Node) []WireField {
	var out []WireField
	switch {
	case n.Request != nil:
		out = append(out, WireField{ID: "with.", Label: "a value passed in…", NeedsName: true}, WireField{ID: "headers.", Label: "a header…", NeedsName: true})
	case n.Subflow != nil:
		out = append(out, WireField{ID: "with.", Label: "an input…", NeedsName: true})
	case n.Type == "set":
		for i, row := range valueRowsOf(n) {
			out = append(out, WireField{ID: "values." + strconv.Itoa(i), Label: "the value of " + cmpOr(row.Name, "row "+strconv.Itoa(i+1))})
		}
	}
	for _, p := range catalog().byType[string(n.Type)].Params {
		if p.Kind == "text" || p.Kind == "any" || p.Kind == "string" || p.Kind == "expr" {
			out = append(out, WireField{ID: "param." + p.Name, Label: p.Name})
		}
	}
	return out
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
