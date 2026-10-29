// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"context"
	"sync"
	"time"
)

// Match values of an Entry.
const (
	MatchOK         = "ok"          // answered from a saved response
	MatchNone       = "none"        // no request fits the call
	MatchNoResponse = "no-response" // a request fits, but no response (or not the one asked for)
	MatchBad        = "bad-request" // a malformed X-Flowi-* header
	MatchRecorded   = "recorded"    // forwarded upstream and saved
	MatchForwarded  = "forwarded"   // forwarded upstream, not saved
	MatchFault      = "fault"       // an error or a dropped connection made on purpose
)

// Entry is one call the replayer handled.
type Entry struct {
	Seq      int           `json:"seq"`
	At       time.Time     `json:"at"`
	Method   string        `json:"method"`
	Path     string        `json:"path"`
	Status   int           `json:"status"`
	Match    string        `json:"match"`
	Request  string        `json:"request,omitempty"` // the matched request's path
	Version  string        `json:"version,omitempty"`
	Example  string        `json:"example,omitempty"`
	Source   string        `json:"source,omitempty"`
	Latency  time.Duration `json:"latency"`  // simulated delay
	Duration time.Duration `json:"duration"` // time to answer, delay included
	Remote   string        `json:"remote,omitempty"`
}

// subscriberBuffer is how many entries a slow subscriber can fall behind
// before it starts missing them.
const subscriberBuffer = 256

// Log keeps the newest entries and streams new ones to subscribers. It is
// safe for concurrent use.
type Log struct {
	mu   sync.Mutex // guards every field below
	size int
	next int
	list []Entry
	subs map[chan Entry]struct{}
}

// NewLog keeps at most size entries (at least 1).
func NewLog(size int) *Log {
	return &Log{size: max(size, 1), subs: map[chan Entry]struct{}{}}
}

// Add records an entry, numbering it. It never blocks: a subscriber whose
// buffer is full misses the entry.
func (l *Log) Add(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next++
	e.Seq = l.next
	l.list = append(l.list, e)
	if len(l.list) > l.size {
		l.list = l.list[len(l.list)-l.size:]
	}
	for ch := range l.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Entries returns a copy of the kept entries, oldest first.
func (l *Log) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Entry(nil), l.list...)
}

// Subscribe streams entries added from now on. The channel is closed when
// ctx ends; the goroutine started here exits then.
func (l *Log) Subscribe(ctx context.Context) <-chan Entry {
	ch := make(chan Entry, subscriberBuffer)
	l.mu.Lock()
	l.subs[ch] = struct{}{}
	l.mu.Unlock()
	go func() {
		<-ctx.Done()
		l.mu.Lock()
		delete(l.subs, ch)
		l.mu.Unlock()
		close(ch)
	}()
	return ch
}
