// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// InputTypes are the types a flow input or output may declare; "" means any.
func InputTypes() []string {
	return []string{"string", "number", "boolean", "object", "array"}
}

// Input is a declared flow input with an optional type and default template.
// A flow keeps its inputs in the order they were written: the Start card shows
// them in that order.
type Input struct {
	Name    string
	Type    string
	Default string
}

// Output is a value a flow hands back when it ends: a subflow's caller reads
// it as nodes.<id>.<name>, and the End card shows it. Value is a template
// rendered in the flow's final scope.
type Output struct {
	Name  string
	Type  string
	Value string
}

// Execution controls where concurrency is allowed to go. RateLimit
// applies to every host; HostRateLimits to the hosts they name.
type Execution struct {
	MaxConcurrency int
	RateLimit      string
	HostRateLimits []KeyValue
	OnBranchError  string
}

// RunTill repeats a whole flow until a condition or limit.
type RunTill struct {
	Until         string
	MaxIterations int
	MaxDuration   time.Duration
	Interval      time.Duration
}

// Flow is a graph of nodes. Path is relative to flows/ without the
// .flow.yaml suffix.
type Flow struct {
	Meta
	// Tags label the flow for `flowi run --tag`; the tags of its folders apply as well.
	Tags          []string
	Path          string
	PreferVersion string
	Inputs        []Input
	Outputs       []Output // rendered when the flow ends, in this order
	Execution     Execution
	Run           *RunTill
	Nodes         []Node
	File          string
}
