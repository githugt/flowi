// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

const (
	sessionDirMode  = 0o700
	sessionFileMode = 0o600 // histories hold response bodies
)

// SessionStore keeps what carries over between sessions as small JSON files
// in one directory: the open tabs, and (when the user wants it) the run log
// and the response history. Its methods block on the disk: call them through
// a Worker.
type SessionStore struct{ dir string }

// NewSessionStore keeps the session in dir, created when first written.
func NewSessionStore(dir string) *SessionStore { return &SessionStore{dir: dir} }

// SaveTabs stores the node ids of the open tabs.
func (s *SessionStore) SaveTabs(ids []string) error { return s.write("tabs.json", ids) }

// LoadTabs reads the stored tabs; a missing file is an empty list, a damaged
// one an error and an empty list.
func (s *SessionStore) LoadTabs() ([]string, error) {
	var out []string
	return out, s.read("tabs.json", &out)
}

// SaveLog stores the run log's lines.
func (s *SessionStore) SaveLog(lines []string) error { return s.write("log.json", lines) }

// LoadLog reads the stored run log lines.
func (s *SessionStore) LoadLog() ([]string, error) {
	var out []string
	return out, s.read("log.json", &out)
}

// SaveHistory stores the response history.
func (s *SessionStore) SaveHistory(items []StoredExchange) error {
	return s.write("history.json", items)
}

// LoadHistory reads the stored response history.
func (s *SessionStore) LoadHistory() ([]StoredExchange, error) {
	var out []StoredExchange
	return out, s.read("history.json", &out)
}

// DropHistory removes the stored run log and history (the setting that keeps
// them is off, so they should not linger).
func (s *SessionStore) DropHistory() error {
	var first error
	for _, name := range []string{"log.json", "history.json"} {
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return first
}

func (s *SessionStore) read(name string, into any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s is damaged and was ignored: %w", name, err)
	}
	return nil
}

// write replaces the file whole (a temporary file, then a rename), so a
// crash leaves the old content rather than half of the new.
func (s *SessionStore) write(name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", name, err)
	}
	if err := os.MkdirAll(s.dir, sessionDirMode); err != nil {
		return fmt.Errorf("creating %s: %w", s.dir, err)
	}
	tmp, err := os.CreateTemp(s.dir, name+".*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already after a successful rename
	if err := tmp.Chmod(sessionFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return os.Rename(tmp.Name(), filepath.Join(s.dir, name))
}

// StoredExchange is an Exchange as kept on disk: the record is already
// masked of secrets by the library.
type StoredExchange struct {
	ID         int            `json:"id"`
	At         time.Time      `json:"at"`
	Title      string         `json:"title"`
	Collection model.ID       `json:"collection"`
	Request    model.ID       `json:"request"`
	Version    string         `json:"version"`
	RunID      string         `json:"run_id"`
	Status     engine.Status  `json:"status"`
	Record     *engine.Record `json:"record,omitempty"`
}

// Export is the history as it can be stored, newest first.
func (h *History) Export() []StoredExchange {
	out := make([]StoredExchange, 0, min(len(h.items), MaxStoredHistory))
	for _, ex := range h.items[:min(len(h.items), MaxStoredHistory)] {
		out = append(out, StoredExchange{ID: ex.ID, At: ex.At, Title: ex.Title, Collection: ex.Target.Collection, Request: ex.Target.Request,
			Version: ex.Version, RunID: ex.Result.RunID, Status: ex.Result.Status, Record: ex.Result.Record})
	}
	return out
}

// Restore adds stored exchanges, newest first, behind what the history
// holds (they are older), at most MaxStoredHistory in all. They get new ids
// after the ones already used, so none collides with an exchange sent while
// the session was being read.
func (h *History) Restore(stored []StoredExchange) {
	room := max(MaxStoredHistory-len(h.items), 0)
	stored = stored[:min(len(stored), room)]
	restored := make([]Exchange, len(stored))
	for i := len(stored) - 1; i >= 0; i-- { // oldest first, so ids grow with age
		st := stored[i]
		h.nextID++
		restored[i] = Exchange{ID: h.nextID, At: st.At, Title: st.Title, Target: flowi.RequestTarget{Collection: st.Collection, Request: st.Request},
			Version: st.Version, Result: engine.SendResult{RunID: st.RunID, Status: st.Status, Record: st.Record}}
	}
	h.items = append(h.items, restored...)
	h.changed()
}

// Restore puts lines from an earlier session in front of the log.
func (l *RunLog) Restore(lines []string) {
	l.lines = slices.Concat(lines, l.lines)
	l.at = slices.Concat(make([]time.Time, len(lines)), l.at)
	if over := len(l.lines) - maxLogLines; over > 0 {
		l.lines, l.at = l.lines[over:], l.at[over:]
	}
	for _, f := range l.onChange {
		f()
	}
}

// MaxStoredHistory bounds the exchanges kept in the history, in memory and on
// disk: they carry response bodies, and every session would otherwise add to
// what the last one left.
const MaxStoredHistory = 50

// SessionWriter writes session files in the background, one at a time and
// only the newest of the writes queued under a name: a stale snapshot can
// never be renamed over a newer one, and a burst of changes is one write.
// Wait returns once the queue is empty (call it when quitting).
type SessionWriter struct {
	onError func(error)
	mu      sync.Mutex
	pending map[string]func() error
	running bool
	done    sync.WaitGroup
}

// NewSessionWriter returns a writer that hands failures to onError, which
// is called from the writer's goroutine.
func NewSessionWriter(onError func(error)) *SessionWriter {
	return &SessionWriter{onError: onError, pending: map[string]func() error{}}
}

// Queue replaces any write still waiting under name with write.
func (s *SessionWriter) Queue(name string, write func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[name] = write
	if !s.running {
		s.running = true
		s.done.Add(1)
		go s.drain()
	}
}

// drain runs the queued writes until none is left. It is started by Queue
// and ends by itself, so Wait cannot hang.
func (s *SessionWriter) drain() {
	defer s.done.Done()
	for {
		write := s.next()
		if write == nil {
			return
		}
		if err := write(); err != nil {
			s.onError(err)
		}
	}
}

func (s *SessionWriter) next() func() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, write := range s.pending {
		delete(s.pending, name)
		return write
	}
	s.running = false
	return nil
}

// Wait blocks until every queued write has finished.
func (s *SessionWriter) Wait() { s.done.Wait() }
