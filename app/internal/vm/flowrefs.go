// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// stepRef matches a template's reference to step id: nodes.id (then a dot, a bracket or the end of the name) or
// nodes["id"].
func stepRef(id string) *regexp.Regexp {
	q := regexp.QuoteMeta(id)
	return regexp.MustCompile(`\bnodes(?:\.` + q + `\b|\[\s*["']` + q + `["']\s*\])`)
}

// mapStrings calls fn on every string reachable from v (struct fields, slices, maps, pointers and interfaces) and
// stores what it returns.
func mapStrings(v reflect.Value, fn func(string) string) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			inner := v.Elem()
			if v.Kind() == reflect.Interface && inner.Kind() == reflect.String && v.CanSet() {
				v.Set(reflect.ValueOf(fn(inner.String())).Convert(inner.Type()))
				return
			}
			mapStrings(inner, fn)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				mapStrings(v.Field(i), fn)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			mapStrings(v.Index(i), fn)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			elem := v.MapIndex(k)
			if elem.Kind() == reflect.Interface && !elem.IsNil() && elem.Elem().Kind() == reflect.String {
				v.SetMapIndex(k, reflect.ValueOf(fn(elem.Elem().String())))
				continue
			}
			if elem.Kind() == reflect.String {
				v.SetMapIndex(k, reflect.ValueOf(fn(elem.String())).Convert(elem.Type()))
				continue
			}
			mapStrings(elem, fn)
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString(fn(v.String()))
		}
	}
}

// renameStepRefs rewrites the templates and tests of the flow that read step from's result, so they read to's.
//
// Before: renaming a step changed its id and its needs, and left {{nodes.old.status}} and the checks that used it
// pointing at nothing.
func renameStepRefs(f *model.Flow, from, to string) {
	re := stepRef(from)
	mapStrings(reflect.ValueOf(f).Elem(), func(s string) string {
		if !strings.Contains(s, from) {
			return s
		}
		return re.ReplaceAllStringFunc(s, func(m string) string {
			if strings.HasPrefix(m, "nodes.") {
				return "nodes." + to
			}
			return `nodes["` + to + `"]`
		})
	})
}

// stepsReading lists the steps (by id, in flow order) outside the given set whose templates or tests read the
// results of step id.
func stepsReading(f model.Flow, id string, inside map[string]bool) []string {
	re := stepRef(id)
	var out []string
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for i := range list {
			n := list[i]
			if !inside[n.ID] {
				found := false
				probe := n
				// Look at this step's own fields only: the steps inside it are visited on their own.
				probe.Body, probe.Then, probe.Else, probe.Branches = nil, nil, nil, nil
				probe.Cases = append([]model.Case(nil), n.Cases...)
				for c := range probe.Cases {
					probe.Cases[c].Nodes = nil
				}
				mapStrings(reflect.ValueOf(&probe).Elem(), func(s string) string {
					if re.MatchString(s) {
						found = true
					}
					return s
				})
				if found {
					out = append(out, n.ID)
				}
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return out
}
