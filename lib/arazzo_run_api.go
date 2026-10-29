// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/migrate"
	"github.com/githugt/flowi-lib/model"
)

// ArazzoDoc is a standalone Arazzo document opened to run. Its workflows are flows of a temporary
// collection that exists only until Close: nothing is mounted or added to the workspace file.
type ArazzoDoc struct {
	Flows []FlowTarget // one for each workflow, in document order
	Diags diag.List    // what the import could not carry over
	// Env names an environment that holds base_url, taken from the first server an OpenAPI source lists,
	// so the document runs without an environment of the person's own; "" when no source lists a server.
	// Not a file: it lives and dies with the temporary collection.
	Env string

	w   *Workspace
	id  model.ID
	dir string
}

// Close removes the temporary collection from the workspace and from disk. It is safe to call twice.
func (d *ArazzoDoc) Close() {
	d.w.mu.Lock()
	d.w.collections = slices.DeleteFunc(d.w.collections, func(c model.Collection) bool { return c.Manifest.ID == d.id })
	d.w.mu.Unlock()
	_ = os.RemoveAll(d.dir) // best effort: a temporary directory the system will clear anyway
}

// IsArazzoRef reports whether ref names an Arazzo document to open rather than a saved flow: a URL, or an
// existing file that is YAML or JSON. A flow's path or id never has a file behind it.
func IsArazzoRef(ref string) bool {
	if IsImportURL(ref) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(ref))
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return false
	}
	info, err := os.Stat(ref)
	return err == nil && !info.IsDir()
}

// OpenArazzo reads the Arazzo document at ref (a file or an http(s) URL), imports the OpenAPI documents
// its sourceDescriptions name into a temporary collection, then the workflows. A relative source URL is
// read from the document's own folder or URL. Close the result when the runs are over.
func (w *Workspace) OpenArazzo(ctx context.Context, ref string) (*ArazzoDoc, error) {
	data, name, base, err := readArazzoRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	sources, err := arazzoSources(data)
	if err != nil {
		return nil, err
	}
	doc, err := w.newTemporaryCollection(name)
	if err != nil {
		return nil, err
	}
	if err := w.fillTemporary(ctx, doc, data, name, base, sources); err != nil {
		doc.Close()
		return nil, err
	}
	return doc, nil
}

func (w *Workspace) newTemporaryCollection(name string) (*ArazzoDoc, error) {
	dir, err := os.MkdirTemp("", "flowi-arazzo-")
	if err != nil {
		return nil, fmt.Errorf("making a temporary collection: %w", err)
	}
	id := w.newID()
	m := model.Manifest{Format: migrate.CurrentFormat, Meta: model.Meta{ID: id, Name: name}}
	if err := w.store.Init(dir, m); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	c, _, err := w.store.Load(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("loading the temporary collection: %w", err)
	}
	w.mu.Lock()
	w.collections = append(w.collections, w.withGlobalEnvironments(c))
	w.mu.Unlock()
	return &ArazzoDoc{w: w, id: id, dir: dir}, nil
}

func (w *Workspace) fillTemporary(ctx context.Context, doc *ArazzoDoc, data []byte, name string, base sourceBase, sources []arazzoSource) error {
	var server string
	for _, s := range sources {
		address, err := w.importSource(ctx, doc.id, base, s)
		if err != nil {
			return err
		}
		server = cmp.Or(server, address)
	}
	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportArazzo, Name: name, Data: data}, doc.id)
	if err != nil {
		return err
	}
	doc.Diags = plan.Diags
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		return err
	}
	if err := w.collectFlows(doc); err != nil {
		return err
	}
	w.addServerEnvironment(doc, server)
	return nil
}

// documentEnvironment is the name of the environment made from the document's own server.
const documentEnvironment = "document"

// addServerEnvironment lets the temporary collection resolve {{env.base_url}} to the document's server.
func (w *Workspace) addServerEnvironment(doc *ArazzoDoc, server string) {
	if server == "" {
		return
	}
	env := model.Environment{Meta: model.Meta{ID: w.newID(), Name: documentEnvironment}, Vars: []model.EnvVar{{Name: "base_url", Value: strings.TrimRight(server, "/")}}}
	w.mu.Lock()
	defer w.mu.Unlock()
	if i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == doc.id }); i >= 0 {
		w.collections[i].Environments = append(w.collections[i].Environments, env)
		doc.Env = documentEnvironment
	}
}

func (w *Workspace) collectFlows(doc *ArazzoDoc) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(doc.id)
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(doc.id)})
	}
	for _, f := range c.Flows {
		doc.Flows = append(doc.Flows, FlowTarget{Collection: c.Manifest.ID, Flow: f.ID})
	}
	if len(doc.Flows) == 0 {
		return errors.New("the document has no workflows")
	}
	return nil
}

// importSource reads one OpenAPI source description and imports it. Another kind of source (a nested
// Arazzo document) is not followed.
func (w *Workspace) importSource(ctx context.Context, id model.ID, base sourceBase, s arazzoSource) (server string, err error) {
	if s.Type != "" && s.Type != "openapi" {
		return "", nil
	}
	data, name, refDir, err := base.read(ctx, s.URL)
	if err != nil {
		return "", fmt.Errorf("source description %q: %w", s.Name, err)
	}
	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Name: name, Data: data, RefDir: refDir}, id)
	if err != nil {
		return "", fmt.Errorf("source description %q: %w", s.Name, err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{SkipExisting: true}); err != nil {
		return "", err
	}
	return firstServer(data), nil
}

// firstServer is the first address of an OpenAPI 3 document's servers, or "" (a relative or templated address is
// no use as a base).
func firstServer(data []byte) string {
	var doc struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
	}
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Servers) == 0 || !IsImportURL(doc.Servers[0].URL) || strings.Contains(doc.Servers[0].URL, "{") {
		return ""
	}
	return doc.Servers[0].URL
}

type arazzoSource struct{ Name, URL, Type string }

// arazzoSources reads sourceDescriptions without importing anything. A document that is not Arazzo
// is an error that says so.
func arazzoSources(data []byte) ([]arazzoSource, error) {
	var doc struct {
		Arazzo  string `yaml:"arazzo"`
		Sources []struct {
			Name string `yaml:"name"`
			URL  string `yaml:"url"`
			Type string `yaml:"type"`
		} `yaml:"sourceDescriptions"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("reading the Arazzo document: %w", err)
	}
	if doc.Arazzo == "" {
		return nil, errors.New("this is not an Arazzo document: it has no `arazzo:` version")
	}
	out := make([]arazzoSource, 0, len(doc.Sources))
	for _, s := range doc.Sources {
		out = append(out, arazzoSource{Name: s.Name, URL: s.URL, Type: s.Type})
	}
	return out, nil
}

// sourceBase is where a document's relative source URLs are resolved: a folder, or the URL the
// document came from.
type sourceBase struct {
	dir string
	url *url.URL
}

func readArazzoRef(ctx context.Context, ref string) (data []byte, name string, base sourceBase, err error) {
	if IsImportURL(ref) {
		data, name, err = FetchImportSource(ctx, ref)
		u, _ := url.Parse(ref) // FetchImportSource checked it
		return data, name, sourceBase{url: u}, err
	}
	data, err = os.ReadFile(ref) //nolint:gosec // the person names the file to run
	if err != nil {
		return nil, "", sourceBase{}, fmt.Errorf("reading %s: %w", ref, err)
	}
	return data, filepath.Base(ref), sourceBase{dir: filepath.Dir(ref)}, nil
}

// read returns the document a source URL names. refDir is the folder its file references are read from:
// only for a local file, never for a download.
func (b sourceBase) read(ctx context.Context, ref string) (data []byte, name, refDir string, err error) {
	if IsImportURL(ref) {
		data, name, err = FetchImportSource(ctx, ref)
		return data, name, "", err
	}
	if b.url != nil {
		rel, perr := url.Parse(ref)
		if perr != nil {
			return nil, "", "", fmt.Errorf("%q is not a usable address: %w", ref, perr)
		}
		data, name, err = FetchImportSource(ctx, b.url.ResolveReference(rel).String())
		return data, name, "", err
	}
	file := ref
	if !filepath.IsAbs(file) {
		file = filepath.Join(b.dir, filepath.FromSlash(ref))
	}
	data, err = os.ReadFile(file) //nolint:gosec // a source description of the document the person chose to run
	if err != nil {
		return nil, "", "", fmt.Errorf("reading %s: %w", file, err)
	}
	return data, path.Base(filepath.ToSlash(file)), filepath.Dir(file), nil
}

// KeepArazzo opens a document like OpenArazzo and leaves it open for browsing until the process ends: its
// collection stays in the workspace, named after the document, and a run started in it with no environment
// chosen uses the document's own server. That is how a window runs an Arazzo file: the flows open like any other.
func (w *Workspace) KeepArazzo(ctx context.Context, ref string) (*ArazzoDoc, error) {
	doc, err := w.OpenArazzo(ctx, ref)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.documentEnvs == nil {
		w.documentEnvs = map[model.ID]string{}
	}
	if len(doc.Flows) > 0 {
		w.documentEnvs[doc.Flows[0].Collection] = doc.Env
		if i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == doc.id }); i >= 0 {
			w.collections[i].Manifest.Name += " (Arazzo)"
		}
	}
	return doc, nil
}
