// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
)

// gateAsync runs gate in a goroutine and reports when it got through.
func gateAsync(ctx context.Context, l *limits, host string) <-chan error {
	out := make(chan error, 1)
	go func() {
		release, err := l.gate(ctx, host)
		if err == nil {
			defer release()
		}
		out <- err
	}()
	return out
}

func TestLimits_should_space_attempts_by_rate_with_fake_clock(t *testing.T) {
	clock := newFakeClock()
	l, err := newLimits(10, "2/s", nil, clock.Now, clock.After)
	if err != nil {
		t.Fatal(err)
	}

	first := gateAsync(context.Background(), l, "a.test")
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	second := gateAsync(context.Background(), l, "a.test")
	clock.WaitForTimers(t, 1)
	select {
	case <-second:
		t.Fatal("second attempt was not held back by the rate")
	default:
	}
	clock.Advance(500 * 1e6)

	if err := <-second; err != nil {
		t.Fatal(err)
	}
}

func TestLimits_should_limit_only_the_named_host(t *testing.T) {
	clock := newFakeClock()
	l, err := newLimits(10, "", []model.KeyValue{{Key: "slow.test", Value: "1/s"}}, clock.Now, clock.After)
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := <-gateAsync(context.Background(), l, "fast.test"); err != nil {
			t.Fatal(err)
		}
	}
	<-gateAsync(context.Background(), l, "slow.test")
	held := gateAsync(context.Background(), l, "slow.test")
	clock.WaitForTimers(t, 1)
	clock.Advance(1e9)

	if err := <-held; err != nil {
		t.Fatal(err)
	}
}

func TestLimits_should_cap_in_flight_and_release_on_cancel(t *testing.T) {
	l, err := newLimits(1, "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	release, err := l.gate(context.Background(), "a.test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	waiting := gateAsync(ctx, l, "a.test")
	cancel()
	if err := <-waiting; err == nil {
		t.Fatal("a cancelled wait must fail")
	}
	release()

	if err := <-gateAsync(context.Background(), l, "a.test"); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
}

func TestNewLimits_Given_bad_rate_Then_error(t *testing.T) {
	if _, err := newLimits(1, "fast", nil, nil, nil); err == nil {
		t.Fatal("want an error")
	}
}

func TestLaneJar_should_follow_http_cookies(t *testing.T) {
	parent := httpx.NewJar()
	tests := []struct {
		policy string
		check  func(t *testing.T, f *frame)
	}{
		{policy: "isolated", check: func(t *testing.T, f *frame) {
			if f.jar == nil || f.jar == parent || f.noJar {
				t.Fatalf("isolated lane must get its own clone: %p", f.jar)
			}
		}},
		{policy: "shared", check: func(t *testing.T, f *frame) {
			if f.jar != parent || f.noJar {
				t.Fatal("shared lane must use the enclosing jar")
			}
		}},
		{policy: "off", check: func(t *testing.T, f *frame) {
			if !f.noJar {
				t.Fatal("off lane must have no jar")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.policy, func(t *testing.T) {
			r := &runner{laneJars: tt.policy}
			rt := &nodeRuntime{r: r, f: newRootFrame(&flowScope{}, parent), path: "users"}

			child := rt.laneFrame(nodes.Lane{Label: "[0]"}, true)

			tt.check(t, child)
			if plain := rt.laneFrame(nodes.Lane{Label: "then"}, false); plain.jar != parent {
				t.Fatal("lanes without LaneJars keep the enclosing jar")
			}
		})
	}
}
