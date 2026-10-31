// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/blueprint"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

const blueprintSuffix = ".blueprint.yaml"

// BlueprintTarget identifies a blueprint of a mounted collection.
type BlueprintTarget struct {
	Collection model.ID
	Blueprint  model.ID
}

// FindBlueprint resolves a blueprint reference as users type it: an id, or
// a path ("onboarding", optionally with blueprints/ and .blueprint.yaml).
// collection narrows the search by name or id. Problems are
// *diag.ListError: FLW-BP-006 (no match), FLW-BP-007 (several collections
// match), FLW-WS-005 (unknown collection).
func (w *Workspace) FindBlueprint(ref, collection string) (BlueprintTarget, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	candidates, err := w.collectionsNamed(collection)
	if err != nil {
		return BlueprintTarget{}, err
	}
	path := strings.TrimSuffix(strings.TrimPrefix(ref, "blueprints/"), blueprintSuffix)
	var found []BlueprintTarget
	for _, c := range candidates {
		if i := slices.IndexFunc(c.Blueprints, func(b model.Blueprint) bool { return string(b.ID) == ref || b.Path == path }); i >= 0 {
			found = append(found, BlueprintTarget{Collection: c.Manifest.ID, Blueprint: c.Blueprints[i].ID})
		}
	}
	switch len(found) {
	case 0:
		return BlueprintTarget{}, diagErr(diag.CodeBlueprintNotFound, map[string]string{"ref": ref})
	case 1:
		return found[0], nil
	}
	return BlueprintTarget{}, diagErr(diag.CodeBlueprintAmbiguous, map[string]string{"ref": ref, "count": strconv.Itoa(len(found))})
}

// Blueprint returns the loaded blueprint.
func (w *Workspace) Blueprint(t BlueprintTarget) (model.Blueprint, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(t.Collection)
	if !ok {
		return model.Blueprint{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(t.Collection)})
	}
	i := slices.IndexFunc(c.Blueprints, func(b model.Blueprint) bool { return b.ID == t.Blueprint })
	if i < 0 {
		return model.Blueprint{}, diagErr(diag.CodeBlueprintNotFound, map[string]string{"ref": string(t.Blueprint)})
	}
	return c.Blueprints[i], nil
}

// DeleteBlueprint removes the blueprint's file. Flows made from it are not
// touched: they keep what they were made with. An unknown blueprint is
// FLW-BP-006.
func (w *Workspace) DeleteBlueprint(t BlueprintTarget) error {
	bp, err := w.Blueprint(t)
	if err != nil {
		return err
	}
	return w.removeFile(t.Collection, bp.File)
}

// CreateBlueprint saves flow as blueprints/<path>.blueprint.yaml in the
// flow's collection (see blueprint.FromFlow for what becomes a parameter).
// Path rules are as for NewRequest (FLW-WRITE-002).
func (w *Workspace) CreateBlueprint(flow FlowTarget, path, name string) (BlueprintTarget, error) {
	c, f, err := w.flowFor(flow)
	if err != nil {
		return BlueprintTarget{}, err
	}
	file, err := newEntityFile(c, "blueprints", path, blueprintSuffix)
	if err != nil {
		return BlueprintTarget{}, err
	}
	bp, err := blueprint.FromFlow(f, w.newID, name)
	if err != nil {
		return BlueprintTarget{}, fmt.Errorf("making a blueprint from %s: %w", f.Path, err)
	}
	bp.Path, bp.File = path, file
	data, err := w.store.EncodeBlueprint(bp)
	if err != nil {
		return BlueprintTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(c.Manifest.ID, file, data, SaveOptions{}); err != nil {
		return BlueprintTarget{}, err
	}
	return BlueprintTarget{Collection: c.Manifest.ID, Blueprint: bp.ID}, nil
}

// NewFlowFromBlueprint creates flows/<path>.arazzo.yaml in collection from the
// blueprint, with the given parameter values (defaults fill the rest).
// References are looked up in collection, which need not be the blueprint's
// own. The flow is an ordinary flow with no link to the blueprint. Problems
// with the values are FLW-BP-005; path rules are as for NewFlow.
func (w *Workspace) NewFlowFromBlueprint(t BlueprintTarget, collection model.ID, path string, values map[string]string) (FlowTarget, error) {
	bp, err := w.Blueprint(t)
	if err != nil {
		return FlowTarget{}, err
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return FlowTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	file, err := newEntityFile(c, "flows", path, flowSuffix)
	if err != nil {
		return FlowTarget{}, err
	}
	f, diags := blueprint.Instantiate(bp, blueprint.Input{
		Values: values, Resolve: blueprintResolver{c: c}, NewID: w.newID, Path: path, Name: filepath.Base(path),
	})
	if len(diags) > 0 {
		return FlowTarget{}, diags.Err()
	}
	f.File = file
	data, err := w.store.EncodeFlow(c, f)
	if err != nil {
		return FlowTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(collection, file, data, SaveOptions{}); err != nil {
		return FlowTarget{}, err
	}
	return FlowTarget{Collection: collection, Flow: f.ID}, nil
}

// flowSuffix is the file suffix of a flow.
const flowSuffix = ".arazzo.yaml"

// newEntityFile is the file for a new entity of the given kind at path,
// which must be a safe relative path naming a file that does not exist.
func newEntityFile(c model.Collection, dir, path, suffix string) (string, error) {
	if detail := requestPathProblem(path); detail != "" {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	file := filepath.Join(c.Root, dir, filepath.FromSlash(path)+suffix)
	if _, err := os.Stat(file); err == nil {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": "it already exists"})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("checking %s: %w", file, err)
	}
	return file, nil
}

// blueprintResolver looks requests and flows up in one collection by id or
// path, the way users name them.
type blueprintResolver struct{ c model.Collection }

func (r blueprintResolver) Request(value string) (model.Ref, error) {
	path := strings.TrimSuffix(strings.TrimPrefix(value, "requests/"), ".req.yaml")
	i := slices.IndexFunc(r.c.Requests, func(q model.Request) bool { return string(q.ID) == value || q.Path == path })
	if i < 0 {
		return model.Ref{}, fmt.Errorf("no request %q in collection %q", value, r.c.Manifest.Name)
	}
	return model.Ref{ID: r.c.Requests[i].ID, Path: r.c.Requests[i].Path}, nil
}

func (r blueprintResolver) Flow(value string) (model.Ref, error) {
	path := strings.TrimSuffix(strings.TrimPrefix(value, "flows/"), flowSuffix)
	i := slices.IndexFunc(r.c.Flows, func(f model.Flow) bool { return string(f.ID) == value || f.Path == path })
	if i < 0 {
		return model.Ref{}, fmt.Errorf("no flow %q in collection %q", value, r.c.Manifest.Name)
	}
	return model.Ref{ID: r.c.Flows[i].ID, Path: r.c.Flows[i].Path}, nil
}
