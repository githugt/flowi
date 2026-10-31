// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"sync"
	"time"
)

// Errors Ask returns when no passphrase comes back. The vault stays locked,
// and asks again on the next use of a secret.
var (
	ErrPassphraseCancelled = errors.New("the vault passphrase was not entered")
	ErrPassphraseClosed    = errors.New("the window closed before the vault passphrase was entered")
)

// PassphraseService asks the window for the vault's passphrase. A vault made
// with a passphrase (where no OS keychain was available) needs it before any
// secret can be read or written.
//
// The vault calls Ask off the actor, from the goroutine using a secret, and
// that goroutine waits until the window calls Answer or Cancel. Neither
// touches the actor, so a question asked from the actor still gets its answer.
//
// Secrets are often read by several goroutines at once (every open tab). They
// share one question and its answer: a Cancel ends them all, rather than the
// window asking again for each. After a Cancel the vault stays locked, and
// later reads fail at once, until the user does something that needs secrets
// and the window calls Unlock.
type PassphraseService struct {
	app *App

	mu       sync.Mutex // guards open and declined
	open     *question  // the question the window shows; nil when none is
	declined bool       // the user cancelled: do not ask until Unlock

	closeOnce sync.Once
	closed    chan struct{} // closed by Close: questions still open end
}

// question is one question to the window. answer is written once, before
// done is closed, and read only after.
type question struct {
	done   chan struct{}
	answer askedPassword
}

type askedPassword struct {
	phrase string
	err    error
}

func newPassphraseService(a *App) *PassphraseService {
	return &PassphraseService{app: a, closed: make(chan struct{})}
}

// passphraseReminder is how often an unanswered question is published again.
const passphraseReminder = 2 * time.Second

// Ask publishes the "passphrase" event and waits for the window's answer.
// It is the vault's Passphrase function.
//
// The goroutine that opened the question publishes it again every
// passphraseReminder until it is answered: an event published while the
// window was not connected (a reload, the browser build's event socket
// reconnecting) is lost, and the vault would wait for good. The window
// ignores a repeat while its dialog shows.
func (s *PassphraseService) Ask() (string, error) {
	q, asking := s.question()
	if q == nil {
		return "", ErrPassphraseCancelled
	}
	var remind <-chan time.Time
	if asking {
		s.app.emit("passphrase", nil)
		t := time.NewTicker(passphraseReminder)
		defer t.Stop()
		remind = t.C
	}
	for {
		select {
		case <-q.done:
			return q.answer.phrase, q.answer.err
		case <-s.closed:
			return "", ErrPassphraseClosed
		case <-remind:
			s.app.emit("passphrase", nil)
		}
	}
}

// question returns the open question, opening one when there is none;
// asking says it is new, so the window has yet to be asked. It is nil while
// the user has declined to answer.
func (s *PassphraseService) question() (q *question, asking bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.declined {
		return nil, false
	}
	if s.open == nil {
		s.open, asking = &question{done: make(chan struct{})}, true
	}
	return s.open, asking
}

// Pending reports whether a question waits for an answer. The window asks
// when it starts: a question published before it listened (a reload) would
// otherwise never show, and the goroutines waiting on it never end.
func (s *PassphraseService) Pending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.open != nil
}

// Answer gives the open question its passphrase. Without an open question
// it does nothing.
func (s *PassphraseService) Answer(phrase string) {
	s.reply(askedPassword{phrase: phrase})
}

// Cancel answers the open question with ErrPassphraseCancelled, and every
// later one too, until Unlock.
func (s *PassphraseService) Cancel() {
	s.reply(askedPassword{err: ErrPassphraseCancelled})
}

// Unlock lets the next use of a secret ask for the passphrase again. The
// window calls it when the user turns to secrets (an environment, the Secrets
// screen) after cancelling.
func (s *PassphraseService) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.declined = false
}

func (s *PassphraseService) reply(a askedPassword) {
	s.mu.Lock()
	q := s.open
	s.open = nil
	s.declined = errors.Is(a.err, ErrPassphraseCancelled)
	s.mu.Unlock()
	if q != nil {
		q.answer = a
		close(q.done)
	}
}

// Close ends every open and later question with ErrPassphraseClosed. Call it
// when the window shuts down, so no goroutine waits on a window that is gone.
func (s *PassphraseService) Close() {
	s.closeOnce.Do(func() { close(s.closed) })
}
