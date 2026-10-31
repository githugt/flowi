// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// ScheduleView is a cron schedule of workspace.yaml.
type ScheduleView struct {
	Name       string `json:"name"`
	Flow       string `json:"flow"`
	Collection string `json:"collection"`
	Cron       string `json:"cron"`
	Env        string `json:"env"`
	Inputs     string `json:"inputs"` // one name=value line each
	TZ         string `json:"tz"`
	Disabled   bool   `json:"disabled"`
	FlowLabel  string `json:"flowLabel"` // the flow it runs, as "Collection › path", "" when none matches
}

// FlowChoice is a flow a schedule can run.
type FlowChoice struct {
	Label      string `json:"label"`
	Collection string `json:"collection"` // collection id
	Flow       string `json:"flow"`       // flow id
}

// UpcomingView is when a schedule fires next.
type UpcomingView struct {
	Name string `json:"name"`
	At   string `json:"at"` // local time, "2026-10-22 03:00"
}

// RunnerView is the state of the schedules running inside the window.
type RunnerView struct {
	State       string         `json:"state"` // stopped, starting, running or stopping
	Error       string         `json:"error,omitempty"`
	Notes       []string       `json:"notes"`
	Upcoming    []UpcomingView `json:"upcoming"`
	TriggerAddr string         `json:"triggerAddr"` // where triggers listen while it runs
}

// SchedulesView is the scheduler mode.
type SchedulesView struct {
	Runner       RunnerView     `json:"runner"`
	Schedules    []ScheduleView `json:"schedules"`
	Flows        []FlowChoice   `json:"flows"`
	Environments []string       `json:"environments"`
	TriggerAddr  string         `json:"triggerAddr"`
	HasToken     bool           `json:"hasToken"`
	Error        string         `json:"error,omitempty"`
}

// SchedulesService edits the cron schedules and the HTTP trigger's token, and
// runs them while the window is open when asked to; `flowi serve` runs them
// without a window.
type SchedulesService struct {
	app    *App
	vm     *vm.Schedules
	runner *vm.ScheduleRunner
	first  *firstLoad
}

// newSchedulesService builds the service for the scheduler.
func newSchedulesService(a *App) *SchedulesService {
	s := &SchedulesService{app: a, vm: vm.NewSchedules(a.backend, a.worker, a.workspace), runner: vm.NewScheduleRunner(a.backend, a.worker), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("schedules", s.view())
	})
	s.runner.OnChange(func() { a.emit("schedules", s.view()) })
	return s
}

// StartRunner starts the schedules (and the HTTP triggers, when a token
// exists): they fire while the window is open.
func (s *SchedulesService) StartRunner() { s.app.actor.Do(s.runner.Start) }

// StopRunner stops them; runs in progress finish first.
func (s *SchedulesService) StopRunner() { s.app.actor.Do(s.runner.Stop) }

// Close stops the runner and waits for it. The shell calls it on shutdown, off the actor.
func (s *SchedulesService) Close() { s.runner.Close() }

func runnerStates() map[vm.ScheduleRunnerState]string {
	return map[vm.ScheduleRunnerState]string{
		vm.ScheduleStopped: "stopped", vm.ScheduleStarting: "starting", vm.ScheduleRunning: "running", vm.ScheduleStopping: "stopping",
	}
}

func (s *SchedulesService) runnerView() RunnerView {
	v := RunnerView{State: runnerStates()[s.runner.State()], Error: s.runner.Err(), Notes: nonNil(s.runner.Notes()), Upcoming: []UpcomingView{}, TriggerAddr: s.runner.TriggerAddr()}
	for _, u := range s.runner.Upcoming() {
		v.Upcoming = append(v.Upcoming, UpcomingView{Name: u.Name, At: u.At.Local().Format("2006-01-02 15:04")})
	}
	return v
}

// State loads the schedules and returns them; changes arrive as the "schedules" event.
func (s *SchedulesService) State() SchedulesView {
	var v SchedulesView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Save adds a schedule (old is "") or replaces the one called old. It
// returns the problem, "" when it was saved.
func (s *SchedulesService) Save(old string, sch ScheduleView) string {
	inputs, err := vm.ParseInputs(sch.Inputs)
	if err != nil {
		return err.Error()
	}
	return s.await(func(done func(error)) {
		s.vm.Save(old, model.Schedule{
			Name: sch.Name, Flow: sch.Flow, Collection: sch.Collection, Cron: sch.Cron, Env: sch.Env, Inputs: inputs, TZ: sch.TZ, Disabled: sch.Disabled,
		}, done)
	})
}

// Remove deletes a schedule.
func (s *SchedulesService) Remove(name string) string {
	return s.await(func(done func(error)) { s.vm.Remove(name, done) })
}

// NewToken makes a fresh trigger token and returns it: the only time its
// value is shown.
func (s *SchedulesService) NewToken() (string, error) {
	type result struct {
		token string
		err   error
	}
	ch := make(chan result, 1)
	s.app.actor.Do(func() { s.vm.NewToken(func(token string, err error) { ch <- result{token, err} }) })
	r := <-ch
	return r.token, r.err
}

// await runs an action on the actor and waits, off it, for its outcome.
func (s *SchedulesService) await(start func(done func(error))) string {
	ch := make(chan error, 1)
	s.app.actor.Do(func() { start(func(err error) { ch <- err }) })
	if err := <-ch; err != nil {
		return vm.ErrorText(err)
	}
	return ""
}

func (s *SchedulesService) view() SchedulesView {
	v := SchedulesView{Runner: s.runnerView(), Schedules: []ScheduleView{}, Flows: []FlowChoice{}, Environments: nonNil(s.vm.Environments()), TriggerAddr: s.vm.TriggerAddr(), HasToken: s.vm.HasToken(), Error: s.vm.Err()}
	flows := s.vm.Flows()
	for _, f := range flows {
		v.Flows = append(v.Flows, FlowChoice{Label: f.Label(), Collection: string(f.CollectionID), Flow: string(f.ID)})
	}
	for _, x := range s.vm.List() {
		sv := ScheduleView{Name: x.Name, Flow: x.Flow, Collection: x.Collection, Cron: x.Cron, Env: x.Env, Inputs: vm.FormatInputs(x.Inputs), TZ: x.TZ, Disabled: x.Disabled}
		if i := s.vm.MatchFlow(x); i >= 0 {
			sv.FlowLabel = flows[i].Label()
		}
		v.Schedules = append(v.Schedules, sv)
	}
	return v
}
