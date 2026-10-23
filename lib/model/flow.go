// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// Input is a declared flow input with an optional default template.
type Input struct {
	Name    string
	Default string
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
	Path          string
	PreferVersion string
	Inputs        []Input
	Outputs       []KeyValue // name → template rendered when the flow ends (subflows)
	Execution     Execution
	Run           *RunTill
	Nodes         []Node
	File          string
}
