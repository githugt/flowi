// SPDX-License-Identifier: Apache-2.0

// Package tokens keeps the bearer tokens that authorise HTTP triggers: each has a name, what it may start (flows,
// environments) and when it expires. Only a hash of a token is stored, so the value exists once, when it is made.
//
// Before: the trigger server had one token with full power, no expiry and no way to rotate it without ending every
// caller at once. A CI job that only needed to run one smoke flow in staging could run any flow in production.
package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Prefix starts every token this package makes, so a token that leaks into a log or a repository is recognised
// (and found by secret scanners) for what it is.
const Prefix = "flwt_"

const (
	idBytes     = 4
	secretBytes = 32
	// MaxTokens bounds the file: a workspace needs a handful, not thousands.
	MaxTokens = 100
)

// Errors a caller can tell apart.
var (
	ErrInvalid  = errors.New("invalid token settings")
	ErrNotFound = errors.New("no such token")
	ErrExists   = errors.New("a token with that name exists")
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// Scopes limit what a token may start. An empty list means no limit of that kind.
type Scopes struct {
	// Flows are patterns for the flow as named in the call's path (users/signup): * matches any run of characters,
	// slashes included, and anything else matches itself. A flow named by id never matches a pattern.
	Flows []string `json:"flows,omitempty"`
	// Envs are the environment names a call may name. A call that names none (it would use the default) is refused
	// when Envs is not empty, so the scope cannot be dodged by leaving it out.
	Envs []string `json:"envs,omitempty"`
}

// Unrestricted says the token may start anything.
func (s Scopes) Unrestricted() bool { return len(s.Flows) == 0 && len(s.Envs) == 0 }

// AllowsFlow says whether a call to flow is within the scope.
func (s Scopes) AllowsFlow(flow string) bool {
	if len(s.Flows) == 0 {
		return true
	}
	return slices.ContainsFunc(s.Flows, func(p string) bool { return Glob(p, flow) })
}

// AllowsEnv says whether a call naming environment env (or none, when "") is within the scope.
func (s Scopes) AllowsEnv(env string) bool {
	return len(s.Envs) == 0 || (env != "" && slices.Contains(s.Envs, env))
}

// Glob reports whether s matches pattern, where * stands for any run of characters (slashes too).
func Glob(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(s, mid)
		if i < 0 {
			return false
		}
		s = s[i+len(mid):]
	}
	return strings.HasSuffix(s, last) && len(s) >= len(last)
}

// Token is what is kept about one token. The secret itself is not.
type Token struct {
	ID      string    `json:"id"`   // 8 hex digits, part of the token, so a call finds its entry without trying them all
	Name    string    `json:"name"` // what the person calls it: "ci-smoke"
	Hash    string    `json:"hash"` // hex SHA-256 of the whole token
	Scopes  Scopes    `json:"scopes"`
	Created time.Time `json:"created"`
	// Lifetime is how long the token lives from Created, 0 for never; Expires is that moment, zero for never. Rotating
	// starts the lifetime again.
	Lifetime time.Duration `json:"lifetime,omitempty"`
	Expires  time.Time     `json:"expires,omitempty"`
}

// Expired says whether the token has run out at now.
func (t Token) Expired(now time.Time) bool { return !t.Expires.IsZero() && !now.Before(t.Expires) }

// Result is how a presented token fared.
type Result int

// The results of Authenticate.
const (
	Unknown Result = iota // not a token of this store
	Valid
	Expired // a token of this store, past its time
)

// Store is the file of tokens. It is safe for concurrent use in one process; the file is replaced atomically, so
// several processes see a whole file, and the last writer wins.
type Store struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

// Open returns the store kept in path. The file need not exist yet. now is the clock; nil uses time.Now.
func Open(path string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{path: path, now: now}
}

type file struct {
	Tokens []Token `json:"tokens"`
}

func (s *Store) read() ([]Token, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the trigger tokens: %w", err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("reading the trigger tokens in %s: %w", s.path, err)
	}
	return f.Tokens, nil
}

func (s *Store) write(list []Token) error {
	data, err := json.MarshalIndent(file{Tokens: list}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the trigger tokens: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("creating the folder of the trigger tokens: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".trigger-tokens-*.tmp")
	if err != nil {
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // a temp file that is gone after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("writing the trigger tokens: %w", err)
	}
	return nil
}

// List returns the tokens, oldest first.
func (s *Store) List() ([]Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Validate checks a name and scopes without storing anything.
func Validate(name string, sc Scopes, lifetime time.Duration) error {
	switch {
	case !namePattern.MatchString(name):
		return fmt.Errorf("%w: a name is 1 to 40 lower-case letters, digits, - or _, starting with a letter or digit, got %q", ErrInvalid, name)
	case lifetime < 0:
		return fmt.Errorf("%w: a lifetime cannot be negative", ErrInvalid)
	case lifetime > 0 && lifetime < time.Minute:
		return fmt.Errorf("%w: a lifetime under a minute is a mistake (%s)", ErrInvalid, lifetime)
	}
	for _, p := range sc.Flows {
		if strings.TrimSpace(p) == "" || strings.ContainsAny(p, " \t\n") {
			return fmt.Errorf("%w: a flow pattern is a path such as users/* without spaces, got %q", ErrInvalid, p)
		}
	}
	for _, e := range sc.Envs {
		if strings.TrimSpace(e) == "" || strings.Contains(e, "*") {
			return fmt.Errorf("%w: an environment scope is a name, not a pattern, got %q", ErrInvalid, e)
		}
	}
	return nil
}

// New makes a token called name. It returns the secret, which is not stored and cannot be shown again.
func (s *Store) New(name string, sc Scopes, lifetime time.Duration) (Token, string, error) {
	if err := Validate(name, sc, lifetime); err != nil {
		return Token{}, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return Token{}, "", err
	}
	if len(list) >= MaxTokens {
		return Token{}, "", fmt.Errorf("%w: there are already %d tokens; remove some", ErrInvalid, MaxTokens)
	}
	if slices.ContainsFunc(list, func(t Token) bool { return t.Name == name }) {
		return Token{}, "", fmt.Errorf("%w: %q", ErrExists, name)
	}
	t, secret, err := s.mint(name, sc, lifetime, list)
	if err != nil {
		return Token{}, "", err
	}
	if err := s.write(append(list, t)); err != nil {
		return Token{}, "", err
	}
	return t, secret, nil
}

// mint makes a token whose id is not taken by the tokens in list.
func (s *Store) mint(name string, sc Scopes, lifetime time.Duration, list []Token) (Token, string, error) {
	for range 8 {
		raw := make([]byte, idBytes+secretBytes)
		if _, err := rand.Read(raw); err != nil {
			return Token{}, "", fmt.Errorf("generating a token: %w", err)
		}
		id := hex.EncodeToString(raw[:idBytes])
		if slices.ContainsFunc(list, func(t Token) bool { return t.ID == id }) {
			continue
		}
		secret := Prefix + id + "_" + hex.EncodeToString(raw[idBytes:])
		now := s.now().UTC().Truncate(time.Second)
		t := Token{ID: id, Name: name, Hash: hash(secret), Scopes: sc, Created: now, Lifetime: lifetime}
		if lifetime > 0 {
			t.Expires = now.Add(lifetime)
		}
		return t, secret, nil
	}
	return Token{}, "", errors.New("generating a token: no free id")
}

// Rotate replaces the secret of the token ref (a name or an id), keeping its name and scopes and starting its
// lifetime again. The old secret stops working at once. It returns the new secret.
func (s *Store) Rotate(ref string) (Token, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return Token{}, "", err
	}
	i := find(list, ref)
	if i < 0 {
		return Token{}, "", fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	old := list[i]
	rest := slices.Delete(slices.Clone(list), i, i+1)
	t, secret, err := s.mint(old.Name, old.Scopes, old.Lifetime, rest)
	if err != nil {
		return Token{}, "", err
	}
	list[i] = t
	if err := s.write(list); err != nil {
		return Token{}, "", err
	}
	return t, secret, nil
}

// Revoke removes the token ref (a name or an id); it stops working at once.
func (s *Store) Revoke(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	i := find(list, ref)
	if i < 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	return s.write(slices.Delete(list, i, i+1))
}

func find(list []Token, ref string) int {
	return slices.IndexFunc(list, func(t Token) bool { return t.Name == ref || t.ID == ref })
}

func hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Authenticate looks a presented secret up. The comparison takes the same time whatever the secret is. An expired
// token is told apart from an unknown one so the caller can be told to ask for a new one.
func (s *Store) Authenticate(secret string) (Token, Result) {
	id, ok := idOf(secret)
	if !ok {
		return Token{}, Unknown
	}
	s.mu.Lock()
	list, err := s.read()
	s.mu.Unlock()
	if err != nil {
		return Token{}, Unknown // an unreadable file authorises nothing
	}
	i := slices.IndexFunc(list, func(t Token) bool { return t.ID == id })
	if i < 0 {
		return Token{}, Unknown
	}
	want, err := hex.DecodeString(list[i].Hash)
	got := sha256.Sum256([]byte(secret))
	if err != nil || subtle.ConstantTimeCompare(got[:], want) != 1 {
		return Token{}, Unknown
	}
	if list[i].Expired(s.now()) {
		return list[i], Expired
	}
	return list[i], Valid
}

// idOf is the id inside "flwt_<id>_<secret>".
func idOf(secret string) (string, bool) {
	rest, ok := strings.CutPrefix(secret, Prefix)
	if !ok {
		return "", false
	}
	id, _, ok := strings.Cut(rest, "_")
	return id, ok && len(id) == idBytes*2
}
