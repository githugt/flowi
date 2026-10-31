// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	paho311 "github.com/eclipse/paho.mqtt.golang"

	"github.com/githugt/flowi/lib/transport"
)

// Protocol levels of the `version` binding. MQTT 5 is the default.
const (
	version5     = "5"
	version311   = "3.1.1"
	version31    = "3.1"
	level31      = 3
	level311     = 4
	disconnectMS = 250
)

// versionOf reads the `version` binding: "" and "5" are MQTT 5; "3.1.1" and
// "3.1" are the older protocols that many brokers, devices and hosted
// services still speak alone.
func versionOf(binding map[string]string) (string, error) {
	switch v := binding["version"]; v {
	case "", version5:
		return version5, nil
	case version311, version31:
		return v, nil
	default:
		return "", fmt.Errorf("binding version must be 5, 3.1.1 or 3.1, got %q", v)
	}
}

// run311 is Run for MQTT 3.1.1 and 3.1, on the Eclipse Paho client.
//
// What differs from MQTT 5: a message has no user properties, so message
// headers cannot be sent (an error, never silently dropped) and a received
// message has only the topic header; and there is no "no local" option, so
// the first message that comes back equal to the one just sent, on the same
// topic, is taken for our own echo and not kept.
//
// Goroutine ownership: the Paho client owns its goroutines; Disconnect ends
// them. The handler never blocks past the end of the run (it watches done).
func run311(ctx context.Context, op transport.Operation, level uint) (transport.Result, error) {
	started := op.Now()
	qos, retain, err := publishOptions(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	if op.Send != nil && len(op.Send.Headers) > 0 {
		return transport.Result{}, errors.New("MQTT 3.1.1 messages have no headers: remove the message headers, or set the binding version to 5")
	}
	broker, err := brokerURL(op)
	if err != nil {
		return transport.Result{}, err
	}
	in, failed, done := make(chan transport.Message), make(chan error, 1), make(chan struct{})
	defer close(done)
	client := paho311.NewClient(options311(op, broker, level, failed))
	if err := waitFor(ctx, client.Connect()); err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	defer client.Disconnect(disconnectMS)
	res := transport.Result{Connect: op.Now().Sub(started)}
	echo := &echoFilter{}
	if op.Receive != nil {
		handler := func(_ paho311.Client, m paho311.Message) {
			if echo.isEcho(m.Topic(), m.Payload()) {
				return
			}
			select {
			case in <- received311(m, op.Now()):
			case <-done:
			case <-ctx.Done():
			}
		}
		if err := waitFor(ctx, client.Subscribe(op.Channel, qos, handler)); err != nil {
			return res, fmt.Errorf("subscribing to %s: %w", op.Channel, err)
		}
	}
	if op.Send != nil {
		echo.expect(op.Channel, op.Send.Body, op.Receive != nil && !retain)
		if err := waitFor(ctx, client.Publish(op.Channel, qos, retain, op.Send.Body)); err != nil {
			return res, fmt.Errorf("publishing to %s: %w", op.Channel, err)
		}
		m := *op.Send
		m.Direction, m.At = "sent", op.Now()
		res.Sent = &m
	}
	if op.Receive != nil {
		res.Received, err = transport.Wait(ctx, *op.Receive, in, failed)
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

// echoFilter drops the first message that matches what was just published
// (MQTT 3.1.1 cannot ask the broker not to send it back).
type echoFilter struct {
	mu      sync.Mutex
	topic   string
	payload []byte
	armed   bool
}

func (e *echoFilter) expect(topic string, payload []byte, armed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.topic, e.payload, e.armed = topic, payload, armed
}

func (e *echoFilter) isEcho(topic string, payload []byte) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.armed && topic == e.topic && bytes.Equal(payload, e.payload) {
		e.armed = false
		return true
	}
	return false
}

func options311(op transport.Operation, broker string, level uint, failed chan<- error) *paho311.ClientOptions {
	id := op.Binding["client_id"]
	if id == "" {
		id = "flowi-" + strconv.FormatInt(op.Now().UnixNano(), 36)
	}
	o := paho311.NewClientOptions().AddBroker(broker).SetClientID(id).SetProtocolVersion(level).
		SetCleanSession(true).SetKeepAlive(keepAlive * time.Second).
		SetAutoReconnect(false).SetConnectRetry(false).SetOrderMatters(true)
	if u := op.Binding["username"]; u != "" {
		o.SetUsername(u)
	}
	if p := op.Binding["password"]; p != "" {
		o.SetPassword(p)
	}
	if strings.HasPrefix(broker, "ssl://") || strings.HasPrefix(broker, "wss://") {
		u, _ := url.Parse(op.Server)
		o.SetTLSConfig(tlsFor(op, u))
	}
	o.SetConnectionLostHandler(func(_ paho311.Client, err error) {
		report(failed, fmt.Errorf("the connection to the broker was lost: %w", err))
	})
	return o
}

// brokerURL turns the server URL into Paho's: mqtt→tcp, mqtts→ssl.
func brokerURL(op transport.Operation) (string, error) {
	u, err := url.Parse(op.Server)
	if err != nil {
		return "", fmt.Errorf("server %q is not a URL: %w", op.Server, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "mqtt":
		return "tcp://" + withPort(u, "1883"), nil
	case "mqtts":
		return "ssl://" + withPort(u, "8883"), nil
	case "ws", "wss":
		return transport.JoinURL(op.Server, ""), nil
	}
	return "", fmt.Errorf("mqtt cannot use the scheme %q (use mqtt, mqtts, ws or wss)", u.Scheme)
}

// waitFor waits for a Paho token, or for the run to be cancelled.
func waitFor(ctx context.Context, t paho311.Token) error {
	select {
	case <-t.Done():
		return t.Error()
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func received311(m paho311.Message, at time.Time) transport.Message {
	return transport.Message{Direction: "received", At: at, Body: m.Payload(), Headers: []transport.Header{{Name: "topic", Value: m.Topic()}}}
}
