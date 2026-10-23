// SPDX-License-Identifier: Apache-2.0

package report

import "time"

// Run is the report of one run (a flow run or a standalone send).
type Run struct {
	ID         string        `json:"id"`
	Kind       string        `json:"kind"` // "flow" or "send"
	FlowID     string        `json:"flow_id,omitempty"`
	FlowPath   string        `json:"flow_path,omitempty"`
	FlowName   string        `json:"flow_name,omitempty"`
	Collection string        `json:"collection,omitempty"`
	Env        string        `json:"env,omitempty"`
	Status     string        `json:"status"`
	Seed       uint64        `json:"seed"`
	Started    time.Time     `json:"started,omitzero"`
	Duration   time.Duration `json:"-"`
	Nodes      []Node        `json:"nodes"`
	Warnings   []Diagnostic  `json:"warnings,omitempty"`
	Notes      []Note        `json:"notes,omitempty"`
	Versions   []VersionUse  `json:"versions,omitempty"`
	Aborted    *Abort        `json:"aborted,omitempty"`
}

// Node is one node instance. Containers group their children in Lanes
// (batch iterations, parallel branches, the chosen condition branch).
type Node struct {
	Path        string         `json:"path"`
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Status      string         `json:"status"`
	Handled     bool           `json:"handled,omitempty"` // failed, but on_error let the run continue
	Started     time.Time      `json:"started,omitzero"`
	Duration    time.Duration  `json:"-"`
	Outputs     map[string]any `json:"outputs,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Attempts    []Attempt      `json:"attempts,omitempty"`
	Hooks       []Hook         `json:"hooks,omitempty"`
	Lanes       []Lane         `json:"lanes,omitempty"`
}

// Hook is one API hook step of a request node.
type Hook struct {
	Phase       string        `json:"phase"`
	Level       string        `json:"level"`
	Step        string        `json:"step"`
	Index       int           `json:"index"`
	Status      string        `json:"status"`
	Duration    time.Duration `json:"-"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	Nodes       []Node        `json:"nodes,omitempty"`
}

// Lane is one group of a container's children.
type Lane struct {
	Label string `json:"label"`
	Nodes []Node `json:"nodes"`
}

// Attempt is one HTTP attempt of a request.
type Attempt struct {
	N        int           `json:"n"`
	Method   string        `json:"method"`
	URL      string        `json:"url"`
	Status   int           `json:"status,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"-"`
}

// Diagnostic is a diagnostic with its message already rendered.
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

// Note is the text of a note node.
type Note struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// VersionUse records which version of a request a node ran.
type VersionUse struct {
	Path        string `json:"path"`
	RequestID   string `json:"request_id"`
	RequestPath string `json:"request_path"`
	Version     string `json:"version"`
	Source      string `json:"source"`
}

// Abort says why a cancelled run was forced to stop.
type Abort struct {
	Reason   string        `json:"reason"`
	Grace    time.Duration `json:"-"`
	Inflight []string      `json:"inflight"`
}

// Leaves returns every non-container node, depth-first in list order.
// Nodes inside hooks belong to their request node and are not included.
func (r Run) Leaves() []Node {
	var out []Node
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if len(n.Lanes) == 0 {
				out = append(out, n)
				continue
			}
			for _, l := range n.Lanes {
				walk(l.Nodes)
			}
		}
	}
	walk(r.Nodes)
	return out
}

// Find returns the node with path, searching lanes and hooks.
func (r Run) Find(path string) (Node, bool) {
	return findIn(r.Nodes, path)
}

func findIn(nodes []Node, path string) (Node, bool) {
	for _, n := range nodes {
		if n.Path == path {
			return n, true
		}
		for _, l := range n.Lanes {
			if found, ok := findIn(l.Nodes, path); ok {
				return found, true
			}
		}
		for _, h := range n.Hooks {
			if found, ok := findIn(h.Nodes, path); ok {
				return found, true
			}
		}
	}
	return Node{}, false
}
