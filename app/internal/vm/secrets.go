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
	vault    flowi.VaultState
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

// Vault is where the vault stands: open or locked, and when it locks by itself.
func (s *Secrets) Vault() flowi.VaultState { return s.vault }

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

// Declare makes name a secret of the environment (when it is not one) and stores value as its value.
func (s *Secrets) Declare(info flowi.SecretInfo, value string, done func(error)) {
	if value == "" {
		done(errors.New("a secret needs a value"))
		return
	}
	s.change(func() error { return s.b.DeclareSecret(info.Environment, info.Name, value) }, done)
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
		vault  flowi.VaultState
	}
	Run(s.w, func() loaded {
		actErr := act()
		infos, err := s.b.Secrets()
		return loaded{infos, err, actErr, s.b.VaultState()}
	}, func(l loaded) {
		s.err, s.rows, s.vault = "", groupSecrets(l.infos), l.vault
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

// Lock forgets the vault's key now and shows the new state. The list is not read again: that would open the vault
// at once (and ask for the passphrase) to show what it holds.
func (s *Secrets) Lock() {
	s.queue.add(func(finish func()) {
		Run(s.w, func() flowi.VaultState {
			s.b.LockVault()
			return s.b.VaultState()
		}, func(st flowi.VaultState) {
			s.vault = st
			s.onChange.fire()
			finish()
		})
	})
}

// Tick is what a clock calls from time to time: the vault locks when vault.auto_lock says it has been idle long
// enough, and the change is shown. A vault that stays open only has its countdown refreshed when the window asks
// (Refresh).
func (s *Secrets) Tick() {
	s.queue.add(func(finish func()) {
		type result struct {
			locked bool
			vault  flowi.VaultState
		}
		Run(s.w, func() result { return result{s.b.AutoLockVault(), s.b.VaultState()} }, func(r result) {
			if r.locked || r.vault != s.vault {
				s.vault = r.vault
				s.onChange.fire()
			}
			finish()
		})
	})
}

// Refresh reads the vault's state again without touching the secrets.
func (s *Secrets) Refresh() {
	s.queue.add(func(finish func()) {
		Run(s.w, s.b.VaultState, func(st flowi.VaultState) {
			s.vault = st
			s.onChange.fire()
			finish()
		})
	})
}
