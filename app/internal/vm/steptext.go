// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"path"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/card"
)

// StepLine says in one line what a request or flow step does: the action word and where it goes, then what rides
// with it ("POST /login · JSON · Bearer", "PRODUCE flow-demo-inbox · JSON", "Runs signup"). It is "" for any other step.
func StepLine(c model.Collection, n model.Node) string {
	switch {
	case n.Request != nil:
		if !slices.ContainsFunc(c.Requests, func(r model.Request) bool { return sameRequest(r, n.Request.Request) }) {
			return "" // a request that is not there has no method or target to tell
		}
		f := requestFacts(c, n)
		parts := []string{strings.TrimSpace(card.Summarize(n, f).Badge + " " + f.Target)}
		for _, extra := range []string{f.Body, f.Auth} {
			if extra != "" {
				parts = append(parts, extra)
			}
		}
		return strings.Join(parts, " · ")
	case n.Subflow != nil:
		if t := flowFacts(c, n.Subflow.Flow).Target; t != "" {
			return "Runs " + t
		}
	}
	return ""
}

// StepTitle is what a request or flow step is called by the thing it calls: the request's name, else the last part of
// its path; the flow's name, else its path. It is "" for any other step, or when the target cannot be found.
func StepTitle(c model.Collection, n model.Node) string {
	switch {
	case n.Request != nil:
		ref := n.Request.Request
		for _, r := range c.Requests {
			if sameRequest(r, ref) {
				return firstText(r.Name, path.Base(r.Path))
			}
		}
		if ref.Path != "" {
			return path.Base(ref.Path)
		}
	case n.Subflow != nil:
		ref := n.Subflow.Flow
		for _, f := range c.Flows {
			if f.ID == ref.ID || (ref.ID == "" && f.Path == ref.Path) {
				return firstText(f.Name, path.Base(f.Path))
			}
		}
		if ref.Path != "" {
			return path.Base(ref.Path)
		}
	}
	return ""
}

func calledFromDependency(n model.Node) bool {
	return (n.Request != nil && n.Request.Request.Collection != "") || (n.Subflow != nil && n.Subflow.Flow.Collection != "")
}

// FillStepText gives a request or flow step the name and description its target suggests, where it has none: a flow
// made from requests is otherwise only ids and paths, which says nothing about what each step is for. What the person
// wrote is never replaced, and the name is left out when it would only repeat the step's id.
func FillStepText(c model.Collection, n *model.Node) {
	if n.Name == "" && !calledFromDependency(*n) {
		if title := StepTitle(c, *n); title != "" && title != n.ID {
			n.Name = title
		}
	}
	// A request of a dependency is not in c, so what it does cannot be told from here.
	if n.Description == "" && !calledFromDependency(*n) {
		n.Description = StepLine(c, *n)
	}
}

// StepComment is the line the flow's code view shows above a step: what the step calls and does, made from the request
// or flow each time the view is drawn and never written to the file, so it cannot drift from the request. It is "" for
// a step that calls nothing, or whose target is not there.
func StepComment(c model.Collection, n model.Node) string {
	line := StepLine(c, n)
	if line == "" {
		return ""
	}
	if n.Request != nil {
		if title := StepTitle(c, n); title != "" {
			return title + " — " + line
		}
	}
	return line
}

// StepComments is the comment of every step of f, nested ones included, by step id.
func StepComments(c model.Collection, f model.Flow) map[string]string {
	out := map[string]string{}
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for _, n := range list {
			if text := StepComment(c, n); text != "" {
				out[n.ID] = text
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return out
}
