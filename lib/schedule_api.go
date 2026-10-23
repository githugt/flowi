// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/serve/scheduler"
)

// Schedules lists the cron schedules in workspace.yaml.
func (w *Workspace) Schedules() []model.Schedule {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return slices.Clone(w.model.Schedules)
}

// AddSchedule saves a schedule in workspace.yaml. Problems with the
// schedule itself are FLW-SERVE-001 (a *diag.ListError) and nothing is
// saved. A flow that cannot be found yet is allowed: collections come and
// go, and Validate warns (FLW-SERVE-002).
func (w *Workspace) AddSchedule(s model.Schedule) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	problems := scheduleProblems(s)
	if slices.ContainsFunc(w.model.Schedules, func(x model.Schedule) bool { return x.Name == s.Name }) {
		problems = append(problems, scheduleDiag(s.Name, "", "a schedule with this name already exists"))
	}
	if len(problems) > 0 {
		return problems.Err()
	}
	next := w.model
	next.Schedules = append(slices.Clone(w.model.Schedules), s)
	return w.saveModel(next)
}

// SaveSchedule replaces the schedule called old with s, in the same place in
// workspace.yaml, in one write. s may carry a new name, as long as no other
// schedule has it. Problems are reported as AddSchedule does and nothing is
// saved.
func (w *Workspace) SaveSchedule(old string, s model.Schedule) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.model.Schedules, func(x model.Schedule) bool { return x.Name == old })
	if i < 0 {
		return diag.List{scheduleDiag(old, "", "there is no schedule with this name")}.Err()
	}
	problems := scheduleProblems(s)
	for j, x := range w.model.Schedules {
		if j != i && x.Name == s.Name {
			problems = append(problems, scheduleDiag(s.Name, "", "a schedule with this name already exists"))
		}
	}
	if len(problems) > 0 {
		return problems.Err()
	}
	next := w.model
	next.Schedules = slices.Clone(w.model.Schedules)
	next.Schedules[i] = s
	return w.saveModel(next)
}

// RemoveSchedule deletes the schedule called name from workspace.yaml.
func (w *Workspace) RemoveSchedule(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.model.Schedules, func(x model.Schedule) bool { return x.Name == name })
	if i < 0 {
		return diag.List{scheduleDiag(name, "", "there is no schedule with this name")}.Err()
	}
	next := w.model
	next.Schedules = slices.Delete(slices.Clone(w.model.Schedules), i, i+1)
	return w.saveModel(next)
}

// ResolveSchedule finds the flow a schedule runs: FLW-FLOW-002/003 or
// FLW-WS-005 as FindFlow reports them.
func (w *Workspace) ResolveSchedule(s model.Schedule) (FlowTarget, error) {
	return w.FindFlow(s.Flow, s.Collection)
}

func scheduleProblems(s model.Schedule) diag.List {
	var out diag.List
	for _, p := range scheduler.Check(s) {
		out = append(out, scheduleDiag(s.Name, p.Field, p.Detail))
	}
	return out
}

func scheduleDiag(name, field, detail string) diag.Diagnostic {
	if field != "" {
		detail = field + " " + detail
	}
	return diag.New(diag.CodeServeSchedule, diag.Location{}, map[string]string{"name": name, "detail": detail})
}

// scheduleDiags are the problems in workspace.yaml's schedules, positioned
// at their lines. Callers hold mu.
func (w *Workspace) scheduleDiags() diag.List {
	var out diag.List
	dups := scheduler.Duplicates(w.model.Schedules)
	for i, s := range w.model.Schedules {
		at := func(field string) diag.Location {
			return diag.Location{File: w.model.File, Path: fmt.Sprintf("$.schedules[%d].%s", i, field)}
		}
		for _, p := range scheduler.Check(s) {
			out = append(out, diag.New(diag.CodeServeSchedule, at(p.Field), map[string]string{"name": s.Name, "detail": p.Field + " " + p.Detail}))
		}
		if slices.Contains(dups, i) {
			out = append(out, diag.New(diag.CodeServeSchedule, at("name"), map[string]string{"name": s.Name, "detail": "another schedule already uses this name"}))
		}
		if s.Flow == "" {
			continue
		}
		if _, err := w.findFlow(s.Flow, s.Collection); err != nil {
			out = append(out, diag.New(diag.CodeServeScheduleFlow, at("flow"), map[string]string{"name": s.Name, "detail": flowProblem(err)}))
		}
	}
	return out
}

// flowProblem is the one-line reason a flow reference did not resolve.
func flowProblem(err error) string {
	var list *diag.ListError
	if errors.As(err, &list) && len(list.List) > 0 {
		msg, _ := diag.English().Message(list.List[0])
		return msg
	}
	return err.Error()
}
