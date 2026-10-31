// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"

	"github.com/githugt/flowi/lib/transport"
)

// Open connects, subscribes to the channel, and keeps the connection open for
// later steps. Only MQTT 5 can be kept open: for 3.1.1 and 3.1 it is an error.
//
// Goroutine ownership: the paho client owns its read and keep-alive
// goroutines; Close ends them with Disconnect and waits for Done.
func (Transport) Open(ctx context.Context, op transport.Operation) (transport.Session, error) {
	version, err := versionOf(op.Binding)
	if err != nil {
		return nil, err
	}
	if version != version5 {
		return nil, errors.New("only MQTT 5 can be kept open between steps: set the binding version to 5, or send and receive in one step")
	}
	qos, retain, err := publishOptions(op.Binding)
	if err != nil {
		return nil, err
	}
	conn, err := dial(ctx, op)
	if err != nil {
		return nil, err
	}
	inbox := transport.NewInbox(0)
	client := paho.NewClient(paho.ClientConfig{
		Conn: packets.NewThreadSafeConn(conn),
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){func(p paho.PublishReceived) (bool, error) {
			inbox.Push(received(p.Packet, op.Now()))
			return true, nil
		}},
		OnServerDisconnect: func(d *paho.Disconnect) { inbox.Fail(fmt.Errorf("the broker disconnected (reason %d)", d.ReasonCode)) },
		OnClientError:      inbox.Fail,
	})
	if _, err := client.Connect(ctx, connectPacket(op)); err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	if err := subscribe(ctx, client, op.Channel, qos); err != nil {
		finish(client)
		return nil, err
	}
	return &session{client: client, inbox: inbox, op: op, qos: qos, retain: retain}, nil
}

var _ transport.Opener = Transport{}

type session struct {
	client *paho.Client
	inbox  *transport.Inbox
	op     transport.Operation
	qos    byte
	retain bool
	once   sync.Once
}

func (s *session) Send(ctx context.Context, m transport.Message) (transport.Message, error) {
	op := s.op
	op.Send = &m
	if m.Channel != "" {
		op.Channel = m.Channel
	}
	sent, err := publish(ctx, s.client, op, s.qos, s.retain)
	if err != nil {
		return m, err
	}
	return *sent, nil
}

func (s *session) Receive(ctx context.Context, r transport.Receive) ([]transport.Message, error) {
	return s.inbox.Wait(ctx, r)
}

func (s *session) Close() error {
	s.once.Do(func() { finish(s.client) })
	return nil
}
