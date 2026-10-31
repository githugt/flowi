// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

// timing collects httptrace callbacks, which can arrive from several
// goroutines (parallel dials), so mu guards every field.
type timing struct {
	now func() time.Time

	mu                               sync.Mutex
	dnsStart, connectStart, tlsStart time.Time
	dns, connect, tls                time.Duration
	firstByte                        time.Time
}

func newTiming(now func() time.Time) *timing { return &timing{now: now} }

func withTrace(ctx context.Context, t *timing) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { t.mark(&t.dnsStart) },
		DNSDone:              func(httptrace.DNSDoneInfo) { t.since(&t.dns, t.dnsStart) },
		ConnectStart:         func(string, string) { t.mark(&t.connectStart) },
		ConnectDone:          func(string, string, error) { t.since(&t.connect, t.connectStart) },
		TLSHandshakeStart:    func() { t.mark(&t.tlsStart) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { t.since(&t.tls, t.tlsStart) },
		GotFirstResponseByte: func() { t.mark(&t.firstByte) },
	})
}

func (t *timing) mark(at *time.Time) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	*at = now
}

func (t *timing) since(d *time.Duration, start time.Time) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	*d = now.Sub(start)
}

func (t *timing) result(start, end time.Time) Timings {
	t.mu.Lock()
	defer t.mu.Unlock()
	r := Timings{DNS: t.dns, Connect: t.connect, TLS: t.tls, Total: end.Sub(start)}
	if !t.firstByte.IsZero() {
		r.TTFB = t.firstByte.Sub(start)
	}
	return r
}
