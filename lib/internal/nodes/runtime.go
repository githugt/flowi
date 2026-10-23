// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
	"github.com/githugt/flowi-lib/template"
)

// Phase is when a node runs relative to its request.
type Phase int

// Phases. PhaseFlow is a node of a flow, with no request in reach.
const (
	PhaseFlow Phase = iota
	PhasePre
	PhasePost
)

// Masker hides secrets in text (vault.Masker, or the engine's live one).
type Masker interface{ String(s string) string }

// Status is a node or lane outcome. The values match engine.Status.
type Status string

// Statuses.
const (
	StatusPassed    Status = "passed"
	StatusFailed    Status = "failed"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
)

// Emitter receives what nodes report while running. Implementations must
// be safe for concurrent use: nodes in different lanes call it at once.
type Emitter interface {
	Log(node, message string)
	Warn(d diag.Diagnostic)
	Digest(node, format, text string)
}

// Outcome is what a node produced: Outputs become nodes.<id> (written
// once). An empty Status means failed when Diags has errors, else passed.
type Outcome struct {
	Outputs map[string]any
	Diags   diag.List
	Status  Status
}

// Runtime is what the engine does for nodes: everything that needs the
// scheduler, the network, the clock or the workspace. Nodes describe the
// work; they never start goroutines or open connections themselves.
type Runtime interface {
	// Lanes runs node lists (parallel branches, batch or run_till
	// iterations) under the run's concurrency and branch-error policies.
	// Results are in lane order.
	Lanes(ctx context.Context, lanes []Lane, o LaneOptions) []LaneResult
	Request(ctx context.Context, c RequestCall) Outcome
	Subflow(ctx context.Context, ref model.Ref, inputs map[string]any) Outcome
	HTTP(ctx context.Context, c HTTPCall) HTTPResult
	// Background sends c without waiting (webhook wait: false); failures
	// become FLW-RUN-016 warnings.
	Background(node string, c HTTPCall)
	Sleep(ctx context.Context, d time.Duration) error
	// StopRun ends the run: running steps finish, no step that has not
	// started will (a stop node). Stopped reports it, for loops.
	StopRun()
	Stopped() bool
	Now() time.Time
	RandomDuration(lo, hi time.Duration) time.Duration // seeded by the run
	// Report is the report so far of the whole run ("run") or of a
	// finished node, by id, as seen from the calling node.
	Report(of string) (report.Run, bool)
	// WriteFile writes under the run's data directory ("saved" or
	// "digests"). name is one or more "/"-separated segments, each a
	// plain file name; anything else wraps ErrFileName.
	WriteFile(dir, name string, data []byte, appendTo bool) (string, error)
}

// ErrFileName is wrapped by Runtime.WriteFile when a name is not a plain
// file name (FLW-RUN-017).
var ErrFileName = errors.New("not a plain file name")

// Lane is one node list a container runs.
type Lane struct {
	Label    string // "[3]", "b1", "then"
	Nodes    []model.Node
	OwnVars  bool // writes stay in the lane (batch iterations)
	OwnNodes bool // nodes.<id> stay in the lane (batch, run_till)
	Item     any
	Index    int
	HasItem  bool // item and index are in scope
}

// LaneOptions say how lanes run.
type LaneOptions struct {
	Concurrency int    // lanes in flight; capped by the run-wide limit
	Wait        string // "all" (default), "any" or "first-success"
	LaneJars    bool   // give lanes their own cookie jars per http.cookies
}

// LaneResult is one lane's outcome. Vars and Nodes are the lane's own
// layers (empty unless the lane had them). First marks the lane that
// settled a wait of any or first-success.
type LaneResult struct {
	Status Status
	Vars   map[string]any
	Nodes  map[string]any
	Diags  diag.List
	First  bool
}

// RequestCall is a saved request to run from a request node.
type RequestCall struct {
	Ref     model.Ref
	Version string
	Hooks   model.HookControl
	With    map[string]any
	Headers map[string]string
}

// HTTPCall is an ad-hoc HTTP call (fetch, webhook, digest webhook). A
// string Body is sent as text; any other non-nil value as JSON.
type HTTPCall struct {
	Method, URL    string
	Headers, Query map[string]string
	Body           any
}

// HTTPResult is an ad-hoc call's response (nil when it failed).
type HTTPResult struct {
	Response *Response
	Status   Status
	Diags    diag.List
}

// State is what a node can read and change.
type State struct {
	Phase    Phase
	Path     string               // the node instance's path, e.g. users[2]/create
	Vars     *Vars                // vars.<name>
	Request  *MutableRequest      // overrides for the request; nil outside pre-hooks
	Response *Response            // nil before the response
	Cookies  []httpx.StoredCookie // the run's cookie jar, for extract from: cookie
	// Scope returns the template scope for the current state (vars,
	// request, response, env, secrets, …); it is rebuilt by the engine on
	// each call so nodes always see the latest values.
	Scope  func() template.Scope
	Emit   Emitter
	Masker Masker
	// Secret registers a value derived from a secret (a template result
	// that read secret.*), so it is masked wherever it appears later. Nil
	// means nothing is registered.
	Secret func(v any)
	// Runtime does what nodes cannot do alone; nil only in contexts
	// that need none of it.
	Runtime Runtime
}

// derived registers r's value as secret material when it read a secret.
func (s *State) derived(r template.Result) {
	if r.Tainted && s.Secret != nil {
		s.Secret(r.Value)
	}
}

// MutableRequest collects what pre-hook set nodes change. The engine
// applies it on top of the rendered request, so overrides win. Mu guards
// every field: set nodes in parallel hook branches write concurrently.
type MutableRequest struct {
	Mu        sync.Mutex
	Method    *string
	URL       *string
	Headers   map[string]string
	Query     map[string]string
	Body      *any           // replaces the whole body
	BodyPaths map[string]any // dot paths into a JSON body
	Cookies   map[string]string
}

// Response is the response as nodes see it.
type Response struct {
	Status int
	Header http.Header
	Body   any    // parsed JSON, or the text
	Text   string // the body as text (for regex extraction)
	Time   time.Duration
}

// Values is the response scope for templates: status, headers (by lower-
// case name), body, text, time and cookies (by name).
func (r *Response) Values() map[string]any {
	headers := map[string]any{}
	for name, values := range r.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	cookies := map[string]any{}
	for _, c := range r.cookies() {
		cookies[c.Name] = c.Value
	}
	return map[string]any{"status": r.Status, "headers": headers, "body": r.Body, "text": r.Text, "time": r.Time, "cookies": cookies}
}

func (r *Response) cookies() []*http.Cookie {
	return (&http.Response{Header: r.Header}).Cookies()
}

func (m *MutableRequest) setHeader(name, v string) {
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	m.Headers[name] = v
}
