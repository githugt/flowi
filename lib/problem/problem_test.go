// SPDX-License-Identifier: Apache-2.0

package problem

import (
	"strings"
	"testing"
)

func TestCheck_should_accept_a_problem_details_document(t *testing.T) {
	tests := []struct {
		name, contentType string
		status            int
		body              any
	}{
		{name: "all members", contentType: "application/problem+json", status: 404, body: map[string]any{
			"type": "https://example.com/probs/out-of-credit", "title": "Out of credit", "status": 404, "detail": "No credit.", "instance": "/account/12345/msgs/abc"}},
		{name: "only a title", contentType: "application/problem+json; charset=utf-8", status: 400, body: map[string]any{"title": "Bad"}},
		{name: "an empty object (type about:blank)", contentType: "application/problem+json", status: 500, body: map[string]any{}},
		{name: "extension members", contentType: "application/problem+json", status: 422, body: map[string]any{"title": "x", "errors": []any{"a"}, "balance": 30}},
		{name: "status as a float, as decoded JSON gives it", contentType: "application/problem+json", status: 400, body: map[string]any{"status": float64(400)}},
		{name: "a relative type reference", contentType: "application/problem+json", status: 400, body: map[string]any{"type": "/probs/x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.status, tt.contentType, tt.body)

			if len(got) != 0 {
				t.Fatalf("violations %v", got)
			}
		})
	}
}

func TestCheck_should_name_what_is_wrong(t *testing.T) {
	tests := []struct {
		name, contentType string
		status            int
		body              any
		wantPath, want    string
	}{
		{name: "plain JSON content type", contentType: "application/json", status: 400, body: map[string]any{"title": "x"}, wantPath: "Content-Type", want: "application/problem+json"},
		{name: "no content type", contentType: "", status: 400, body: map[string]any{"title": "x"}, wantPath: "Content-Type", want: "application/problem+json"},
		{name: "not an object", contentType: "application/problem+json", status: 400, body: []any{1}, wantPath: "$", want: "object"},
		{name: "text body", contentType: "application/problem+json", status: 400, body: "oops", wantPath: "$", want: "object"},
		{name: "status differs", contentType: "application/problem+json", status: 400, body: map[string]any{"status": 404}, wantPath: "$.status", want: "404"},
		{name: "status not a number", contentType: "application/problem+json", status: 400, body: map[string]any{"status": "400"}, wantPath: "$.status", want: "number"},
		{name: "title not a string", contentType: "application/problem+json", status: 400, body: map[string]any{"title": 5}, wantPath: "$.title", want: "string"},
		{name: "detail not a string", contentType: "application/problem+json", status: 400, body: map[string]any{"detail": true}, wantPath: "$.detail", want: "string"},
		{name: "type with a space", contentType: "application/problem+json", status: 400, body: map[string]any{"type": "not a uri"}, wantPath: "$.type", want: "URI reference"},
		{name: "instance not a string", contentType: "application/problem+json", status: 400, body: map[string]any{"instance": 1}, wantPath: "$.instance", want: "string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.status, tt.contentType, tt.body)

			found := false
			for _, v := range got {
				if v.Path == tt.wantPath && strings.Contains(v.Message, tt.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("violations %v, want one at %s mentioning %q", got, tt.wantPath, tt.want)
			}
		})
	}
}

func TestIsProblemType_should_accept_the_json_and_xml_forms_only(t *testing.T) {
	for contentType, want := range map[string]bool{
		"application/problem+json": true, "Application/Problem+JSON; charset=utf-8": true, "application/problem+xml": true,
		"application/json": false, "text/plain": false, "": false, "application/problem": false,
	} {
		if got := IsProblemType(contentType); got != want {
			t.Errorf("%q: got %v, want %v", contentType, got, want)
		}
	}
}
