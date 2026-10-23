// SPDX-License-Identifier: Apache-2.0

// Package validate checks loaded collections for structural problems that
// a single file cannot reveal: duplicate ids, broken references, unknown
// node types, dependency cycles, and suspicious characters in values.
//
// Each check is a small function over a shared index; Validate runs them
// in a fixed order. Positions are filled in later by the store, from the
// YAML paths recorded here.
package validate

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Target is everything validated together: all loaded collections, so
// that cross-collection references can be checked.
type Target struct {
	Collections []model.Collection
}

type check func(ix *index) diag.List

func checks() []check {
	return []check{
		checkDuplicateIDs,
		checkDefaultVersions,
		checkRefs,
		checkNodes,
		checkParams,
		checkExecution,
		checkGotos,
		checkAttach,
		checkConcurrentWrites,
		checkSubflowInputs,
		checkReferences,
		checkNeedsCycles,
		checkSubflowCycles,
		checkHookCycles,
		checkTemplates,
		checkSetTargets,
		checkLint,
		checkHeaderNames,
		checkAsync,
		checkBlueprints,
	}
}

// Validate runs every check and returns the diagnostics sorted.
func Validate(t Target) diag.List {
	ix := buildIndex(t.Collections)
	var out diag.List
	for _, run := range checks() {
		out = append(out, run(ix)...)
	}
	return out.Sorted()
}
