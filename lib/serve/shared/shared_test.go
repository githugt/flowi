// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/serve/runs"
	"github.com/githugt/flowi-lib/serve/shared/sharedtest"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newValkey(t *testing.T, srv *sharedtest.Server, password string) *Valkey {
	t.Helper()
	v, err := NewValkey(ValkeyConfig{Addr: srv.Addr, Password: password, Prefix: "flowi:ws1:", Timeout: time.Second, Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

// storeContract is what every Store must do.
func storeContract(t *testing.T, newStore func(t *testing.T) (Store, func(time.Duration))) {
	t.Run("put then get", func(t *testing.T) {
		s, _ := newStore(t)

		err := s.Put(context.Background(), "a", []byte("hello\r\n\x00world"), time.Minute)
		got, ok, getErr := s.Get(context.Background(), "a")

		if err != nil || getErr != nil || !ok || !bytes.Equal(got, []byte("hello\r\n\x00world")) {
			t.Fatalf("put %v get %q ok=%v err=%v", err, got, ok, getErr)
		}
	})
	t.Run("get of a missing key", func(t *testing.T) {
		s, _ := newStore(t)

		got, ok, err := s.Get(context.Background(), "missing")

		if err != nil || ok || got != nil {
			t.Fatalf("got %q ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("a value expires", func(t *testing.T) {
		s, advance := newStore(t)
		if err := s.Put(context.Background(), "a", []byte("x"), time.Minute); err != nil {
			t.Fatal(err)
		}

		advance(2 * time.Minute)
		_, ok, err := s.Get(context.Background(), "a")

		if err != nil || ok {
			t.Fatalf("still there: ok=%v err=%v", ok, err)
		}
	})
	t.Run("claim is first come first served and expires", func(t *testing.T) {
		s, advance := newStore(t)

		first, err1 := s.Claim(context.Background(), "tick", time.Minute)
		second, err2 := s.Claim(context.Background(), "tick", time.Minute)
		advance(2 * time.Minute)
		third, err3 := s.Claim(context.Background(), "tick", time.Minute)

		if err1 != nil || err2 != nil || err3 != nil || !first || second || !third {
			t.Fatalf("claims %v %v %v (errs %v %v %v)", first, second, third, err1, err2, err3)
		}
	})
	t.Run("keys are independent", func(t *testing.T) {
		s, _ := newStore(t)

		a, _ := s.Claim(context.Background(), "a", time.Minute)
		b, _ := s.Claim(context.Background(), "b", time.Minute)

		if !a || !b {
			t.Fatalf("a=%v b=%v", a, b)
		}
	})
}

func TestMemory_should_satisfy_the_store_contract(t *testing.T) {
	storeContract(t, func(*testing.T) (Store, func(time.Duration)) {
		clock := &fakeClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
		return NewMemory(clock.Now), clock.Advance
	})
}

func TestValkey_should_satisfy_the_store_contract(t *testing.T) {
	storeContract(t, func(t *testing.T) (Store, func(time.Duration)) {
		srv := sharedtest.Start(t, "")
		return newValkey(t, srv, ""), srv.Advance
	})
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func TestValkey_should_scope_keys_with_the_prefix(t *testing.T) {
	srv := sharedtest.Start(t, "")
	v := newValkey(t, srv, "")

	_ = v.Put(context.Background(), "run:1", []byte("x"), time.Minute)

	if keys := srv.Keys(); len(keys) != 1 || keys[0] != "flowi:ws1:run:1" {
		t.Fatalf("keys %v", keys)
	}
}

func TestValkey_should_authenticate_with_the_password(t *testing.T) {
	srv := sharedtest.Start(t, "s3cret")

	good := newValkey(t, srv, "s3cret")
	bad := newValkey(t, srv, "wrong")
	none := newValkey(t, srv, "")

	if err := good.Put(context.Background(), "a", []byte("x"), time.Minute); err != nil {
		t.Fatalf("right password: %v", err)
	}
	for name, v := range map[string]*Valkey{"wrong password": bad, "no password": none} {
		err := v.Put(context.Background(), "a", []byte("x"), time.Minute)
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if err != nil && strings.Contains(err.Error(), "wrong") {
			t.Errorf("%s: the error repeats the password: %v", name, err)
		}
	}
}

func TestValkey_Given_a_dropped_connection_Then_it_reconnects_once(t *testing.T) {
	srv := sharedtest.Start(t, "")
	v := newValkey(t, srv, "")
	if err := v.Put(context.Background(), "a", []byte("x"), time.Minute); err != nil {
		t.Fatal(err)
	}
	srv.DropConnections()

	got, ok, err := v.Get(context.Background(), "a")

	if err != nil || !ok || string(got) != "x" || srv.Connections() != 2 {
		t.Fatalf("got %q ok=%v err=%v after %d connections", got, ok, err, srv.Connections())
	}
}

func TestValkey_Given_an_unreachable_server_Then_it_fails_fast_and_stays_quiet_for_a_while(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens on addr any more
	clock := &fakeClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	v, err := NewValkey(ValkeyConfig{Addr: addr, Timeout: 500 * time.Millisecond, Logger: quiet(), Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v.Close() }()

	first := v.Put(context.Background(), "a", []byte("x"), time.Minute)
	started := time.Now()
	second := v.Put(context.Background(), "a", []byte("x"), time.Minute)
	elapsed := time.Since(started)

	if !errors.Is(first, ErrUnavailable) || !errors.Is(second, ErrUnavailable) {
		t.Fatalf("errors %v / %v", first, second)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("a call during the quiet period took %v; it should not touch the network", elapsed)
	}
}

func TestValkey_should_try_again_after_the_quiet_period(t *testing.T) {
	srv := sharedtest.Start(t, "")
	clock := &fakeClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	v, err := NewValkey(ValkeyConfig{Addr: srv.Addr, Timeout: time.Second, Logger: quiet(), Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v.Close() }()
	srv.Close()
	if err := v.Put(context.Background(), "a", []byte("x"), time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected the outage to be noticed: %v", err)
	}

	srv2 := sharedtest.Start(t, "")
	v.cfg.Addr = srv2.Addr // the "same" server is back
	clock.Advance(quietPeriod + time.Second)
	err = v.Put(context.Background(), "a", []byte("x"), time.Minute)

	if err != nil {
		t.Fatalf("still unavailable after the quiet period: %v", err)
	}
}

func TestValkey_should_refuse_oversized_values_and_empty_config(t *testing.T) {
	srv := sharedtest.Start(t, "")
	v := newValkey(t, srv, "")

	err := v.Put(context.Background(), "a", make([]byte, maxValue+1), time.Minute)
	_, noAddr := NewValkey(ValkeyConfig{})

	if !errors.Is(err, ErrTooLarge) || noAddr == nil {
		t.Fatalf("put %v, empty config %v", err, noAddr)
	}
}

func TestValkey_should_never_put_the_password_in_errors_or_logs(t *testing.T) {
	var logs bytes.Buffer
	srv := sharedtest.Start(t, "right")
	v, err := NewValkey(ValkeyConfig{Addr: srv.Addr, Password: "hunter2-wrong", Timeout: time.Second, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v.Close() }()

	putErr := v.Put(context.Background(), "a", []byte("x"), time.Minute)

	if putErr == nil || strings.Contains(putErr.Error(), "hunter2") || strings.Contains(logs.String(), "hunter2") {
		t.Fatalf("error %v, logs %s", putErr, logs.String())
	}
}

func TestRecords_should_round_trip_a_run_through_a_store(t *testing.T) {
	r := Records{Store: NewMemory(nil)}
	rec := runs.Record{ID: "r1", Flow: "f", Status: "passed", Started: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), Outcome: runs.Outcome{EngineRunID: "e1", Messages: []string{"m"}}}

	err := r.SaveRecord(context.Background(), rec)
	got, ok, loadErr := r.LoadRecord(context.Background(), "r1")
	_, missing, _ := r.LoadRecord(context.Background(), "r2")

	if err != nil || loadErr != nil || !ok || missing || got.ID != "r1" || got.EngineRunID != "e1" || !got.Started.Equal(rec.Started) || len(got.Messages) != 1 {
		t.Fatalf("got %+v ok=%v err=%v/%v", got, ok, err, loadErr)
	}
}
