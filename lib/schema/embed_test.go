// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/schema"
)

func TestKinds_should_list_every_schema_but_the_shared_definitions(t *testing.T) {
	// Before: the schemas were files in the repository only; nothing could print or point at them.
	kinds := schema.Kinds()

	if len(kinds) == 0 {
		t.Fatal("no kinds")
	}
	for _, k := range kinds {
		if k == "common" {
			t.Fatal("common is not a kind of file")
		}
		b, ok := schema.Get(k)
		if !ok || !json.Valid(b) {
			t.Errorf("%s: schema missing or not JSON", k)
		}
	}
	if _, ok := schema.Get("request"); !ok {
		t.Error("request has no schema")
	}
}

func TestGet_should_refuse_a_path(t *testing.T) {
	for _, k := range []string{"", "../go.mod", "a/b", "request.schema"} {
		if _, ok := schema.Get(k); ok {
			t.Errorf("Get(%q) answered", k)
		}
	}
}

func TestModeline_should_name_the_schema_of_the_kind(t *testing.T) {
	got := schema.Modeline("flow")

	if !strings.HasPrefix(got, "# yaml-language-server: $schema=") || !strings.HasSuffix(got, "/flow.schema.json\n") {
		t.Fatalf("got %q", got)
	}
	if schema.Modeline("nope") != "" {
		t.Fatal("a kind with no schema has no modeline")
	}
}
