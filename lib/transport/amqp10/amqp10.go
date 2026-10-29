// SPDX-License-Identifier: Apache-2.0

// Package amqp10 is the AMQP 1.0 transport: ActiveMQ Artemis and Classic, Apache
// Qpid, IBM MQ, RabbitMQ 4 and other brokers that speak the ISO standard
// protocol. AMQP 1.0 and the AMQP 0-9-1 of package amqp are different,
// incompatible protocols that only share a name and a port. Azure Service Bus
// has its own transport (it uses the same wire protocol but its own SDK).
package amqp10

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	amqp "github.com/Azure/go-amqp"

	"github.com/githugt/flowi-lib/transport"
)

const (
	dialTimeout = 10 * time.Second
	maxCredit   = 100
)

// Transport runs AMQP 1.0 operations.
type Transport struct{}

// New returns the AMQP 1.0 transport.
func New() Transport { return Transport{} }

// target is where an operation goes, worked out from the server, channel and
// binding.
type target struct {
	addr     string // amqp[s]://host:port, with no credentials
	address  string // the node (queue, topic or address) to send to and receive from
	user     string
	password string
	tls      bool
}

// targetOf reads the server URL, the channel (the node address) and the
// binding: username, password.
func targetOf(op transport.Operation) (target, error) {
	u, err := url.Parse(op.Server)
	if err != nil {
		return target{}, fmt.Errorf("server is not a URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "amqp" && scheme != "amqps" {
		return target{}, fmt.Errorf("amqp10 cannot use the scheme %q (use amqp or amqps)", u.Scheme)
	}
	if op.Channel == "" {
		return target{}, errors.New("an AMQP 1.0 request needs a channel: the address of the queue or topic")
	}
	t := target{address: op.Channel, tls: scheme == "amqps", user: op.Binding["username"], password: op.Binding["password"]}
	if u.User != nil {
		if t.user == "" {
			t.user = u.User.Username()
		}
		if p, ok := u.User.Password(); ok && t.password == "" {
			t.password = p
		}
	}
	port := u.Port()
	if port == "" {
		port = map[bool]string{false: "5672", true: "5671"}[t.tls]
	}
	t.addr = scheme + "://" + u.Hostname() + ":" + port
	return t, nil
}

// connOptions says how to connect: TLS for amqps, SASL PLAIN when there is a
// user name (otherwise none is offered: brokers that allow anonymous
// connections do not ask for it).
func (t target) connOptions(op transport.Operation) *amqp.ConnOptions {
	o := &amqp.ConnOptions{IdleTimeout: time.Minute}
	if t.tls {
		o.TLSConfig = op.TLS
	}
	if t.user != "" {
		o.SASLType = amqp.SASLTypePlain(t.user, t.password)
	}
	return o
}

// redact removes the password from text that may quote it.
func (t target) redact(s string) string {
	if t.password == "" {
		return s
	}
	return strings.ReplaceAll(s, t.password, "***")
}

// Run connects, attaches the receiving link first (so a reply is not missed),
// sends, collects, and closes. The run's own message is recognised by its
// message id and skipped. Messages received are accepted (consumed from the
// queue). Binding names: username, password, subject, content_type, durable,
// ttl (milliseconds).
//
// Goroutine ownership: go-amqp owns the connection's goroutines; Run starts one
// goroutine that feeds received messages, stops it by closing the receiver
// and cancelling its context, and waits for it.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	t, err := targetOf(op)
	if err != nil {
		return transport.Result{}, err
	}
	props, err := sendProps(op)
	if err != nil {
		return transport.Result{}, err
	}
	dialCtx, cancel := context.WithTimeoutCause(ctx, dialTimeout, errors.New("connecting timed out"))
	conn, err := amqp.Dial(dialCtx, t.addr, t.connOptions(op))
	cancel()
	if err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %s", t.addr, t.redact(err.Error()))
	}
	defer func() { _ = conn.Close() }() // everything is done or abandoned by now
	sess, err := conn.NewSession(ctx, nil)
	if err != nil {
		return transport.Result{}, fmt.Errorf("opening a session: %s", t.redact(err.Error()))
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	r := &run{op: op, t: t, sess: sess, props: props, id: "flowi-" + strconv.FormatInt(op.Now().UnixNano(), 36)}
	err = r.do(ctx, &res)
	res.Total = op.Now().Sub(started)
	return res, err
}

type run struct {
	op    transport.Operation
	t     target
	sess  *amqp.Session
	props sendProperties
	id    string // message id of the run's own message
	wg    sync.WaitGroup
}

func (r *run) do(ctx context.Context, res *transport.Result) error {
	var in chan transport.Message
	var failed chan error
	if r.op.Receive != nil {
		var stop func()
		var err error
		if in, failed, stop, err = r.consume(ctx); err != nil {
			return err
		}
		defer stop()
	}
	if r.op.Send != nil {
		sent, err := r.send(ctx)
		if err != nil {
			return err
		}
		res.Sent = sent
	}
	if r.op.Receive == nil {
		return nil
	}
	var err error
	res.Received, err = transport.Wait(ctx, *r.op.Receive, in, failed)
	return err
}

func (r *run) send(ctx context.Context) (*transport.Message, error) {
	sender, err := r.sess.NewSender(ctx, r.t.address, nil)
	if err != nil {
		return nil, fmt.Errorf("attaching to %s: %s", r.t.address, r.t.redact(err.Error()))
	}
	defer func() { _ = sender.Close(ctx) }() // the message is settled or the run is over
	msg := r.message()
	if err := sender.Send(ctx, msg, nil); err != nil {
		return nil, fmt.Errorf("sending to %s: %s", r.t.address, r.t.redact(err.Error()))
	}
	m := *r.op.Send
	m.Direction, m.At = "sent", r.op.Now()
	return &m, nil
}

// consume attaches a receiver and starts the goroutine that feeds it to in.
// stop ends the goroutine and waits for it.
func (r *run) consume(ctx context.Context) (in chan transport.Message, failed chan error, stop func(), err error) {
	rc := r.op.Receive.Normalized()
	credit := int32(min(max(rc.Count, 1), maxCredit)) //nolint:gosec // bounded by maxCredit
	recv, err := r.sess.NewReceiver(ctx, r.t.address, &amqp.ReceiverOptions{Credit: credit})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("attaching to %s: %s", r.t.address, r.t.redact(err.Error()))
	}
	in, failed = make(chan transport.Message), make(chan error, 1)
	loopCtx, cancel := context.WithCancel(ctx)
	r.wg.Go(func() {
		for {
			m, err := recv.Receive(loopCtx, nil)
			if err != nil {
				if loopCtx.Err() == nil {
					failed <- fmt.Errorf("receiving from %s: %s", r.t.address, r.t.redact(err.Error()))
				}
				return
			}
			_ = recv.AcceptMessage(loopCtx, m) // consumed: a message left unsettled would come back on the next run
			if r.isOwn(m) {
				continue
			}
			select {
			case in <- r.received(m):
			case <-loopCtx.Done():
				return
			}
		}
	})
	return in, failed, func() {
		cancel()
		_ = recv.Close(context.WithoutCancel(ctx)) // the run is over; a broker that already detached is fine
		r.wg.Wait()
	}, nil
}
