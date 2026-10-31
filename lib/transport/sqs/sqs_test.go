// SPDX-License-Identifier: Apache-2.0

package sqs_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/sqs"
)

const secretKey = "wJalrXUtnFEMI-s3cret-key"

// operation targets the fake with explicit test credentials.
func operation(f *fakeSQS, channel string, extra map[string]string) transport.Operation {
	binding := map[string]string{"auth": "explicit", "access_key_id": "AKIDEXPLICIT", "secret_access_key": secretKey, "region": "us-east-1", "wait": "1"}
	for k, v := range extra {
		binding[k] = v
	}
	return transport.Operation{Server: f.URL, Channel: channel, Binding: binding}
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

func TestRun_should_send_the_body_with_headers_as_message_attributes(t *testing.T) {
	f := startFakeSQS(t, "orders.fifo")
	op := operation(f, "orders.fifo", map[string]string{"group_id": "g1", "dedup_id": "d1", "delay": "5"})
	op.Send = &transport.Message{Body: []byte("payload"), Headers: []transport.Header{{Name: "type", Value: "order"}}}

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	got := f.snapshot("orders.fifo")
	if len(got) != 1 || got[0].body != "payload" || got[0].attrs["type"] != "order" || got[0].group != "g1" || got[0].dedup != "d1" || got[0].delay != 5 {
		t.Fatalf("queue holds %+v", got)
	}
	if res.Sent == nil || headerOf(*res.Sent, "message_id") != got[0].id {
		t.Fatalf("sent %+v, want message_id %s", res.Sent, got[0].id)
	}
}

func TestRun_should_look_a_queue_name_up_and_use_a_queue_url_as_given(t *testing.T) {
	f := startFakeSQS(t, "orders")
	byName := operation(f, "orders", nil)
	byName.Send = &transport.Message{Body: []byte("a")}
	byURL := operation(f, f.queueURL("orders"), nil)
	byURL.Send = &transport.Message{Body: []byte("b")}

	if _, err := sqs.New().Run(t.Context(), byName); err != nil {
		t.Fatal(err)
	}
	afterName := f.callList()
	if _, err := sqs.New().Run(t.Context(), byURL); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(afterName, []string{"GetQueueUrl", "SendMessage"}) {
		t.Fatalf("by name the calls were %v", afterName)
	}
	if rest := f.callList()[len(afterName):]; !slices.Equal(rest, []string{"SendMessage"}) {
		t.Fatalf("by url the calls were %v, want no GetQueueUrl", rest)
	}
}

func TestRun_should_delete_the_messages_kept_and_leave_the_unneeded_one_in_the_queue(t *testing.T) {
	f := startFakeSQS(t, "orders")
	f.put("orders", "one")
	f.put("orders", "two")
	f.put("orders", "three")
	op := operation(f, "orders", nil)
	op.Receive = recv(2, 10*time.Second)

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 2 {
		t.Fatalf("err %v, received %d, want 2", err, len(res.Received))
	}
	left := f.snapshot("orders")
	if len(left) != 1 || left[0].body != "three" || !f.visible(left[0]) {
		t.Fatalf("queue holds %+v, want only a visible \"three\"", left)
	}
}

func TestRun_should_return_a_skipped_message_to_the_queue_at_once_and_delete_the_kept_one(t *testing.T) {
	f := startFakeSQS(t, "orders")
	f.put("orders", "skip me")
	f.put("orders", "keep me")
	op := operation(f, "orders", nil)
	op.Receive = recv(1, 10*time.Second)
	op.Receive.Match = func(m transport.Message) (bool, error) { return strings.HasPrefix(string(m.Body), "keep"), nil }

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 1 || string(res.Received[0].Body) != "keep me" {
		t.Fatalf("err %v, received %+v", err, res.Received)
	}
	left := f.snapshot("orders")
	if len(left) != 1 || left[0].body != "skip me" || !f.visible(left[0]) {
		t.Fatalf("queue holds %+v, want only the skipped message, visible again", left)
	}
}

func TestRun_should_leave_the_messages_in_the_queue_when_delete_is_false(t *testing.T) {
	f := startFakeSQS(t, "orders")
	f.put("orders", "one")
	op := operation(f, "orders", map[string]string{"delete": "false"})
	op.Receive = recv(1, 10*time.Second)

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d", err, len(res.Received))
	}
	left := f.snapshot("orders")
	if len(left) != 1 || !f.visible(left[0]) {
		t.Fatalf("queue holds %+v, want the message visible again", left)
	}
}

func TestRun_should_skip_its_own_message_and_return_the_reply(t *testing.T) {
	f := startFakeSQS(t, "orders")
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for len(f.snapshot("orders")) == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		f.put("orders", "reply")
	}()
	op := operation(f, "orders", nil)
	op.Send = &transport.Message{Body: []byte("request")}
	op.Receive = recv(1, 10*time.Second)

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil || len(res.Received) != 1 || string(res.Received[0].Body) != "reply" {
		t.Fatalf("err %v, received %+v, want only the reply", err, res.Received)
	}
	left := f.snapshot("orders")
	if len(left) != 1 || left[0].body != "request" {
		t.Fatalf("queue holds %+v, want the request still there for its consumer", left)
	}
}

func TestRun_should_carry_the_attributes_of_a_message_as_headers(t *testing.T) {
	f := startFakeSQS(t, "orders")
	m := f.put("orders", "x")
	m.attrs["kind"] = "a"
	op := operation(f, "orders", nil)
	op.Receive = recv(1, 10*time.Second)

	res, err := sqs.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	got := res.Received[0]
	if headerOf(got, "kind") != "a" || headerOf(got, "message_id") != m.id || headerOf(got, "receive_count") != "1" {
		t.Fatalf("headers %+v", got.Headers)
	}
}

func TestRun_should_return_what_arrived_with_a_timeout(t *testing.T) {
	f := startFakeSQS(t, "orders")
	f.put("orders", "only one")
	op := operation(f, "orders", nil)
	op.Receive = recv(2, 600*time.Millisecond)

	res, err := sqs.New().Run(t.Context(), op)

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v, received %d, want ErrTimeout and 1", err, len(res.Received))
	}
}

func TestRun_should_return_promptly_when_the_context_ends(t *testing.T) {
	f := startFakeSQS(t, "orders")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	started := time.Now()
	op := operation(f, "orders", nil)
	op.Receive = recv(1, time.Minute)

	_, err := sqs.New().Run(ctx, op)

	if err == nil || time.Since(started) > 5*time.Second {
		t.Fatalf("err %v after %v, want an error well before the receive timeout", err, time.Since(started))
	}
}

func TestRun_should_sign_with_the_explicit_credentials(t *testing.T) {
	f := startFakeSQS(t, "orders")
	op := operation(f, "orders", nil)
	op.Send = &transport.Message{Body: []byte("x")}

	if _, err := sqs.New().Run(t.Context(), op); err != nil {
		t.Fatal(err)
	}

	if keys := f.usedKeys(); len(keys) == 0 || keys[0] != "AKIDEXPLICIT" {
		t.Fatalf("requests were signed with %v, want AKIDEXPLICIT", keys)
	}
}

func TestRun_should_name_the_missing_credential_when_auth_is_explicit(t *testing.T) {
	f := startFakeSQS(t, "orders")
	op := operation(f, "orders", map[string]string{"secret_access_key": ""})
	op.Send = &transport.Message{Body: []byte("x")}

	_, err := sqs.New().Run(t.Context(), op)

	if err == nil || !strings.Contains(err.Error(), "secret_access_key") {
		t.Fatalf("err %v, want one naming secret_access_key", err)
	}
}

func TestRun_should_use_the_provider_login_and_ignore_binding_credentials_when_auth_is_default(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDFROMENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/none")
	f := startFakeSQS(t, "orders")
	op := operation(f, "orders", map[string]string{"auth": "default"}) // the binding still holds AKIDEXPLICIT
	op.Send = &transport.Message{Body: []byte("x")}

	if _, err := sqs.New().Run(t.Context(), op); err != nil {
		t.Fatal(err)
	}

	if keys := f.usedKeys(); len(keys) == 0 || keys[0] != "AKIDFROMENV" {
		t.Fatalf("requests were signed with %v, want AKIDFROMENV", keys)
	}
}

func TestRun_should_not_put_the_secret_key_in_an_error(t *testing.T) {
	op := transport.Operation{Server: "http://127.0.0.1:1", Channel: "orders", Send: &transport.Message{Body: []byte("x")},
		Binding: map[string]string{"auth": "explicit", "access_key_id": "AKIDEXPLICIT", "secret_access_key": secretKey, "region": "us-east-1"}}

	_, err := sqs.New().Run(t.Context(), op)

	if err == nil || strings.Contains(err.Error(), secretKey) {
		t.Fatalf("err %v, want a failure that does not contain the secret key", err)
	}
}
