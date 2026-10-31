// SPDX-License-Identifier: Apache-2.0

// Package scheduler runs flows on cron schedules. Check validates a
// schedule's text; Scheduler (scheduler.go) runs the valid ones.
package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/cron"
)

// neverCheckFrom is the date from which Check looks for a first fire time.
func neverCheckFrom() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// Problem is one thing wrong with a schedule.
type Problem struct {
	Field  string // name, flow, cron, tz or inputs
	Detail string
}

// Check reports every problem in one schedule's own fields. It does not
// look at flows or collections (the workspace does) or at other schedules
// (see Duplicates).
func Check(s model.Schedule) []Problem {
	var out []Problem
	add := func(field, detail string) { out = append(out, Problem{Field: field, Detail: detail}) }
	switch {
	case s.Name == "":
		add("name", "is required")
	case !names.Valid(s.Name):
		add("name", "may use letters, digits, '.', '_' and '-' and must not start with '.'")
	}
	if s.Flow == "" {
		add("flow", "is required")
	}
	if s.Cron == "" {
		add("cron", "is required")
	} else if expr, err := cron.Parse(s.Cron); err != nil {
		add("cron", err.Error())
	} else if expr.Next(neverCheckFrom()).IsZero() {
		add("cron", "never fires (that day of the month does not occur in that month)")
	}
	if s.TZ != "" {
		if _, err := time.LoadLocation(s.TZ); err != nil {
			add("tz", fmt.Sprintf("unknown time zone %q (use a name such as Europe/Berlin)", s.TZ))
		}
	}
	for _, detail := range inputProblems(s.Inputs) {
		add("inputs", detail)
	}
	return out
}

func inputProblems(inputs []model.KeyValue) []string {
	var out []string
	seen := map[string]bool{}
	for _, kv := range inputs {
		switch {
		case kv.Key == "":
			out = append(out, "an input has an empty name")
		case seen[kv.Key]:
			out = append(out, fmt.Sprintf("input %q is given twice", kv.Key))
		}
		seen[kv.Key] = true
	}
	return out
}

// Duplicates returns the indexes of schedules whose name an earlier
// schedule already uses.
func Duplicates(list []model.Schedule) []int {
	var out []int
	seen := map[string]bool{}
	for i, s := range list {
		if s.Name != "" && seen[s.Name] {
			out = append(out, i)
		}
		seen[s.Name] = true
	}
	return out
}

// Job is a schedule ready to fire: parsed cron and zone.
type Job struct {
	model.Schedule
	Expr cron.Schedule
	Loc  *time.Location
}

// ErrInvalid wraps the problems that stop a schedule from becoming a Job.
var ErrInvalid = errors.New("invalid schedule")

// NewJob parses a schedule. A schedule with problems is ErrInvalid.
func NewJob(s model.Schedule) (Job, error) {
	if problems := Check(s); len(problems) > 0 {
		return Job{}, fmt.Errorf("%w %q: %s %s", ErrInvalid, s.Name, problems[0].Field, problems[0].Detail)
	}
	expr, _ := cron.Parse(s.Cron) // Check parsed it
	loc := time.Local
	if s.TZ != "" {
		loc, _ = time.LoadLocation(s.TZ) // Check loaded it
	}
	return Job{Schedule: s, Expr: expr, Loc: loc}, nil
}

// Next is when the job fires after the given time.
func (j Job) Next(after time.Time) time.Time { return j.Expr.Next(after.In(j.Loc)) }
