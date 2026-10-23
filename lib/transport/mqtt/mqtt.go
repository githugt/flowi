// SPDX-License-Identifier: Apache-2.0

// Package mqtt is the MQTT 5 transport. MQTT 3.1.1 brokers are not supported.
package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"

	"github.com/githugt/flowi-lib/transport"
)

const keepAlive = 30 // seconds

// Transport runs MQTT operations.
type Transport struct{}

// New returns the MQTT transport.
func New() Transport { return Transport{} }

// Run connects, subscribes (before publishing, with no-local set so a message
// published on the same topic does not come back), publishes, collects, and
// disconnects. Binding names: qos, retain, client_id, username, password.
//
// Goroutine ownership: the paho client owns its read and keep-alive
// goroutines; Run ends them with Disconnect and waits for Done.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	qos, retain, err := publishOptions(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	conn, err := dial(ctx, op)
	if err != nil {
		return transport.Result{}, err
	}
	in, failed := make(chan transport.Message), make(chan error, 1)
	client := paho.NewClient(paho.ClientConfig{
		Conn: packets.NewThreadSafeConn(conn),
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){func(p paho.PublishReceived) (bool, error) {
			select {
			case in <- received(p.Packet, op.Now()):
			case <-ctx.Done():
			}
			return true, nil
		}},
		OnServerDisconnect: func(d *paho.Disconnect) {
			report(failed, fmt.Errorf("the broker disconnected (reason %d)", d.ReasonCode))
		},
		OnClientError: func(err error) { report(failed, err) },
	})
	defer finish(client)
	if _, err := client.Connect(ctx, connectPacket(op)); err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	if op.Receive != nil {
		if err := subscribe(ctx, client, op.Channel, qos); err != nil {
			return res, err
		}
	}
	if op.Send != nil {
		if res.Sent, err = publish(ctx, client, op, qos, retain); err != nil {
			return res, err
		}
	}
	if op.Receive != nil {
		res.Received, err = transport.Wait(ctx, *op.Receive, in, failed)
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

// report passes the first error on without blocking the client's goroutine.
func report(failed chan<- error, err error) {
	select {
	case failed <- err:
	default:
	}
}

// finish says goodbye and waits for the client's goroutines to stop.
func finish(c *paho.Client) {
	_ = c.Disconnect(&paho.Disconnect{ReasonCode: 0}) // the run is over; a broker that already hung up is fine
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
	}
}

func publishOptions(binding map[string]string) (qos byte, retain bool, err error) {
	if v := binding["qos"]; v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 0 || n > 2 {
			return 0, false, fmt.Errorf("binding qos must be 0, 1 or 2, got %q", v)
		}
		qos = byte(n)
	}
	if v := binding["retain"]; v != "" {
		if retain, err = strconv.ParseBool(v); err != nil {
			return 0, false, fmt.Errorf("binding retain must be true or false, got %q", v)
		}
	}
	return qos, retain, nil
}

func connectPacket(op transport.Operation) *paho.Connect {
	id := op.Binding["client_id"]
	if id == "" {
		id = "flowi-" + strconv.FormatInt(op.Now().UnixNano(), 36)
	}
	c := &paho.Connect{ClientID: id, KeepAlive: keepAlive, CleanStart: true}
	if u := op.Binding["username"]; u != "" {
		c.Username, c.UsernameFlag = u, true
	}
	if p := op.Binding["password"]; p != "" {
		c.Password, c.PasswordFlag = []byte(p), true
	}
	return c
}

func subscribe(ctx context.Context, c *paho.Client, topic string, qos byte) error {
	ack, err := c.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: qos, NoLocal: true}}})
	if err != nil {
		return fmt.Errorf("subscribing to %s: %w", topic, err)
	}
	for _, code := range ack.Reasons {
		if code >= 0x80 {
			return fmt.Errorf("the broker refused the subscription to %s (reason %d)", topic, code)
		}
	}
	return nil
}

func publish(ctx context.Context, c *paho.Client, op transport.Operation, qos byte, retain bool) (*transport.Message, error) {
	p := &paho.Publish{Topic: op.Channel, QoS: qos, Retain: retain, Payload: op.Send.Body}
	if len(op.Send.Headers) > 0 {
		p.Properties = &paho.PublishProperties{}
		for _, h := range op.Send.Headers {
			p.Properties.User.Add(h.Name, h.Value)
		}
	}
	resp, err := c.Publish(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("publishing to %s: %w", op.Channel, err)
	}
	if resp != nil && resp.ReasonCode >= 0x80 {
		return nil, fmt.Errorf("the broker refused the message (reason %d)", resp.ReasonCode)
	}
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	return &m, nil
}

// received turns a PUBLISH into a message: the topic and user properties
// become headers.
func received(p *paho.Publish, at time.Time) transport.Message {
	m := transport.Message{Direction: "received", At: at, Body: p.Payload, Headers: []transport.Header{{Name: "topic", Value: p.Topic}}}
	if p.Properties != nil {
		for _, u := range p.Properties.User {
			m.Headers = append(m.Headers, transport.Header{Name: u.Key, Value: u.Value})
		}
	}
	return m
}

// dial opens the network connection for the server URL: mqtt, mqtts, ws or wss.
func dial(ctx context.Context, op transport.Operation) (net.Conn, error) {
	u, err := url.Parse(op.Server)
	if err != nil {
		return nil, fmt.Errorf("server %q is not a URL: %w", op.Server, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "mqtt":
		return dialTCP(ctx, withPort(u, "1883"), nil)
	case "mqtts":
		return dialTCP(ctx, withPort(u, "8883"), tlsFor(op, u))
	case "ws", "wss":
		return dialWebSocket(ctx, op)
	}
	return nil, fmt.Errorf("mqtt cannot use the scheme %q (use mqtt, mqtts, ws or wss)", u.Scheme)
}

func withPort(u *url.URL, def string) string {
	if u.Port() != "" {
		return u.Host
	}
	return net.JoinHostPort(u.Hostname(), def)
}

func tlsFor(op transport.Operation, u *url.URL) *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if op.TLS != nil {
		cfg = op.TLS.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = u.Hostname()
	}
	return cfg
}

func dialTCP(ctx context.Context, addr string, cfg *tls.Config) (net.Conn, error) {
	d := &net.Dialer{}
	if cfg != nil {
		conn, err := (&tls.Dialer{NetDialer: d, Config: cfg}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("connecting to %s: %w", addr, err)
		}
		return conn, nil
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	return conn, nil
}

func dialWebSocket(ctx context.Context, op transport.Operation) (net.Conn, error) {
	c, _, err := websocket.Dial(ctx, transport.JoinURL(op.Server, ""), &websocket.DialOptions{Subprotocols: []string{"mqtt"}}) //nolint:bodyclose // the library hands back a response whose body is already read and closed
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	return websocket.NetConn(context.WithoutCancel(ctx), c, websocket.MessageBinary), nil
}
