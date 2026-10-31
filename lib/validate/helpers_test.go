// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/ids"
	"github.com/githugt/flowi/lib/model"
)

// fixture builds small, valid collections in Go so each test states only
// what it breaks.
type fixture struct {
	t   *testing.T
	ids *ids.Source
}

func newFixture(t *testing.T) *fixture {
	return &fixture{t: t, ids: ids.NewSource(func() time.Time { return time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC) }, rand.NewChaCha8([32]byte{9}))}
}

func (f *fixture) collection(name string) model.Collection {
	root := "/c/" + name
	return model.Collection{
		Root:     root,
		Manifest: model.Manifest{Format: 1, Meta: model.Meta{ID: f.ids.New(), Name: name}, File: root + "/flowi.yaml"},
	}
}

func (f *fixture) request(c *model.Collection, path string, labels ...string) model.Request {
	r := model.Request{Meta: model.Meta{ID: f.ids.New(), Name: path}, Path: path, File: c.Root + "/requests/" + path + ".req.yaml"}
	for _, l := range labels {
		r.Versions = append(r.Versions, model.RequestVersion{Label: l, Method: "GET", URL: "http://x"})
	}
	c.Requests = append(c.Requests, r)
	return r
}

func (f *fixture) flow(c *model.Collection, path string, nodes ...model.Node) model.Flow {
	fl := model.Flow{Meta: model.Meta{ID: f.ids.New(), Name: path}, Path: path, File: c.Root + "/flows/" + path + ".flow.yaml", Nodes: nodes}
	c.Flows = append(c.Flows, fl)
	return fl
}

func requestNode(id string, r model.Request) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: r.ID, Path: r.Path}}}
}

func codesOf(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		out = append(out, d.Code)
	}
	return out
}

func only(t *testing.T, l diag.List, code diag.Code) diag.Diagnostic {
	t.Helper()
	if len(l) != 1 || l[0].Code != code {
		t.Fatalf("got %v, want exactly %s", codesOf(l), code)
	}
	return l[0]
}
