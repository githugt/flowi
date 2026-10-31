// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"time"
)

const (
	retryBaseDelay = 200 * time.Millisecond
	retryMaxDelay  = 10 * time.Second
	// retryAfterCap bounds a server's Retry-After, so a misconfigured
	// server cannot park a run for hours.
	retryAfterCap = 60 * time.Second
)

// RetryPolicy says which failed attempts are retried and how long to wait.
type RetryPolicy struct {
	Max     int      // extra attempts; 0 = no retries
	Backoff string   // exponential | constant
	Jitter  bool     // full jitter: a random delay in [0, computed]
	On      []string // 5xx | 429 | timeout | connection

	base time.Duration // tests shorten the delay; 0 → retryBaseDelay
}

// next decides whether attempt n (0-based) is followed by another, and
// after how long.
func (p RetryPolicy) next(n int, a Attempt, now time.Time, r *rand.Rand) (time.Duration, bool) {
	if n >= p.Max || !slices.Contains(p.On, failureKind(a)) {
		return 0, false
	}
	if wait, ok := retryAfter(a, now); ok {
		return wait, true
	}
	return p.delay(n, r), true
}

// failureKind classifies an attempt for RetryPolicy.On; "" means success
// or a failure that is never retried.
func failureKind(a Attempt) string {
	var te *TimeoutError
	switch {
	case errors.As(a.Err, &te):
		return "timeout"
	case a.Err != nil:
		return "connection"
	case a.Response.Status == http.StatusTooManyRequests:
		return "429"
	case a.Response.Status >= http.StatusInternalServerError:
		return "5xx"
	}
	return ""
}

// delay is the backoff before attempt n+1.
func (p RetryPolicy) delay(n int, r *rand.Rand) time.Duration {
	base := cmpOr(p.base, retryBaseDelay)
	d := base
	if p.Backoff != "constant" {
		d = base << min(n, 30) // bounded shift; capped just below
		if d <= 0 || d > retryMaxDelay {
			d = retryMaxDelay
		}
	}
	if p.Jitter {
		d = time.Duration(r.Int64N(int64(d) + 1))
	}
	return d
}

func cmpOr(d, fallback time.Duration) time.Duration {
	if d == 0 {
		return fallback
	}
	return d
}

// retryAfter reads Retry-After (seconds or an HTTP date), capped.
func retryAfter(a Attempt, now time.Time) (time.Duration, bool) {
	if a.Response == nil {
		return 0, false
	}
	h := a.Response.Header.Get("Retry-After")
	if h == "" {
		return 0, false
	}
	var wait time.Duration
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		wait = time.Duration(secs) * time.Second
	} else if at, err := http.ParseTime(h); err == nil {
		wait = max(at.Sub(now), 0)
	} else {
		return 0, false
	}
	return min(wait, retryAfterCap), true
}
