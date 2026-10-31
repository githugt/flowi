// SPDX-License-Identifier: Apache-2.0

package redis_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/redis"
)

func startRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func server(m *miniredis.Miniredis) string { return "redis://" + m.Addr() }

func client(t *testing.T, m *miniredis.Miniredis) *goredis.Client {
	t.Helper()
	c := goredis.NewClient(&goredis.Options{Addr: m.Addr(), Protocol: 2, DisableIdentity: true})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func body(s string) *transport.Message { return &transport.Message{Body: []byte(s)} }

func recv(count int, timeout time.Duration) *transport.Receive {
	return &transport.Receive{Count: count, Timeout: timeout}
}

// publishWhenSubscribed publishes the messages once a subscriber is on
// channel (or pattern), polling instead of sleeping a fixed time.
func publishWhenSubscribed(t *testing.T, m *miniredis.Miniredis, channel string, messages ...string) {
	t.Helper()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for m.PubSubNumSub(channel)[channel] == 0 && m.PubSubNumPat() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		for _, msg := range messages {
			m.Publish(channel, msg)
		}
	}()
}

func TestRun_should_publish_to_a_channel_a_subscriber_is_on(t *testing.T) {
	m := startRedis(t)
	sub := m.NewSubscriber()
	sub.Subscribe("orders")
	got := make(chan miniredis.PubsubMessage, 1)
	go func() { got <- <-sub.Messages() }() // the subscriber blocks a publisher until its message is read

	res, err := redis.New().Run(t.Context(), transport.Operation{Server: server(m), Channel: "orders", Send: body("hello")})

	if err != nil {
		t.Fatal(err)
	}
	delivered := <-got
	if delivered.Message != "hello" || res.Sent == nil || res.Sent.Direction != "sent" {
		t.Fatalf("subscriber got %q, sent %+v", delivered.Message, res.Sent)
	}
}

func TestRun_should_receive_from_a_channel_subscribing_before_anything_is_published(t *testing.T) {
	m := startRedis(t)
	publishWhenSubscribed(t, m, "orders", "one", "two")

	res, err := redis.New().Run(t.Context(), transport.Operation{Server: server(m), Channel: "orders", Receive: recv(2, 5*time.Second)})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 2 || string(res.Received[0].Body) != "one" || string(res.Received[1].Body) != "two" {
		t.Fatalf("received %+v", res.Received)
	}
	if h := headerOf(res.Received[0], "channel"); h != "orders" {
		t.Fatalf("channel header %q, want orders", h)
	}
}

func TestRun_should_subscribe_to_a_pattern_when_asked(t *testing.T) {
	m := startRedis(t)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for m.PubSubNumPat() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		m.Publish("orders.eu", "pattern hit")
	}()

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders.*", Binding: map[string]string{"pattern": "true"}, Receive: recv(1, 5*time.Second)})

	if err != nil {
		t.Fatal(err)
	}
	if h := headerOf(res.Received[0], "channel"); h != "orders.eu" {
		t.Fatalf("channel header %q, want orders.eu", h)
	}
}

func TestRun_should_return_what_arrived_with_a_timeout(t *testing.T) {
	m := startRedis(t)
	publishWhenSubscribed(t, m, "orders", "only one")

	res, err := redis.New().Run(t.Context(), transport.Operation{Server: server(m), Channel: "orders", Receive: recv(2, 300*time.Millisecond)})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d, want ErrTimeout and 1", err, len(res.Received))
	}
}

func TestRun_should_return_promptly_when_the_context_ends(t *testing.T) {
	m := startRedis(t)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	started := time.Now()

	_, err := redis.New().Run(ctx, transport.Operation{Server: server(m), Channel: "orders", Receive: recv(1, time.Minute)})

	if err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("err %v after %v, want an error well before the receive timeout", err, time.Since(started))
	}
}

func TestRun_should_add_a_stream_entry_with_the_headers_as_fields(t *testing.T) {
	m := startRedis(t)
	msg := &transport.Message{Body: []byte("payload"), Headers: []transport.Header{{Name: "type", Value: "order"}}}

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Binding: map[string]string{"mode": "stream"}, Send: msg})

	if err != nil {
		t.Fatal(err)
	}
	entries, err := m.Stream("orders")
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, err %v", entries, err)
	}
	if got := strings.Join(entries[0].Values, ","); got != "body,payload,type,order" {
		t.Fatalf("fields %q, want body,payload,type,order", got)
	}
	if id := headerOf(*res.Sent, "id"); id != entries[0].ID {
		t.Fatalf("sent id %q, entry %q", id, entries[0].ID)
	}
}

func TestRun_should_read_a_stream_from_the_start_when_asked(t *testing.T) {
	m := startRedis(t)
	addEntry(t, m, "orders", "old")
	addEntry(t, m, "orders", "older")

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Binding: map[string]string{"mode": "stream", "start": "0"}, Receive: recv(2, 5*time.Second)})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 2 || string(res.Received[0].Body) != "old" {
		t.Fatalf("received %+v", res.Received)
	}
	if headerOf(res.Received[0], "stream") != "orders" || headerOf(res.Received[0], "id") == "" {
		t.Fatalf("headers %+v lack stream and id", res.Received[0].Headers)
	}
}

func TestRun_should_not_return_older_entries_when_a_stream_is_read_from_now(t *testing.T) {
	m := startRedis(t)
	addEntry(t, m, "orders", "old")

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Binding: map[string]string{"mode": "stream"}, Receive: recv(1, 300*time.Millisecond)})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 0 {
		t.Fatalf("err %v, received %+v, want ErrTimeout and nothing", err, res.Received)
	}
}

func TestRun_should_skip_its_own_entry_and_return_the_reply_on_a_stream(t *testing.T) {
	m := startRedis(t)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if entries, _ := m.Stream("orders"); len(entries) == 1 {
				_, _ = m.XAdd("orders", "*", []string{"body", "reply"})
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Binding: map[string]string{"mode": "stream"},
		Send: body("request"), Receive: recv(1, 5*time.Second)})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || string(res.Received[0].Body) != "reply" {
		t.Fatalf("received %+v, want only the reply", res.Received)
	}
}

func TestRun_should_ack_the_entries_kept_and_leave_the_skipped_ones_pending_in_a_group(t *testing.T) {
	m := startRedis(t)
	skipped := addEntry(t, m, "orders", "skip me")
	kept := addEntry(t, m, "orders", "keep me")
	rc := recv(1, 5*time.Second)
	rc.Match = func(msg transport.Message) (bool, error) { return strings.HasPrefix(string(msg.Body), "keep"), nil }

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Receive: rc,
		Binding: map[string]string{"mode": "stream", "group": "g", "consumer": "c1", "start": "0"}})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || headerOf(res.Received[0], "id") != kept {
		t.Fatalf("received %+v, want the entry %s", res.Received, kept)
	}
	pending, err := client(t, m).XPendingExt(t.Context(), &goredis.XPendingExtArgs{Stream: "orders", Group: "g", Start: "-", End: "+", Count: 10}).Result()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range pending {
		ids[p.ID] = true
	}
	if ids[kept] || !ids[skipped] {
		t.Fatalf("pending %v: kept %s must be acked, skipped %s must not", ids, kept, skipped)
	}
}

func TestRun_should_sign_in_with_the_credentials_of_the_binding(t *testing.T) {
	m := startRedis(t)
	m.RequireUserAuth("flowi", "s3cret-pw")
	publishWhenSubscribed(t, m, "orders", "hi")

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Receive: recv(1, 5*time.Second),
		Binding: map[string]string{"username": "flowi", "password": "s3cret-pw"}})

	if err != nil || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d", err, len(res.Received))
	}
}

func TestRun_should_not_put_a_password_in_an_error(t *testing.T) {
	m := startRedis(t)
	m.RequireUserAuth("flowi", "right-pw")

	_, err := redis.New().Run(t.Context(), transport.Operation{
		Server: "redis://flowi:wrong-pw@" + m.Addr(), Channel: "orders", Send: body("x")})

	if err == nil || strings.Contains(err.Error(), "wrong-pw") || strings.Contains(err.Error(), "right-pw") {
		t.Fatalf("err %v, want a failure that does not name the password", err)
	}
}

func TestRun_should_refuse_an_unknown_mode(t *testing.T) {
	m := startRedis(t)

	_, err := redis.New().Run(t.Context(), transport.Operation{
		Server: server(m), Channel: "orders", Send: body("x"), Binding: map[string]string{"mode": "queue"}})

	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("err %v, want one naming mode", err)
	}
}

func TestRun_should_use_the_tls_settings_for_a_rediss_server(t *testing.T) {
	cert, pool := selfSigned(t)
	m, err := miniredis.RunTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	publishWhenSubscribed(t, m, "orders", "secure")

	res, err := redis.New().Run(t.Context(), transport.Operation{
		Server: "rediss://" + m.Addr(), Channel: "orders", Receive: recv(1, 5*time.Second),
		TLS: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})

	if err != nil || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d", err, len(res.Received))
	}
}

func addEntry(t *testing.T, m *miniredis.Miniredis, stream, body string) string {
	t.Helper()
	id, err := m.XAdd(stream, "*", []string{"body", body})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func headerOf(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

// selfSigned makes a certificate for 127.0.0.1 and a pool that trusts it.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "flowi-test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}
