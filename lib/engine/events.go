// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"time"

	"github.com/githugt/flowi/lib/diag"
)

// Status is the outcome of a run, hook or node.
type Status string

// Statuses.
const (
	StatusPassed    Status = "passed"
	StatusFailed    Status = "failed"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
	// StatusAborted is a cancelled run that did not stop within its grace
	// period.
	StatusAborted Status = "aborted"
)

// Event is something that happened during a run. EventName is the
// CapitalCase name used in logs and NDJSON output.
type Event interface{ EventName() string }

// RunStarted opens a run's event stream. Kind is "send" or "flow"; the
// Flow fields are set for flow runs.
type RunStarted struct {
	RunID         string
	Kind          string
	FlowID        string
	FlowPath, Env string
	Overlay       string // the run-level flowi overlay, if any
	Seed          uint64
	At            time.Time
}

// NodeStarted marks the start of a node instance. Path is unique in the
// run (users[2]/create); Parent is the enclosing container or request
// node's path ("" at the top) and Lane the lane label within it ("[2]",
// "b1", "then", "pre:request:0"). Depth is the hook nesting depth.
type NodeStarted struct {
	RunID, Path, Parent, Lane string
	Node, Type, Name          string
	Depth                     int
}

// NodeFinished reports a node instance's outcome. Outputs are masked and
// leave out request/fetch bodies and headers (they are in the response
// record). Handled means on_error let the run continue past a failure.
type NodeFinished struct {
	RunID, Path, Parent, Lane string
	Node, Type                string
	Status                    Status
	Handled                   bool
	Duration                  time.Duration
	Outputs                   map[string]any
	Diags                     diag.List
	Depth                     int
	// Reason says why a node was skipped by the person ("skipped by you");
	// "" otherwise, including a node skipped by its when.
	Reason string
}

// RecordCaptured carries the record of a request node's call: the request as
// sent and the response (or messages), masked like every record the engine
// hands out. Path and Node are the request node's, as in NodeFinished; it comes
// before that node's NodeFinished. A node that sent nothing (it failed before
// sending, or was skipped) has none. Nothing else changes: the report and the
// saved responses are as before.
type RecordCaptured struct {
	RunID, Path, Node string
	Depth             int
	Record            Record
}

// AuthPrompted says a request is waiting for the person to do something
// before it can be sent: Kind "open" asks them to open URL in a browser and
// approve; "device" asks them to open URL on any device and enter Code. It
// arrives while the request waits, so a window shows it and a terminal prints
// it. Expires is when the request stops being valid.
type AuthPrompted struct {
	RunID, Path string // Path is the request node's path; "" for a standalone send
	Kind        string
	URL, Code   string
	Expires     time.Time
}

// HookStarted marks the start of one hook step. Level is collection,
// folder, request or version; Phase is pre or post; Step names the step
// (a flow path, or "inline"); Index is its position in its level. Path is
// the request node's path ("" for a standalone send).
type HookStarted struct {
	RunID, Path, Level, Phase, Step string
	Index, Depth                    int
}

// HookFinished reports a hook step's outcome.
type HookFinished struct {
	RunID, Path, Level, Phase, Step string
	Index, Depth                    int
	Status                          Status
	Duration                        time.Duration
	Diags                           diag.List
}

// RequestSent is emitted for every attempt; URL is masked. Path is the
// node making the call ("" for a standalone send).
type RequestSent struct {
	RunID, Path, RequestID, Version string
	RequestPath                     string // the saved request's path; "" for ad-hoc calls
	VersionSource                   string // pinned, preferred, default, highest
	Method, URL                     string
	Attempt                         int
	Depth                           int // 0 for the request being sent, >0 inside hook flows
}

// ResponseReceived reports an attempt's response (Status 0 when the
// attempt failed; Error then says why, masked).
type ResponseReceived struct {
	RunID, Path, RequestID string
	Status                 int
	Error                  string
	Duration               time.Duration
	Wait                   time.Duration // backoff before the next attempt, if any
	Attempt                int
	Depth                  int
	Outcome                string // async requests: ok, timeout or error (Status stays 0)
}

// MessageReceived reports a message an async request has received and kept,
// while it is still waiting for more. Seq counts from 1. The message is masked.
type MessageReceived struct {
	RunID, Path, RequestID string
	Seq                    int
	Message                RecordMessage
	Depth                  int
}

// Log is a message from a log node, masked.
type Log struct {
	RunID, Path, Node, Message string
}

// Warning carries a warning diagnostic (masked args).
type Warning struct {
	RunID string
	Diag  diag.Diagnostic
}

// Digest is a digest node's summary for the stdout sink, masked.
type Digest struct {
	RunID, Path, Node, Format, Text string
}

// RunCancelled means the run's context was cancelled.
type RunCancelled struct{ RunID string }

// RunAborted means a cancelled run did not stop within its grace period:
// Inflight are the node paths still running, in start order.
type RunAborted struct {
	RunID    string
	Reason   string // grace_period_exceeded
	Grace    time.Duration
	Inflight []string
}

// RunFinished closes a run's event stream.
type RunFinished struct {
	RunID    string
	Status   Status
	Duration time.Duration
}

// EventName returns "RunStarted".
func (RunStarted) EventName() string { return "RunStarted" }

// EventName returns "NodeStarted".
func (NodeStarted) EventName() string { return "NodeStarted" }

// EventName returns "NodeFinished".
func (NodeFinished) EventName() string { return "NodeFinished" }

// EventName returns "RunAborted".
func (RunAborted) EventName() string { return "RunAborted" }

// EventName returns "AuthPrompted".
func (AuthPrompted) EventName() string { return "AuthPrompted" }

// EventName returns "RecordCaptured".
func (RecordCaptured) EventName() string { return "RecordCaptured" }

// EventName returns "HookStarted".
func (HookStarted) EventName() string { return "HookStarted" }

// EventName returns "HookFinished".
func (HookFinished) EventName() string { return "HookFinished" }

// EventName returns "RequestSent".
func (RequestSent) EventName() string { return "RequestSent" }

// EventName returns "ResponseReceived".
func (ResponseReceived) EventName() string { return "ResponseReceived" }

// EventName returns "MessageReceived".
func (MessageReceived) EventName() string { return "MessageReceived" }

// EventName returns "Log".
func (Log) EventName() string { return "Log" }

// EventName returns "Warning".
func (Warning) EventName() string { return "Warning" }

// EventName returns "Digest".
func (Digest) EventName() string { return "Digest" }

// EventName returns "RunCancelled".
func (RunCancelled) EventName() string { return "RunCancelled" }

// EventName returns "RunFinished".
func (RunFinished) EventName() string { return "RunFinished" }
