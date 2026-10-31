// SPDX-License-Identifier: Apache-2.0

// Package amqp is the AMQP 0-9-1 transport (RabbitMQ and compatible brokers).
// AMQP 1.0 is a different protocol and is not supported.
package amqp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/githugt/flowi/lib/transport"
)

const dialTimeout = 10 * time.Second

// Transport runs AMQP operations.
type Transport struct{}

// New returns the AMQP transport.
func New() Transport { return Transport{} }

// target is where an operation goes, worked out from the server, channel
// and binding.
type target struct {
	url      *url.URL
	exchange string // "" is the default exchange
	key      string // the routing key
	queue    string // "" → a server-named queue of the run's own
	password string // for redacting errors
}

// targetOf reads the server URL and the binding: exchange, routing_key
// (default: the channel), queue, username, password.
func targetOf(op transport.Operation) (target, error) {
	u, err := url.Parse(op.Server)
	if err != nil {
		return target{}, fmt.Errorf("server is not a URL: %w", err)
	}
	if s := strings.ToLower(u.Scheme); s != "amqp" && s != "amqps" {
		return target{}, fmt.Errorf("amqp cannot use the scheme %q (use amqp or amqps)", u.Scheme)
	}
	t := target{url: u, exchange: op.Binding["exchange"], key: op.Binding["routing_key"], queue: op.Binding["queue"]}
	if t.key == "" {
		t.key = op.Channel
	}
	if name := op.Binding["username"]; name != "" {
		t.url.User = url.UserPassword(name, op.Binding["password"])
	}
	if t.url.User != nil {
		t.password, _ = t.url.User.Password()
	}
	return t, nil
}

// checkReceive says whether there is anything to consume from: a queue, or
// an exchange to bind a queue of the run's own to.
func (t target) checkReceive() error {
	if t.queue == "" && t.exchange == "" {
		return errors.New("receiving needs a binding named queue, or exchange (to bind a temporary queue to)")
	}
	return nil
}

// redact removes the password from text that may quote the URL.
func (t target) redact(s string) string {
	if t.password == "" {
		return s
	}
	return strings.ReplaceAll(s, t.password, "***")
}

// Run connects, sets up the consumer (before publishing, so a reply is not
// missed), publishes with a broker confirm, collects, and closes. The run's
// own message is recognised by its message id and skipped.
//
// Goroutine ownership: the amqp091 connection owns its goroutines; Run
// starts one forwarding goroutine for the deliveries, stops it by closing
// the channel, and waits for it.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	tg, err := targetOf(op)
	if err != nil {
		return transport.Result{}, err
	}
	if op.Receive != nil {
		if err := tg.checkReceive(); err != nil {
			return transport.Result{}, err
		}
	}
	conn, err := amqp091.DialConfig(tg.url.String(), amqp091.Config{
		TLSClientConfig: op.TLS, Dial: amqp091.DefaultDial(dialTimeout), Heartbeat: 10 * time.Second,
	})
	if err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %s", tg.url.Redacted(), tg.redact(err.Error()))
	}
	defer func() { _ = conn.Close() }() // everything is done or abandoned by now
	ch, err := conn.Channel()
	if err != nil {
		return transport.Result{}, fmt.Errorf("opening a channel: %s", tg.redact(err.Error()))
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	run := &session{op: op, tg: tg, ch: ch, id: "flowi-" + strconv.FormatInt(op.Now().UnixNano(), 36)}
	err = run.do(ctx, &res)
	res.Total = op.Now().Sub(started)
	return res, err
}

// session is one run on an open channel.
type session struct {
	op transport.Operation
	tg target
	ch *amqp091.Channel
	id string // message id of the run's own message
	wg sync.WaitGroup
}

func (s *session) do(ctx context.Context, res *transport.Result) error {
	var in chan transport.Message
	if s.op.Receive != nil {
		var stop func()
		var err error
		if in, stop, err = s.consume(ctx); err != nil {
			return err
		}
		defer stop()
	}
	if s.op.Send != nil {
		sent, err := s.publish(ctx)
		if err != nil {
			return err
		}
		res.Sent = sent
	}
	if in == nil {
		return nil
	}
	r := *s.op.Receive
	r.Match = s.skipOwn(r.Match)
	var err error
	res.Received, err = transport.Wait(ctx, r, in, nil)
	return err
}

// consume declares and binds the queue and starts forwarding deliveries. The
// returned function stops the forwarding and gives unhandled messages back.
func (s *session) consume(ctx context.Context) (chan transport.Message, func(), error) {
	queue, err := s.prepareQueue()
	if err != nil {
		return nil, nil, err
	}
	if err := s.ch.Qos(1, 0, false); err != nil { // one in flight: the rest stay on the broker
		return nil, nil, fmt.Errorf("setting prefetch: %w", err)
	}
	deliveries, err := s.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("consuming from %s: %w", queue, err)
	}
	in := make(chan transport.Message)
	fwdCtx, cancel := context.WithCancel(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				select {
				case in <- received(d, s.op.Now()):
					_ = d.Ack(false) // a lost ack only means a redelivery
				case <-fwdCtx.Done():
					_ = d.Nack(false, true) // not collected: back to the queue
					return
				}
			case <-fwdCtx.Done():
				return
			}
		}
	}()
	return in, func() { cancel(); s.wg.Wait() }, nil
}

// prepareQueue returns the queue to consume: the named one (which must
// exist), or a temporary one bound to the exchange with the routing key.
func (s *session) prepareQueue() (string, error) {
	if s.tg.queue != "" {
		if _, err := s.ch.QueueDeclarePassive(s.tg.queue, false, false, false, false, nil); err != nil {
			return "", fmt.Errorf("queue %s: %w", s.tg.queue, err)
		}
		return s.tg.queue, nil
	}
	q, err := s.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return "", fmt.Errorf("declaring a temporary queue: %w", err)
	}
	if err := s.ch.QueueBind(q.Name, s.tg.key, s.tg.exchange, false, nil); err != nil {
		return "", fmt.Errorf("binding to exchange %s with key %s: %w", s.tg.exchange, s.tg.key, err)
	}
	return q.Name, nil
}

func (s *session) publish(ctx context.Context) (*transport.Message, error) {
	if err := s.ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("enabling publisher confirms: %w", err)
	}
	pub := amqp091.Publishing{Body: s.op.Send.Body, MessageId: s.id, Timestamp: s.op.Now()}
	if len(s.op.Send.Headers) > 0 {
		pub.Headers = amqp091.Table{}
		for _, h := range s.op.Send.Headers {
			pub.Headers[h.Name] = h.Value
		}
	}
	conf, err := s.ch.PublishWithDeferredConfirmWithContext(ctx, s.tg.exchange, s.tg.key, false, false, pub)
	if err != nil {
		return nil, fmt.Errorf("publishing: %w", err)
	}
	acked, err := conf.WaitContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for the broker to confirm: %w", err)
	}
	if !acked {
		return nil, errors.New("the broker did not accept the message")
	}
	m := *s.op.Send
	m.Direction, m.At = "sent", s.op.Now()
	return &m, nil
}

// skipOwn drops the message this run published, which a bound queue gets back.
func (s *session) skipOwn(next func(transport.Message) (bool, error)) func(transport.Message) (bool, error) {
	return func(m transport.Message) (bool, error) {
		for _, h := range m.Headers {
			if h.Name == "message_id" && h.Value == s.id {
				return false, nil
			}
		}
		if next == nil {
			return true, nil
		}
		return next(m)
	}
}

// received turns a delivery into a message: the exchange, routing key and
// message id become headers, then the AMQP headers.
func received(d amqp091.Delivery, at time.Time) transport.Message {
	m := transport.Message{Direction: "received", At: at, Body: d.Body}
	add := func(name, value string) {
		if value != "" {
			m.Headers = append(m.Headers, transport.Header{Name: name, Value: value})
		}
	}
	add("exchange", d.Exchange)
	add("routing_key", d.RoutingKey)
	add("message_id", d.MessageId)
	add("content_type", d.ContentType)
	names := make([]string, 0, len(d.Headers))
	for name := range d.Headers {
		names = append(names, name)
	}
	slices.Sort(names) // a table is a map: keep the message's headers in a stable order
	for _, name := range names {
		add(name, fmt.Sprint(d.Headers[name]))
	}
	return m
}
