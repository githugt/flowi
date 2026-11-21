// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"sync/atomic"

	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/vm"
)

// SecretView is a secret declared by an environment. Values are not part of it:
// the window asks for one at a time with Reveal.
type SecretView struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	State       string `json:"state"` // missing, stored or overridden
	// UsedBy are the requests, flows and authentications that write {{secret.name}}: what a removed value would break.
	UsedBy []SecretUseView `json:"usedBy"`
}

// SecretUseView is one file that reads a secret.
type SecretUseView struct {
	Kind  string `json:"kind"`  // request, flow or auth
	Label string `json:"label"` // its path or name
	Node  string `json:"node"`  // the tree node that opens it; empty for an authentication
}

// secretUseViews are the files that read a secret, as the window lists them. A request or flow opens by its tree node.
func secretUseViews(uses []flowi.SecretUse) []SecretUseView {
	out := make([]SecretUseView, 0, len(uses))
	for _, u := range uses {
		v := SecretUseView{Kind: u.Kind, Label: u.Name}
		switch u.Kind {
		case flowi.UseRequest:
			v.Node = "r:" + string(u.Collection) + ":" + string(u.ID)
		case flowi.UseFlow:
			v.Node = "f:" + string(u.Collection) + ":" + string(u.ID)
		}
		out = append(out, v)
	}
	return out
}

// VaultView is where the secrets vault stands, for the lock chip.
type VaultView struct {
	Exists   bool `json:"exists"`   // a vault file exists
	Unlocked bool `json:"unlocked"` // its key is in memory
	// Prompts says that opening it asks for the passphrase; otherwise it opens again by itself after a lock.
	Prompts bool `json:"prompts"`
	// AutoLockSeconds is the vault.auto_lock setting, 0 for never; LocksInSeconds the time left, 0 when locked or off.
	AutoLockSeconds int `json:"autoLockSeconds"`
	LocksInSeconds  int `json:"locksInSeconds"`
}

// SecretsView is the secrets explorer.
type SecretsView struct {
	Vault   VaultView    `json:"vault"`
	Secrets []SecretView `json:"secrets"`
	Error   string       `json:"error,omitempty"`
	// Reveals counts the values shown in this session: "Revealed 3 times".
	Reveals int `json:"reveals"`
}

// SecretsService is the secrets explorer: set or clear values in the vault.
type SecretsService struct {
	reveals atomic.Int64
	app     *App
	vm      *vm.Secrets
	first   *firstLoad
	uses    map[string][]flowi.SecretUse // by secret name, read in State off the UI goroutine
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
	uses := s.app.backend.SecretUses() // reads the collections' text: off the actor
	s.app.actor.Do(func() { s.uses = uses; s.vm.Load() })
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Set stores a value in the vault. It is never read back. A refusal comes back as the error, so the window can say so
// and keep what was typed. Before: the outcome was dropped, and the screen said "Stored" whatever the vault did.
func (s *SecretsService) Set(environment, name, value string) error {
	return s.settle(func(done func(error)) {
		s.vm.Set(flowi.SecretInfo{Environment: environment, Name: name}, value, done)
	})
}

// settle runs start on the actor and waits for the outcome it reports, as an error with a message for people.
func (s *SecretsService) settle(start func(done func(error))) error {
	out := make(chan error, 1)
	s.app.actor.Do(func() {
		start(func(err error) {
			if err != nil {
				err = errors.New(vm.ErrorText(err))
			}
			out <- err
		})
	})
	return <-out
}

// Declare makes name a secret of the environment, declaring it in the environment's file when it is not one yet,
// and stores value for it. It answers "" when done, or what went wrong: the request editor moves a password out of
// a connection string with it.
func (s *SecretsService) Declare(environment, name, value string) string {
	out := make(chan string, 1)
	s.app.actor.Do(func() {
		s.vm.Declare(flowi.SecretInfo{Environment: environment, Name: name}, value, func(err error) {
			if err != nil {
				out <- vm.ErrorText(err)
				return
			}
			out <- ""
		})
	})
	return <-out
}

// Reveal returns one stored value for the window to show for a moment; it is the only call that sends a
// value to the window, and it sends one. The window asks the passphrase first (PassphraseService.Unlock),
// so this never waits for the user. The reveal is logged, without the value.
func (s *SecretsService) Reveal(environment, name string) (string, error) {
	value, err := s.app.backend.RevealSecret(environment, name)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	s.reveals.Add(1)
	s.app.logger.Info("a secret was revealed", "Event", "SecretRevealed", "Environment", environment, "Name", name)
	return value, nil
}

// Unset removes the stored value; a failure comes back as the error. Before: it was hidden, and the row looked unchanged
// with no word why.
func (s *SecretsService) Unset(environment, name string) error {
	return s.settle(func(done func(error)) {
		s.vm.Unset(flowi.SecretInfo{Environment: environment, Name: name}, done)
	})
}

// LockVault forgets the vault's key now. The window shows the new state from the "secrets" event.
func (s *SecretsService) LockVault() {
	s.app.actor.Do(func() { s.vm.Lock() })
}

// RefreshVault reads the vault's state again (the countdown to its auto-lock) without opening it.
func (s *SecretsService) RefreshVault() {
	s.app.actor.Do(func() { s.vm.Refresh() })
}

func vaultView(st flowi.VaultState) VaultView {
	return VaultView{Exists: st.Exists, Unlocked: st.Unlocked, Prompts: st.Prompts, AutoLockSeconds: int(st.AutoLock.Seconds()), LocksInSeconds: int(st.LocksIn.Seconds())}
}

func (s *SecretsService) view() SecretsView {
	v := SecretsView{Vault: vaultView(s.vm.Vault()), Secrets: []SecretView{}, Error: s.vm.Err(), Reveals: int(s.reveals.Load())}
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
		v.Secrets = append(v.Secrets, SecretView{Environment: r.Info.Environment, Name: r.Info.Name, State: state, UsedBy: secretUseViews(s.uses[r.Info.Name])})
	}
	return v
}
