// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// checkShadowing warns when a bare {{name}} in a flow could mean two things: the flow's own input or variable, and
// an environment variable of the same name. The flow's value wins (the engine searches vars, inputs, then env), so
// the environment's is silently hidden.
//
// Before: a flow input called token hid the environment's token and nothing said so.
func checkShadowing(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		env := envNames(c)
		if len(env) == 0 {
			continue
		}
		for _, fl := range c.Flows {
			out = append(out, shadowsInFlow(c, fl, env)...)
		}
	}
	return out
}

// envNames maps each environment variable name of the collection to the environments that set it.
func envNames(c model.Collection) map[string][]string {
	out := map[string][]string{}
	for _, e := range c.Environments {
		for _, v := range e.Vars {
			if !slices.Contains(out[v.Name], e.Name) {
				out[v.Name] = append(out[v.Name], e.Name)
			}
		}
	}
	return out
}

func shadowsInFlow(c model.Collection, fl model.Flow, env map[string][]string) diag.List {
	own := map[string]string{} // name -> "input" or "variable"
	at := map[string]string{}
	for _, in := range fl.Inputs {
		own[in.Name] = "input"
		at[in.Name] = yamlpath.Key("$.inputs", in.Name)
	}
	for name := range sites.WrittenVars(fl.Nodes) {
		if _, isInput := own[name]; !isInput {
			own[name] = "variable"
		}
	}
	bare := map[string]bool{}
	for _, s := range sites.Templates(c) {
		if s.File != fl.File {
			continue
		}
		refs, err := template.References(s.Src, s.Expression)
		if err != nil {
			continue
		}
		for _, r := range refs {
			if len(r.Path) == 1 {
				bare[r.Path[0]] = true
			}
		}
	}
	names := make([]string, 0, len(own))
	for name := range own {
		names = append(names, name)
	}
	slices.Sort(names)
	var out diag.List
	for _, name := range names {
		envs, hides := env[name]
		if !hides || !bare[name] {
			continue
		}
		path := at[name]
		if path == "" {
			path = "$"
		}
		out = append(out, diag.New(diag.CodeTemplateShadow, diag.Location{File: fl.File, Path: path},
			map[string]string{"name": name, "kind": own[name], "envs": strings.Join(envs, ", ")}))
	}
	return out
}
