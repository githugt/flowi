// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	flowi "github.com/githugt/flowi-lib"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func schemasFixture(t *testing.T) (*vm.Schemas, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := sampleBackend()
	b.SchemaStore = vmtest.Schemas{
		{Kind: "response", Status: 404}: {"type": "object"},
		{Kind: "request"}:               {"type": "object", "properties": map[string]any{"email": map[string]any{"type": "string"}}},
		{Kind: "response", Status: 200}: {"type": "object", "x-flowi-source": "openapi"},
	}
	r, w := newRequestVM(t, b)
	return vm.NewSchemas(b, w, r), b, w
}

func TestSchemas_Load_should_list_the_request_body_then_the_responses_with_pretty_JSON(t *testing.T) {
	s, _, w := schemasFixture(t)

	s.Load()
	w.Wait()

	items := s.Items()
	var titles []string
	for _, it := range items {
		titles = append(titles, it.Title)
	}
	if strings.Join(titles, "|") != "Request body|Response 200|Response 404" || !strings.Contains(items[0].Text, "\n  \"properties\"") {
		t.Fatalf("titles %v first text %q", titles, items[0].Text)
	}
	if items[0].Source != flowi.SchemaInferred || s.Any() != true {
		t.Fatalf("source %v any %v", items[0].Source, s.Any())
	}
}

func TestSchemas_Delete_should_remove_the_schema_and_show_the_rest(t *testing.T) {
	s, b, w := schemasFixture(t)
	s.Load()
	w.Wait()
	first := s.Items()[0]

	s.Delete(first)
	w.Wait()

	if !b.Called("DeleteSchema request 0") || len(s.Items()) != 2 {
		t.Fatalf("calls %v items %d", b.Calls, len(s.Items()))
	}
}

func TestSchemas_Load_Given_no_schemas_Then_empty(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)
	s := vm.NewSchemas(b, w, r)

	s.Load()
	w.Wait()

	if len(s.Items()) != 0 || s.Any() {
		t.Fatalf("items %v", s.Items())
	}
}
