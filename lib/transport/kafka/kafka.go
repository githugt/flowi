// SPDX-License-Identifier: Apache-2.0

// Package kafka is the Kafka transport.
package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/githugt/flowi/lib/transport"
)

const (
	dialTimeout     = 10 * time.Second
	deliveryTimeout = 15 * time.Second
)

// Transport runs Kafka operations.
type Transport struct{}

// New returns the Kafka transport.
func New() Transport { return Transport{} }

// settings are the binding options, checked before anything connects.
type settings struct {
	key       string
	partition *int32
	group     string
	earliest  bool
	mechanism sasl.Mechanism
}

// Run produces to the topic (channel) and reads from it. Binding names: key,
// partition (produce there and read only that partition), group (join a
// consumer group and commit what was read; without one nothing is committed),
// offset (latest, the default, or earliest: where a read without committed
// offsets starts), sasl.mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512),
// username, password.
//
// Without a group the read starts at the end of every partition as it was
// before the produce, so a reply to the message is not missed and records
// that were already there are not returned. The run's own record is skipped.
//
// Goroutine ownership: Run starts one polling goroutine when it reads, stops
// it by cancelling its context, and waits for it before closing the client.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	brokers, useTLS, err := parseServer(op.Server)
	if err != nil {
		return transport.Result{}, err
	}
	cfg, err := parseBinding(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	var reader *poller
	if op.Receive != nil {
		reader = &poller{topic: op.Channel, cfg: cfg}
	}
	cl, err := newClient(brokers, useTLS, op, cfg, reader)
	if err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %w", op.Server, err)
	}
	defer cl.Close()
	res := transport.Result{}
	if reader != nil {
		if err := reader.start(ctx, cl); err != nil {
			return res, err
		}
		defer reader.stop()
	}
	res.Connect = op.Now().Sub(started)
	var own atomic.Pointer[position]
	if op.Send != nil {
		if res.Sent, err = produce(ctx, cl, op, cfg, &own); err != nil {
			return res, err
		}
	}
	if reader != nil {
		r := *op.Receive
		r.Match = skipOwn(r.Match, &own)
		res.Received, err = transport.Wait(ctx, r, reader.messages, reader.failed)
		if err == nil || errors.Is(err, transport.ErrTimeout) || errors.Is(err, transport.ErrTooLarge) {
			reader.commit(ctx, res.Received, err == nil)
		}
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

// position identifies a record.
type position struct {
	partition int32
	offset    int64
}

// skipOwn drops the record this run produced; it runs after the produce, so
// the position is known by then.
func skipOwn(next func(transport.Message) (bool, error), own *atomic.Pointer[position]) func(transport.Message) (bool, error) {
	return func(m transport.Message) (bool, error) {
		if p := own.Load(); p != nil && p.partition == m.Partition && p.offset == m.Offset {
			return false, nil
		}
		if next == nil {
			return true, nil
		}
		return next(m)
	}
}

// parseServer reads kafka://h1:9092,h2:9092 (kafkas:// for TLS).
func parseServer(server string) (brokers []string, useTLS bool, err error) {
	scheme, rest, found := strings.Cut(server, "://")
	if !found {
		scheme, rest = "kafka", server // "localhost:9092"
	}
	switch strings.ToLower(scheme) {
	case "kafka":
	case "kafkas":
		useTLS = true
	default:
		return nil, false, fmt.Errorf("kafka cannot use the scheme %q (use kafka or kafkas)", scheme)
	}
	hosts, _, _ := strings.Cut(rest, "/")
	for _, h := range strings.Split(hosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			brokers = append(brokers, h)
		}
	}
	if len(brokers) == 0 {
		return nil, false, errors.New("the server names no broker")
	}
	return brokers, useTLS, nil
}

func parseBinding(b map[string]string) (settings, error) {
	s := settings{key: b["key"], group: b["group"]}
	if v := b["partition"]; v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			return s, fmt.Errorf("binding partition must be a number from 0, got %q", v)
		}
		p := int32(n)
		s.partition = &p
	}
	switch v := b["offset"]; v {
	case "", "latest":
	case "earliest":
		s.earliest = true
	default:
		return s, fmt.Errorf("binding offset must be latest or earliest, got %q", v)
	}
	var err error
	s.mechanism, err = mechanism(b)
	return s, err
}

func mechanism(b map[string]string) (sasl.Mechanism, error) {
	name := strings.ToUpper(b["sasl.mechanism"])
	if name == "" && b["username"] == "" {
		return nil, nil
	}
	if name == "" {
		name = "PLAIN"
	}
	user, pass := b["username"], b["password"]
	switch name {
	case "PLAIN":
		return plain.Auth{User: user, Pass: pass}.AsMechanism(), nil
	case "SCRAM-SHA-256":
		return scram.Auth{User: user, Pass: pass}.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return scram.Auth{User: user, Pass: pass}.AsSha512Mechanism(), nil
	}
	return nil, fmt.Errorf("binding sasl.mechanism %q is not supported (use PLAIN, SCRAM-SHA-256 or SCRAM-SHA-512)", name)
}

func newClient(brokers []string, useTLS bool, op transport.Operation, cfg settings, r *poller) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...), kgo.ClientID("flowi"), kgo.DialTimeout(dialTimeout),
		kgo.RecordDeliveryTimeout(deliveryTimeout), kgo.RetryTimeout(deliveryTimeout),
	}
	if useTLS {
		tlsCfg := op.TLS
		if tlsCfg == nil {
			tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}
	if op.Send != nil {
		// Like kcat and the Kafka console producer: a send to a topic that does
		// not exist creates it when the broker allows that.
		opts = append(opts, kgo.AllowAutoTopicCreation())
	}
	if cfg.mechanism != nil {
		opts = append(opts, kgo.SASL(cfg.mechanism))
	}
	if cfg.partition != nil {
		opts = append(opts, kgo.RecordPartitioner(kgo.ManualPartitioner()))
	}
	if r != nil && cfg.group != "" {
		opts = append(opts, kgo.ConsumerGroup(cfg.group), kgo.ConsumeTopics(r.topic), kgo.DisableAutoCommit(),
			kgo.ConsumeResetOffset(resetOffset(cfg)))
	}
	return kgo.NewClient(opts...)
}

func resetOffset(cfg settings) kgo.Offset {
	if cfg.earliest {
		return kgo.NewOffset().AtStart()
	}
	return kgo.NewOffset().AtEnd()
}

func produce(ctx context.Context, cl *kgo.Client, op transport.Operation, cfg settings, own *atomic.Pointer[position]) (*transport.Message, error) {
	rec := &kgo.Record{Topic: op.Channel, Value: op.Send.Body}
	if cfg.key != "" {
		rec.Key = []byte(cfg.key)
	}
	if cfg.partition != nil {
		rec.Partition = *cfg.partition
	}
	for _, h := range op.Send.Headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: h.Name, Value: []byte(h.Value)})
	}
	if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return nil, fmt.Errorf("producing to %s: %w", op.Channel, err)
	}
	own.Store(&position{partition: rec.Partition, offset: rec.Offset})
	m := *op.Send
	m.Direction, m.At, m.Key, m.Partition, m.Offset = "sent", op.Now(), cfg.key, rec.Partition, rec.Offset
	return &m, nil
}

// poller reads records into messages.
type poller struct {
	topic    string
	cfg      settings
	messages chan transport.Message
	failed   chan error
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	cl       *kgo.Client
	read     []*kgo.Record // the records handed over, for the group commit
	mu       sync.Mutex
}

// start positions the consumer and begins polling.
func (p *poller) start(ctx context.Context, cl *kgo.Client) error {
	p.cl = cl
	p.messages, p.failed = make(chan transport.Message), make(chan error, 1)
	if p.cfg.group == "" {
		starts, err := p.startOffsets(ctx, cl)
		if err != nil {
			return err
		}
		cl.AddConsumePartitions(starts)
	}
	pctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.poll(pctx)
	}()
	return nil
}

// startOffsets is where to read each partition from, decided before the
// produce: the end (or the start with offset=earliest).
func (p *poller) startOffsets(ctx context.Context, cl *kgo.Client) (map[string]map[int32]kgo.Offset, error) {
	adm := kadm.NewClient(cl)
	list := adm.ListEndOffsets
	if p.cfg.earliest {
		list = adm.ListStartOffsets
	}
	listed, err := list(ctx, p.topic)
	if err == nil {
		err = listed.Error()
	}
	if err != nil {
		return nil, fmt.Errorf("finding where to read %s: %w", p.topic, err)
	}
	offsets := map[int32]kgo.Offset{}
	listed.Each(func(o kadm.ListedOffset) {
		if p.cfg.partition == nil || *p.cfg.partition == o.Partition {
			offsets[o.Partition] = kgo.NewOffset().At(o.Offset)
		}
	})
	if len(offsets) == 0 {
		return nil, fmt.Errorf("topic %s has no matching partition", p.topic)
	}
	return map[string]map[int32]kgo.Offset{p.topic: offsets}, nil
}

func (p *poller) poll(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := p.cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			select {
			case p.failed <- fmt.Errorf("reading %s: %w", errs[0].Topic, errs[0].Err):
			default:
			}
			return
		}
		for _, r := range fetches.Records() {
			// Noted before it is handed over: once the run has what it wants it commits at once, and a
			// record noted only after the hand-over could be missing then, so its offset would never be
			// committed. A record noted but never handed over is harmless: commit keeps only the
			// records the run kept.
			p.mu.Lock()
			p.read = append(p.read, r)
			p.mu.Unlock()
			select {
			case p.messages <- received(r):
			case <-ctx.Done():
				return
			}
		}
	}
}

func received(r *kgo.Record) transport.Message {
	m := transport.Message{Direction: "received", At: r.Timestamp, Body: r.Value, Key: string(r.Key), Partition: r.Partition, Offset: r.Offset,
		Headers: []transport.Header{{Name: "topic", Value: r.Topic}}}
	for _, h := range r.Headers {
		m.Headers = append(m.Headers, transport.Header{Name: h.Key, Value: string(h.Value)})
	}
	return m
}

// commit records progress for a consumer group: the records the run kept
// (a run that failed commits nothing, so they are read again).
func (p *poller) commit(ctx context.Context, kept []transport.Message, ok bool) {
	if p.cfg.group == "" || !ok || len(kept) == 0 {
		return
	}
	p.mu.Lock()
	var recs []*kgo.Record
	for _, r := range p.read {
		for _, m := range kept {
			if m.Partition == r.Partition && m.Offset == r.Offset {
				recs = append(recs, r)
			}
		}
	}
	p.mu.Unlock()
	if len(recs) > 0 {
		_ = p.cl.CommitRecords(ctx, recs...) // a failed commit only means the next run reads them again
	}
}

func (p *poller) stop() {
	p.cancel()
	p.wg.Wait()
}
