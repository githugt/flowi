// SPDX-License-Identifier: Apache-2.0

package model

// EnvVar is one environment variable. When Secret is set, Value is empty:
// the value lives in the workspace vault.
type EnvVar struct {
	Name   string
	Value  string
	Secret bool
}

// Environment is a named set of variables. Name defaults to the file stem.
type Environment struct {
	Meta
	Vars []EnvVar
	File string
	// Scope is the vault scope of this environment's secrets. Empty means
	// the owning collection; global environments carry the workspace scope.
	Scope ID
}
