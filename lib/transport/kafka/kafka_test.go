// SPDX-License-Identifier: Apache-2.0

package kafka_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/kafka"
)

// cluster starts an in-memory Kafka with the given single-partition topics
// and returns its server URL.
func cluster(t *testing.T, opts ...kfake.Opt) string {
	t.Helper()
	c, err := kfake.NewCluster(append([]kfake.Opt{kfake.NumBrokers(1), kfake.SeedTopics(1, "orders", "ask")}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return "kafka://" + strings.Join(c.ListenAddrs(), ",")
}

func client(t *testing.T, server string, opts ...kgo.Opt) *kgo.Client {
	t.Helper()
	cl, err := kgo.NewClient(append([]kgo.Opt{kgo.SeedBrokers(strings.Split(strings.TrimPrefix(server, "kafka://"), ",")...)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)
	return cl
}

func produce(t *testing.T, server, topic string, values ...string) {
	t.Helper()
	cl := client(t, server)
	for _, v := range values {
		if err := cl.ProduceSync(t.Context(), &kgo.Record{Topic: topic, Value: []byte(v)}).FirstErr(); err != nil {
			t.Fatal(err)
		}
	}
}

func bodies(ms []transport.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.Body))
	}
	return out
}

func TestRun_should_produce_with_key_and_headers_and_report_where_it_landed(t *testing.T) {
	server := cluster(t)

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Binding: map[string]string{"key": "k1"},
		Send: &transport.Message{Body: []byte("one"), Headers: []transport.Header{{Name: "type", Value: "order"}}},
	})

	if err != nil {
		t.Fatal(err)
	}
	if res.Sent == nil || res.Sent.Key != "k1" || res.Sent.Partition != 0 || res.Sent.Offset != 0 {
		t.Fatalf("sent %+v", res.Sent)
	}
	cl := client(t, server, kgo.ConsumeTopics("orders"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	fetches := cl.PollFetches(t.Context())
	recs := fetches.Records()
	if len(recs) != 1 || string(recs[0].Key) != "k1" || string(recs[0].Value) != "one" || string(recs[0].Headers[0].Key) != "type" {
		t.Fatalf("topic holds %+v", recs)
	}
}

func TestRun_should_read_from_the_start_when_asked(t *testing.T) {
	server := cluster(t)
	produce(t, server, "orders", "m1", "m2", "m3")

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Binding: map[string]string{"offset": "earliest"},
		Receive: &transport.Receive{Count: 2, Timeout: 10 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("received %v", got)
	}
	if res.Received[1].Offset != 1 || res.Received[0].Partition != 0 {
		t.Fatalf("position %+v", res.Received[1])
	}
}

func TestRun_should_skip_what_was_already_there_by_default(t *testing.T) {
	server := cluster(t)
	produce(t, server, "orders", "old")

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Receive: &transport.Receive{Timeout: 300 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 0 {
		t.Fatalf("err %v received %v", err, bodies(res.Received))
	}
}

func TestRun_should_get_the_reply_but_not_its_own_record(t *testing.T) {
	server := cluster(t)
	responder := client(t, server, kgo.ConsumeTopics("ask"), kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	go func() {
		for ctx.Err() == nil {
			for _, r := range responder.PollFetches(ctx).Records() {
				if string(r.Value) == "question" {
					responder.Produce(ctx, &kgo.Record{Topic: "ask", Value: []byte("answer")}, nil)
				}
			}
		}
	}()

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "ask", Send: &transport.Message{Body: []byte("question")},
		Receive: &transport.Receive{Timeout: 10 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); len(got) != 1 || got[0] != "answer" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_advance_a_consumer_group_between_runs(t *testing.T) {
	server := cluster(t)
	produce(t, server, "orders", "m1", "m2", "m3")
	op := transport.Operation{
		Server: server, Channel: "orders", Binding: map[string]string{"group": "g1", "offset": "earliest"},
		Receive: &transport.Receive{Timeout: 20 * time.Second},
	}

	first, err1 := kafka.New().Run(t.Context(), op)
	second, err2 := kafka.New().Run(t.Context(), op)

	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if a, b := bodies(first.Received), bodies(second.Received); len(a) != 1 || a[0] != "m1" || len(b) != 1 || b[0] != "m2" {
		t.Fatalf("first %v second %v", a, b)
	}
}

func TestRun_should_time_out_with_the_partial_messages(t *testing.T) {
	server := cluster(t)
	produce(t, server, "orders", "only")

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Binding: map[string]string{"offset": "earliest"},
		Receive: &transport.Receive{Count: 2, Timeout: 500 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_should_authenticate_with_sasl_plain(t *testing.T) {
	server := cluster(t, kfake.EnableSASL(), kfake.Superuser("PLAIN", "u", "p"))

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Send: &transport.Message{Body: []byte("x")},
		Binding: map[string]string{"sasl.mechanism": "PLAIN", "username": "u", "password": "p"},
	})

	if err != nil || res.Sent == nil {
		t.Fatalf("err %v sent %+v", err, res.Sent)
	}
}

func TestRun_should_reject_options_it_cannot_use(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		binding map[string]string
		want    string
	}{
		{"scheme", "http://h:9092", nil, "scheme"},
		{"partition", "kafka://127.0.0.1:1", map[string]string{"partition": "x"}, "partition"},
		{"offset", "kafka://127.0.0.1:1", map[string]string{"offset": "soon"}, "offset"},
		{"mechanism", "kafka://127.0.0.1:1", map[string]string{"sasl.mechanism": "KERBEROS", "username": "u"}, "mechanism"},
		{"no_brokers", "kafka://", nil, "broker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kafka.New().Run(t.Context(), transport.Operation{
				Server: tt.server, Channel: "t", Binding: tt.binding, Send: &transport.Message{Body: []byte("x")},
			})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestRun_should_accept_brokers_written_without_a_scheme(t *testing.T) {
	server := strings.TrimPrefix(cluster(t), "kafka://")

	res, err := kafka.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders", Send: &transport.Message{Body: []byte("x")},
	})

	if err != nil || res.Sent == nil {
		t.Fatalf("err %v sent %+v", err, res.Sent)
	}
}
