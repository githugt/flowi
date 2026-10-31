// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// StepEntry is one thing the step popup offers. Type is the node type it
// makes; Mode picks a sibling type that does the same job another way (a
// request with mode "url" is a fetch), "" for the entry's own.
type StepEntry struct {
	Group       string
	Name        string
	Description string
	Type        string
	Mode        string
}

// StepCatalog lists the ten things a person can add, in five groups. The
// file format still has more node types (fetch, webhook, run_till, save and
// digest, note); the popup offers them as modes of these ten.
func StepCatalog() []StepEntry {
	return []StepEntry{
		{Group: "Call", Name: "Request", Description: "Send a saved request, or call a URL", Type: "request"},
		{Group: "Call", Name: "Flow", Description: "Run another flow with its inputs", Type: "subflow"},
		{Group: "Data", Name: "Set value", Description: "Keep a value for later steps: typed, read from a response, or generated", Type: "set"},
		{Group: "Data", Name: "Check", Description: "Fail the run unless every expression is true", Type: "assert"},
		{Group: "Branch", Name: "Route", Description: "Take a different path by status or by value", Type: "condition"},
		{Group: "Branch", Name: "Loop", Description: "Repeat steps over a list, a number of times, or until something holds", Type: "batch"},
		{Group: "Order", Name: "Group", Description: "Run steps together and wait for all of them", Type: "parallel"},
		{Group: "Order", Name: "Wait", Description: "Pause for a fixed or random time", Type: "delay"},
		{Group: "Finish", Name: "Output", Description: "Write a value to the run log, a file or a webhook", Type: "log"},
		{Group: "Finish", Name: "Stop", Description: "End the run here, as passed or as failed", Type: "stop"},
	}
}

// SearchSteps lists the entries whose name or description contains every
// word of query, ignoring case; a blank query lists them all.
func SearchSteps(query string) []StepEntry {
	words := strings.Fields(strings.ToLower(query))
	var out []StepEntry
	for _, e := range StepCatalog() {
		text := strings.ToLower(e.Name + " " + e.Description)
		if allIn(words, text) {
			out = append(out, e)
		}
	}
	return out
}

func allIn(words []string, text string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// StepType is the node type an entry makes once its mode is applied.
func StepType(e StepEntry) model.NodeType {
	modes := map[string]map[string]model.NodeType{
		"request": {"url": "fetch", "send": "webhook"},
		"batch":   {"until": "run_till"},
		"log":     {"file": "save", "digest": "digest"},
	}
	if t, ok := modes[e.Type][e.Mode]; ok {
		return t
	}
	return model.NodeType(e.Type)
}

// ConvertStep turns n into the kind entry names, keeping what carries over
// (the id, name, colour, ordering, the steps inside, and every parameter
// the new kind also has) and naming what does not, so the person can be
// told before they agree. A saved request that becomes a URL call is kept
// in the step's description.
func ConvertStep(n model.Node, to StepEntry) (model.Node, []string) {
	target := StepType(to)
	if n.Type == target {
		return n, nil
	}
	out := n
	out.Type = target
	var lost []string
	lost = append(lost, convertCall(&out, target)...)
	lost = append(lost, convertParams(&out, target)...)
	lost = append(lost, convertChildren(&out, target)...)
	return out, lost
}

// convertCall drops the saved-request or subflow part and returns its
// fields that the new kind cannot hold.
func convertCall(n *model.Node, target model.NodeType) []string {
	var lost []string
	if c := n.Request; c != nil && target != "request" {
		n.Description = noteOf(n.Description, "was request "+refText(c.Request))
		lost = append(lost, requestLosses(c, target)...)
		n.Params = carryHeaders(n.Params, c.Headers, target)
		n.Request = nil
	}
	if c := n.Subflow; c != nil && target != "subflow" {
		n.Description = noteOf(n.Description, "was flow "+refText(c.Flow))
		lost = append(lost, "flow")
		if len(c.With) > 0 {
			lost = append(lost, "with")
		}
		n.Subflow = nil
	}
	return lost
}

func requestLosses(c *model.RequestCall, target model.NodeType) []string {
	lost := []string{"request"}
	for name, set := range map[string]bool{
		"version": c.Version != "", "overlay": c.Overlay != "", "with": len(c.With) > 0,
		"connection": c.Open != "" || c.Over != "" || c.Close, "hooks": c.Hooks.Mode != "" || len(c.Hooks.Skip) > 0,
		"headers": len(c.Headers) > 0 && !acceptsParam(target, "headers"),
	} {
		if set {
			lost = append(lost, name)
		}
	}
	slices.Sort(lost[1:])
	return lost
}

// carryHeaders keeps a request's own headers as the new step's headers when
// the new kind has them.
func carryHeaders(params []model.Field, headers []model.KeyValue, target model.NodeType) []model.Field {
	if len(headers) == 0 || !acceptsParam(target, "headers") {
		return params
	}
	fields := make([]model.Field, 0, len(headers))
	for _, h := range headers {
		fields = append(fields, model.Field{Key: h.Key, Value: h.Value})
	}
	return SetParam(params, "headers", fields)
}

// convertParams keeps the parameters the new kind has, renaming the two
// that mean the same thing under another name, and names the rest.
func convertParams(n *model.Node, target model.NodeType) []string {
	renames := map[string]string{"payload": "body", "body": "payload"}
	var kept []model.Field
	var lost []string
	for _, f := range n.Params {
		if !acceptsParam(target, f.Key) {
			if to := renames[f.Key]; to != "" && acceptsParam(target, to) {
				kept = append(kept, model.Field{Key: to, Value: f.Value})
				continue
			}
			lost = append(lost, f.Key)
			continue
		}
		kept = append(kept, f)
	}
	n.Params = kept
	return lost
}

// convertChildren clears the nested lists the new kind has no place for and
// names "steps inside" once if any of them held steps.
func convertChildren(n *model.Node, target model.NodeType) []string {
	holdsBody := target == "batch" || target == "run_till"
	had := len(n.Children()) > 0
	keptBody := n.Body
	keptBranches := n.Branches
	n.Body, n.Branches, n.Then, n.Else, n.Cases = nil, nil, nil, nil, nil
	switch {
	case holdsBody:
		n.Body = keptBody
	case target == "parallel":
		n.Branches = keptBranches
	}
	if had && len(n.Children()) == 0 {
		return []string{"steps inside"}
	}
	return nil
}

// acceptsParam says whether the library's catalogue lists name as a
// parameter of node type t. A set node also takes any "<variable>: value".
func acceptsParam(t model.NodeType, name string) bool {
	for _, s := range flowi.NodeCatalog() {
		if s.Type != string(t) {
			continue
		}
		return s.MapForm || slices.ContainsFunc(s.Params, func(p flowi.NodeParam) bool { return p.Name == name })
	}
	return false
}

func refText(r model.Ref) string {
	where := r.Path
	if where == "" {
		where = string(r.ID)
	}
	if r.Collection == "" {
		return where
	}
	return fmt.Sprintf("%s: %s", r.Collection, where)
}

func noteOf(existing, note string) string {
	if existing == "" {
		return note
	}
	return existing + " (" + note + ")"
}
