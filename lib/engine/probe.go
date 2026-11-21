// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/diag"
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

func (httpExchange) probe(context.Context) Probe {
	return Probe{Status: ProbeUnsupported, Detail: "an HTTP request has no connection test: send it to try it"}
}

func (rpcExchange) probe(context.Context) Probe {
	return Probe{Status: ProbeUnsupported, Detail: "a gRPC request has no connection test yet: send it to try it"}
}

func (e messagingExchange) probe(ctx context.Context) Probe {
	x := e.x
	b, diags := x.renderAsync(false)
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
	if !slices.Contains(model.DatasetDrivers(), b.query.Driver) {
		return Probe{Status: ProbeUnsupported, Target: b.sent.URL, Detail: "a data file has no connection to test: send the request to read it"}
	}
	ctx, cancel := x.bounded(ctx)
	defer cancel()
	q := b.query
	q.Statement, q.Params, q.Limit, q.AllowWrites = "SELECT 1", nil, 1, false
	started := time.Now()
	_, err := x.r.datasetRunner().Run(ctx, q)
	return x.probeDone(b.sent.URL, time.Since(started), err)
}
