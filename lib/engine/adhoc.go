// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"path"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/internal/nodes"
)

// HTTP makes an ad-hoc call (fetch, webhook, digest webhook) with the
// flow's settings, the frame's cookie jar and the run's limits.
func (rt *nodeRuntime) HTTP(ctx context.Context, c nodes.HTTPCall) nodes.HTTPResult {
	return rt.r.adhoc(ctx, rt.f, rt.path, c)
}

// Background sends c without waiting (webhook wait: false).
//
// Goroutine ownership: the run owns it through r.bg and waits for it
// before RunFinished; it ends with the call, which the request timeout
// bounds, or when the run's context is cancelled.
func (rt *nodeRuntime) Background(node string, c nodes.HTTPCall) {
	r, f, p := rt.r, rt.f, rt.path
	r.bg.Add(1)
	go func() {
		defer r.bg.Done()
		res := r.adhoc(r.ctx, f, p, c)
		if res.Status == nodes.StatusPassed || res.Status == nodes.StatusCancelled {
			return
		}
		r.warn(diag.New(diag.CodeRunBackground, diag.Location{}, map[string]string{"node": node, "detail": firstMessage(res.Diags)}))
	}()
}

func (r *runner) adhoc(ctx context.Context, f *frame, nodePath string, c nodes.HTTPCall) nodes.HTTPResult {
	cfg := newConfig(r.deps.Lookup.Settings(f.fs.collection.Manifest.ID, scopeFolder(f.fs), nil))
	req, err := adhocRequest(c)
	if err != nil {
		return failedCall(diag.New(diag.CodeRequestHeader, diag.Location{}, map[string]string{"name": "body", "detail": err.Error()}))
	}
	r.addTraceparent(req.Header, nodePath)
	o := r.baseOptions(cfg, f.cookieJar(), path.Join(r.deps.DataDir(f.fs.collection.Manifest.ID), "downloads", "adhoc"))
	o.BeforeAttempt, o.AfterAttempt = r.attemptEvents(attemptInfo{path: nodePath, method: req.Method, url: req.URL, depth: f.depth})
	client, err := httpx.NewClient(o)
	if err != nil {
		return failedCall(diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()}))
	}
	defer client.Close()
	attempts, err := client.Do(ctx, req)
	if err != nil || len(attempts) == 0 {
		return nodes.HTTPResult{Status: nodes.StatusCancelled}
	}
	last := attempts[len(attempts)-1]
	if last.Err != nil {
		return failedCall(maskDiag(transportDiag(last.Err), r.secrets))
	}
	return nodes.HTTPResult{Status: nodes.StatusPassed, Response: nodeResponse(last.Response)}
}

// scopeFolder is the settings folder of a flow scope (flows/<dir>).
func scopeFolder(fs *flowScope) string {
	if fs.flow == nil {
		return "flows"
	}
	return flowFolder(*fs.flow)
}

func failedCall(d diag.Diagnostic) nodes.HTTPResult {
	return nodes.HTTPResult{Status: nodes.StatusError, Diags: diag.List{d}}
}

// adhocRequest builds the request: query appended in name order, a string
// body sent as text, anything else as JSON.
func adhocRequest(c nodes.HTTPCall) (httpx.Request, error) {
	req := httpx.Request{Method: c.Method, URL: withQuery(c.URL, c.Query), Header: http.Header{}}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	switch b := c.Body.(type) {
	case nil:
	case string:
		req.Body = httpx.BytesBody([]byte(b), "text/plain; charset=utf-8")
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return httpx.Request{}, err
		}
		req.Body = httpx.BytesBody(data, "application/json")
	}
	return req, nil
}

func withQuery(raw string, q map[string]string) string {
	if len(q) == 0 {
		return raw
	}
	values := url.Values{}
	for _, k := range slices.Sorted(maps.Keys(q)) {
		values.Set(k, q[k])
	}
	sep := "?"
	if u, err := url.Parse(raw); err == nil && u.RawQuery != "" {
		sep = "&"
	}
	return raw + sep + values.Encode()
}

// nodeResponse is a response as nodes see it: JSON bodies parsed.
func nodeResponse(resp *httpx.Response) *nodes.Response {
	b := responseBody(resp)
	var body any = b.Text
	if b.JSON != nil {
		body = scopeValue(b.JSON)
	}
	return &nodes.Response{Status: resp.Status, Header: resp.Header, Body: body, Text: b.Text, Time: resp.Timings.Total}
}

func firstMessage(l diag.List) string {
	for _, d := range l {
		msg, _ := diag.English().Message(d)
		return msg
	}
	return "failed"
}
