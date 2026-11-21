// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// The states of one cell of the environment comparison.
const (
	CellSet         = "set"          // the environment sets a plain value
	CellInherited   = "inherited"    // the value comes from an environment it builds on
	CellMissing     = "missing"      // the environment has no such variable
	CellSecret      = "secret"       // a secret with a value in the vault
	CellSecretUnset = "secret-unset" // a secret declared but without a value
)

// EnvCell is one variable in one environment.
type EnvCell struct {
	State  string
	Value  string // the text of a plain value; "" for a secret or a missing variable
	Kind   string // how the value is read: "" is text, else number, boolean, object…; "" for a secret or a missing variable
	Source string // for an inherited value: the environment it comes from
}

// EnvCompareRow is one variable across all environments.
type EnvCompareRow struct {
	Name  string
	Cells []EnvCell // in the order of EnvMatrix.Envs
	// Differs: at least two environments have a plain value and the values, or the types they are read as, are not all the same.
	Differs bool
	// Gaps counts the environments where it is missing or a secret has no value.
	Gaps int
}

// EnvMatrix is the variables of every environment side by side.
type EnvMatrix struct {
	Envs []string
	Rows []EnvCompareRow
}

// CompareEnvironments lays the variables of envs side by side: a row for each name any of them has, a cell for each
// environment. An environment that extends another shows the inherited plain variables as such (secrets are not
// inherited). infos says which secrets hold a value.
//
// Before: finding why staging behaved unlike local meant opening each environment in turn and comparing by eye.
func CompareEnvironments(envs []model.Environment, infos []flowi.SecretInfo) EnvMatrix {
	byName := map[string]model.Environment{}
	for _, e := range envs {
		byName[e.Name] = e
	}
	m := EnvMatrix{Envs: make([]string, 0, len(envs))}
	for _, e := range envs {
		m.Envs = append(m.Envs, e.Name)
	}
	var names []string
	seen := map[string]bool{}
	for _, e := range envs {
		for _, v := range e.Vars {
			if !seen[v.Name] {
				seen[v.Name] = true
				names = append(names, v.Name)
			}
		}
		for _, v := range inheritedVars(e, byName) {
			if !seen[v.Name] {
				seen[v.Name] = true
				names = append(names, v.Name)
			}
		}
	}
	slices.Sort(names)
	for _, name := range names {
		row := EnvCompareRow{Name: name}
		var plain []string // the value and its type, so "1" as a number and "1" as text differ
		for _, e := range envs {
			cell := cellOf(e, name, byName, infos)
			row.Cells = append(row.Cells, cell)
			switch cell.State {
			case CellSet, CellInherited:
				plain = append(plain, cell.Kind+"\x00"+cell.Value)
			case CellMissing, CellSecretUnset:
				row.Gaps++
			}
		}
		row.Differs = len(plain) > 1 && slices.ContainsFunc(plain, func(v string) bool { return v != plain[0] })
		m.Rows = append(m.Rows, row)
	}
	return m
}

func cellOf(e model.Environment, name string, byName map[string]model.Environment, infos []flowi.SecretInfo) EnvCell {
	for _, v := range e.Vars {
		if v.Name != name {
			continue
		}
		if v.Secret {
			if slices.ContainsFunc(infos, func(s flowi.SecretInfo) bool {
				return s.Environment == e.Name && s.Name == name && (s.Set || s.Override)
			}) {
				return EnvCell{State: CellSecret}
			}
			return EnvCell{State: CellSecretUnset}
		}
		return EnvCell{State: CellSet, Value: v.Value, Kind: string(v.Kind)}
	}
	for _, v := range inheritedVars(e, byName) {
		if v.Name == name {
			return EnvCell{State: CellInherited, Value: v.Value, Kind: string(v.Kind), Source: v.source}
		}
	}
	return EnvCell{State: CellMissing}
}

type inheritedVar struct {
	model.EnvVar
	source string
}

// inheritedVars are the plain variables e gets from the environments it builds on, nearest first, without those
// it sets itself. A loop in Extends stops where it closes.
func inheritedVars(e model.Environment, byName map[string]model.Environment) []inheritedVar {
	own := map[string]bool{}
	for _, v := range e.Vars {
		own[v.Name] = true
	}
	var out []inheritedVar
	visited := map[string]bool{e.Name: true}
	for base := strings.TrimSpace(e.Extends); base != "" && !visited[base]; {
		visited[base] = true
		b, ok := byName[base]
		if !ok {
			break
		}
		for _, v := range b.Vars {
			if v.Secret || own[v.Name] {
				continue
			}
			own[v.Name] = true
			out = append(out, inheritedVar{EnvVar: v, source: b.Name})
		}
		base = strings.TrimSpace(b.Extends)
	}
	return out
}
