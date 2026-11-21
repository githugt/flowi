// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"slices"

	"github.com/githugt/flowi/app/internal/vm"
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// RequestMatrixChoices are what the request's matrix dialog offers to pick from.
type RequestMatrixChoices struct {
	Envs     []string `json:"envs"`     // environments of the workspace
	Overlays []string `json:"overlays"` // flowi overlays of the request's collection
	Names    []string `json:"names"`    // the bare names the request uses ({{email}}), which a matrix can give values to
}

// matrixTarget is the request's target and the options every cell starts from, once the request is saved (a matrix
// sends the file on disk, like a flow's).
func (s *RequestService) matrixTarget(node string) (t flowi.RequestTarget, base flowi.SendOptions, names []string, err error) {
	found := false
	s.with(node, func(o *openRequest) {
		found = true
		if o.req.Dirty() || s.app.workspace.InScratch(node) {
			err = errors.New("save the request first: a matrix sends the file on disk")
			return
		}
		t = o.req.Target()
		base = flowi.SendOptions{Env: s.app.workspace.SelectedEnv(), Version: o.req.SelectedLabel(), Hooks: model.HookControl{Mode: "all"}}
		cur := o.req.Current()
		vars, secrets := s.app.workspace.VariableNames()
		known := knownWithOwn(vm.NewNameChecker(vars, secrets), keysOf(cur.Variables))
		for _, m := range missingVars(templatedTexts(cur), known) {
			if m.Name != "" && !m.Secret && !slices.Contains(names, m.Name) {
				names = append(names, m.Name)
			}
		}
		for _, k := range keysOf(cur.Variables) {
			if !slices.Contains(names, k) {
				names = append(names, k)
			}
		}
	})
	if !found && err == nil {
		err = errors.New("this request is not open")
	}
	return t, base, names, err
}

// MatrixChoices lists what the request's matrix dialog offers: environments, the collection's overlays and the names
// the request uses.
func (s *RequestService) MatrixChoices(node string) (RequestMatrixChoices, error) {
	t, _, names, err := s.matrixTarget(node)
	if err != nil {
		return RequestMatrixChoices{}, err
	}
	v := RequestMatrixChoices{Envs: []string{}, Overlays: []string{}, Names: nonNil(names)}
	for _, e := range s.app.backend.Environments() {
		v.Envs = append(v.Envs, e.Name)
	}
	if ovs, err := s.app.backend.Overlays(t.Collection); err == nil {
		for _, o := range ovs {
			v.Overlays = append(v.Overlays, o.Name)
		}
	}
	return v, nil
}

// RunMatrix sends the saved request once for every combination of the dimensions' values (env, overlay or
// with.<name>) and returns the grid. The responses are not saved. Progress arrives as the "matrix" event, and
// CancelMatrix stops it with the cells that ran so far returned (Cancelled set).
func (s *RequestService) RunMatrix(node string, dims []MatrixDimView) (MatrixView, error) {
	t, base, _, err := s.matrixTarget(node)
	if err != nil {
		return MatrixView{}, err
	}
	ctx, finish, err := s.app.matrices.begin(node)
	if err != nil {
		return MatrixView{}, err
	}
	defer finish()
	cells, err := vm.RunRequestMatrixProgress(ctx, s.app.backend, t, base, matrixDims(dims), s.app.matrices.progress(s.app.emit, node))
	return matrixResult(dims, cells, err)
}

// CancelMatrix stops the matrix running for the node, if any; the grid so far comes back from RunMatrix.
func (s *RequestService) CancelMatrix(node string) { s.app.matrices.cancel(node) }
