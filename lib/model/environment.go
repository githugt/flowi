// SPDX-License-Identifier: Apache-2.0

package model

import "strings"

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
	// Colour marks the environment wherever it is chosen, so that one is not mistaken for another.
	// It is one of EnvColours(), or empty for the default; an unknown name is shown as the default.
	Colour string
	// Scope is the vault scope of this environment's secrets. Empty means
	// the owning collection; global environments carry the workspace scope.
	Scope ID
}

// EnvColours are the colours an environment can be marked with.
func EnvColours() []string {
	return []string{"red", "orange", "yellow", "lime", "green", "aqua", "blue", "purple", "gray"}
}

// DefaultEnvColour is the colour a new environment of that name starts with: red for a name that
// begins with "prod" (any case), since that is the environment where a slip costs the most.
func DefaultEnvColour(name string) string {
	if strings.HasPrefix(strings.ToLower(name), "prod") {
		return "red"
	}
	return ""
}
