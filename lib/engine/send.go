// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"net/http"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/authx"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/vault"
)

// Lookup gives the engine the loaded collections. The facade implements
// it over the workspace.
type Lookup interface {
	Collection(id model.ID) (model.Collection, bool)
	// Request and Flow resolve a reference made from collection from
	// (cross-collection references name their collection).
	Request(ref model.Ref, from model.ID) (model.Collection, model.Request, bool)
	Flow(ref model.Ref, from model.ID) (model.Collection, model.Flow, bool)
	// Settings resolves every layer for a request version in folder
	// (collection-relative, e.g. "requests/auth"), including the user,
	// workspace, environment-variable and flag layers.
	Settings(collection model.ID, folder string, version model.Settings) settings.Resolved
}

// SendInput is one standalone send.
type SendInput struct {
	Collection    model.Collection
	Request       model.Request
	Env           string // environment name; "" → defaults.environment; still "" → no environment
	PinnedVersion string
	PreferVersion string
	With          map[string]string // templates, rendered once against env and generators
	Hooks         model.HookControl
	Seed          uint64 // 0 → a random seed, reported in the result
	Save          string // "" → the responses.save setting

	// Overlay, when set, is laid over the resolved version before anything is
	// rendered or sent. What does not fit is reported as warnings.
	Overlay *Overlay
}

// Overlay is a flowi overlay chosen for a send: its name and the operations
// that apply to the request (shared part first).
type Overlay struct {
	Name string
	Ops  []model.OverlayOp
}

// sendCall is the call a standalone send makes.
func sendCall(in SendInput) call {
	return call{collection: in.Collection, request: in.Request, pinned: in.PinnedVersion, with: in.With, hooks: in.Hooks, save: in.Save, overlay: in.Overlay}
}

// SchemaChecker validates a response against the schemas stored for its
// request and learns from it (the facade implements it). Its diagnostics are
// notes on the send: they never change its status.
type SchemaChecker interface {
	Check(c model.Collection, r model.Request, version string, rec Record) diag.List
}

// SchemaMatcher is a SchemaChecker that can also check one body against the
// response schema kept for a request, for a flow's schema check. found is
// false when no schema is kept for that status.
type SchemaMatcher interface {
	Match(c model.Collection, r model.Request, version string, status int, body any) (violations []SchemaViolation, found bool)
}

// SchemaViolation is one place a body does not match its schema.
type SchemaViolation struct {
	Path    string
	Message string
}

// Deps are the engine's collaborators.
type Deps struct {
	Lookup  Lookup
	Schemas SchemaChecker // nil: bodies are not checked or learned
	// Transports runs async requests, by protocol (nil → every protocol this
	// build speaks); tests pass fakes.
	Transports transport.Registry
	// GRPC makes gRPC calls (nil → the real client, grpcx.Run); tests pass fakes.
	GRPC    GRPCRunner
	Vault   *vault.Vault
	Environ []string // for the proxy fallback (HTTP_PROXY, …)
	DataDir func(collection model.ID) string
	Clock   func() time.Time
	NewID   func() string // ULIDs for runs and records
	// After waits like time.After (nil → time.After); tests pass a
	// controllable clock so delays and grace periods need no sleeping.
	After func(time.Duration) <-chan time.Time
	// Nodes returns a node type's runner (nil → nodes.Lookup); tests add
	// recording or blocking node types.
	Nodes func(model.NodeType) (nodes.Runner, bool)
	// Observe, when set, receives every event of every run as it is
	// emitted, in order, on the emitting goroutine under the run's emit
	// lock. It must return quickly and must not call back into the run.
	Observe func(Event)
	// UserAgent is the User-Agent requests send unless a setting or the
	// request names one ("" → "Flowi").
	UserAgent string
	// Traceparent, when set, returns the W3C traceparent header value for
	// the call made by node path ("" for a standalone send) of run runID.
	// It is added to outgoing requests that do not carry one already.
	Traceparent func(runID, path string) string
	// AuthTokens keeps OAuth 2.0 tokens between sends (nil: every send asks
	// the token endpoint again).
	AuthTokens *authx.TokenCache
	// Transport, when set, sends every HTTP attempt instead of the network (nil: the real one). A run played
	// back from a recording uses it.
	Transport http.RoundTripper
}

// Observers combines observers into one for Deps.Observe, calling each in
// order; nil entries are skipped. It returns nil when there is none, so a
// run with no observer pays nothing per event.
func Observers(fns ...func(Event)) func(Event) {
	var live []func(Event)
	for _, f := range fns {
		if f != nil {
			live = append(live, f)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	}
	return func(ev Event) {
		for _, f := range live {
			f(ev)
		}
	}
}

// SendResult is a finished send. Everything in it is masked.
type SendResult struct {
	RunID  string
	Seed   uint64
	Status Status
	Record *Record        // nil when nothing was sent
	Vars   map[string]any // variables set by hooks
	Diags  diag.List
	Saved  []string // files written (saved responses)
	// Err is a system failure that did not stop the send (saving a
	// response or the cookie jar failed).
	Err error
}

const eventBuffer = 64

// Send is a send in progress.
//
// Goroutine ownership: StartSend starts one goroutine that runs the send.
// It closes Events after the last event (RunFinished or RunCancelled) and
// then completes Wait. Events must be read: a full buffer blocks the run.
type Send struct {
	events chan Event
	done   chan struct{}
	result SendResult
}

// StartSend starts sending in.Request. Cancelling ctx aborts the request in
// flight; nothing is saved then.
func StartSend(ctx context.Context, d Deps, in SendInput) *Send {
	s := &Send{events: make(chan Event, eventBuffer), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		defer close(s.events)
		s.result = newRunner(d, in, s.events).run(ctx)
	}()
	return s
}

// Events streams the send's events; it is closed when the send ends.
func (s *Send) Events() <-chan Event { return s.events }

// Wait blocks until the send has ended and returns its result.
func (s *Send) Wait() SendResult {
	<-s.done
	return s.result
}
