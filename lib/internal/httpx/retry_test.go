// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// noSleep records waits instead of sleeping.
func noSleep(waits *[]time.Duration) func(context.Context, time.Duration) error {
	return func(ctx context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return ctx.Err()
	}
}

func statusSequence(codes ...int) (*httptest.Server, *atomic.Int32) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(n.Add(1)) - 1
		w.WriteHeader(codes[min(i, len(codes)-1)])
	}))
	return srv, &n
}

func retryOptions(waits *[]time.Duration) Options {
	o := defaultOptions()
	o.Retry = RetryPolicy{Max: 3, Backoff: "exponential", On: []string{"5xx", "429", "timeout", "connection"}}
	o.sleep = noSleep(waits)
	return o
}

func TestClient_Given_retry_on_5xx_Then_retries_until_success_and_records_attempts(t *testing.T) {
	srv, hits := statusSequence(503, 503, 200)
	defer srv.Close()
	var waits []time.Duration

	attempts, err := newClient(t, retryOptions(&waits)).Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

	if err != nil || len(attempts) != 3 || hits.Load() != 3 {
		t.Fatalf("err %v attempts %d hits %d", err, len(attempts), hits.Load())
	}
	if attempts[0].Response.Status != 503 || attempts[2].Response.Status != 200 || attempts[2].Wait != 0 {
		t.Fatalf("attempts %+v", attempts)
	}
	if len(waits) != 2 || waits[0] != retryBaseDelay || waits[1] != 2*retryBaseDelay || attempts[0].Wait != waits[0] {
		t.Fatalf("waits %v", waits)
	}
}

func TestClient_Given_retry_max_reached_Then_returns_all_failed_attempts(t *testing.T) {
	srv, hits := statusSequence(500)
	defer srv.Close()
	var waits []time.Duration

	attempts, err := newClient(t, retryOptions(&waits)).Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

	if err != nil || len(attempts) != 4 || hits.Load() != 4 || attempts[3].Response.Status != 500 {
		t.Fatalf("err %v attempts %d hits %d", err, len(attempts), hits.Load())
	}
}

func TestClient_Given_status_not_in_retry_on_Then_no_retry(t *testing.T) {
	srv, hits := statusSequence(429, 200)
	defer srv.Close()
	var waits []time.Duration
	o := retryOptions(&waits)
	o.Retry.On = []string{"5xx"}

	attempts, _ := newClient(t, o).Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

	if len(attempts) != 1 || hits.Load() != 1 {
		t.Fatalf("attempts %d", len(attempts))
	}
}

func TestClient_Given_Retry_After_Then_waits_that_long_capped(t *testing.T) {
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"2", 2 * time.Second},
		{"3600", retryAfterCap},
		{"Tue, 06 Oct 2026 12:00:05 GMT", 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			var n atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if n.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.header)
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			}))
			defer srv.Close()
			var waits []time.Duration
			o := retryOptions(&waits)
			o.Clock = func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) }

			attempts, err := newClient(t, o).Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

			if err != nil || len(attempts) != 2 || attempts[0].Wait != tt.want {
				t.Fatalf("err %v attempts %d wait %v", err, len(attempts), attempts[0].Wait)
			}
		})
	}
}

func TestClient_Given_cancel_during_backoff_Then_returns_promptly(t *testing.T) {
	srv, _ := statusSequence(503)
	defer srv.Close()
	o := defaultOptions()
	o.Retry = RetryPolicy{Max: 3, Backoff: "constant", On: []string{"5xx"}}
	o.Retry.base = 10 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()

	attempts, err := newClient(t, o).Do(ctx, Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

	if err == nil || len(attempts) != 1 || time.Since(start) > time.Second {
		t.Fatalf("err %v attempts %d after %v", err, len(attempts), time.Since(start))
	}
}

func TestRetryDelay_should_follow_exponential_with_full_jitter(t *testing.T) {
	const seed = 99
	p := RetryPolicy{Backoff: "exponential"}
	jittered := RetryPolicy{Backoff: "exponential", Jitter: true}
	constant := RetryPolicy{Backoff: "constant"}
	r := rand.New(rand.NewPCG(seed, seed))

	for n, want := range []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond} {
		if got := p.delay(n, r); got != want {
			t.Errorf("seed %d: delay(%d) = %v, want %v", seed, n, got, want)
		}
		if got := jittered.delay(n, r); got < 0 || got > want {
			t.Errorf("seed %d: jittered delay(%d) = %v, want within [0, %v]", seed, n, got, want)
		}
		if got := constant.delay(n, r); got != retryBaseDelay {
			t.Errorf("constant delay(%d) = %v", n, got)
		}
	}
	if got := p.delay(20, r); got != retryMaxDelay {
		t.Errorf("delay(20) = %v, want cap %v", got, retryMaxDelay)
	}
}
