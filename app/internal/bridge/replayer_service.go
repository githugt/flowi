// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/lib/serve/replay"
)

// ReplayerForm is what the user chooses before starting.
type ReplayerForm struct {
	Addr     string `json:"addr"`
	Latency  string `json:"latency"`
	Sequence bool   `json:"sequence"`
	CORS     bool   `json:"cors"`
	Record   string `json:"record"`
	RecordTo string `json:"recordTo"` // collection id; "" is the Scratch draft
	// RecordHAR is a file the recorded calls are also written to, in order.
	RecordHAR string `json:"recordHar"`
	// Faults: percentages 0 to 100, the status of an injected error, and up to how much extra delay.
	FaultErrorPercent string `json:"faultErrorPercent"`
	FaultStatus       string `json:"faultStatus"`
	FaultDropPercent  string `json:"faultDropPercent"`
	FaultJitter       string `json:"faultJitter"`
}

// TrafficRow is one call the replayer answered.
type TrafficRow struct {
	Seq      int    `json:"seq"`
	At       string `json:"at"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Match    string `json:"match"`
	Request  string `json:"request"`
	Duration string `json:"duration"`
	Remote   string `json:"remote"`
	// Which saved answer was given, and the delay added on purpose ("" when there was none).
	Version string `json:"version"`
	Example string `json:"example"`
	Source  string `json:"source"`
	Delay   string `json:"delay"`
}

// ReplayerView is the replayer mode.
type ReplayerView struct {
	State   string       `json:"state"` // stopped, starting, running or stopping
	Addr    string       `json:"addr"`
	Error   string       `json:"error,omitempty"`
	Notes   []string     `json:"notes"`
	Form    ReplayerForm `json:"form"`
	Targets []Choice     `json:"targets"` // where recorded requests can be saved
	Traffic []TrafficRow `json:"traffic"` // newest first
}

const maxTrafficShown = 300

func replayerStates() map[vm.ReplayerState]string {
	return map[vm.ReplayerState]string{
		vm.ReplayerStopped: "stopped", vm.ReplayerStarting: "starting", vm.ReplayerRunning: "running", vm.ReplayerStopping: "stopping",
	}
}

// ReplayerService runs the replayer of `flowi serve` inside the app and
// shows its traffic.
type ReplayerService struct {
	app   *App
	vm    *vm.Replayer
	first *firstLoad
}

// newReplayerService builds the service for the replayer.
func newReplayerService(a *App) *ReplayerService {
	s := &ReplayerService{app: a, vm: vm.NewReplayer(a.backend, a.worker), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("replayer", s.view())
	})
	return s
}

// State loads the defaults and returns the replayer; changes arrive as the "replayer" event.
func (s *ReplayerService) State() ReplayerView {
	var v ReplayerView
	s.app.actor.Do(s.vm.LoadDefaults)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Start starts the replayer with the form's values.
func (s *ReplayerService) Start(f ReplayerForm) {
	s.app.actor.Do(func() {
		s.vm.Start(vm.ReplayerForm(f))
	})
}

// Stop asks the replayer to stop; calls in progress finish first.
func (s *ReplayerService) Stop() { s.app.actor.Do(s.vm.Stop) }

// Clear forgets the calls shown.
func (s *ReplayerService) Clear() { s.app.actor.Do(s.vm.Clear) }

// Close stops the replayer and waits for it. The shell calls it on shutdown,
// off the actor.
func (s *ReplayerService) Close() { s.vm.Close() }

func (s *ReplayerService) view() ReplayerView {
	f := s.vm.Form()
	v := ReplayerView{
		State: replayerStates()[s.vm.State()], Addr: s.vm.Addr(), Error: s.vm.Err(), Notes: nonNil(s.vm.Notes()),
		Form: ReplayerForm(f), Targets: s.targets(), Traffic: []TrafficRow{},
	}
	entries := s.vm.Entries()
	for i := len(entries) - 1; i >= 0 && len(v.Traffic) < maxTrafficShown; i-- {
		v.Traffic = append(v.Traffic, trafficRowOf(entries[i]))
	}
	return v
}

func trafficRowOf(e replay.Entry) TrafficRow {
	row := TrafficRow{
		Seq: e.Seq, At: e.At.Local().Format(time.TimeOnly), Method: e.Method, Path: e.Path, Status: e.Status, Match: e.Match,
		Request: e.Request, Duration: e.Duration.Round(time.Millisecond).String(), Remote: e.Remote,
		Version: e.Version, Example: e.Example, Source: e.Source,
	}
	if e.Latency > 0 {
		row.Delay = e.Latency.Round(time.Millisecond).String()
	}
	return row
}

// targets are the collections recorded requests can go to; the Scratch draft comes first.
func (s *ReplayerService) targets() []Choice {
	out := []Choice{{ID: "", Label: "Scratch (drafts, not saved to a folder yet)"}}
	for _, c := range s.app.workspace.Collections() {
		if s.app.workspace.IsDraftCollection(c.Manifest.ID) {
			continue
		}
		out = append(out, Choice{ID: string(c.Manifest.ID), Label: c.Manifest.Name, Path: c.Manifest.Name})
	}
	return out
}

// CallHeader is one header line of a captured call.
type CallHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CallPart is one side of a captured call.
type CallPart struct {
	Headers   []CallHeader `json:"headers"`
	Body      string       `json:"body"`
	Truncated bool         `json:"truncated"`
}

// CallView is what a call carried and what it was answered with. Credential values are masked.
type CallView struct {
	Seq      int      `json:"seq"`
	Captured bool     `json:"captured"` // false for a call made before capture was on, or one the log has dropped
	Sent     CallPart `json:"sent"`
	Got      CallPart `json:"got"`
}

// Call returns the headers and bodies of one call by its number.
func (s *ReplayerService) Call(seq int) CallView {
	var v CallView
	s.app.actor.Do(func() {
		e, _ := s.vm.Entry(seq)
		v = callViewOf(e)
	})
	return v
}

func callViewOf(e replay.Entry) CallView {
	v := CallView{Seq: e.Seq}
	if e.Exchange == nil {
		return v
	}
	v.Captured = true
	v.Sent, v.Got = callPartOf(e.Exchange.Sent), callPartOf(e.Exchange.Got)
	return v
}

// callPartOf lists the headers by name, a line for each value.
func callPartOf(m replay.Message) CallPart {
	names := make([]string, 0, len(m.Header))
	for name := range m.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	part := CallPart{Headers: []CallHeader{}, Body: m.Body, Truncated: m.Truncated}
	for _, name := range names {
		for _, value := range m.Header[name] {
			part.Headers = append(part.Headers, CallHeader{Name: name, Value: value})
		}
	}
	return part
}
