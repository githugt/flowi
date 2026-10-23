// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"path"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// Hook levels, outermost first.
const (
	levelCollection = "collection"
	levelFolder     = "folder"
	levelRequest    = "request"
	levelVersion    = "version"
)

// HookRef is one hook step and where it was declared.
type HookRef struct {
	Level string // collection | folder | request | version
	Step  model.HookStep
}

// Label names the step for events and logs: the hook flow's path, or
// "inline".
func (h HookRef) Label() string {
	if h.Step.Flow != nil {
		return h.Step.Flow.Path
	}
	return "inline"
}

// HookChain is every hook step for one request version, in run order:
// Pre outermost first (collection → folders → request → version), Post
// the exact reverse.
type HookChain struct {
	Pre, Post []HookRef
}

// ChainFor collects the hooks that apply to version v of request r in
// collection c. Folder hooks come from every _folder.yaml on the way down
// to the request's folder.
func ChainFor(c model.Collection, r model.Request, v model.RequestVersion) HookChain {
	levels := []levelHooks{{levelCollection, c.Manifest.Hooks}}
	for _, f := range ancestorFolders(c.Folders, r) {
		levels = append(levels, levelHooks{levelFolder, f.Hooks})
	}
	levels = append(levels, levelHooks{levelRequest, r.Hooks}, levelHooks{levelVersion, v.Hooks})

	var chain HookChain
	for _, l := range levels {
		chain.Pre = append(chain.Pre, refs(l.level, l.hooks.Pre)...)
	}
	for _, l := range slices.Backward(levels) {
		chain.Post = append(chain.Post, refs(l.level, l.hooks.Post)...)
	}
	return chain
}

type levelHooks struct {
	level string
	hooks model.Hooks
}

func refs(level string, steps []model.HookStep) []HookRef {
	out := make([]HookRef, len(steps))
	for i, s := range steps {
		out[i] = HookRef{Level: level, Step: s}
	}
	return out
}

// ancestorFolders returns the folders containing the request, outermost
// first. Folder dirs are collection-relative ("requests/auth"); request
// paths are relative to requests/ ("auth/login").
func ancestorFolders(folders []model.Folder, r model.Request) []model.Folder {
	dir := path.Join("requests", path.Dir(r.Path))
	var out []model.Folder
	for _, f := range folders {
		if dir == f.Dir || strings.HasPrefix(dir, f.Dir+"/") {
			out = append(out, f)
		}
	}
	slices.SortFunc(out, func(a, b model.Folder) int { return len(a.Dir) - len(b.Dir) })
	return out
}

// Filter applies a node's or send's hook control: all (default), none, or
// skip pre and/or post.
func (c HookChain) Filter(ctl model.HookControl) HookChain {
	if ctl.Mode == "none" {
		return HookChain{}
	}
	out := c
	if slices.Contains(ctl.Skip, "pre") {
		out.Pre = nil
	}
	if slices.Contains(ctl.Skip, "post") {
		out.Post = nil
	}
	return out
}
