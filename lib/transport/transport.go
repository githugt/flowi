// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/githugt/flowi-lib/model"
)

// Header is one message header (or connection header), kept in order.
type Header struct{ Name, Value string }

// Message is one message, sent or received. Fields a protocol has no word for
// stay zero.
type Message struct {
	Direction string // "sent" or "received"
	At        time.Time
	Headers   []Header
	Body      []byte
	// Channel, on a message sent over a Session, is where to send it; empty
	// means the channel the connection was opened on.
	Channel   string
	Key       string // Kafka key
	Partition int32  // Kafka partition
	Offset    int64  // Kafka offset
	Sequence  uint64 // JetStream stream sequence
}

// Receive says what to wait for. Zero Count means 1, zero Timeout 30s.
type Receive struct {
	Count   int
	Timeout time.Duration
	// Match, when set, decides whether a message counts; a message that does
	// not match is dropped. An error stops the receive.
	Match func(Message) (bool, error)
	// MaxBytes, when positive, stops the receive once the bodies it kept add
	// up to more than this.
	MaxBytes int64
	// OnMessage, when set, is called with each message kept, as it is kept
	// (so a caller can show progress while Count is still being waited for).
	// It runs on the goroutine that called Wait and must return quickly.
	OnMessage func(Message)
	// OnKept, when set, is called after OnMessage with each message kept. An
	// error stops the receive: Wait returns the messages kept so far, this one
	// included, and the error. It is how a hook can stop a run at the first
	// message that fails its check. It runs on the goroutine that called Wait.
	OnKept func(Message) error
}

// Operation is one run: where to go and what to do there.
type Operation struct {
	Server  string
	Channel string
	Send    *Message
	Receive *Receive
	// Headers are connection headers (the WebSocket handshake, the SSE
	// request); message headers are Send.Headers.
	Headers []Header
	// Binding holds the protocol's options by name.
	Binding map[string]string
	TLS     *tls.Config
	Clock   func() time.Time // nil → time.Now
}

// Result is what a run did. It is also returned with ErrTimeout and ErrTooLarge,
// holding what was collected before the receive stopped.
type Result struct {
	Sent     *Message
	Received []Message
	Connect  time.Duration
	Total    time.Duration
}

// Transport runs operations for one protocol. Run must return promptly when
// ctx ends and leave no goroutine behind.
type Transport interface {
	Run(ctx context.Context, op Operation) (Result, error)
}

var (
	// ErrTimeout means the receive timed out before enough messages arrived.
	ErrTimeout = errors.New("receive timed out")
	// ErrTooLarge means the messages kept exceeded Receive.MaxBytes.
	ErrTooLarge = errors.New("received messages exceed the size limit")
)

// Registry looks transports up by protocol name.
type Registry map[string]Transport

// Lookup returns the transport for a protocol.
func (r Registry) Lookup(protocol string) (Transport, bool) {
	t, ok := r[protocol]
	return t, ok
}

// Defaults for a receive with no count or timeout.
const (
	DefaultCount   = 1
	DefaultTimeout = 30 * time.Second
)

// Normalized returns r with its defaults filled in.
func (r Receive) Normalized() Receive {
	if r.Count <= 0 {
		r.Count = DefaultCount
	}
	if r.Timeout <= 0 {
		r.Timeout = DefaultTimeout
	}
	return r
}

// Now is the operation's clock.
func (op Operation) Now() time.Time {
	if op.Clock != nil {
		return op.Clock()
	}
	return time.Now()
}

// HeaderValue returns the value of the first connection header called name
// (case-insensitive).
func (op Operation) HeaderValue(name string) string {
	for _, h := range op.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// JoinURL puts a channel after a server URL: "wss://h" and "events" give
// "wss://h/events"; an empty channel leaves the server alone.
func JoinURL(server, channel string) string { return model.JoinURL(server, channel) }

// IsText reports whether a body should be sent as a text frame.
func IsText(b []byte) bool { return utf8.Valid(b) }

// Wait collects messages from in until the receive is satisfied, the timeout
// passes, fail reports an error, or ctx ends. The result is the messages
// kept; with ErrTimeout or ErrTooLarge it is the partial list. in must be
// fed by a goroutine the caller stops after Wait returns.
func Wait(ctx context.Context, r Receive, in <-chan Message, fail <-chan error) ([]Message, error) {
	return collect(ctx, r, func(ctx context.Context) (Message, error) {
		select {
		case <-ctx.Done():
			return Message{}, context.Cause(ctx)
		case err := <-fail:
			return Message{}, err
		case m := <-in:
			return m, nil
		}
	})
}

// collect keeps what next hands over until r is satisfied. next blocks for the
// next message and returns the error that ends the receive (the context's
// cause, a lost connection).
func collect(ctx context.Context, r Receive, next func(context.Context) (Message, error)) ([]Message, error) {
	r = r.Normalized()
	ctx, cancel := context.WithTimeoutCause(ctx, r.Timeout, ErrTimeout)
	defer cancel()
	var kept []Message
	var size int64
	for len(kept) < r.Count {
		m, err := next(ctx)
		if err != nil {
			return kept, err
		}
		ok, err := matches(r, m)
		if err != nil {
			return kept, fmt.Errorf("matching a message: %w", err)
		}
		if !ok {
			continue
		}
		kept = append(kept, m)
		if r.OnMessage != nil {
			r.OnMessage(m)
		}
		if r.OnKept != nil {
			if err := r.OnKept(m); err != nil {
				return kept, err
			}
		}
		size += int64(len(m.Body))
		if r.MaxBytes > 0 && size > r.MaxBytes {
			return kept, ErrTooLarge
		}
	}
	return kept, nil
}

func matches(r Receive, m Message) (bool, error) {
	if r.Match == nil {
		return true, nil
	}
	return r.Match(m)
}
