// SPDX-License-Identifier: Apache-2.0

package tokens

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	return Open(filepath.Join(t.TempDir(), "sub", "trigger_tokens.json"), c.now), c
}

func TestGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{"users/signup", "users/signup", true},
		{"users/signup", "users/signup2", false},
		{"users/*", "users/signup", true},
		{"users/*", "users/a/b", true},
		{"users/*", "users", false},
		{"*", "anything/at/all", true},
		{"*", "", true},
		{"smoke-*", "smoke-login", true},
		{"smoke-*", "full-smoke-login", false},
		{"*-smoke", "login-smoke", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "aXcYb", false},
		{"ab*ba", "aba", false},
		{"users/*", "01JA00000000000000000000F1", false}, // an id never matches a path pattern
	}
	for _, tt := range tests {
		t.Run(tt.pattern+" "+tt.s, func(t *testing.T) {
			if got := Glob(tt.pattern, tt.s); got != tt.want {
				t.Fatalf("Glob(%q, %q) = %v", tt.pattern, tt.s, got)
			}
		})
	}
}

func TestScopes_Given_no_scope_Then_everything_is_allowed_and_a_scope_limits_flows_and_environments(t *testing.T) {
	open := Scopes{}
	scoped := Scopes{Flows: []string{"smoke/*", "health"}, Envs: []string{"staging"}}

	if !open.Unrestricted() || !open.AllowsFlow("any") || !open.AllowsEnv("") || !open.AllowsEnv("prod") {
		t.Fatal("an empty scope limits nothing")
	}
	if scoped.Unrestricted() || !scoped.AllowsFlow("smoke/login") || !scoped.AllowsFlow("health") || scoped.AllowsFlow("admin/wipe") {
		t.Fatal("flows are limited by pattern")
	}
	// Before: a token could run any flow in any environment.
	if !scoped.AllowsEnv("staging") || scoped.AllowsEnv("prod") || scoped.AllowsEnv("") {
		t.Fatal("a scoped token must name an allowed environment; leaving it out would use the default")
	}
}

func TestStore_New_Then_the_secret_authenticates_and_only_its_hash_is_on_disk(t *testing.T) {
	s, _ := newStore(t)

	tok, secret, err := s.New("ci-smoke", Scopes{Flows: []string{"smoke/*"}}, 0)
	got, result := s.Authenticate(secret)
	data, readErr := os.ReadFile(s.path)

	if err != nil || readErr != nil || result != Valid || got.Name != "ci-smoke" || got.ID != tok.ID {
		t.Fatalf("err %v %v, result %v, got %+v", err, readErr, result, got)
	}
	if !strings.HasPrefix(secret, Prefix) || strings.Contains(string(data), secret) || strings.Contains(string(data), strings.TrimPrefix(secret, Prefix+tok.ID+"_")) {
		t.Fatalf("the secret must not be stored: %s", data)
	}
	if info, _ := os.Stat(s.path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 { // Windows has no owner-only mode bits
		t.Errorf("the file is readable by others: %v", info.Mode())
	}
}

func TestStore_Authenticate_Given_wrong_or_foreign_secrets_Then_unknown(t *testing.T) {
	s, _ := newStore(t)
	tok, secret, _ := s.New("a", Scopes{}, 0)
	tampered := secret[:len(secret)-1] + "0"
	if tampered == secret {
		tampered = secret[:len(secret)-1] + "1"
	}

	for name, presented := range map[string]string{
		"tampered":               tampered,
		"right id, wrong secret": Prefix + tok.ID + "_" + strings.Repeat("0", 64),
		"no prefix":              strings.TrimPrefix(secret, Prefix),
		"unknown id":             Prefix + "ffffffff_" + strings.Repeat("0", 64),
		"empty":                  "",
		"just prefix":            Prefix,
	} {
		t.Run(name, func(t *testing.T) {
			if _, r := s.Authenticate(presented); r != Unknown {
				t.Fatalf("result %v", r)
			}
		})
	}
}

func TestStore_Authenticate_Given_a_lifetime_Then_the_token_expires_and_is_told_apart(t *testing.T) {
	s, clk := newStore(t)
	tok, secret, _ := s.New("short", Scopes{}, time.Hour)

	_, before := s.Authenticate(secret)
	clk.t = clk.t.Add(time.Hour)
	got, after := s.Authenticate(secret)

	if before != Valid || after != Expired || got.Name != "short" || !got.Expired(clk.t) || !tok.Expires.Equal(clk.t) {
		t.Fatalf("before %v after %v token %+v", before, after, got)
	}
}

func TestStore_Rotate_Then_the_old_secret_stops_the_new_one_works_and_the_lifetime_starts_again(t *testing.T) {
	s, clk := newStore(t)
	_, old, _ := s.New("ci", Scopes{Envs: []string{"staging"}}, 24*time.Hour)
	clk.t = clk.t.Add(20 * time.Hour)

	rotated, fresh, err := s.Rotate("ci")
	_, oldResult := s.Authenticate(old)
	got, newResult := s.Authenticate(fresh)

	if err != nil || oldResult != Unknown || newResult != Valid {
		t.Fatalf("err %v, old %v, new %v", err, oldResult, newResult)
	}
	if got.Name != "ci" || len(got.Scopes.Envs) != 1 || !rotated.Expires.Equal(clk.t.Add(24*time.Hour)) {
		t.Fatalf("the name and scopes are kept and the lifetime starts again: %+v", rotated)
	}
	if list, _ := s.List(); len(list) != 1 {
		t.Fatalf("rotating replaces the token: %d", len(list))
	}
}

func TestStore_Revoke_Then_the_token_is_gone_by_name_or_id(t *testing.T) {
	s, _ := newStore(t)
	a, secretA, _ := s.New("a", Scopes{}, 0)
	_, secretB, _ := s.New("b", Scopes{}, 0)

	revoked := s.Revoke(a.ID)
	_, resultA := s.Authenticate(secretA)
	_, resultB := s.Authenticate(secretB)
	missing := s.Revoke("a")

	if revoked != nil || resultA != Unknown || resultB != Valid || !errors.Is(missing, ErrNotFound) {
		t.Fatalf("revoked %v, a %v, b %v, missing %v", revoked, resultA, resultB, missing)
	}
}

func TestStore_New_Given_bad_settings_Then_it_is_refused_and_nothing_is_written(t *testing.T) {
	s, _ := newStore(t)
	if _, _, err := s.New("taken", Scopes{}, 0); err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		name     string
		scopes   Scopes
		lifetime time.Duration
		want     error
	}{
		"empty name":        {"", Scopes{}, 0, ErrInvalid},
		"upper case":        {"CI", Scopes{}, 0, ErrInvalid},
		"space":             {"my token", Scopes{}, 0, ErrInvalid},
		"too long":          {strings.Repeat("a", 41), Scopes{}, 0, ErrInvalid},
		"negative lifetime": {"x", Scopes{}, -time.Hour, ErrInvalid},
		"tiny lifetime":     {"x", Scopes{}, time.Second, ErrInvalid},
		"empty flow":        {"x", Scopes{Flows: []string{" "}}, 0, ErrInvalid},
		"env pattern":       {"x", Scopes{Envs: []string{"stag*"}}, 0, ErrInvalid},
		"duplicate name":    {"taken", Scopes{}, 0, ErrExists},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := s.New(tt.name, tt.scopes, tt.lifetime)

			if !errors.Is(err, tt.want) {
				t.Fatalf("err %v", err)
			}
		})
	}
	if list, _ := s.List(); len(list) != 1 {
		t.Fatalf("%d tokens", len(list))
	}
}

func TestStore_Given_an_unreadable_file_Then_nothing_authenticates_and_List_says_why(t *testing.T) {
	s, _ := newStore(t)
	_, secret, _ := s.New("a", Scopes{}, 0)
	if err := os.WriteFile(s.path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, result := s.Authenticate(secret)
	_, err := s.List()

	if result != Unknown || err == nil {
		t.Fatalf("result %v err %v", result, err)
	}
}
