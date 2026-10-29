// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// NodeType names a kind of flow node.
type NodeType string

// NodeTypes returns every node type in catalogue order.
func NodeTypes() []NodeType {
	return []NodeType{
		"request", "fetch", "webhook", "subflow",
		"set",
		"assert", "fuzz", "condition",
		"delay", "batch", "run_till", "parallel", "stop",
		"log", "note", "digest", "save",
	}
}

// NodeColors are the colours a person may give a step on the canvas. A step
// with no colour is drawn in its category's colour.
func NodeColors() []string {
	return []string{"blue", "green", "amber", "pink", "purple", "teal"}
}

// HookControl decides which API hooks a request node runs. Mode is "all"
// (default) or "none"; Skip lists "pre", "post" and/or "message".
type HookControl struct {
	Mode string
	Skip []string
}

// RequestCall is the configuration of a request node.
type RequestCall struct {
	Request Ref
	Version string
	// Overlay names a flowi overlay of the request's collection laid over the
	// version, "" for none.
	Overlay string
	// Open names a connection this step opens and leaves open for later steps
	// (an async request that can be kept open: WebSocket, MQTT 5, NATS); ""
	// for none. The step's own send and receive run on it.
	Open string
	// Over names a connection an earlier step opened: this step sends and
	// receives over it instead of connecting.
	Over string
	// Close ends the connection Over names once this step has run. A
	// connection still open when the run ends is closed then.
	Close   bool
	Hooks   HookControl
	With    []KeyValue
	Headers []KeyValue
}

// SubflowCall is the configuration of a subflow node.
type SubflowCall struct {
	Flow Ref
	With []KeyValue
}

// Branch is one lane of a parallel node.
type Branch struct {
	ID    string
	Nodes []Node
}

// Case is one arm of a condition node's switch.
type Case struct {
	When  string
	Nodes []Node
}

// SetValue is one value a set node writes into the variable Name.
//
// From says where the value comes from: "expr" (the default; Value is a
// template or expression), "literal" (Value as written, never evaluated),
// "response" (a part of the last response, named by Where) or "generate"
// (Value is a template using generators such as {{$uuid-v4}}; Seed makes it
// repeatable on its own).
type SetValue struct {
	Name  string
	From  string
	Value any
	// Where, for From "response", is "body", "header", "cookie" or "status";
	// Path, Header, Cookie and Regex then say which part (a body regex wins
	// over the path).
	Where  string
	Path   string
	Header string
	Cookie string
	Regex  string
	Seed   int64
}

// RemovedNodeTypes maps a node type that no longer exists to what replaces
// it, so a file that still uses one fails with a message that says what to
// write instead.
func RemovedNodeTypes() map[NodeType]string {
	return map[NodeType]string{
		"extract":   "set with a value that has from: response",
		"generate":  "set with a value that has from: generate",
		"transform": "set with a value that has from: expr",
		"calc":      "set with a value that has from: expr",
	}
}

// Node is one step of a flow. Only the fields relevant to its Type are set.
// Type-specific options without a typed field are kept, in file order, in
// Params until the engine milestone types them.
type Node struct {
	ID          string
	Type        NodeType
	Name        string
	Description string
	Needs       []string
	When        string
	Concurrent  bool
	OnError     string
	Timeout     time.Duration
	// Attach names a request or subflow node of the same list that this node is shown
	// with: the canvas draws them as one block. It is a view only; the run order is
	// the list order, as always.
	Attach string
	// AttachAt says where, around that request, the canvas draws the node: "above" or
	// "below" it in a stack, or "" beside it (before it in the list is drawn left of it,
	// after it right of it). Like Attach it is a view only.
	AttachAt string
	// Color is a colour from NodeColors the canvas draws the step in; "" for
	// the category's own. It is cosmetic: a run never reads it.
	Color string

	Request  *RequestCall
	Subflow  *SubflowCall
	Body     []Node
	Branches []Branch
	Then     []Node
	Else     []Node
	Cases    []Case
	// Values are what a set node writes, in order; a later one may read an
	// earlier one. A set node with a target (and no values) writes one
	// variable or request field instead.
	Values []SetValue
	Params []Field
}

// Children returns the node's non-empty nested node lists in a fixed
// order: body, branches, then, else, cases.
func (n Node) Children() [][]Node {
	lists := [][]Node{n.Body}
	for _, b := range n.Branches {
		lists = append(lists, b.Nodes)
	}
	lists = append(lists, n.Then, n.Else)
	for _, c := range n.Cases {
		lists = append(lists, c.Nodes)
	}
	nonEmpty := lists[:0]
	for _, l := range lists {
		if len(l) > 0 {
			nonEmpty = append(nonEmpty, l)
		}
	}
	return nonEmpty
}
