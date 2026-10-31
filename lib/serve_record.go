// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/importers/har"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/replay"
)

// recordedFolder is where requests made of recorded calls go, under requests/.
const recordedFolder = "recorded"

// noisyHeaders are sent by every client and say nothing about the API; a
// request made of a recorded call leaves them out.
func noisyHeaders() []string {
	return []string{"user-agent", "accept-encoding", "accept-language", "connection", "host", "content-length", "origin", "referer", "priority", "upgrade-insecure-requests"}
}

// recordCollection is the collection that recorded calls are saved in:
// the one named (by name or id), or the Scratch draft when name is empty.
func (w *Workspace) recordCollection(name string) (model.ID, error) {
	if name == "" {
		m, err := w.Scratch()
		if err != nil {
			return "", fmt.Errorf("opening the Scratch draft to record into: %w", err)
		}
		return m.ID, nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	cs, err := w.collectionsNamed(name)
	if err != nil {
		return "", err
	}
	return cs[0].Manifest.ID, nil
}

// captureTo is the replay.Capture that saves a call no saved request matches
// as a request of collection. It goes through the HAR importer, so a call
// becomes what a recorded browser session would: ids in the path become
// {{id}} (the next call to another id matches the same request), the query
// becomes query rows, credentials become {{secret.name}} placeholders. No
// response is pinned: the recorder keeps responses in the data dir.
func (w *Workspace) captureTo(collection model.ID) replay.Capture {
	return func(call replay.Call) (replay.Captured, error) {
		doc, err := harOf(call)
		if err != nil {
			return replay.Captured{}, err
		}
		result, diags := har.Import(doc, "recorded call")
		if diags.HasErrors() || len(result.Requests) == 0 {
			return replay.Captured{}, errors.New("the call cannot be saved as a request")
		}
		path := recordedPath(result.Requests[0].Path)
		result.Requests[0].Path = path
		result.Flows, result.Examples = nil, nil // one call is not a recording to replay, and a response is not pinned
		w.mu.RLock()
		c, ok := w.collection(collection)
		w.mu.RUnlock()
		if !ok {
			return replay.Captured{}, fmt.Errorf("the collection to record into is not loaded")
		}
		plan, err := w.planResult(c, result)
		if err != nil {
			return replay.Captured{}, fmt.Errorf("planning the request: %w", err)
		}
		if err := w.ApplyImport(plan, ApplyOptions{SkipExisting: true}); err != nil {
			return replay.Captured{}, fmt.Errorf("saving the request: %w", err)
		}
		return w.capturedAt(collection, path)
	}
}

// capturedAt finds the request saved at path after the reload.
func (w *Workspace) capturedAt(collection model.ID, path string) (replay.Captured, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return replay.Captured{}, errors.New("the collection to record into is not loaded")
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.Path == path })
	if i < 0 || len(c.Requests[i].Versions) == 0 {
		return replay.Captured{}, fmt.Errorf("the saved request %s did not load", path)
	}
	return replay.Captured{Collection: c, Request: c.Requests[i], Version: c.Requests[i].Versions[0]}, nil
}

// recordedPath moves a request the HAR importer put under har/ to recorded/.
func recordedPath(path string) string {
	return recordedFolder + "/" + strings.TrimPrefix(path, "har/")
}

// harOf is a one-entry HAR document holding the call, without a response.
func harOf(call replay.Call) ([]byte, error) {
	type pair struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type post struct {
		MimeType string `json:"mimeType"`
		Text     string `json:"text"`
	}
	type request struct {
		Method      string `json:"method"`
		URL         string `json:"url"`
		Headers     []pair `json:"headers"`
		QueryString []pair `json:"queryString"`
		PostData    *post  `json:"postData,omitempty"`
	}
	req := request{Method: call.Method, URL: call.URL, Headers: []pair{}, QueryString: []pair{}}
	for name, values := range call.Header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-flowi-") || slices.Contains(noisyHeaders(), lower) || strings.HasPrefix(lower, "sec-") {
			continue
		}
		for _, v := range values {
			req.Headers = append(req.Headers, pair{name, v})
		}
	}
	slices.SortFunc(req.Headers, func(a, b pair) int { return strings.Compare(a.Name+"\x00"+a.Value, b.Name+"\x00"+b.Value) })
	if i := strings.Index(call.URL, "?"); i >= 0 {
		for _, kv := range strings.Split(call.URL[i+1:], "&") {
			name, value, _ := strings.Cut(kv, "=")
			req.QueryString = append(req.QueryString, pair{name, value})
		}
	}
	if len(call.Body) > 0 {
		req.PostData = &post{MimeType: http.Header(call.Header).Get("Content-Type"), Text: string(call.Body)}
	}
	return json.Marshal(map[string]any{"log": map[string]any{"entries": []any{map[string]any{"request": req}}}})
}
