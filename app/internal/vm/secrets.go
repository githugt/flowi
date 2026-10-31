// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"

	flowi "github.com/githugt/flowi/lib"
)

// SecretState is where a secret's value comes from.
type SecretState int

// States.
const (
	SecretMissing    SecretState = iota // no value anywhere: runs that need it fail
	SecretStored                        // in the vault
	SecretOverridden                    // FLOWI_SECRET_<ENV>_<NAME> is set, and wins over the vault
)

// SecretRow is a line of the secrets list: an environment heading, or one
// secret.
type SecretRow struct {
	Header string
	Info   flowi.SecretInfo
}

// State says where the secret's value comes from.
func (r SecretRow) State() SecretState {
	switch {
	case r.Info.Override:
		return SecretOverridden
	case r.Info.Set:
		return SecretStored
	}
	return SecretMissing
}

// SecretKey identifies a secret in lists and tab ids.
func SecretKey(s flowi.SecretInfo) string {
	return s.Environment + "\x00" + s.Name
}

// Secrets is the secrets explorer: every secret the environments declare,
// with whether it has a value, and setting or clearing
// that value in the vault. Values are never read back.
type Secrets struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	rows     []SecretRow
	err      string
	onChange listeners
	queue    opQueue
}

// NewSecrets returns the explorer; call Load.
func NewSecrets(b Backend, w *Worker, ws *Workspace) *Secrets {
	return &Secrets{b: b, w: w, ws: ws}
}

// OnChange registers f to run after the rows or the error change.
// It returns the function that removes f again.
func (s *Secrets) OnChange(f func()) (remove func()) { return s.onChange.add(f) }

// Rows are the list's lines, grouped by environment.
func (s *Secrets) Rows() []SecretRow { return s.rows }

// Err is the last problem, or "".
func (s *Secrets) Err() string { return s.err }

// Find returns the secret with key (SecretKey).
func (s *Secrets) Find(key string) (flowi.SecretInfo, bool) {
	for _, r := range s.rows {
		if r.Header == "" && SecretKey(r.Info) == key {
			return r.Info, true
		}
	}
	return flowi.SecretInfo{}, false
}

// Load reads the secrets in the background.
func (s *Secrets) Load() { s.change(func() error { return nil }, nil) }

// Set stores value as the secret's value. An empty value is refused: it
// would make a secret that looks set and is useless. done gets the outcome
// once the list shows it.
func (s *Secrets) Set(info flowi.SecretInfo, value string, done func(error)) {
	if value == "" {
		done(errors.New("a secret needs a value; use Unset to remove it"))
		return
	}
	s.change(func() error { return s.b.SetSecret(info.Environment, info.Name, value) }, done)
}

// Unset removes the stored value.
func (s *Secrets) Unset(info flowi.SecretInfo, done func(error)) {
	s.change(func() error { return s.b.UnsetSecret(info.Environment, info.Name) }, done)
}

// change runs act, then reads the secrets, both off the UI goroutine, shows
// them, and tells done how act went.
func (s *Secrets) change(act func() error, done func(error)) {
	s.queue.add(func(finish func()) { s.run(act, done, finish) })
}

func (s *Secrets) run(act func() error, done func(error), finish func()) {
	type loaded struct {
		infos  []flowi.SecretInfo
		err    error
		actErr error
	}
	Run(s.w, func() loaded {
		actErr := act()
		infos, err := s.b.Secrets()
		return loaded{infos, err, actErr}
	}, func(l loaded) {
		s.err, s.rows = "", groupSecrets(l.infos)
		switch {
		case l.err != nil:
			s.err = ErrorText(l.err)
		case l.actErr != nil:
			s.err = ErrorText(l.actErr)
		}
		s.onChange.fire()
		if done != nil {
			done(l.actErr)
		}
		finish()
	})
}

// groupSecrets lines the secrets up under a heading per environment, in
// the order the library listed them.
func groupSecrets(infos []flowi.SecretInfo) []SecretRow {
	var rows []SecretRow
	lastHeader := ""
	for _, info := range infos {
		if info.Environment != lastHeader {
			rows = append(rows, SecretRow{Header: info.Environment})
			lastHeader = info.Environment
		}
		rows = append(rows, SecretRow{Info: info})
	}
	return rows
}
