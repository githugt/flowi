// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
	// Protected asks the apps to confirm before a request that changes data is sent with this environment.
	// The CLI does not ask; it runs unattended.
	Protected bool
	// Groups names the sets of environments this one belongs to ("the checkout services", "the data jobs"): one
	// environment can serve several sets. Sorted, without repeats. None means the default group alone; the entry
	// DefaultGroupToken keeps the environment in the default group as well as in the named ones. The apps compare the
	// environments of one group at a time. The `groups:` line goes wherever the environment file goes (git, when it is
	// shared); a group can also exist with no environment in it (see Workspace.EnvironmentGroups).
	Groups []string
	// Extends names the environment this one builds on: its plain variables apply unless this one sets the same name.
	// Secrets are not inherited, because a secret's value lives in the vault under the environment that declares it.
	Extends string
	// Scope is the vault scope of this environment's secrets. Empty means
	// the owning collection; global environments carry the workspace scope.
	Scope ID
	// Shipped marks an environment that comes from a collection's own environments folder and that the workspace has no
	// copy of. The workspace sets it when it loads one; it is never read from a file. A shipped environment has no vault
	// entries, so its secrets come from FLOWI_SECRET_<ENV>_<NAME>.
	Shipped bool
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

// MaxGroupRunes bounds the length of a group name.
const MaxGroupRunes = 40

// DefaultGroupToken is the entry of Environment.Groups that keeps an environment in the default group while it is in
// named groups too. An environment with no entries at all is in the default group alone, so the token only ever stands
// beside a named group. It is the one spelling of "default" a group list accepts; a group may not be named so.
const DefaultGroupToken = "default"

// DefaultGroupLabel is what the apps call the group of an environment that names none. A named group may not use it
// (in any case), so the two can never be mistaken for each other.
const DefaultGroupLabel = "Default"

// CheckGroup says why name cannot be the name of an environment group, or returns nil. The empty name is the
// default group and is valid. A group name is text for people: any letters, digits, spaces and punctuation, but no control
// characters, no space at either end, at most MaxGroupRunes characters, and not "default".
func CheckGroup(name string) error {
	switch {
	case name == "":
		return nil
	case strings.TrimSpace(name) != name:
		return errors.New("a group name cannot start or end with a space")
	case utf8.RuneCountInString(name) > MaxGroupRunes:
		return fmt.Errorf("a group name has at most %d characters", MaxGroupRunes)
	case strings.EqualFold(name, DefaultGroupLabel):
		return fmt.Errorf("%q is the name of the group of environments that have none: choose another name", DefaultGroupLabel)
	case strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }):
		return errors.New("a group name cannot hold control characters")
	}
	return nil
}

// CheckGroups says why groups cannot be the groups of one environment: a name CheckGroup refuses, an empty name
// or a repeat. DefaultGroupToken is allowed (once): it is how an environment is in the default group and in others.
func CheckGroups(groups []string) error {
	for i, g := range groups {
		if g == "" {
			return errors.New("a group name cannot be empty")
		}
		if g != DefaultGroupToken {
			if err := CheckGroup(g); err != nil {
				return err
			}
		}
		if slices.Contains(groups[:i], g) {
			return fmt.Errorf("the group %q is named twice", g)
		}
	}
	return nil
}

// NormaliseGroups returns groups sorted and without repeats or empty names: how an environment file holds them. The
// token alone is the same as no entry (the default group alone), so it is dropped.
func NormaliseGroups(groups []string) []string {
	out := slices.DeleteFunc(slices.Clone(groups), func(g string) bool { return g == "" })
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) == 1 && out[0] == DefaultGroupToken {
		return nil
	}
	return out
}

// InGroup says whether e belongs to group; "" is the default group: the environments with no group, and those that
// hold DefaultGroupToken.
func (e Environment) InGroup(group string) bool {
	if group == "" {
		return len(e.Groups) == 0 || slices.Contains(e.Groups, DefaultGroupToken)
	}
	return group != DefaultGroupToken && slices.Contains(e.Groups, group)
}

// Memberships lists the groups e is in, the default group (as "") first when it is one, then the named groups sorted.
func (e Environment) Memberships() []string {
	var out []string
	if e.InGroup("") {
		out = append(out, "")
	}
	for _, g := range e.Groups {
		if g != DefaultGroupToken {
			out = append(out, g)
		}
	}
	return out
}

// EnvironmentGroups lists the named groups the environments belong to, sorted. The default group is not in the list.
func EnvironmentGroups(envs []Environment) []string {
	var out []string
	for _, e := range envs {
		for _, g := range e.Groups {
			if g != "" && g != DefaultGroupToken && !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	slices.Sort(out)
	return out
}

// EnvironmentsInGroup returns the environments of one group, in their order; "" is the default group.
func EnvironmentsInGroup(envs []Environment, group string) []Environment {
	var out []Environment
	for _, e := range envs {
		if e.InGroup(group) {
			out = append(out, e)
		}
	}
	return out
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
