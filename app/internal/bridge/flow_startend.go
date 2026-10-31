// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/app/internal/card"
)

// IOView is one declared input or output as the editor behind its row needs
// it: Value is the default of an input and the template of an output.
type IOView struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

func inputViewsOf(f model.Flow) []IOView {
	out := make([]IOView, 0, len(f.Inputs))
	for _, in := range f.Inputs {
		out = append(out, IOView{Name: in.Name, Type: in.Type, Value: in.Default})
	}
	return out
}

func outputViewsOf(f model.Flow) []IOView {
	out := make([]IOView, 0, len(f.Outputs))
	for _, o := range f.Outputs {
		out = append(out, IOView{Name: o.Name, Type: o.Type, Value: o.Value})
	}
	return out
}

// startEndOf is the Start card and, when the flow has outputs, the End card.
// After a finished run the End card carries the run's values and short report.
func startEndOf(o *openFlow) (card.Card, *card.Card) {
	draft := o.doc.Draft()
	var run *report.Run
	if r := o.run.Result(); r.RunID != "" && !o.run.Running() {
		run = &r.Report
	}
	end, drawn := card.EndCard(draft.Outputs, run)
	if !drawn {
		return card.StartCard(draft.Inputs), nil
	}
	return card.StartCard(draft.Inputs), &end
}

// MoveInput moves the input at index from to index to. The file's order
// changes with it, so the Start card and a subflow's caller see the new one.
func (s *FlowService) MoveInput(node string, from, to int) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if from < 0 || from >= len(f.Inputs) || to < 0 || to >= len(f.Inputs) {
			return fmt.Errorf("no place %d to %d among %d inputs", from, to, len(f.Inputs))
		}
		return nil
	}, "flow/inputs/move", func(f *model.Flow) {
		in := f.Inputs[from]
		f.Inputs = slices.Delete(slices.Clone(f.Inputs), from, from+1)
		f.Inputs = slices.Insert(f.Inputs, to, in)
	})
}

// AddInput adds an input at the end and returns its name; name "" or one
// already in use makes a free one ("input", "input-2"…).
func (s *FlowService) AddInput(node, name string) (string, error) {
	var made string
	err := s.editFlowIO(node, func(f model.Flow) error {
		taken := map[string]bool{}
		for _, in := range f.Inputs {
			taken[in.Name] = true
		}
		made = freeName(name, "input", taken)
		return nil
	}, "", func(f *model.Flow) { f.Inputs = append(slices.Clone(f.Inputs), model.Input{Name: made}) })
	return made, err
}

// SetInput changes the input at index. A name must be free, a type one of the
// flow input types or "".
func (s *FlowService) SetInput(node string, index int, name, typ, def string) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if index < 0 || index >= len(f.Inputs) {
			return fmt.Errorf("no input %d", index)
		}
		for i, in := range f.Inputs {
			if i != index && in.Name == name {
				return fmt.Errorf("an input named %q already exists", name)
			}
		}
		return checkIOField(name, typ)
	}, "flow/input/"+strconv.Itoa(index), func(f *model.Flow) {
		f.Inputs = slices.Clone(f.Inputs)
		f.Inputs[index] = model.Input{Name: name, Type: typ, Default: def}
	})
}

// RemoveInput removes the input at index.
func (s *FlowService) RemoveInput(node string, index int) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if index < 0 || index >= len(f.Inputs) {
			return fmt.Errorf("no input %d", index)
		}
		return nil
	}, "", func(f *model.Flow) { f.Inputs = slices.Delete(slices.Clone(f.Inputs), index, index+1) })
}

// AddOutput adds an output at the end and returns its name; the End card
// appears with the first one. A name that is "" or in use makes a free one.
func (s *FlowService) AddOutput(node, name string) (string, error) {
	var made string
	err := s.editFlowIO(node, func(f model.Flow) error {
		taken := map[string]bool{}
		for _, o := range f.Outputs {
			taken[o.Name] = true
		}
		made = freeName(name, "output", taken)
		return nil
	}, "", func(f *model.Flow) { f.Outputs = append(slices.Clone(f.Outputs), model.Output{Name: made}) })
	return made, err
}

// SetOutputValue sets the template the output called name is worked out from.
func (s *FlowService) SetOutputValue(node, name, value string) error {
	return s.editOutput(node, name, "flow/output/"+name+"/value", func(o *model.Output) { o.Value = value }, nil)
}

// SetOutput changes the output at index: its name, type and value template.
func (s *FlowService) SetOutput(node string, index int, name, typ, value string) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if index < 0 || index >= len(f.Outputs) {
			return fmt.Errorf("no output %d", index)
		}
		for i, o := range f.Outputs {
			if i != index && o.Name == name {
				return fmt.Errorf("an output named %q already exists", name)
			}
		}
		return checkIOField(name, typ)
	}, "flow/output/"+strconv.Itoa(index), func(f *model.Flow) {
		f.Outputs = slices.Clone(f.Outputs)
		f.Outputs[index] = model.Output{Name: name, Type: typ, Value: value}
	})
}

// RemoveOutput removes the output at index; the End card goes with the last.
func (s *FlowService) RemoveOutput(node string, index int) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if index < 0 || index >= len(f.Outputs) {
			return fmt.Errorf("no output %d", index)
		}
		return nil
	}, "", func(f *model.Flow) { f.Outputs = slices.Delete(slices.Clone(f.Outputs), index, index+1) })
}

func (s *FlowService) editOutput(node, name, merge string, change func(o *model.Output), check func(f model.Flow) error) error {
	return s.editFlowIO(node, func(f model.Flow) error {
		if !slices.ContainsFunc(f.Outputs, func(o model.Output) bool { return o.Name == name }) {
			return fmt.Errorf("no output %q", name)
		}
		if check != nil {
			return check(f)
		}
		return nil
	}, merge, func(f *model.Flow) {
		f.Outputs = slices.Clone(f.Outputs)
		for i := range f.Outputs {
			if f.Outputs[i].Name == name {
				change(&f.Outputs[i])
			}
		}
	})
}

// editFlowIO checks the change against the draft, then applies it as one undo
// step (edits with the same merge key in a row are one step; "" never merges).
func (s *FlowService) editFlowIO(node string, check func(f model.Flow) error, merge string, change func(f *model.Flow)) error {
	return s.edit(node, func(o *openFlow) error {
		if err := check(o.doc.Draft()); err != nil {
			return err
		}
		if merge == "" {
			o.doc.EditFlow(change)
		} else {
			o.doc.EditFlowMerged(merge, change)
		}
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

func checkIOField(name, typ string) error {
	if name == "" {
		return errors.New("give it a name")
	}
	if typ != "" && !slices.Contains(model.InputTypes(), typ) {
		return fmt.Errorf("%q is not a type (%v)", typ, model.InputTypes())
	}
	return nil
}

// freeName is name when it is set and free, else base with a number.
func freeName(name, base string, taken map[string]bool) string {
	if name != "" && !taken[name] {
		return name
	}
	if name != "" {
		base = name
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		if n := base + "-" + strconv.Itoa(i); !taken[n] {
			return n
		}
	}
}
