// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

// specContracts gives the schemas a spec would have kept: by request path and version.
type specContracts map[string][]Sample

func (s specContracts) Contract(_ model.Collection, r model.Request, version string) (Contract, diag.List) {
	samples := s[r.Path+"@"+version]
	if len(samples) == 0 {
		return Contract{}, nil
	}
	return Contract{Source: SourceSchema, Samples: samples}, nil
}

func obj(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

func loginSchemas() []Sample {
	ok := obj(map[string]any{
		"token": map[string]any{"type": "string"},
		"extra": map[string]any{"type": "string"},
		"user":  obj(map[string]any{"id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}),
	})
	bad := obj(map[string]any{"error": map[string]any{"type": "string"}})
	return []Sample{{Source: SourceSchema, Status: 200, Schema: ok}, {Source: SourceSchema, Status: 401, Schema: bad}}
}

func specified(t *testing.T) []model.Collection {
	t.Helper()
	cs := loadUsage(t)
	for ci := range cs {
		for ri := range cs[ci].Requests {
			cs[ci].Requests[ri].Operation = cs[ci].Requests[ri].Path
		}
	}
	return cs
}

func operationOf(t *testing.T, c Coverage, request, version string) OperationCoverage {
	t.Helper()
	for _, op := range c.Operations {
		if op.Request == request && op.Version == version {
			return op
		}
	}
	t.Fatalf("no operation %s@%s in %+v", request, version, c.Operations)
	return OperationCoverage{}
}

func TestCover_Given_a_flow_that_reads_some_fields_Then_the_read_and_unread_fields_are_listed(t *testing.T) {
	// Arrange
	src := specContracts{"auth/login@2": loginSchemas()}

	// Act
	got := Cover(specified(t), src, nil)

	// Assert
	op := operationOf(t, got, "auth/login", "2")
	if !slices.Contains(op.Used, "signup") {
		t.Errorf("used by %v", op.Used)
	}
	for _, read := range []string{"$.token", "$.user.id"} {
		if !slices.Contains(op.Read, read) {
			t.Errorf("%s should be read: read %v unread %v", read, op.Read, op.Unread)
		}
	}
	for _, unread := range []string{"$.extra", "$.user.name"} {
		if !slices.Contains(op.Unread, unread) {
			t.Errorf("%s should be unread: read %v unread %v", unread, op.Read, op.Unread)
		}
	}
}

func TestCover_Given_an_operation_no_flow_calls_Then_it_is_listed_with_no_users(t *testing.T) {
	// Arrange
	cs := specified(t)
	cs[0].Requests = append(cs[0].Requests, model.Request{
		Meta: model.Meta{ID: "01JB00000000000000000000ZZ", Name: "Orphan"}, Path: "orphan", Operation: "orphan",
		Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "https://x.test/orphan"}},
	})
	src := specContracts{"orphan@1": {{Source: SourceSchema, Status: 200, Schema: obj(map[string]any{"a": map[string]any{"type": "string"}})}}}

	// Act
	got := Cover(cs, src, nil)

	// Assert
	op := operationOf(t, got, "orphan", "1")
	if len(op.Used) != 0 || got.Totals().Operations != 1 || got.Totals().Used != 0 {
		t.Fatalf("op %+v totals %+v", op, got.Totals())
	}
}

func TestCover_Given_runs_Then_the_statuses_they_got_are_seen_and_the_others_are_not(t *testing.T) {
	// Arrange
	src := specContracts{"auth/login@2": loginSchemas()}
	run := report.Run{
		Nodes:    []report.Node{{Path: "login", ID: "login", Attempts: []report.Attempt{{N: 1, Status: 200}}}},
		Versions: []report.VersionUse{{Path: "login", RequestID: string(loginID), RequestPath: "auth/login", Version: "2"}},
	}

	// Act
	got := Cover(specified(t), src, []report.Run{run})

	// Assert
	op := operationOf(t, got, "auth/login", "2")
	seen := map[int]bool{}
	for _, s := range op.Statuses {
		seen[s.Status] = s.Seen
	}
	if !seen[200] || seen[401] || len(op.Statuses) != 2 {
		t.Fatalf("statuses %+v", op.Statuses)
	}
}

func TestCover_Given_a_request_without_a_spec_operation_Then_it_is_not_part_of_the_spec_universe(t *testing.T) {
	// Arrange: no Operation set, so nothing was imported from a spec.
	cs := loadUsage(t)
	src := specContracts{"auth/login@2": loginSchemas()}

	// Act
	got := Cover(cs, src, nil)

	// Assert
	if len(got.Operations) != 0 {
		t.Fatalf("operations %+v", got.Operations)
	}
}
