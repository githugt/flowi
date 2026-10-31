// SPDX-License-Identifier: Apache-2.0

//go:build integration

// These tests talk to real brokers: run `just test-integration`, which starts
// testdata/compose/docker-compose.yml (podman compose, else docker compose).
// Addresses can be changed with FLOWI_TEST_MQTT, _NATS, _KAFKA, _AMQP and _AMQP10.
// WebSocket and SSE need no broker and are tested with the unit tests.

package transport_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/amqp"
	"github.com/githugt/flowi-lib/transport/amqp10"
	"github.com/githugt/flowi-lib/transport/kafka"
	"github.com/githugt/flowi-lib/transport/mqtt"
	"github.com/githugt/flowi-lib/transport/nats"
)

const brokerWait = 90 * time.Second

// address is a broker's server URL: the environment variable, else the compose default.
func address(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return fallback
}

// waitForPort waits until the broker behind server accepts connections.
func waitForPort(t *testing.T, server string) {
	t.Helper()
	u, err := url.Parse(strings.Replace(server, "kafka://", "tcp://", 1))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(brokerWait)
	for time.Now().Before(deadline) {
		c, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", u.Host)
		if err == nil {
			_ = c.Close()
			return
		}
		select {
		case <-t.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatalf("%s did not come up in %s: is the compose file running? (just test-integration)", server, brokerWait)
}

// retry runs op until it succeeds or the broker has had time to settle: a
// port that accepts connections is not always a broker that is ready.
func retry(t *testing.T, op func() error) {
	t.Helper()
	deadline := time.Now().Add(brokerWait)
	var err error
	for time.Now().Before(deadline) {
		if err = op(); err == nil {
			return
		}
		select {
		case <-t.Context().Done():
			t.Fatal(err)
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("still failing after %s: %v", brokerWait, err)
}

func unique(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func bodies(ms []transport.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.Body))
	}
	return out
}

func TestMQTT_against_mosquitto(t *testing.T) {
	server := address("FLOWI_TEST_MQTT", "mqtt://127.0.0.1:1883")
	waitForPort(t, server)
	topic := unique("flowi/it")

	var published transport.Result
	retry(t, func() (err error) {
		published, err = mqtt.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: topic, Binding: map[string]string{"qos": "1", "retain": "true"},
			Send: &transport.Message{Body: []byte("hello")},
		})
		return err
	})
	got, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: topic, Receive: &transport.Receive{Timeout: 10 * time.Second},
	})

	if err != nil || published.Sent == nil || len(got.Received) != 1 || string(got.Received[0].Body) != "hello" {
		t.Fatalf("err %v received %v", err, bodies(got.Received))
	}
}

func TestNATS_against_nats_server_core_and_jetstream(t *testing.T) {
	server := address("FLOWI_TEST_NATS", "nats://127.0.0.1:4222")
	waitForPort(t, server)
	nc, err := natsgo.Connect(server)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	subject := unique("it") + ".svc"
	if _, err := nc.Subscribe(subject, func(m *natsgo.Msg) { _ = m.Respond(append([]byte("echo:"), m.Data...)) }); err != nil {
		t.Fatal(err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatal(err)
	}
	reply, err := nats.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: subject, Binding: map[string]string{"reply_to": subject + ".reply"},
		Send: &transport.Message{Body: []byte("ping")}, Receive: &transport.Receive{Timeout: 10 * time.Second},
	})
	if err != nil || len(reply.Received) != 1 || string(reply.Received[0].Body) != "echo:ping" {
		t.Fatalf("core: err %v received %v", err, bodies(reply.Received))
	}

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream := strings.ToUpper(strings.ReplaceAll(unique("s"), "-", "_"))
	if _, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: stream, Subjects: []string{stream + ".>"}}); err != nil {
		t.Fatal(err)
	}
	sent, err := nats.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: stream + ".a", Binding: map[string]string{"stream": stream}, Send: &transport.Message{Body: []byte("one")},
	})
	read, err2 := nats.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: stream + ".a", Binding: map[string]string{"stream": stream, "deliver": "all"},
		Receive: &transport.Receive{Timeout: 10 * time.Second},
	})
	if err != nil || err2 != nil || sent.Sent.Sequence != 1 || len(read.Received) != 1 || read.Received[0].Sequence != 1 {
		t.Fatalf("jetstream: %v %v sent %+v received %+v", err, err2, sent.Sent, read.Received)
	}
}

func TestKafka_against_apache_kafka(t *testing.T) {
	server := address("FLOWI_TEST_KAFKA", "kafka://127.0.0.1:9092")
	waitForPort(t, server)
	topic := unique("flowi-it")

	retry(t, func() error {
		_, err := kafka.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: topic, Binding: map[string]string{"key": "k"}, Send: &transport.Message{Body: []byte("one")},
		})
		return err
	})
	got, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: topic, Binding: map[string]string{"offset": "earliest"}, Receive: &transport.Receive{Timeout: 20 * time.Second},
	})

	if err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "one" || got.Received[0].Key != "k" {
		t.Fatalf("err %v received %+v", err, got.Received)
	}
}

func TestAMQP_against_rabbitmq(t *testing.T) {
	server := address("FLOWI_TEST_AMQP", "amqp://flowi:flowi@127.0.0.1:5672/")
	waitForPort(t, strings.Replace(server, "amqp://", "tcp://", 1))
	var conn *amqp091.Connection
	retry(t, func() (err error) { conn, err = amqp091.Dial(server); return err })
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("publish to a queue and read it back", func(t *testing.T) {
		queue := unique("flowi.it")
		if _, err := ch.QueueDeclare(queue, false, true, false, false, nil); err != nil {
			t.Fatal(err)
		}

		_, sendErr := amqp.New().Run(t.Context(), transport.Operation{Server: server, Channel: queue, Send: &transport.Message{Body: []byte("one")}})
		got, err := amqp.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: queue, Binding: map[string]string{"queue": queue}, Receive: &transport.Receive{Timeout: 10 * time.Second},
		})

		if sendErr != nil || err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "one" {
			t.Fatalf("send %v receive %v received %v", sendErr, err, bodies(got.Received))
		}
	})

	t.Run("request and reply through an exchange", func(t *testing.T) {
		key := unique("ask")
		responder, err := conn.Channel()
		if err != nil {
			t.Fatal(err)
		}
		q, err := responder.QueueDeclare("", false, true, true, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := responder.QueueBind(q.Name, key, "amq.direct", false, nil); err != nil {
			t.Fatal(err)
		}
		deliveries, err := responder.Consume(q.Name, "", true, false, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, stop := context.WithCancel(t.Context())
		defer stop()
		go func() { // answers "ping" with "pong" on the same key; it ignores its own answer
			for {
				select {
				case <-ctx.Done():
					return
				case d := <-deliveries:
					if string(d.Body) == "ping" {
						_ = responder.PublishWithContext(ctx, "amq.direct", key, false, false, amqp091.Publishing{Body: []byte("pong")})
					}
				}
			}
		}()

		got, err := amqp.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: key, Binding: map[string]string{"exchange": "amq.direct"},
			Send: &transport.Message{Body: []byte("ping")}, Receive: &transport.Receive{Timeout: 15 * time.Second},
		})

		if err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "pong" {
			t.Fatalf("err %v received %v", err, bodies(got.Received))
		}
	})
}

// AMQP 1.0 is spoken by ActiveMQ Artemis here (RabbitMQ in this compose file is the 0-9-1 transport's).
func TestAMQP10_against_artemis(t *testing.T) {
	server := address("FLOWI_TEST_AMQP10", "amqp://127.0.0.1:5673")
	waitForPort(t, strings.Replace(server, "amqp://", "tcp://", 1))
	binding := map[string]string{"username": "flowi", "password": "flowi"}
	queue := fmt.Sprintf("flowi.test.%d", time.Now().UnixNano())

	t.Run("send then receive keeps the body and the headers", func(t *testing.T) {
		retry(t, func() error {
			_, err := amqp10.New().Run(t.Context(), transport.Operation{
				Server: server, Channel: queue, Binding: binding,
				Send: &transport.Message{Body: []byte("one"), Headers: []transport.Header{{Name: "type", Value: "order"}}},
			})
			return err
		})
		got, err := amqp10.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: queue, Binding: binding, Receive: &transport.Receive{Timeout: 10 * time.Second},
		})
		if err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "one" {
			t.Fatalf("err %v received %+v", err, got.Received)
		}
		headers := map[string]string{}
		for _, h := range got.Received[0].Headers {
			headers[h.Name] = h.Value
		}
		if headers["type"] != "order" {
			t.Errorf("headers %v", headers)
		}
	})

	t.Run("a wrong password is refused", func(t *testing.T) {
		_, err := amqp10.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: queue, Binding: map[string]string{"username": "flowi", "password": "nope"}, Send: &transport.Message{Body: []byte("x")},
		})
		if err == nil || strings.Contains(err.Error(), "nope") {
			t.Fatalf("err %v (must fail, and must not quote the password)", err)
		}
	})
}
