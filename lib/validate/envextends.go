// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// checkEnvironmentExtends reports an environment whose `extends:` names a missing base or leads back to
// itself, whose `group:` cannot be used, or that builds on a base in another group. Global environments appear in
// every collection, so each file is checked once.
func checkEnvironmentExtends(ix *index) diag.List {
	var out diag.List
	checked := map[string]bool{}
	for _, c := range ix.collections {
		for _, e := range c.Environments {
			if checked[e.File] {
				continue
			}
			checked[e.File] = true
			out = append(out, checkEnvironmentGroup(e)...)
			if e.Extends == "" {
				continue
			}
			if _, err := model.EnvironmentChain(c.Environments, e.Name); err != nil {
				loc := diag.Location{File: e.File, Path: "$.extends"}
				out = append(out, diag.New(diag.CodeRequestEnvExtends, loc, map[string]string{"env": e.Name, "detail": err.Error()}))
				continue
			}
			out = append(out, checkBaseGroup(e, c.Environments)...)
		}
	}
	return out
}

// checkEnvironmentGroup reports the groups of an environment that cannot be used: a name model.CheckGroup refuses, an
// empty one or a repeat.
func checkEnvironmentGroup(e model.Environment) diag.List {
	err := model.CheckGroups(e.Groups)
	if err == nil {
		return nil
	}
	loc := diag.Location{File: e.File, Path: "$.groups"}
	return diag.List{diag.New(diag.CodeRequestEnvGroup, loc, map[string]string{"env": e.Name, "detail": err.Error()})}
}

// checkBaseGroup warns when the environment's direct base is missing from a group the environment is in: Compare
// environments shows one group at a time, so the values e inherits would come from an environment the comparison does
// not list. Only the direct base is checked; each environment of a chain is checked on its own file.
func checkBaseGroup(e model.Environment, envs []model.Environment) diag.List {
	base, ok := findEnvironment(envs, e.Extends)
	if !ok {
		return nil
	}
	for _, g := range e.Memberships() {
		if !base.InGroup(g) {
			loc := diag.Location{File: e.File, Path: "$.extends"}
			return diag.List{diag.New(diag.CodeRequestEnvGroupBase, loc, map[string]string{
				"env": e.Name, "base": base.Name, "group": groupText(g), "baseGroup": groupsText(base.Memberships()),
			})}
		}
	}
	return nil
}

// groupText is how a message names a group.
func groupText(group string) string {
	if group == "" {
		return "the default group"
	}
	return `the group "` + group + `"`
}

// groupsText names groups as Environment.Memberships lists them; "" is the default group.
func groupsText(groups []string) string {
	if len(groups) == 1 {
		return groupText(groups[0])
	}
	quoted := make([]string, len(groups))
	for i, g := range groups {
		quoted[i] = `"` + g + `"`
		if g == "" {
			quoted[i] = "the default group"
		}
	}
	return "the groups " + strings.Join(quoted, ", ")
}

func findEnvironment(envs []model.Environment, name string) (model.Environment, bool) {
	for _, e := range envs {
		if e.Name == name {
			return e, true
		}
	}
	return model.Environment{}, false
}
