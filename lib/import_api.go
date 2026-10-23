// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/arazzo"
	"github.com/githugt/flowi-lib/asyncapi"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/importers/bruno"
	"github.com/githugt/flowi-lib/importers/curl"
	"github.com/githugt/flowi-lib/importers/har"
	"github.com/githugt/flowi-lib/importers/httpfile"
	"github.com/githugt/flowi-lib/importers/postman"
	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/mermaid"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/openapi"
	"github.com/githugt/flowi-lib/specoverlay"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/workspace"
)

// ImportKind names a document kind Flow-I imports.
type ImportKind string

// Import kinds.
const (
	ImportOpenAPI ImportKind = "openapi"
	// ImportAsyncAPI is an AsyncAPI 3.x (or 2.x) document: async requests.
	ImportAsyncAPI ImportKind = "asyncapi"
	// ImportProto is a .proto file: one gRPC request per unary or
	// server-streaming method of its services.
	ImportProto   ImportKind = "proto"
	ImportArazzo  ImportKind = "arazzo"
	ImportCurl    ImportKind = "curl"
	ImportHAR     ImportKind = "har"
	ImportHTTP    ImportKind = "http"    // a .http / .rest file (REST Client, JetBrains)
	ImportPostman ImportKind = "postman" // a collection or an environment file
	ImportBruno   ImportKind = "bruno"   // a collection folder: ImportSource.Dir
	ImportMermaid ImportKind = "mermaid" // a flowchart sketch, resolved against the collection
)

// ImportSource is a document to import: its kind, a name for reports (a
// file name, or "stdin"), and its bytes — or, for a folder format such as
// Bruno, the folder.
type ImportSource struct {
	Kind ImportKind
	Name string
	Data []byte
	Dir  string
	// Overlays are OpenAPI Overlay documents applied, in order, to an OpenAPI
	// or AsyncAPI document before it is imported. The document itself is kept
	// as it was, with the overlays beside it in descriptions/.
	Overlays [][]byte
}

// ImportPlan is what an import would write. Files are in path order;
// Diags are the import's lossy reports (FLW-IMPORT-001).
type ImportPlan struct {
	Collection model.ID
	Files      []PlannedFile
	Diags      diag.List
	// Items is the per-item report of importers other than OpenAPI and
	// Arazzo; skipped items contribute no files.
	Items []importers.Item
	data  map[string][]byte // by File; nil for a file to remove
}

// PlannedFile is one file an import writes.
type PlannedFile struct {
	File   string // absolute
	Rel    string // slash-separated, relative to the collection root
	Exists bool   // the file is already there (at preview time)
	// Same marks an existing file the import would write back unchanged, so
	// a preview can leave it out of what changes.
	Same bool
	// Remove marks a tool-owned file the import replaces by removing it:
	// a response schema the document no longer has. It is removed only
	// when existing files are overwritten.
	Remove bool
	// Workspace marks an environment, which belongs to the workspace and is
	// written to its environments directory, not to the collection.
	Workspace bool
}

// ApplyOptions decide what happens to files that already exist. Without
// either, ApplyImport refuses (FLW-IMPORT-003) and writes nothing.
type ApplyOptions struct {
	Overwrite    bool // replace them (requests and flows keep their ids)
	SkipExisting bool // leave them and write only the new files
}

// PreviewImport turns an OpenAPI or Arazzo document into the files it
// would add to collection, writing nothing. A document that is not one
// is FLW-IMPORT-002. Arazzo steps call the collection's requests by their
// operation (see Request.Operation), so import OpenAPI first.
//
// Files that already hold a request or flow keep its id, so re-importing
// does not break references to it.
func (w *Workspace) PreviewImport(kind ImportKind, data []byte, collection model.ID) (ImportPlan, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return ImportPlan{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return w.previewIn(c, ImportSource{Kind: kind, Data: data})
}

// PreviewImportSource is PreviewImport for a named source, so the report
// can say where each problem is.
func (w *Workspace) PreviewImportSource(src ImportSource, collection model.ID) (ImportPlan, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return ImportPlan{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return w.previewIn(c, src)
}

// PreviewImportNew plans an import into a collection not created yet at
// dir, for a dry run before InitCollection. The plan cannot be applied:
// create the collection, then preview into it.
func (w *Workspace) PreviewImportNew(kind ImportKind, data []byte, dir string) (ImportPlan, error) {
	return w.PreviewImportSourceNew(ImportSource{Kind: kind, Data: data}, dir)
}

// PreviewImportSourceNew is PreviewImportNew for a named source or a
// folder; like it, the plan cannot be applied.
func (w *Workspace) PreviewImportSourceNew(src ImportSource, dir string) (ImportPlan, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ImportPlan{}, fmt.Errorf("resolving %s: %w", dir, err)
	}
	return w.previewIn(model.Collection{Root: abs}, src)
}

func (w *Workspace) previewIn(c model.Collection, src ImportSource) (ImportPlan, error) {
	if transform, ok := importerFor(src.Kind); ok {
		r, fatal := transform(src, c)
		if fatal.HasErrors() {
			return ImportPlan{}, fatal.Err()
		}
		return w.planResult(c, r)
	}
	var (
		files map[string][]byte
		diags diag.List
		err   error
	)
	switch src.Kind {
	case ImportOpenAPI:
		files, diags, err = w.planOpenAPI(c, src)
	case ImportAsyncAPI:
		files, diags, err = w.planAsyncAPI(c, src)
	case ImportProto:
		files, diags, err = w.planProto(c, src)
	case ImportArazzo:
		files, diags, err = w.planArazzo(c, src.Data)
	default:
		return ImportPlan{}, fmt.Errorf("unknown import kind %q", src.Kind)
	}
	if err != nil {
		return ImportPlan{}, err
	}
	return planOf(c, files, diags, "")
}

// planOf lists the files an import writes. Files in envDir are the
// workspace's environments: they are shown as environments/<name>.yaml
// and are the only files allowed outside the collection.
func planOf(c model.Collection, files map[string][]byte, diags diag.List, envDir string) (ImportPlan, error) {
	plan := ImportPlan{Collection: c.Manifest.ID, Diags: diags, data: files}
	for _, file := range slices.Sorted(maps.Keys(files)) {
		exists, err := fileExists(file)
		if err != nil {
			return ImportPlan{}, err
		}
		same := false
		if exists && files[file] != nil {
			old, err := os.ReadFile(file)
			if err != nil {
				return ImportPlan{}, fmt.Errorf("reading %s: %w", file, err)
			}
			same = bytes.Equal(old, files[file])
		}
		if envDir != "" && filepath.Dir(file) == envDir {
			plan.Files = append(plan.Files, PlannedFile{File: file, Rel: "environments/" + filepath.Base(file), Exists: exists, Workspace: true})
			continue
		}
		// Compare where both really are: a symlink inside the collection must
		// not carry a write out of it.
		rel, err := filepath.Rel(resolvedPath(c.Root), resolvedPath(file))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return ImportPlan{}, fmt.Errorf("import would write %s, outside the collection at %s", file, c.Root)
		}
		plan.Files = append(plan.Files, PlannedFile{File: file, Rel: filepath.ToSlash(rel), Exists: exists, Same: same, Remove: files[file] == nil})
	}
	slices.SortStableFunc(plan.Files, func(a, b PlannedFile) int { return strings.Compare(a.Rel, b.Rel) }) // environments sit elsewhere on disk but are listed in place
	return plan, nil
}

// ApplyImport writes a previewed plan and reloads the collection.
// Existing files are checked again at apply time; see ApplyOptions.
func (w *Workspace) ApplyImport(p ImportPlan, o ApplyOptions) error {
	w.mu.RLock()
	c, ok := w.collection(p.Collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(p.Collection)})
	}
	changes, existing, err := importChanges(p)
	if err != nil {
		return err
	}
	if len(existing) > 0 && !o.Overwrite && !o.SkipExisting {
		return diagErr(diag.CodeImportExists, map[string]string{"count": strconv.Itoa(len(existing)), "files": strings.Join(existing, ", ")})
	}
	if o.SkipExisting && !o.Overwrite {
		changes = slices.DeleteFunc(changes, func(ch store.FileChange) bool { return ch.Old != nil })
	}
	writes, removals := splitRemovals(changes)
	if err := store.Apply(writes); err != nil {
		return err
	}
	for _, file := range removals {
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", file, err)
		}
	}
	if slices.ContainsFunc(p.Files, func(f PlannedFile) bool { return f.Workspace }) {
		if err := w.reloadEnvironments(); err != nil {
			return err
		}
	}
	_, err = w.Reload(Change{Root: c.Root, File: c.Root, Op: store.ChangeRescan})
	return err
}

// splitRemovals separates files to write from files to remove (New nil).
func splitRemovals(changes []store.FileChange) ([]store.FileChange, []string) {
	var writes []store.FileChange
	var removals []string
	for _, ch := range changes {
		if ch.New == nil {
			removals = append(removals, ch.File)
		} else {
			writes = append(writes, ch)
		}
	}
	return writes, removals
}

// importChanges reads what is on disk now for each planned file.
func importChanges(p ImportPlan) ([]store.FileChange, []string, error) {
	var changes []store.FileChange
	var existing []string
	for _, f := range p.Files {
		old, err := os.ReadFile(f.File)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("reading %s: %w", f.File, err)
		}
		if old != nil {
			existing = append(existing, f.Rel)
		}
		changes = append(changes, store.FileChange{File: f.File, Old: old, New: p.data[f.File]})
	}
	return changes, existing, nil
}

// planOpenAPI maps requests to requests/<path>.req.yaml, their response
// schemas to <request>.schemas/<version>-<status>.json and suggested
// flows to flows/<path>.arazzo.yaml.
func (w *Workspace) planOpenAPI(c model.Collection, src ImportSource) (map[string][]byte, diag.List, error) {
	kept, hasKept := keptSpecNamed(c.Root, specBaseName(src))
	// The overlay Flow-I saves request edits to always applies, after the
	// overlays the import names: it is part of the kept spec.
	overlays, err := withEdits(src.Overlays, kept, hasKept)
	if err != nil {
		return nil, nil, err
	}
	data, overlayDiags := specoverlay.Apply(src.Data, overlays)
	if overlayDiags.HasErrors() {
		return nil, nil, overlayDiags.Err()
	}
	r, diags := openapi.Import(data, openapi.Options{NewID: w.newID})
	if diags.HasErrors() {
		return nil, nil, diags.Err()
	}
	diags = append(overlayDiags, diags...)
	base := previousImport(kept, hasKept)
	ids := map[model.ID]model.ID{} // imported id → existing id it keeps
	files := map[string][]byte{}
	for _, req := range r.Requests {
		req, mergeDiags := mergeWithExisting(c, base, req)
		diags = append(diags, mergeDiags...)
		if err := w.planRequest(c, req, r.Schemas[req.Path], ids, files); err != nil {
			return nil, nil, err
		}
	}
	if err := w.planFlows(c, r.Flows, ids, files); err != nil {
		return nil, nil, err
	}
	planSpecCopy(c, src, "openapi", files)
	return files, diags, nil
}

// planAsyncAPI maps the operations of an AsyncAPI document to async requests
// under requests/, keeping the document in descriptions/.
func (w *Workspace) planAsyncAPI(c model.Collection, src ImportSource) (map[string][]byte, diag.List, error) {
	data, overlayDiags := specoverlay.Apply(src.Data, src.Overlays)
	if overlayDiags.HasErrors() {
		return nil, nil, overlayDiags.Err()
	}
	r, diags := asyncapi.Import(data, asyncapi.Options{NewID: w.newID})
	if diags.HasErrors() {
		return nil, nil, diags.Err()
	}
	diags = append(overlayDiags, diags...)
	ids := map[model.ID]model.ID{}
	files := map[string][]byte{}
	for _, req := range r.Requests {
		if _, err := w.planRequestFile(c, req, ids, files); err != nil {
			return nil, nil, err
		}
	}
	planSpecCopy(c, src, "asyncapi", files)
	return files, diags, nil
}

// planProto maps each unary or server-streaming method of a .proto file to a
// gRPC request under requests/<Service>/<Method>, and keeps the .proto file and
// the local files it imports under protos/ for the requests to compile. Client
// and bidirectional streaming have no request kind yet: they are reported.
func (w *Workspace) planProto(c model.Collection, src ImportSource) (map[string][]byte, diag.List, error) {
	dir := ""
	if src.Name != "" {
		dir = filepath.Dir(src.Name)
	}
	p, err := grpcx.ReadProto(context.Background(), src.Name, src.Data, dir)
	if err != nil {
		return nil, nil, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": err.Error()})}.Err()
	}
	files := map[string][]byte{}
	for _, f := range p.Files {
		files[filepath.Join(c.Root, "protos", filepath.FromSlash(f.Path))] = f.Data
	}
	main := "protos/" + p.Files[0].Path
	ids := map[model.ID]model.ID{}
	var diags diag.List
	for _, m := range p.Methods {
		if m.ClientStream {
			diags = append(diags, diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
				"what": "method " + m.Full(), "how": "skipped", "detail": "client and bidirectional streaming are not supported"}))
			continue
		}
		service := m.Service[strings.LastIndex(m.Service, ".")+1:]
		g := model.GRPC{Server: "{{env.grpc_server}}", Method: m.Full(), Body: model.Body{JSON: m.Example}, Proto: []string{main}, Import: []string{"protos"}}
		if m.ServerStream {
			g.Receive = &model.AsyncReceive{}
		}
		req := model.Request{Meta: model.Meta{ID: w.newID(), Name: service + "." + m.Name}, Path: service + "/" + m.Name,
			Versions: []model.RequestVersion{{Label: "1", GRPC: &g}}}
		if _, err := w.planRequestFile(c, req, ids, files); err != nil {
			return nil, nil, err
		}
	}
	return files, diags, nil
}

// planSpecCopy keeps the imported document, as it was, in
// descriptions/<name>.<kind>.<yaml|json> (and its overlays beside it as
// <name>.overlay.yaml, <name>.overlay-2.yaml, …): the generated description
// (requests.openapi.yaml) sits beside it and is never overwritten by it.
// The name is the source file's, else "imported".
func planSpecCopy(c model.Collection, src ImportSource, kind string, files map[string][]byte) {
	base := specBaseName(src)
	ext := ".yaml"
	if trimmed := strings.TrimSpace(string(src.Data)); strings.HasPrefix(trimmed, "{") {
		ext = ".json"
	}
	files[filepath.Join(c.Root, "descriptions", base+"."+kind+ext)] = src.Data
	for i, ov := range src.Overlays {
		suffix := ".overlay"
		if i > 0 {
			suffix = fmt.Sprintf(".overlay-%d", i+1)
		}
		files[filepath.Join(c.Root, "descriptions", base+suffix+".yaml")] = ov
	}
}

// specBaseName is the name a source is kept under in descriptions/: the
// source file's, else "imported".
func specBaseName(src ImportSource) string {
	base := ""
	if src.Name != "" {
		base = strings.TrimSuffix(filepath.Base(src.Name), filepath.Ext(src.Name))
		base = importers.FileName(strings.TrimSuffix(strings.TrimSuffix(base, ".openapi"), ".asyncapi"))
	}
	if base == "" || base == "requests" { // "requests" is the generated description's name
		base = "imported"
	}
	return base
}

func (w *Workspace) planRequest(c model.Collection, req model.Request, schemas map[string]map[string]any, ids map[model.ID]model.ID, files map[string][]byte) error {
	file, err := w.planRequestFile(c, req, ids, files)
	if err != nil {
		return err
	}
	dir := strings.TrimSuffix(file, ".req.yaml") + ".schemas"
	for version, byStatus := range schemas {
		for status, schema := range byStatus {
			data, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				return fmt.Errorf("encoding the %s %s schema of %s: %w", version, status, req.Path, err)
			}
			files[filepath.Join(dir, version+"-"+status+".json")] = append(data, '\n')
		}
	}
	return planStaleSchemas(dir, files)
}

// planRequestFile plans requests/<path>.req.yaml and returns the file. A
// request replacing an existing file keeps its id (recorded in ids); a
// request without an id gets a fresh one. It also keeps the existing
// file's dirty-edit slot.
func (w *Workspace) planRequestFile(c model.Collection, req model.Request, ids map[model.ID]model.ID, files map[string][]byte) (string, error) {
	if detail := requestPathProblem(req.Path); detail != "" {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": req.Path, "detail": detail})
	}
	req.File = filepath.Join(c.Root, "requests", filepath.FromSlash(req.Path)+".req.yaml")
	if req.ID == "" {
		req.ID = w.newID()
	}
	if i := slices.IndexFunc(c.Requests, func(x model.Request) bool { return x.File == req.File }); i >= 0 {
		ids[req.ID], req.ID = c.Requests[i].ID, c.Requests[i].ID
		req.Edit, req.EditBase = c.Requests[i].Edit, c.Requests[i].EditBase // the user's dirty edit survives a re-import
	}
	data, err := w.store.EncodeRequest(req)
	if err != nil {
		return "", fmt.Errorf("encoding %s: %w", req.File, err)
	}
	files[req.File] = data
	return req.File, nil
}

// planStaleSchemas plans the removal of schema files in dir the import
// does not write: the document no longer has those responses, and a
// stale schema would keep judging consumers.
func planStaleSchemas(dir string, files map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		file := filepath.Join(dir, e.Name())
		if _, planned := files[file]; !planned && !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files[file] = nil
		}
	}
	return nil
}

// planFlows encodes flows (Path relative to flows/), pointing their
// request and subflow refs at kept ids and keeping existing flows' ids.
func (w *Workspace) planFlows(c model.Collection, flows []model.Flow, ids map[model.ID]model.ID, files map[string][]byte) error {
	for i := range flows {
		if detail := requestPathProblem(flows[i].Path); detail != "" {
			return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": flows[i].Path, "detail": detail})
		}
		flows[i].File = filepath.Join(c.Root, "flows", filepath.FromSlash(flows[i].Path)+flowSuffix)
		if flows[i].ID == "" {
			flows[i].ID = w.newID()
		}
		if j := slices.IndexFunc(c.Flows, func(x model.Flow) bool { return x.File == flows[i].File }); j >= 0 {
			ids[flows[i].ID], flows[i].ID = c.Flows[j].ID, c.Flows[j].ID
		}
	}
	for _, f := range flows {
		f.Nodes = keptRefs(f.Nodes, ids)
		data, err := w.store.EncodeFlow(c, f)
		if err != nil {
			return fmt.Errorf("encoding %s: %w", f.File, err)
		}
		files[f.File] = data
	}
	return nil
}

// keptRefs returns nodes with refs to re-imported entities moved to the
// ids those entities keep.
func keptRefs(nodes []model.Node, ids map[model.ID]model.ID) []model.Node {
	out := slices.Clone(nodes)
	for i, n := range out {
		if n.Request != nil {
			call := *n.Request
			call.Request.ID = cmp.Or(ids[call.Request.ID], call.Request.ID)
			out[i].Request = &call
		}
		if n.Subflow != nil {
			call := *n.Subflow
			call.Flow.ID = cmp.Or(ids[call.Flow.ID], call.Flow.ID)
			out[i].Subflow = &call
		}
	}
	return out
}

// planArazzo maps workflows to flows/arazzo/<workflowId>.arazzo.yaml,
// calling the collection's requests by operation.
func (w *Workspace) planArazzo(c model.Collection, data []byte) (map[string][]byte, diag.List, error) {
	r, diags := arazzo.Import(data, arazzo.Options{NewID: w.newID, Resolve: collectionResolver{c: c}})
	if diags.HasErrors() {
		return nil, nil, diags.Err()
	}
	files := map[string][]byte{}
	if err := w.planFlows(c, r.Flows, map[model.ID]model.ID{}, files); err != nil {
		return nil, nil, err
	}
	return files, diags, nil
}

// importerFor returns the transform for kinds that produce an
// importers.Result; OpenAPI and Arazzo have their own planning.
func importerFor(kind ImportKind) (importer, bool) {
	f, ok := map[ImportKind]importer{
		ImportCurl: func(s ImportSource, _ model.Collection) (importers.Result, diag.List) {
			return curl.Import(s.Data, s.Name)
		},
		ImportHAR: func(s ImportSource, _ model.Collection) (importers.Result, diag.List) {
			return har.Import(s.Data, s.Name)
		},
		ImportHTTP: func(s ImportSource, _ model.Collection) (importers.Result, diag.List) {
			return httpfile.Import(s.Data, s.Name)
		},
		ImportPostman: func(s ImportSource, _ model.Collection) (importers.Result, diag.List) {
			return postman.Import(s.Data, s.Name)
		},
		ImportBruno: func(s ImportSource, _ model.Collection) (importers.Result, diag.List) {
			return bruno.Import(os.DirFS(s.Dir), cmp.Or(s.Name, s.Dir))
		},
		ImportMermaid: func(s ImportSource, c model.Collection) (importers.Result, diag.List) {
			return mermaid.Import(s.Data, s.Name, pathResolver{c: c})
		},
	}[kind]
	return f, ok
}

// importer is a transform that produces an importers.Result; Mermaid
// resolves labels against the target collection.
type importer func(ImportSource, model.Collection) (importers.Result, diag.List)

// pathResolver finds a collection's requests and flows by path.
type pathResolver struct{ c model.Collection }

func (r pathResolver) Request(p string) (model.Ref, bool) {
	i := slices.IndexFunc(r.c.Requests, func(x model.Request) bool { return x.Path == p })
	if i < 0 {
		return model.Ref{}, false
	}
	return model.Ref{ID: r.c.Requests[i].ID, Path: p}, true
}

func (r pathResolver) Flow(p string) (model.Ref, bool) {
	i := slices.IndexFunc(r.c.Flows, func(x model.Flow) bool { return x.Path == p })
	if i < 0 {
		return model.Ref{}, false
	}
	return model.Ref{ID: r.c.Flows[i].ID, Path: p}, true
}

// ExportMermaid writes flow t as a Mermaid flowchart.
func (w *Workspace) ExportMermaid(t FlowTarget) (string, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return "", err
	}
	return mermaid.Export(f), nil
}

// planResult plans what an importer (Postman, Bruno, HAR, curl, Mermaid)
// produced. Importers leave ids empty and point flows at requests by
// path; ids are assigned here, existing files keep theirs, and path refs
// resolve to the planned or existing request.
func (w *Workspace) planResult(c model.Collection, r importers.Result) (ImportPlan, error) {
	ids := map[model.ID]model.ID{}
	files := map[string][]byte{}
	byPath := map[string]model.ID{}
	for _, x := range c.Requests {
		byPath[x.Path] = x.ID
	}
	for _, req := range r.Requests {
		req.ID = w.newID()
		if _, err := w.planRequestFile(c, req, ids, files); err != nil {
			return ImportPlan{}, err
		}
		byPath[req.Path] = cmp.Or(ids[req.ID], req.ID)
	}
	for _, ex := range r.Examples {
		if err := planExample(c, ex, files); err != nil {
			return ImportPlan{}, err
		}
	}
	for _, env := range r.Environments {
		if err := w.planEnvironment(env, files); err != nil {
			return ImportPlan{}, err
		}
	}
	flows := slices.Clone(r.Flows)
	for i := range flows {
		flows[i].Nodes = refsByPath(flows[i].Nodes, byPath)
	}
	if err := w.planFlows(c, flows, ids, files); err != nil {
		return ImportPlan{}, err
	}
	plan, err := planOf(c, files, r.Diags(), workspace.EnvironmentsDir(w.opts.Paths, w.opts.Name))
	plan.Items = r.Items
	return plan, err
}

// refsByPath fills request refs that have only a path. Importers only
// produce top-level request nodes, so nested lists are not walked.
func refsByPath(nodes []model.Node, byPath map[string]model.ID) []model.Node {
	out := slices.Clone(nodes)
	for i, n := range out {
		if n.Request != nil && n.Request.Request.ID == "" && n.Request.Request.Collection == "" {
			call := *n.Request
			call.Request.ID = byPath[call.Request.Path]
			out[i].Request = &call
		}
	}
	return out
}

// planExample plans <request>.examples/<name>.json.
func planExample(c model.Collection, ex importers.Example, files map[string][]byte) error {
	if detail := requestPathProblem(ex.RequestPath); detail != "" || !names.Valid(ex.Name) {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": ex.RequestPath + " example " + ex.Name, "detail": cmp.Or(detail, "not a valid example name")})
	}
	data, err := json.MarshalIndent(ex, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding example %s of %s: %w", ex.Name, ex.RequestPath, err)
	}
	file := filepath.Join(c.Root, "requests", filepath.FromSlash(ex.RequestPath)+".examples", ex.Name+".json")
	files[file] = append(data, '\n')
	return nil
}

// planEnvironment plans <workspace>/environments/<name>.yaml, keeping an
// existing environment's id. Imported environments are global. Secret variables are declared without values.
func (w *Workspace) planEnvironment(env model.Environment, files map[string][]byte) error {
	name := importers.FileName(env.Name)
	if !names.Valid(name) {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": "environments/" + env.Name, "detail": "not a valid environment name"})
	}
	env.File = filepath.Join(workspace.EnvironmentsDir(w.opts.Paths, w.opts.Name), name+".yaml")
	env.ID = w.newID()
	if existing := w.Environments(); slices.ContainsFunc(existing, func(x model.Environment) bool { return x.File == env.File }) {
		env.ID = existing[slices.IndexFunc(existing, func(x model.Environment) bool { return x.File == env.File })].ID
	}
	for i := range env.Vars {
		if env.Vars[i].Secret {
			env.Vars[i].Value = ""
		}
	}
	data, err := w.store.EncodeEnvironment(env)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", env.File, err)
	}
	files[env.File] = data
	return nil
}

// ExportArazzo writes flow t, and the flows of its collection it calls as
// subflows, as one standalone Arazzo document for other tools: standard steps
// only (what Arazzo cannot express is left out and reported in diags), naming
// the requests by the operationIds of the collection's generated description.
// Copy descriptions/requests.openapi.yaml (and requests.asyncapi.yaml when the
// flow has async steps) next to the document, as
// ./requests.openapi.yaml and ./requests.asyncapi.yaml, for the operations to resolve.
func (w *Workspace) ExportArazzo(t FlowTarget) ([]byte, diag.List, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return nil, nil, err
	}
	out, diags := arazzo.Export(calledFlows(c, f), arazzo.ExportOptions{
		Resolve: arazzo.NewResolver(c), Source: "flowi", SourceURL: "./requests.openapi.yaml", Title: f.Name,
	})
	return out, diags, nil
}

// calledFlows returns f followed by every flow of c it calls, directly or
// through other subflows, each once.
func calledFlows(c model.Collection, f model.Flow) []model.Flow {
	out := []model.Flow{f}
	seen := map[model.ID]bool{f.ID: true}
	for i := 0; i < len(out); i++ {
		for _, id := range subflowIDs(out[i].Nodes) {
			j := slices.IndexFunc(c.Flows, func(x model.Flow) bool { return x.ID == id })
			if j >= 0 && !seen[id] {
				seen[id] = true
				out = append(out, c.Flows[j])
			}
		}
	}
	return out
}

func subflowIDs(nodes []model.Node) []model.ID {
	var out []model.ID
	for _, n := range nodes {
		if n.Subflow != nil && n.Subflow.Flow.Collection == "" {
			out = append(out, n.Subflow.Flow.ID)
		}
		for _, list := range n.Children() {
			out = append(out, subflowIDs(list)...)
		}
	}
	return out
}

// collectionResolver finds requests by the operation they were imported
// from, and describes a request call's parameters for export. A request's
// operation is its first version (OpenAPI import keeps the lowest API
// version's operationId), so calls are pinned to that version and calls
// that may run another version have no known operation.
type collectionResolver struct{ c model.Collection }

func (r collectionResolver) RequestFor(operation string) (model.Ref, string, bool) {
	i := slices.IndexFunc(r.c.Requests, func(x model.Request) bool { return x.Operation == operation && len(x.Versions) > 0 })
	if i < 0 {
		return model.Ref{}, "", false
	}
	req := r.c.Requests[i]
	return model.Ref{ID: req.ID, Path: req.Path}, req.Versions[0].Label, true
}

func (r collectionResolver) OperationOf(call model.RequestCall) (arazzo.Operation, bool) {
	if call.Request.Collection != "" {
		return arazzo.Operation{}, false
	}
	i := slices.IndexFunc(r.c.Requests, func(x model.Request) bool { return x.ID == call.Request.ID })
	if i < 0 || r.c.Requests[i].Operation == "" || len(r.c.Requests[i].Versions) == 0 {
		return arazzo.Operation{}, false
	}
	req := r.c.Requests[i]
	// An unpinned call's version also depends on the flow's and the run's
	// preferences, so it is only certain with a single version.
	if pinned := call.Version; pinned != req.Versions[0].Label && (pinned != "" || len(req.Versions) > 1) {
		return arazzo.Operation{}, false
	}
	return arazzo.Operation{ID: req.Operation, Params: paramsOf(req.Versions[0])}, true
}

var placeholder = regexp.MustCompile(`^\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}$`)

// paramsOf finds the parameters a version is filled through: {{name}}
// path segments, and query entries and headers whose value is {{name}}.
func paramsOf(v model.RequestVersion) map[string]arazzo.Param {
	out := map[string]arazzo.Param{}
	for _, seg := range strings.Split(v.URL, "/") {
		if m := placeholder.FindStringSubmatch(seg); m != nil {
			out[m[1]] = arazzo.Param{Name: m[1], In: "path"}
		}
	}
	for in, list := range map[string][]model.KeyValue{"query": v.Query, "header": v.Headers} {
		for _, kv := range list {
			if m := placeholder.FindStringSubmatch(kv.Value); m != nil {
				out[m[1]] = arazzo.Param{Name: kv.Key, In: in}
			}
		}
	}
	return out
}

func fileExists(file string) (bool, error) {
	_, err := os.Stat(file)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, fmt.Errorf("checking %s: %w", file, err)
}

// FindCollection resolves a collection by name or id; "" means the only
// mounted collection (FLW-WS-004 when there are several, FLW-WS-005 for
// an unknown name).
func (w *Workspace) FindCollection(ref string) (model.ID, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, err := w.oneCollection(ref)
	if err != nil {
		return "", err
	}
	return c.Manifest.ID, nil
}
