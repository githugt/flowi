// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// limits are the run-wide HTTP limits (plan Decision 2): at most cap
// exchanges in flight, paced by an all-hosts rate and per-host rates. A
// slot is held for one attempt only, so callers never wait on each other
// while holding one — nested containers and hook requests cannot
// deadlock.
type limits struct {
	slots chan struct{}
	all   *limiter            // nil: no all-hosts rate
	hosts map[string]*limiter // by exact URL host name
}

func newLimits(maxInFlight int, all string, hosts []model.KeyValue, clock func() time.Time, after func(time.Duration) <-chan time.Time) (*limits, error) {
	l := &limits{slots: make(chan struct{}, max(1, maxInFlight)), hosts: map[string]*limiter{}}
	if all != "" {
		lim, err := newLimiter(all, clock, after)
		if err != nil {
			return nil, fmt.Errorf("rate_limit: %w", err)
		}
		l.all = lim
	}
	for _, h := range hosts {
		lim, err := newLimiter(h.Value, clock, after)
		if err != nil {
			return nil, fmt.Errorf("rate_limit %s: %w", h.Key, err)
		}
		l.hosts[h.Key] = lim
	}
	return l, nil
}

// gate waits for the host's rates, then for a slot; release frees it.
func (l *limits) gate(ctx context.Context, host string) (func(), error) {
	if err := l.all.wait(ctx); err != nil {
		return nil, err
	}
	if err := l.hosts[host].wait(ctx); err != nil {
		return nil, err
	}
	select {
	case l.slots <- struct{}{}:
		return func() { <-l.slots }, nil
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

// limiter spaces calls at least interval apart. Each caller reserves the
// next free time slot under mu, then waits for it without holding mu.
type limiter struct {
	interval time.Duration
	clock    func() time.Time
	after    func(time.Duration) <-chan time.Time

	mu   sync.Mutex
	next time.Time // guarded by mu
}

func newLimiter(rate string, clock func() time.Time, after func(time.Duration) <-chan time.Time) (*limiter, error) {
	interval, err := settings.ParseRate(rate)
	if err != nil {
		return nil, err
	}
	if clock == nil {
		clock = time.Now
	}
	if after == nil {
		after = time.After
	}
	return &limiter{interval: interval, clock: clock, after: after}, nil
}

// wait returns when the caller's reserved slot has come. A nil limiter
// never waits.
func (l *limiter) wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	now := l.clock()
	slot := l.next
	if slot.Before(now) {
		slot = now
	}
	l.next = slot.Add(l.interval)
	l.mu.Unlock()
	if !slot.After(now) {
		return nil
	}
	select {
	case <-l.after(slot.Sub(now)):
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
