// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"slices"
)

// Reference is a variable a template or expression reads, as its constant
// steps: {{nodes.login.token}} is ["nodes", "login", "token"] and an index
// step [0] is "0". A step that is not a constant ends the chain. Guarded
// is set when every use of the path is under ?? or ?., so a miss is nil,
// not an error.
type Reference struct {
	Path    []string
	Guarded bool
}

// References lists what src reads, in source order and without
// duplicates (a path used both guarded and unguarded is unguarded). expression says src is a bare expression (an assert check,
// a when) rather than a {{…}} template. Syntax errors are *SyntaxError.
func References(src string, expression bool) ([]Reference, error) {
	chains, err := chainsOf(src, expression)
	if err != nil {
		return nil, err
	}
	var out []Reference
	for _, c := range chains {
		path := make([]string, len(c.path))
		for i, step := range c.path {
			path[i] = fmt.Sprint(step)
		}
		i := slices.IndexFunc(out, func(r Reference) bool { return slices.Equal(r.Path, path) })
		if i < 0 {
			out = append(out, Reference{Path: path, Guarded: c.guarded})
			continue
		}
		out[i].Guarded = out[i].Guarded && c.guarded
	}
	return out, nil
}

func chainsOf(src string, expression bool) ([]chain, error) {
	if expression {
		e, err := compile(src)
		if err != nil {
			return nil, &SyntaxError{Detail: err.Error()}
		}
		return e.chains, nil
	}
	t, err := Parse(src)
	if err != nil {
		return nil, err
	}
	var chains []chain
	for _, s := range t.segments {
		if s.expr != nil {
			chains = append(chains, s.expr.chains...)
		}
	}
	return chains, nil
}
