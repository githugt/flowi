// SPDX-License-Identifier: Apache-2.0

package pull_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/internal/pull"
)

func msg(id string) transport.Message {
	return transport.Message{Body: []byte(id), Headers: []transport.Header{{Name: "message_id", Value: id}}}
}

func idOf(m transport.Message) string { return string(m.Body) }

// feed hands the messages to the collector, then waits for the run to end.
func feed(ids ...string) func(context.Context, *pull.Book[string], chan<- transport.Message) error {
	return func(ctx context.Context, book *pull.Book[string], in chan<- transport.Message) error {
		for _, id := range ids {
			book.Hold(id, "h-"+id)
			select {
			case in <- msg(id):
			case <-ctx.Done():
				return nil
			}
		}
		<-ctx.Done()
		return nil
	}
}

type settled struct{ kept, others []string }

func recorder(out *settled) func(context.Context, []string, []string) error {
	return func(_ context.Context, kept, others []string) error {
		out.kept, out.others = kept, others
		return nil
	}
}

func TestCollect_should_settle_the_kept_handles_apart_from_the_rest(t *testing.T) {
	var got settled
	rc := transport.Receive{Count: 1, Timeout: 5 * time.Second, Match: func(m transport.Message) (bool, error) { return idOf(m) == "b", nil }}

	kept, err := pull.Collect(t.Context(), rc, idOf, feed("a", "b", "c"), recorder(&got))

	if err != nil || len(kept) != 1 || idOf(kept[0]) != "b" {
		t.Fatalf("err %v, kept %+v", err, kept)
	}
	slices.Sort(got.others)
	if !slices.Equal(got.kept, []string{"h-b"}) || !slices.Equal(got.others, []string{"h-a", "h-c"}) {
		t.Fatalf("settled %+v, want kept [h-b] and others [h-a h-c]", got)
	}
}

func TestCollect_should_settle_what_was_kept_when_the_receive_times_out(t *testing.T) {
	var got settled
	rc := transport.Receive{Count: 2, Timeout: 100 * time.Millisecond}

	kept, err := pull.Collect(t.Context(), rc, idOf, feed("a"), recorder(&got))

	if !errors.Is(err, transport.ErrTimeout) || len(kept) != 1 || !slices.Equal(got.kept, []string{"h-a"}) {
		t.Fatalf("err %v, kept %d, settled %+v", err, len(kept), got)
	}
}

func TestCollect_should_report_a_failed_pump(t *testing.T) {
	var got settled
	pump := func(context.Context, *pull.Book[string], chan<- transport.Message) error {
		return errors.New("pull failed")
	}

	_, err := pull.Collect(t.Context(), transport.Receive{Timeout: 5 * time.Second}, idOf, pump, recorder(&got))

	if err == nil || !strings.Contains(err.Error(), "pull failed") {
		t.Fatalf("err %v, want the pump's error", err)
	}
}

func TestCollect_should_report_a_failed_settle_when_nothing_else_failed(t *testing.T) {
	rc := transport.Receive{Count: 1, Timeout: 5 * time.Second}
	settle := func(context.Context, []string, []string) error { return errors.New("ack failed") }

	kept, err := pull.Collect(t.Context(), rc, idOf, feed("a"), settle)

	if err == nil || !strings.Contains(err.Error(), "ack failed") || len(kept) != 1 {
		t.Fatalf("err %v, kept %d, want the settle error with the message still returned", err, len(kept))
	}
}

func TestCollect_should_still_settle_when_the_context_has_ended(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var live *bool
	settle := func(c context.Context, _, _ []string) error { ok := c.Err() == nil; live = &ok; return nil }

	_, _ = pull.Collect(ctx, transport.Receive{Timeout: time.Second}, idOf, feed("a"), settle)

	if live == nil || !*live {
		t.Fatalf("settle must run with a live context of its own (ran: %v)", live != nil)
	}
}
