// SPDX-License-Identifier: Apache-2.0

// Package nats is the NATS transport: core publish and subscribe, and
// JetStream when the binding names a stream.
package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/githugt/flowi-lib/transport"
)

const connectTimeout = 10 * time.Second

// Transport runs NATS operations.
type Transport struct{}

// New returns the NATS transport.
func New() Transport { return Transport{} }

// Run connects and then does the operation over core NATS, or over
// JetStream when binding "stream" is set. Core: binding queue_group,
// reply_to (the subject replies come back on, instead of the channel),
// token, username, password. JetStream: stream, consumer (a durable name;
// without one the consumer is ephemeral), deliver (new, all, last or
// start_time:<RFC3339>) and ack (auto or none).
//
// Goroutine ownership: the NATS client owns its goroutines; Run closes the
// connection (which ends every subscription and consumer) before returning.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	nc, err := natsgo.Connect(op.Server, connectOptions(op)...)
	if err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	defer nc.Close()
	res := transport.Result{Connect: op.Now().Sub(started)}
	if op.Binding["stream"] != "" {
		err = runJetStream(ctx, nc, op, &res)
	} else {
		err = runCore(ctx, nc, op, &res)
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

func connectOptions(op transport.Operation) []natsgo.Option {
	opts := []natsgo.Option{natsgo.Timeout(connectTimeout), natsgo.NoEcho(), natsgo.MaxReconnects(0), natsgo.Name("flowi")}
	// Only a tls:// server asks for TLS: forcing it on a plain nats:// server
	// fails with "secure connection not available".
	if strings.HasPrefix(strings.ToLower(op.Server), "tls://") && op.TLS != nil {
		opts = append(opts, natsgo.Secure(op.TLS))
	}
	if t := op.Binding["token"]; t != "" {
		opts = append(opts, natsgo.Token(t))
	}
	if u := op.Binding["username"]; u != "" {
		opts = append(opts, natsgo.UserInfo(u, op.Binding["password"]))
	}
	return opts
}

// runCore subscribes (to reply_to when set, else the channel), publishes,
// and collects. NoEcho keeps the connection's own message from coming back.
func runCore(ctx context.Context, nc *natsgo.Conn, op transport.Operation, res *transport.Result) error {
	var in chan transport.Message
	if op.Receive != nil {
		in = make(chan transport.Message)
		subject := cmpOr(op.Binding["reply_to"], op.Channel)
		handler := func(m *natsgo.Msg) { deliver(ctx, in, coreMessage(m, op.Now())) }
		var err error
		if q := op.Binding["queue_group"]; q != "" {
			_, err = nc.QueueSubscribe(subject, q, handler)
		} else {
			_, err = nc.Subscribe(subject, handler)
		}
		if err != nil {
			return fmt.Errorf("subscribing to %s: %w", subject, err)
		}
		if err := nc.FlushTimeout(connectTimeout); err != nil {
			return fmt.Errorf("subscribing to %s: %w", subject, err)
		}
	}
	if op.Send != nil {
		msg := outgoing(op)
		msg.Reply = op.Binding["reply_to"]
		if err := nc.PublishMsg(msg); err != nil {
			return fmt.Errorf("publishing to %s: %w", op.Channel, err)
		}
		if err := nc.FlushTimeout(connectTimeout); err != nil {
			return fmt.Errorf("publishing to %s: %w", op.Channel, err)
		}
		res.Sent = sentMessage(op, 0)
	}
	if in == nil {
		return nil
	}
	var err error
	res.Received, err = transport.Wait(ctx, *op.Receive, in, nil)
	return err
}

func outgoing(op transport.Operation) *natsgo.Msg {
	msg := &natsgo.Msg{Subject: op.Channel, Data: op.Send.Body}
	if len(op.Send.Headers) > 0 {
		msg.Header = natsgo.Header{}
		for _, h := range op.Send.Headers {
			msg.Header.Add(h.Name, h.Value)
		}
	}
	return msg
}

func sentMessage(op transport.Operation, sequence uint64) *transport.Message {
	m := *op.Send
	m.Direction, m.At, m.Sequence = "sent", op.Now(), sequence
	return &m
}

// deliver hands a message to the collector; false means the run ended first.
func deliver(ctx context.Context, in chan<- transport.Message, m transport.Message) bool {
	select {
	case in <- m:
		return true
	case <-ctx.Done():
		return false
	}
}

func coreMessage(m *natsgo.Msg, at time.Time) transport.Message {
	out := transport.Message{Direction: "received", At: at, Body: m.Data, Headers: []transport.Header{{Name: "subject", Value: m.Subject}}}
	if m.Reply != "" {
		out.Headers = append(out.Headers, transport.Header{Name: "reply", Value: m.Reply})
	}
	return withHeaders(out, m.Header)
}

func withHeaders(m transport.Message, h natsgo.Header) transport.Message {
	for name, values := range h {
		for _, v := range values {
			m.Headers = append(m.Headers, transport.Header{Name: name, Value: v})
		}
	}
	return m
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// runJetStream publishes to the stream (waiting for its ack) and reads from
// it through a consumer that is created before the publish, so a reply is
// not missed.
func runJetStream(ctx context.Context, nc *natsgo.Conn, op transport.Operation, res *transport.Result) error {
	name := op.Binding["stream"]
	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("using JetStream: %w", err)
	}
	if _, err := js.Stream(ctx, name); err != nil {
		return fmt.Errorf("stream %s: %w", name, err)
	}
	var in chan transport.Message
	if op.Receive != nil {
		in = make(chan transport.Message)
		// The consumer's handler stops handing messages over once this run
		// is done; the one it was holding is given back to the stream.
		var done context.CancelFunc
		var runCtx context.Context
		runCtx, done = context.WithCancel(ctx)
		defer done()
		stop, err := consume(runCtx, js, op, in)
		if err != nil {
			return err
		}
		defer stop()
	}
	var sentSeq atomic.Uint64
	if op.Send != nil {
		ack, err := js.PublishMsg(ctx, outgoing(op))
		if err != nil {
			return fmt.Errorf("publishing to stream %s: %w", name, err)
		}
		sentSeq.Store(ack.Sequence)
		res.Sent = sentMessage(op, ack.Sequence)
	}
	if in == nil {
		return nil
	}
	r := *op.Receive
	r.Match = skipOwn(r.Match, &sentSeq)
	res.Received, err = transport.Wait(ctx, r, in, nil)
	return err
}

// skipOwn drops the message this run published itself (a stream replays it
// to the run's own consumer); it runs after the publish, so the sequence is known.
func skipOwn(next func(transport.Message) (bool, error), sent *atomic.Uint64) func(transport.Message) (bool, error) {
	return func(m transport.Message) (bool, error) {
		if seq := sent.Load(); seq != 0 && m.Sequence == seq {
			return false, nil
		}
		if next == nil {
			return true, nil
		}
		return next(m)
	}
}

// consume starts the consumer and returns the function that stops it.
func consume(ctx context.Context, js jetstream.JetStream, op transport.Operation, in chan<- transport.Message) (func(), error) {
	stream := op.Binding["stream"]
	cons, err := consumerFor(ctx, js, op)
	if err != nil {
		return nil, err
	}
	acking := op.Binding["ack"] != "none"
	// One message in flight at a time: anything the client had prefetched
	// would sit unacknowledged until the ack wait passed, and a named
	// consumer would not see it again on the next run.
	cc, err := cons.Consume(func(m jetstream.Msg) {
		if !deliver(ctx, in, streamMessage(m, op.Now())) {
			_ = m.Nak() // not collected: let the stream offer it again
			return
		}
		if acking {
			_ = m.Ack() // a lost ack only means the message is redelivered later
		}
	}, jetstream.PullMaxMessages(1))
	if err != nil {
		return nil, fmt.Errorf("reading stream %s: %w", stream, err)
	}
	return cc.Stop, nil
}

func streamMessage(m jetstream.Msg, at time.Time) transport.Message {
	out := transport.Message{Direction: "received", At: at, Body: m.Data(), Headers: []transport.Header{{Name: "subject", Value: m.Subject()}}}
	if md, err := m.Metadata(); err == nil {
		out.Sequence = md.Sequence.Stream
	}
	return withHeaders(out, m.Headers())
}

// consumerFor finds or creates the consumer: the named durable one (kept
// as it is when it exists) or an ephemeral one.
func consumerFor(ctx context.Context, js jetstream.JetStream, op transport.Operation) (jetstream.Consumer, error) {
	stream, durable := op.Binding["stream"], op.Binding["consumer"]
	if durable != "" {
		c, err := js.Consumer(ctx, stream, durable)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, jetstream.ErrConsumerNotFound) {
			return nil, fmt.Errorf("consumer %s of stream %s: %w", durable, stream, err)
		}
	}
	cfg, err := consumerConfig(op, durable)
	if err != nil {
		return nil, err
	}
	c, err := js.CreateOrUpdateConsumer(ctx, stream, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating a consumer on stream %s: %w", stream, err)
	}
	return c, nil
}

func consumerConfig(op transport.Operation, durable string) (jetstream.ConsumerConfig, error) {
	cfg := jetstream.ConsumerConfig{Durable: durable, FilterSubject: op.Channel, AckPolicy: jetstream.AckExplicitPolicy}
	if durable == "" {
		cfg.InactiveThreshold = time.Minute // the server removes an abandoned ephemeral consumer
	}
	if op.Binding["ack"] == "none" {
		cfg.AckPolicy = jetstream.AckNonePolicy
	}
	switch d := op.Binding["deliver"]; {
	case d == "" || d == "new":
		cfg.DeliverPolicy = jetstream.DeliverNewPolicy
	case d == "all":
		cfg.DeliverPolicy = jetstream.DeliverAllPolicy
	case d == "last":
		cfg.DeliverPolicy = jetstream.DeliverLastPolicy
	case strings.HasPrefix(d, "start_time:"):
		at, err := time.Parse(time.RFC3339, strings.TrimPrefix(d, "start_time:"))
		if err != nil {
			return cfg, fmt.Errorf("binding deliver %q: the time must be RFC 3339", d)
		}
		cfg.DeliverPolicy, cfg.OptStartTime = jetstream.DeliverByStartTimePolicy, &at
	default:
		return cfg, fmt.Errorf("binding deliver must be new, all, last or start_time:<RFC3339>, got %q", d)
	}
	return cfg, nil
}
