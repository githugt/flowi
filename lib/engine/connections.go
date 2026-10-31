// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/transport"
)

// connUse is how a request node uses a connection kept open between steps.
type connUse struct {
	open       string // opens and keeps a connection of this name
	over       string // runs over the connection of this name
	closeAfter bool   // closes the connection `over` names after the step
}

// connectionError is a failure to open, find or use a connection kept open;
// it is reported as FLW-ASYNC-006 rather than as a transport failure.
type connectionError struct{ detail string }

func (e connectionError) Error() string { return e.detail }

// connections are the sessions a run holds, by the name a step gave them.
// Names are shared by the whole run, subflows and hook flows included.
type connections struct {
	mu       sync.Mutex
	sessions map[string]transport.Session
}

// add keeps s under name; false when a connection of that name is open.
func (c *connections) add(name string, s transport.Session) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, taken := c.sessions[name]; taken {
		return false
	}
	if c.sessions == nil {
		c.sessions = map[string]transport.Session{}
	}
	c.sessions[name] = s
	return true
}

func (c *connections) get(name string) (transport.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[name]
	return s, ok
}

// close ends the connection of that name, if one is open.
func (c *connections) close(name string) {
	c.mu.Lock()
	s, ok := c.sessions[name]
	delete(c.sessions, name)
	c.mu.Unlock()
	if ok {
		_ = s.Close() // the run is done with it; there is nobody to tell about a goodbye that failed
	}
}

// closeAll ends every connection still open. It is safe to call twice.
func (c *connections) closeAll() {
	c.mu.Lock()
	open := c.sessions
	c.sessions = nil
	c.mu.Unlock()
	for _, s := range open {
		_ = s.Close() // see close
	}
}

// runAsync runs the operation the way the node asks: on its own one-shot
// connection, on a new connection it keeps, or on one an earlier step kept.
func (x *execution) runAsync(ctx context.Context, tr transport.Transport, b builtAsync) (transport.Result, error) {
	switch use := x.c.conn; {
	case use.over != "":
		return x.runOver(ctx, b, use)
	case use.open != "":
		return x.runOpen(ctx, tr, b, use.open)
	}
	return x.runTransport(ctx, tr, b)
}

// runOpen connects, subscribes, keeps the connection under name, and then does
// the step's own send and receive on it.
func (x *execution) runOpen(ctx context.Context, tr transport.Transport, b builtAsync, name string) (transport.Result, error) {
	opener, ok := tr.(transport.Opener)
	if !ok {
		return transport.Result{}, connectionError{fmt.Sprintf("a %s request cannot be kept open between steps (WebSocket, MQTT 5 and core NATS can): send and receive in one step", b.protocol)}
	}
	started := x.r.deps.Clock()
	openCtx, cancel := withBudget(ctx, x.cfg.timeout)
	sess, err := opener.Open(openCtx, b.op)
	cancel()
	if err != nil {
		return transport.Result{}, err
	}
	if !x.r.conns.add(name, sess) {
		_ = sess.Close() // the name is taken: this connection was never kept
		return transport.Result{}, connectionError{fmt.Sprintf("a connection named %q is already open", name)}
	}
	res := transport.Result{Connect: x.r.deps.Clock().Sub(started)}
	err = x.useSession(ctx, sess, b, &res)
	res.Total = x.r.deps.Clock().Sub(started)
	return res, err
}

// runOver does the step's send and receive on the connection an earlier step
// kept, and closes it afterwards when the step says so.
func (x *execution) runOver(ctx context.Context, b builtAsync, use connUse) (transport.Result, error) {
	sess, ok := x.r.conns.get(use.over)
	if !ok {
		return transport.Result{}, connectionError{fmt.Sprintf("no connection named %q is open: the step that opens it (open: %s) must run first", use.over, use.over)}
	}
	if use.closeAfter {
		defer x.r.conns.close(use.over)
	}
	started := x.r.deps.Clock()
	var res transport.Result
	err := x.useSession(ctx, sess, b, &res)
	res.Total = x.r.deps.Clock().Sub(started)
	return res, err
}

// useSession sends the step's message, then waits for what it receives. Both
// are on the connection the step opened or was given; its server is not used.
func (x *execution) useSession(ctx context.Context, sess transport.Session, b builtAsync, res *transport.Result) error {
	if b.op.Send != nil {
		m := *b.op.Send
		m.Channel = b.op.Channel
		sendCtx, cancel := withBudget(ctx, x.cfg.timeout)
		sent, err := sess.Send(sendCtx, m)
		cancel()
		if err != nil {
			return err
		}
		res.Sent = &sent
	}
	if b.op.Receive == nil {
		return nil
	}
	var err error
	res.Received, err = sess.Receive(ctx, *b.op.Receive)
	return err
}

// withBudget bounds ctx by d when d is positive.
func withBudget(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
