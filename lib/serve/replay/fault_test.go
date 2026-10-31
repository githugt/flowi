// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/serve/replay"
)

func loginFixture(t *testing.T, mutate func(*replay.Options)) *fixture {
	t.Helper()
	f := newFixture(t, spec{
		path: "auth/login", versions: []ver{{label: "1", method: "POST", url: "{{env.base_url}}/v1/login"}},
		examples: map[string]string{"ok": ex(200, `{"ok": true}`)},
	}).materialise()
	mutate(&f.options)
	return f.start()
}

func rolling(values ...float64) func() float64 {
	i := 0
	return func() float64 {
		v := values[i%len(values)]
		i++
		return v
	}
}

func TestHandler_Given_an_error_rate_When_the_roll_falls_inside_it_Then_the_call_gets_the_error_status(t *testing.T) {
	// Arrange
	f := loginFixture(t, func(o *replay.Options) {
		o.Faults = replay.Faults{ErrorRate: 0.3, ErrorStatus: 503}
		o.Rand = rolling(0.1, 0.9)
	})

	// Act
	failed := f.do("POST", "/v1/login")
	passed := f.do("POST", "/v1/login")

	// Assert
	if failed.status != 503 || failed.header.Get("X-Flowi-Match") != "fault" {
		t.Errorf("failed: %d %s", failed.status, failed.header.Get("X-Flowi-Match"))
	}
	if passed.status != 200 {
		t.Errorf("passed: %d", passed.status)
	}
}

func TestHandler_Given_a_fault_header_Then_it_overrides_the_rates_both_ways(t *testing.T) {
	// Arrange
	f := loginFixture(t, func(o *replay.Options) {
		o.Faults = replay.Faults{ErrorRate: 1}
		o.Rand = rolling(0.5)
	})

	// Act
	none := f.do("POST", "/v1/login", "X-Flowi-Fault", "none")
	forced := f.do("POST", "/v1/login", "X-Flowi-Fault", "error:418")
	bad := f.do("POST", "/v1/login", "X-Flowi-Fault", "melt")

	// Assert
	if none.status != 200 || forced.status != 418 || bad.status != 400 {
		t.Fatalf("none %d forced %d bad %d", none.status, forced.status, bad.status)
	}
}

func TestHandler_Given_jitter_Then_each_answer_waits_the_latency_plus_a_share_of_the_jitter(t *testing.T) {
	// Arrange
	f := loginFixture(t, func(o *replay.Options) {
		o.Latency = 100 * time.Millisecond
		o.Faults = replay.Faults{Jitter: 200 * time.Millisecond}
		o.Rand = rolling(0.5)
	})

	// Act
	f.do("POST", "/v1/login")

	// Assert
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sleeps) != 1 || f.sleeps[0] != 200*time.Millisecond {
		t.Fatalf("sleeps %v", f.sleeps)
	}
}

func TestHandler_Given_a_drop_Then_the_connection_closes_without_an_answer(t *testing.T) {
	// Arrange
	f := loginFixture(t, func(o *replay.Options) { o.Faults = replay.Faults{DropRate: 1}; o.Rand = rolling(0.5) })
	srv := httptest.NewServer(f.h)
	t.Cleanup(srv.Close)

	// Act
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/v1/login", nil)
	resp, err := srv.Client().Do(req)

	// Assert
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("want a dropped connection, got status %d", resp.StatusCode)
	}
	if errors.Is(err, t.Context().Err()) {
		t.Fatalf("the test ended, not the connection: %v", err)
	}
}

func TestFaults_Validate_Given_rates_outside_zero_to_one_or_summing_over_one_Then_it_refuses(t *testing.T) {
	cases := map[string]replay.Faults{
		"negative": {ErrorRate: -0.1}, "over one": {DropRate: 1.2}, "sum": {ErrorRate: 0.7, DropRate: 0.6},
		"bad status": {ErrorRate: 0.1, ErrorStatus: 99}, "negative jitter": {Jitter: -time.Second},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			if err := f.Validate(); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	if err := (replay.Faults{ErrorRate: 0.5, DropRate: 0.5}).Validate(); err != nil {
		t.Fatalf("a full split is fine: %v", err)
	}
}
