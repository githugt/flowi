// SPDX-License-Identifier: Apache-2.0

package version

import (
	"errors"
	"fmt"
	"slices"
)

// Source says which rule chose the resolved version.
type Source int

// Resolution rules, in the order they are tried.
const (
	SourcePinned Source = iota + 1
	SourceRunPreference
	SourceFlowPreference
	SourceDefault
	SourceHighest
)

// Errors returned by Resolve.
var (
	ErrNoVersions     = errors.New("request has no versions")
	ErrPinnedMissing  = errors.New("pinned version does not exist")
	ErrDefaultMissing = errors.New("default version does not exist")
)

// Query is everything that influences which version a request node runs.
// Nil fields are unset.
type Query struct {
	Available  []Label
	Default    *Label
	Pinned     *Label
	RunPrefer  *Label
	FlowPrefer *Label
}

// Resolve picks a version: the node's pin, then the run preference, then
// the flow preference (each only if available), then the request default,
// then the highest version. The returned label is the request's own label,
// so "2.0" is returned when the preference was "2".
func Resolve(q Query) (Label, Source, error) {
	if len(q.Available) == 0 {
		return Label{}, 0, ErrNoVersions
	}
	if q.Pinned != nil {
		l, ok := find(q.Available, *q.Pinned)
		if !ok {
			return Label{}, 0, fmt.Errorf("%w: %s", ErrPinnedMissing, q.Pinned)
		}
		return l, SourcePinned, nil
	}
	if l, ok := findOptional(q.Available, q.RunPrefer); ok {
		return l, SourceRunPreference, nil
	}
	if l, ok := findOptional(q.Available, q.FlowPrefer); ok {
		return l, SourceFlowPreference, nil
	}
	if q.Default != nil {
		l, ok := find(q.Available, *q.Default)
		if !ok {
			return Label{}, 0, fmt.Errorf("%w: %s", ErrDefaultMissing, q.Default)
		}
		return l, SourceDefault, nil
	}
	highest, _ := Highest(q.Available)
	return highest, SourceHighest, nil
}

func find(available []Label, want Label) (Label, bool) {
	i := slices.IndexFunc(available, func(l Label) bool { return l.Compare(want) == 0 })
	if i < 0 {
		return Label{}, false
	}
	return available[i], true
}

func findOptional(available []Label, want *Label) (Label, bool) {
	if want == nil {
		return Label{}, false
	}
	return find(available, *want)
}
