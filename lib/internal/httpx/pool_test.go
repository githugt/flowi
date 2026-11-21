// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// countingServer answers every request and counts the connections it accepted.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, &conns
}

func TestPool_Given_requests_with_one_pool_When_each_has_its_own_client_Then_they_share_a_connection(t *testing.T) {
	// Before: each request built a new client, so each one made a new connection.
	srv, conns := countingServer(t)
	pool := &Pool{}
	defer pool.Close()
	o := defaultOptions()
	o.Pool = pool

	for range 3 {
		c := newClient(t, o)
		get(t, c, srv.URL)
		c.Close() // the client is done; the pool's connection stays
	}

	if got := conns.Load(); got != 1 {
		t.Fatalf("connections = %d, want 1", got)
	}
	if pool.Size() != 1 {
		t.Fatalf("transports = %d, want 1", pool.Size())
	}
}

func TestPool_Given_no_pool_When_each_request_has_its_own_client_Then_each_makes_a_connection(t *testing.T) {
	// The cold-connection view: the setting that turns reuse off leaves each request starting from nothing.
	srv, conns := countingServer(t)

	for range 3 {
		c := newClient(t, defaultOptions())
		get(t, c, srv.URL)
		c.Close()
	}

	if got := conns.Load(); got != 3 {
		t.Fatalf("connections = %d, want 3", got)
	}
}

func TestPool_Given_different_transport_options_When_requested_Then_each_gets_its_own_transport(t *testing.T) {
	// A proxy or an HTTP version is part of the connection; sharing one transport across them would send a request
	// down a connection built for another.
	pool := &Pool{}
	defer pool.Close()
	a, b := defaultOptions(), defaultOptions()
	b.Version = "1.1"

	ta, err1 := pool.transport(a)
	tb, err2 := pool.transport(b)
	ta2, err3 := pool.transport(a)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatal(err1, err2, err3)
	}
	if ta == tb || ta != ta2 || pool.Size() != 2 {
		t.Fatalf("transports: a==b %t, a!=a2 %t, size %d", ta == tb, ta != ta2, pool.Size())
	}
}

func TestPool_Given_a_closed_pool_When_a_client_is_built_Then_it_is_refused(t *testing.T) {
	// A run that has ended must not start connections that nothing will release.
	pool := &Pool{}
	pool.Close()
	o := defaultOptions()
	o.Pool = pool

	_, err := NewClient(o)

	if err == nil {
		t.Fatal("NewClient succeeded on a closed pool")
	}
	pool.Close() // twice is fine
}

func TestPool_Given_a_pool_When_closed_Then_idle_connections_are_released(t *testing.T) {
	srv, _ := countingServer(t)
	pool := &Pool{}
	o := defaultOptions()
	o.Pool = pool
	c := newClient(t, o)
	get(t, c, srv.URL)

	pool.Close()

	if pool.Size() != 0 {
		t.Fatalf("size after close = %d", pool.Size())
	}
}
