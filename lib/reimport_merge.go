// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Re-importing a spec used to replace each request file with what the spec
// made of it: checks, extractions, hooks and settings, which a spec has no
// place for, were lost, and so was every saved edit. A request a spec
// import replaces is now merged instead:
//
//   - what only Flowi has (the version's title, checks, extractions,
//     settings, hooks and variables; the request's hooks; versions the spec
//     never had) is kept;
//   - what the spec decides (method, URL, parameters, body, docs) is merged
//     three ways against base, what the previous import of the kept spec
//     made: a field you changed keeps your value, otherwise the spec's
//     new value is taken. When both changed it, yours is kept and
//     FLW-IMPORT-007 says so.
//
// Without a base (the spec was imported before it was kept) the spec's
// fields are taken as they come, as before.

// mergeReimport merges ours, the request on disk, into theirs, what the
// import made of it. base is the previous import's request, or nil.
func mergeReimport(base *model.Request, ours, theirs model.Request) (model.Request, diag.List) {
	out := theirs
	out.Hooks = ours.Hooks
	var diags diag.List
	if base != nil {
		out.Name = pick(base.Name, ours.Name, theirs.Name, eq, conflictAt(&diags, ours.Path, "", "the title"))
		out.Description = pick(base.Description, ours.Description, theirs.Description, eq, conflictAt(&diags, ours.Path, "", "the description"))
	}
	out.Versions = nil
	for _, tv := range theirs.Versions {
		ov, mine := versionByLabel(ours, tv.Label)
		if !mine {
			out.Versions = append(out.Versions, tv)
			continue
		}
		var bv *model.RequestVersion
		if base != nil {
			if b, ok := versionByLabel(*base, tv.Label); ok {
				bv = &b
			}
		}
		out.Versions = append(out.Versions, mergeVersion(bv, ov, tv, conflictAt(&diags, ours.Path, tv.Label, "")))
	}
	out.Versions = append(out.Versions, onlyOurs(base, ours, theirs)...)
	if _, ok := versionByLabel(out, ours.Default); ok {
		out.Default = ours.Default
	}
	return out, diags
}

// onlyOurs are the versions the spec does not make: kept, unless the
// previous import made them (the spec has since dropped them).
func onlyOurs(base *model.Request, ours, theirs model.Request) []model.RequestVersion {
	var out []model.RequestVersion
	for _, ov := range ours.Versions {
		_, inTheirs := versionByLabel(theirs, ov.Label)
		dropped := false
		if base != nil {
			_, dropped = versionByLabel(*base, ov.Label)
		}
		if !inTheirs && !dropped {
			out = append(out, ov)
		}
	}
	return out
}

func mergeVersion(base *model.RequestVersion, ours, theirs model.RequestVersion, conflict func(string)) model.RequestVersion {
	m := theirs
	m.Name, m.Extract, m.Assert, m.Settings, m.Hooks, m.Variables = ours.Name, ours.Extract, ours.Assert, ours.Settings, ours.Hooks, ours.Variables
	if base == nil {
		return m
	}
	m.Method = pick(base.Method, ours.Method, theirs.Method, eq, named(conflict, "the method"))
	m.URL = pick(base.URL, ours.URL, theirs.URL, eq, named(conflict, "the URL"))
	m.Docs = pick(base.Docs, ours.Docs, theirs.Docs, eq, named(conflict, "the docs"))
	m.Body = pick(base.Body, ours.Body, theirs.Body, deepEq, named(conflict, "the body"))
	m.Query = mergePairs(base.Query, ours.Query, theirs.Query, "query parameter", conflict)
	m.Headers = mergePairs(base.Headers, ours.Headers, theirs.Headers, "header", conflict)
	return m
}

// pick is the three-way choice for one field: ours when we changed it,
// theirs otherwise. conflict hears of a field both changed differently.
func pick[T any](base, ours, theirs T, same func(a, b T) bool, conflict func(string)) T {
	if same(ours, base) {
		return theirs
	}
	if !same(theirs, base) && !same(theirs, ours) {
		conflict("")
	}
	return ours
}

// mergePairs merges parameters by name: ours where we changed, removed or
// added one; theirs otherwise, in the spec's order with our additions after.
// kind names them in a conflict ("header").
func mergePairs(base, ours, theirs []model.KeyValue, kind string, conflict func(string)) []model.KeyValue {
	var out []model.KeyValue
	for _, t := range theirs {
		b, inBase := pairValue(base, t.Key)
		o, inOurs := pairValue(ours, t.Key)
		switch {
		case inBase && !inOurs:
			continue // we removed it
		case inOurs:
			v := pick(b, o, t.Value, eq, named(conflict, kind+" "+t.Key))
			if !inBase {
				v = o
			}
			out = append(out, model.KeyValue{Key: t.Key, Value: v})
		default:
			out = append(out, t)
		}
	}
	for _, o := range ours {
		_, inTheirs := pairValue(theirs, o.Key)
		_, inBase := pairValue(base, o.Key)
		if !inTheirs && !inBase {
			out = append(out, o) // we added it
		}
	}
	return out
}

func pairValue(pairs []model.KeyValue, key string) (string, bool) {
	i := slices.IndexFunc(pairs, func(p model.KeyValue) bool { return p.Key == key })
	if i < 0 {
		return "", false
	}
	return pairs[i].Value, true
}

func versionByLabel(r model.Request, label string) (model.RequestVersion, bool) {
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == label })
	if i < 0 {
		return model.RequestVersion{}, false
	}
	return r.Versions[i], true
}

func eq(a, b string) bool { return a == b }

func deepEq(a, b model.Body) bool { return reflect.DeepEqual(a, b) }

// conflictAt reports a field both changed as FLW-IMPORT-007; label is ""
// for the request itself, and field is filled in by named.
func conflictAt(diags *diag.List, path, label, field string) func(string) {
	return func(f string) {
		if f == "" {
			f = field
		}
		what := fmt.Sprintf("%s of request %s", f, path)
		if label != "" {
			what = fmt.Sprintf("%s of request %s version %s", f, path, label)
		}
		*diags = append(*diags, diag.New(diag.CodeImportKeptEdit, diag.Location{}, map[string]string{"what": what}))
	}
}

// named fills in which field a conflict is about.
func named(conflict func(string), field string) func(string) {
	return func(string) { conflict(field) }
}
