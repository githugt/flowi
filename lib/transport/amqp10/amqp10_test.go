// SPDX-License-Identifier: Apache-2.0

package amqp10

import (
	"slices"
	"strings"
	"testing"
	"time"

	amqp "github.com/Azure/go-amqp"

	"github.com/githugt/flowi/lib/transport"
)

func TestTargetOf_should_read_the_server_channel_and_credentials(t *testing.T) {
	// Arrange
	op := transport.Operation{Server: "amqps://user:secret@broker.example:5671", Channel: "orders", Binding: map[string]string{"password": "other"}}

	// Act
	got, err := targetOf(op)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got.addr != "amqps://broker.example:5671" || got.address != "orders" || got.user != "user" || got.password != "other" || !got.tls {
		t.Errorf("target %+v (the binding wins over the URL)", got)
	}
	if strings.Contains(got.addr, "secret") {
		t.Error("the dialled address must carry no credentials")
	}
}

func TestTargetOf_should_default_the_port_by_scheme(t *testing.T) {
	// Act
	plain, _ := targetOf(transport.Operation{Server: "amqp://h", Channel: "q"})
	secure, _ := targetOf(transport.Operation{Server: "amqps://h", Channel: "q"})

	// Assert
	if plain.addr != "amqp://h:5672" || secure.addr != "amqps://h:5671" {
		t.Errorf("plain %q secure %q", plain.addr, secure.addr)
	}
}

func TestTargetOf_should_refuse_a_wrong_scheme_and_a_missing_address(t *testing.T) {
	cases := map[string]transport.Operation{
		"scheme":  {Server: "mqtt://h", Channel: "q"},
		"channel": {Server: "amqp://h"},
	}
	for name, op := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := targetOf(op)

			// Assert
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestTarget_redact_should_hide_the_password(t *testing.T) {
	// Act
	got := target{password: "hunter2"}.redact("dial amqp://u:hunter2@h failed")

	// Assert
	if strings.Contains(got, "hunter2") || !strings.Contains(got, "***") {
		t.Errorf("got %q", got)
	}
}

func TestSendProps_should_read_the_binding_and_refuse_bad_values(t *testing.T) {
	// Act
	good, goodErr := sendProps(transport.Operation{Binding: map[string]string{"subject": "s", "content_type": "application/json", "durable": "true", "ttl": "1500"}})
	_, badDurable := sendProps(transport.Operation{Binding: map[string]string{"durable": "maybe"}})
	_, badTTL := sendProps(transport.Operation{Binding: map[string]string{"ttl": "soon"}})

	// Assert
	if goodErr != nil || good.subject != "s" || good.contentType != "application/json" || !good.durable || good.ttl != 1500*time.Millisecond {
		t.Errorf("good %+v err %v", good, goodErr)
	}
	if badDurable == nil || badTTL == nil {
		t.Errorf("bad values accepted: %v %v", badDurable, badTTL)
	}
}

func TestRun_message_should_carry_the_body_headers_and_the_runs_id(t *testing.T) {
	// Arrange
	r := &run{id: "flowi-1", props: sendProperties{subject: "hello", durable: true, ttl: time.Second},
		op: transport.Operation{Send: &transport.Message{Body: []byte("body"), Headers: []transport.Header{{Name: "type", Value: "order"}}}}}

	// Act
	m := r.message()

	// Assert
	if string(m.Data[0]) != "body" || m.ApplicationProperties["type"] != "order" || m.Properties.MessageID != "flowi-1" {
		t.Errorf("message %+v", m)
	}
	if *m.Properties.Subject != "hello" || m.Header == nil || !m.Header.Durable || m.Header.TTL != time.Second {
		t.Errorf("properties %+v header %+v", m.Properties, m.Header)
	}
	if !r.isOwn(&amqp.Message{Properties: &amqp.MessageProperties{MessageID: "flowi-1"}}) || r.isOwn(&amqp.Message{Properties: &amqp.MessageProperties{MessageID: "other"}}) {
		t.Error("only the run's own message id is its own")
	}
}

func TestRun_received_should_turn_properties_into_headers_in_a_fixed_order(t *testing.T) {
	// Arrange
	subject, ctype := "greeting", "text/plain"
	m := &amqp.Message{
		Properties:            &amqp.MessageProperties{MessageID: "m-1", Subject: &subject, ContentType: &ctype},
		ApplicationProperties: map[string]any{"b": "2", "a": 1},
		Data:                  [][]byte{[]byte("hel"), []byte("lo")},
	}
	r := &run{op: transport.Operation{Clock: func() time.Time { return time.Unix(0, 0) }}}

	// Act
	got := r.received(m)

	// Assert
	if string(got.Body) != "hello" || got.Direction != "received" {
		t.Errorf("message %+v", got)
	}
	var names []string
	for _, h := range got.Headers {
		names = append(names, h.Name+"="+h.Value)
	}
	want := []string{"message-id=m-1", "subject=greeting", "content-type=text/plain", "a=1", "b=2"}
	if !slices.Equal(names, want) {
		t.Errorf("headers %v, want %v", names, want)
	}
}

func TestBodyOf_should_read_data_value_and_sequence_bodies(t *testing.T) {
	cases := map[string]struct {
		m    *amqp.Message
		want string
	}{
		"data":     {&amqp.Message{Data: [][]byte{[]byte("a"), []byte("b")}}, "ab"},
		"string":   {&amqp.Message{Value: "text"}, "text"},
		"bytes":    {&amqp.Message{Value: []byte("raw")}, "raw"},
		"map":      {&amqp.Message{Value: map[string]any{"k": 1}}, `{"k":1}`},
		"sequence": {&amqp.Message{Sequence: [][]any{{1, "x"}}}, `[[1,"x"]]`},
		"empty":    {&amqp.Message{}, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := string(bodyOf(c.m))

			// Assert
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
