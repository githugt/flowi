// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// NodeType names a kind of flow node.
type NodeType string

// NodeTypes returns every node type in catalogue order.
func NodeTypes() []NodeType {
	return []NodeType{
		"request", "fetch", "webhook", "subflow",
		"extract", "set", "generate", "transform", "calc",
		"assert", "condition",
		"delay", "batch", "run_till", "parallel", "stop",
		"log", "note", "digest", "save",
	}
}

// HookControl decides which API hooks a request node runs. Mode is "all"
// (default) or "none"; Skip lists "pre" and/or "post".
type HookControl struct {
	Mode string
	Skip []string
}

// RequestCall is the configuration of a request node.
type RequestCall struct {
	Request Ref
	Version string
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

	Request  *RequestCall
	Subflow  *SubflowCall
	Body     []Node
	Branches []Branch
	Then     []Node
	Else     []Node
	Cases    []Case
	Params   []Field
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
