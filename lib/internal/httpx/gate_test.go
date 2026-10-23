// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_Gate_should_wrap_each_attempt_and_not_count_toward_timeout(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(srv.Close)
	var gates, releases atomic.Int32
	var deadlineInGate atomic.Bool
	var host atomic.Value
	o := defaultOptions()
	o.Retry = RetryPolicy{Max: 1, Backoff: "constant", On: []string{"5xx"}}
	o.Gate = func(ctx context.Context, h string) (func(), error) {
		gates.Add(1)
		host.Store(h)
		if _, has := ctx.Deadline(); has {
			deadlineInGate.Store(true)
		}
		return func() { releases.Add(1) }, nil
	}
	c := newClient(t, o)

	attempts, err := c.Do(context.Background(), Request{Method: "GET", URL: srv.URL})

	if err != nil || len(attempts) != 2 || gates.Load() != 2 || releases.Load() != 2 {
		t.Fatalf("err %v attempts %d gates %d releases %d", err, len(attempts), gates.Load(), releases.Load())
	}
	if deadlineInGate.Load() || host.Load() != "127.0.0.1" {
		t.Fatalf("gate ran under the attempt timeout (%v) or got host %v", deadlineInGate.Load(), host.Load())
	}
}

func TestClient_Gate_Given_error_Then_Do_returns_it_without_sending(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	t.Cleanup(srv.Close)
	o := defaultOptions()
	stop := errors.New("cancelled while queued")
	o.Gate = func(context.Context, string) (func(), error) { return nil, stop }
	c := newClient(t, o)

	attempts, err := c.Do(context.Background(), Request{Method: "GET", URL: srv.URL})

	if !errors.Is(err, stop) || len(attempts) != 0 || hits.Load() != 0 {
		t.Fatalf("err %v attempts %d hits %d", err, len(attempts), hits.Load())
	}
}
