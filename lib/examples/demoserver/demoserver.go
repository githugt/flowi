// SPDX-License-Identifier: Apache-2.0

// Package demoserver is a small API matching the user-creation example
// collection, so the collection (and Flow-I) can be tried without a real
// backend. It also has helper routes for exercising HTTP behaviour:
// redirects, cookies, status codes, compression and large bodies.
//
// It keeps no state: tokens are signed with a key made when the handler
// is created, so restarting the server invalidates them.
package demoserver

import (
	"compress/gzip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ClientID is what GET /v1/config returns and POST /v1/verify requires
// in X-Client-Id.
const ClientID = "demo-client"

// maxBytes bounds /bytes/{n}.
const maxBytes = 64 << 20

// Options configures the demo API.
type Options struct {
	Password string           // the password login accepts
	Clock    func() time.Time // nil → time.Now
}

type api struct {
	opts Options
	key  []byte

	mu    sync.Mutex     // guards polls
	polls map[string]int // status polls per subject
}

// Handler returns the demo API.
func Handler(o Options) http.Handler {
	if o.Clock == nil {
		o.Clock = time.Now
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key) // crypto/rand.Read does not fail on supported platforms
	a := &api{opts: o, key: key, polls: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/login", a.login(loginV1))
	mux.HandleFunc("POST /v2/login", a.login(loginV2))
	mux.HandleFunc("POST /v1/verify", a.authed(a.verify))
	mux.HandleFunc("PUT /v1/onboarding/{subject}/personal", a.authed(a.onboarding("firstName", "lastName")))
	mux.HandleFunc("PUT /v1/onboarding/{subject}/address", a.authed(a.onboarding("city")))
	mux.HandleFunc("GET /v1/onboarding/{subject}/status", a.authed(a.status))
	mux.HandleFunc("GET /v1/config", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"client_id": ClientID})
	})
	addHelpers(mux)
	return mux
}

type loginShape func(token, sub string) any

func loginV1(token, sub string) any {
	return map[string]any{"data": map[string]any{"token": token, "sub": sub}}
}

func loginV2(token, sub string) any {
	return map[string]any{"token": token, "user": map[string]any{"id": sub}}
}

func (a *api) login(shape loginShape) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Email, Password string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "send {email, password}"})
			return
		}
		if !hmac.Equal([]byte(body.Password), []byte(a.opts.Password)) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "wrong password"})
			return
		}
		sub := "u-" + a.sign(body.Email)[:12]
		writeJSON(w, http.StatusOK, shape(a.token(sub), sub))
	}
}

// token is "<subject>.<signature>".
func (a *api) token(sub string) string { return sub + "." + a.sign(sub) }

func (a *api) sign(s string) string {
	m := hmac.New(sha256.New, a.key)
	m.Write([]byte(s))
	return hex.EncodeToString(m.Sum(nil))
}

// authed requires a valid bearer token and passes the subject on.
func (a *api) authed(next func(w http.ResponseWriter, r *http.Request, sub string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		sub, sig, _ := strings.Cut(token, ".")
		if !ok || !hmac.Equal([]byte(sig), []byte(a.sign(sub))) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "missing or invalid bearer token"})
			return
		}
		next(w, r, sub)
	}
}

func (a *api) verify(w http.ResponseWriter, r *http.Request, sub string) {
	var body struct{ Subject, Code string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Subject != sub || body.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "send {subject, code} for the token's subject"})
		return
	}
	if r.Header.Get("X-Client-Id") != ClientID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "X-Client-Id must be " + ClientID + " (see GET /v1/config)"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) onboarding(fields ...string) func(http.ResponseWriter, *http.Request, string) {
	return func(w http.ResponseWriter, r *http.Request, sub string) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.PathValue("subject") != sub {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "send JSON for your own subject"})
			return
		}
		for _, f := range fields {
			if s, _ := body[f].(string); s == "" {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": f + " is required"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"subject": sub, "saved": body})
	}
}

// readyAfter is how many status polls see "pending" before "ready", so
// run_till examples have something to wait for.
const readyAfter = 2

// status reports onboarding as pending for the first polls of a subject,
// then ready.
func (a *api) status(w http.ResponseWriter, r *http.Request, sub string) {
	if r.PathValue("subject") != sub {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "ask about your own subject"})
		return
	}
	a.mu.Lock()
	a.polls[sub]++
	n := a.polls[sub]
	a.mu.Unlock()
	state := "pending"
	if n > readyAfter {
		state = "ready"
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": sub, "state": state})
}

// addHelpers registers routes for exercising HTTP behaviour.
func addHelpers(mux *http.ServeMux) {
	mux.HandleFunc("GET /redirect/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.PathValue("n"))
		if n <= 0 {
			_, _ = io.WriteString(w, "arrived")
			return
		}
		http.Redirect(w, r, "/redirect/"+strconv.Itoa(n-1), http.StatusFound)
	})
	mux.HandleFunc("GET /cookie/set", func(w http.ResponseWriter, r *http.Request) {
		value := r.URL.Query().Get("value")
		if value == "" {
			value = "chocolate"
		}
		// Not Secure: the demo serves plain HTTP on localhost.
		http.SetCookie(w, &http.Cookie{Name: "flavour", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}) //nolint:gosec // see above
		writeJSON(w, http.StatusOK, map[string]any{"set": value})
	})
	mux.HandleFunc("GET /cookie/echo", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("flavour")
		value := ""
		if err == nil {
			value = c.Value
		}
		writeJSON(w, http.StatusOK, map[string]any{"flavour": value})
	})
	mux.HandleFunc("/status/{code}", func(w http.ResponseWriter, r *http.Request) {
		code, err := strconv.Atoi(r.PathValue("code"))
		if err != nil || code < 100 || code > 599 {
			code = http.StatusBadRequest
		}
		w.WriteHeader(code)
	})
	mux.HandleFunc("GET /bytes/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.PathValue("n"))
		if err != nil || n < 0 || n > maxBytes {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("n must be 0..%d", maxBytes)})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(n))
		_, _ = io.CopyN(w, deterministic{}, int64(n))
	})
	mux.HandleFunc("GET /gzip", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		_, _ = io.WriteString(zw, `{"compressed":true}`)
		_ = zw.Close()
	})
	mux.HandleFunc("/echo", echo)
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(slowDelay(r)):
			_, _ = io.WriteString(w, "finally")
		}
	})
}

// slowDelay is ?ms=… (default 30s), for trying cancellation and timeouts.
func slowDelay(r *http.Request) time.Duration {
	if ms, err := strconv.Atoi(r.URL.Query().Get("ms")); err == nil && ms >= 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return 30 * time.Second
}

// echo returns the request as JSON: method, path, query, headers, body.
func echo(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	headers := map[string]string{}
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}
	writeJSON(w, http.StatusOK, map[string]any{"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "headers": headers, "body": string(body)})
}

// deterministic yields the same bytes every time (i*31 mod 251).
type deterministic struct{}

func (deterministic) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(i * 31 % 251)
	}
	return len(p), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client went away; nothing to do
}
