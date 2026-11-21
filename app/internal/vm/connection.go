// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// ConnectionTest is what the last "Test connection" of a request found. The zero value is "not tried".
type ConnectionTest struct {
	Busy bool // a test is running
	// Status is engine.ProbeOK, ProbeFailed or ProbeUnsupported; "" when not tried.
	Status engine.ProbeStatus
	Target string // where it went, secrets masked
	Detail string // what happened, or why nothing was tried
	Took   time.Duration
	// For is what the test was made for (see connectionKey); a result is shown only while the request still has it.
	key string
}

// CanTestConnection says whether the selected version is a kind of request that can be tested without sending it:
// a message or a database query. HTTP, gRPC and data files cannot.
func (r *Request) CanTestConnection() bool {
	v := r.Current()
	if v.Async != nil {
		return true
	}
	d := v.SQL()
	return d != nil && v.Dataset != nil
}

// Connection is the last test of the selected version's connection, empty once the server, the credentials or the
// protocol it was made for have changed.
func (r *Request) Connection() ConnectionTest {
	if r.conn.Busy || r.conn.key == connectionKey(r.Current()) {
		return r.conn
	}
	return ConnectionTest{}
}

// connectionKey is what a connection test depends on: the target and how it signs in. An edit to anything else (the
// body, a header, the channel's message) leaves the result standing.
func connectionKey(v model.RequestVersion) string {
	var b strings.Builder
	if a := v.Async; a != nil {
		b.WriteString(a.Protocol + "|" + a.Server)
		for _, kv := range a.Binding {
			b.WriteString("|" + kv.Key + "=" + kv.Value)
		}
		for _, kv := range v.Headers {
			if !kv.Disabled {
				b.WriteString("|h:" + kv.Key + "=" + kv.Value)
			}
		}
	}
	if d := v.Dataset; d != nil {
		b.WriteString(d.Driver + "|" + d.Server)
	}
	return b.String()
}

// TestConnection tests the selected version's connection in the background, then calls the request's change
// callbacks. It connects and signs in, and sends, reads and changes nothing.
func (s *Send) TestConnection(req *Request) {
	if !req.CanTestConnection() || req.conn.Busy {
		return
	}
	version, target, env := req.Current(), req.Target(), s.ws.SelectedEnv()
	req.conn = ConnectionTest{Busy: true, key: connectionKey(version)}
	req.changed()
	type outcome struct {
		p   engine.Probe
		err error
	}
	Run(s.w, func() outcome {
		p, err := s.b.TestConnection(context.Background(), target, version, flowi.SendOptions{Env: env, Version: version.Label})
		return outcome{p, err}
	}, func(o outcome) {
		res := ConnectionTest{key: connectionKey(version), Status: o.p.Status, Target: o.p.Target, Detail: o.p.Detail, Took: o.p.Took}
		switch {
		case o.err != nil:
			res.Status, res.Detail = engine.ProbeFailed, ErrorText(o.err)
		case o.p.Diags.HasErrors():
			res.Status, res.Detail = engine.ProbeFailed, ErrorText(&diag.ListError{List: o.p.Diags})
		}
		req.conn = res
		req.changed()
	})
}
