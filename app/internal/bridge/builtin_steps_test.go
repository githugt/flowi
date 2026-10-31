// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestBuiltInRows_should_show_the_schema_step_as_the_last_send_left_it(t *testing.T) {
	sent := func(d ...diag.Diagnostic) *vm.Exchange {
		return &vm.Exchange{Result: engine.SendResult{Record: &engine.Record{Status: 200}, Diags: d}}
	}
	tests := []struct {
		name       string
		on         bool
		last       *vm.Exchange
		wantState  string
		wantReason bool
	}{
		{name: "off", on: false, last: sent(), wantState: "not-run", wantReason: true},
		{name: "not sent yet", on: true, wantState: "idle"},
		{name: "a send that matched", on: true, last: sent(), wantState: "passed"},
		{name: "a send that learned a new schema", on: true, last: sent(diag.New(diag.CodeSchemaUpdated, diag.Location{}, nil)), wantState: "passed"},
		{name: "a response that broke the schema", on: true, last: sent(diag.New(diag.CodeSchemaResponse, diag.Location{}, nil)), wantState: "failed", wantReason: true},
		{name: "a send that never got a response", on: true, last: &vm.Exchange{}, wantState: "idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows := builtInRows(tt.on, tt.last, true)

			if len(rows) != 2 || rows[0].Kind != "section" || rows[1].Kind != "builtin" || rows[1].Title != "schema" {
				t.Fatalf("rows %+v, want a heading and the schema step", rows)
			}
			if got := rows[1]; got.State != tt.wantState || (got.Reason != "") != tt.wantReason {
				t.Fatalf("state %q reason %q, want %q (a reason: %v)", got.State, got.Reason, tt.wantState, tt.wantReason)
			}
		})
	}
}

func TestSchemasOn_should_prefer_the_versions_own_setting(t *testing.T) {
	tests := []struct {
		name     string
		version  model.Settings
		resolved map[string]any
		want     bool
	}{
		{name: "nothing set: the default", want: true},
		{name: "off for the collection", resolved: map[string]any{"schemas.infer": false}, want: false},
		{name: "on for the version, off for the collection", version: model.Settings{"schemas.infer": true}, resolved: map[string]any{"schemas.infer": false}, want: true},
		{name: "off for the version", version: model.Settings{"schemas.infer": false}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: tt.resolved}

			got := schemasOn(b, "C1", model.RequestVersion{Settings: tt.version})

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// The step was drawn under "Built in", apart from the run, and did not say
// when it happens: the engine checks schemas after the post-hooks.
func TestBuiltInRows_should_say_when_the_schema_step_runs(t *testing.T) {
	tests := []struct {
		name    string
		hasPost bool
		want    string
	}{
		{name: "with post-hooks", hasPost: true, want: "After the post-hooks"},
		{name: "without post-hooks", hasPost: false, want: "After the response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows := builtInRows(true, nil, tt.hasPost)

			if rows[0].Title != tt.want {
				t.Fatalf("heading %q, want %q", rows[0].Title, tt.want)
			}
		})
	}
}
