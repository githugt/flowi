// SPDX-License-Identifier: Apache-2.0

// Package redis is the Redis transport. Pub/sub mode (the default) publishes
// to and subscribes to a channel; stream mode appends to and reads a stream,
// optionally through a consumer group.
//
// Entries read through a group but not kept (the filter skipped them, or the
// run ended first) stay pending for the group, never acknowledged.
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/githugt/flowi-lib/transport"
)

var (
	errBadServer = errors.New("server must be redis://host:port or rediss://host:port")
	errNoAnswer  = errors.New("no answer from the server")
)

const (
	connectTimeout = 10 * time.Second
	modePubSub     = "pubsub"
	modeStream     = "stream"
)

// Transport runs Redis operations.
type Transport struct{}

// New returns the Redis transport.
func New() Transport { return Transport{} }

// Run connects and then publishes and/or receives. Binding: mode (pubsub or
// stream), pattern (true: PSUBSCRIBE), username and password (they replace
// any in the server URL), and for streams start ($ new entries, 0 all, or an
// entry id), group, consumer and maxlen.
//
// Goroutine ownership: a receive starts one reader goroutine; Run stops it and
// waits for it before it returns, and closes the client last.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	client, addr, err := newClient(op)
	if err != nil {
		return transport.Result{}, err
	}
	defer client.Close() //nolint:errcheck // nothing useful to do if closing fails
	if err := ping(ctx, client); err != nil {
		return transport.Result{}, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	switch mode := modeOf(op); mode {
	case modePubSub:
		err = runPubSub(ctx, client, op, &res)
	case modeStream:
		err = runStream(ctx, client, op, &res)
	default:
		err = fmt.Errorf("binding mode %q must be %s or %s", mode, modePubSub, modeStream)
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

func modeOf(op transport.Operation) string {
	if m := op.Binding["mode"]; m != "" {
		return m
	}
	return modePubSub
}

// newClient builds the client from the server URL, the binding credentials
// and the TLS settings. addr is the host:port for error messages: the URL
// itself may hold a password.
func newClient(op transport.Operation) (*goredis.Client, string, error) {
	opts, err := goredis.ParseURL(op.Server)
	if err != nil {
		// The parse error would echo the URL, which can hold a password.
		return nil, "", errBadServer
	}
	if u := op.Binding["username"]; u != "" {
		opts.Username = u
	}
	if p := op.Binding["password"]; p != "" {
		opts.Password = p
	}
	if opts.TLSConfig != nil && op.TLS != nil {
		cfg := op.TLS.Clone()
		if cfg.ServerName == "" {
			cfg.ServerName = opts.TLSConfig.ServerName
		}
		opts.TLSConfig = cfg
	}
	opts.Protocol = 2 // RESP2: works against every server and proxy
	opts.DisableIdentity = true
	opts.MaxRetries = -1
	opts.DialTimeout = connectTimeout
	return goredis.NewClient(opts), opts.Addr, nil
}

func ping(ctx context.Context, c *goredis.Client) error {
	ctx, cancel := context.WithTimeoutCause(ctx, connectTimeout, errNoAnswer)
	defer cancel()
	return c.Ping(ctx).Err()
}
