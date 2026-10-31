// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/internal/yamldoc"
)

// Inlining limits. Acyclic references can still fan out exponentially
// (every schema referring to every other), so expansion stops at a nesting
// depth and after a number of expansions per resolved value.
const (
	maxRefDepth   = 6
	maxExpansions = 500
	// maxRefFiles and maxRefFileSize bound what a document may make Flowi read
	// through references to other files.
	maxRefFiles    = 200
	maxRefFileSize = 8 << 20
)

// refDoc is a document references are resolved in: the main one, or a file it
// refers to. dir is where its own relative references start, relative to the
// root of the files that may be read.
type refDoc struct {
	name string // "" for the main document, else the file's path under the root
	dir  string
	root map[string]any
}

// resolver inlines $refs: local ones ("#/components/schemas/Pet") and, when
// files is set, references to other files beside the document
// ("./schemas/pet.yaml", "common.yaml#/Pet"). A reference already being
// expanded (a cycle), or one past the limits, becomes {} — "anything", which
// contract checks treat as unknown rather than broken. A reference that cannot
// be followed (a URL, a file outside the root, a file that is not there) is
// recorded with the reason and also becomes {}.
type resolver struct {
	main     refDoc
	files    fs.FS             // where references to files are read from; nil: none are followed
	loaded   map[string]refDoc // by path under the root
	raw      map[string][]byte // what each loaded file held
	external map[string]string // reference → why it was not followed
	order    []string          // external, in the order they were met
}

func newResolver(root map[string]any, files fs.FS) *resolver {
	return &resolver{main: refDoc{root: root}, files: files, loaded: map[string]refDoc{}, raw: map[string][]byte{}, external: map[string]string{}}
}

// resolve inlines v's references with a fresh expansion budget.
func (r *resolver) resolve(v any) any {
	budget := maxExpansions
	return r.inline(r.main, v, nil, &budget)
}

func (r *resolver) inline(doc refDoc, v any, stack []string, budget *int) any {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok {
			return r.follow(doc, ref, stack, budget)
		}
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = r.inline(doc, item, stack, budget)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = r.inline(doc, item, stack, budget)
		}
		return out
	}
	return v
}

func (r *resolver) follow(doc refDoc, ref string, stack []string, budget *int) any {
	target, pointer, ok := r.target(doc, ref)
	if !ok {
		return map[string]any{}
	}
	key := target.name + "#" + pointer
	if slices.Contains(stack, key) || len(stack) >= maxRefDepth || *budget <= 0 {
		return map[string]any{} // a cycle, or past the limits: cut here
	}
	value, ok := pointerIn(target.root, pointer)
	if !ok {
		return map[string]any{}
	}
	*budget--
	return r.inline(target, value, append(slices.Clone(stack), key), budget)
}

// target is the document a reference points into and the JSON pointer inside
// it ("" for the whole document). It records why a reference is not followed.
func (r *resolver) target(doc refDoc, ref string) (refDoc, string, bool) {
	file, pointer, _ := strings.Cut(ref, "#")
	if file == "" {
		return doc, pointer, true
	}
	loaded, err := r.load(doc, file)
	if err != nil {
		r.skip(ref, err.Error())
		return refDoc{}, "", false
	}
	return loaded, pointer, true
}

func (r *resolver) skip(ref, why string) {
	if _, seen := r.external[ref]; !seen {
		r.order = append(r.order, ref)
	}
	r.external[ref] = why
}

// load reads the file a reference names, relative to the file that refers to
// it. Only files under the root are read; a URL is never fetched.
func (r *resolver) load(from refDoc, file string) (refDoc, error) {
	switch {
	case r.files == nil:
		return refDoc{}, fmt.Errorf("only references within the document are followed")
	case strings.Contains(file, "://"):
		return refDoc{}, fmt.Errorf("a reference to a URL is not followed")
	}
	name := path.Join(from.dir, file)
	if !fs.ValidPath(name) {
		return refDoc{}, fmt.Errorf("the file is outside the folder of the document")
	}
	if doc, ok := r.loaded[name]; ok {
		return doc, nil
	}
	if len(r.loaded) >= maxRefFiles {
		return refDoc{}, fmt.Errorf("more than %d files are referred to", maxRefFiles)
	}
	data, err := readLimited(r.files, name)
	if err != nil {
		return refDoc{}, err
	}
	root, err := yamldoc.Decode(data)
	if err != nil { // the error is not shown: it can quote the file's text
		return refDoc{}, fmt.Errorf("the file is not a YAML or JSON object")
	}
	doc := refDoc{name: name, dir: path.Dir(name), root: root}
	if doc.dir == "." {
		doc.dir = ""
	}
	r.loaded[name] = doc
	r.raw[name] = data
	return doc, nil
}

func readLimited(files fs.FS, name string) ([]byte, error) {
	info, err := fs.Stat(files, name)
	if err != nil {
		return nil, fmt.Errorf("the file cannot be read: %w", unwrapPathError(err))
	}
	if info.Size() > maxRefFileSize {
		return nil, fmt.Errorf("the file is larger than %d MiB", maxRefFileSize>>20)
	}
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return nil, fmt.Errorf("the file cannot be read: %w", unwrapPathError(err))
	}
	return data, nil
}

// unwrapPathError drops the path from a file error, which the report names already.
func unwrapPathError(err error) error {
	if pe, ok := err.(*fs.PathError); ok { //nolint:errorlint // fs returns PathError itself, never wrapped
		return pe.Err
	}
	return err
}

// pointerIn looks up a JSON pointer ("/components/schemas/Pet"); "" and "/"
// are the whole document.
func pointerIn(root map[string]any, pointer string) (any, bool) {
	if pointer == "" || pointer == "/" {
		return root, true
	}
	var cur any = root
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// fileContents are the files read through references; nil when none were.
func (r *resolver) fileContents() map[string][]byte {
	if len(r.raw) == 0 {
		return nil
	}
	return r.raw
}
