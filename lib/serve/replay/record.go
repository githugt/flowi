// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// MatchUpstreamError is the Match value when the upstream could not be
// reached or answered badly.
const MatchUpstreamError = "upstream-error"

const (
	defaultRecordTimeout = 30 * time.Second
	defaultMaxBody       = 16 << 20
)

// Recorder turns the replayer into a recording proxy: every call is
// forwarded to Upstream and answered with the real response. The response is
// also saved as a saved response (<status>.json and last.json in the workspace
// data dir, where Flowi keeps responses it receives) under the saved request
// the call matches. A call that matches none is turned into a new request by
// Capture, when it is set, and saved under that; without Capture it is only
// forwarded. Nothing else is written into a collection: pin an example from a
// saved response to commit it.
//
// Saved bodies and response headers are kept as received: the data dir is not
// part of a collection. Request headers are saved too, except the replayer's
// own (X-Flowi-*), with credential-looking values replaced by a
// {{secret.name}} placeholder, as the importers do.
type Recorder struct {
	Upstream *url.URL         // scheme, host and an optional path prefix
	Client   *http.Client     // nil: a 30s timeout and no redirect following
	MaxBody  int64            // largest upstream body forwarded (default 16 MiB)
	Now      func() time.Time // nil: time.Now
	// Capture makes a saved request of a call no saved request matches and
	// returns it, so the response is saved under it and the next such call
	// matches it. Calls are captured one at a time.
	Capture Capture
	// Tape, when set, keeps every forwarded call in a HAR file (see Tape).
	Tape *Tape
}

// Call is a call the replayer forwarded that no saved request matches.
type Call struct {
	Method string
	URL    string // where it was forwarded to, with its query
	Header http.Header
	Body   []byte
}

// Captured is the saved request a Capture made of a Call.
type Captured struct {
	Collection model.Collection
	Request    model.Request
	Version    model.RequestVersion
}

// Capture saves a call as a request of a collection.
type Capture func(Call) (Captured, error)

func (r Recorder) withDefaults() Recorder {
	if r.Client == nil {
		r.Client = &http.Client{
			Timeout:       defaultRecordTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	if r.MaxBody <= 0 {
		r.MaxBody = defaultMaxBody
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	return r
}

// upstreamResponse is a response read in full.
type upstreamResponse struct {
	status int
	proto  string
	header http.Header
	body   []byte
}

var errTooLarge = errors.New("upstream response is larger than the recording limit")

// forward sends the call upstream and reads the answer.
func (r Recorder) forward(req *http.Request, body []byte) (upstreamResponse, error) {
	// The host is the operator's --record upstream; a call only supplies the
	// path (kept under the upstream's prefix) and query.
	out, err := http.NewRequestWithContext(req.Context(), req.Method, r.target(req).String(), bytes.NewReader(body)) //nolint:gosec // see above
	if err != nil {
		return upstreamResponse{}, err
	}
	out.ContentLength = int64(len(body))
	for name, values := range req.Header {
		if forwardable(name) {
			out.Header[name] = append([]string(nil), values...)
		}
	}
	res, err := r.Client.Do(out) //nolint:gosec // see above
	if err != nil {
		return upstreamResponse{}, err
	}
	defer res.Body.Close() //nolint:errcheck // everything needed has been read
	answer, err := io.ReadAll(io.LimitReader(res.Body, r.MaxBody+1))
	if err != nil {
		return upstreamResponse{}, fmt.Errorf("reading the upstream response: %w", err)
	}
	if int64(len(answer)) > r.MaxBody {
		return upstreamResponse{}, fmt.Errorf("%w (%d bytes)", errTooLarge, r.MaxBody)
	}
	return upstreamResponse{status: res.StatusCode, proto: res.Proto, header: res.Header, body: answer}, nil
}

func (r Recorder) target(req *http.Request) *url.URL {
	u := *r.Upstream
	// Clean the call's path on its own first, so ".." cannot climb out of
	// the upstream's path prefix.
	u.Path = path.Join("/", r.Upstream.Path, path.Clean("/"+req.URL.Path))
	if strings.HasSuffix(req.URL.Path, "/") && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	u.RawPath, u.RawQuery = "", req.URL.RawQuery
	return &u
}

// forwardable says which request headers go upstream: not the hop-by-hop
// ones, not the host or encoding negotiation (Go decodes gzip itself, so
// bodies arrive readable), and not the replayer's own steering headers.
func forwardable(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "x-flowi-") {
		return false
	}
	switch lower {
	case "host", "accept-encoding", "content-length", "expect", "connection", "keep-alive",
		"proxy-authorization", "proxy-connection", "te", "trailer", "transfer-encoding", "upgrade":
		return false
	}
	return true
}

// serveRecording forwards one call, answers with the real response and saves
// it under the saved request the call matches, or under a request made of it.
func (h *Handler) serveRecording(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, err := io.ReadAll(io.LimitReader(r.Body, h.rec.MaxBody+1))
	if err == nil && int64(len(body)) > h.rec.MaxBody {
		err = errTooLarge
	}
	var res upstreamResponse
	if err == nil {
		res, err = h.rec.forward(r, body)
	}
	if err != nil {
		a := problem(r, http.StatusBadGateway, MatchUpstreamError, "upstream: "+err.Error())
		h.deliver(w, r, a)
		h.record(r, a, time.Since(start))
		return
	}
	a := answer{status: res.status, match: MatchForwarded}
	if rt, ok := h.routeForRecording(r, body); ok {
		a.route = &rt
		if err := h.saveRecorded(rt, r, res); err != nil {
			h.opts.Logger.Warn("recorded response not saved", "Event", "ReplayRecordFailed", "Request", rt.req.Path, "Error", err)
		} else {
			a.match = MatchRecorded
		}
	}
	h.relay(w, r, a, res)
	h.keepOnTape(r, body, res, time.Since(start))
	h.record(r, a, time.Since(start))
}

// keepOnTape adds the call to the tape; a tape that cannot be written is a warning, never a failed call.
func (h *Handler) keepOnTape(r *http.Request, body []byte, res upstreamResponse, took time.Duration) {
	if h.rec.Tape == nil {
		return
	}
	if err := h.rec.Tape.add(h.tapeRecord(r, body, res, took)); err != nil {
		h.opts.Logger.Warn("call not kept on the tape", "Event", "ReplayTapeFailed", "Error", err)
	}
}

// routeForRecording is the saved request a call is recorded under: the one it
// matches, or, when Capture is set, a new one made of the call. Capturing is
// serialised and looks again first, so two calls for one new path make one
// request.
func (h *Handler) routeForRecording(r *http.Request, body []byte) (route, bool) {
	find := func() (route, bool) {
		look := h.idx.Load().find(r.Method, splitPath(r.URL.Path), r.URL.Query())
		if len(look.candidates) == 0 {
			return route{}, false
		}
		return look.candidates[0].route, true
	}
	if rt, ok := find(); ok || h.rec.Capture == nil {
		return rt, ok
	}
	h.captureMu.Lock()
	defer h.captureMu.Unlock()
	if rt, ok := find(); ok {
		return rt, true
	}
	got, err := h.rec.Capture(Call{Method: r.Method, URL: h.rec.target(r).String(), Header: r.Header, Body: body})
	if err != nil {
		h.opts.Logger.Warn("call not saved as a request", "Event", "ReplayCaptureFailed", "Method", r.Method, "Path", r.URL.Path, "Error", err)
		return route{}, false
	}
	h.Refresh()
	return newRoute(got.Collection, got.Request, got.Version, 0), true
}

// relay answers with the upstream response.
func (h *Handler) relay(w http.ResponseWriter, r *http.Request, a answer, res upstreamResponse) {
	hd := w.Header()
	hd.Set(HeaderMatch, a.match)
	if a.route != nil {
		hd.Set("X-Flowi-Request", a.route.req.Path)
		hd.Set("X-Flowi-Version", a.route.ver.Label)
	}
	for name, values := range res.header {
		for _, v := range values {
			if safeHeader(header{name: name, value: v}) {
				hd.Add(name, v)
			}
		}
	}
	hasBody := res.status != http.StatusNoContent && res.status != http.StatusNotModified
	if hasBody {
		hd.Set("Content-Length", strconv.Itoa(len(res.body)))
	}
	w.WriteHeader(res.status)
	if hasBody && r.Method != http.MethodHead {
		_, _ = w.Write(res.body) // the caller hung up: nothing to do about it
	}
}

// saveRecorded writes the response as <status>.json and last.json.
func (h *Handler) saveRecorded(rt route, r *http.Request, res upstreamResponse) error {
	if h.opts.DataDir == nil {
		return errors.New("no data directory to save into")
	}
	data, err := json.MarshalIndent(h.rec.recordOf(rt, r, res), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the record: %w", err)
	}
	dir := filepath.Join(h.opts.DataDir(rt.coll.Manifest.ID), "responses", string(rt.req.ID), rt.ver.Label)
	for _, name := range []string{strconv.Itoa(res.status) + ".json", "last.json"} {
		if err := writeAtomic(filepath.Join(dir, name), append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (r Recorder) recordOf(rt route, req *http.Request, res upstreamResponse) engine.Record {
	rec := engine.Record{
		ID: "rec-" + strconv.FormatInt(r.Now().UnixNano(), 36), CollectionID: rt.coll.Manifest.ID,
		RequestID: rt.req.ID, RequestPath: rt.req.Path, Version: rt.ver.Label, At: r.Now(),
		Request: engine.SentRequest{Method: req.Method, URL: req.URL.RequestURI(), Header: sentHeaders(req.Header)},
		Status:  res.status, Proto: res.proto, Body: bodyOf(res),
	}
	for name, values := range res.header {
		for _, v := range values {
			rec.Header = append(rec.Header, engine.Header{Name: name, Value: v})
		}
	}
	return rec
}

// sentHeaders are the request headers as they are saved: sorted by name, not
// the replayer's own, and a credential as a {{secret.name}} placeholder, which
// is also what the request made of the call holds.
func sentHeaders(h http.Header) []engine.Header {
	var out []engine.Header
	for name, values := range h {
		if strings.HasPrefix(strings.ToLower(name), "x-flowi-") {
			continue
		}
		for _, v := range values {
			if importers.SensitiveHeader(name) {
				v = "{{secret." + importers.SecretName(name) + "}}"
			}
			out = append(out, engine.Header{Name: name, Value: v})
		}
	}
	slices.SortFunc(out, func(a, b engine.Header) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func bodyOf(res upstreamResponse) engine.Body {
	b := engine.Body{Size: int64(len(res.body)), ContentType: res.header.Get("Content-Type")}
	if len(res.body) == 0 {
		return b
	}
	switch parsed, ok := parseJSON(res.body); {
	case ok && strings.Contains(b.ContentType, "json"):
		b.JSON = parsed
	case utf8.Valid(res.body):
		b.Text = string(res.body)
	default:
		b.Base64 = base64.StdEncoding.EncodeToString(res.body)
	}
	return b
}

// parseJSON reads a whole body as one JSON value, keeping numbers exact.
func parseJSON(body []byte) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil || dec.More() {
		return nil, false
	}
	return v, true
}

// writeAtomic writes through a temp file and a rename, so a replayer
// reading the directory never sees half a file.
func writeAtomic(file string, data []byte) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // removes the temp file only if the rename did not happen
	_, err = io.Copy(tmp, bytes.NewReader(data))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), file)
	}
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}
