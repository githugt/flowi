// SPDX-License-Identifier: Apache-2.0

// Package modeldiff lists where two versions of a saved thing (a request, a flow) differ, by field path, for the
// banner that says a file changed on disk under unsaved edits.
//
// Before: the banner offered "Load from disk" or "Keep mine" and nothing to decide with, so either choice could
// throw away someone's work unseen.
//
// The comparison is of the decoded model, not of the file text, so a comment or a changed order of keys is not a
// difference. A value in a field named like a credential, or the value of a key/value pair whose name is one
// (Authorization, api_key…), is not shown: it reads (redacted) and the number of them is reported.
package modeldiff

import (
	"encoding/json"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/respdiff"
)

// Change is one difference: Path is a field path ($.Versions[0].URL), and Disk and Mine are the values as short
// text, "" for the side that has none.
type Change struct {
	Path string
	Kind respdiff.Kind
	Disk string
	Mine string
}

// Result is the comparison of what is on disk with what is being edited.
type Result struct {
	Changes []Change
	// More counts changes found beyond the limit and not listed.
	More int
	// Hidden counts credential values left out of the comparison.
	Hidden int
}

// Same reports that nothing differs.
func (r Result) Same() bool { return len(r.Changes) == 0 && r.More == 0 }

// maxChanges bounds the list: a rewritten flow is not a list of hundreds of lines.
const maxChanges = 100

// Compare compares disk with mine. Fields named in ignore (top-level, by Go name: the slot of an autosaved edit)
// are left out.
func Compare(disk, mine any, ignore ...string) Result {
	a, b := decode(disk, ignore), decode(mine, ignore)
	a, ha := redact(a)
	b, hb := redact(b)
	list, more := respdiff.Values(a, b, maxChanges)
	r := Result{More: more, Hidden: max(ha, hb)}
	for _, c := range list {
		r.Changes = append(r.Changes, Change{Path: c.Path, Kind: c.Kind, Disk: c.Old, Mine: c.New})
	}
	return r
}

func decode(v any, ignore []string) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	if m, ok := out.(map[string]any); ok {
		for _, k := range ignore {
			delete(m, k)
		}
	}
	return out
}

// redact replaces credential values, and counts them. Both sides are redacted the same way, so a credential that
// changed is not a difference and is not shown.
func redact(v any) (any, int) {
	n := 0
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for _, label := range []string{"Key", "Name"} { // a header, query parameter or variable: Key; a field: Name
				if name, ok := x[label].(string); ok && importers.SensitiveHeader(name) {
					if val, ok := x["Value"].(string); ok && val != "" && !importers.HasVar(val) {
						x["Value"] = importers.Redacted
						n++
					}
				}
			}
			for k, e := range x {
				// A missing list and an empty one are the same thing in a file.
				if l, isList := e.([]any); e == nil || isList && len(l) == 0 {
					delete(x, k)
					continue
				}
				if k == "Key" || k == "Name" {
					continue // the name of an entry is not its secret
				}
				if s, ok := e.(string); ok && s != "" && importers.SensitiveHeader(k) && !importers.HasVar(s) && s != importers.Redacted {
					x[k] = importers.Redacted
					n++
					continue
				}
				x[k] = walk(e)
			}
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		}
		return v
	}
	return walk(v), n
}
