// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	// maxFailures bad tokens from one address inside failureWindow lock that
	// address out until the window ends.
	maxFailures   = 10
	failureWindow = time.Minute
	// maxTracked bounds the table: past it, entries whose window ended are
	// dropped, so a flood of addresses cannot grow it without limit.
	maxTracked = 4096
)

// failureLimiter counts failed token checks per remote address. The token is
// 256 random bits, so guessing is not a realistic attack; the limiter keeps
// an exposed endpoint from being hammered and its log from being flooded.
type failureLimiter struct {
	now func() time.Time

	mu   sync.Mutex // guards seen
	seen map[string]failures
}

type failures struct {
	count int
	since time.Time // start of the window the count belongs to
}

func newFailureLimiter(now func() time.Time) *failureLimiter {
	return &failureLimiter{now: now, seen: map[string]failures{}}
}

// locked reports whether addr has used up its failures in the current window.
func (l *failureLimiter) locked(addr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.seen[addr]
	return ok && l.now().Sub(f.since) < failureWindow && f.count >= maxFailures
}

// recordFailure counts one bad token from addr.
func (l *failureLimiter) recordFailure(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.seen[addr]
	if now.Sub(f.since) >= failureWindow {
		f = failures{since: now}
	}
	f.count++
	l.seen[addr] = f
	if len(l.seen) > maxTracked {
		l.dropExpired(now)
	}
}

// dropExpired forgets addresses whose window has ended. Callers hold mu.
func (l *failureLimiter) dropExpired(now time.Time) {
	for addr, f := range l.seen {
		if now.Sub(f.since) >= failureWindow {
			delete(l.seen, addr)
		}
	}
}

// remoteHost is the address of the peer without its port. It is the TCP peer,
// never a forwarded header: a caller must not choose its own bucket.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
