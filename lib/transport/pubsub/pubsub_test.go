// SPDX-License-Identifier: Apache-2.0

package pubsub_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/pubsub"
)

const project = "demo"

// startPubSub runs an in-process Pub/Sub server with a topic and a
// subscription called name (a topic and a subscription may share an id).
func startPubSub(t *testing.T, name string) *pstest.Server {
	t.Helper()
	srv := pstest.NewServer()
	t.Cleanup(func() { _ = srv.Close() })
	ctx := t.Context()
	topic := "projects/" + project + "/topics/" + name
	if _, err := srv.GServer.CreateTopic(ctx, &pubsubpb.Topic{Name: topic}); err != nil {
		t.Fatal(err)
	}
	sub := &pubsubpb.Subscription{Name: "projects/" + project + "/subscriptions/" + name, Topic: topic, AckDeadlineSeconds: 10, EnableMessageOrdering: true}
	if _, err := srv.GServer.CreateSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	return srv
}

func operation(srv *pstest.Server, channel string, extra map[string]string) transport.Operation {
	binding := map[string]string{"endpoint": srv.Addr}
	for k, v := range extra {
		binding[k] = v
	}
	return transport.Operation{Server: "pubsub://" + project, Channel: channel, Binding: binding}
}

func recv(count int, timeout time.Duration) *transport.Receive {
	return &transport.Receive{Count: count, Timeout: timeout}
}

func headerOf(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

func TestRun_should_publish_the_body_with_attributes_and_an_ordering_key(t *testing.T) {
	srv := startPubSub(t, "orders")
	op := operation(srv, "orders", map[string]string{"ordering_key": "customer-7"})
	op.Send = &transport.Message{Body: []byte("payload"), Headers: []transport.Header{{Name: "type", Value: "order"}}}

	res, err := pubsub.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	published := srv.Messages()
	if len(published) != 1 || string(published[0].Data) != "payload" || published[0].Attributes["type"] != "order" || published[0].OrderingKey != "customer-7" {
		t.Fatalf("server holds %+v", published)
	}
	if res.Sent == nil || headerOf(*res.Sent, "message_id") != published[0].ID {
		t.Fatalf("sent %+v, want message_id %s", res.Sent, published[0].ID)
	}
}

func TestRun_should_pull_messages_with_their_ids_and_attributes(t *testing.T) {
	srv := startPubSub(t, "orders")
	srv.Publish("projects/"+project+"/topics/orders", []byte("one"), map[string]string{"kind": "a"})
	srv.Publish("projects/"+project+"/topics/orders", []byte("two"), nil)
	op := operation(srv, "orders", nil)
	op.Receive = recv(2, 10*time.Second)

	res, err := pubsub.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 2 {
		t.Fatalf("received %d, want 2", len(res.Received))
	}
	bodies := map[string]transport.Message{}
	for _, m := range res.Received {
		bodies[string(m.Body)] = m
	}
	if headerOf(bodies["one"], "kind") != "a" || headerOf(bodies["one"], "message_id") == "" || headerOf(bodies["two"], "publish_time") == "" {
		t.Fatalf("headers lost: %+v", res.Received)
	}
}

func TestRun_should_ack_the_messages_kept_and_nack_the_skipped_so_they_can_be_delivered_again(t *testing.T) {
	srv := startPubSub(t, "orders")
	topic := "projects/" + project + "/topics/orders"
	skippedID := srv.Publish(topic, []byte("skip me"), nil)
	keptID := srv.Publish(topic, []byte("keep me"), nil)
	op := operation(srv, "orders", nil)
	op.Receive = recv(1, 10*time.Second)
	op.Receive.Match = func(m transport.Message) (bool, error) { return strings.HasPrefix(string(m.Body), "keep"), nil }

	res, err := pubsub.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 1 || string(res.Received[0].Body) != "keep me" {
		t.Fatalf("err %v, received %+v", err, res.Received)
	}
	if srv.Message(keptID).Acks != 1 {
		t.Fatalf("the kept message was acknowledged %d times, want 1", srv.Message(keptID).Acks)
	}
	if srv.Message(skippedID).Acks != 0 {
		t.Fatalf("the skipped message was acknowledged")
	}
	again := operation(srv, "orders", nil)
	again.Receive = recv(1, 10*time.Second)
	res, err = pubsub.New().Run(t.Context(), again)
	if err != nil || len(res.Received) != 1 || string(res.Received[0].Body) != "skip me" {
		t.Fatalf("second run: err %v, received %+v, want the skipped message again", err, res.Received)
	}
}

func TestRun_should_skip_its_own_message_and_return_the_reply(t *testing.T) {
	srv := startPubSub(t, "orders")
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for len(srv.Messages()) == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		srv.Publish("projects/"+project+"/topics/orders", []byte("reply"), nil)
	}()
	op := operation(srv, "orders", nil)
	op.Send = &transport.Message{Body: []byte("request")}
	op.Receive = recv(1, 10*time.Second)

	res, err := pubsub.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 1 || string(res.Received[0].Body) != "reply" {
		t.Fatalf("err %v, received %+v, want only the reply", err, res.Received)
	}
}

func TestRun_should_return_what_arrived_with_a_timeout(t *testing.T) {
	srv := startPubSub(t, "orders")
	srv.Publish("projects/"+project+"/topics/orders", []byte("only one"), nil)
	op := operation(srv, "orders", nil)
	op.Receive = recv(2, 1500*time.Millisecond)

	res, err := pubsub.New().Run(t.Context(), op)

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d, want ErrTimeout and 1", err, len(res.Received))
	}
}

func TestRun_should_return_promptly_when_the_context_ends(t *testing.T) {
	srv := startPubSub(t, "orders")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(300*time.Millisecond, cancel)
	started := time.Now()
	op := operation(srv, "orders", nil)
	op.Receive = recv(1, time.Minute)

	_, err := pubsub.New().Run(ctx, op)

	if err == nil || time.Since(started) > 10*time.Second {
		t.Fatalf("err %v after %v, want an error well before the receive timeout", err, time.Since(started))
	}
}

func TestRun_should_name_the_binding_when_explicit_credentials_are_missing_or_wrong(t *testing.T) {
	srv := startPubSub(t, "orders")
	tests := []struct {
		name, json, want string
	}{
		{"missing", "", "credentials_json"},
		{"not_json", "s3cret-not-json", "credentials_json"},
		{"wrong_type", `{"type":"external_account","note":"s3cret"}`, "service_account"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := operation(srv, "orders", map[string]string{"auth": "explicit", "credentials_json": tt.json})
			op.Send = &transport.Message{Body: []byte("x")}

			_, err := pubsub.New().Run(t.Context(), op)

			if err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "s3cret") {
				t.Fatalf("err %v, want one naming %q without the secret", err, tt.want)
			}
		})
	}
}

func TestRun_should_ignore_the_credentials_when_the_provider_login_is_used(t *testing.T) {
	srv := startPubSub(t, "orders")
	op := operation(srv, "orders", map[string]string{"auth": "default", "credentials_json": "not even json"})
	op.Send = &transport.Message{Body: []byte("x")}

	_, err := pubsub.New().Run(t.Context(), op)

	if err != nil {
		t.Fatalf("auth default must not read credentials_json: %v", err)
	}
}

func TestRun_should_refuse_a_server_without_a_project(t *testing.T) {
	op := transport.Operation{Server: "pubsub://", Channel: "orders", Send: &transport.Message{Body: []byte("x")}}

	_, err := pubsub.New().Run(t.Context(), op)

	if err == nil || !strings.Contains(err.Error(), "pubsub://") {
		t.Fatalf("err %v, want one that shows the expected server form", err)
	}
}
