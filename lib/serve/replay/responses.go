// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// Where a response came from (X-Flowi-Source).
const (
	SourceExample = "example"
	SourceSaved   = "saved"
)

// maxResponseFile bounds a response file the replayer will read.
const maxResponseFile = 64 << 20

type header struct{ name, value string }

// response is one answer the replayer can give.
type response struct {
	name   string // the file's name without .json: what X-Flowi-Example picks
	source string
	status int
	header []header
	body   []byte
	kind   bodyKind
}

type bodyKind int

const (
	bodyJSON bodyKind = iota
	bodyText
	bodyBytes
)

type headerLine struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// exampleFile is the pinned example format (engine.PinExample).
type exampleFile struct {
	Version string       `json:"version"`
	Status  int          `json:"status"`
	Headers []headerLine `json:"headers"`
	// Body stays raw: decoding into any would turn big integers into
	// floats and reorder keys.
	Body json.RawMessage `json:"body"`
}

// savedFile is the part of a saved response record (engine.Record) the
// replayer answers with.
type savedFile struct {
	Status  int          `json:"status"`
	Headers []headerLine `json:"headers"`
	Body    struct {
		JSON   json.RawMessage `json:"json"`
		Text   string          `json:"text"`
		Base64 string          `json:"base64"`
	} `json:"body"`
}

// responsesFor lists what a route can answer with: pinned examples first
// (they are curated), then saved responses, each sorted by file name. Files
// that do not parse are skipped and reported to bad.
func responsesFor(r route, dataDir func(model.ID) string, bad func(file string, err error)) []response {
	var out []response
	if r.req.File != "" {
		dir := strings.TrimSuffix(r.req.File, ".req.yaml") + ".examples"
		out = append(out, readDir(dir, SourceExample, func(data []byte, name string) (response, bool, error) {
			return parseExample(data, name, r.ver.Label)
		}, bad)...)
	}
	if dataDir != nil {
		dir := filepath.Join(dataDir(r.coll.Manifest.ID), "responses", string(r.req.ID), r.ver.Label)
		out = append(out, readDir(dir, SourceSaved, parseSaved, bad)...)
	}
	return out
}

type parser func(data []byte, name string) (resp response, keep bool, err error)

func readDir(dir, source string, parse parser, bad func(string, error)) []response {
	entries, err := os.ReadDir(dir) // sorted by name
	if err != nil {
		return nil // no directory is the normal case
	}
	var out []response
	for _, e := range entries {
		name, isJSON := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !isJSON || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		resp, keep, err := readOne(filepath.Join(dir, e.Name()), name, parse)
		if err != nil {
			bad(filepath.Join(dir, e.Name()), err)
			continue
		}
		if keep {
			resp.source = source
			out = append(out, resp)
		}
	}
	return out
}

func readOne(file, name string, parse parser) (response, bool, error) {
	info, err := os.Stat(file)
	if err != nil {
		return response{}, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxResponseFile {
		return response{}, false, fmt.Errorf("not a regular file under %d MiB", maxResponseFile>>20)
	}
	data, err := os.ReadFile(file) //nolint:gosec // inside the collection or the data dir
	if err != nil {
		return response{}, false, err
	}
	return parse(data, name)
}

func parseExample(data []byte, name, version string) (response, bool, error) {
	var ex exampleFile
	if err := json.Unmarshal(data, &ex); err != nil {
		return response{}, false, err
	}
	if ex.Version != "" && ex.Version != version {
		return response{}, false, nil // pinned to another version of the request
	}
	if err := checkStatus(ex.Status); err != nil {
		return response{}, false, err
	}
	resp := response{name: name, status: ex.Status, header: headersOf(ex.Headers)}
	body, kind, err := exampleBody(ex.Body)
	if err != nil {
		return response{}, false, err
	}
	resp.body, resp.kind = body, kind
	return resp, true, nil
}

// exampleBody turns an example's body into bytes. PinExample writes a
// non-JSON body as a JSON string holding its text.
func exampleBody(raw json.RawMessage) ([]byte, bodyKind, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, bodyJSON, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, 0, fmt.Errorf("body: %w", err)
		}
		return []byte(text), bodyText, nil
	}
	return compact(raw), bodyJSON, nil
}

func parseSaved(data []byte, name string) (response, bool, error) {
	var rec savedFile
	if err := json.Unmarshal(data, &rec); err != nil {
		return response{}, false, err
	}
	if err := checkStatus(rec.Status); err != nil {
		return response{}, false, err
	}
	resp := response{name: name, status: rec.Status, header: headersOf(rec.Headers)}
	switch {
	case len(rec.Body.JSON) > 0 && string(rec.Body.JSON) != "null":
		resp.body, resp.kind = compact(rec.Body.JSON), bodyJSON
	case rec.Body.Base64 != "":
		raw, err := base64.StdEncoding.DecodeString(rec.Body.Base64)
		if err != nil {
			return response{}, false, fmt.Errorf("body.base64: %w", err)
		}
		resp.body, resp.kind = raw, bodyBytes
	default:
		resp.body, resp.kind = []byte(rec.Body.Text), bodyText
	}
	return resp, true, nil
}

func checkStatus(s int) error {
	if s < 200 || s > 599 {
		return fmt.Errorf("status %d is not a final HTTP status (200-599)", s)
	}
	return nil
}

func headersOf(lines []headerLine) []header {
	out := make([]header, len(lines))
	for i, l := range lines {
		out[i] = header{name: l.Name, value: l.Value}
	}
	return out
}

// compact removes insignificant white space and changes nothing else.
func compact(raw json.RawMessage) []byte {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return raw
	}
	return b.Bytes()
}

// pick chooses the response for a call. An explicit example name or status
// wins; then, in sequence mode, the next response in turn; else the
// default (status 200, the lowest 2xx, or the first).
type chooser struct {
	sequence bool
	next     func() int // the sequence position for this call (sequence mode)
}

func (c chooser) pick(rs []response, p picks) (response, bool) {
	switch {
	case p.example != "":
		return find(rs, func(r response) bool { return r.name == p.example })
	case p.status != 0:
		return find(rs, func(r response) bool { return r.status == p.status })
	case c.sequence && len(rs) > 0:
		order := sequenceOrder(rs)
		return order[c.next()%len(order)], true
	}
	return defaultResponse(rs)
}

func find(rs []response, ok func(response) bool) (response, bool) {
	if i := slices.IndexFunc(rs, ok); i >= 0 {
		return rs[i], true
	}
	return response{}, false
}

func defaultResponse(rs []response) (response, bool) {
	if r, ok := find(rs, func(r response) bool { return r.status == 200 }); ok {
		return r, true
	}
	best, found := response{}, false
	for _, r := range rs {
		if r.status/100 == 2 && (!found || r.status < best.status) {
			best, found = r, true
		}
	}
	if found {
		return best, true
	}
	if len(rs) > 0 {
		return rs[0], true
	}
	return response{}, false
}

// sequenceOrder is the order a sequence walks: examples, then saved
// responses, by file name. last.json repeats one of the per-status files,
// so it is left out when others exist.
func sequenceOrder(rs []response) []response {
	savedOthers := slices.ContainsFunc(rs, func(r response) bool { return r.source == SourceSaved && r.name != "last" })
	return slices.DeleteFunc(slices.Clone(rs), func(r response) bool {
		return savedOthers && r.source == SourceSaved && r.name == "last"
	})
}

// Header safety. Names must be HTTP tokens and values must carry no
// control characters, so a response file cannot split the response or add
// framing headers.
var tokenName = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")

// skippedHeaders are managed by the server or are hop-by-hop; the body is
// stored decoded, so Content-Encoding would be wrong too.
func skippedHeader(lower string) bool {
	if strings.HasPrefix(lower, "x-flowi-") {
		return true
	}
	switch lower {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "proxy-connection",
		"te", "trailer", "transfer-encoding", "upgrade", "content-length", "content-encoding",
		"access-control-allow-origin", "access-control-expose-headers":
		return true
	}
	return false
}

func safeHeader(h header) bool {
	if !tokenName.MatchString(h.name) || skippedHeader(strings.ToLower(h.name)) {
		return false
	}
	for i := 0; i < len(h.value); i++ {
		if c := h.value[i]; (c < 0x20 && c != '\t') || c == 0x7f {
			return false
		}
	}
	return true
}
