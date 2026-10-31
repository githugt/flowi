// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
)

// RequestSchemer is an optional part of a SchemaChecker: it gives the schema kept for the body a request sends,
// for a fuzz step to read the bounds from. ok is false when none is kept.
type RequestSchemer interface {
	RequestSchema(c model.Collection, r model.Request, version string) (schema map[string]any, ok bool)
}

var errNoJSONBody = errors.New("the request has no JSON body to make bad ones from")

// FuzzSubject renders the request as it would be sent, without sending it, and hands back its JSON body and the
// body schema kept for it. A request with another kind of body cannot be fuzzed.
func (rt *nodeRuntime) FuzzSubject(_ context.Context, c nodes.RequestCall) (nodes.FuzzSubject, error) {
	coll, req, ok := rt.r.deps.Lookup.Request(c.Ref, rt.f.fs.collection.Manifest.ID)
	if !ok {
		return nodes.FuzzSubject{}, errors.New("the request does not exist")
	}
	overlay, diags := overlayOfCall(coll, req, firstOf(c.Overlay, rt.r.overlay))
	if diags.HasErrors() {
		return nodes.FuzzSubject{}, firstProblem(diags)
	}
	with := c.With
	if with == nil {
		with = map[string]any{}
	}
	x, diags := rt.r.prepare(call{collection: coll, request: req, pinned: c.Version, flowPrefer: rt.f.fs.prefer, withValues: with,
		depth: rt.f.depth, path: rt.path, headers: c.Headers, overlay: overlay})
	if diags.HasErrors() {
		return nodes.FuzzSubject{}, firstProblem(diags)
	}
	b, diags := x.render(false)
	if diags.HasErrors() {
		return nodes.FuzzSubject{}, firstProblem(diags)
	}
	body, err := decodeJSONBody(b.bodyText)
	if err != nil {
		return nodes.FuzzSubject{}, err
	}
	subject := nodes.FuzzSubject{Body: body}
	if s, ok := rt.r.deps.Schemas.(RequestSchemer); ok {
		subject.Schema, _ = s.RequestSchema(coll, req, x.version.Label)
	}
	return subject, nil
}

func decodeJSONBody(text string) (any, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errNoJSONBody
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, errNoJSONBody
	}
	return v, nil
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// firstProblem is the first error of a list as an error, for a step that cannot go on.
func firstProblem(l diag.List) error {
	for _, d := range l {
		if d.Severity == diag.SeverityError {
			msg, _ := diag.English().Message(d)
			return errors.New(msg)
		}
	}
	return l.Err()
}
