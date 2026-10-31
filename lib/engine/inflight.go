// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"sync"

	"github.com/githugt/flowi-lib/model"
)

// inflight tracks the top-level leaf nodes running, in start order, so an
// aborted run can say what was stuck. Containers are left out (their
// running children are listed instead) and so are nodes inside hooks
// (their request node is listed). mu guards paths.
type inflight struct {
	mu    sync.Mutex
	paths []string
}

func (in *inflight) add(path string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.paths = append(in.paths, path)
}

func (in *inflight) remove(path string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.paths = slices.DeleteFunc(in.paths, func(p string) bool { return p == path })
}

func (in *inflight) snapshot() []string {
	in.mu.Lock()
	defer in.mu.Unlock()
	return slices.Clone(in.paths)
}

// isContainer reports node types that run other nodes.
func isContainer(t model.NodeType) bool {
	return slices.Contains([]model.NodeType{"batch", "run_till", "parallel", "condition", "subflow"}, t)
}
