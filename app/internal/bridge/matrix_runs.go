// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"errors"
	"sync"

	"github.com/githugt/flowi/app/internal/vm"
)

// MatrixProgressView is the "matrix" event: how far the matrix running for a node has got.
type MatrixProgressView struct {
	Node  string `json:"node"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// matrixRuns keeps a way to stop each matrix in progress, by node. A matrix waits for every cell, so it runs off the
// actor on the call that started it, and Cancel arrives on another call: the mutex is what they share.
type matrixRuns struct {
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// begin registers a matrix for the node and returns its context and the func that ends it. One matrix runs per node.
func (m *matrixRuns) begin(node string) (context.Context, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, busy := m.running[node]; busy {
		return nil, nil, errors.New("a matrix is already running here: cancel it or wait for it to end")
	}
	if m.running == nil {
		m.running = map[string]context.CancelFunc{}
	}
	ctx, stop := context.WithCancel(context.Background())
	m.running[node] = stop
	return ctx, func() {
		m.mu.Lock()
		delete(m.running, node)
		m.mu.Unlock()
		stop()
	}, nil
}

// cancel stops the node's matrix; nothing happens when none is running.
func (m *matrixRuns) cancel(node string) {
	m.mu.Lock()
	stop := m.running[node]
	m.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// progress is the callback that publishes the "matrix" event for the node.
func (m *matrixRuns) progress(emit Emit, node string) vm.MatrixProgress {
	return func(done, total int) { emit("matrix", MatrixProgressView{Node: node, Done: done, Total: total}) }
}

func matrixDims(dims []MatrixDimView) []vm.MatrixDim {
	out := make([]vm.MatrixDim, len(dims))
	for i, d := range dims {
		out[i] = vm.MatrixDim{Name: d.Name, Values: d.Values}
	}
	return out
}

// matrixResult is the grid of a run's cells. A cancelled run is not an error: it is the grid so far, marked.
func matrixResult(dims []MatrixDimView, cells []vm.MatrixCell, err error) (MatrixView, error) {
	cancelled := errors.Is(err, context.Canceled)
	if err != nil && !cancelled {
		return MatrixView{}, err
	}
	v := MatrixView{Dims: dims, Cells: []MatrixCellView{}, Total: 1, Cancelled: cancelled}
	for _, d := range dims {
		v.Total *= len(d.Values)
	}
	for _, c := range cells {
		v.Cells = append(v.Cells, MatrixCellView{Labels: c.Labels, Status: c.Status, Why: c.Why, Detail: c.Detail})
		if c.Status == "passed" {
			v.Passed++
		}
	}
	// A stop that landed during the last cell leaves every cell in place with that one cancelled.
	if !cancelled && len(cells) > 0 && cells[len(cells)-1].Status == "cancelled" {
		v.Cancelled = true
	}
	return v, nil
}
