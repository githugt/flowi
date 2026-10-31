// SPDX-License-Identifier: Apache-2.0

// Package tapeplayer answers HTTP calls from recorded responses instead of the network, so a run can be shown
// again without the servers it first called.
package tapeplayer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/githugt/flowi/lib/engine"
)

// ErrNotRecorded is returned for a call the recording has no answer for.
var ErrNotRecorded = errors.New("the recording has no response for this call")

// Player is an http.RoundTripper that plays records back. It is safe for concurrent use.
type Player struct {
	mu      sync.Mutex // guards used
	records []engine.Record
	used    []bool
}

// New plays records back. HTTP records only: a message-based record (async, gRPC) is never matched.
func New(records []engine.Record) *Player {
	return &Player{records: records, used: make([]bool, len(records))}
}

// RoundTrip answers with the first unused record for the same method and URL; failing that, the first
// unused one for the same method and path, since a timestamp in the query is the usual reason a replayed
// URL differs. A call with no record is ErrNotRecorded.
func (p *Player) RoundTrip(req *http.Request) (*http.Response, error) {
	p.mu.Lock()
	i := p.take(req)
	p.mu.Unlock()
	if i < 0 {
		return nil, fmt.Errorf("%w: %s %s", ErrNotRecorded, req.Method, req.URL.Redacted())
	}
	return responseOf(p.records[i], req), nil
}

func (p *Player) take(req *http.Request) int {
	for _, exact := range []bool{true, false} {
		for i, rec := range p.records {
			if p.used[i] || rec.HasMessages() || rec.Request.Method != req.Method || !sameCall(rec.Request.URL, req.URL, exact) {
				continue
			}
			p.used[i] = true
			return i
		}
	}
	return -1
}

// sameCall compares a recorded URL with the one being called, with the query or without it.
func sameCall(recorded string, called *url.URL, withQuery bool) bool {
	u, err := url.Parse(recorded)
	if err != nil {
		return false
	}
	if u.Scheme != called.Scheme || u.Host != called.Host || u.EscapedPath() != called.EscapedPath() {
		return false
	}
	return !withQuery || u.RawQuery == called.RawQuery
}

func responseOf(rec engine.Record, req *http.Request) *http.Response {
	body := bodyBytes(rec.Body)
	h := http.Header{}
	for _, hd := range rec.Header {
		h.Add(hd.Name, hd.Value)
	}
	// The recorded body is already decoded, so the headers that describe the wire form no longer apply.
	h.Del("Content-Encoding")
	h.Del("Transfer-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	return &http.Response{
		Status: strconv.Itoa(rec.Status) + " " + http.StatusText(rec.Status), StatusCode: rec.Status,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: h, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: req,
	}
}

func bodyBytes(b engine.Body) []byte {
	switch {
	case b.JSON != nil:
		data, err := json.Marshal(b.JSON)
		if err != nil {
			return nil
		}
		return data
	case b.Base64 != "":
		data, err := base64.StdEncoding.DecodeString(b.Base64)
		if err != nil {
			return nil
		}
		return data
	}
	return []byte(b.Text)
}
