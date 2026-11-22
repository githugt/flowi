// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
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
	// Differs: at least two environments define it (a plain value or a secret) and what they define is not all the same.
	// A plain value is the same as another only if its text and the type it is read as both match, so 3 as a number and
	// "3" as text differ. A secret differs from any plain value, and two secrets do not differ: a value in the vault is
	// never read here, so whether they hold the same text is not known.
	Differs bool
	// KindsDiffer: the plain values are not all read as the same type, so each cell should say its type.
	KindsDiffer bool
	// Gaps counts the environments where it is missing or a secret has no value.
	Gaps int
}

// EnvMatrix is the variables of every environment side by side.
type EnvMatrix struct {
	Envs []string
	Rows []EnvCompareRow
	// Notes say what the comparison cannot show on its own, such as an environment that builds on a base outside the group.
	Notes []string
}

// CompareEnvironments lays the variables of envs side by side: a row for each name any of them has, a cell for each
// environment. An environment that extends another shows the inherited plain variables as such (secrets are not
// inherited). infos says which secrets hold a value.
//
// Before: finding why staging behaved unlike local meant opening each environment in turn and comparing by eye.
func CompareEnvironments(envs []model.Environment, infos []flowi.SecretInfo) EnvMatrix {
	return compareColumns(envs, envs, infos)
}

// CompareGroup is CompareEnvironments for the environments of one group of all ("" is the default group). A base
// outside the group still supplies the values its environments inherit, so the cells tell the truth, and a note says
// that the base is not a column.
//
// Before: every environment of the workspace was one table, which with several sets of services was too wide to read.
func CompareGroup(all []model.Environment, group string, infos []flowi.SecretInfo) EnvMatrix {
	columns := model.EnvironmentsInGroup(all, group)
	m := compareColumns(columns, all, infos)
	m.Notes = baseNotes(columns, all)
	return m
}

// baseNotes names the environments of columns that build on one outside their group.
func baseNotes(columns, all []model.Environment) []string {
	var notes []string
	for _, e := range columns {
		i := slices.IndexFunc(all, func(x model.Environment) bool { return x.Name == strings.TrimSpace(e.Extends) })
		if e.Extends == "" || i < 0 {
			continue
		}
		if _, missing := missingGroup(e, all[i]); !missing {
			continue
		}
		notes = append(notes, fmt.Sprintf("%s builds on %s, which is in %s: the values it inherits are shown, but %s is not a column here.", e.Name, all[i].Name, groupsLabel(all[i].Memberships()), all[i].Name))
	}
	return notes
}

// GroupLabel is how a screen names a group: the default group has no name of its own.
func GroupLabel(group string) string {
	if group == "" {
		return "the default group"
	}
	return "the group " + group
}

// groupsLabel names the groups of one environment for a screen, as Environment.Memberships lists them ("" is the
// default group).
func groupsLabel(groups []string) string {
	if len(groups) == 1 {
		return GroupLabel(groups[0])
	}
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g
		if g == "" {
			names[i] = "default"
		}
	}
	return "the groups " + strings.Join(names, ", ")
}

// compareColumns builds the matrix for columns; all supplies the bases they inherit from.
func compareColumns(columns, all []model.Environment, infos []flowi.SecretInfo) EnvMatrix {
	byName := map[string]model.Environment{}
	for _, e := range all {
		byName[e.Name] = e
	}
	m := EnvMatrix{Envs: make([]string, 0, len(columns))}
	for _, e := range columns {
		m.Envs = append(m.Envs, e.Name)
	}
	names := variableNames(columns, byName)
	for _, name := range names {
		row := EnvCompareRow{Name: name}
		shapes := map[string]bool{} // what each environment that defines the variable defines, so a repeat counts once
		kinds := map[string]bool{}
		for _, e := range columns {
			cell := cellOf(e, name, byName, infos)
			row.Cells = append(row.Cells, cell)
			switch cell.State {
			case CellSet, CellInherited:
				shapes[cell.Kind+"\x00"+cell.Value] = true
				kinds[cell.Kind] = true
			case CellSecret, CellSecretUnset:
				shapes["secret"] = true
			}
			if cell.State == CellMissing || cell.State == CellSecretUnset {
				row.Gaps++
			}
		}
		row.Differs = len(shapes) > 1
		row.KindsDiffer = len(kinds) > 1
		m.Rows = append(m.Rows, row)
	}
	return m
}

// variableNames lists, sorted, every name one of the columns sets or inherits.
func variableNames(columns []model.Environment, byName map[string]model.Environment) []string {
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, e := range columns {
		for _, v := range e.Vars {
			add(v.Name)
		}
		for _, v := range inheritedVars(e, byName) {
			add(v.Name)
		}
	}
	slices.Sort(names)
	return names
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
