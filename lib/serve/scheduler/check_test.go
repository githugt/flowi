// SPDX-License-Identifier: Apache-2.0

package scheduler_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/serve/scheduler"
)

func valid() model.Schedule {
	return model.Schedule{Name: "nightly", Flow: "user-creation", Cron: "0 3 * * *", TZ: "UTC",
		Inputs: []model.KeyValue{{Key: "email", Value: "a@example.com"}}}
}

func TestCheck_should_accept_a_complete_schedule(t *testing.T) {
	if got := scheduler.Check(valid()); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestCheck_should_name_each_problem_by_field(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*model.Schedule)
		field  string
		detail string
	}{
		{"no name", func(s *model.Schedule) { s.Name = "" }, "name", "is required"},
		{"bad name", func(s *model.Schedule) { s.Name = "../x" }, "name", "letters, digits"},
		{"no flow", func(s *model.Schedule) { s.Flow = "" }, "flow", "is required"},
		{"no cron", func(s *model.Schedule) { s.Cron = "" }, "cron", "is required"},
		{"bad cron", func(s *model.Schedule) { s.Cron = "61 * * * *" }, "cron", "minute: 61 is outside 0-59"},
		{"bad tz", func(s *model.Schedule) { s.TZ = "Mars/Base" }, "tz", "unknown time zone"},
		{"empty input name", func(s *model.Schedule) { s.Inputs = []model.KeyValue{{Key: "", Value: "x"}} }, "inputs", "empty name"},
		{"duplicate input", func(s *model.Schedule) {
			s.Inputs = []model.KeyValue{{Key: "a", Value: "1"}, {Key: "a", Value: "2"}}
		}, "inputs", `"a" is given twice`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := valid()
			c.edit(&s)

			got := scheduler.Check(s)

			if len(got) != 1 || got[0].Field != c.field || !strings.Contains(got[0].Detail, c.detail) {
				t.Fatalf("got %+v, want field %q containing %q", got, c.field, c.detail)
			}
		})
	}
}

func TestDuplicates_should_find_names_used_twice(t *testing.T) {
	a, b, c := valid(), valid(), valid()
	b.Name, c.Name = "other", "nightly"

	got := scheduler.Duplicates([]model.Schedule{a, b, c})

	if !slices.Equal(got, []int{2}) {
		t.Fatalf("got %v, want the second use at index 2", got)
	}
}
