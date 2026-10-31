// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"

	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/vm"
)

// SecretView is a secret declared by an environment. Values are not part of it:
// the window asks for one at a time with Reveal.
type SecretView struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	State       string `json:"state"` // missing, stored or overridden
}

// SecretsView is the secrets explorer.
type SecretsView struct {
	Secrets []SecretView `json:"secrets"`
	Error   string       `json:"error,omitempty"`
}

// SecretsService is the secrets explorer: set or clear values in the vault.
type SecretsService struct {
	app   *App
	vm    *vm.Secrets
	first *firstLoad
}

// newSecretsService builds the service for the secrets explorer.
func newSecretsService(a *App) *SecretsService {
	s := &SecretsService{app: a, vm: vm.NewSecrets(a.backend, a.worker, a.workspace), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("secrets", s.view())
	})
	return s
}

// State loads the secrets and returns them; changes arrive as the "secrets" event.
func (s *SecretsService) State() SecretsView {
	var v SecretsView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Set stores a value in the vault. It is never read back.
func (s *SecretsService) Set(environment, name, value string) {
	s.app.actor.Do(func() {
		s.vm.Set(flowi.SecretInfo{Environment: environment, Name: name}, value, func(error) {})
	})
}

// Reveal returns one stored value for the window to show for a moment; it is the only call that sends a
// value to the window, and it sends one. The window asks the passphrase first (PassphraseService.Unlock),
// so this never waits for the user. The reveal is logged, without the value.
func (s *SecretsService) Reveal(environment, name string) (string, error) {
	value, err := s.app.backend.RevealSecret(environment, name)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	s.app.logger.Info("a secret was revealed", "Event", "SecretRevealed", "Environment", environment, "Name", name)
	return value, nil
}

// Unset removes the stored value.
func (s *SecretsService) Unset(environment, name string) {
	s.app.actor.Do(func() {
		s.vm.Unset(flowi.SecretInfo{Environment: environment, Name: name}, func(error) {})
	})
}

func (s *SecretsService) view() SecretsView {
	v := SecretsView{Secrets: []SecretView{}, Error: s.vm.Err()}
	for _, r := range s.vm.Rows() {
		if r.Header != "" {
			continue
		}
		state := "missing"
		switch r.State() {
		case vm.SecretStored:
			state = "stored"
		case vm.SecretOverridden:
			state = "overridden"
		}
		v.Secrets = append(v.Secrets, SecretView{Environment: r.Info.Environment, Name: r.Info.Name, State: state})
	}
	return v
}
