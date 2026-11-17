// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/importers"
)

// Tape keeps the calls of a recording session as a HAR file, in the order they were made, so the session can
// be turned into a flow later (`flowi import har`), with the values that passed from one call to the next
// wired together. The file is rewritten after each call, through a temporary file, so it is whole whenever
// the process stops. Credentials are masked before they are kept: request and response headers that carry
// one, and JSON values under credential-looking names.
type Tape struct {
	Path    string // where the HAR file is written
	Version string // Flowi's version, written as the HAR creator's

	mu      sync.Mutex // guards records
	records []engine.Record
}

// add keeps one call and rewrites the file.
func (t *Tape) add(rec engine.Record) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, masked(rec))
	var buf bytes.Buffer
	if err := engine.WriteHAR(&buf, t.records, t.Version); err != nil {
		return err
	}
	if err := writeAtomic(t.Path, buf.Bytes()); err != nil {
		return fmt.Errorf("writing the tape %s: %w", t.Path, err)
	}
	return nil
}

// masked is rec with credentials replaced.
func masked(rec engine.Record) engine.Record {
	rec.Request.Header = maskedHeaders(rec.Request.Header)
	rec.Header = maskedHeaders(rec.Header)
	if rec.Request.Body != nil {
		body := maskedBody(*rec.Request.Body)
		rec.Request.Body = &body
	}
	rec.Body = maskedBody(rec.Body)
	return rec
}

func maskedHeaders(hs []engine.Header) []engine.Header {
	out := make([]engine.Header, len(hs))
	for i, h := range hs {
		out[i] = h
		if importers.SensitiveHeader(h.Name) {
			out[i].Value = importers.Redacted
		}
	}
	return out
}

// maskedBody replaces credential values in a JSON body. The text form is rebuilt from the masked value, so
// the HAR, which holds text, carries no credential either.
func maskedBody(b engine.Body) engine.Body {
	if b.JSON != nil {
		b.JSON, _ = importers.RedactJSON(b.JSON)
		b.Text = ""
	} else if parsed, ok := parseJSON([]byte(b.Text)); ok && strings.Contains(strings.ToLower(b.ContentType), "json") {
		clean, n := importers.RedactJSON(parsed)
		if n > 0 {
			b.Text = marshalText(clean, b.Text)
		}
	}
	return b
}

// tapeRecord builds the record of one forwarded call: the upstream URL it went to, the request as the
// caller sent it, and the response as it came back.
func (h *Handler) tapeRecord(now func() time.Time, target string, r *http.Request, body []byte, res upstreamResponse, took time.Duration) engine.Record {
	rec := engine.Record{
		At: now(), Proto: res.proto, Status: res.status,
		Request: engine.SentRequest{Method: r.Method, URL: target, Header: sentHeaders(r.Header)},
		Header:  nil, Body: bodyOf(res), Timings: engine.Timings{Total: took},
	}
	if len(body) > 0 {
		b := engine.Body{Size: int64(len(body)), ContentType: r.Header.Get("Content-Type")}
		if parsed, ok := parseJSON(body); ok && strings.Contains(strings.ToLower(b.ContentType), "json") {
			b.JSON = parsed
		} else {
			b.Text = string(body)
		}
		rec.Request.Body = &b
	}
	for name, values := range res.header {
		for _, v := range values {
			rec.Header = append(rec.Header, engine.Header{Name: name, Value: v})
		}
	}
	return rec
}

// marshalText encodes v as JSON text, or returns fallback when it cannot be encoded.
func marshalText(v any, fallback string) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fallback
	}
	return string(data)
}

// tapeWriter passes an answer through and keeps its status, headers and (up to CaptureLimit) body for the tape.
type tapeWriter struct {
	http.ResponseWriter
	status  int
	header  http.Header
	body    bytes.Buffer
	reqBody []byte
}

// newTapeWriter also reads the request body (up to CaptureLimit) and puts it back for the handler.
func newTapeWriter(w http.ResponseWriter, r *http.Request) *tapeWriter {
	tw := &tapeWriter{ResponseWriter: w}
	if r.Body != nil && r.Body != http.NoBody {
		all, _ := io.ReadAll(io.LimitReader(r.Body, CaptureLimit+1))
		tw.reqBody = all[:min(len(all), CaptureLimit)]
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(all), r.Body))
	}
	return tw
}

func (t *tapeWriter) WriteHeader(status int) {
	if t.status == 0 {
		t.status, t.header = status, t.Header().Clone()
	}
	t.ResponseWriter.WriteHeader(status)
}

func (t *tapeWriter) Write(p []byte) (int, error) {
	if t.status == 0 {
		t.WriteHeader(http.StatusOK)
	}
	if room := CaptureLimit - t.body.Len(); room > 0 {
		t.body.Write(p[:min(len(p), room)])
	}
	return t.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the real writer.
func (t *tapeWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }

// keepAnswered adds a call the replayer answered itself to the tape; a tape that cannot be written is a warning.
func (h *Handler) keepAnswered(tw *tapeWriter, r *http.Request, took time.Duration) {
	res := upstreamResponse{status: tw.status, proto: r.Proto, header: tw.header, body: tw.body.Bytes()}
	if res.status == 0 {
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	u := url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
	rec := h.tapeRecord(time.Now, u.String(), r, tw.reqBody, res, took)
	if err := h.opts.Tape.add(rec); err != nil {
		h.opts.Logger.Warn("call not kept on the tape", "Event", "ReplayTapeFailed", "Error", err)
	}
}
