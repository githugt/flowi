// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/bundle"
	"github.com/githugt/flowi-lib/internal/tapeplayer"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
	"github.com/githugt/flowi-lib/vault"
)

// BundleExtension is what a run bundle file is named.
const BundleExtension = bundle.Extension

// IsBundleRef reports whether ref is an existing run bundle file.
func IsBundleRef(ref string) bool {
	if !strings.HasSuffix(strings.ToLower(ref), bundle.Extension) {
		return false
	}
	info, err := os.Stat(ref)
	return err == nil && !info.IsDir()
}

// WriteBundle writes one finished flow run as a bundle: the flow and the requests it used, the seed and
// inputs, the report and the masked record of every call. Nothing a person typed as a secret is in it; the
// environment is written with its secrets blank. A bundle made from a run of a flow that calls another
// collection is not complete: those requests are not in it.
func (w *Workspace) WriteBundle(out io.Writer, t FlowTarget, o RunOptions, res engine.RunResult) error {
	c, f, err := w.flowFor(t)
	if err != nil {
		return err
	}
	files, err := collectionFiles(c.Root)
	if err != nil {
		return err
	}
	envFile, secrets, err := w.bundledEnvironment(c, res.Report.Env)
	if err != nil {
		return err
	}
	var sequence bytes.Buffer
	if err := report.WriteMermaid(&sequence, res.Report); err != nil {
		return fmt.Errorf("drawing the sequence diagram: %w", err)
	}
	return bundle.Write(out, bundle.Bundle{
		Manifest: bundle.Manifest{
			Collection: c.Manifest.Name, FlowID: f.ID, FlowPath: f.Path, FlowName: f.Name, Env: res.Report.Env, Seed: res.Seed,
			Inputs: o.Inputs, Status: res.Report.Status, At: res.Report.Started, Secrets: secrets,
		},
		Report: res.Report, Records: res.Captured, Files: files, Environment: envFile, Sequence: sequence.Bytes(),
	})
}

// collectionFiles reads the files a run needs to be shown again: the manifest, flows, requests and folder
// settings. Examples, schemas, saved responses and generated descriptions are left out.
func collectionFiles(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, file)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && !keepDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !keepFile(rel) || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		data, readErr := os.ReadFile(file) //nolint:gosec // a file of the collection being bundled
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", file, readErr)
		}
		files[rel] = data
		return nil
	})
	return files, err
}

func keepDir(rel string) bool {
	top, _, _ := strings.Cut(rel, "/")
	return (top == "flows" || top == "requests") && !strings.HasSuffix(rel, ".examples") && !strings.HasSuffix(rel, ".schemas")
}

func keepFile(rel string) bool {
	switch {
	case rel == "flowi.yaml":
		return true
	case strings.HasSuffix(rel, "/_folder.yaml"):
		return true
	case strings.HasPrefix(rel, "flows/"):
		return strings.HasSuffix(rel, ".yaml") || strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".json")
	case strings.HasPrefix(rel, "requests/"):
		return strings.HasSuffix(rel, ".req.yaml")
	}
	return false
}

// bundledEnvironment is the environment the run used, flattened (what it extends is folded in) and with
// secrets blank, and the names of its secrets.
func (w *Workspace) bundledEnvironment(c model.Collection, name string) ([]byte, []string, error) {
	if name == "" {
		return nil, nil, nil
	}
	chain, err := model.EnvironmentChain(c.Environments, name)
	if err != nil {
		return nil, nil, err
	}
	own := chain[len(chain)-1]
	flat := model.Environment{Meta: model.Meta{ID: own.ID, Name: own.Name}, Colour: own.Colour}
	index := map[string]int{}
	for _, e := range chain {
		for _, v := range e.Vars {
			if v.Secret && e.Name != own.Name {
				continue // not inherited
			}
			if i, ok := index[v.Name]; ok {
				flat.Vars[i] = v
				continue
			}
			index[v.Name] = len(flat.Vars)
			flat.Vars = append(flat.Vars, v)
		}
	}
	data, err := w.store.EncodeEnvironment(flat)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding the environment: %w", err)
	}
	var secrets []string
	for _, v := range flat.Vars {
		if v.Secret {
			secrets = append(secrets, v.Name)
		}
	}
	return data, secrets, nil
}

// BundleRun is a run bundle opened to play back. Its flow is in a temporary collection that lasts until
// Close; nothing is mounted and the workspace file is not touched.
type BundleRun struct {
	Target   FlowTarget
	Options  RunOptions // the recorded environment, seed and inputs, answered from the recording
	Manifest BundleManifest
	// Original is the report of the run that was bundled, to compare a replay with.
	Original engine.RunResult

	w   *Workspace
	id  model.ID
	dir string
}

// BundleManifest says what the bundled run was.
type BundleManifest = bundle.Manifest

// Close removes the temporary collection. It is safe to call twice.
func (b *BundleRun) Close() { b.w.dropTemporary(b.id, b.dir) }

// OpenBundle reads a bundle and prepares to play its run back: the flow and requests go into a temporary
// collection, the environment is the recorded one with every secret set to the mask, and every HTTP call is
// answered from the recording. Running it needs no network, no vault and none of the original collection.
func (w *Workspace) OpenBundle(data []byte) (*BundleRun, error) {
	b, err := bundle.Read(data)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "flowi-bundle-")
	if err != nil {
		return nil, fmt.Errorf("making a temporary collection: %w", err)
	}
	w.freshIDs(&b)
	if err := writeFiles(dir, b.Files); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	id, err := w.addTemporary(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	br := &BundleRun{Manifest: b.Manifest, w: w, id: id, dir: dir, Original: engine.RunResult{Report: b.Report, Captured: b.Records, Seed: b.Manifest.Seed}}
	if err := w.prepareReplay(br, b); err != nil {
		br.Close()
		return nil, err
	}
	return br, nil
}

// ulidToken finds an entity id wherever a file writes one.
var ulidToken = regexp.MustCompile(`\b[0-9A-HJKMNP-TV-Z]{26}\b`)

// freshIDs gives every entity of the bundle a new id, the same one everywhere it is written. The bundle may be
// played on the machine that made it, where the original collection is mounted with the same ids; validation
// refuses two collections that share one, and the copy must never stand in for the original.
func (w *Workspace) freshIDs(b *bundle.Bundle) {
	changed := map[string]string{}
	swap := func(data []byte) []byte {
		return ulidToken.ReplaceAllFunc(data, func(old []byte) []byte {
			if _, ok := changed[string(old)]; !ok {
				changed[string(old)] = string(w.newID())
			}
			return []byte(changed[string(old)])
		})
	}
	for name, data := range b.Files {
		b.Files[name] = swap(data)
	}
	if b.Environment != nil {
		b.Environment = swap(b.Environment)
	}
}

// writeFiles puts the bundle's collection files under dir. Names were checked by bundle.Read.
func writeFiles(dir string, files map[string][]byte) error {
	for _, rel := range slices.Sorted(mapKeysOf(files)) {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // a temporary collection
			return fmt.Errorf("creating %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, files[rel], 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}
	}
	return nil
}

func mapKeysOf[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// prepareReplay finds the flow, puts the recorded environment in place and fills the run options.
func (w *Workspace) prepareReplay(br *BundleRun, b bundle.Bundle) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == br.id })
	if i < 0 {
		return fmt.Errorf("the temporary collection is gone")
	}
	c := &w.collections[i]
	fi := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.Path == b.Manifest.FlowPath })
	if fi < 0 {
		return fmt.Errorf("the bundle has no flow %q", b.Manifest.FlowPath)
	}
	br.Target = FlowTarget{Collection: c.Manifest.ID, Flow: c.Flows[fi].ID}
	br.Options = RunOptions{
		Env: b.Manifest.Env, Seed: b.Manifest.Seed, Inputs: b.Manifest.Inputs,
		Transport: tapeplayer.New(b.Records), Secrets: maskedSecrets(b.Manifest.Secrets),
	}
	if b.Environment == nil {
		return nil
	}
	env, diags := w.store.DecodeEnvironment("environment.yaml", b.Environment)
	if diags.HasErrors() {
		return diags.Err()
	}
	// The recorded environment replaces any workspace environment of the same name: its values built the
	// recorded URLs.
	c.Environments = slices.DeleteFunc(c.Environments, func(e model.Environment) bool { return e.Name == env.Name })
	c.Environments = append(c.Environments, env)
	return nil
}

func maskedSecrets(names []string) map[string]string {
	out := make(map[string]string, len(names))
	for _, n := range names {
		out[n] = vault.Mask
	}
	return out
}

// addTemporary loads the collection at dir and adds it to the workspace for this process only, under a new
// id so that it never stands in for a mounted collection that has the same one.
func (w *Workspace) addTemporary(dir string) (model.ID, error) {
	c, diags, err := w.store.Load(dir)
	if err != nil {
		return "", fmt.Errorf("loading the temporary collection: %w", err)
	}
	if diags.HasErrors() {
		return "", diags.Err()
	}
	id := c.Manifest.ID
	w.mu.Lock()
	w.collections = append(w.collections, w.withGlobalEnvironments(c))
	w.mu.Unlock()
	return id, nil
}

func (w *Workspace) dropTemporary(id model.ID, dir string) {
	w.mu.Lock()
	w.collections = slices.DeleteFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == id })
	w.mu.Unlock()
	_ = os.RemoveAll(dir) // best effort: a temporary directory the system will clear anyway
}

// KeepBundle opens a bundle like OpenBundle and leaves it open for browsing: its collection stays in the
// workspace until the process ends, and every run started in it, whatever environment or seed the caller
// asks for, is a replay of the recording. That is how a window shows a teammate's run on the canvas: the
// flow opens as any other, and Run plays the recording back.
func (w *Workspace) KeepBundle(data []byte) (*BundleRun, error) {
	br, err := w.OpenBundle(data)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.replays == nil {
		w.replays = map[model.ID]replayOf{}
	}
	w.replays[br.Target.Collection] = replayOf{options: br.Options, records: br.Original.Captured}
	if i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == br.Target.Collection }); i >= 0 {
		// A copy that carries the original's name would be mistaken for it in the tree.
		w.collections[i].Manifest.Name = fmt.Sprintf("%s (replay of %s)", br.Manifest.Collection, br.Manifest.At.Local().Format("2006-01-02 15:04"))
	}
	return br, nil
}

// replayOf is what makes a run a replay: the recorded options, and the records to answer from. Each run gets
// its own player, since a player remembers which records it has given.
type replayOf struct {
	options RunOptions
	records []engine.Record
}

// IsReplayCollection says whether the collection was opened from a run bundle.
func (w *Workspace) IsReplayCollection(id model.ID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.replays[id]
	return ok
}

// asReplay makes the options those of a replay when the collection came from a bundle: the recorded
// environment, transport and secrets always, and the recorded seed and inputs unless the caller gave their own.
func (w *Workspace) asReplay(collection model.ID, o RunOptions) RunOptions {
	w.mu.RLock()
	base, ok := w.replays[collection]
	w.mu.RUnlock()
	if !ok {
		return o
	}
	o.Env, o.Transport, o.Secrets = base.options.Env, tapeplayer.New(base.records), base.options.Secrets
	if o.Seed == 0 {
		o.Seed = base.options.Seed
	}
	if len(o.Inputs) == 0 {
		o.Inputs = base.options.Inputs
	}
	return o
}
