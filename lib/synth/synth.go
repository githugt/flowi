// SPDX-License-Identifier: Apache-2.0

package synth

import (
	"slices"
	"strings"
)

// Op is one request as far as synthesis is concerned: where it goes and what it takes and returns.
type Op struct {
	Request string   // the request's path in its collection, e.g. pets/create
	Method  string   // HTTP method
	Path    string   // URL path with {param} placeholders, without the server
	Params  []string // names of the path parameters
	Fields  []string // top-level fields of a successful JSON response
}

// Edge says that the response of From holds a value the path of To needs.
type Edge struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Field      string  `json:"field"`      // the response field of From
	Param      string  `json:"param"`      // the path parameter of To
	Confidence float64 `json:"confidence"` // 0 to 1; only edges at or above MinConfidence are made
}

// MinConfidence is the least an edge needs to be proposed. A matching name alone scores 0.5: an id of one
// resource must not feed another; the resource has to match as well.
const MinConfidence = 0.8

// Graph finds which request feeds which. Only a POST is taken to make a resource, and only into the path of the
// same resource (/pets makes what /pets/{petId} reads).
func Graph(ops []Op) []Edge {
	var edges []Edge
	for _, maker := range ops {
		if maker.Method != "POST" {
			continue
		}
		for _, user := range ops {
			for _, param := range user.Params {
				if e, ok := edgeOf(maker, user, param); ok {
					edges = append(edges, e)
				}
			}
		}
	}
	return edges
}

// edgeOf judges whether maker's response feeds param of user.
func edgeOf(maker, user Op, param string) (Edge, bool) {
	resource, after := splitAtParam(user.Path, param)
	if after != "" || trim(resource) != trim(maker.Path) {
		return Edge{}, false // the parameter is not the resource's own id
	}
	for _, field := range maker.Fields {
		score := nameScore(field, param, lastSegment(maker.Path)) + 0.5
		if score >= MinConfidence {
			return Edge{From: maker.Request, To: user.Request, Field: field, Param: param, Confidence: min(score, 1)}, true
		}
	}
	return Edge{}, false
}

// splitAtParam cuts path at {param}: what comes before it, and what follows it.
func splitAtParam(path, param string) (before, after string) {
	before, after, _ = strings.Cut(path, "{"+param+"}")
	return before, after
}

func trim(p string) string { return strings.Trim(p, "/") }

func lastSegment(path string) string {
	segments := strings.Split(trim(path), "/")
	return segments[len(segments)-1]
}

// nameScore says how well a response field names a path parameter: the same name, the same name written another
// way (pet_id, petId), or a plain id for the resource's own id.
func nameScore(field, param, resource string) float64 {
	f, p := normalise(field), normalise(param)
	switch {
	case f == p:
		return 0.4
	case f == "id" && (p == "id" || p == normalise(singular(resource))+"id"):
		return 0.4
	}
	return 0
}

func normalise(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

func singular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return strings.TrimSuffix(s, "ies") + "y"
	case strings.HasSuffix(s, "s"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// Step is one request of a proposed flow and the values it takes from earlier steps.
type Step struct {
	Request string `json:"request"`
	Method  string `json:"method"`
	Takes   []Edge `json:"takes"`
}

// Proposal is a flow worth trying: a create, then the reads, updates and deletes of what it made.
type Proposal struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

// methodOrder is the order a resource's lifecycle is exercised in: delete goes last, or nothing is left to read.
func methodOrder() []string { return []string{"POST", "GET", "HEAD", "PUT", "PATCH", "DELETE"} }

// Propose makes one proposal for each request that creates something others use.
func Propose(ops []Op, edges []Edge) []Proposal {
	byRequest := map[string]Op{}
	for _, op := range ops {
		byRequest[op.Request] = op
	}
	var out []Proposal
	for _, maker := range ops {
		takes := edgesFrom(edges, maker.Request)
		if len(takes) == 0 {
			continue
		}
		p := Proposal{Name: lastSegment(maker.Path), Steps: []Step{{Request: maker.Request, Method: maker.Method}}}
		for _, e := range takes {
			p.Steps = append(p.Steps, Step{Request: e.To, Method: byRequest[e.To].Method, Takes: []Edge{e}})
		}
		order := methodOrder()
		slices.SortStableFunc(p.Steps[1:], func(a, b Step) int {
			return slices.Index(order, a.Method) - slices.Index(order, b.Method)
		})
		out = append(out, p)
	}
	return out
}

func edgesFrom(edges []Edge, from string) []Edge {
	var out []Edge
	for _, e := range edges {
		if e.From == from {
			out = append(out, e)
		}
	}
	return out
}
