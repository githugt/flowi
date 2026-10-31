// SPDX-License-Identifier: Apache-2.0

package nats

import (
	"context"
	"errors"
	"fmt"
	"sync"

	natsgo "github.com/nats-io/nats.go"

	"github.com/githugt/flowi/lib/transport"
)

// Open connects, subscribes (to reply_to when set, else the channel) and keeps
// the connection open for later steps. Core NATS only: a JetStream stream is
// read by a consumer that belongs to one run.
//
// Goroutine ownership: the NATS client owns its goroutines; Close ends them.
func (Transport) Open(_ context.Context, op transport.Operation) (transport.Session, error) {
	if op.Binding["stream"] != "" {
		return nil, errors.New("a JetStream stream cannot be kept open between steps: send and receive in one step")
	}
	nc, err := natsgo.Connect(op.Server, connectOptions(op)...)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	inbox := transport.NewInbox(0)
	nc.SetClosedHandler(func(*natsgo.Conn) { inbox.Fail(errors.New("the connection to the server closed")) })
	nc.SetDisconnectErrHandler(func(_ *natsgo.Conn, err error) {
		if err != nil {
			inbox.Fail(fmt.Errorf("the connection to the server was lost: %w", err))
		}
	})
	subject := cmpOr(op.Binding["reply_to"], op.Channel)
	handler := func(m *natsgo.Msg) { inbox.Push(coreMessage(m, op.Now())) }
	if q := op.Binding["queue_group"]; q != "" {
		_, err = nc.QueueSubscribe(subject, q, handler)
	} else {
		_, err = nc.Subscribe(subject, handler)
	}
	if err == nil {
		err = nc.FlushTimeout(connectTimeout)
	}
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("subscribing to %s: %w", subject, err)
	}
	return &session{nc: nc, inbox: inbox, op: op}, nil
}

var _ transport.Opener = Transport{}

type session struct {
	nc    *natsgo.Conn
	inbox *transport.Inbox
	op    transport.Operation
	once  sync.Once
}

func (s *session) Send(_ context.Context, m transport.Message) (transport.Message, error) {
	op := s.op
	op.Send = &m
	if m.Channel != "" {
		op.Channel = m.Channel
	}
	msg := outgoing(op)
	msg.Reply = op.Binding["reply_to"]
	if err := s.nc.PublishMsg(msg); err != nil {
		return m, fmt.Errorf("publishing to %s: %w", op.Channel, err)
	}
	if err := s.nc.FlushTimeout(connectTimeout); err != nil {
		return m, fmt.Errorf("publishing to %s: %w", op.Channel, err)
	}
	return *sentMessage(op, 0), nil
}

func (s *session) Receive(ctx context.Context, r transport.Receive) ([]transport.Message, error) {
	return s.inbox.Wait(ctx, r)
}

func (s *session) Close() error {
	s.once.Do(func() { s.nc.Close() })
	return nil
}
