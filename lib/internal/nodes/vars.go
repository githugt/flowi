// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"maps"
	"sync"
)

// Vars is a layered variable store: writes go to the own layer, reads fall
// back to the parent's. It is safe for concurrent use; mu guards own.
type Vars struct {
	mu     sync.RWMutex
	own    map[string]any
	parent *Vars
}

// NewVars returns an empty layer over parent (nil for a root).
func NewVars(parent *Vars) *Vars {
	return &Vars{own: map[string]any{}, parent: parent}
}

// Get returns a variable from this layer or the nearest parent having it.
func (v *Vars) Get(name string) (any, bool) {
	v.mu.RLock()
	value, ok := v.own[name]
	v.mu.RUnlock()
	if ok || v.parent == nil {
		return value, ok
	}
	return v.parent.Get(name)
}

// Set writes name in this layer.
func (v *Vars) Set(name string, value any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.own[name] = value
}

// Snapshot is a merged copy of every layer, nearer layers winning.
func (v *Vars) Snapshot() map[string]any {
	out := map[string]any{}
	if v.parent != nil {
		out = v.parent.Snapshot()
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	maps.Copy(out, v.own)
	return out
}

// Own is a copy of this layer alone (a batch iteration's variables).
func (v *Vars) Own() map[string]any {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return maps.Clone(v.own)
}
