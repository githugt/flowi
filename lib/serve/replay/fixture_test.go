// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/serve/replay"
)

// ver is one version of a fixture request.
type ver struct {
	label, method, url string
	query              []model.KeyValue
}

// spec describes one request and the response files next to it.
type spec struct {
	path     string
	id       string // overrides the generated request id
	def      string
	versions []ver
	examples map[string]string            // file name → JSON text
	saved    map[string]map[string]string // version → file name → JSON text
}

func get(url string) []ver { return []ver{{label: "1", method: "GET", url: url}} }

type fixture struct {
	t       *testing.T
	dir     string
	specs   []spec
	sleeps  []time.Duration
	mu      sync.Mutex
	h       *replay.Handler
	options replay.Options
}

func newFixture(t *testing.T, specs ...spec) *fixture {
	t.Helper()
	f := &fixture{t: t, dir: t.TempDir(), specs: specs}
	f.options = replay.Options{
		Collections: f.collections,
		DataDir:     func(id model.ID) string { return filepath.Join(f.dir, "data", string(id)) },
		Sleep: func(_ context.Context, d time.Duration) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.sleeps = append(f.sleeps, d)
			return nil
		},
	}
	return f
}

// start builds the handler (call after setting f.options).
func (f *fixture) start() *fixture {
	f.h = replay.New(f.options)
	return f
}

func (f *fixture) write(file, text string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) collections() []model.Collection {
	c := model.Collection{Root: filepath.Join(f.dir, "coll"), Manifest: model.Manifest{Meta: model.Meta{ID: "C1", Name: "shop"}}}
	for i, s := range f.specs {
		file := filepath.Join(c.Root, "requests", filepath.FromSlash(s.path)+".req.yaml")
		r := model.Request{Meta: model.Meta{ID: model.ID("R" + string(rune('A'+i)))}, Path: s.path, File: file, Default: s.def}
		if s.id != "" {
			r.ID = model.ID(s.id)
		}
		for _, v := range s.versions {
			r.Versions = append(r.Versions, model.RequestVersion{Label: v.label, Method: v.method, URL: v.url, Query: v.query})
		}
		c.Requests = append(c.Requests, r)
	}
	return []model.Collection{c}
}

// materialise writes the response files; call once after newFixture.
func (f *fixture) materialise() *fixture {
	for i, s := range f.specs {
		base := filepath.Join(f.dir, "coll", "requests", filepath.FromSlash(s.path))
		for name, body := range s.examples {
			f.write(base+".examples/"+name+".json", body)
		}
		id := "R" + string(rune('A'+i))
		for v, files := range s.saved {
			for name, body := range files {
				f.write(filepath.Join(f.dir, "data", "C1", "responses", id, v, name+".json"), body)
			}
		}
	}
	return f
}

type reply struct {
	status int
	header http.Header
	body   string
}

func (f *fixture) do(method, target string, headers ...string) reply {
	f.t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return reply{status: res.StatusCode, header: res.Header, body: string(body)}
}

func (r reply) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.body), &m); err != nil {
		t.Fatalf("body %q: %v", r.body, err)
	}
	return m
}

func ex(status int, body string) string {
	return `{"status": ` + strings.TrimSpace(itoa(status)) + `, "body": ` + body + `}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (f *fixture) doRequest(req *http.Request) reply {
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return reply{status: res.StatusCode, header: res.Header, body: string(body)}
}
