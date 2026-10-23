// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/vault"
	"github.com/githugt/flowi-lib/workspace"
)

// RequestTarget identifies a request of a mounted collection.
type RequestTarget struct {
	Collection model.ID
	Request    model.ID
}

// SendOptions configure SendRequest.
type SendOptions struct {
	Env           string            // environment name; "" uses defaults.environment
	Version       string            // pin a version
	PreferVersion string            // use this version if the request has it
	Save          string            // none | last | per-status; "" uses responses.save
	With          map[string]string // values for the request's bare names ({{email}}), as templates
	Hooks         model.HookControl
	Seed          uint64 // 0 picks a random seed (reported in the result)

	// UseEdit sends the request's dirty edit (see StoreEdit) in place of
	// the saved version it started from, when the version asked for
	// (Version, or the default) is that base. Flow runs, serve and the CLI
	// never set it.
	UseEdit bool
}

// FindRequest resolves a request reference as users type it: an id, or a
// path ("auth/login", optionally with requests/ and .req.yaml). collection
// narrows the search by name or id. Problems are *diag.ListError:
// FLW-REQ-001 (no match), FLW-REQ-002 (several collections match),
// FLW-WS-005 (unknown collection).
func (w *Workspace) FindRequest(ref, collection string) (RequestTarget, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	candidates, err := w.collectionsNamed(collection)
	if err != nil {
		return RequestTarget{}, err
	}
	path := strings.TrimSuffix(strings.TrimPrefix(ref, "requests/"), ".req.yaml")
	var found []RequestTarget
	for _, c := range candidates {
		for _, r := range c.Requests {
			if string(r.ID) == ref || r.Path == path {
				found = append(found, RequestTarget{Collection: c.Manifest.ID, Request: r.ID})
				break
			}
		}
	}
	switch len(found) {
	case 0:
		return RequestTarget{}, diagErr(diag.CodeRequestNotFound, map[string]string{"ref": ref})
	case 1:
		return found[0], nil
	}
	return RequestTarget{}, diagErr(diag.CodeRequestAmbiguous, map[string]string{"ref": ref, "count": strconv.Itoa(len(found))})
}

// collectionsNamed returns every loaded collection, or the one named
// (name or id). Callers hold mu.
func (w *Workspace) collectionsNamed(name string) ([]model.Collection, error) {
	if name == "" {
		return w.collections, nil
	}
	for _, c := range w.collections {
		if c.Manifest.Name == name || string(c.Manifest.ID) == name {
			return []model.Collection{c}, nil
		}
	}
	return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": name})
}

// oneCollection is collectionsNamed for commands that act on a single
// collection: "" is allowed only when exactly one is mounted.
func (w *Workspace) oneCollection(name string) (model.Collection, error) {
	cs, err := w.collectionsNamed(name)
	if err != nil {
		return model.Collection{}, err
	}
	if len(cs) != 1 {
		return model.Collection{}, diagErr(diag.CodeWSManyCollections, map[string]string{"count": strconv.Itoa(len(cs))})
	}
	return cs[0], nil
}

// ResolveCollection picks the collection a command acts on: by name or id,
// or the only mounted one when name is empty. Problems are *diag.ListError
// (FLW-WS-005 unknown, FLW-WS-004 several mounted and none named).
func (w *Workspace) ResolveCollection(name string) (model.Manifest, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, err := w.oneCollection(name)
	if err != nil {
		return model.Manifest{}, err
	}
	return c.Manifest, nil
}

func diagErr(c diag.Code, args map[string]string) error {
	return diag.List{diag.New(c, diag.Location{}, args)}.Err()
}

// SendRequest starts sending a request. The request's collection (and the
// collections it depends on) must validate without errors, and the
// environment must exist; otherwise a *diag.ListError is returned and
// nothing is sent. The send runs until its Events are drained and Wait
// returns.
func (w *Workspace) SendRequest(ctx context.Context, t RequestTarget, o SendOptions) (*engine.Send, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return nil, err
	}
	if err := w.sendable(c, o.Env); err != nil {
		return nil, err
	}
	if err := w.unlockFor(c, o.Env); err != nil {
		return nil, err
	}
	return engine.StartSend(ctx, w.engineDeps(), sendInput(c, r, o)), nil
}

// sendInput builds the engine input for sending r with o.
func sendInput(c model.Collection, r model.Request, o SendOptions) engine.SendInput {
	r, pinned := withEditApplied(r, o)
	return engine.SendInput{
		Collection: c, Request: r, Env: o.Env, PinnedVersion: pinned, PreferVersion: o.PreferVersion,
		With: o.With, Hooks: o.Hooks, Seed: o.Seed, Save: o.Save,
	}
}

// withEditApplied returns r with its dirty edit standing in for the
// version it started from, and the version to pin, when o asks for the
// edit and the edit belongs to the version being sent (a request whose
// base version is gone sends its saved versions). Otherwise r and
// o.Version are returned unchanged. The edit keeps the base's label, so
// history and records name the version the user was working on.
func withEditApplied(r model.Request, o SendOptions) (model.Request, string) {
	if !o.UseEdit || r.Edit == nil || (o.Version != "" && o.Version != r.EditBase) {
		return r, o.Version
	}
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == r.EditBase })
	if i < 0 {
		return r, o.Version
	}
	r.Versions = slices.Clone(r.Versions)
	r.Versions[i] = *r.Edit
	r.Versions[i].Label = r.EditBase
	return r, r.EditBase
}

// PreviewRequest renders version v of request t, as shown in an editor and
// not necessarily saved, the way a send would after its pre-hooks: nothing
// is sent and no hook runs (see engine.PreviewSend). v replaces the saved
// version with its label. Names that cannot be resolved stay as written.
// The vault is read, and may prompt, only when includeSecrets asks for the
// real values.
func (w *Workspace) PreviewRequest(t RequestTarget, v model.RequestVersion, o SendOptions, includeSecrets bool) (engine.Preview, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return engine.Preview{}, err
	}
	if includeSecrets {
		if err := w.unlockFor(c, o.Env); err != nil {
			return engine.Preview{}, err
		}
	}
	r.Versions = slices.Clone(r.Versions)
	if i := slices.IndexFunc(r.Versions, func(x model.RequestVersion) bool { return x.Label == v.Label }); i >= 0 {
		r.Versions[i] = v
	} else {
		r.Versions = append(r.Versions, v)
	}
	o.Version, o.UseEdit = v.Label, false
	return engine.PreviewSend(w.engineDeps(), sendInput(c, r, o), includeSecrets), nil
}

// GRPCMethods lists the methods of the schema the gRPC version v of request t
// would use: its .proto files, or what the server's reflection offers. v
// replaces the saved version of the same label (the screen's edit). The vault
// is unlocked first when the call's metadata reads a secret. A version that is
// not a gRPC call is FLW-GRPC-001.
func (w *Workspace) GRPCMethods(ctx context.Context, t RequestTarget, v model.RequestVersion, o SendOptions) ([]grpcx.Method, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return nil, err
	}
	if err := w.unlockFor(c, o.Env); err != nil {
		return nil, err
	}
	r.Versions = slices.Clone(r.Versions)
	if i := slices.IndexFunc(r.Versions, func(x model.RequestVersion) bool { return x.Label == v.Label }); i >= 0 {
		r.Versions[i] = v
	} else {
		r.Versions = append(r.Versions, v)
	}
	o.Version, o.UseEdit = v.Label, false
	call, diags := engine.GRPCCallOf(w.engineDeps(), sendInput(c, r, o), true)
	if diags.HasErrors() {
		return nil, &diag.ListError{List: diags}
	}
	return grpcx.Methods(ctx, call)
}

func (w *Workspace) requestFor(t RequestTarget) (model.Collection, model.Request, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(t.Collection)
	if !ok {
		return model.Collection{}, model.Request{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(t.Collection)})
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == t.Request })
	if i < 0 {
		return model.Collection{}, model.Request{}, diagErr(diag.CodeRequestNotFound, map[string]string{"ref": string(t.Request)})
	}
	return c, c.Requests[i], nil
}

// sendable refuses a send whose collection has validation errors (in its
// own files or its dependencies'), or whose environment does not exist.
func (w *Workspace) sendable(c model.Collection, env string) error {
	if env != "" && !slices.ContainsFunc(c.Environments, func(e model.Environment) bool { return e.Name == env }) {
		return diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": env, "collection": c.Manifest.Name})
	}
	roots := []string{c.Root}
	for _, dep := range c.Manifest.DependsOn {
		if d, ok := w.loadedCollection(dep.ID); ok {
			roots = append(roots, d.Root)
		}
	}
	// Contract warnings never block, so they are not computed here.
	var blocking diag.List
	for _, d := range w.validateStructure() {
		if d.Severity == diag.SeverityError && underAny(d.File, roots) {
			blocking = append(blocking, d)
		}
	}
	return blocking.Err()
}

// unlockFor opens the vault before the send starts when the environment
// declares a secret that no FLOWI_SECRET_* variable overrides, so a
// passphrase prompt never interrupts live output. (Secrets of other
// collections, read inside hook flows, still unlock on first use.)
func (w *Workspace) unlockFor(c model.Collection, env string) error {
	if env == "" {
		v, _ := w.Settings(Scope{Collection: c.Manifest.ID}).Get("defaults.environment")
		env, _ = v.Value.(string)
	}
	for _, e := range c.Environments {
		if e.Name != env {
			continue
		}
		for _, v := range e.Vars {
			if v.Secret && !overridden(w.opts.Environ, e.Name, v.Name) {
				return vaultErr(w.secrets().Unlock())
			}
		}
	}
	return nil
}

func (w *Workspace) loadedCollection(id model.ID) (model.Collection, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.collection(id)
}

func underAny(file string, roots []string) bool {
	for _, r := range roots {
		if file == r || strings.HasPrefix(file, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (w *Workspace) engineDeps() engine.Deps {
	return engine.Deps{
		Lookup: engineLookup{w}, Schemas: schemaChecker{w}, Vault: w.secrets(), Environ: w.opts.Environ,
		DataDir: w.DataDir, Clock: time.Now, NewID: func() string { return string(w.newID()) },
		Observe: w.opts.Observe, Traceparent: w.opts.Traceparent, UserAgent: w.opts.UserAgent,
	}
}

// DataDir is where the workspace keeps a collection's data: saved
// responses, downloads and cookies.
func (w *Workspace) DataDir(collection model.ID) string {
	return workspace.DataDir(w.opts.Paths, w.opts.Name, collection)
}

func (w *Workspace) newID() model.ID {
	w.idMu.Lock()
	defer w.idMu.Unlock()
	return w.ids.New()
}

// secrets returns the workspace vault, created on first use. Nothing is
// unlocked until a secret is read or written.
func (w *Workspace) secrets() *vault.Vault {
	w.vaultOnce.Do(func() {
		w.vault = vault.New(vault.Options{
			File:        filepath.Join(workspace.Dir(w.opts.Paths, w.opts.Name), "vault.enc"),
			WorkspaceID: w.Model().ID, Keys: w.opts.Keys, Environ: w.opts.Environ, Passphrase: w.opts.Passphrase,
		})
	})
	return w.vault
}

// PinExample saves a send's record as a pinned example next to its
// request (<request>.examples/<name>.json; name defaults to the status).
// Pinned examples are committed with the collection.
func (w *Workspace) PinExample(rec engine.Record, name string) (string, error) {
	if err := CheckExampleName(name); err != nil {
		return "", err
	}
	_, r, err := w.requestFor(RequestTarget{Collection: rec.CollectionID, Request: rec.RequestID})
	if err != nil {
		return "", err
	}
	return engine.PinExample(rec, r.File, name)
}

// CheckExampleName refuses pin names that are not plain file names
// (FLW-REQ-007), so callers can check before sending. "" is allowed and
// means "name it after the status".
func CheckExampleName(name string) error {
	if name == "" || engine.ValidExampleName(name) {
		return nil
	}
	return diagErr(diag.CodeRequestExampleName, map[string]string{"name": name})
}

// ClearCookies deletes the saved cookie jar of a collection (name or id),
// or of every mounted collection when collection is "".
func (w *Workspace) ClearCookies(collection string) error {
	w.mu.RLock()
	cs, err := w.collectionsNamed(collection)
	w.mu.RUnlock()
	if err != nil {
		return err
	}
	for _, c := range cs {
		jar := filepath.Join(w.DataDir(c.Manifest.ID), "cookies", "jar.json")
		if err := os.Remove(jar); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("clearing cookies of %s: %w", c.Manifest.Name, err)
		}
	}
	return nil
}

// engineLookup serves the workspace's collections to the engine.
type engineLookup struct{ w *Workspace }

func (l engineLookup) Collection(id model.ID) (model.Collection, bool) {
	return l.w.loadedCollection(id)
}

func (l engineLookup) Request(ref model.Ref, from model.ID) (model.Collection, model.Request, bool) {
	c, ok := l.target(ref, from)
	if !ok {
		return model.Collection{}, model.Request{}, false
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == ref.ID })
	if i < 0 {
		return model.Collection{}, model.Request{}, false
	}
	return c, c.Requests[i], true
}

func (l engineLookup) Flow(ref model.Ref, from model.ID) (model.Collection, model.Flow, bool) {
	c, ok := l.target(ref, from)
	if !ok {
		return model.Collection{}, model.Flow{}, false
	}
	i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.ID == ref.ID })
	if i < 0 {
		return model.Collection{}, model.Flow{}, false
	}
	return c, c.Flows[i], true
}

// target is the collection a reference points into: the referencing one,
// or a declared dependency named by ref.Collection (name or id).
func (l engineLookup) target(ref model.Ref, from model.ID) (model.Collection, bool) {
	src, ok := l.w.loadedCollection(from)
	if !ok || ref.Collection == "" {
		return src, ok
	}
	for _, dep := range src.Manifest.DependsOn {
		if dep.Name == ref.Collection || string(dep.ID) == ref.Collection {
			return l.w.loadedCollection(dep.ID)
		}
	}
	return model.Collection{}, false
}

func (l engineLookup) Settings(collection model.ID, folder string, version model.Settings) settings.Resolved {
	w := l.w
	w.mu.RLock()
	defer w.mu.RUnlock()
	layers := w.settingsLayers(Scope{Collection: collection, Folder: path.Clean(folder)})
	layers = append(layers, settings.LayerValues{Layer: settings.LayerRequest, Values: version})
	return settings.Resolve(w.schema, layers...)
}
