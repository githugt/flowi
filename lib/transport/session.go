// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"sync"
)

// Session is a connection kept open across several steps of a flow: one step
// opens it, later steps send over it and collect what arrived, and it closes
// when the run ends. Messages that arrive between steps are held, so a reply
// that comes before the step that waits for it is not lost.
//
// A session serves one step at a time: Send and Receive wait for each other.
type Session interface {
	// Send puts one message on the connection and returns it as sent.
	Send(ctx context.Context, m Message) (Message, error)
	// Receive collects messages as Receive says, from those already held and
	// those still to come. A timeout returns what was kept with ErrTimeout and
	// leaves the connection open.
	Receive(ctx context.Context, r Receive) ([]Message, error)
	// Close ends the connection and every goroutine it started. Closing twice
	// is harmless.
	Close() error
}

// Opener is implemented by a Transport that can keep a connection open. The
// session subscribes at once, whether or not the opening step receives.
type Opener interface {
	Open(ctx context.Context, op Operation) (Session, error)
}

// ErrInboxFull means messages arrived faster than any step collected them and
// the session stopped keeping them.
var ErrInboxFull = errors.New("a connection kept open held more unread messages than the limit")

// DefaultInboxBytes is how much unread message data a session holds.
const DefaultInboxBytes = 64 << 20

// Inbox holds the messages a session's reader goroutine receives until a step
// asks for them. Push and Fail are safe to call from that goroutine while
// another one waits.
type Inbox struct {
	mu    sync.Mutex
	queue []Message
	size  int64
	limit int64
	err   error
	wake  chan struct{}
	// waiting serves one Wait at a time, so two steps never split a reply.
	waiting sync.Mutex
}

// NewInbox returns an inbox that holds up to limit bytes of unread bodies
// (DefaultInboxBytes when limit is not positive).
func NewInbox(limit int64) *Inbox {
	if limit <= 0 {
		limit = DefaultInboxBytes
	}
	return &Inbox{limit: limit, wake: make(chan struct{}, 1)}
}

// Push adds a received message. Past the limit the message is dropped and the
// inbox fails with ErrInboxFull, after the messages already held.
func (b *Inbox) Push(m Message) {
	b.mu.Lock()
	if b.err == nil {
		if b.size+int64(len(m.Body)) > b.limit {
			b.err = ErrInboxFull
		} else {
			b.queue = append(b.queue, m)
			b.size += int64(len(m.Body))
		}
	}
	b.mu.Unlock()
	b.signal()
}

// Fail ends the inbox: a Wait returns the messages held and then err. The
// first failure counts.
func (b *Inbox) Fail(err error) {
	b.mu.Lock()
	if b.err == nil {
		b.err = err
	}
	b.mu.Unlock()
	b.signal()
}

func (b *Inbox) signal() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Wait collects messages as r says, taking them out of the inbox.
func (b *Inbox) Wait(ctx context.Context, r Receive) ([]Message, error) {
	b.waiting.Lock()
	defer b.waiting.Unlock()
	return collect(ctx, r, b.next)
}

// next returns the oldest held message, or waits for one.
func (b *Inbox) next(ctx context.Context) (Message, error) {
	for {
		b.mu.Lock()
		if len(b.queue) > 0 {
			m := b.queue[0]
			b.queue = b.queue[1:]
			b.size -= int64(len(m.Body))
			b.mu.Unlock()
			return m, nil
		}
		err := b.err
		b.mu.Unlock()
		if err != nil {
			return Message{}, err
		}
		select {
		case <-ctx.Done():
			return Message{}, context.Cause(ctx)
		case <-b.wake:
		}
	}
}
