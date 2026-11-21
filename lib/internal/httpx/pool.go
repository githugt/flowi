// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/quic-go/quic-go/http3"
)

// Pool keeps the transports of one run, so the requests of a run that go to the same host reuse a connection (one DNS
// lookup, one TCP and TLS handshake) instead of each building a client and starting from nothing. A transport is shared
// by the requests whose options build the same one: the HTTP version, the proxy, the extra CAs and the client
// certificate. The zero Pool is ready; Close releases every connection it holds.
//
// Before: each request step built its own client, so a thousand-step batch made a thousand handshakes, and the timings
// of a step never showed a warm connection.
type Pool struct {
	mu     sync.Mutex
	byKey  map[string]http.RoundTripper
	closed bool
}

// transport returns the shared transport for o, building it the first time. After Close it builds none and says so:
// a run that has ended must not start connections nothing will release.
func (p *Pool) transport(o Options) (http.RoundTripper, error) {
	key := transportKey(o)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errPoolClosed
	}
	if tr, ok := p.byKey[key]; ok {
		return tr, nil
	}
	tr, err := newRoundTripper(o)
	if err != nil {
		return nil, err
	}
	if p.byKey == nil {
		p.byKey = map[string]http.RoundTripper{}
	}
	p.byKey[key] = tr
	return tr, nil
}

var errPoolClosed = fmt.Errorf("the run has ended; no new connections are made")

// Size is how many transports the pool holds.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.byKey)
}

// Close releases every connection the pool holds, including an HTTP/3 transport's UDP socket. It is safe to call twice.
func (p *Pool) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for _, tr := range p.byKey {
		switch t := tr.(type) {
		case *http.Transport:
			t.CloseIdleConnections()
		case *http3.Transport:
			_ = t.Close() // the run is over; a close error has no one to act on it
		}
	}
	p.byKey = nil
}

// transportKey names the transport o builds: everything newRoundTripper reads. A client certificate is told apart by
// its bytes, because each request loads its own copy of the same file.
func transportKey(o Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "v=%s", o.Version)
	if o.Proxy != nil {
		fmt.Fprintf(&b, "|proxy=%q,%q,%q,%t", o.Proxy.HTTPProxy, o.Proxy.HTTPSProxy, o.Proxy.NoProxy, o.Proxy.CGI)
	}
	for _, f := range o.CAFiles {
		fmt.Fprintf(&b, "|ca=%q", f)
	}
	if o.ClientCert != nil {
		h := sha256.New()
		for _, der := range o.ClientCert.Certificate {
			h.Write(der)
		}
		fmt.Fprintf(&b, "|cert=%x", h.Sum(nil))
	}
	return b.String()
}
