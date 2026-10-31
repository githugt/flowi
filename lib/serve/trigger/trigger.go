// SPDX-License-Identifier: Apache-2.0

// Package trigger is the HTTP interface that starts flow runs:
//
//	POST /flows/<flow>/run[?collection=<c>][&wait=<duration>]
//	GET  /runs/<id>
//	GET  /healthz
//
// Every route but /healthz needs "Authorization: Bearer <token>". The
// token is checked before the path is looked at, so a caller without it
// learns nothing about which flows exist. An address that sends ten bad
// tokens in a minute is answered 429 until the minute ends.
//
// The endpoint speaks plain HTTP, so the token crosses the network in the
// clear: keep it on loopback, or put a TLS-terminating proxy in front.
package trigger

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/serve/runs"
)

// ErrNoToken is returned by New when the token is empty: triggers without
// authentication are never offered.
var ErrNoToken = errors.New("triggers need a token")

const (
	maxBody        = 1 << 20
	defaultMaxWait = 5 * time.Minute
	retryAfter     = "5"
	maxMessages    = 20
)

// Options configure the handler.
type Options struct {
	Token   string
	Runs    *runs.Manager
	MaxWait time.Duration // longest ?wait= honoured (default 5m)
	Logger  *slog.Logger
	Now     func() time.Time // for the failure window (default time.Now)
}

type handler struct {
	digest  [sha256.Size]byte // of the token, so comparison time does not depend on its length
	runs    *runs.Manager
	maxWait time.Duration
	log     *slog.Logger
	denied  *failureLimiter
}

// New returns the trigger handler.
func New(o Options) (http.Handler, error) {
	if o.Token == "" {
		return nil, ErrNoToken
	}
	if o.MaxWait <= 0 {
		o.MaxWait = defaultMaxWait
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &handler{digest: sha256.Sum256([]byte(o.Token)), runs: o.Runs, maxWait: o.MaxWait, log: o.Logger, denied: newFailureLimiter(o.Now)}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	remote := remoteHost(r)
	if h.denied.locked(remote) {
		h.log.Warn("trigger call refused", "Event", "TriggerLockedOut", "Remote", r.RemoteAddr)
		w.Header().Set("Retry-After", strconv.Itoa(int(failureWindow/time.Second)))
		writeError(w, http.StatusTooManyRequests, "too many failed attempts; try again later")
		return
	}
	if !h.authorised(r) {
		h.denied.recordFailure(remote)
		h.log.Warn("trigger call denied", "Event", "TriggerDenied", "Remote", r.RemoteAddr, "Method", r.Method)
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "a bearer token is required")
		return
	}
	switch flow, ok := flowRoute(r.URL.Path); {
	case ok && r.Method == http.MethodPost:
		h.start(w, r, flow)
	case ok:
		methodNotAllowed(w, http.MethodPost)
	case strings.HasPrefix(r.URL.Path, "/runs/") && r.Method == http.MethodGet:
		h.report(w, strings.TrimPrefix(r.URL.Path, "/runs/"))
	case strings.HasPrefix(r.URL.Path, "/runs/"):
		methodNotAllowed(w, http.MethodGet)
	default:
		writeError(w, http.StatusNotFound, "no such route")
	}
}

func (h *handler) authorised(r *http.Request) bool {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	got := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(got[:], h.digest[:]) == 1
}

// flowRoute extracts <flow> from /flows/<flow>/run. A flow path may hold
// slashes (users/signup).
func flowRoute(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/flows/")
	if !ok {
		return "", false
	}
	flow, ok := strings.CutSuffix(rest, "/run")
	return flow, ok
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "use "+allow)
}

// body is the optional JSON body of a trigger call.
type body struct {
	Env           string         `json:"env"`
	Inputs        map[string]any `json:"inputs"`
	PreferVersion string         `json:"prefer_version"`
	Seed          uint64         `json:"seed"`
}

func (h *handler) start(w http.ResponseWriter, r *http.Request, flow string) {
	if flow == "" {
		writeError(w, http.StatusNotFound, "no flow named in the path")
		return
	}
	b, err := readBody(w, r)
	if err != nil {
		writeBodyError(w, err)
		return
	}
	wait, err := parseWait(r.URL.Query().Get("wait"), h.maxWait)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := h.runs.Start(runs.Request{
		Flow: flow, Collection: r.URL.Query().Get("collection"), Env: b.Env, PreferVersion: b.PreferVersion,
		Inputs: stringInputs(b.Inputs), Seed: b.Seed, Source: "trigger",
	})
	if err != nil {
		h.refuse(w, flow, err)
		return
	}
	h.log.Info("trigger accepted", "Event", "TriggerAccepted", "RunID", rec.ID, "Flow", flow, "Remote", r.RemoteAddr)
	w.Header().Set("Location", "/runs/"+rec.ID)
	if wait > 0 {
		ctx, cancel := contextWithWait(r, wait)
		defer cancel()
		if done, ok := h.runs.Await(ctx, rec.ID); ok && done.Status != runs.StatusRunning {
			writeJSON(w, http.StatusOK, view(done))
			return
		}
	}
	writeJSON(w, http.StatusAccepted, view(rec))
}

// reportLookup bounds asking other instances about a run.
const reportLookup = 3 * time.Second

func (h *handler) report(w http.ResponseWriter, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), reportLookup)
	defer cancel()
	rec, ok := h.runs.Find(ctx, id)
	if !ok {
		writeError(w, http.StatusNotFound, "no such run (it may be too old to remember)")
		return
	}
	writeJSON(w, http.StatusOK, view(rec))
}

// refuse answers a start that did not happen. Only messages written for
// users are shown; anything else is logged and answered generically.
func (h *handler) refuse(w http.ResponseWriter, flow string, err error) {
	var se *runs.StartError
	switch {
	case errors.Is(err, runs.ErrBusy):
		w.Header().Set("Retry-After", retryAfter)
		writeError(w, http.StatusTooManyRequests, "too many runs in flight; try again shortly")
	case errors.Is(err, runs.ErrClosed):
		writeError(w, http.StatusServiceUnavailable, "the server is shutting down")
	case errors.As(err, &se) && se.Kind == runs.KindNotFound:
		writeError(w, http.StatusNotFound, se.Message)
	case errors.As(err, &se) && se.Kind == runs.KindAmbiguous:
		writeError(w, http.StatusConflict, se.Message)
	case errors.As(err, &se) && se.Kind == runs.KindInvalid:
		writeError(w, http.StatusBadRequest, se.Message)
	default:
		h.log.Error("trigger could not start a run", "Event", "TriggerFailed", "Flow", flow, "Error", err)
		writeError(w, http.StatusInternalServerError, "the run could not be started")
	}
}

type bodyError struct {
	status int
	msg    string
}

func (e *bodyError) Error() string { return e.msg }

func writeBodyError(w http.ResponseWriter, err error) {
	var be *bodyError
	if errors.As(err, &be) {
		writeError(w, be.status, be.msg)
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func readBody(w http.ResponseWriter, r *http.Request) (body, error) {
	var b body
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return b, nil // no body is fine
		case errors.As(err, &tooBig):
			return b, &bodyError{http.StatusRequestEntityTooLarge, fmt.Sprintf("the body is larger than %d bytes", maxBody)}
		}
		return b, &bodyError{http.StatusBadRequest, "the body is not valid: " + err.Error()}
	}
	if dec.More() {
		return b, &bodyError{http.StatusBadRequest, "the body has content after the JSON object"}
	}
	for name, v := range b.Inputs {
		if name == "" {
			return b, &bodyError{http.StatusBadRequest, "an input has an empty name"}
		}
		switch v.(type) {
		case string, float64, bool:
		default:
			return b, &bodyError{http.StatusBadRequest, fmt.Sprintf("input %q must be text, a number or true/false", name)}
		}
	}
	return b, nil
}

// stringInputs renders scalar JSON values as the text a template would hold.
func stringInputs(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch x := v.(type) {
		case string:
			out[k] = x
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			out[k] = strconv.FormatBool(x)
		}
	}
	return out
}

func parseWait(raw string, limit time.Duration) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("wait must be a positive duration such as 30s, got %q", raw)
	}
	return min(d, limit), nil
}
