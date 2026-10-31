// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sync"
	"time"

	"github.com/githugt/flowi/lib/engine"
)

// maxEvents bounds one report; a long-running serve does not grow it
// without limit.
const maxEvents = 200

// Collector gathers the events of one program run. It is safe for
// concurrent use. With LevelOff it keeps nothing.
type Collector struct {
	level  Level
	client Client
	now    func() time.Time

	mu     sync.Mutex // guards events and runs
	events []Event
	runs   map[string]*runTally
}

type runTally struct {
	kind  string
	nodes int
}

// NewCollector returns a collector for level; client is made safe first.
func NewCollector(level Level, client Client, now func() time.Time) *Collector {
	return &Collector{level: level, client: client.Sanitised(), now: now, runs: map[string]*runTally{}}
}

// Level is the level the collector currently collects at.
func (c *Collector) Level() Level {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.level
}

// Lower reduces the level, dropping events the new level does not allow. It
// never raises it: turning telemetry down while the program runs must stop
// what was collected under the higher level from being sent.
func (c *Collector) Lower(l Level) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l >= c.level {
		return
	}
	c.level = l
	kept := c.events[:0]
	for _, e := range c.events {
		if spec, ok := specOf(e.Name); ok && spec.Level <= l && l != LevelOff {
			kept = append(kept, e)
		}
	}
	c.events = kept
}

// Started records that the application started.
func (c *Collector) Started() { _ = c.Record(EventAppStarted, nil) } // a catalogued event with no fields cannot be refused

// Record adds an event. Anything the catalogue does not name is refused with
// ErrNotInCatalogue and not kept; an event above the level is dropped
// silently.
func (c *Collector) Record(name string, fields map[string]string) error {
	if err := checkEvent(name, fields); err != nil {
		return err
	}
	spec, _ := specOf(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.level == LevelOff || spec.Level > c.level {
		return nil
	}
	if len(c.events) < maxEvents {
		c.events = append(c.events, Event{Name: name, Fields: fields})
	}
	return nil
}

// Observe reports finished runs from the engine's events. It matches
// engine.Deps.Observe; it keeps only counts, never the events themselves.
func (c *Collector) Observe(ev engine.Event) {
	if c.Level() < LevelFull {
		return
	}
	switch e := ev.(type) {
	case engine.RunStarted:
		c.mu.Lock()
		c.runs[e.RunID] = &runTally{kind: e.Kind}
		c.mu.Unlock()
	case engine.NodeFinished:
		c.mu.Lock()
		if t, ok := c.runs[e.RunID]; ok {
			t.nodes++
		}
		c.mu.Unlock()
	case engine.RunFinished:
		c.finishRun(e.RunID, string(e.Status), e.Duration)
	case engine.RunAborted:
		c.finishRun(e.RunID, string(engine.StatusAborted), 0)
	case engine.RunCancelled:
		if t := c.tally(e.RunID); t != nil && t.kind == "send" {
			c.finishRun(e.RunID, string(engine.StatusCancelled), 0)
		}
	}
}

func (c *Collector) tally(runID string) *runTally {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runs[runID]
}

func (c *Collector) finishRun(runID, status string, d time.Duration) {
	c.mu.Lock()
	t, ok := c.runs[runID]
	delete(c.runs, runID)
	c.mu.Unlock()
	if !ok {
		return
	}
	// A status outside the catalogue (a future engine status) is refused by
	// Record, which is the point of the catalogue.
	_ = c.Record(EventRunFinished, map[string]string{"kind": t.kind, "status": status, "duration": durationBucket(d), "nodes": countBucket(t.nodes)})
}

// Payload returns what would be sent. ok is false when nothing should be
// sent: telemetry is off, or there is nothing to report.
func (c *Collector) Payload() (Payload, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.level == LevelOff || len(c.events) == 0 {
		return Payload{}, false
	}
	return Payload{
		Schema: payloadSchema, Date: c.now().UTC().Format("2006-01-02"), Level: c.level.String(),
		Client: c.client, Events: append([]Event(nil), c.events...),
	}, true
}

func durationBucket(d time.Duration) string {
	switch {
	case d < time.Second:
		return "<1s"
	case d < 10*time.Second:
		return "<10s"
	case d < time.Minute:
		return "<1m"
	case d < 10*time.Minute:
		return "<10m"
	}
	return ">=10m"
}

func countBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n <= 5:
		return "1-5"
	case n <= 20:
		return "6-20"
	}
	return "21+"
}
