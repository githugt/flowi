// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Contract sources, best first (design §9).
const (
	SourceSchema   = "schema"   // OpenAPI response schemas kept at import
	SourceExample  = "example"  // pinned examples
	SourceResponse = "response" // the last saved response
)

// Sample is one response a contract is made of.
type Sample struct {
	Source       string      // SourceExample or SourceResponse
	File         string      // the example or saved-response file
	Status       int         // HTTP status
	Headers      http.Header // response headers
	HeadersKnown bool        // false when an example lists no headers: header reads cannot be judged
	Body         any         // the JSON body, or nil when the body is not JSON
	Text         string      // the body text, when it is not JSON
	Schema       any         // for SourceSchema: the response's JSON Schema
}

// Contract is what a request version is known to return. Source is empty
// when nothing is known.
type Contract struct {
	Source  string
	Files   []string
	Samples []Sample
	// Removed says the request version is gone: a changed API description no longer has it. Every consumer
	// of it is broken, whatever it reads.
	Removed bool
}

// Contracts finds the contract of one request version.
type Contracts interface {
	Contract(c model.Collection, r model.Request, version string) (Contract, diag.List)
}

// FileContracts reads response schemas (<request>.schemas/<version>-
// <status>.json, written by OpenAPI import), pinned examples
// (<request>.examples/*.json, next to the request file) and saved responses (<data>/responses/<request-id>/
// <version>/last.json). DataDir maps a collection id to its data dir.
type FileContracts struct {
	DataDir func(model.ID) string
}

// Contract returns the examples for version (and unversioned ones), or
// else the last saved response. Unreadable examples are warnings and are
// skipped.
func (f FileContracts) Contract(c model.Collection, r model.Request, version string) (Contract, diag.List) {
	if schemas, diags := readSchemas(r, version); len(schemas) > 0 {
		return contractOf(SourceSchema, schemas), diags
	}
	examples, diags := readExamples(r, version)
	if len(examples) > 0 {
		return contractOf(SourceExample, examples), diags
	}
	if saved, ok := f.readSaved(c, r, version); ok {
		return contractOf(SourceResponse, []Sample{saved}), diags
	}
	return Contract{}, diags
}

func contractOf(source string, samples []Sample) Contract {
	c := Contract{Source: source, Samples: samples}
	for _, s := range samples {
		c.Files = append(c.Files, s.File)
	}
	return c
}

// exampleFile is the pinned example format written by engine.PinExample.
type exampleFile struct {
	Version string        `json:"version"`
	Status  int           `json:"status"`
	Headers *[]headerLine `json:"headers"` // nil: the example does not record headers
	Body    any           `json:"body"`
}

type headerLine struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// examplesDir mirrors engine.PinExample: login.req.yaml → login.examples.
func examplesDir(r model.Request) string {
	return strings.TrimSuffix(r.File, ".req.yaml") + ".examples"
}

func readExamples(r model.Request, version string) ([]Sample, diag.List) {
	entries, err := os.ReadDir(examplesDir(r)) // sorted by name, so samples are in a stable order
	if err != nil {
		return nil, missingIsFine(err, examplesDir(r))
	}
	var samples []Sample
	var diags diag.List
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		file := filepath.Join(examplesDir(r), e.Name())
		s, v, err := readExample(file)
		if err != nil {
			diags = append(diags, diag.New(diag.CodeContractExample, diag.Location{File: file}, map[string]string{"detail": err.Error()}))
			continue
		}
		if v == "" || v == version {
			samples = append(samples, s)
		}
	}
	return samples, diags
}

func readExample(file string) (Sample, string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Sample{}, "", err
	}
	var ex exampleFile
	if err := json.Unmarshal(data, &ex); err != nil {
		return Sample{}, "", err
	}
	s := Sample{Source: SourceExample, File: file, Status: ex.Status, Headers: http.Header{}}
	if ex.Headers != nil {
		s.Headers, s.HeadersKnown = headerOf(*ex.Headers), true
	}
	// PinExample writes a non-JSON body as its text.
	if text, isText := ex.Body.(string); isText {
		s.Text = text
	} else {
		s.Body = ex.Body
	}
	return s, ex.Version, nil
}

// savedRecord is the part of engine.Record a contract needs.
type savedRecord struct {
	Status  int          `json:"status"`
	Headers []headerLine `json:"headers"`
	Body    struct {
		JSON any    `json:"json"`
		Text string `json:"text"`
	} `json:"body"`
}

// readSaved reads the last saved response. A missing or unreadable file
// means "no saved response": saved responses are a cache the user does
// not edit, so a bad one is not worth a warning.
func (f FileContracts) readSaved(c model.Collection, r model.Request, version string) (Sample, bool) {
	if f.DataDir == nil {
		return Sample{}, false
	}
	file := filepath.Join(f.DataDir(c.Manifest.ID), "responses", string(r.ID), version, "last.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return Sample{}, false
	}
	var rec savedRecord
	if json.Unmarshal(data, &rec) != nil {
		return Sample{}, false
	}
	return Sample{Source: SourceResponse, File: file, Status: rec.Status, Headers: headerOf(rec.Headers),
		HeadersKnown: true, Body: rec.Body.JSON, Text: rec.Body.Text}, true
}

func headerOf(lines []headerLine) http.Header {
	h := http.Header{}
	for _, l := range lines {
		h.Add(l.Name, l.Value)
	}
	return h
}

// missingIsFine: no examples directory is the normal case.
func missingIsFine(err error, dir string) diag.List {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return diag.List{diag.New(diag.CodeContractExample, diag.Location{File: dir}, map[string]string{"detail": err.Error()})}
}

// schemasDir mirrors examplesDir: login.req.yaml → login.schemas.
func schemasDir(r model.Request) string {
	return strings.TrimSuffix(r.File, ".req.yaml") + ".schemas"
}

// readSchemas reads <version>-<status>.json files for version. A schema
// that does not parse is a warning and is skipped.
func readSchemas(r model.Request, version string) ([]Sample, diag.List) {
	entries, err := os.ReadDir(schemasDir(r))
	if err != nil {
		return nil, missingIsFine(err, schemasDir(r))
	}
	var out []Sample
	var diags diag.List
	for _, e := range entries {
		label, status, ok := schemaName(e.Name())
		if !ok || label != version {
			continue
		}
		file := filepath.Join(schemasDir(r), e.Name())
		var schema any
		data, err := os.ReadFile(file)
		if err == nil {
			err = json.Unmarshal(data, &schema)
		}
		if err != nil {
			diags = append(diags, diag.New(diag.CodeContractExample, diag.Location{File: file}, map[string]string{"detail": err.Error()}))
			continue
		}
		out = append(out, Sample{Source: SourceSchema, File: file, Status: status, Schema: schema})
	}
	return out, diags
}

// schemaName splits "1.1-200.json" into its version label and status.
func schemaName(name string) (string, int, bool) {
	base, ok := strings.CutSuffix(name, ".json")
	cut := strings.LastIndex(base, "-")
	if !ok || cut < 0 {
		return "", 0, false
	}
	status, err := strconv.Atoi(base[cut+1:])
	if err != nil {
		return "", 0, false
	}
	return base[:cut], status, true
}
