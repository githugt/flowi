// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

// SaveOptions control writes of edited files.
type SaveOptions struct {
	// DropComments allows rewriting a file that has comments. The
	// canonical encoding does not keep them, so without it such a write is
	// refused with FLW-WRITE-001.
	DropComments bool
	// Expect is the FileSum the editor got when it loaded the file. When it is set and the file on disk no longer has that
	// sum (an editor or git changed it), the write is refused with FLW-WRITE-005 instead of overwriting the change.
	// Before: a save wrote over whatever was on disk, with no warning. Empty skips the check, which is also how an
	// "overwrite anyway" save is made.
	Expect string
}

// FileSum is a short fingerprint of a file's bytes ("" for a file that does not exist yet), for SaveOptions.Expect.
func FileSum(data []byte) string {
	if data == nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FlowSum is the FileSum of flow t's file as it is on disk now.
func (w *Workspace) FlowSum(t FlowTarget) (string, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return "", err
	}
	return w.sumOf(f.File)
}

// RequestSum is the FileSum of request t's file as it is on disk now.
func (w *Workspace) RequestSum(t RequestTarget) (string, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return "", err
	}
	return w.sumOf(r.File)
}

func (w *Workspace) sumOf(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("reading %s: %w", file, err)
	}
	return FileSum(data), nil
}

// SaveRequest writes r, an edited copy of request t, in canonical form and
// reloads it. Problems are *diag.ListError: FLW-REQ-001 (unknown request),
// FLW-WRITE-001 (the file has comments and o.DropComments is false).
func (w *Workspace) SaveRequest(t RequestTarget, r model.Request, o SaveOptions) error {
	_, current, err := w.requestFor(t)
	if err != nil {
		return err
	}
	r.ID, r.File, r.Path = current.ID, current.File, current.Path // identity and location are not editable here
	data, err := w.store.EncodeRequest(r)                         // the working copy is not in the file: see edit_store.go
	if err != nil {
		return fmt.Errorf("encoding %s: %w", r.File, err)
	}
	if err := w.writeAndReload(t.Collection, r.File, data, o); err != nil {
		return err
	}
	// The file is saved; now the working copy, which a promotion or a discard clears. After, so that a refused save
	// (a commented file, a file changed on disk) does not lose the draft.
	if sameEdit(current, r) {
		return nil
	}
	if err := w.writeEdit(t.Collection, r); err != nil {
		return err
	}
	w.setEdit(t.Collection, r)
	return nil
}

// NewRequest creates requests/<path>.req.yaml in collection with a fresh
// id and one empty GET version "1", and loads it. path is slash-separated
// and each segment must be a plain name; an existing file or a bad path
// is FLW-WRITE-002.
func (w *Workspace) NewRequest(collection model.ID, path string) (RequestTarget, error) {
	return w.newRequestWith(collection, path, model.RequestVersion{Label: "1", Method: "GET"})
}

// NewAsyncRequest is NewRequest for an async request of the given protocol
// (any of model.AsyncProtocols): its version "1" has an empty server
// and channel, and sends (sse: receives) one message. An unknown protocol is
// FLW-ASYNC-001.
func (w *Workspace) NewAsyncRequest(collection model.ID, path, protocol string) (RequestTarget, error) {
	if !model.KnownProtocol(protocol) {
		return RequestTarget{}, diagErr(diag.CodeAsyncProtocol, map[string]string{"protocol": protocol})
	}
	a := &model.Async{Protocol: protocol}
	if protocol == model.ProtocolServiceBus {
		// A new Service Bus request only looks at its queue: receiving removes the
		// messages it keeps and counts a delivery for every other one it sees, which
		// a test of a live queue should not do. It is written into the file, where
		// it can be seen and changed.
		a.Binding = []model.KeyValue{{Key: "read", Value: "peek"}}
	}
	if protocol == model.ProtocolSSE {
		a.Receive = &model.AsyncReceive{}
	} else {
		a.Send = &model.AsyncSend{}
	}
	return w.newRequestWith(collection, path, model.RequestVersion{Label: "1", Async: a})
}

// NewGRPCRequest is NewRequest for a gRPC request: its version "1" has an
// empty server and method and an empty JSON message.
func (w *Workspace) NewGRPCRequest(collection model.ID, path string) (RequestTarget, error) {
	return w.newRequestWith(collection, path, model.RequestVersion{Label: "1", GRPC: &model.GRPC{}})
}

// NewDatasetRequest is NewRequest for a dataset request of the given driver (any of model.DatasetDrivers): its version
// "1" has a server and a query that run as they are on any store of that driver, and one param to show how a value
// is passed. An unknown driver is FLW-DATA-001.
func (w *Workspace) NewDatasetRequest(collection model.ID, path, driver string) (RequestTarget, error) {
	starter, ok := datasetStarters()[driver]
	if !ok {
		return RequestTarget{}, diagErr(diag.CodeDatasetRequest, map[string]string{"detail": "the driver is one of " + strings.Join(model.DatasetDrivers(), ", ")})
	}
	d := model.Dataset{Driver: driver, Server: starter.server, Query: starter.query, Params: []string{"hello"}}
	return w.newRequestWith(collection, path, model.RequestVersion{Label: "1", Dataset: &d})
}

// datasetStarters is the server and query a new dataset request of each driver starts with. The server has no
// password: the owner adds it as {{secret.name}}.
func datasetStarters() map[string]struct{ server, query string } {
	return map[string]struct{ server, query string }{
		model.DriverPostgres: {"postgres://postgres@localhost:5432/postgres", "SELECT $1::text AS greeting"},
		model.DriverMySQL:    {"mysql://root@localhost:3306/mysql", "SELECT ? AS greeting"},
		model.DriverMariaDB:  {"mariadb://root@localhost:3306/mysql", "SELECT ? AS greeting"},
		model.DriverSQLite:   {"data.db", "SELECT ? AS greeting"},
	}
}

// fileStarters is the path and query a new file request of each format starts with.
func fileStarters() map[string]struct{ path, query string } {
	return map[string]struct{ path, query string }{
		model.FormatCSV:     {"data.csv", "SELECT * FROM data LIMIT 10"},
		model.FormatXLSX:    {"data.xlsx", "SELECT * FROM Sheet1 LIMIT 10"},
		model.FormatParquet: {"data.parquet", "SELECT * FROM data LIMIT 10"},
	}
}

// NewFileRequest is NewRequest for a data file request of the given format (any of model.DataFileFormats): its
// version "1" has a path and a query that run as they are on a file of that format. An unknown format is FLW-DATA-001.
func (w *Workspace) NewFileRequest(collection model.ID, path, format string) (RequestTarget, error) {
	starter, ok := fileStarters()[format]
	if !ok {
		return RequestTarget{}, diagErr(diag.CodeDatasetRequest, map[string]string{"detail": "the format is one of " + strings.Join(model.DataFileFormats(), ", ")})
	}
	f := model.DataFile{Format: format, Path: starter.path, Query: starter.query}
	return w.newRequestWith(collection, path, model.RequestVersion{Label: "1", File: &f})
}

// GeneratedGRPC is what GenerateGRPCRequests did.
type GeneratedGRPC struct {
	Created []RequestTarget
	// Skipped says, one line per method, why no request was made for it.
	Skipped []string
}

// grpcFolder is where generated gRPC requests go, under requests/.
const grpcFolder = "grpc"

// GenerateGRPCRequests creates a gRPC request for each method, at
// grpc/<Service>/<Method> of collection, with the server, proto files and
// import roots of base, the method's example message as its body, and the
// starting hooks. A method that streams from the client starts with an array of
// one example message. A method whose request already exists is skipped and reported.
func (w *Workspace) GenerateGRPCRequests(collection model.ID, base model.GRPC, methods []grpcx.Method) (GeneratedGRPC, error) {
	var out GeneratedGRPC
	for _, m := range methods {
		body := m.Example
		if m.ClientStream {
			body = "[" + m.Example + "]" // one example message; add more elements to send more
		}
		g := model.GRPC{Server: base.Server, Method: m.Full(), Body: model.Body{JSON: body},
			Proto: slices.Clone(base.Proto), Import: slices.Clone(base.Import)}
		path := grpcFolder + "/" + shortName(m.Service) + "/" + m.Name
		target, err := w.newRequestWith(collection, path, model.RequestVersion{Label: "1", GRPC: &g})
		var le *diag.ListError
		if errors.As(err, &le) && len(le.List) > 0 && le.List[0].Code == diag.CodeWriteRequestPath {
			out.Skipped = append(out.Skipped, path+": it already exists or is not a usable path")
			continue
		}
		if err != nil {
			return out, err
		}
		if err := w.SeedRequestHooks(target); err != nil {
			return out, err
		}
		out.Created = append(out.Created, target)
	}
	return out, nil
}

// shortName is the last part of a dotted name: shop.v1.Shop is Shop.
func shortName(full string) string {
	return full[strings.LastIndex(full, ".")+1:]
}

func (w *Workspace) newRequestWith(collection model.ID, path string, version model.RequestVersion) (RequestTarget, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return RequestTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	path, name := pathForName(path)
	if detail := requestPathProblem(path); detail != "" {
		return RequestTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	file := filepath.Join(c.Root, "requests", filepath.FromSlash(path)+".req.yaml")
	if _, err := os.Stat(file); err == nil {
		return RequestTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": "it already exists"})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return RequestTarget{}, fmt.Errorf("checking %s: %w", file, err)
	}
	r := model.Request{Meta: model.Meta{ID: w.newID(), Name: name}, Path: path, File: file,
		Versions: []model.RequestVersion{version}}
	data, err := w.store.EncodeRequest(r)
	if err != nil {
		return RequestTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(collection, file, data, SaveOptions{}); err != nil {
		return RequestTarget{}, err
	}
	w.noteOrigin(collection, activityRequest, r.ID, OriginCreated, "")
	return RequestTarget{Collection: collection, Request: r.ID}, nil
}

// pathForName is the path a new request typed as path is kept at, and its name. The last part is what the request
// is called, so it may be written as a name ("Test postgres"): the file then takes it with each run of other
// characters as one "-" (Test-postgres) and the request keeps the name as typed. Letters of any script stay as they
// are (Café-menu). Folders stay plain names.
func pathForName(path string) (string, string) {
	dir, name := "", path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		dir, name = path[:i+1], path[i+1:]
	}
	name = names.Normalize(strings.TrimSpace(name))
	if names.Valid(name) || strings.HasPrefix(name, ".") {
		return dir + name, name // a leading dot is refused as it is: it would hide the file
	}
	var file strings.Builder
	gap := false
	for _, r := range name {
		if names.Valid(string(r)) || (r == '.' && file.Len() > 0) {
			if gap && file.Len() > 0 {
				file.WriteByte('-')
			}
			gap = false
			file.WriteRune(r)
			continue
		}
		gap = true
	}
	return dir + strings.TrimRight(file.String(), "."), name // a name may not end in a dot: Windows drops it
}

// NewFolder creates the folder path under tree ("requests" or "flows") of
// collection, as a _folder.yaml, so it exists and shows before anything is
// put in it. path is slash-separated, each part a plain name; a bad path,
// an unknown tree or an existing folder is FLW-WRITE-002.
func (w *Workspace) NewFolder(collection model.ID, tree, path string) error {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	fail := func(detail string) error {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	if tree != "requests" && tree != "flows" {
		return fail("folders go under requests or flows")
	}
	// "Test data" is a fine folder name: each part takes the file-safe form (Test-data), as a request's file does.
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i], _ = pathForName(p)
	}
	path = strings.Join(parts, "/")
	if detail := requestPathProblem(path); detail != "" {
		return fail(detail)
	}
	dir := tree + "/" + path
	file := filepath.Join(c.Root, filepath.FromSlash(dir), "_folder.yaml")
	if _, err := os.Stat(file); err == nil {
		return fail("it already exists")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", file, err)
	}
	data, err := w.store.EncodeFolder(model.Folder{Dir: dir, File: file})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", file, err)
	}
	return w.writeAndReload(collection, file, data, SaveOptions{})
}

func requestPathProblem(path string) string {
	if path == "" {
		return "the path is empty"
	}
	for _, seg := range strings.Split(path, "/") {
		if !names.Valid(seg) {
			return "each part must use letters, digits, '.', '_' or '-' and not start with '.'"
		}
	}
	return ""
}

// writeAndReload writes data to file atomically, refusing to drop
// comments unless allowed, then reloads the file into its collection.
func (w *Workspace) writeAndReload(collection model.ID, file string, data []byte, o SaveOptions) error {
	old, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", file, err)
	}
	if o.Expect != "" && FileSum(old) != o.Expect {
		return diag.List{diag.New(diag.CodeWriteChangedOnDisk, diag.Location{File: file}, map[string]string{"file": filepath.Base(file)})}.Err()
	}
	if !o.DropComments && old != nil && w.store.HasComments(old) {
		return diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{File: file}, map[string]string{"file": filepath.Base(file)})}.Err()
	}
	if err := store.Apply([]store.FileChange{{File: file, Old: old, New: data}}); err != nil {
		return err
	}
	return w.reloadFile(collection, file)
}

// reloadFile re-reads file into its collection, then refreshes the generated
// description when file is a request or a flow.
func (w *Workspace) reloadFile(collection model.ID, file string) error {
	if err := w.reloadEntity(collection, file); err != nil {
		return err
	}
	if w.describesFile(collection, file) {
		return w.RegenerateDescription(collection)
	}
	return nil
}

func (w *Workspace) reloadEntity(collection model.ID, file string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == collection })
	if i < 0 {
		return nil // unmounted meanwhile: nothing to refresh
	}
	next, diags, err := w.store.ReloadFile(w.collections[i], file)
	if err != nil {
		return fmt.Errorf("reloading %s: %w", file, err)
	}
	w.collections[i] = w.withEdits(next)
	w.replaceLoadDiags(func(f string) bool { return f == file }, diags)
	return nil
}

// SetFolderOrder sets the order the entries of folder dir under tree
// ("requests" or "flows") are shown in: names are the entries' names (a request
// or flow without its file suffix, or a sub-folder's name). dir is "" for the top
// of the tree. The order is kept in the folder's _folder.yaml, which is created
// when it is not there yet, so it travels with the collection.
func (w *Workspace) SetFolderOrder(collection model.ID, tree, dir string, order []string, o SaveOptions) error {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	fail := func(detail string) error {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": dir, "detail": detail})
	}
	if tree != "requests" && tree != "flows" {
		return fail("folders go under requests or flows")
	}
	if dir != "" {
		if detail := requestPathProblem(dir); detail != "" {
			return fail(detail)
		}
	}
	for _, name := range order {
		if !names.Valid(name) {
			return fail("the name " + name + " is not a plain name")
		}
	}
	rel := strings.Trim(tree+"/"+dir, "/")
	folder := model.Folder{Dir: rel, File: filepath.Join(c.Root, filepath.FromSlash(rel), "_folder.yaml")}
	if i := slices.IndexFunc(c.Folders, func(f model.Folder) bool { return f.Dir == rel }); i >= 0 {
		folder = c.Folders[i]
	}
	folder.Order = slices.Clone(order)
	data, err := w.store.EncodeFolder(folder)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", folder.File, err)
	}
	return w.writeAndReload(collection, folder.File, data, o)
}
