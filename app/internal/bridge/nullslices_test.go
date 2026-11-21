// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// nilCollections walks a value that goes to the page as JSON and lists the paths of slices and maps that are nil.
// A nil slice or map reaches JavaScript as null, and the front end reads .length or iterates it. Fields tagged
// omitempty are absent from the JSON when empty, so they are skipped.
func nilCollections(path string, v reflect.Value, out *[]string, depth int) {
	if depth > 8 {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			nilCollections(path, v.Elem(), out, depth+1)
		}
	case reflect.Slice:
		if v.IsNil() {
			*out = append(*out, path)
			return
		}
		for i := 0; i < v.Len() && i < 3; i++ {
			nilCollections(fmt.Sprintf("%s[%d]", path, i), v.Index(i), out, depth+1)
		}
	case reflect.Map:
		if v.IsNil() {
			*out = append(*out, path)
			return
		}
		n := 0
		for _, k := range v.MapKeys() {
			if n++; n > 3 {
				break
			}
			nilCollections(fmt.Sprintf("%s[%v]", path, k), v.MapIndex(k), out, depth+1)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if !f.IsExported() || tag == "-" || strings.Contains(tag, "omitempty") {
				continue
			}
			nilCollections(path+"."+f.Name, v.Field(i), out, depth+1)
		}
	}
}

// Before: protection from null lists was by memory (81 `?? []` guards in the page), so one view that returned a nil
// slice blanked a tab, and nothing in CI said which.
func TestViews_Given_a_workspace_Then_no_slice_or_map_field_is_nil(t *testing.T) {
	flows, flow, pub := openSignup(t)
	reqs, req, _, _ := openLogin(t)
	_ = flows
	_ = reqs

	app, _ := fixture(t)
	app.Start(t.Context())
	views := map[string]any{
		"FlowView":       flow,
		"RequestView":    req,
		"Tree":           app.Workspace().Tree(),
		"SettingsView":   app.Settings().State(),
		"AuthsView":      app.Auths().State(),
		"SchedulesView":  app.Schedules().State(),
		"TrashView":      app.Trash().State(),
		"BlueprintsView": app.Blueprints().State(),
		"WelcomeView":    app.Welcome().State(),
		"Panels":         app.Panels().State(),
		"ReplayerView":   app.Replayer().State(),
		"ManagerView":    app.Manager().State(),
		"ImportView":     app.Import().State(),
	}
	for name, v := range pub.last {
		views["event:"+name] = v
	}
	var bad []string
	for name, v := range views {
		var found []string
		nilCollections(name, reflect.ValueOf(v), &found, 0)
		bad = append(bad, found...)
	}

	if len(bad) > 0 {
		t.Fatalf("%d nil slice or map fields reach the page as null:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
