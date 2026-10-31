// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

func statusScope(status int) template.Scope {
	return template.Scope{Values: map[string]any{"response": map[string]any{"status": status}, "status": status}, Lenient: true}
}

func TestMatchOutcomes(t *testing.T) {
	named := []model.Outcome{
		{Name: "success"}, {Name: "failure"},
		{Name: "ok", When: "response.status in [200, 201]"},
		{Name: "not-found", When: "response.status == 404"},
		{Name: "server-down", When: "response.status in [500, 503]", Disabled: true},
	}
	tests := []struct {
		name   string
		list   []model.Outcome
		status Status
		scope  template.Scope
		want   []string
	}{
		{"a passing request is success and ok", named, StatusPassed, statusScope(201), []string{"success", "ok"}},
		{"a 404 the default hook failed is failure and not-found", named, StatusFailed, statusScope(404), []string{"failure", "not-found"}},
		{"a disabled outcome never matches", named, StatusFailed, statusScope(503), []string{"failure"}},
		{"a request that was not sent has only the built-ins that hold", named, StatusError, template.Scope{Values: map[string]any{}, Lenient: true}, []string{"failure"}},
		{"leaving success out of the list removes it", []model.Outcome{{Name: "failure"}, {Name: "ok", When: "response.status == 200"}}, StatusPassed, statusScope(200), []string{"ok"}},
		{"a rule that cannot be evaluated does not match", []model.Outcome{{Name: "x", When: "response.body.id == 1"}}, StatusPassed, statusScope(200), nil},
		{"a cancelled request has no outcome", named, StatusCancelled, statusScope(200), nil},
		{"the default list", model.DefaultOutcomes(), StatusPassed, statusScope(200), []string{"success"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchOutcomes(tt.list, tt.status, tt.scope)

			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
