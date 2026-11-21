// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi/lib/arazzo"
	"github.com/githugt/flowi/lib/model"
)

// LineMerger merges three versions of a text line by line, the way `git merge-file` does. merged is the result
// (with conflict markers when it is not clean).
type LineMerger func(base, ours, theirs []byte) (merged []byte, clean bool, err error)

// MergedFlow is the outcome of MergeFlowFile.
type MergedFlow struct {
	// Data is the merged flow file, with the derived Arazzo steps and the digest written afresh. Empty unless Clean.
	Data []byte
	// Clean is true when the two sides' edits to the flow itself did not touch the same lines.
	Clean bool
	// Reason says why Clean is false: "conflict" when both sides changed the same lines of the flow, or what made the
	// merge impossible (a file that is not a Flowi flow, a collection that is not mounted).
	Reason string
}

// MergeFlowFile merges three versions of a flow file (git's base, ours and theirs) for the merge driver.
//
// A flow file holds the flow's own data (x-flowi-flow) and, derived from it, the standard Arazzo steps and a digest
// of them. Both change on every edit, so two branches that each changed the flow always conflicted on lines no
// person wrote. This merges only the flow's own data, line by line, and writes the derived parts again from the
// result: the two sides conflict only where they changed the same lines of what a person edits.
//
// file is the flow file's path (inside a mounted collection). A file that is not a Flowi-written flow, or a body the
// merge cannot read back as a flow, is not clean with a Reason: the caller merges the whole files as text.
func (w *Workspace) MergeFlowFile(file string, base, ours, theirs []byte, lines LineMerger) (MergedFlow, error) {
	c, flowPath, err := w.flowFileOwner(file)
	if err != nil {
		return MergedFlow{Reason: err.Error()}, nil
	}
	var bodies [3][]byte
	for i, data := range [][]byte{base, ours, theirs} {
		body, ok, err := arazzo.OwnBody(data)
		if err != nil || !ok {
			return MergedFlow{Reason: "not a flow file Flowi wrote"}, nil
		}
		bodies[i] = body
	}
	merged, clean, err := lines(bodies[0], bodies[1], bodies[2])
	if err != nil {
		return MergedFlow{}, fmt.Errorf("merging %s: %w", file, err)
	}
	if !clean {
		return MergedFlow{Reason: "conflict"}, nil
	}
	f, diags := w.store.DecodeFlowBody(file, flowPath, merged)
	if diags.HasErrors() {
		return MergedFlow{Reason: "the merged flow is not valid: " + diags.Err().Error()}, nil
	}
	f.File = file
	data, err := w.store.EncodeFlow(c, f)
	if err != nil {
		return MergedFlow{}, fmt.Errorf("writing %s: %w", file, err)
	}
	return MergedFlow{Data: data, Clean: true}, nil
}

// flowFileOwner finds the mounted collection a flow file is in, and the flow's path in it.
func (w *Workspace) flowFileOwner(file string) (c model.Collection, flowPath string, err error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return c, "", err
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(dir, filepath.Base(abs))
	}
	for _, coll := range w.Collections() {
		rel, err := filepath.Rel(coll.Root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		rel = filepath.ToSlash(rel)
		path, isFlow := strings.CutPrefix(rel, "flows/")
		path, isFlow2 := strings.CutSuffix(path, ".arazzo.yaml")
		if !isFlow || !isFlow2 {
			return c, "", errors.New("not a flow file of the collection " + coll.Manifest.Name)
		}
		return coll, path, nil
	}
	return c, "", errors.New("the file is in no mounted collection")
}
