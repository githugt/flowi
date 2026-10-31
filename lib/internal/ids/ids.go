// SPDX-License-Identifier: Apache-2.0

// Package ids generates and checks entity ids (ULIDs).
package ids

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/githugt/flowi/lib/model"
)

// Source generates monotonically increasing ULIDs. It is safe for
// concurrent use.
type Source struct {
	now     func() time.Time
	mu      sync.Mutex // guards entropy: ulid.MonotonicEntropy is not goroutine-safe
	entropy *ulid.MonotonicEntropy
}

// NewSource returns a Source reading time from now and randomness from entropy.
func NewSource(now func() time.Time, entropy io.Reader) *Source {
	return &Source{now: now, entropy: ulid.Monotonic(entropy, 0)}
}

// New returns a fresh id.
func (s *Source) New() model.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.ID(ulid.MustNew(ulid.Timestamp(s.now()), s.entropy).String())
}

// Valid reports whether id is a canonical upper-case ULID.
func Valid(id model.ID) bool {
	s := string(id)
	if s != strings.ToUpper(s) {
		return false
	}
	_, err := ulid.ParseStrict(s)
	return err == nil
}
