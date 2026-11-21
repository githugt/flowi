// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/model"
)

// Forward is what the replayer does with a call that no saved response can answer (no request fits it, or the one
// that does has no such response): it sends the call to a real API and relays the answer. A call a saved response
// can answer is replayed and never forwarded.
type Forward struct {
	// Save keeps the real answer as a saved response of the request the call matches, so the next such call is
	// replayed. With Recorder.Capture, a call that matches no request (it can only be forwarded to Upstream) also
	// becomes a new request.
	Save bool
	// Upstream, when set, takes every such call. When nil, a call goes to the base URL of the saved request it
	// matches (BaseURL); a call that matches no request has nowhere to go and is answered as not found.
	Upstream *url.URL
	// Only is, when Upstream is nil, the base URLs a call may be sent to: scheme, host and a path prefix. Empty
	// allows the base URL of any saved request.
	Only []*url.URL
	// BaseURL works out the base URL of a saved request, with its variables filled in.
	BaseURL func(coll model.Collection, req model.Request, ver model.RequestVersion) (*url.URL, error)
	// Self is the replayer's own address (host:port). It is never a destination, so a request whose base URL is the
	// replayer's own address cannot loop.
	Self string
	// Recorder supplies the HTTP client, the body limit, the clock and Capture; its Upstream is not used. Its Tape keeps every forwarded call in a HAR file.
	Recorder Recorder
}

// upstreamFor says where a call goes: the upstream, or why it goes nowhere ("" when forwarding simply does not apply).
func (f Forward) upstreamFor(a answer) (up *url.URL, why string) {
	if f.Upstream != nil {
		return f.Upstream, ""
	}
	if a.route == nil {
		return nil, "no saved request matches it, so there is no API to send it to"
	}
	if f.BaseURL == nil {
		return nil, ""
	}
	base, err := f.BaseURL(a.route.coll, a.route.req, a.route.ver)
	if err != nil {
		return nil, "the API of " + a.route.req.Path + " is not known: " + err.Error()
	}
	if f.Self != "" && SameHost(base.Host, f.Self) {
		return nil, "the API of " + a.route.req.Path + " is the replayer itself"
	}
	if len(f.Only) > 0 && !f.allows(base) {
		return nil, "the API of " + a.route.req.Path + " (" + base.String() + ") is not one of the APIs to forward to"
	}
	return base, ""
}

// Allows reports whether base is one of the APIs a call may be forwarded to: Only is empty, or has an entry with the
// same scheme and host whose path is a prefix of base's.
func (f Forward) Allows(base *url.URL) bool { return len(f.Only) == 0 || f.allows(base) }

func (f Forward) allows(base *url.URL) bool {
	for _, o := range f.Only {
		if strings.EqualFold(o.Scheme, base.Scheme) && strings.EqualFold(o.Host, base.Host) && pathWithin(o.Path, base.Path) {
			return true
		}
	}
	return false
}

// pathWithin reports whether path is prefix or below it, by whole segments.
func pathWithin(prefix, p string) bool {
	prefix, p = strings.Trim(prefix, "/"), strings.Trim(p, "/")
	return prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/")
}

// serveForward sends the call to up and relays the answer, saving it when Forward.Save is on.
func (h *Handler) serveForward(w http.ResponseWriter, r *http.Request, start time.Time, up *url.URL) {
	f := h.fwd
	rec := f.Recorder
	rec.Upstream = up
	rec.derived = f.Upstream == nil // the base URL came from a request: calls may already carry its path prefix
	body, err := io.ReadAll(io.LimitReader(r.Body, rec.MaxBody+1))
	if err == nil && int64(len(body)) > rec.MaxBody {
		err = errTooLarge
	}
	var res upstreamResponse
	if err == nil {
		res, err = rec.forward(r, body)
	}
	if err != nil {
		a := problem(r, http.StatusBadGateway, MatchUpstreamError, "upstream: "+err.Error())
		h.deliver(w, r, a)
		h.record(r, a, time.Since(start), nil)
		return
	}
	a := answer{status: res.status, match: MatchPassed}
	if f.Save {
		if rt, ok := h.routeFor(rec, r, body); ok {
			a.route = &rt
			if err := h.saveRecorded(rec, rt, r, res); err != nil {
				h.opts.Logger.Warn("forwarded response not saved", "Event", "ReplayRecordFailed", "Request", rt.req.Path, "Error", err)
			} else {
				a.match = MatchRecorded
			}
		}
	}
	h.relay(w, r, a, res)
	h.keepOnTape(rec, r, body, res, time.Since(start))
	h.record(r, a, time.Since(start), nil)
}

// withReason adds why a call was not forwarded to the answer that says it was not found.
func withReason(a answer, why string) answer {
	if why == "" || a.problem == nil {
		return a
	}
	if detail, ok := a.problem["detail"].(string); ok {
		a.problem["detail"] = detail + ". Not forwarded: " + why
	}
	if msg, ok := a.problem["error"].(string); ok {
		a.problem["error"] = msg + ". Not forwarded: " + why
	}
	return a
}

// BaseOf is the part of a request's URL before its path: a scheme and host, or a leading {{variable}}. It is "" for a
// URL that has neither.
func BaseOf(rawURL string) string {
	raw, _, _ := strings.Cut(strings.TrimSpace(rawURL), "#")
	raw, _, _ = strings.Cut(raw, "?")
	switch {
	case strings.Contains(raw, "://"):
		scheme, rest, _ := strings.Cut(raw, "://")
		host, _, _ := strings.Cut(rest, "/")
		return scheme + "://" + host
	case strings.HasPrefix(raw, "{{"):
		if end := strings.Index(raw, "}}"); end >= 0 {
			return raw[:end+2]
		}
	}
	return ""
}

// callPath is the path a call is forwarded on under up. A base URL that came from a request may have a path prefix
// the call already carries (the replayer matches behind such a prefix), and the prefix is then not added twice.
func callPath(up *url.URL, callURLPath string, derived bool) string {
	clean := path.Clean("/" + callURLPath)
	if derived && up.Path != "" && up.Path != "/" && pathWithin(up.Path, clean) {
		return path.Join("/", clean)
	}
	return path.Join("/", up.Path, clean)
}
