// SPDX-License-Identifier: Apache-2.0

// Package arazzo imports Arazzo 1.0/1.1 workflows as Flowi flows and
// exports flows as Arazzo 1.0.1. Both are pure transforms over model
// values. Flowi and Arazzo overlap but do not match: every element that
// does not carry over is reported (FLW-IMPORT-001, FLW-EXPORT-001/002)
// and, on import, kept visible as a note node — nothing is guessed.
package arazzo

import (
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Operation is what export needs to know about a request: its OpenAPI
// operationId and its parameters by with key (openapi.ParamKey of the
// name). With keys that are not parameters are sent as body fields.
type Operation struct {
	ID     string
	Params map[string]Param
	// Async operations (AsyncAPI) set the rest: Source names the source
	// description the operation is in ("" → the OpenAPI one), Action is send
	// or receive, Timeout is how long a receive waits, and Reply and Match
	// say the request also waits for a reply, or filters the messages it
	// counts: neither has a place in an Arazzo step.
	Source  string
	Action  string
	Timeout time.Duration
	Reply   bool
	Match   bool
	// GRPC marks a call to a gRPC request: no standard description of it
	// exists, so its step is a custom one that keeps the call in x-flowi-flow.
	GRPC bool
}

// AsyncSource is the name of the generated AsyncAPI description in a
// document's sourceDescriptions.
const AsyncSource = "flowi-async"

// Param is an OpenAPI parameter: its real name and where it goes
// ("path", "query", "header" or "cookie").
type Param struct {
	Name, In string
}

// Resolver maps between operations and the collection's requests.
// RequestFor returns the request an operation was imported as and the
// version label that operation is ("" when any version will do).
// OperationOf describes the operation a request call runs, and is false
// when the call runs a version no operation is known for.
type Resolver interface {
	RequestFor(operationID string) (model.Ref, string, bool)
	OperationOf(call model.RequestCall) (Operation, bool)
}

// Options configure Import.
type Options struct {
	NewID   func() model.ID // ids for new flows
	Resolve Resolver
}

// Result is what an Arazzo document becomes: one flow per workflow, with
// Path relative to flows/ (arazzo/<workflowId>).
type Result struct {
	Flows []model.Flow
}

// ExportOptions configure Export.
type ExportOptions struct {
	Resolve Resolver
	// Source names the OpenAPI source description the steps call;
	// SourceURL is its url (default "./<Source>.openapi.yaml").
	Source    string
	SourceURL string
	// AsyncSourceURL is the url of the AsyncAPI source description a flow
	// with async steps adds (default "./requests.asyncapi.yaml").
	AsyncSourceURL string
	Title          string // info.title (default: the first flow's name)
}

// FlowVersion is the Arazzo version flows are stored as (flows/*.arazzo.yaml).
const FlowVersion = "1.1.0"

// BuiltinOperation is the operation a custom step calls: a step for a node
// Arazzo has no word for. The generated description declares it, so the step
// resolves in any Arazzo tool, which cannot run it; Flowi runs the node it
// stands for.
const BuiltinOperation = "flowiCustomStep"

// SourceDescription is one entry of a document's sourceDescriptions.
type SourceDescription struct{ Name, URL, Type string }

// FlowBody is the YAML form of a flow's own data (the store's flow codec). It
// is kept whole inside the document as x-flowi-flow, so nothing a flow holds
// can be lost by the Arazzo form.
type FlowBody interface {
	EncodeFlow(model.Flow) ([]byte, error)
	DecodeFlow(file, path string, data []byte) (model.Flow, diag.List)
}

// FlowOptions configure EncodeFlow and DecodeFlow.
type FlowOptions struct {
	Body    FlowBody
	Resolve Resolver
	// Sources are the document's source descriptions; the first is the
	// collection's generated description and must be named "flowi".
	Sources []SourceDescription
	NewID   func() model.ID // an id for a flow read from a foreign document
}
