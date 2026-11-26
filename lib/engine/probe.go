// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// ProbeStatus is how a connection test ended.
type ProbeStatus string

// The ways a connection test ends.
const (
	ProbeOK          ProbeStatus = "ok"          // connected and signed in
	ProbeFailed      ProbeStatus = "failed"      // the service could not be reached, or refused the sign-in
	ProbeUnsupported ProbeStatus = "unsupported" // this kind of request has no connection test
)

// Probe is the result of testing the connection of a request without sending it.
type Probe struct {
	Status ProbeStatus
	// Target is where the test went, as a send records it: secrets masked.
	Target string
	// Detail is what happened in a sentence: what the service said, with secrets masked, or why there was no test.
	Detail string
	Took   time.Duration
	// Diags are problems that kept the test from starting: an unknown variable, a missing secret. Status is failed.
	Diags diag.List
}

// probeTimeout bounds a test whose request sets no timeout.
const probeTimeout = 15 * time.Second

// ProbeSend renders the request in in as a send would after its pre-hooks, then tests its connection: it connects
// to the broker or database, signs in with the credentials the request names and disconnects. Nothing is sent,
// published, read or changed, and no hook runs. A kind of request with no safe test says so (ProbeUnsupported).
func ProbeSend(ctx context.Context, d Deps, in SendInput) Probe {
	r := newRunState(d, in.Seed, nil)
	r.closed = true // a test is not a send: nothing listens to its events, and an HTTP attempt would wait on them
	r.in, r.kind, r.env, r.prefer = in, "send", in.Env, in.PreferVersion
	x, diags := r.prepare(sendCall(in))
	if diags.HasErrors() {
		return Probe{Status: ProbeFailed, Diags: diags}
	}
	return x.exchange().probe(ctx)
}

// bounded is ctx with the request's timeout, or probeTimeout when it sets none.
func (x *execution) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	limit := x.cfg.timeout
	if limit <= 0 {
		limit = probeTimeout
	}
	return context.WithTimeout(ctx, limit)
}

func (x *execution) probeDone(target string, took time.Duration, err error) Probe {
	p := Probe{Target: target, Took: took}
	switch {
	case err == nil:
		p.Status, p.Detail = ProbeOK, "connected and signed in"
	case errors.Is(err, transport.ErrNoProbe):
		p.Status, p.Detail = ProbeUnsupported, err.Error()+": send the request to try it"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, dataset.ErrTimeout):
		p.Status, p.Detail = ProbeFailed, "no answer in time"
	default:
		p.Status, p.Detail = ProbeFailed, x.r.secrets.String(driverHint(err.Error()))
	}
	return p
}

// probeHTTP is the test of an HTTP request: the same request, rendered and signed in as a send would, goes out as a
// HEAD with no body, which asks for the headers of the answer and changes nothing on a server that follows HTTP.
//
// Before: an HTTP request had no connection test, so a wrong host, a bad certificate or a refused token showed only
// when the request itself was sent.
func (e httpExchange) probe(ctx context.Context) Probe {
	x := e.x
	b, diags := x.render(false)
	diags = append(diags, x.r.secrets.problems(x.env, b.missingSecrets)...)
	if diags.HasErrors() {
		return Probe{Status: ProbeFailed, Diags: diags}
	}
	b.request.Method, b.request.Body = http.MethodHead, nil
	b.request.Header = b.request.Header.Clone()
	b.request.Header.Del("Content-Length")
	ctx, cancel := x.bounded(ctx)
	defer cancel()
	started := time.Now()
	resp, ok := x.send(ctx, b)
	took := time.Since(started)
	if !ok {
		if out := x.outcome(); len(out.diags) > 0 {
			return Probe{Status: ProbeFailed, Target: b.sent.URL, Took: took, Diags: out.diags}
		}
		return Probe{Status: ProbeFailed, Target: b.sent.URL, Took: took, Detail: "no answer in time"}
	}
	p := Probe{Target: b.sent.URL, Took: took}
	p.Status, p.Detail = headVerdict(resp.Status)
	return p
}

// headVerdict is what the status a HEAD was answered with says of the address and the sign-in.
func headVerdict(code int) (ProbeStatus, string) {
	text := strconv.Itoa(code) + " " + http.StatusText(code)
	switch {
	case code >= 200 && code < 400:
		return ProbeOK, text
	case code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented:
		return ProbeOK, "reached, but the server does not answer HEAD (" + strconv.Itoa(code) + ")"
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return ProbeFailed, text + ": check the authentication"
	case code == http.StatusNotFound || code == http.StatusGone:
		return ProbeFailed, text + ": nothing at this address"
	}
	return ProbeFailed, text
}

// probe of a gRPC request connects to the server and sends the call's metadata, then stops: no method is called. A
// gRPC-Web request goes over plain HTTP, which has no such test.
func (e rpcExchange) probe(ctx context.Context) Probe {
	x := e.x
	b, diags := x.renderGRPC(false)
	diags = append(diags, x.r.secrets.problems(x.env, b.missing)...)
	if diags.HasErrors() {
		return Probe{Status: ProbeFailed, Diags: diags}
	}
	if b.call.Web {
		return Probe{Status: ProbeUnsupported, Target: b.sent.URL, Detail: "a gRPC-Web request has no connection test: send it to try it"}
	}
	ctx, cancel := x.bounded(ctx)
	defer cancel()
	started := time.Now()
	err := grpcx.Probe(ctx, b.call)
	p := x.probeDone(b.sent.URL, time.Since(started), err)
	if p.Status == ProbeOK {
		p.Detail = "connected to the server"
		if len(b.call.Protos) > 0 {
			p.Detail += " and the .proto files compile"
		}
	}
	return p
}

func (e messagingExchange) probe(ctx context.Context) Probe {
	x := e.x
	b, diags := x.renderAsync(false)
	return x.probeAsync(ctx, b, diags)
}

// probeAsync tests the connection of a rendered operation.
func (x *execution) probeAsync(ctx context.Context, b builtAsync, diags diag.List) Probe {
	diags = append(diags, x.r.secrets.problems(x.env, b.missing)...)
	if diags.HasErrors() {
		return Probe{Status: ProbeFailed, Diags: diags}
	}
	tr, ok := x.r.registry().Lookup(b.protocol)
	if !ok {
		return Probe{Status: ProbeFailed, Diags: diag.List{diag.New(diag.CodeAsyncProtocol, diag.Location{}, map[string]string{"protocol": b.protocol})}}
	}
	ctx, cancel := x.bounded(ctx)
	defer cancel()
	started := time.Now()
	err := transport.Probe(ctx, tr, b.op)
	return x.probeDone(b.sent.URL, time.Since(started), err)
}

func (e datasetExchange) probe(ctx context.Context) Probe {
	x := e.x
	b, diags := x.renderDataset(false)
	diags = append(diags, x.r.secrets.problems(x.env, b.missing)...)
	if diags.HasErrors() {
		return Probe{Status: ProbeFailed, Diags: diags}
	}
	ctx, cancel := x.bounded(ctx)
	defer cancel()
	if !slices.Contains(model.DatasetDrivers(), b.query.Driver) {
		return e.probeFile(ctx, b)
	}
	q := b.query
	q.Statement, q.Params, q.Limit, q.AllowWrites = "SELECT 1", nil, 1, false
	started := time.Now()
	_, err := x.r.datasetRunner().Run(ctx, q)
	return x.probeDone(b.sent.URL, time.Since(started), err)
}

// tableLister is a data file source that can read the tables of a file without running a query.
type tableLister interface {
	Tables(ctx context.Context, q dataset.Query) ([]dataset.TableInfo, error)
}

// probeFile is the test of a data file request: the file (or the files of the folder) must exist, be inside what the
// request may read and be a valid file of its format; the tables found are what the query can read.
//
// Before: a data file request had no test, so a wrong path or a file that is not what its extension says showed only
// when the query ran.
func (e datasetExchange) probeFile(ctx context.Context, b builtDataset) Probe {
	lister, ok := e.x.r.datasetRunner().(tableLister)
	if !ok {
		return Probe{Status: ProbeUnsupported, Target: b.sent.URL, Detail: "this data file source cannot be checked: send the request to read it"}
	}
	started := time.Now()
	tables, err := lister.Tables(ctx, b.query)
	p := e.x.probeDone(b.sent.URL, time.Since(started), err)
	if p.Status == ProbeOK {
		p.Detail = fileFound(b.query.Driver, tables)
	}
	return p
}

// fileFound says what a data file turned out to hold, in a sentence.
func fileFound(format string, tables []dataset.TableInfo) string {
	var names []string
	rows := 0
	for _, t := range tables {
		names = append(names, t.Name)
		rows += t.Rows
	}
	noun := "tables"
	if len(tables) == 1 {
		noun = "table"
	}
	return fmt.Sprintf("the file exists and reads as %s: %d %s (%s), %d rows", strings.ToUpper(format), len(tables), noun, strings.Join(names, ", "), rows)
}
