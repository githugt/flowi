// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/githugt/flowi/lib/transport"
)

const (
	bodyField     = "body"
	pollBlock     = 200 * time.Millisecond // how long one read waits; bounds how soon Run notices the end of the run
	defaultWorker = "flowi"
)

// streamConfig is what the binding says about a stream read.
type streamConfig struct {
	key, start, group, consumer string
}

func streamConfigOf(op transport.Operation) streamConfig {
	c := streamConfig{key: op.Channel, start: op.Binding["start"], group: op.Binding["group"], consumer: op.Binding["consumer"]}
	if c.start == "" {
		c.start = "$"
	}
	if c.consumer == "" {
		c.consumer = defaultWorker
	}
	return c
}

// runStream prepares the read (so the start point is fixed before the entry
// is added), adds the entry, then reads.
func runStream(ctx context.Context, client *goredis.Client, op transport.Operation, res *transport.Result) error {
	cfg := streamConfigOf(op)
	var cursor string
	if op.Receive != nil {
		var err error
		if cursor, err = prepareRead(ctx, client, cfg); err != nil {
			return err
		}
	}
	var ownID string
	if op.Send != nil {
		id, err := addEntry(ctx, client, op)
		if err != nil {
			return err
		}
		ownID = id
		res.Sent = sentMessage(op, []transport.Header{{Name: "id", Value: id}})
	}
	if op.Receive == nil {
		return nil
	}
	return readEntries(ctx, client, op, readPlan{cfg: cfg, cursor: cursor, ownID: ownID}, res)
}

// addEntry appends the message: its body as field "body", each header as a
// field of its own name.
func addEntry(ctx context.Context, client *goredis.Client, op transport.Operation) (string, error) {
	values := []any{bodyField, op.Send.Body} // an ordered list: a map would shuffle the fields
	for _, h := range op.Send.Headers {
		values = append(values, h.Name, h.Value)
	}
	args := &goredis.XAddArgs{Stream: op.Channel, Values: values}
	if raw := op.Binding["maxlen"]; raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return "", fmt.Errorf("binding maxlen %q must be a positive number", raw)
		}
		args.MaxLen, args.Approx = n, true
	}
	id, err := client.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("adding to stream %s: %w", op.Channel, err)
	}
	return id, nil
}

// prepareRead returns the id to read after. With a group the group is created
// when missing and the cursor is unused (">" is read).
func prepareRead(ctx context.Context, client *goredis.Client, cfg streamConfig) (string, error) {
	if cfg.group != "" {
		err := client.XGroupCreateMkStream(ctx, cfg.key, cfg.group, cfg.start).Err()
		if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			return "", fmt.Errorf("creating group %s on %s: %w", cfg.group, cfg.key, err)
		}
		return ">", nil
	}
	if cfg.start != "$" {
		return cfg.start, nil
	}
	// "$" means "entries from now": XREAD repeats it every call, so fix it to
	// the id of the newest entry instead.
	last, err := client.XRevRangeN(ctx, cfg.key, "+", "-", 1).Result()
	if err != nil {
		return "", fmt.Errorf("reading stream %s: %w", cfg.key, err)
	}
	if len(last) == 0 {
		return "0-0", nil
	}
	return last[0].ID, nil
}

type readPlan struct {
	cfg    streamConfig
	cursor string
	ownID  string
}

// readEntries starts the reader goroutine and collects. Kept entries are
// acknowledged (group reads) as they are kept.
func readEntries(ctx context.Context, client *goredis.Client, op transport.Operation, plan readPlan, res *transport.Result) error {
	runCtx, cancel := context.WithCancel(ctx)
	in := make(chan transport.Message)
	fail := make(chan error, 1)
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		if err := pumpEntries(runCtx, client, plan, op.Now, in); err != nil {
			fail <- err
		}
	}()
	defer done.Wait()
	defer cancel()

	var ackErr error
	rc := *op.Receive
	next := rc.OnMessage
	rc.OnMessage = func(m transport.Message) {
		if plan.cfg.group != "" && ackErr == nil {
			ackErr = ackEntry(ctx, client, plan.cfg, headerValue(m, "id"))
		}
		if next != nil {
			next(m)
		}
	}
	var err error
	res.Received, err = transport.Wait(ctx, rc, in, fail)
	if err == nil {
		err = ackErr
	}
	return err
}

func ackEntry(ctx context.Context, client *goredis.Client, cfg streamConfig, id string) error {
	if err := client.XAck(ctx, cfg.key, cfg.group, id).Err(); err != nil {
		return fmt.Errorf("acknowledging %s: %w", id, err)
	}
	return nil
}

// pumpEntries reads until ctx ends and hands each entry (except the one this
// run added) to in. A read waits at most pollBlock so the end of the run is
// noticed; the error returned is a failed read, never the end of the run.
func pumpEntries(ctx context.Context, client *goredis.Client, plan readPlan, now func() time.Time, in chan<- transport.Message) error {
	cursor := plan.cursor
	for ctx.Err() == nil {
		entries, err := readBatch(ctx, client, plan.cfg, cursor)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("reading stream %s: %w", plan.cfg.key, err)
		}
		for _, e := range entries {
			cursor = e.ID
			if e.ID == plan.ownID {
				continue
			}
			select {
			case in <- entryMessage(plan.cfg.key, e, now()):
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

// readBatch reads the next entries; no entries within pollBlock is not an error.
func readBatch(ctx context.Context, client *goredis.Client, cfg streamConfig, cursor string) ([]goredis.XMessage, error) {
	var streams []goredis.XStream
	var err error
	if cfg.group != "" {
		// One at a time: an entry taken but not kept stays pending for the group.
		streams, err = client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group: cfg.group, Consumer: cfg.consumer, Streams: []string{cfg.key, ">"}, Count: 1, Block: pollBlock}).Result()
	} else {
		streams, err = client.XRead(ctx, &goredis.XReadArgs{Streams: []string{cfg.key, cursor}, Count: 10, Block: pollBlock}).Result()
	}
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil || len(streams) == 0 {
		return nil, err
	}
	return streams[0].Messages, nil
}

// entryMessage turns an entry into a message: field "body" is the body, the
// id and stream are headers, and every other field is a header, in name order.
func entryMessage(stream string, e goredis.XMessage, at time.Time) transport.Message {
	m := transport.Message{Direction: "received", At: at,
		Headers: []transport.Header{{Name: "id", Value: e.ID}, {Name: "stream", Value: stream}}}
	names := make([]string, 0, len(e.Values))
	for name := range e.Values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := fmt.Sprint(e.Values[name])
		if name == bodyField {
			m.Body = []byte(value)
			continue
		}
		m.Headers = append(m.Headers, transport.Header{Name: name, Value: value})
	}
	return m
}

func headerValue(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}
