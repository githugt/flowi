// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

var service = regexp.MustCompile(`application\.NewService\(app\.(\w+)\(\)\)`)

func servicesIn(t *testing.T, file string) []string {
	t.Helper()
	text, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range service.FindAllStringSubmatch(string(text), -1) {
		out = append(out, m[1])
	}
	slices.Sort(out)
	return out
}

// Before: nothing served the front end without the full app; once something does, it must offer the services the
// app does, or a screen works in the app and fails in the browser.
func TestUIDev_Should_register_the_same_services_as_the_app(t *testing.T) {
	app, dev := servicesIn(t, "../../main.go"), servicesIn(t, "main.go")

	if len(app) == 0 || !slices.Equal(app, dev) {
		t.Fatalf("app registers %v, uidev registers %v", app, dev)
	}
}
