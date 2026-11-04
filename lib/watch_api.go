// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/watch"
)

// WatchResult is one watch pass over a flow.
type WatchResult struct {
	Status engine.Status  `json:"status"`
	First  bool           `json:"first"` // nothing to compare with: this run became the baseline
	Steps  int            `json:"steps"`
	Change []watch.Change `json:"changes"`
}

// readOnlyMethods are the methods a watched flow may send: they do not change anything on the server.
func readOnlyMethods() []string { return []string{"GET", "HEAD", "OPTIONS"} }

// WatchProblems lists why a flow may not be watched: a watch runs it again and again, unattended, so every step
// must be one that changes nothing. An empty list means it may.
func (w *Workspace) WatchProblems(t FlowTarget) ([]string, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	return watchProblemsOf(c, f), nil
}

func watchProblemsOf(c model.Collection, f model.Flow) []string {
	problems := []string{}
	var visit func(nodes []model.Node)
	visit = func(nodes []model.Node) {
		for _, n := range nodes {
			if p := stepWatchProblem(c, n); p != "" {
				problems = append(problems, p)
			}
			for _, child := range n.Children() {
				visit(child)
			}
		}
	}
	visit(f.Nodes)
	return problems
}

// stepWatchProblem is why one step may not run unattended, or "".
func stepWatchProblem(c model.Collection, n model.Node) string {
	switch n.Type {
	case "request":
		return requestWatchProblem(c, n)
	case "webhook", "subflow", "fuzz", "save":
		return fmt.Sprintf("step %s is a %s step, which can change something outside the flow", n.ID, n.Type)
	}
	return ""
}

func requestWatchProblem(c model.Collection, n model.Node) string {
	if n.Request == nil {
		return ""
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool {
		return (n.Request.Request.ID != "" && r.ID == n.Request.Request.ID) || (n.Request.Request.Path != "" && r.Path == n.Request.Request.Path)
	})
	if i < 0 {
		return fmt.Sprintf("step %s calls a request that cannot be found", n.ID)
	}
	v, ok := defaultVersion(c.Requests[i])
	if !ok {
		return ""
	}
	if v.Async != nil || v.GRPC != nil || v.Dataset != nil {
		return fmt.Sprintf("step %s is not a plain HTTP request", n.ID)
	}
	if method := strings.ToUpper(v.Method); !slices.Contains(readOnlyMethods(), method) {
		return fmt.Sprintf("step %s sends %s, which may change something on the server", n.ID, method)
	}
	return ""
}

// WatchFlow runs a read-only flow once and compares what its calls answered with the last watch pass: another
// status, a response whose shape moved, a call much slower. The run becomes the new baseline. A flow that is not
// read-only is refused, with the reasons.
func (w *Workspace) WatchFlow(ctx context.Context, t FlowTarget, o RunOptions) (WatchResult, error) {
	problems, err := w.WatchProblems(t)
	if err != nil {
		return WatchResult{}, err
	}
	if len(problems) > 0 {
		return WatchResult{}, fmt.Errorf("this flow cannot be watched: %s", strings.Join(problems, "; "))
	}
	run, err := w.RunFlow(ctx, t, o)
	if err != nil {
		return WatchResult{}, err
	}
	res := run.Wait()
	cur := watch.Take(res.Captured)
	file := w.watchFile(t)
	prev, err := readSnapshot(file)
	if err != nil {
		return WatchResult{}, err
	}
	out := WatchResult{Status: res.Status, First: prev == nil, Steps: len(cur.Steps), Change: watch.Compare(prev, cur)}
	if err := w.recordPass(t, watch.PassOf(cur, string(res.Status), out.Change)); err != nil {
		return WatchResult{}, err
	}
	if res.Status != engine.StatusPassed {
		// A failed run says little about the API's shape: keep the last good baseline.
		return out, nil
	}
	return out, writeSnapshot(file, cur)
}

// WatchHistory lists the passes of a flow's watch, oldest first: when each ran, how it ended, how much had moved and
// how long the calls took. A flow never watched has none.
func (w *Workspace) WatchHistory(t FlowTarget) ([]watch.Pass, error) {
	data, err := os.ReadFile(w.watchHistoryFile(t)) //nolint:gosec // the workspace's own data directory
	if errors.Is(err, fs.ErrNotExist) {
		return []watch.Pass{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the watch history: %w", err)
	}
	var history []watch.Pass
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("reading the watch history: %w", err)
	}
	return history, nil
}

// ScheduleWatchHistory is WatchHistory for the flow a schedule watches. A flow that cannot be found has no passes: the
// schedule may be waiting for a collection that is not mounted yet.
func (w *Workspace) ScheduleWatchHistory(s model.Schedule) ([]watch.Pass, error) {
	target, err := w.ResolveSchedule(s)
	if err != nil {
		return []watch.Pass{}, nil //nolint:nilerr // see above: an unresolved flow is not a failure to read history
	}
	return w.WatchHistory(target)
}

func (w *Workspace) recordPass(t FlowTarget, p watch.Pass) error {
	history, err := w.WatchHistory(t)
	if err != nil {
		return err
	}
	data, err := json.Marshal(watch.Append(history, p, watch.HistoryLimit))
	if err != nil {
		return fmt.Errorf("encoding the watch history: %w", err)
	}
	file := w.watchHistoryFile(t)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil { //nolint:gosec // the workspace's own data directory
		return fmt.Errorf("making the watch directory: %w", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("saving the watch history: %w", err)
	}
	return nil
}

func (w *Workspace) watchHistoryFile(t FlowTarget) string {
	return filepath.Join(w.DataDir(t.Collection), "watch", string(t.Flow)+".history.json")
}

func (w *Workspace) watchFile(t FlowTarget) string {
	return filepath.Join(w.DataDir(t.Collection), "watch", string(t.Flow)+".json")
}

func readSnapshot(file string) (*watch.Snapshot, error) {
	data, err := os.ReadFile(file) //nolint:gosec // the workspace's own data directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the last watch pass: %w", err)
	}
	var s watch.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("reading the last watch pass %s: %w", file, err)
	}
	return &s, nil
}

func writeSnapshot(file string, s watch.Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil { //nolint:gosec // the workspace's own data directory
		return fmt.Errorf("making the watch directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the watch pass: %w", err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("saving the watch pass: %w", err)
	}
	return nil
}
