// SPDX-License-Identifier: Apache-2.0

// Package graphqlws is the transport of GraphQL subscriptions: the graphql-transport-ws protocol over a WebSocket.
//
// A run connects, sends connection_init, waits for connection_ack, subscribes with the operation in Send.Body (the
// {"query", "operationName", "variables"} object), and keeps the payload of each "next" message until the receive
// is satisfied. A server's "error" message fails the run; its "complete" message or a normal close ends the stream
// early without failing it. The older subscriptions-transport-ws protocol (start/data) is a different protocol and
// is not spoken.
package graphqlws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/transport"
)

// Subprotocol is the WebSocket subprotocol the server must select.
const Subprotocol = "graphql-transport-ws"

// ackTimeout is how long the server has to acknowledge the connection.
const ackTimeout = 10 * time.Second

// readLimit is the largest message accepted when the receive sets no limit.
const readLimit = 16 << 20

// subscriptionID names the one subscription of a connection.
const subscriptionID = "1"

// Transport runs GraphQL subscriptions.
type Transport struct{}

// New returns the GraphQL subscription transport.
func New() Transport { return Transport{} }

// ServerError is a subscription the server ended with an "error" message, or a connection it closed with a reason.
type ServerError struct{ Detail string }

func (e *ServerError) Error() string { return e.Detail }

// errComplete is what the reader reports when the stream ended: the server sent "complete" or closed normally.
var errComplete = errors.New("the server completed the subscription")

// frame is one protocol message.
type frame struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Run connects, subscribes and collects. The ctx bounds the whole run, handshake included.
//
// Goroutine ownership: Run starts one reader goroutine after the handshake; Run stops it and waits for it before
// returning.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	if op.Send == nil || len(op.Send.Body) == 0 {
		return transport.Result{}, errors.New("a subscription needs an operation to send")
	}
	conn, err := dial(ctx, op)
	if err != nil {
		return transport.Result{}, err
	}
	defer func() { _ = conn.CloseNow() }() // already closed after a normal close; this frees an abandoned connection
	if err := handshake(ctx, conn); err != nil {
		return transport.Result{}, err
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	if err := write(ctx, conn, frame{ID: subscriptionID, Type: "subscribe", Payload: op.Send.Body}); err != nil {
		return res, err
	}
	sent := *op.Send
	sent.Direction, sent.At, sent.Type = "sent", op.Now(), "subscribe"
	res.Sent = &sent

	rd := startReader(ctx, conn, op)
	defer rd.stop()
	receive := transport.Receive{}
	if op.Receive != nil {
		receive = *op.Receive
	}
	res.Received, err = transport.Wait(ctx, receive, rd.messages, rd.failed)
	if errors.Is(err, errComplete) {
		err = nil // a finished stream is a result, whatever count was asked for
	}
	if err == nil {
		_ = write(ctx, conn, frame{ID: subscriptionID, Type: "complete"}) // a polite end; the messages are already collected
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

// Probe connects and gets the connection acknowledged, without subscribing.
func (Transport) Probe(ctx context.Context, op transport.Operation) error {
	conn, err := dial(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()
	if err := handshake(ctx, conn); err != nil {
		return err
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
	return nil
}

var _ transport.Prober = Transport{}

// wsURL is the address of the WebSocket: an http or https URL of a GraphQL endpoint is the same endpoint over ws or
// wss, so a request can keep the URL it has for its queries.
func wsURL(server, channel string) string {
	u := transport.JoinURL(server, channel)
	switch {
	case strings.HasPrefix(u, "https://"):
		return "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		return "ws://" + strings.TrimPrefix(u, "http://")
	}
	return u
}

func dial(ctx context.Context, op transport.Operation) (*websocket.Conn, error) {
	header := http.Header{}
	for _, h := range op.Headers {
		header.Add(h.Name, h.Value)
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = op.TLS
	target := wsURL(op.Server, op.Channel)
	conn, resp, err := websocket.Dial( //nolint:bodyclose // the library hands back a response whose body is already read and closed
		ctx, target, &websocket.DialOptions{HTTPHeader: header, HTTPClient: &http.Client{Transport: t}, Subprotocols: []string{Subprotocol}})
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("the server did not accept the WebSocket upgrade (HTTP %s): does it serve subscriptions at this URL?", resp.Status)
		}
		return nil, fmt.Errorf("connecting to %s: %s", hostOf(target), withoutURL(err.Error(), target))
	}
	if conn.Subprotocol() != Subprotocol {
		_ = conn.Close(websocket.StatusProtocolError, "expected "+Subprotocol)
		return nil, fmt.Errorf("the server does not speak %s (it selected %q)", Subprotocol, conn.Subprotocol())
	}
	limit := int64(readLimit)
	if op.Receive != nil && op.Receive.MaxBytes > 0 {
		limit = op.Receive.MaxBytes + 1
	}
	conn.SetReadLimit(limit)
	return conn, nil
}

// hostOf is the host of a URL for an error message: the path and query may hold values the user would not want shown.
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return "the server"
}

// withoutURL takes the address out of a dial error: it has the path and query, which may hold a token. The library
// quotes the http form of the address.
func withoutURL(msg, target string) string {
	host := hostOf(target)
	for _, form := range []string{target, "http" + strings.TrimPrefix(target, "ws")} {
		msg = strings.ReplaceAll(msg, form, host)
	}
	return msg
}

func write(ctx context.Context, conn *websocket.Conn, f frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("encoding a %s message: %w", f.Type, err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("sending: %w", err)
	}
	return nil
}

// handshake sends connection_init and waits for connection_ack, answering pings meanwhile.
func handshake(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithTimeoutCause(ctx, ackTimeout, errNoAck)
	defer cancel()
	if err := write(ctx, conn, frame{Type: "connection_init"}); err != nil {
		return err
	}
	for {
		f, err := read(ctx, conn)
		if err != nil {
			if errors.Is(context.Cause(ctx), errNoAck) {
				return &ServerError{Detail: "the server did not acknowledge the connection (no connection_ack within " + ackTimeout.String() + ")"}
			}
			return err
		}
		switch f.Type {
		case "connection_ack":
			return nil
		case "ping":
			if err := write(ctx, conn, frame{Type: "pong"}); err != nil {
				return err
			}
		}
	}
}

var errNoAck = errors.New("no connection_ack")

// read reads one protocol message. A close with a reason is a ServerError that says it.
func read(ctx context.Context, conn *websocket.Conn) (frame, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		var ce websocket.CloseError
		if errors.As(err, &ce) {
			if ce.Code == websocket.StatusNormalClosure || ce.Code == websocket.StatusGoingAway {
				return frame{}, errComplete
			}
			return frame{}, &ServerError{Detail: closeDetail(ce)}
		}
		return frame{}, fmt.Errorf("receiving: %w", err)
	}
	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		return frame{}, &ServerError{Detail: "the server sent a message that is not graphql-transport-ws JSON: " + err.Error()}
	}
	return f, nil
}

// closeDetail says how the server closed the connection. The 44xx codes are the ones the protocol names.
func closeDetail(ce websocket.CloseError) string {
	meaning := map[websocket.StatusCode]string{
		4400: "bad request", 4401: "unauthorized", 4403: "forbidden", 4408: "connection initialisation timeout",
		4409: "subscriber for this id already exists", 4429: "too many initialisation requests",
	}[ce.Code]
	detail := fmt.Sprintf("the server closed the connection with code %d", ce.Code)
	if meaning != "" {
		detail += " (" + meaning + ")"
	}
	if ce.Reason != "" {
		detail += ": " + ce.Reason
	}
	return detail
}

// reader feeds the payloads of next messages into messages until the stream ends.
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
			f, err := read(ctx, conn)
			if err != nil {
				if ctx.Err() == nil {
					rd.failed <- err
				}
				return
			}
			switch f.Type {
			case "ping":
				if err := write(ctx, conn, frame{Type: "pong"}); err != nil && ctx.Err() == nil {
					rd.failed <- err
					return
				}
			case "next":
				body := []byte(f.Payload)
				if len(body) == 0 {
					body = []byte("null")
				}
				select {
				case rd.messages <- transport.Message{Direction: "received", At: op.Now(), Body: body, Type: "next"}:
				case <-ctx.Done():
					return
				}
			case "error":
				rd.failed <- &ServerError{Detail: "the server refused the subscription: " + errorText(f.Payload)}
				return
			case "complete":
				rd.failed <- errComplete
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

// errorText is the messages of an error message's payload (a list of GraphQL errors), or the payload as it came.
func errorText(payload json.RawMessage) string {
	var list []struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &list) == nil && len(list) > 0 {
		var msgs []string
		for _, e := range list {
			msgs = append(msgs, e.Message)
		}
		return strings.Join(msgs, "; ")
	}
	if len(payload) == 0 {
		return "(no details)"
	}
	return string(payload)
}
