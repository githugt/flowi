// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/authx"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/internal/ids"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/vault"
	"github.com/githugt/flowi-lib/workspace"
)

// Change is a watched file change (see store.Change).
type Change = store.Change

// FileChange is a pending formatting change (see store.FileChange).
type FileChange = store.FileChange

// Change operations, re-exported from store.
const (
	ChangeCreated  = store.ChangeCreated
	ChangeModified = store.ChangeModified
	ChangeRemoved  = store.ChangeRemoved
	ChangeRescan   = store.ChangeRescan
)

// BlocksRewrite reports whether a load diagnostic means Flowi must not
// rewrite that file: errors (content was not understood) and unknown
// settings (which a rewrite would drop).
func BlocksRewrite(d diag.Diagnostic) bool {
	return d.Severity == diag.SeverityError || d.Code == diag.CodeSettingUnknown
}

func blocking(l diag.List) diag.List {
	return slices.DeleteFunc(slices.Clone(l), func(d diag.Diagnostic) bool { return !BlocksRewrite(d) })
}

// WorkspaceOptions configure OpenWorkspace.
type WorkspaceOptions struct {
	Paths   settings.Paths
	Name    string               // "" means workspace.DefaultName
	Environ []string             // from settings.ProcessEnviron()
	Flags   settings.LayerValues // LayerFlag values from the command line
	Home    string               // used to expand "~" in mount paths
	// Keys is the OS keychain holding the vault key (vault.OSKeyStore());
	// nil when there is none, and the vault then needs a passphrase.
	Keys vault.KeyStore
	// Passphrase asks the user for the vault passphrase; nil when no
	// prompt is possible (FLOWI_VAULT_KEY is the alternative).
	Passphrase func() (string, error)
	// Observe receives every event of every run and send, as the engine
	// emits it; it must return quickly (see engine.Deps.Observe). The otel
	// package's Exporter plugs in here.
	Observe func(engine.Event)
	// UserAgent is the client's User-Agent ("Flowi-UI/1.2"); "" sends "Flowi".
	UserAgent string
	// Traceparent names the trace context sent with outgoing requests (see
	// engine.Deps.Traceparent); nil sends none.
	Traceparent func(runID, path string) string
}

// Workspace is an open workspace with its mounted collections loaded.
// It is safe for concurrent use.
type Workspace struct {
	opts   WorkspaceOptions
	schema []settings.Key
	codec  *yamlcodec.Codec
	store  *store.Store

	idMu sync.Mutex // guards ids, which is not safe for concurrent use
	ids  *ids.Source

	vaultOnce sync.Once
	vault     *vault.Vault

	// authTokens keeps OAuth 2.0 tokens between sends for as long as the
	// workspace is open.
	authTokens *authx.TokenCache

	// draftsMu serialises draft operations (Scratch's check-then-create,
	// picking a free untitled name, save, discard). It is taken before mu,
	// never while holding it.
	draftsMu sync.Mutex

	mu          sync.RWMutex // guards every field below
	model       model.Workspace
	collections []model.Collection
	missing     []model.ID
	globalEnvs  []model.Environment // the workspace's environments, shared by every collection
	// replays are the collections opened from run bundles, with the options that make every run in them a
	// replay of the recording (see KeepBundle).
	replays map[model.ID]replayOf
	// documentEnvs is, for each collection opened from an Arazzo document, the environment made from its server.
	documentEnvs map[model.ID]string
	loadDiags    diag.List
	user         settings.LayerValues
	// Problems found when reading workspace.yaml and settings.yaml. While
	// either is non-empty that file is never rewritten: it may not be in
	// git, so overwriting it could lose the user's data for good.
	modelProblems diag.List
	userProblems  diag.List
}

// OpenWorkspace opens (creating on first use) a workspace and loads every
// mounted collection. Problems with files are returned as diagnostics;
// the error is reserved for system failures.
func OpenWorkspace(ctx context.Context, o WorkspaceOptions) (*Workspace, diag.List, error) {
	if o.Name == "" {
		o.Name = workspace.DefaultName
	}
	schema := settings.Schema()
	codec := yamlcodec.New(schema)
	w := &Workspace{
		opts: o, schema: schema, codec: codec, store: store.New(codec),
		ids: ids.NewSource(time.Now, rand.Reader), authTokens: authx.NewTokenCache(),
	}
	ws, diags, err := workspace.LoadOrCreate(o.Paths, o.Name, codec, w.ids)
	if err != nil {
		return nil, nil, fmt.Errorf("opening workspace %q: %w", o.Name, err)
	}
	w.model = ws
	w.modelProblems = w.positioned(blocking(diags))
	userDiags, err := w.loadUserSettings()
	if err != nil {
		return nil, nil, err
	}
	diags = append(diags, userDiags...)
	_, envDiags := settings.FromEnviron(schema, o.Environ)
	diags = append(diags, envDiags...)
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("opening workspace %q: %w", o.Name, err)
	}
	mountDiags, err := w.loadCollections()
	if err != nil {
		return nil, nil, err
	}
	w.loadDiags = slices.Concat(diags, mountDiags)
	return w, w.positioned(w.loadDiags), nil
}

func (w *Workspace) userSettingsFile() string {
	return UserSettingsFile(w.opts.Paths)
}

func (w *Workspace) loadUserSettings() (diag.List, error) {
	lv, diags, err := readUserLayer(w.userSettingsFile(), w.codec, w.schema)
	w.user = lv
	w.userProblems = blocking(diags)
	return diags, err
}

// loadCollections loads every mounted collection that exists. A missing or
// broken mount is reported and the others still load.
func (w *Workspace) loadCollections() (diag.List, error) {
	diags := workspace.CheckMounts(w.model, w.opts.Home, workspace.DirExists)
	w.collections, w.missing = nil, nil
	envDiags, err := w.loadGlobalEnvironments()
	if err != nil {
		return nil, err
	}
	diags = append(diags, envDiags...)
	for _, m := range w.model.Collections {
		dir := workspace.ExpandPath(m.Path, w.opts.Home)
		if !workspace.DirExists(dir) {
			w.missing = append(w.missing, m.ID)
			continue
		}
		c, cdiags, err := w.store.Load(dir)
		if errors.Is(err, store.ErrNotCollection) {
			w.missing = append(w.missing, m.ID)
			diags = append(diags, notCollection(dir))
			continue
		}
		if err != nil {
			// One unreadable collection must not take the workspace down.
			w.missing = append(w.missing, m.ID)
			diags = append(diags, diag.New(diag.CodeFileUnreadable, diag.Location{File: dir},
				map[string]string{"name": dir, "detail": err.Error()}))
			continue
		}
		w.collections = append(w.collections, w.withGlobalEnvironments(c))
		diags = append(diags, cdiags...)
	}
	return diags, nil
}

// loadGlobalEnvironments reads the workspace's environments. They are
// shared by every collection.
func (w *Workspace) loadGlobalEnvironments() (diag.List, error) {
	dir := workspace.EnvironmentsDir(w.opts.Paths, w.opts.Name)
	envs, diags, err := w.store.LoadEnvironments(dir, vault.WorkspaceScope)
	if err != nil {
		return nil, err
	}
	w.globalEnvs = envs
	return diags, nil
}

// withGlobalEnvironments gives c the workspace's environments. Environments
// are global: any a collection holds are not used.
func (w *Workspace) withGlobalEnvironments(c model.Collection) model.Collection {
	c.Environments = slices.Clone(w.globalEnvs)
	return c
}

func notCollection(dir string) diag.Diagnostic {
	return diag.New(diag.CodeWSNotCollection, diag.Location{File: dir}, map[string]string{"path": dir})
}

// Model returns the workspace definition.
func (w *Workspace) Model() model.Workspace {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.model
}

// Collections returns the loaded collections in mount order.
func (w *Workspace) Collections() []model.Collection {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return slices.Clone(w.collections)
}

// Missing returns the ids of mounted collections that could not be found.
func (w *Workspace) Missing() []model.ID {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return slices.Clone(w.missing)
}

// Mount adds the collection in dir to the workspace and loads it. Paths
// are stored absolute, except that a leading "~" is kept.
func (w *Workspace) Mount(dir string) (model.Manifest, error) {
	m, err := w.mount(dir)
	if err != nil {
		return m, err
	}
	// Best effort: the generated description is derived, and a collection that
	// cannot be written to still works.
	_ = w.RegenerateDescription(m.ID)
	return m, nil
}

func (w *Workspace) mount(dir string) (model.Manifest, error) {
	c, diags, err := w.store.Load(workspace.ExpandPath(dir, w.opts.Home))
	if errors.Is(err, store.ErrNotCollection) {
		return model.Manifest{}, diag.List{notCollection(dir)}.Err()
	}
	if err != nil {
		return model.Manifest{}, fmt.Errorf("loading %s: %w", dir, err)
	}
	stored, err := w.storedPath(dir)
	if err != nil {
		return model.Manifest{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	next, err := workspace.Mount(w.model, c.Manifest.ID, stored, w.resolvePath)
	if errors.Is(err, workspace.ErrAlreadyMounted) {
		d := diag.New(diag.CodeWSAlreadyMounted, diag.Location{File: dir}, map[string]string{"path": dir})
		return model.Manifest{}, diag.List{d}.Err()
	}
	if err != nil {
		return model.Manifest{}, err
	}
	if err := w.saveModel(next); err != nil {
		return model.Manifest{}, err
	}
	w.collections = append(w.collections, w.withGlobalEnvironments(c))
	w.loadDiags = slices.Concat(w.loadDiags, diags)
	return c.Manifest, nil
}

func (w *Workspace) storedPath(dir string) (string, error) {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		return dir, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", dir, err)
	}
	return abs, nil
}

// resolvePath gives mount paths a comparable form for duplicate detection.
func (w *Workspace) resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(workspace.ExpandPath(p, w.opts.Home))
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil // a missing mount still compares by its absolute path
}

func (w *Workspace) saveModel(next model.Workspace) error {
	if len(w.modelProblems) > 0 {
		return w.modelProblems.Err()
	}
	if err := workspace.Save(w.opts.Paths, w.codec, next); err != nil {
		return fmt.Errorf("saving workspace: %w", err)
	}
	w.model = next
	return nil
}

// Unmount removes a collection from the workspace. Its files are untouched.
func (w *Workspace) Unmount(id model.ID) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	next, err := workspace.Unmount(w.model, id)
	if err != nil {
		return err
	}
	if err := w.saveModel(next); err != nil {
		return err
	}
	w.collections = slices.DeleteFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == id })
	w.missing = slices.DeleteFunc(w.missing, func(m model.ID) bool { return m == id })
	return nil
}

// Relocate points a mounted collection at a new directory, which must hold
// the same collection.
func (w *Workspace) Relocate(id model.ID, dir string) error {
	c, _, err := w.store.Load(workspace.ExpandPath(dir, w.opts.Home))
	if errors.Is(err, store.ErrNotCollection) {
		return diag.List{notCollection(dir)}.Err()
	}
	if err != nil {
		return fmt.Errorf("loading %s: %w", dir, err)
	}
	if c.Manifest.ID != id {
		return fmt.Errorf("%s holds collection %s, not %s", dir, c.Manifest.ID, id)
	}
	stored, err := w.storedPath(dir)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	next, err := workspace.Relocate(w.model, id, stored)
	if err != nil {
		return err
	}
	if err := w.saveModel(next); err != nil {
		return err
	}
	_, err = w.loadCollections()
	return err
}

// Watch reports changes to files in every loaded collection. See
// store.Watch for goroutine ownership; the final Change carries Err when
// the watcher fails.
func (w *Workspace) Watch(ctx context.Context) (<-chan Change, error) {
	w.mu.RLock()
	roots := make([]string, 0, len(w.collections))
	for _, c := range w.collections {
		roots = append(roots, c.Root)
	}
	w.mu.RUnlock()
	return store.Watch(ctx, roots)
}

// Reload applies one watched change to the in-memory collection.
func (w *Workspace) Reload(ch Change) (diag.List, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Root == ch.Root })
	if i < 0 {
		return nil, nil
	}
	if ch.Op == ChangeRescan {
		c, diags, err := w.store.Load(ch.Root)
		if err != nil {
			return nil, err
		}
		w.collections[i] = w.withGlobalEnvironments(c)
		w.replaceLoadDiags(func(file string) bool { return within(ch.Root, file) }, diags)
		return w.positioned(diags), nil
	}
	next, diags, err := w.store.ReloadFile(w.collections[i], ch.File)
	if err != nil {
		return nil, err
	}
	// A reloaded file may be a (no longer used) environment of the collection.
	next.Environments = slices.Clone(w.globalEnvs)
	w.collections[i] = next
	w.replaceLoadDiags(func(file string) bool { return file == ch.File }, diags)
	return w.positioned(diags), nil
}

// replaceLoadDiags swaps the load problems of the reloaded files (those
// stale matches) for fresh ones, so Validate reports the files as they are
// now. Callers hold mu for writing.
func (w *Workspace) replaceLoadDiags(stale func(file string) bool, fresh diag.List) {
	kept := slices.DeleteFunc(slices.Clone(w.loadDiags), func(d diag.Diagnostic) bool { return stale(d.File) })
	w.loadDiags = slices.Concat(kept, fresh)
}

// ErrWorkspaceName is returned by Rename for a name that is empty once
// trimmed.
var ErrWorkspaceName = errors.New("a workspace needs a name")

// Rename changes the workspace's display name in workspace.yaml. The folder
// keeps its name: drafts, sessions and the vault are found by it.
func (w *Workspace) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrWorkspaceName
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	next := w.model
	next.Name = name
	return w.saveModel(next)
}

// Folder is the folder holding workspace.yaml.
func (w *Workspace) Folder() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return filepath.Dir(w.model.File)
}
