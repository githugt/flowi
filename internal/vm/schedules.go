// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// FlowOption is a flow a schedule can run.
type FlowOption struct {
	Collection   string // the collection's name
	CollectionID model.ID
	ID           model.ID
	Path         string
}

// Label is how the flow is listed.
func (f FlowOption) Label() string { return f.Collection + " › " + f.Path }

// Schedules is the scheduler mode: the cron schedules of workspace.yaml,
// and the HTTP trigger's address and token. The app does not run them;
// `flowi serve` does.
type Schedules struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	list     []model.Schedule
	addr     string
	hasToken bool
	err      string
	onChange listeners
	queue    opQueue
}

// NewSchedules returns the view-model; call Load.
func NewSchedules(b Backend, w *Worker, ws *Workspace) *Schedules {
	return &Schedules{b: b, w: w, ws: ws}
}

// OnChange registers f to run after the schedules or the error change.
// It returns the function that removes f again.
func (s *Schedules) OnChange(f func()) (remove func()) { return s.onChange.add(f) }

// List is the schedules, in the order of workspace.yaml.
func (s *Schedules) List() []model.Schedule { return s.list }

// TriggerAddr is where `flowi serve` listens for HTTP triggers.
func (s *Schedules) TriggerAddr() string { return s.addr }

// HasToken reports that a trigger token exists. Triggers stay off without one.
func (s *Schedules) HasToken() bool { return s.hasToken }

// Err is the last problem, or "".
func (s *Schedules) Err() string { return s.err }

// Find returns the schedule called name.
func (s *Schedules) Find(name string) (model.Schedule, bool) {
	i := slices.IndexFunc(s.list, func(x model.Schedule) bool { return x.Name == name })
	if i < 0 {
		return model.Schedule{}, false
	}
	return s.list[i], true
}

// Flows are the flows of every mounted collection.
func (s *Schedules) Flows() []FlowOption {
	var out []FlowOption
	for _, c := range s.ws.Collections() {
		for _, f := range c.Flows {
			out = append(out, FlowOption{Collection: c.Manifest.Name, CollectionID: c.Manifest.ID, ID: f.ID, Path: f.Path})
		}
	}
	return out
}

// MatchFlow is the index in Flows of the flow a schedule runs, found as the
// library finds it: by id, or by path (with or without flows/ and
// .arazzo.yaml), in the collection the schedule names (by name or id), or in
// any. -1 when no flow matches, or when several collections do (the library
// refuses that too), so the form keeps the schedule's own words.
func (s *Schedules) MatchFlow(x model.Schedule) int {
	ref := strings.TrimSuffix(strings.TrimPrefix(x.Flow, "flows/"), ".arazzo.yaml")
	found, at := 0, -1
	seen := map[model.ID]bool{}
	for i, f := range s.Flows() {
		inCollection := x.Collection == "" || x.Collection == f.Collection || x.Collection == string(f.CollectionID)
		if !inCollection || seen[f.CollectionID] || (string(f.ID) != x.Flow && f.Path != ref) {
			continue
		}
		seen[f.CollectionID] = true
		found, at = found+1, i
	}
	if found != 1 {
		return -1
	}
	return at
}

// Environments are the environment names of every mounted collection,
// each once.
func (s *Schedules) Environments() []string {
	var out []string
	for _, e := range s.ws.Environment() {
		if !slices.Contains(out, e.Name) {
			out = append(out, e.Name)
		}
	}
	return out
}

// Load reads the schedules and the trigger state in the background.
func (s *Schedules) Load() { s.change(func() error { return nil }, nil) }

// Save adds a schedule (old is "") or replaces the one called old.
func (s *Schedules) Save(old string, sched model.Schedule, done func(error)) {
	s.change(func() error {
		if old == "" {
			return s.b.AddSchedule(sched)
		}
		return s.b.SaveSchedule(old, sched)
	}, done)
}

// Remove deletes the schedule called name.
func (s *Schedules) Remove(name string, done func(error)) {
	s.change(func() error { return s.b.RemoveSchedule(name) }, done)
}

// NewToken makes a fresh trigger token, replacing any earlier one, and
// hands it to done. This is the only time the value exists outside the
// vault: it is not kept here.
func (s *Schedules) NewToken(done func(token string, err error)) {
	var token string
	s.change(func() error {
		var err error
		token, err = s.b.NewTriggerToken()
		return err
	}, func(err error) { done(token, err) })
}

// change runs act, then reads the state, both off the UI goroutine, shows
// it, and tells done how act went.
func (s *Schedules) change(act func() error, done func(error)) {
	s.queue.add(func(finish func()) { s.run(act, done, finish) })
}

func (s *Schedules) run(act func() error, done func(error), finish func()) {
	type loaded struct {
		list     []model.Schedule
		addr     string
		hasToken bool
		actErr   error
		err      error
	}
	Run(s.w, func() loaded {
		actErr := act()
		has, err := s.b.HasTriggerToken()
		return loaded{s.b.Schedules(), s.b.ServeDefaults().TriggerAddr, has, actErr, err}
	}, func(l loaded) {
		s.list, s.addr, s.hasToken, s.err = l.list, l.addr, l.hasToken, ""
		switch {
		case l.actErr != nil:
			s.err = ErrorText(l.actErr)
		case l.err != nil:
			s.err = ErrorText(l.err)
		}
		s.onChange.fire()
		if done != nil {
			done(l.actErr)
		}
		finish()
	})
}

// FormatInputs writes inputs as one key=value line each.
func FormatInputs(in []model.KeyValue) string {
	lines := make([]string, len(in))
	for i, kv := range in {
		lines[i] = kv.Key + "=" + kv.Value
	}
	return strings.Join(lines, "\n")
}

// ParseInputs reads key=value lines (blank lines are skipped; the value is
// everything after the first =).
func ParseInputs(text string) ([]model.KeyValue, error) {
	var out []model.KeyValue
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("line %d: write an input as name=value", i+1)
		}
		out = append(out, model.KeyValue{Key: strings.TrimSpace(key), Value: value})
	}
	return out, nil
}
