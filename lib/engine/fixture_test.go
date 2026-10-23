// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/vault"
)

const (
	collID  model.ID = "01JA0000000000000000000001"
	envID   model.ID = "01JA0000000000000000000002"
	wsID    model.ID = "01JA00000000000000000000W1"
	secretV          = "hunter2"
)

// fakeLookup serves in-memory collections the way the facade will.
type fakeLookup struct{ collections []model.Collection }

func (l *fakeLookup) Collection(id model.ID) (model.Collection, bool) {
	for _, c := range l.collections {
		if c.Manifest.ID == id {
			return c, true
		}
	}
	return model.Collection{}, false
}

func (l *fakeLookup) Request(ref model.Ref, _ model.ID) (model.Collection, model.Request, bool) {
	for _, c := range l.collections {
		for _, r := range c.Requests {
			if r.ID == ref.ID {
				return c, r, true
			}
		}
	}
	return model.Collection{}, model.Request{}, false
}

func (l *fakeLookup) Flow(ref model.Ref, _ model.ID) (model.Collection, model.Flow, bool) {
	for _, c := range l.collections {
		for _, f := range c.Flows {
			if f.ID == ref.ID {
				return c, f, true
			}
		}
	}
	return model.Collection{}, model.Flow{}, false
}

func (l *fakeLookup) Settings(collection model.ID, folder string, version model.Settings) settings.Resolved {
	schema := settings.Schema()
	var layers []settings.LayerValues
	if c, ok := l.Collection(collection); ok {
		lv, _ := settings.FromBlock(schema, settings.LayerCollection, "flowi.yaml", c.Manifest.Settings)
		layers = append(layers, lv)
		for _, f := range c.Folders {
			if folder == f.Dir || strings.HasPrefix(folder, f.Dir+"/") {
				fl, _ := settings.FromBlock(schema, settings.LayerFolder, f.File, f.Settings)
				layers = append(layers, fl)
			}
		}
	}
	rl, _ := settings.FromBlock(schema, settings.LayerRequest, "request", version)
	return settings.Resolve(schema, append(layers, rl)...)
}

// fixture is a collection, a test API and a vault.
type fixture struct {
	t       *testing.T
	srv     *httptest.Server
	hits    atomic.Int32
	lookup  *fakeLookup
	vault   *vault.Vault
	dataDir string
	release chan struct{}
	environ []string
	seq     atomic.Int64

	transports transport.Registry // injected into Deps; nil → every real protocol
	grpc       GRPCRunner         // injected into Deps; nil → the real gRPC client

	hold chan struct{} // /hold answers once this is closed
	held chan struct{} // /hold reports each arrival here

	echoMu      sync.Mutex
	echoHeaders map[string]string // the headers /echo/ last received
	counters    map[string]int    // /counter/{name} calls, guarded by echoMu
	echoBodies  []string          // bodies /echo/ received, guarded by echoMu
}

// count increments and returns a named counter.
func (f *fixture) count(name string) int {
	f.echoMu.Lock()
	defer f.echoMu.Unlock()
	if f.counters == nil {
		f.counters = map[string]int{}
	}
	f.counters[name]++
	return f.counters[name]
}

// lastEcho returns the headers /echo/ last received.
func (f *fixture) lastEcho() map[string]string {
	f.echoMu.Lock()
	defer f.echoMu.Unlock()
	return f.echoHeaders
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, dataDir: t.TempDir(), release: make(chan struct{}), hold: make(chan struct{}), held: make(chan struct{}, 8)}
	f.srv = httptest.NewServer(f.handler())
	t.Cleanup(func() { close(f.release); f.srv.Close() })
	coll := model.Collection{
		Root:     t.TempDir(),
		Manifest: model.Manifest{Format: 1, Meta: model.Meta{ID: collID, Name: "Test"}, Settings: model.Settings{}},
		Environments: []model.Environment{{
			Meta: model.Meta{ID: envID, Name: "local"},
			Vars: []model.EnvVar{{Name: "base_url", Value: f.srv.URL}, {Name: "password", Secret: true}, {Name: "api_key", Secret: true}},
		}},
	}
	f.lookup = &fakeLookup{collections: []model.Collection{coll}}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "vault.enc"), WorkspaceID: wsID, Keys: &memKeys{}})
	if err := f.vault.Set(vault.Entry{Collection: collID, Environment: envID, Name: "password"}, secretV); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Password != secretV {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"data": map[string]any{"token": "t-1", "sub": "u1"}})
	})
	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.echoMu.Lock()
		f.echoBodies = append(f.echoBodies, string(body))
		f.echoMu.Unlock()
		headers := map[string]string{}
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}
		f.echoMu.Lock()
		f.echoHeaders = headers
		f.echoMu.Unlock()
		writeJSON(w, map[string]any{"path": r.URL.Path, "query": r.URL.RawQuery, "headers": headers, "body": string(body)})
	})
	mux.HandleFunc("/echo-secret", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "you sent "+secretV) })
	mux.HandleFunc("/status/{code}", func(w http.ResponseWriter, r *http.Request) {
		var code int
		_, _ = fmt.Sscan(r.PathValue("code"), &code)
		w.WriteHeader(code)
	})
	mux.HandleFunc("/slow", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-f.release:
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/cookie/set", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s-1", Path: "/"})
	})
	mux.HandleFunc("/hold", func(_ http.ResponseWriter, r *http.Request) {
		f.held <- struct{}{}
		select {
		case <-f.hold:
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/cookie/set/{v}", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: r.PathValue("v"), Path: "/"})
	})
	mux.HandleFunc("/counter/{name}", func(w http.ResponseWriter, r *http.Request) {
		n := f.count(r.PathValue("name"))
		writeJSON(w, map[string]any{"n": n})
	})
	mux.HandleFunc("/cookie/echo", func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("sid")
		if c != nil {
			_, _ = io.WriteString(w, c.Value)
		}
	})
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"token": "cfg-token", "inner": r.Header.Get("X-Inner")})
	})
	mux.HandleFunc("/binary", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0xff, 0xfe, 0x00, 0x80})
	})
	mux.HandleFunc("/badjson", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{not json")
	})
	mux.HandleFunc("/bignum", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"z":1,"id":9007199254740993,"ratio":0.5}`)
	})
	mux.HandleFunc("/nocontent", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		mux.ServeHTTP(w, r)
	})
}

// memKeys is an in-memory keychain (fast: no Argon2 derivation).
type memKeys struct {
	mu   sync.Mutex
	keys map[string]string
}

func (k *memKeys) Get(service, account string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.keys[service+account]
	if !ok {
		return "", vault.ErrKeyNotFound
	}
	return v, nil
}

func (k *memKeys) Set(service, account, secret string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.keys == nil {
		k.keys = map[string]string{}
	}
	k.keys[service+account] = secret
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// addRequest adds a request; its id derives from path.
func (f *fixture) addRequest(p string, versions ...model.RequestVersion) model.Request {
	f.t.Helper()
	for i := range versions {
		if versions[i].Label == "" {
			versions[i].Label = fmt.Sprint(i + 1)
		}
	}
	r := model.Request{Meta: model.Meta{ID: idFor(p), Name: path.Base(p)}, Path: p, Versions: versions,
		File: filepath.Join(f.coll().Root, "requests", p+".req.yaml")}
	f.update(func(c *model.Collection) { c.Requests = append(c.Requests, r) })
	return r
}

func (f *fixture) addFlow(p string, nodes ...model.Node) model.Flow {
	fl := model.Flow{Meta: model.Meta{ID: idFor("flow:" + p), Name: p}, Path: p, Nodes: nodes}
	f.update(func(c *model.Collection) { c.Flows = append(c.Flows, fl) })
	return fl
}

func idFor(p string) model.ID {
	var sum uint64
	for _, c := range p {
		sum = sum*31 + uint64(c)
	}
	return model.ID(fmt.Sprintf("01JB%022d", sum%1_000_000_000_000_000_000))
}

func (f *fixture) coll() model.Collection { return f.lookup.collections[0] }

func (f *fixture) update(change func(*model.Collection)) {
	change(&f.lookup.collections[0])
}

func (f *fixture) deps() Deps {
	return Deps{
		Lookup: f.lookup, Vault: f.vault, Environ: f.environ,
		DataDir: func(id model.ID) string { return filepath.Join(f.dataDir, string(id)) },
		Clock:   time.Now, Transports: f.transports, GRPC: f.grpc,
		NewID: func() string { return fmt.Sprintf("01JC%022d", f.seq.Add(1)) },
	}
}

func (f *fixture) input(r model.Request) SendInput {
	// Re-read the request so tests can add hooks after creating it.
	for _, cur := range f.coll().Requests {
		if cur.ID == r.ID {
			r = cur
		}
	}
	return SendInput{Collection: f.coll(), Request: r, Env: "local"}
}

// send runs a send to completion, collecting its events.
func (f *fixture) send(in SendInput) (SendResult, []Event) {
	f.t.Helper()
	return f.sendCtx(context.Background(), in)
}

func (f *fixture) sendCtx(ctx context.Context, in SendInput) (SendResult, []Event) {
	f.t.Helper()
	s := StartSend(ctx, f.deps(), in)
	var events []Event
	for ev := range s.Events() {
		events = append(events, ev)
	}
	return s.Wait(), events
}

func get(url string) model.RequestVersion {
	return model.RequestVersion{Method: "GET", URL: "{{env.base_url}}" + url}
}

func inlineHook(nodes ...model.Node) model.HookStep { return model.HookStep{Inline: nodes} }

func setNode(id, target, value string) model.Node {
	return model.Node{ID: id, Type: "set", Params: []model.Field{{Key: "target", Value: target}, {Key: "value", Value: value}}}
}

func assertNode(id, check string) model.Node {
	return model.Node{ID: id, Type: "assert", Params: []model.Field{{Key: "check", Value: check}}}
}
