// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

// Header is one header line, kept in order.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Body is a request or response body as recorded.
type Body struct {
	Text        string `json:"text,omitempty"`        // UTF-8 bodies
	Base64      string `json:"base64,omitempty"`      // other bodies
	JSON        any    `json:"json,omitempty"`        // parsed, when the body is JSON
	ParseErr    string `json:"parse_error,omitempty"` // the body claimed JSON but is not
	File        string `json:"file,omitempty"`        // a streamed download, or an uploaded file
	SHA256      string `json:"sha256,omitempty"`
	Size        int64  `json:"size"`
	RawSize     int64  `json:"raw_size,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

// SentRequest is the request as actually sent, masked.
type SentRequest struct {
	Method string   `json:"method"`
	URL    string   `json:"url"`
	Header []Header `json:"headers,omitempty"`
	Body   *Body    `json:"body,omitempty"`
}

// Hop is one redirect.
type Hop struct {
	Status   int      `json:"status"`
	URL      string   `json:"url"`
	Location string   `json:"location"`
	Header   []Header `json:"headers,omitempty"`
	// Forwarded names the credential headers sent on to Location although
	// it is another origin, because http.redirect_credentials or
	// http.redirect_keep_headers allows it.
	Forwarded []string `json:"forwarded,omitempty"`
}

// Timings of the final attempt.
type Timings struct {
	DNS     time.Duration `json:"dns"`
	Connect time.Duration `json:"connect"`
	TLS     time.Duration `json:"tls"`
	TTFB    time.Duration `json:"ttfb"`
	Total   time.Duration `json:"total"`
}

// AttemptRecord is one attempt: a status, or the error that stopped it.
type AttemptRecord struct {
	Status   int           `json:"status,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Wait     time.Duration `json:"wait,omitempty"`
}

// Record is a response record (design §6.4). Its JSON form is the saved
// response format.
type Record struct {
	ID            string   `json:"id"`
	RunID         string   `json:"run_id"`
	CollectionID  model.ID `json:"collection_id"`
	RequestID     model.ID `json:"request_id"`
	RequestPath   string   `json:"request_path"`
	Version       string   `json:"version"`
	VersionSource string   `json:"version_source"` // pinned | preferred | default | highest
	// Overlay names the flowi overlay the request was sent with, if any.
	Overlay   string          `json:"overlay,omitempty"`
	At        time.Time       `json:"at"`
	Request   SentRequest     `json:"request"`
	Status    int             `json:"status"`
	Proto     string          `json:"proto,omitempty"`
	Header    []Header        `json:"headers,omitempty"`
	Body      Body            `json:"body"`
	Redirects []Hop           `json:"redirects,omitempty"`
	Timings   Timings         `json:"timings"`
	Attempts  []AttemptRecord `json:"attempts"`
	Outputs   map[string]any  `json:"outputs,omitempty"`

	// Kind is "async" for an async request; "" is an HTTP one. An async
	// record has Status 0, Request.Method is the protocol and Outcome says
	// how it ended. Body is the first message received (else the one sent).
	Kind     string          `json:"kind,omitempty"`
	Outcome  string          `json:"outcome,omitempty"`
	Messages []RecordMessage `json:"messages,omitempty"`

	// A gRPC record ("grpc" kind) is also message based: Messages holds the
	// request as sent and the responses, Header the response metadata, and
	// these the final status (its name, e.g. NOT_FOUND, and message) and the
	// trailers. Status stays 0.
	GRPCCode    string   `json:"grpc_code,omitempty"`
	GRPCMessage string   `json:"grpc_message,omitempty"`
	Trailers    []Header `json:"trailers,omitempty"`
}

// HasMessages reports whether the record is message based (async or gRPC)
// and so has an Outcome and Messages instead of a status.
func (r *Record) HasMessages() bool { return r.Kind == KindAsync || r.Kind == KindGRPC }

// RecordMessage is one message of an async record, masked.
type RecordMessage struct {
	Direction string    `json:"direction"` // sent | received
	At        time.Time `json:"at"`
	Headers   []Header  `json:"headers,omitempty"`
	Body      Body      `json:"body"`
	Key       string    `json:"key,omitempty"`
	Partition int32     `json:"partition,omitempty"`
	Offset    int64     `json:"offset,omitempty"`
	Sequence  uint64    `json:"sequence,omitempty"`
}

// The kinds of record, and the outcomes of an async one.
const (
	KindAsync      = "async"
	KindGRPC       = "grpc"
	KindDataset    = "dataset"
	OutcomeOK      = "ok"
	OutcomeTimeout = "timeout"
	OutcomeError   = "error"
	// OutcomeCancelled is a message-based run the user stopped; the messages that
	// had arrived are kept.
	OutcomeCancelled = "cancelled"
	// OutcomeStopped is a message-based run a message hook ended: the messages kept up to
	// the one that failed its check are the record.
	OutcomeStopped = "stopped"
	directionSent  = "sent"
	directionRecvd = "received"
)

func (m RecordMessage) masked(mk vault.Masker) RecordMessage {
	out := m
	out.Headers = maskHeaders(m.Headers, mk)
	out.Body = m.Body.masked(mk)
	out.Key = mk.String(m.Key)
	return out
}

// Masked returns a deep copy with every known secret masked in every
// string the record holds.
func (r Record) Masked(m vault.Masker) Record {
	out := r
	out.Request = r.Request.masked(m)
	out.Header = maskHeaders(r.Header, m)
	out.Body = r.Body.masked(m)
	out.Redirects = make([]Hop, len(r.Redirects))
	for i, h := range r.Redirects {
		out.Redirects[i] = Hop{Status: h.Status, URL: m.String(h.URL), Location: m.String(h.Location), Header: maskHeaders(h.Header, m), Forwarded: h.Forwarded}
	}
	if r.Redirects == nil {
		out.Redirects = nil
	}
	out.Attempts = slices.Clone(r.Attempts)
	for i := range out.Attempts {
		out.Attempts[i].Error = m.String(out.Attempts[i].Error)
	}
	if r.Outputs != nil {
		out.Outputs = maskValue(r.Outputs, m).(map[string]any)
	}
	if r.Messages != nil {
		out.Messages = make([]RecordMessage, len(r.Messages))
		for i, msg := range r.Messages {
			out.Messages[i] = msg.masked(m)
		}
	}
	return out
}

func (s SentRequest) masked(m vault.Masker) SentRequest {
	out := SentRequest{Method: s.Method, URL: m.String(s.URL), Header: maskHeaders(s.Header, m)}
	if s.Body != nil {
		b := s.Body.masked(m)
		out.Body = &b
	}
	return out
}

func (b Body) masked(m vault.Masker) Body {
	out := b
	out.Text = m.String(b.Text)
	out.ParseErr = m.String(b.ParseErr)
	out.JSON = maskValue(b.JSON, m)
	if b.Base64 != "" {
		if raw, err := base64.StdEncoding.DecodeString(b.Base64); err == nil {
			out.Base64 = base64.StdEncoding.EncodeToString(m.Bytes(raw))
		}
	}
	return out
}

func maskHeaders(hs []Header, m vault.Masker) []Header {
	if hs == nil {
		return nil
	}
	out := make([]Header, len(hs))
	for i, h := range hs {
		out[i] = Header{Name: h.Name, Value: m.String(h.Value)}
	}
	return out
}

// maskValue deep-copies a decoded JSON value with its strings masked.
func maskValue(v any, m vault.Masker) any {
	switch x := v.(type) {
	case string:
		return m.String(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[m.String(k)] = maskValue(val, m)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = maskValue(val, m)
		}
		return out
	}
	return v
}

// headerList flattens an http.Header in a stable order (by name).
func headerList(h http.Header) []Header {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	slices.Sort(names)
	var out []Header
	for _, name := range names {
		for _, v := range h[name] {
			out = append(out, Header{Name: name, Value: v})
		}
	}
	return out
}

// responseBody records a received body: text or base64, parsed when it is
// JSON, and parse errors kept rather than failing the record.
func responseBody(resp *httpx.Response) Body {
	ctype := resp.Header.Get("Content-Type")
	b := Body{File: resp.File, SHA256: resp.SHA256, Size: resp.Size, RawSize: resp.RawSize, ContentType: ctype}
	if resp.File != "" {
		return b
	}
	if utf8.Valid(resp.Body) {
		b.Text = string(resp.Body)
	} else {
		b.Base64 = base64.StdEncoding.EncodeToString(resp.Body)
	}
	if len(resp.Body) == 0 || !looksJSON(ctype, resp.Body) {
		return b
	}
	parsed, err := decodeJSON(resp.Body)
	if err != nil {
		b.ParseErr = err.Error()
		return b
	}
	b.JSON = parsed
	return b
}

// decodeJSON parses a body keeping numbers exact (json.Number): float64
// would corrupt integer ids above 2^53, and the record is what is saved
// and compared later.
func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return v, nil
}

// scopeValue converts exact JSON numbers for templates: integers become
// int (comparisons like response.body.n == 1 work), other numbers float64.
func scopeValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil && int64(int(n)) == n {
			return int(n)
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = scopeValue(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = scopeValue(e)
		}
		return out
	}
	return v
}

// looksJSON is true for a JSON media type (application/json, *+json), or
// no media type with a body that starts like JSON.
func looksJSON(contentType string, body []byte) bool {
	media, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return media == "application/json" || strings.HasSuffix(media, "+json")
	}
	trimmed := strings.TrimSpace(string(body[:min(len(body), 1)]))
	return contentType == "" && (trimmed == "{" || trimmed == "[")
}

// attemptRecords converts httpx attempts; error text is masked later
// with the record.
func attemptRecords(attempts []httpx.Attempt) []AttemptRecord {
	out := make([]AttemptRecord, len(attempts))
	for i, a := range attempts {
		out[i] = AttemptRecord{Duration: a.Duration, Wait: a.Wait}
		if a.Err != nil {
			out[i].Error = a.Err.Error()
		} else {
			out[i].Status = a.Response.Status
		}
	}
	return out
}

// fillResponse copies the final response into the record.
func fillResponse(r *Record, resp *httpx.Response) {
	r.Status, r.Proto = resp.Status, resp.Proto
	r.Header = headerList(resp.Header)
	r.Body = responseBody(resp)
	for _, h := range resp.Redirects {
		r.Redirects = append(r.Redirects, Hop{Status: h.Status, URL: h.URL, Location: h.Location, Header: headerList(h.Header), Forwarded: h.Forwarded})
	}
	t := resp.Timings
	r.Timings = Timings{DNS: t.DNS, Connect: t.Connect, TLS: t.TLS, TTFB: t.TTFB, Total: t.Total}
}
