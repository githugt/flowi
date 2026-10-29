// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// EnvVar is one environment variable. When Secret is set, Value is empty:
// the value lives in the workspace vault.
type EnvVar struct {
	Name   string
	Value  string // the text form; for VarJSON, JSON text
	Secret bool
	// Kind says what Value is read as. "" is text. A number, a boolean or an object keeps its type when
	// the variable is used on its own, so {"n": {{env.count}}} sends a number and not "5".
	Kind VarKind
}

// VarKind is the type of a variable's value.
type VarKind string

// Variable kinds.
const (
	VarString  VarKind = ""
	VarNumber  VarKind = "number"
	VarBoolean VarKind = "boolean"
	VarJSON    VarKind = "json" // an object or a list, held as JSON text
)

// Typed returns the value as its kind says: text, an int64 or float64, a bool, or the decoded JSON. A value
// that does not fit its kind is an error naming the variable.
func (v EnvVar) Typed() (any, error) {
	switch v.Kind {
	case VarString:
		return v.Value, nil
	case VarNumber:
		if n, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
			return n, nil
		}
		f, err := strconv.ParseFloat(v.Value, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("variable %s: %q is not a number", v.Name, v.Value)
		}
		return f, nil
	case VarBoolean:
		b, err := strconv.ParseBool(v.Value)
		if err != nil || (v.Value != "true" && v.Value != "false") {
			return nil, fmt.Errorf("variable %s: %q is not true or false", v.Name, v.Value)
		}
		return b, nil
	case VarJSON:
		var out any
		if err := json.Unmarshal([]byte(v.Value), &out); err != nil {
			return nil, fmt.Errorf("variable %s: not valid JSON: %w", v.Name, err)
		}
		return out, nil
	}
	return nil, fmt.Errorf("variable %s: unknown type %q", v.Name, v.Kind)
}

// Environment is a named set of variables. Name defaults to the file stem.
type Environment struct {
	Meta
	Vars []EnvVar
	File string
	// Colour marks the environment wherever it is chosen, so that one is not mistaken for another.
	// It is one of EnvColours(), or empty for the default; an unknown name is shown as the default.
	Colour string
	// Extends names the environment this one builds on: its plain variables apply unless this one sets the same name.
	// Secrets are not inherited, because a secret's value lives in the vault under the environment that declares it.
	Extends string
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

// Errors EnvironmentChain wraps, so a caller can tell a missing base from a loop.
var (
	ErrEnvironmentBase = errors.New("base environment not found")
	ErrEnvironmentLoop = errors.New("environments extend each other in a loop")
)

// EnvironmentChain lists the environment called name and the ones it extends, base first. A base that
// is not in envs, or a loop, is an error that names the environment at fault.
func EnvironmentChain(envs []Environment, name string) ([]Environment, error) {
	var chain []Environment
	seen := map[string]bool{}
	for next := name; next != ""; {
		if seen[next] {
			return nil, fmt.Errorf("%w: %q", ErrEnvironmentLoop, next)
		}
		seen[next] = true
		e, ok := findEnvironment(envs, next)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrEnvironmentBase, next)
		}
		chain = append([]Environment{e}, chain...)
		next = e.Extends
	}
	return chain, nil
}

func findEnvironment(envs []Environment, name string) (Environment, bool) {
	for _, e := range envs {
		if e.Name == name {
			return e, true
		}
	}
	return Environment{}, false
}
