// SPDX-License-Identifier: Apache-2.0

// Package sse is the Server-Sent Events transport. It only receives.
package sse

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/githugt/flowi/lib/transport"
)

// Transport runs SSE operations.
type Transport struct{}

// New returns the SSE transport.
func New() Transport { return Transport{} }

// Run opens the event stream at the server URL and collects events as messages.
// Each event's id and event fields are kept as headers named "id" and "event".
// The channel is an event name: when it is set, only events of that name are
// kept (an event with no name is "message", as in the spec); empty keeps all.
//
// Goroutine ownership: Run starts one goroutine that reads the stream; Run
// cancels the request and waits for it before returning.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	if op.Send != nil {
		return transport.Result{}, errors.New("sse cannot send: it is a one-way stream. Remove the message to send (Message section) and set what to receive")
	}
	if op.Receive == nil {
		return transport.Result{}, errors.New("sse needs a receive")
	}
	started := op.Now()
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()

	resp, err := open(ctx, op) //nolint:bodyclose // closed by the deferred call just below
	if err != nil {
		return transport.Result{}, err
	}
	defer resp.Body.Close()
	res := transport.Result{Connect: op.Now().Sub(started)}
	messages, failed := make(chan transport.Message), make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := readEvents(ctx, resp, op, messages); err != nil && ctx.Err() == nil {
			failed <- err
		}
	}()
	res.Received, err = transport.Wait(ctx, *op.Receive, messages, failed)
	res.Total = op.Now().Sub(started)
	return res, err
}

func open(ctx context.Context, op transport.Operation) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, op.Server, nil)
	if err != nil {
		return nil, fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for _, h := range op.Headers {
		req.Header.Add(h.Name, h.Value)
	}
	if id := op.Binding["last_event_id"]; id != "" {
		req.Header.Set("Last-Event-ID", id)
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = op.TLS
	resp, err := (&http.Client{Transport: t}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_ = resp.Body.Close() // reporting the status; the close error adds nothing
		return nil, fmt.Errorf("event stream refused: %s", resp.Status)
	}
	return resp, nil
}

// readEvents parses the stream and sends each dispatched event to out.
func readEvents(ctx context.Context, resp *http.Response, op transport.Operation, out chan<- transport.Message) error {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	var data []string
	var id, event string
	dispatch := func() bool {
		defer func() { data, id, event = nil, "", "" }()
		if len(data) == 0 || (op.Channel != "" && eventType(event) != op.Channel) {
			return true
		}
		m := transport.Message{Direction: "received", At: op.Now(), Body: []byte(strings.Join(data, "\n"))}
		if id != "" {
			m.Headers = append(m.Headers, transport.Header{Name: "id", Value: id})
		}
		if event != "" {
			m.Headers = append(m.Headers, transport.Header{Name: "event", Value: event})
		}
		select {
		case out <- m:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if !dispatch() {
				return nil
			}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			data = append(data, value)
		case "id":
			id = value
		case "event":
			event = value
		}
	}
	return sc.Err()
}

// eventType is the name of an event: "message" when it has none.
func eventType(name string) string {
	if name == "" {
		return "message"
	}
	return name
}
