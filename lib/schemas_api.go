// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/schemainfer"
	"github.com/githugt/flowi-lib/store"
)

// SchemaPart names one schema of a request version: the body it sends, the
// body of the response with a status, or (async requests) the payload of the
// messages it receives.
type SchemaPart struct {
	Kind   string // "request", "response" or "message"
	Status int    // for a response
}

// SchemaSource says where a stored schema came from.
type SchemaSource string

// Where schemas come from. Only inferred ones are rewritten by later sends.
const (
	SchemaInferred SchemaSource = "inferred"
	SchemaOpenAPI  SchemaSource = "openapi"
	SchemaUser     SchemaSource = "user"
)

// SchemaChange is a change a send made to a stored schema.
type SchemaChange struct {
	Part   SchemaPart
	Change schemainfer.Change
}

// schemaFile is <request>.schemas/<version>-request.json or
// <version>-<status>.json, the layout OpenAPI imports already use.
func schemaFile(requestFile, version string, part SchemaPart) (string, error) {
	if !names.Valid(version) {
		return "", fmt.Errorf("version %q cannot name a schema file", version)
	}
	var name string
	switch part.Kind {
	case "request", "message":
		name = version + "-" + part.Kind + ".json"
	case "response":
		name = version + "-" + strconv.Itoa(part.Status) + ".json"
	default:
		return "", fmt.Errorf("unknown schema part %q", part.Kind)
	}
	return filepath.Join(strings.TrimSuffix(requestFile, ".req.yaml")+".schemas", name), nil
}

// SchemaFor is the stored schema of part of version of request t.
func (w *Workspace) SchemaFor(t RequestTarget, version string, part SchemaPart) (schemainfer.Schema, SchemaSource, bool) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return nil, "", false
	}
	file, err := schemaFile(r.File, version, part)
	if err != nil {
		return nil, "", false
	}
	return readSchema(file)
}

func readSchema(file string) (schemainfer.Schema, SchemaSource, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", false
	}
	var s schemainfer.Schema
	if json.Unmarshal(data, &s) != nil || s == nil {
		return nil, "", false // a cache nobody edits: a bad file is not worth an error
	}
	switch src, _ := s["x-flowi-source"].(string); SchemaSource(src) {
	case SchemaOpenAPI:
		return s, SchemaOpenAPI, true
	case SchemaUser:
		return s, SchemaUser, true
	}
	return s, SchemaInferred, true
}

// LearnSchemas infers or widens the request-body and response schemas of
// version of request t from rec, the record of a send, and returns what it
// changed in schemas that already existed. A first sample creates the schema
// silently. Bodies that are not JSON learn nothing. Schemas from a spec or
// edited by hand are never rewritten.
func (w *Workspace) LearnSchemas(t RequestTarget, version string, rec engine.Record) ([]SchemaChange, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return nil, err
	}
	var changes []SchemaChange
	for _, s := range samplesOf(rec) {
		got, err := learnOne(r.File, version, s.part, s.sample)
		if err != nil {
			return changes, err
		}
		changes = append(changes, got...)
	}
	return changes, w.RegenerateDescription(t.Collection)
}

type schemaSample struct {
	part   SchemaPart
	sample any
}

// samplesOf are the bodies of a record worth learning, in the order they are
// learned: the request body, then the response (HTTP) or each received message.
func samplesOf(rec engine.Record) []schemaSample {
	out := []schemaSample{}
	add := func(part SchemaPart, sample any) {
		if sample != nil {
			out = append(out, schemaSample{part, sample})
		}
	}
	add(SchemaPart{Kind: "request"}, requestJSON(rec))
	if rec.HasMessages() { // async and gRPC: every message received is a "message"
		for _, m := range rec.Messages {
			if m.Direction == "received" {
				add(SchemaPart{Kind: "message"}, m.Body.JSON)
			}
		}
		return out
	}
	if rec.Status != 0 {
		add(SchemaPart{Kind: "response", Status: rec.Status}, rec.Body.JSON)
	}
	return out
}

// requestJSON is the request body as JSON, or nil when it is not. The record
// keeps a request body as text, so it is parsed here; an object or an array
// counts, a bare word or number does not (a form or a plain text body).
func requestJSON(rec engine.Record) any {
	b := rec.Request.Body
	if b == nil {
		return nil
	}
	if b.JSON != nil {
		return b.JSON
	}
	text := strings.TrimSpace(b.Text)
	if text == "" || (text[0] != '{' && text[0] != '[') {
		return nil
	}
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil
	}
	return v
}

func learnOne(requestFile, version string, part SchemaPart, sample any) ([]SchemaChange, error) {
	file, err := schemaFile(requestFile, version, part)
	if err != nil {
		return nil, err
	}
	existing, _, found := readSchema(file)
	if !found {
		if _, statErr := os.Stat(file); statErr == nil {
			return nil, nil // there is a file we cannot read: leave it
		}
		return nil, writeSchema(file, schemainfer.Infer(sample))
	}
	merged, found2 := schemainfer.Merge(existing, sample)
	if len(found2) == 0 {
		return nil, nil
	}
	if err := writeSchema(file, merged); err != nil {
		return nil, err
	}
	out := make([]SchemaChange, len(found2))
	for i, c := range found2 {
		out[i] = SchemaChange{Part: part, Change: c}
	}
	return out, nil
}

func writeSchema(file string, s schemainfer.Schema) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", file, err)
	}
	return store.Apply([]store.FileChange{{File: file, New: append(data, '\n')}})
}

// DeleteSchema removes a stored schema; the next send learns it again.
func (w *Workspace) DeleteSchema(t RequestTarget, version string, part SchemaPart) error {
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	file, err := schemaFile(r.File, version, part)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", file, err)
	}
	return w.RegenerateDescription(t.Collection)
}

// ReplaceSchema stores a hand-edited schema and marks it so later sends leave
// it alone.
func (w *Workspace) ReplaceSchema(t RequestTarget, version string, part SchemaPart, s schemainfer.Schema) error {
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	file, err := schemaFile(r.File, version, part)
	if err != nil {
		return err
	}
	edited := schemainfer.Schema{}
	for k, v := range s {
		edited[k] = v
	}
	edited["x-flowi-source"] = string(SchemaUser)
	if err := writeSchema(file, edited); err != nil {
		return err
	}
	return w.RegenerateDescription(t.Collection)
}

// maxViolationsShown caps the diagnostics one body can raise, so a response
// that is wrong everywhere is one screen, not a hundred lines.
const maxViolationsShown = 3

// schemaChecker is the engine's SchemaChecker over the workspace.
type schemaChecker struct{ w *Workspace }

// Check validates the request body and the response of rec against the
// schemas stored before this send, then learns from them. A request mismatch
// is a warning (FLW-SCHEMA-001), a response mismatch an error (FLW-SCHEMA-002,
// which does not change the send's status), a schema that had to be widened
// a warning (FLW-SCHEMA-003). A first sample creates its schema silently.
func (s schemaChecker) Check(c model.Collection, r model.Request, version string, rec engine.Record) diag.List {
	t := RequestTarget{Collection: c.Manifest.ID, Request: r.ID}
	var out diag.List
	for _, sample := range samplesOf(rec) {
		code := diag.CodeSchemaResponse
		switch sample.part.Kind {
		case "request":
			code = diag.CodeSchemaRequest
		case "message":
			code = diag.CodeSchemaMessage
		}
		out = append(out, s.violations(t, version, sample.part, sample.sample, r.Path, rec.Status, code)...)
	}
	changes, err := s.w.LearnSchemas(t, version, rec)
	if err != nil {
		return out // schemas are a convenience: a write problem must not fail a send
	}
	for _, ch := range changes {
		out = append(out, diag.New(diag.CodeSchemaUpdated, diag.Location{}, map[string]string{
			"what": partName(ch.Part), "request": r.Path, "path": ch.Change.Path, "detail": ch.Change.Detail,
		}))
	}
	return out
}

// Match checks body against the response schema kept for status of version of
// request r, without learning from it. found is false when none is kept.
func (s schemaChecker) Match(c model.Collection, r model.Request, version string, status int, body any) ([]engine.SchemaViolation, bool) {
	t := RequestTarget{Collection: c.Manifest.ID, Request: r.ID}
	schema, _, ok := s.w.SchemaFor(t, version, SchemaPart{Kind: "response", Status: status})
	if !ok {
		return nil, false
	}
	vs, err := schemainfer.Validate(schema, body)
	if err != nil {
		return nil, false // an unusable stored schema checks nothing
	}
	out := make([]engine.SchemaViolation, 0, len(vs))
	for _, v := range vs {
		out = append(out, engine.SchemaViolation{Path: v.Path, Message: v.Message})
	}
	return out, true
}

func (s schemaChecker) violations(t RequestTarget, version string, part SchemaPart, sample any, request string, status int, code diag.Code) diag.List {
	if sample == nil {
		return nil
	}
	schema, _, ok := s.w.SchemaFor(t, version, part)
	if !ok {
		return nil
	}
	vs, err := schemainfer.Validate(schema, sample)
	if err != nil {
		return nil // an unusable stored schema checks nothing
	}
	var out diag.List
	for _, v := range vs[:min(len(vs), maxViolationsShown)] {
		out = append(out, diag.New(code, diag.Location{}, map[string]string{
			"request": request, "path": v.Path, "message": v.Message, "status": strconv.Itoa(status),
		}))
	}
	return out
}

func partName(p SchemaPart) string {
	switch p.Kind {
	case "response":
		return "response (" + strconv.Itoa(p.Status) + ")"
	case "message":
		return "received messages"
	}
	return "request body"
}

// ListSchemas lists the schemas stored for version of request t: the request
// body first, then the responses by status.
func (w *Workspace) ListSchemas(t RequestTarget, version string) []SchemaPart {
	_, r, err := w.requestFor(t)
	if err != nil || !names.Valid(version) {
		return nil
	}
	entries, err := os.ReadDir(strings.TrimSuffix(r.File, ".req.yaml") + ".schemas")
	if err != nil {
		return nil
	}
	var parts []SchemaPart
	for _, e := range entries {
		label, isJSON := strings.CutSuffix(e.Name(), ".json")
		cut := strings.LastIndex(label, "-")
		if !isJSON || cut < 0 || label[:cut] != version {
			continue
		}
		switch tail := label[cut+1:]; tail {
		case "request":
			parts = append(parts, SchemaPart{Kind: "request"})
		case "message":
			parts = append(parts, SchemaPart{Kind: "message"})
		default:
			if status, err := strconv.Atoi(tail); err == nil {
				parts = append(parts, SchemaPart{Kind: "response", Status: status})
			}
		}
	}
	rank := map[string]int{"request": 0, "message": 1, "response": 2}
	slices.SortFunc(parts, func(a, b SchemaPart) int {
		if a.Kind != b.Kind {
			return rank[a.Kind] - rank[b.Kind]
		}
		return a.Status - b.Status
	})
	return parts
}

// RequestSchema is the schema kept for the body a request version sends, for a fuzz step to read its bounds
// from. ok is false when none is kept.
func (s schemaChecker) RequestSchema(_ model.Collection, r model.Request, version string) (map[string]any, bool) {
	file, err := schemaFile(r.File, version, SchemaPart{Kind: "request"})
	if err != nil {
		return nil, false
	}
	schema, _, found := readSchema(file)
	return schema, found
}
