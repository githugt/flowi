// SPDX-License-Identifier: Apache-2.0

package respdiff_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/respdiff"
)

func jsonRec(status int, v any, headers ...engine.Header) engine.Record {
	return engine.Record{Status: status, Header: headers, Body: engine.Body{JSON: v}}
}

func paths(cs []respdiff.Change) []string {
	var out []string
	for _, c := range cs {
		out = append(out, fmt.Sprintf("%s %s %q→%q", c.Kind, c.Path, c.Old, c.New))
	}
	return out
}

func TestCompare_should_name_each_changed_json_value_by_path(t *testing.T) {
	older := jsonRec(200, map[string]any{"id": 1.0, "name": "a", "items": []any{map[string]any{"price": 4.5}, map[string]any{"price": 1.0}}, "gone": true, "my key": 1.0})
	newer := jsonRec(200, map[string]any{"id": 1.0, "name": "b", "items": []any{map[string]any{"price": 5.0}}, "extra": []any{}, "my key": 1.0})

	d := respdiff.Compare(older, newer, respdiff.Options{})

	want := []string{
		`removed $.gone "true"→""`,
		`changed $.items[0].price "4.5"→"5"`,
		`removed $.items[1] "{\"price\":1}"→""`,
		`changed $.name "\"a\""→"\"b\""`,
		`added $.extra ""→"[]"`,
	}
	if got := paths(d.Body); strings.Join(got, "|") != strings.Join(want, "|") || d.BodyMode != "json" || d.Same() {
		t.Fatalf("mode %q\n got %q\nwant %q", d.BodyMode, got, want)
	}
}

func TestCompare_should_see_a_changed_type_and_ignore_equal_bodies(t *testing.T) {
	same := respdiff.Compare(jsonRec(200, map[string]any{"a": 1.0}), jsonRec(200, map[string]any{"a": 1.0}), respdiff.Options{})
	typed := respdiff.Compare(jsonRec(200, map[string]any{"a": "1"}), jsonRec(200, map[string]any{"a": 1.0}), respdiff.Options{})

	if !same.Same() || len(typed.Body) != 1 || typed.Body[0].Kind != respdiff.Changed {
		t.Fatalf("same %+v typed %+v", same, typed.Body)
	}
}

func TestCompare_should_report_status_and_header_changes_and_skip_headers_that_change_every_send(t *testing.T) {
	older := jsonRec(200, nil,
		engine.Header{Name: "Content-Type", Value: "application/json"}, engine.Header{Name: "Date", Value: "Mon"}, engine.Header{Name: "X-Old", Value: "1"}, engine.Header{Name: "Set-Cookie", Value: "sid=1"})
	newer := jsonRec(404, nil,
		engine.Header{Name: "content-type", Value: "text/plain"}, engine.Header{Name: "Date", Value: "Tue"}, engine.Header{Name: "X-New", Value: "2"}, engine.Header{Name: "Set-Cookie", Value: "sid=2"})

	d := respdiff.Compare(older, newer, respdiff.Options{})

	want := []string{`changed content-type "application/json"→"text/plain"`, `removed x-old "1"→""`, `added x-new ""→"2"`}
	if d.OldStatus != 200 || d.NewStatus != 404 || strings.Join(paths(d.Headers), "|") != strings.Join(want, "|") || d.Volatile != 2 || d.Hidden != 2 {
		t.Fatalf("headers %q volatile %d hidden %d", paths(d.Headers), d.Volatile, d.Hidden)
	}
	for _, c := range d.Headers {
		if strings.Contains(c.Old+c.New, "sid=") {
			t.Fatalf("a credential header was shown: %+v", c)
		}
	}
}

func TestCompare_should_not_show_or_compare_credential_fields_of_a_json_body(t *testing.T) {
	older := jsonRec(200, map[string]any{"access_token": "old-token", "name": "a"})
	newer := jsonRec(200, map[string]any{"access_token": "new-token", "name": "a"})

	d := respdiff.Compare(older, newer, respdiff.Options{})

	if len(d.Body) != 0 || d.Hidden != 1 || !d.Same() {
		t.Fatalf("%+v", d)
	}
}

func TestCompare_should_compare_text_by_line(t *testing.T) {
	older := engine.Record{Status: 200, Body: engine.Body{Text: "a\nb\nc\nd"}}
	newer := engine.Record{Status: 200, Body: engine.Body{Text: "a\nB\nc\nd\ne"}}

	d := respdiff.Compare(older, newer, respdiff.Options{})

	want := []string{`added line 2 ""→"B"`, `removed line 2 "b"→""`, `added line 5 ""→"e"`}
	if d.BodyMode != "text" || strings.Join(paths(d.Body), "|") != strings.Join(want, "|") {
		t.Fatalf("mode %q %q", d.BodyMode, paths(d.Body))
	}
}

// Before: a line diff of a large body was refused or took quadratic time.
func TestCompare_Given_two_huge_different_texts_Then_it_reports_one_changed_block_at_once(t *testing.T) {
	a := strings.Repeat("x\n", 5000) + "same"
	b := strings.Repeat("y\n", 5000) + "same"

	d := respdiff.Compare(engine.Record{Body: engine.Body{Text: a}}, engine.Record{Body: engine.Body{Text: b}}, respdiff.Options{})

	if len(d.Body) != 1 || d.Body[0].Kind != respdiff.Changed || d.Body[0].Path != "lines 1–5000" || d.Body[0].Old != "5000 lines" {
		t.Fatalf("%q", paths(d.Body))
	}
}

func TestCompare_should_cap_the_list_and_count_what_it_left_out(t *testing.T) {
	older, newer := map[string]any{}, map[string]any{}
	for i := range 30 {
		older[fmt.Sprintf("k%02d", i)] = 1.0
		newer[fmt.Sprintf("k%02d", i)] = 2.0
	}

	d := respdiff.Compare(jsonRec(200, older), jsonRec(200, newer), respdiff.Options{MaxChanges: 10})

	if len(d.Body) != 10 || d.Truncated != 20 || d.Same() {
		t.Fatalf("%d listed, %d left out", len(d.Body), d.Truncated)
	}
}

func TestCompare_should_tell_binary_bodies_apart_by_digest_and_size(t *testing.T) {
	a := engine.Record{Body: engine.Body{Base64: "AAAA", SHA256: "x", Size: 3}}
	b := engine.Record{Body: engine.Body{Base64: "BBBB", SHA256: "y", Size: 3}}

	d := respdiff.Compare(a, b, respdiff.Options{})
	same := respdiff.Compare(a, a, respdiff.Options{})

	if d.BodyMode != "binary" || len(d.Body) != 1 || !same.Same() {
		t.Fatalf("%+v same %v", d, same.Same())
	}
}
