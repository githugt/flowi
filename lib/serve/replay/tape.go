// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/importers"
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
func (h *Handler) tapeRecord(r *http.Request, body []byte, res upstreamResponse, took time.Duration) engine.Record {
	rec := engine.Record{
		At: h.rec.Now(), Proto: res.proto, Status: res.status,
		Request: engine.SentRequest{Method: r.Method, URL: h.rec.target(r).String(), Header: sentHeaders(r.Header)},
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
