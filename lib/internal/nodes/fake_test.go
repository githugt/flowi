// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// fakeRuntime records what nodes ask for and answers from scripts.
type fakeRuntime struct {
	mu sync.Mutex

	stopped  bool
	now      time.Time
	laneStep time.Duration // Now advances by this per lane run

	lanes     [][]Lane
	laneOpts  []LaneOptions
	laneReply func(call int, lanes []Lane) []LaneResult

	requests   []RequestCall
	requestOut Outcome
	subflows   []map[string]any
	subflowOut Outcome
	https      []HTTPCall
	httpOut    HTTPResult
	background []HTTPCall

	sleeps   []time.Duration
	sleepErr error
	random   time.Duration

	reports map[string]report.Run
	files   []string
	written map[string][]byte
	fileErr error

	schemaOut map[string]SchemaCheck

	fuzz       FuzzSubject
	fuzzErr    error
	requestFor func(RequestCall) Outcome // when set, answers each request
}

func (f *fakeRuntime) CheckSchema(step string) SchemaCheck { return f.schemaOut[step] }

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{now: time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC), reports: map[string]report.Run{}, written: map[string][]byte{}}
}

func (f *fakeRuntime) Lanes(_ context.Context, lanes []Lane, o LaneOptions) []LaneResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := len(f.lanes)
	f.lanes = append(f.lanes, lanes)
	f.laneOpts = append(f.laneOpts, o)
	f.now = f.now.Add(f.laneStep)
	if f.laneReply != nil {
		return f.laneReply(call, lanes)
	}
	out := make([]LaneResult, len(lanes))
	for i := range out {
		out[i] = LaneResult{Status: StatusPassed}
	}
	return out
}

func (f *fakeRuntime) Request(_ context.Context, c RequestCall) Outcome {
	f.requests = append(f.requests, c)
	if f.requestFor != nil {
		return f.requestFor(c)
	}
	return f.requestOut
}

func (f *fakeRuntime) FuzzSubject(context.Context, RequestCall) (FuzzSubject, error) {
	return f.fuzz, f.fuzzErr
}

func (f *fakeRuntime) Subflow(_ context.Context, _ model.Ref, inputs map[string]any) Outcome {
	f.subflows = append(f.subflows, inputs)
	return f.subflowOut
}

func (f *fakeRuntime) HTTP(_ context.Context, c HTTPCall) HTTPResult {
	f.https = append(f.https, c)
	return f.httpOut
}

func (f *fakeRuntime) Background(_ string, c HTTPCall) { f.background = append(f.background, c) }

func (f *fakeRuntime) Sleep(_ context.Context, d time.Duration) error {
	f.sleeps = append(f.sleeps, d)
	f.now = f.now.Add(d)
	return f.sleepErr
}

func (f *fakeRuntime) StopRun() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
}

func (f *fakeRuntime) Stopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

func (f *fakeRuntime) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeRuntime) RandomDuration(lo, _ time.Duration) time.Duration {
	if f.random != 0 {
		return f.random
	}
	return lo
}

func (f *fakeRuntime) Report(of string) (report.Run, bool) {
	r, ok := f.reports[of]
	return r, ok
}

func (f *fakeRuntime) WriteFile(dir, name string, data []byte, appendTo bool) (string, error) {
	if f.fileErr != nil {
		return "", f.fileErr
	}
	key := dir + "/" + name
	if appendTo {
		f.written[key] = append(f.written[key], data...)
	} else {
		f.written[key] = data
	}
	f.files = append(f.files, key)
	return key, nil
}
