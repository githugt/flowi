// SPDX-License-Identifier: Apache-2.0

// Package ws is the WebSocket transport.
package ws

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/transport"
)

// readLimit is the largest message accepted when the receive sets no limit.
const readLimit = 16 << 20

// Transport runs WebSocket operations.
type Transport struct{}

// New returns the WebSocket transport.
func New() Transport { return Transport{} }

// Run connects, listens (before sending, so a fast reply is not lost), sends,
// collects, and closes.
//
// Goroutine ownership: Run starts one reader goroutine when the operation
// receives; Run closes the connection and waits for it before returning.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	conn, err := dial(ctx, op)
	if err != nil {
		return transport.Result{}, err
	}
	defer func() { _ = conn.CloseNow() }() // already closed after a normal close; this frees an abandoned connection
	res := transport.Result{Connect: op.Now().Sub(started)}
	var rd *reader
	if op.Receive != nil {
		rd = startReader(ctx, conn, op)
		defer rd.stop()
	}
	if op.Send != nil {
		if res.Sent, err = send(ctx, conn, op); err != nil {
			return res, err
		}
	}
	if rd != nil {
		res.Received, err = transport.Wait(ctx, *op.Receive, rd.messages, rd.failed)
	}
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "") // the messages are already collected; a failed goodbye changes nothing
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

func dial(ctx context.Context, op transport.Operation) (*websocket.Conn, error) {
	header := http.Header{}
	for _, h := range op.Headers {
		header.Add(h.Name, h.Value)
	}
	opts := &websocket.DialOptions{HTTPHeader: header, HTTPClient: &http.Client{Transport: tlsTransport(op)}}
	if sub := op.Binding["subprotocol"]; sub != "" {
		opts.Subprotocols = strings.Split(sub, ",")
	}
	conn, resp, err := websocket.Dial( //nolint:bodyclose // the library hands back a response whose body is already read and closed
		ctx, transport.JoinURL(op.Server, op.Channel), opts)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket handshake refused: %s", resp.Status)
		}
		return nil, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	limit := int64(readLimit)
	if op.Receive != nil && op.Receive.MaxBytes > 0 {
		limit = op.Receive.MaxBytes + 1
	}
	conn.SetReadLimit(limit)
	return conn, nil
}

func tlsTransport(op transport.Operation) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = op.TLS
	return t
}

func send(ctx context.Context, conn *websocket.Conn, op transport.Operation) (*transport.Message, error) {
	kind := websocket.MessageBinary
	if transport.IsText(op.Send.Body) {
		kind = websocket.MessageText
	}
	if err := conn.Write(ctx, kind, op.Send.Body); err != nil {
		return nil, fmt.Errorf("sending: %w", err)
	}
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	return &m, nil
}

// reader feeds received frames into messages until the connection ends.
type reader struct {
	messages chan transport.Message
	failed   chan error
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

func startReader(ctx context.Context, conn *websocket.Conn, op transport.Operation) *reader {
	ctx, cancel := context.WithCancel(ctx)
	rd := &reader{messages: make(chan transport.Message), failed: make(chan error, 1), cancel: cancel}
	rd.wg.Add(1)
	go func() {
		defer rd.wg.Done()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					rd.failed <- fmt.Errorf("receiving: %w", err)
				}
				return
			}
			select {
			case rd.messages <- transport.Message{Direction: "received", At: op.Now(), Body: data}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return rd
}

// stop ends the reader and waits for it.
func (rd *reader) stop() {
	rd.cancel()
	rd.wg.Wait()
}
