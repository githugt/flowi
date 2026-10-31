// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http/httpproxy"
)

// Options configures a client. Every request it sends uses them.
type Options struct {
	// Transport, when set, sends every attempt instead of the network: a run played back from a recording.
	Transport       http.RoundTripper
	Version         string // auto | 1.1 | 2 | h2c | 3
	FollowRedirects bool
	MaxRedirects    int
	// RedirectCredentials says when a redirect may carry credential headers
	// to a new origin: "strip" (the default, and "" means the same) drops
	// them when the origin changes, "same_site" drops them only when the
	// registrable domain changes or https falls back to http, "keep" never
	// drops them. RedirectKeepHeaders names headers that are forwarded
	// whatever the mode.
	RedirectCredentials string
	RedirectKeepHeaders []string
	Timeout             time.Duration // per attempt
	Proxy               *httpproxy.Config
	CAFiles             []string
	ClientCert          *tls.Certificate // sent when the server asks for one (mutual TLS)
	RequestCompression  string           // none | gzip
	Jar                 http.CookieJar   // nil sends and keeps no cookies
	StreamThreshold     int64            // bodies larger than this stream to DownloadDir
	DownloadDir         string
	Retry               RetryPolicy
	Clock               func() time.Time // nil → time.Now
	Rand                *rand.Rand       // retry jitter; nil → runtime-seeded
	// BeforeAttempt and AfterAttempt, when set, are called around each
	// attempt (n is 0-based) on the goroutine calling Do, for live progress.
	BeforeAttempt func(n int)
	AfterAttempt  func(n int, a Attempt)
	// Gate, when set, is called before each attempt with the target host
	// and must return before the attempt starts; release is called when
	// the attempt ends. Waiting in Gate (for a concurrency slot or a rate
	// limit) does not count toward Timeout. An error stops Do.
	Gate func(ctx context.Context, host string) (release func(), err error)

	sleep func(context.Context, time.Duration) error // tests replace the real wait
}

// Request is what to send. Header is sent as is, plus Accept-Encoding when
// the caller did not set one.
type Request struct {
	Method string
	URL    string
	Header http.Header
	Body   BodySource // nil sends no body
}

// Hop is one redirect response.
type Hop struct {
	Status   int
	URL      string // the URL that answered with the redirect
	Location string
	Header   http.Header
	// Forwarded names the credential headers sent on to Location even
	// though it is another origin (RedirectCredentials, RedirectKeepHeaders).
	Forwarded []string
}

// Timings of one attempt. Total runs from sending to the end of the body.
type Timings struct {
	DNS, Connect, TLS, TTFB, Total time.Duration
}

// Response is a received response.
type Response struct {
	Status    int
	Proto     string // "HTTP/1.1", "HTTP/2.0"
	Header    http.Header
	Body      []byte // decoded; nil when streamed to File
	File      string // where a streamed body was written
	SHA256    string // hex digest of the decoded body
	RawSize   int64  // bytes on the wire
	Size      int64  // decoded bytes
	Encoding  string // Content-Encoding that was decoded; "" if none or unknown
	Redirects []Hop
	Timings   Timings
}

// Attempt is one try at sending the request. Exactly one of Response and
// Err is set. Wait is the backoff before the next attempt (zero for the
// last).
type Attempt struct {
	Response *Response
	Err      error
	Duration time.Duration
	Wait     time.Duration
}

// TimeoutError means an attempt took longer than Options.Timeout.
type TimeoutError struct{ After time.Duration }

func (e *TimeoutError) Error() string { return fmt.Sprintf("timed out after %s", e.After) }

// Client sends requests. One client serves one send or run; it owns its
// connection pool, released by CloseIdleConnections.
type Client struct {
	opts Options
	http *http.Client
	rt   http.RoundTripper
}

// NewClient builds a client. Unreadable or invalid CA files are errors.
func NewClient(o Options) (*Client, error) {
	if o.Clock == nil {
		o.Clock = time.Now
	}
	if o.sleep == nil {
		o.sleep = sleep
	}
	if o.Rand == nil {
		o.Rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // jitter, not security
	}
	tr := o.Transport
	if tr == nil {
		var err error
		if tr, err = newRoundTripper(o); err != nil {
			return nil, err
		}
	}
	hc := &http.Client{Transport: tr, Jar: o.Jar, CheckRedirect: redirectPolicy(o)}
	return &Client{opts: o, http: hc, rt: tr}, nil
}

// CloseIdleConnections releases pooled connections.
func (c *Client) CloseIdleConnections() { c.http.CloseIdleConnections() }

// Close releases everything the client holds. Call it when the client is
// done: an HTTP/3 transport owns a UDP socket that idle-connection cleanup
// does not close.
func (c *Client) Close() {
	c.http.CloseIdleConnections()
	if t, ok := c.rt.(*http3.Transport); ok {
		_ = t.Close() // the client is being discarded; a close error has no one to act on it
	}
}

// Do sends r, retrying per Options.Retry, and returns every attempt in
// order. Transport failures are reported in Attempt.Err; the error result
// is only for ctx being done.
func (c *Client) Do(ctx context.Context, r Request) ([]Attempt, error) {
	var attempts []Attempt
	host := hostOf(r.URL)
	for n := 0; ; n++ {
		release, err := c.enter(ctx, host)
		if err != nil {
			return attempts, err
		}
		if c.opts.BeforeAttempt != nil {
			c.opts.BeforeAttempt(n)
		}
		a := c.attempt(ctx, r)
		release()
		if ctx.Err() != nil {
			return attempts, context.Cause(ctx)
		}
		wait, retry := c.opts.Retry.next(n, a, c.opts.Clock(), c.opts.Rand)
		a.Wait = wait
		attempts = append(attempts, a)
		if c.opts.AfterAttempt != nil {
			c.opts.AfterAttempt(n, a)
		}
		if !retry {
			return attempts, nil
		}
		if err := c.opts.sleep(ctx, wait); err != nil {
			return attempts, err
		}
	}
}

// enter passes the gate, if any.
func (c *Client) enter(ctx context.Context, host string) (func(), error) {
	if c.opts.Gate == nil {
		return func() {}, nil
	}
	return c.opts.Gate(ctx, host)
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "" // the attempt reports the bad URL itself
	}
	return u.Hostname()
}

// attempt sends r once, within Options.Timeout.
func (c *Client) attempt(ctx context.Context, r Request) Attempt {
	start := c.opts.Clock()
	actx, cancel := context.WithTimeoutCause(ctx, c.opts.Timeout, &TimeoutError{After: c.opts.Timeout})
	defer cancel()
	hops := &[]Hop{}
	timing := newTiming(c.opts.Clock)
	actx = withTrace(withHops(actx, hops), timing)

	resp, err := c.send(actx, r)
	if err != nil {
		return Attempt{Err: classify(actx, err), Duration: c.opts.Clock().Sub(start)}
	}
	got, err := c.receive(resp)
	if err != nil {
		return Attempt{Err: classify(actx, err), Duration: c.opts.Clock().Sub(start)}
	}
	got.Redirects = *hops
	got.Timings = timing.result(start, c.opts.Clock())
	return Attempt{Response: got, Duration: got.Timings.Total}
}

func (c *Client) send(ctx context.Context, r Request) (*http.Response, error) {
	req, err := c.newRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

const defaultAcceptEncoding = "gzip, deflate, br, zstd"

func (c *Client) newRequest(ctx context.Context, r Request) (*http.Request, error) {
	if m, ok := r.Body.(multipartBody); ok {
		if boundary := userBoundary(r.Header.Get("Content-Type")); boundary != "" {
			r.Body = m.withBoundary(boundary)
		}
	}
	body, err := openBody(r.Body, c.opts.RequestCompression)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, body.reader)
	if err != nil {
		closeQuietly(body.reader)
		return nil, err
	}
	if r.Body != nil {
		// Bodies are reopenable, so the transport can resend them: for 307
		// and 308 redirects, and when HTTP/2 replays a request after GOAWAY.
		req.GetBody = func() (io.ReadCloser, error) {
			again, err := openBody(r.Body, c.opts.RequestCompression)
			return again.reader, err
		}
	}
	req.Header = r.Header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", defaultAcceptEncoding)
	}
	body.apply(req)
	return req, nil
}

// classify turns a failed attempt's error into what the caller records:
// a *TimeoutError for deadlines, and the inner error of a *url.Error,
// whose text would otherwise embed the full URL (and any secret in it).
func classify(actx context.Context, err error) error {
	var te *TimeoutError
	if cause := context.Cause(actx); errors.As(cause, &te) {
		return te
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// sleep waits d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-t.C:
		return nil
	}
}
