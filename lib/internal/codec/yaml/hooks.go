// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi-lib/model"
)

func (m *mapping) hooks() model.Hooks {
	hm, ok := m.child("hooks")
	if !ok {
		return model.Hooks{}
	}
	h := model.Hooks{Pre: hm.hookSteps("pre"), Post: hm.hookSteps("post"), Message: hm.hookSteps("message")}
	hm.closeStrict()
	return h
}

func (m *mapping) hookSteps(key string) []model.HookStep {
	e, ok := m.take(key)
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, key)
	if !ok {
		return nil
	}
	steps := make([]model.HookStep, 0, len(items))
	for i, item := range items {
		path := indexPath(e.path, i)
		sm, ok := m.d.mapping(item, path, key)
		if !ok {
			continue
		}
		steps = append(steps, sm.hookStep())
	}
	return steps
}

// hookStep reads {flow: <ref>} or {inline: [nodes]}; exactly one is allowed.
func (m *mapping) hookStep() model.HookStep {
	var step model.HookStep
	if r, ok := m.ref("flow"); ok {
		step.Flow = &r
	}
	step.Inline = m.nodes("inline")
	if (step.Flow == nil) == (len(step.Inline) == 0) {
		m.d.typeError(m.path, m.node, "hook step", "exactly one of flow or inline")
	}
	m.closeStrict()
	return step
}

func encodeHooks(h model.Hooks) any {
	var o omap
	o.put("pre", encodeHookSteps(h.Pre))
	o.put("post", encodeHookSteps(h.Post))
	o.put("message", encodeHookSteps(h.Message))
	return o.items
}

func encodeHookSteps(steps []model.HookStep) []any {
	out := make([]any, 0, len(steps))
	for _, s := range steps {
		var o omap
		if s.Flow != nil {
			o.put("flow", encodeRef(*s.Flow))
		}
		o.put("inline", encodeNodes(s.Inline))
		out = append(out, o.items)
	}
	return out
}
