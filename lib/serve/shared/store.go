// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Errors a Store can return.
var (
	// ErrUnavailable means the shared server could not be reached. It is
	// returned at once during the quiet period that follows a failure.
	ErrUnavailable = errors.New("the shared store is unavailable")
	// ErrTooLarge is returned for a value above maxValue.
	ErrTooLarge = errors.New("the value is too large for the shared store")
)

// maxValue bounds one stored value; run records are a few kilobytes.
const maxValue = 1 << 20

// Store is the shared state of serve instances. Implementations are safe
// for concurrent use.
type Store interface {
	// Claim reports whether the caller is the first to claim key; the claim
	// lapses after ttl. It is how one instance out of several takes a
	// schedule tick.
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Put stores value under key for ttl.
	Put(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Get returns the value under key; ok is false when there is none.
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	// Close releases the store's connection.
	Close() error
}

// Memory is a Store in this process's memory: for tests, and for running
// the same code paths without a server.
type Memory struct {
	now func() time.Time

	mu   sync.Mutex // guards data
	data map[string]memEntry
}

type memEntry struct {
	value   []byte
	expires time.Time
}

// NewMemory returns an empty in-memory store; now defaults to time.Now.
func NewMemory(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{now: now, data: map[string]memEntry{}}
}

func (m *Memory) live(key string) (memEntry, bool) {
	e, ok := m.data[key]
	if !ok || !e.expires.After(m.now()) {
		delete(m.data, key)
		return memEntry{}, false
	}
	return e, true
}

// Claim implements Store.
func (m *Memory) Claim(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.live(key); taken {
		return false, nil
	}
	m.data[key] = memEntry{value: []byte("1"), expires: m.now().Add(ttl)}
	return true, nil
}

// Put implements Store.
func (m *Memory) Put(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if len(value) > maxValue {
		return ErrTooLarge
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = memEntry{value: append([]byte(nil), value...), expires: m.now().Add(ttl)}
	return nil
}

// Get implements Store.
func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), e.value...), true, nil
}

// Close implements Store.
func (m *Memory) Close() error { return nil }
