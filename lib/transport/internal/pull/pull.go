// SPDX-License-Identifier: Apache-2.0

// Package pull is the shared shape of the queue-like transports (Pub/Sub,
// SQS, Service Bus): a goroutine pulls messages and holds each one's handle,
// the receive keeps some of them, and when the run ends the kept handles are
// acknowledged and every other handle is given back to the service.
//
// Skipped messages are held until the end rather than given back at once: an
// immediate give-back would deliver them to this same run again and again.
package pull

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/transport"
)

// settleTimeout bounds the final acknowledge and give-back calls, which run
// even when the run's own context has ended.
const settleTimeout = 10 * time.Second

// Book remembers the handle (an ack id, a receipt handle, a received message)
// of every message pulled, by message id, and which ones were kept. It is safe
// for the pulling goroutine and the collector to use together.
type Book[H any] struct {
	mu      sync.Mutex
	handles map[string]H
	order   []string
	kept    map[string]bool
}

// NewBook returns an empty book.
func NewBook[H any]() *Book[H] {
	return &Book[H]{handles: map[string]H{}, kept: map[string]bool{}}
}

// Hold records the handle of a message pulled. A message pulled again keeps
// its place and gets the newer handle.
func (b *Book[H]) Hold(id string, handle H) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, seen := b.handles[id]; !seen {
		b.order = append(b.order, id)
	}
	b.handles[id] = handle
}

func (b *Book[H]) keep(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.kept[id] = true
}

// split returns the handles of the kept messages and of all the others, each
// in the order the messages were pulled.
func (b *Book[H]) split() (kept, others []H) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range b.order {
		if b.kept[id] {
			kept = append(kept, b.handles[id])
		} else {
			others = append(others, b.handles[id])
		}
	}
	return kept, others
}

// Pump pulls messages into in until ctx ends, holding each handle in book. A
// message it does not send on (its own, for instance) is still held, so it is
// given back at the end. The error is a failed pull, never the end of the run.
type Pump[H any] func(ctx context.Context, book *Book[H], in chan<- transport.Message) error

// Settle acknowledges the kept handles and gives the others back. It gets a
// context of its own that outlives the run.
type Settle[H any] func(ctx context.Context, kept, others []H) error

// Collect runs pump, collects the receive, and settles. idOf names a message
// the way the pump's Hold calls do. The messages kept are returned with any
// error, including ErrTimeout (the partial list); a settle failure is
// returned only when nothing else failed.
//
// Goroutine ownership: Collect starts the pump goroutine and waits for it
// before it settles.
func Collect[H any](ctx context.Context, rc transport.Receive, idOf func(transport.Message) string, pump Pump[H], settle Settle[H]) ([]transport.Message, error) {
	book := NewBook[H]()
	runCtx, cancel := context.WithCancel(ctx)
	in := make(chan transport.Message)
	fail := make(chan error, 1)
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		if err := pump(runCtx, book, in); err != nil {
			fail <- err
		}
	}()

	next := rc.OnMessage
	rc.OnMessage = func(m transport.Message) {
		book.keep(idOf(m))
		if next != nil {
			next(m)
		}
	}
	kept, err := transport.Wait(ctx, rc, in, fail)
	cancel()
	done.Wait()

	settleCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer stop()
	keptHandles, others := book.split()
	if serr := settle(settleCtx, keptHandles, others); serr != nil {
		err = errors.Join(err, serr)
	}
	return kept, err
}
