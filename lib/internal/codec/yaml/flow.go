// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// DecodeFlow reads a *.flow.yaml file. path is relative to flows/ without
// the suffix.
func (c *Codec) DecodeFlow(file, path string, data []byte) (model.Flow, diag.List) {
	f := model.Flow{Path: path, File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return f, d.diags
	}
	m, ok := d.mapping(root, "$", "flow")
	if !ok {
		return f, d.diags
	}
	f.Meta = m.meta()
	m.flowBody(&f)
	m.closeStrict()
	return f, d.diags
}

// flowBody reads the keys a flow and a blueprint share: everything but the
// identity fields.
func (m *mapping) flowBody(f *model.Flow) {
	f.PreferVersion = m.versionLabel("prefer_version")
	f.Inputs = m.inputs()
	f.Outputs = m.keyValues("outputs")
	f.Execution = m.execution()
	f.Run = m.runTill()
	f.Nodes = m.nodes("nodes")
}

func (m *mapping) inputs() []model.Input {
	im, ok := m.child("inputs")
	if !ok {
		return nil
	}
	out := make([]model.Input, 0, len(im.entries))
	for _, e := range im.entries {
		in := model.Input{Name: e.key}
		if dm, ok := m.d.mapping(e.value, e.path, e.key); ok {
			in.Default = dm.str("default")
			dm.closeStrict()
		}
		out = append(out, in)
	}
	return out
}

func (m *mapping) execution() model.Execution {
	em, ok := m.child("execution")
	if !ok {
		return model.Execution{}
	}
	ex := model.Execution{MaxConcurrency: int(em.integer("max_concurrency"))}
	ex.RateLimit, ex.HostRateLimits = em.rateLimit()
	ex.OnBranchError = em.str("on_branch_error")
	em.closeStrict()
	return ex
}

// allHosts is the rate_limit map key that applies to every host, so a
// scalar and per-host limits can be written together.
const allHosts = "*"

// rateLimit reads `rate_limit: 20/s` or `rate_limit: {host: 5/s, "*": 20/s}`.
func (m *mapping) rateLimit() (string, []model.KeyValue) {
	e, ok := m.take("rate_limit")
	if !ok {
		return "", nil
	}
	if _, isMap := e.value.(*ast.MappingNode); !isMap {
		return m.d.scalarString(e.value, e.path, "rate_limit", false), nil
	}
	var all string
	var hosts []model.KeyValue
	for _, kv := range m.d.keyValuePairs(e) {
		if kv.Key == allHosts {
			all = kv.Value
			continue
		}
		hosts = append(hosts, kv)
	}
	return all, hosts
}

func (m *mapping) runTill() *model.RunTill {
	rm, ok := m.child("run")
	if !ok {
		return nil
	}
	r := &model.RunTill{
		Until:         rm.str("until"),
		MaxIterations: int(rm.integer("max_iterations")),
		MaxDuration:   rm.duration("max_duration"),
		Interval:      rm.duration("interval"),
	}
	rm.closeStrict()
	return r
}

// EncodeFlow writes a flow in canonical form.
func (c *Codec) EncodeFlow(f model.Flow) ([]byte, error) {
	var o omap
	putMeta(&o, f.Meta)
	putFlowBody(&o, f)
	return marshal(o.items)
}

// putFlowBody writes the keys a flow and a blueprint share.
func putFlowBody(o *omap, f model.Flow) {
	o.put("prefer_version", f.PreferVersion)
	o.put("inputs", encodeInputs(f.Inputs))
	o.put("outputs", keyValues(f.Outputs))
	o.put("execution", encodeExecution(f.Execution))
	if f.Run != nil {
		o.put("run", encodeRunTill(*f.Run))
	}
	o.put("nodes", encodeNodes(f.Nodes))
}

func encodeInputs(ins []model.Input) yaml.MapSlice {
	var out yaml.MapSlice
	for _, in := range ins {
		var o omap
		o.put("default", in.Default)
		value := any(o.items)
		if o.items == nil {
			value = yaml.MapSlice{}
		}
		out = append(out, yaml.MapItem{Key: in.Name, Value: value})
	}
	return out
}

func encodeExecution(ex model.Execution) yaml.MapSlice {
	var o omap
	o.put("max_concurrency", int64(ex.MaxConcurrency))
	o.put("rate_limit", encodeRateLimit(ex))
	o.put("on_branch_error", ex.OnBranchError)
	return o.items
}

func encodeRunTill(r model.RunTill) yaml.MapSlice {
	var o omap
	o.put("until", r.Until)
	o.put("max_iterations", int64(r.MaxIterations))
	if r.MaxDuration > 0 {
		o.put("max_duration", formatDuration(r.MaxDuration))
	}
	if r.Interval > 0 {
		o.put("interval", formatDuration(r.Interval))
	}
	return o.items
}

func encodeRateLimit(ex model.Execution) any {
	if len(ex.HostRateLimits) == 0 {
		return ex.RateLimit
	}
	all := ex.HostRateLimits
	if ex.RateLimit != "" {
		all = append([]model.KeyValue{{Key: allHosts, Value: ex.RateLimit}}, all...)
	}
	return keyValues(all)
}
