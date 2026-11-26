// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// HoverFact is one labelled line of a hover card ("Method", "Checks").
type HoverFact struct {
	Label string
	Value string
}

// HoverProblem is a validation problem of the item, with the code to look it up by and what to do about it.
type HoverProblem struct {
	Severity string // error | warning
	Code     string
	Message  string
	Hint     string
}

// HoverCard is what the sidebar shows when the pointer rests on a row: the important facts about the item, so the
// person need not open it to tell it from another one with a similar name.
//
// Before: the hover text of a row repeated the name the row already showed, plus its status and tags.
type HoverCard struct {
	Title string
	// Kind is what the row is in words: "HTTP request", "gRPC request", "Flow", "Folder", "Collection", "Environment".
	Kind string
	// Verb is the verb or protocol pill of a request (GET, NATS), "FLOW" for a flow; "" for the rest.
	Verb  string
	Where string
	// TargetLabel and Target are the thing the request talks to: the URL, the server, the query or the file.
	TargetLabel string
	Target      string
	Description string
	Facts       []HoverFact
	Problems    []HoverProblem
	// MoreProblems counts the problems left out of Problems (the card lists the first few).
	MoreProblems int
}

// Limits of a hover card: it is a glance, not a report.
const (
	maxHoverProblems    = 4
	maxHoverText        = 220 // the description, the query and the URL, in characters
	maxHoverDescription = 260
)

// Hover builds the hover card of a tree node. problems are the validation problems of the node's file (the Problems
// panel's, see Problems.ForFile); an unreadable file adds its own reason. The zero card is returned for an unknown node.
func (ws *Workspace) Hover(nodeID string, problems []Problem) HoverCard {
	n, ok := ws.snap.nodes[nodeID]
	if !ok {
		return HoverCard{}
	}
	card := HoverCard{Title: ws.Display(nodeID), Where: n.Where}
	c, _ := ws.Collection(n.Collection)
	switch n.Kind {
	case NodeRequest:
		ws.hoverRequest(&card, n, c)
	case NodeFlow:
		hoverFlow(&card, n, c)
	case NodeFolder:
		hoverFolder(&card, n, c)
	case NodeCollection:
		ws.hoverCollection(&card, n, c)
	case NodeEnvironment:
		ws.hoverEnvironment(&card, n)
	case NodeBroken:
		card.Kind = "Unreadable " + strings.TrimSuffix(n.Tree, "s")
		problems = append([]Problem{{Severity: "error", Code: "", Brief: n.Detail, Hint: "Fix the file in a text editor; Flowi reads it again when it changes."}}, problems...)
	case NodeMissing:
		card.Kind = "Missing collection"
		card.Facts = append(card.Facts, HoverFact{"Folder", strings.TrimSuffix(n.Label, " (missing)")})
		problems = []Problem{{Severity: "error", Brief: "The folder is gone.", Hint: "Restore it, or remove the collection from the workspace."}}
	}
	for _, p := range problems {
		if len(card.Problems) == maxHoverProblems {
			card.MoreProblems++
			continue
		}
		card.Problems = append(card.Problems, HoverProblem{Severity: p.Severity, Code: p.Code, Message: p.Brief, Hint: p.Hint})
	}
	if n.Status.Name != "" {
		card.Facts = append(card.Facts, statusFact(n))
	}
	if len(n.Tags) > 0 {
		card.Facts = append(card.Facts, HoverFact{"Tags", strings.Join(n.Tags, ", ")})
	}
	return card
}

func statusFact(n TreeNode) HoverFact {
	value := strings.NewReplacer("-", " ", "_", " ").Replace(n.Status.Name)
	switch {
	case n.StatusUnknown:
		value += " (not one of this collection's statuses)"
	case n.StatusFrom != "":
		value += " (from the folder " + strings.TrimPrefix(n.StatusFrom, "requests/") + ")"
	}
	return HoverFact{"Status", value}
}

func (ws *Workspace) hoverRequest(card *HoverCard, n TreeNode, c model.Collection) {
	card.Verb = n.Method
	card.Kind = n.TypeName + " request"
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == n.Request })
	if i < 0 || len(c.Requests[i].Versions) == 0 {
		return
	}
	r := c.Requests[i]
	v := r.Versions[defaultIndex(r)]
	card.Description = firstNonEmpty(r.Description, docsSummary(v.Docs))
	card.TargetLabel, card.Target, card.Facts = targetOf(v)
	if len(r.Versions) > 1 {
		card.Facts = append(card.Facts, HoverFact{"Versions", fmt.Sprintf("%d, opens on %s", len(r.Versions), versionName(v))})
	}
	if v.Auth != nil && v.Auth.Type != "" {
		card.Facts = append(card.Facts, HoverFact{"Auth", v.Auth.Type})
	}
	if len(v.Assert) > 0 {
		card.Facts = append(card.Facts, HoverFact{"Checks", fmt.Sprintf("%d", len(v.Assert))})
	}
	if v.Expect != "" {
		card.Facts = append(card.Facts, HoverFact{"Expects", v.Expect})
	}
}

func versionName(v model.RequestVersion) string {
	if v.Name != "" {
		return fmt.Sprintf("%s (%s)", v.Label, v.Name)
	}
	return v.Label
}

// targetOf is the labelled thing a version talks to, and the facts that go with it by type. A password in a connection
// string is hidden.
func targetOf(v model.RequestVersion) (label, target string, facts []HoverFact) {
	switch TypeOf(v) {
	case TypeGRPC:
		g := v.GRPC
		return "Server", model.RedactedTarget(g.Server), []HoverFact{{"Call", g.Method}}
	case TypeMessage:
		a := v.Async
		facts = []HoverFact{{"Protocol", model.ProtocolVerb(a.Protocol)}}
		if a.Channel != "" {
			facts = append(facts, HoverFact{"Channel", a.Channel})
		}
		return "Server", model.RedactedTarget(a.Server), facts
	case TypeDB:
		d := v.Dataset
		return "Server", model.RedactedTarget(d.Server), []HoverFact{{"Database", d.Driver}, {"Query", clip(singleLine(d.Query), maxHoverText)}}
	case TypeFile:
		f := v.File
		return "File", f.Path, []HoverFact{{"Format", f.Format}, {"Query", clip(singleLine(f.Query), maxHoverText)}}
	}
	if v.Body.GraphQL != nil {
		kind, name := graphQLFacts(v)
		if kind != "" {
			facts = append(facts, HoverFact{"Operation", strings.TrimSpace(kind + " " + name)})
		}
	}
	return "URL", clip(model.RedactedTarget(v.URL), maxHoverText), facts
}

func hoverFlow(card *HoverCard, n TreeNode, c model.Collection) {
	card.Kind = "Flow"
	card.Verb = "FLOW"
	i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.ID == n.Flow })
	if i < 0 {
		return
	}
	f := c.Flows[i]
	card.Description = firstNonEmpty(f.Description)
	card.Facts = append(card.Facts, HoverFact{"Steps", fmt.Sprintf("%d", len(f.Nodes))})
	if len(f.Inputs) > 0 {
		card.Facts = append(card.Facts, HoverFact{"Inputs", fmt.Sprintf("%d", len(f.Inputs))})
	}
}

func hoverFolder(card *HoverCard, n TreeNode, c model.Collection) {
	card.Kind = "Folder"
	prefix := n.Path + "/"
	var requests, flows int
	if n.Tree == "flows" {
		flows = countUnder(c.Flows, func(f model.Flow) string { return f.Path }, prefix)
	} else {
		requests = countUnder(c.Requests, func(r model.Request) string { return r.Path }, prefix)
	}
	if requests > 0 {
		card.Facts = append(card.Facts, HoverFact{"Requests", fmt.Sprintf("%d", requests)})
	}
	if flows > 0 {
		card.Facts = append(card.Facts, HoverFact{"Flows", fmt.Sprintf("%d", flows)})
	}
	if requests+flows == 0 {
		card.Facts = append(card.Facts, HoverFact{"Contains", "nothing yet"})
	}
}

func countUnder[T any](items []T, pathOf func(T) string, prefix string) int {
	n := 0
	for _, it := range items {
		if strings.HasPrefix(pathOf(it), prefix) {
			n++
		}
	}
	return n
}

func (ws *Workspace) hoverCollection(card *HoverCard, n TreeNode, c model.Collection) {
	card.Kind = "Collection"
	card.Description = c.Manifest.Description
	switch {
	case n.Scratch:
		card.Kind = "Drafts"
		card.Description = "Try-outs that are not saved to a folder yet. Unused drafts are moved to the trash after a while."
	case n.Draft:
		card.Facts = append(card.Facts, HoverFact{"Saved", "not to a folder yet"})
	case c.Root != "":
		card.Facts = append(card.Facts, HoverFact{"Folder", c.Root})
	}
	card.Facts = append(card.Facts, HoverFact{"Requests", fmt.Sprintf("%d", len(c.Requests))}, HoverFact{"Flows", fmt.Sprintf("%d", len(c.Flows))})
}

func (ws *Workspace) hoverEnvironment(card *HoverCard, n TreeNode) {
	card.Kind = "Environment"
	for _, e := range ws.snap.environment {
		if e.File != n.File {
			continue
		}
		card.Description = e.Description
		card.Facts = append(card.Facts, HoverFact{"Variables", fmt.Sprintf("%d", len(e.Vars))})
		if e.Extends != "" {
			card.Facts = append(card.Facts, HoverFact{"Builds on", e.Extends})
		}
		if e.Protected {
			card.Facts = append(card.Facts, HoverFact{"Protected", "asks before a request that changes data is sent"})
		}
		if e.Name == ws.env {
			card.Facts = append(card.Facts, HoverFact{"Selected", "requests are sent with it"})
		}
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			return clip(s, maxHoverDescription)
		}
	}
	return ""
}

// docsSummary is the first paragraph of a Markdown document that is not a heading, as one line: what a request's
// documentation says first, standing in for a description it does not have.
func docsSummary(docs string) string {
	for _, para := range strings.Split(docs, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" || strings.HasPrefix(para, "#") || strings.HasPrefix(para, "```") {
			continue
		}
		return singleLine(para)
	}
	return ""
}

func singleLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// clip cuts s to at most limit characters, with an ellipsis when it was longer.
func clip(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-1]) + "…"
}
