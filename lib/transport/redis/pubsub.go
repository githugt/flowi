// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/githugt/flowi/lib/transport"
)

// runPubSub subscribes first (and waits for the server to confirm), then
// publishes, then collects: a fast message is not missed. Pub/sub has no
// message ids, so the validator refuses a request that sends and receives on
// one channel.
func runPubSub(ctx context.Context, client *goredis.Client, op transport.Operation, res *transport.Result) error {
	var in chan transport.Message
	if op.Receive != nil {
		pump, err := subscribe(ctx, client, op)
		if err != nil {
			return err
		}
		defer pump.stop()
		in = pump.messages
	}
	if op.Send != nil {
		if err := client.Publish(ctx, op.Channel, op.Send.Body).Err(); err != nil {
			return fmt.Errorf("publishing to %s: %w", op.Channel, err)
		}
		res.Sent = sentMessage(op, nil)
	}
	if in == nil {
		return nil
	}
	var err error
	res.Received, err = transport.Wait(ctx, *op.Receive, in, nil)
	return err
}

// subscription is a running subscription and the goroutine feeding messages.
type subscription struct {
	messages chan transport.Message
	sub      *goredis.PubSub
	cancel   context.CancelFunc
	done     sync.WaitGroup
}

// stop ends the subscription and waits for the goroutine.
func (s *subscription) stop() {
	s.cancel()
	_ = s.sub.Close() // ends the message channel; a close error changes nothing for the run
	s.done.Wait()
}

func subscribe(ctx context.Context, client *goredis.Client, op transport.Operation) (*subscription, error) {
	var sub *goredis.PubSub
	if pattern, _ := strconv.ParseBool(op.Binding["pattern"]); pattern {
		sub = client.PSubscribe(ctx, op.Channel)
	} else {
		sub = client.Subscribe(ctx, op.Channel)
	}
	if _, err := sub.Receive(ctx); err != nil { // the server's confirmation
		_ = sub.Close()
		return nil, fmt.Errorf("subscribing to %s: %w", op.Channel, err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	s := &subscription{messages: make(chan transport.Message), sub: sub, cancel: cancel}
	s.done.Add(1)
	go func() {
		defer s.done.Done()
		for m := range sub.Channel() {
			select {
			case s.messages <- pubSubMessage(m, op.Now()):
			case <-runCtx.Done():
				return
			}
		}
	}()
	return s, nil
}

func pubSubMessage(m *goredis.Message, at time.Time) transport.Message {
	headers := []transport.Header{{Name: "channel", Value: m.Channel}}
	if m.Pattern != "" {
		headers = append(headers, transport.Header{Name: "pattern", Value: m.Pattern})
	}
	return transport.Message{Direction: "received", At: at, Headers: headers, Body: []byte(m.Payload)}
}

// sentMessage is the message as sent, with extra headers (a stream entry id).
func sentMessage(op transport.Operation, extra []transport.Header) *transport.Message {
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	m.Headers = append(append([]transport.Header(nil), m.Headers...), extra...)
	return &m
}
