// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func validateMethod(t *testing.T, method string) diag.List {
	t.Helper()
	f := newFixture(t)
	c := f.collection("app")
	f.request(&c, "a", "1")
	c.Requests[0].Versions[0].Method = method
	return Validate(Target{Collections: []model.Collection{c}})
}

func TestValidate_Given_standard_and_custom_methods_Then_none_is_a_problem(t *testing.T) {
	for _, m := range []string{"GET", "QUERY", "PROPFIND", "PURGE", "X-CUSTOM", "VERSION-CONTROL"} {
		t.Run(m, func(t *testing.T) {
			if got := validateMethod(t, m); len(got) != 0 {
				t.Fatalf("%s: %v", m, codesOf(got))
			}
		})
	}
}

func TestValidate_Given_a_method_that_cannot_be_sent_Then_it_is_an_error_at_the_method(t *testing.T) {
	for _, m := range []string{"GET POST", "A/B", "GÉT", "A,B"} {
		t.Run(m, func(t *testing.T) {
			d := only(t, validateMethod(t, m), diag.CodeRequestMethod)
			if d.Path != "$.versions.1.method" || d.Severity != diag.SeverityError {
				t.Fatalf("diagnostic %+v", d)
			}
		})
	}
}

func TestValidate_Given_a_known_method_in_the_wrong_case_Then_it_is_a_warning_that_says_the_right_spelling(t *testing.T) {
	d := only(t, validateMethod(t, "query"), diag.CodeRequestMethodCase)

	if d.Severity != diag.SeverityWarning || d.Args["spelling"] != "QUERY" {
		t.Fatalf("diagnostic %+v", d)
	}
}

func TestValidate_Given_an_async_or_grpc_version_Then_its_method_field_is_not_judged_as_http(t *testing.T) {
	f := newFixture(t)
	c := f.collection("app")
	f.request(&c, "a", "1")
	c.Requests[0].Versions[0].Method = ""
	c.Requests[0].Versions[0].Async = &model.Async{Protocol: "mqtt"}

	got := Validate(Target{Collections: []model.Collection{c}})

	for _, d := range got {
		if d.Code == diag.CodeRequestMethod || d.Code == diag.CodeRequestMethodCase {
			t.Fatalf("%v", codesOf(got))
		}
	}
}
