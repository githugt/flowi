// SPDX-License-Identifier: Apache-2.0

// Package postman imports Postman v2.0/v2.1 collections and Postman
// environment files. Folders become request folders, items become
// requests (one version "1"), saved responses become pinned examples,
// collection variables and environments become environments. Variables
// become env (or secret) references, dynamic variables become
// generators; scripts and unsupported auth are reported, credentials
// never written.
package postman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/importers/jsonpos"
	"github.com/githugt/flowi/lib/model"
)

// Import reads a collection or an environment file.
func Import(src []byte, file string) (importers.Result, diag.List) {
	offsets, err := jsonpos.Scan(src, func(p []string) bool { return len(p) >= 2 && p[len(p)-2] == "item" })
	if err != nil {
		return importers.Result{}, diag.List{syntaxError(src, file, err)}
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(src, &probe); err != nil {
		return importers.Result{}, invalid(file, "a Postman export is a JSON object")
	}
	switch {
	case probe["_postman_variable_scope"] != nil || probe["values"] != nil && probe["item"] == nil:
		return importEnvironment(src, file)
	case probe["requests"] != nil && probe["item"] == nil:
		return importers.Result{}, invalid(file, "Postman v1 collections are not supported; export the collection again as v2.1")
	}
	var c collection
	if err := json.Unmarshal(src, &c); err != nil || c.Info == nil || !strings.Contains(c.Info.Schema, "/v2.") {
		return importers.Result{}, invalid(file, "a Postman v2 collection has info.schema …/collection/v2.1.0/…")
	}
	im := &importer{src: src, file: file, offsets: offsets, lines: diag.NewLineIndex(src)}
	return im.collection(c), nil
}

func invalid(file, detail string) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file}, map[string]string{"detail": detail})}
}

func syntaxError(src []byte, file string, err error) diag.Diagnostic {
	offset, _ := jsonpos.ErrorOffset(err, len(src))
	pos := diag.PositionAt(src, offset)
	d := diag.New(diag.CodeImportSyntax, diag.Location{File: file, Pos: pos}, map[string]string{"format": "Postman", "detail": err.Error()})
	d.Excerpt = diag.ExcerptAt(src, pos)
	return d
}

type importer struct {
	src     []byte
	file    string
	offsets map[string]int
	lines   *diag.LineIndex
	taken   importers.Names
	result  importers.Result
}

func (im *importer) collection(c collection) importers.Result {
	top := importers.Item{Name: "collection " + c.Info.Name, Source: diag.Location{File: im.file, Pos: diag.Position{Line: 1, Column: 1}}}
	reportScripts(&top, "collection", c.Event)
	if env, ok := collectionVariables(&top, c); ok {
		im.result.Environments = append(im.result.Environments, env)
	}
	im.result.Items = append(im.result.Items, top)
	im.walk(c.Item, nil, c.Auth, "item")
	return im.result
}

// collectionVariables become an environment named after the collection.
func collectionVariables(it *importers.Item, c collection) (model.Environment, bool) {
	env := model.Environment{Meta: model.Meta{Name: importers.FileName(strings.ToLower(c.Info.Name))}}
	for _, v := range c.Variable {
		if v.Disabled || v.Key == "" {
			continue
		}
		env.Vars = append(env.Vars, envVar(it, v.Key, string(v.Value), v.Type == "secret"))
	}
	if env.Name == "" {
		env.Name = "postman"
	}
	return env, len(env.Vars) > 0
}

// envVar declares credential-like variables (by name or Postman's secret
// type) as secrets, without their value.
func envVar(it *importers.Item, name, value string, secret bool) model.EnvVar {
	if secret || importers.SensitiveHeader(name) {
		it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": "variable " + name, "placeholder": "a secret declaration"}))
		return model.EnvVar{Name: name, Secret: true}
	}
	text, _ := importers.Translate(value)
	return model.EnvVar{Name: name, Value: text}
}

func reportScripts(it *importers.Item, owner string, events []event) {
	for _, e := range events {
		if strings.TrimSpace(strings.Join(e.Script.Exec, "")) == "" {
			continue
		}
		what := map[string]string{"prerequest": "pre-request script", "test": "test script"}[e.Listen]
		it.Unmapped(owner+" "+cmpOr(what, e.Listen+" script"), "left out", "scripts are JavaScript; use hooks and extract or assert nodes instead")
	}
}

// walk imports items; folders add a path segment and may set auth.
func (im *importer) walk(items []item, folders []string, inherited *auth, ptr string) {
	for i, it := range items {
		p := ptr + "/" + strconv.Itoa(i)
		loc := diag.Location{File: im.file, Pos: im.lines.Position(im.offsets[p])}
		switch {
		case it.Item != nil:
			folderAuth := inherited
			if it.Auth != nil && it.Auth.Type != "inherit" {
				folderAuth = it.Auth
			}
			if len(it.Event) > 0 {
				folder := importers.Item{Name: "folder " + it.Name, Source: loc}
				reportScripts(&folder, "folder "+it.Name, it.Event)
				im.result.Items = append(im.result.Items, folder)
			}
			im.walk(it.Item, append(append([]string{}, folders...), cmpOr(importers.FileName(it.Name), "folder")), folderAuth, p+"/item")
		case it.Request == nil:
			skipped := importers.Item{Name: it.Name, Source: loc}
			skipped.Skip("it is neither a request nor a folder")
			im.result.Items = append(im.result.Items, skipped)
		default:
			im.request(it, folders, inherited, loc)
		}
	}
}

func (im *importer) request(it item, folders []string, inherited *auth, loc diag.Location) {
	path := strings.Join(append(append([]string{}, folders...), cmpOr(importers.FileName(it.Name), "request")), "/")
	rep := importers.Item{Name: path, Source: loc}
	req, err := decodeRequest(it.Request)
	switch {
	case err != nil:
		rep.Skip("the request cannot be read: " + err.Error())
	case req.url.raw == "":
		rep.Skip("the request has no URL")
	default:
		path = im.taken.Unique(path)
		rep.Name = path
		v := im.version(&rep, req, inherited)
		im.result.Requests = append(im.result.Requests, model.Request{Meta: model.Meta{Name: it.Name}, Path: path, Versions: []model.RequestVersion{v}})
		im.examples(&rep, path, it.Response)
		reportScripts(&rep, "request", it.Event)
	}
	im.result.Items = append(im.result.Items, rep)
}

// version builds the request's only version, reporting into rep.
func (im *importer) version(rep *importers.Item, req decoded, inherited *auth) model.RequestVersion {
	t := &translator{Translator: importers.Translator{Item: rep}, it: rep}
	v := model.RequestVersion{Label: "1", Method: strings.ToUpper(cmpOr(req.method, "GET"))}
	v.URL, v.Query = t.url(req.url)
	if t.userinfo {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: rep.Redact("credentials in the URL", "Authorization")})
	}
	for _, h := range req.headers {
		if h.Disabled || h.Key == "" {
			continue
		}
		v.Headers = append(v.Headers, model.KeyValue{Key: h.Key, Value: t.credential("header "+h.Key, h.Key, string(h.Value))})
	}
	a := inherited
	if req.auth != nil && req.auth.Type != "inherit" {
		a = req.auth
	}
	t.auth(a, &v)
	v.Body = t.body(req.body, &v)
	t.reportUnknown()
	return v
}

// examples keeps saved responses as pinned examples.
func (im *importer) examples(rep *importers.Item, path string, responses []savedResponse) {
	var taken importers.Names
	for _, r := range responses {
		if r.Code == 0 {
			continue
		}
		name := taken.Unique(cmpOr(importers.FileName(r.Name), strconv.Itoa(r.Code)))
		ex := importers.Example{RequestPath: path, Name: name, Version: "1", Status: r.Code, Body: r.Body}
		var parsed any
		if json.Unmarshal([]byte(r.Body), &parsed) == nil {
			clean, n := importers.RedactJSON(parsed)
			ex.Body = clean
			if n > 0 {
				rep.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
					"where": fmt.Sprintf("saved response %s (%d values)", r.Name, n), "placeholder": importers.Redacted}))
			}
		}
		for _, h := range r.Header {
			value := string(h.Value)
			if importers.SensitiveHeader(h.Key) {
				value = importers.Redacted
			}
			ex.Headers = append(ex.Headers, importers.Header{Name: h.Key, Value: value})
		}
		im.result.Examples = append(im.result.Examples, ex)
	}
}

// importEnvironment reads a Postman environment file.
func importEnvironment(src []byte, file string) (importers.Result, diag.List) {
	var e struct {
		Name   string `json:"name"`
		Values []struct {
			Key     string `json:"key"`
			Value   flex   `json:"value"`
			Type    string `json:"type"`
			Enabled *bool  `json:"enabled"`
		} `json:"values"`
	}
	if err := json.Unmarshal(src, &e); err != nil {
		return importers.Result{}, invalid(file, "a Postman environment has a name and values")
	}
	it := importers.Item{Name: "environment " + e.Name, Source: diag.Location{File: file, Pos: diag.Position{Line: 1, Column: 1}}}
	env := model.Environment{Meta: model.Meta{Name: cmpOr(importers.FileName(strings.ToLower(e.Name)), "postman")}}
	for _, v := range e.Values {
		if v.Key == "" || v.Enabled != nil && !*v.Enabled {
			continue
		}
		env.Vars = append(env.Vars, envVar(&it, v.Key, string(v.Value), v.Type == "secret"))
	}
	return importers.Result{Environments: []model.Environment{env}, Items: []importers.Item{it}}, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// isJSONString reports whether data is a JSON string (as opposed to an
// object), for fields Postman writes either way.
func isJSONString(data []byte) bool {
	return len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '"'
}
