// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"regexp"
	"strings"
	"testing"
)

// Before: a run had no link to its trace; the id was derived inside the exporter and never shown.
func TestTraceURL_Given_a_template_Then_it_holds_the_runs_trace_id(t *testing.T) {
	got := TraceURL("http://localhost:16686/trace/{trace_id}", "run-1")

	id := TraceID("run-1")
	if got != "http://localhost:16686/trace/"+id {
		t.Fatalf("got %q", got)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
		t.Fatalf("trace id %q is not 32 hex", id)
	}
}

func TestTraceURL_Given_a_template_that_names_no_trace_Then_it_is_empty(t *testing.T) {
	for _, c := range []struct{ tmpl, run string }{{"", "r"}, {"http://x/{trace_id}", ""}, {"http://x/traces", "r"}} {
		if got := TraceURL(c.tmpl, c.run); got != "" {
			t.Errorf("TraceURL(%q, %q) = %q, want empty", c.tmpl, c.run, got)
		}
	}
}

func TestTraceURL_Given_two_runs_Then_the_urls_differ_and_repeat(t *testing.T) {
	a, b := TraceURL("x/{trace_id}", "r1"), TraceURL("x/{trace_id}", "r2")
	if a == b || a != TraceURL("x/{trace_id}", "r1") || !strings.HasPrefix(a, "x/") {
		t.Fatalf("a=%q b=%q", a, b)
	}
}
