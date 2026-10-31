// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"strconv"

	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

const (
	phasePre  = "pre"
	phasePost = "post"
)

// hookScope is one place hooks are attached (design §4.5): the
// collection, a folder, a request or one of its versions.
type hookScope struct {
	label   string // "collection identity", "folder requests/auth", "request auth/login version 2"
	file    string
	path    string // YAML path of the mapping holding "hooks"
	hooks   model.Hooks
	applies func(model.Request, model.RequestVersion) bool
}

func scopesOf(c model.Collection) []hookScope {
	always := func(model.Request, model.RequestVersion) bool { return true }
	out := []hookScope{{label: "collection " + c.Manifest.Name, file: c.Manifest.File, path: "$", hooks: c.Manifest.Hooks, applies: always}}
	for _, f := range c.Folders {
		out = append(out, hookScope{label: "folder " + f.Dir, file: f.File, path: "$", hooks: f.Hooks,
			applies: func(r model.Request, _ model.RequestVersion) bool {
				return slices.ContainsFunc(sites.AncestorFolders(c, r), func(a model.Folder) bool { return a.Dir == f.Dir })
			}})
	}
	for _, r := range c.Requests {
		out = append(out, hookScope{label: "request " + r.Path, file: r.File, path: "$", hooks: r.Hooks,
			applies: func(other model.Request, _ model.RequestVersion) bool { return other.ID == r.ID }})
		for _, v := range r.Versions {
			out = append(out, hookScope{label: "request " + r.Path + " version " + v.Label, file: r.File,
				path: yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), hooks: v.Hooks,
				applies: func(other model.Request, ov model.RequestVersion) bool {
					return other.ID == r.ID && ov.Label == v.Label
				}})
		}
	}
	return out
}

func (s hookScope) steps(phase string) []model.HookStep {
	if phase == phasePre {
		return s.hooks.Pre
	}
	return s.hooks.Post
}

func (s hookScope) stepPath(phase string, i int) string {
	return yamlpath.Index(yamlpath.Key(yamlpath.Key(s.path, "hooks"), phase), i)
}

// stepLabel names a hook step, e.g. "post · collection identity · inline #0"
// or "post · request auth/login · flow hooks/check-login".
func (s hookScope) stepLabel(phase string, i int, step model.HookStep) string {
	what := "inline #" + strconv.Itoa(i)
	if step.Flow != nil {
		what = "flow " + step.Flow.Path
	}
	return phase + " · " + s.label + " · " + what
}

// inlineRoots are the scope's inline hook steps as roots, pre and post.
func (s hookScope) inlineRoots(c model.Collection) []root {
	var out []root
	for _, phase := range []string{phasePre, phasePost} {
		for i, step := range s.steps(phase) {
			if len(step.Inline) > 0 {
				out = append(out, root{c: c, file: s.file, path: yamlpath.Key(s.stepPath(phase, i), "inline"),
					nodes: step.Inline, hook: s.stepLabel(phase, i, step)})
			}
		}
	}
	return out
}

// stepRoot is the node list a hook step runs: its inline nodes, or the
// hook flow's nodes. It fails only for a hook flow that cannot be found.
func (s hookScope) stepRoot(ix *index, c model.Collection, phase string, i int, step model.HookStep) (root, bool) {
	if step.Flow == nil {
		return root{c: c, file: s.file, path: yamlpath.Key(s.stepPath(phase, i), "inline"), nodes: step.Inline,
			hook: s.stepLabel(phase, i, step)}, true
	}
	fc, f, ok := ix.flow(c, *step.Flow)
	if !ok {
		return root{}, false
	}
	return root{c: fc, file: f.File, path: yamlpath.Key("$", "nodes"), nodes: f.Nodes, flow: &f}, true
}

// preHookVars are the variables r's pre-hooks for version v write: the
// request's templates can read them.
func (ix *index) preHookVars(c model.Collection, r model.Request, v model.RequestVersion) map[string]bool {
	key := versionKey{c.Manifest.ID, r.ID, v.Label}
	if vars, ok := ix.varsMemo[key]; ok {
		return vars
	}
	out := map[string]bool{}
	for _, s := range ix.scopes[c.Manifest.ID] {
		if !s.applies(r, v) {
			continue
		}
		for i, step := range s.hooks.Pre {
			if rt, ok := s.stepRoot(ix, c, phasePre, i, step); ok {
				for name := range sites.WrittenVars(rt.nodes) {
					out[name] = true
				}
			}
		}
	}
	ix.varsMemo[key] = out
	return out
}
