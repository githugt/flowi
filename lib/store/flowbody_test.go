// SPDX-License-Identifier: Apache-2.0

package store

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: a copy of steps for another collection (app: "Copy as portable") writes each request by path alone, but the
// flow codec insisted on an id in every reference, so such a copy could not be read back as steps.
func TestFlowSnippet_Given_a_request_named_by_path_alone_Then_it_reads_back_but_the_flow_file_form_still_asks_for_the_id(t *testing.T) {
	s := newStore()
	f := model.Flow{Meta: model.Meta{ID: "01JZ0000000000000000000001"}, Nodes: []model.Node{{
		ID: "login", Type: "request", Name: "Login", Description: "POST /login",
		Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}},
	}}}

	data, err := s.EncodeFlowBody(f)
	got, diags := s.DecodeFlowSnippet("/c/flows/f.arazzo.yaml", "f", data)
	_, strict := s.DecodeFlowBody("/c/flows/f.arazzo.yaml", "f", data)

	if err != nil || strings.Contains(string(data), "id: \"\"") {
		t.Fatalf("text:\n%s\nerr %v", data, err)
	}
	if diags.HasErrors() || len(got.Nodes) != 1 || got.Nodes[0].Request.Request.ID != "" || got.Nodes[0].Request.Request.Path != "auth/login" || got.Nodes[0].Name != "Login" || got.Nodes[0].Description != "POST /login" {
		t.Fatalf("diags %v, nodes %+v", diags, got.Nodes)
	}
	if !strict.HasErrors() {
		t.Fatal("the strict form accepted a reference with no id")
	}
}
