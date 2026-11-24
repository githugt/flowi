// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/speclint"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/specoverlay"
	"github.com/githugt/flowi/lib/store"
)

// SpecLintFile is the file in a collection's descriptions/ folder that says how seriously each spec rule is taken:
//
//	rules:
//	  path-case: warning     # off, warning or error
//	  operation-tags: off
const SpecLintFile = "lint.yaml"

// SpecLintRules are the names of the rules a SpecLintFile can set.
func SpecLintRules() []string { return speclint.RuleNames() }

// SpecRule is one rule of the spec linter as a collection has it set.
type SpecRule struct {
	Name    string
	About   string
	Default string // off, warning or error
	Level   string // what the collection chose, or the default when it chose nothing
	Chosen  bool   // the collection's lint file sets it
}

// ErrNoSpecRule is returned for a rule or a level the linter does not have.
var ErrNoSpecRule = errors.New("that is not a spec rule or not a level (off, warning or error)")

// SpecLintRules lists the rules with their level in the collection's lint file.
func (w *Workspace) SpecLintRules(collection model.ID) ([]SpecRule, error) {
	root, err := w.collectionRoot(collection)
	if err != nil {
		return nil, err
	}
	cfg, _ := readSpecLintFile(filepath.Join(root, "descriptions", SpecLintFile))
	var out []SpecRule
	for _, r := range speclint.Info() {
		rule := SpecRule{Name: r.Name, About: r.About, Default: r.Default, Level: r.Default}
		if l, ok := cfg[r.Name]; ok {
			rule.Level, rule.Chosen = l, true
		}
		out = append(out, rule)
	}
	return out, nil
}

// SetSpecLintRule sets how seriously a rule is taken in the collection: "off", "warning" or "error", or "" for the
// rule's default. Only that rule's line changes: the comments in the file, and the order of its other lines, stay
// (speclint.Edit says when a file has to be written again).
func (w *Workspace) SetSpecLintRule(collection model.ID, rule, level string) error {
	if speclint.DefaultLevel(rule) == "" || !slices.Contains([]string{"", speclint.LevelOff, speclint.LevelWarning, speclint.LevelError}, level) {
		return ErrNoSpecRule
	}
	root, err := w.collectionRoot(collection)
	if err != nil {
		return err
	}
	file := filepath.Join(root, "descriptions", SpecLintFile)
	_, old := readSpecLintFile(file)
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return fmt.Errorf("making the descriptions folder: %w", err)
	}
	return store.Apply([]store.FileChange{{File: file, Old: old, New: speclint.Edit(old, rule, level)}})
}

// readSpecLintFile reads the lint file; a missing or unreadable one has no choices. The text is returned for the change record.
func readSpecLintFile(file string) (speclint.Config, []byte) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil
	}
	cfg, _ := speclint.ParseConfig(data, file) // a typo in the file is reported by Validate; the rest still counts
	return cfg, data
}

func (w *Workspace) collectionRoot(collection model.ID) (string, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return "", diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return c.Root, nil
}

// IsSpecDiagnostic reports whether d is a finding about an API description (FLW-SPEC-*), not about a request or flow.
func IsSpecDiagnostic(d diag.Diagnostic) bool { return len(d.Code) > 9 && d.Code[:9] == "FLW-SPEC-" }

// LintSpecs checks the API descriptions a collection kept at import, as they are with the overlays laid over them, so
// a description added in an overlay clears its finding. Each finding points at the kept file by the YAML path of the
// place in the effective description. Nothing is written.
func (w *Workspace) LintSpecs(collection model.ID) (diag.List, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.positioned(w.lintSpecsOf(c)), nil
}

// specDiags are the findings for every collection's kept descriptions. Callers hold mu.
func (w *Workspace) specDiags(cs []model.Collection) diag.List {
	var out diag.List
	for _, c := range cs {
		out = append(out, w.lintSpecsOf(c)...)
	}
	return out
}

// lintSpecsOf lints the kept descriptions of c: OpenAPI and Swagger, AsyncAPI and OpenRPC from descriptions/, and the
// .proto files in protos/. A description that cannot be read or does not apply its overlays is left out: the import
// reports that, and a lint is no reason to repeat it.
func (w *Workspace) lintSpecsOf(c model.Collection) diag.List {
	found := lintables(c.Root)
	if len(found) == 0 {
		return nil
	}
	cfgFile := filepath.Join(c.Root, "descriptions", SpecLintFile)
	cfgData, err := os.ReadFile(cfgFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		cfgData = nil // an unreadable config is as good as none; the rules keep their defaults
	}
	var out diag.List
	cfg := speclint.Config{}
	if cfgData != nil {
		var problems diag.List
		cfg, problems = speclint.ParseConfig(cfgData, cfgFile)
		out = append(out, problems...)
	}
	for _, l := range found {
		out = append(out, w.lintOne(l, cfg, cfgData)...)
	}
	return out
}

// lintable is one description the linter reads.
type lintable struct {
	name     string // as the messages name it
	file     string // absolute
	proto    bool   // a .proto file, not a document with overlays
	overlays []string
	edits    string // OpenAPI only: the overlay Flowi saves request edits to
	root     string // proto: the folder its imports are read from
}

var messageDescription = regexp.MustCompile(`^(.+)\.(asyncapi|openrpc)\.(yaml|yml|json)$`)

// lintables lists what a collection kept to be linted, in a stable order.
func lintables(root string) []lintable {
	var out []lintable
	specs, _ := keptSpecs(root)
	for _, k := range specs {
		out = append(out, lintable{name: k.Name, file: k.File, overlays: k.Overlays, edits: k.Edits})
	}
	dir := filepath.Join(root, "descriptions")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if m := messageDescription.FindStringSubmatch(e.Name()); m != nil && m[1] != "requests" {
				out = append(out, lintable{name: m[1], file: filepath.Join(dir, e.Name()), overlays: overlaysOf(dir, m[1], entries)})
			}
		}
	}
	protos := filepath.Join(root, "protos")
	_ = filepath.WalkDir(protos, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".proto") {
			rel, _ := filepath.Rel(protos, path)
			out = append(out, lintable{name: filepath.ToSlash(rel), file: path, proto: true, root: protos})
		}
		return nil
	})
	return out
}

// read returns the description and the overlays that apply to it, the edits overlay last when there is one.
func (l lintable) read() (data []byte, overlays [][]byte, err error) {
	if data, err = os.ReadFile(l.file); err != nil {
		return nil, nil, err
	}
	for _, f := range l.overlays {
		ov, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		overlays = append(overlays, ov)
	}
	if l.edits != "" {
		if ov, err := os.ReadFile(l.edits); err == nil {
			overlays = append(overlays, ov)
		}
	}
	return data, overlays, nil
}

// lintOne lints one description with its overlays. The answer is remembered while the files stay the same, because
// Validate runs after every change in the workspace and a description can be large.
func (w *Workspace) lintOne(l lintable, cfg speclint.Config, cfgData []byte) diag.List {
	data, overlays, err := l.read()
	if err != nil {
		return nil
	}
	key := lintKey(l.file, data, overlays, cfgData)
	if got, ok := w.lintCached(key); ok {
		return got
	}
	var found diag.List
	if l.proto {
		found = speclint.LintProto(l.name, data, protoReader(l.root), l.file, cfg)
	} else {
		applied, diags := specoverlay.Apply(data, overlays)
		if diags.HasErrors() {
			return nil
		}
		if found, err = speclint.Lint(applied, l.name, l.file, cfg); err != nil {
			return nil
		}
	}
	w.lintRemember(key, found)
	return found
}

// protoReader reads the files a .proto imports from the folder the collection keeps them in, and from nowhere else.
func protoReader(root string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if !fs.ValidPath(p) {
			return nil, fs.ErrNotExist
		}
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	}
}

type lintKeyT [sha256.Size]byte

// lintKey identifies the inputs of one lint: the description, its overlays in order, and the rule levels.
func lintKey(file string, spec []byte, overlays [][]byte, cfg []byte) lintKeyT {
	h := sha256.New()
	for _, part := range slices.Concat([][]byte{[]byte(file), spec}, overlays, [][]byte{cfg}) {
		h.Write(part)
		h.Write([]byte{0})
	}
	var k lintKeyT
	copy(k[:], h.Sum(nil))
	return k
}

func (w *Workspace) lintCached(k lintKeyT) (diag.List, bool) {
	w.lintMu.Lock()
	defer w.lintMu.Unlock()
	got, ok := w.lintMemo[k]
	return slices.Clone(got), ok
}

// lintRemember keeps the answer for one set of inputs. The memo is small and starts again when it grows, so a
// workspace that edits its descriptions all day does not keep every version.
func (w *Workspace) lintRemember(k lintKeyT, l diag.List) {
	w.lintMu.Lock()
	defer w.lintMu.Unlock()
	if w.lintMemo == nil || len(w.lintMemo) > 64 {
		w.lintMemo = map[lintKeyT]diag.List{}
	}
	w.lintMemo[k] = slices.Clone(l)
}

// DescriptionFiles lists the API descriptions a collection kept and the linter reads, as paths relative to the
// collection: the OpenAPI, Swagger, AsyncAPI and OpenRPC documents in descriptions/ and the .proto files in protos/.
func (w *Workspace) DescriptionFiles(collection model.ID) ([]string, error) {
	root, err := w.collectionRoot(collection)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range lintables(root) {
		rel, err := filepath.Rel(root, l.file)
		if err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out, nil
}
