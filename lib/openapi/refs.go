// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"slices"
	"strings"
)

// Inlining limits. Acyclic references can still fan out exponentially
// (every schema referring to every other), so expansion stops at a nesting
// depth and after a number of expansions per resolved value.
const (
	maxRefDepth   = 6
	maxExpansions = 500
)

// resolver inlines local $refs ("#/components/schemas/Pet"). A reference
// already being expanded (a cycle), or one past the limits, becomes {} —
// "anything", which contract checks treat as unknown rather than broken.
type resolver struct {
	root     map[string]any
	external []string // external references met (not followed)
}

// resolve inlines v's references with a fresh expansion budget.
func (r *resolver) resolve(v any) any {
	budget := maxExpansions
	return r.inline(v, nil, &budget)
}

func (r *resolver) inline(v any, stack []string, budget *int) any {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok {
			return r.follow(ref, stack, budget)
		}
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = r.inline(item, stack, budget)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = r.inline(item, stack, budget)
		}
		return out
	}
	return v
}

func (r *resolver) follow(ref string, stack []string, budget *int) any {
	if !strings.HasPrefix(ref, "#/") {
		if !slices.Contains(r.external, ref) {
			r.external = append(r.external, ref)
		}
		return map[string]any{}
	}
	if slices.Contains(stack, ref) || len(stack) >= maxRefDepth || *budget <= 0 {
		return map[string]any{} // a cycle, or past the limits: cut here
	}
	target, ok := r.pointer(ref)
	if !ok {
		return map[string]any{}
	}
	*budget--
	return r.inline(target, append(slices.Clone(stack), ref), budget)
}

// pointer looks up a local JSON pointer ("#/components/schemas/Pet").
func (r *resolver) pointer(ref string) (any, bool) {
	var cur any = r.root
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}
