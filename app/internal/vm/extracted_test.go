// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: the Extract tab never said whether a path matched anything; the user found out in a later step.
func TestExtractedOf_should_say_what_each_tried_extraction_took_and_which_matched_nothing(t *testing.T) {
	rec := &engine.Record{Outputs: map[string]any{"token": "abc", "ids": []any{1, 2}, "long": strings.Repeat("é", 100)}}
	ex := vm.Exchange{Result: engine.SendResult{Status: engine.StatusFailed, Record: rec}, Extracts: []string{"token", "ids", "missing", "long"}}

	got := vm.ExtractedOf(ex)

	if len(got) != 4 || got[0] != (vm.Extracted{Name: "token", Value: "abc", Found: true}) ||
		got[1] != (vm.Extracted{Name: "ids", Value: "[1,2]", Found: true}) ||
		got[2] != (vm.Extracted{Name: "missing"}) {
		t.Fatalf("got %+v", got)
	}
	if long := got[3]; !long.Found || len([]rune(long.Value)) != 61 || !strings.HasSuffix(long.Value, "…") {
		t.Errorf("long %+v", long)
	}
}

func TestExtractedOf_Given_no_record_or_an_unknown_send_Then_nothing_matched_or_nothing_is_listed(t *testing.T) {
	noRecord := vm.Exchange{Extracts: []string{"token"}}
	restored := vm.Exchange{Result: engine.SendResult{Record: &engine.Record{Outputs: map[string]any{"token": "x"}}}}

	a := vm.ExtractedOf(noRecord)
	b := vm.ExtractedOf(restored)

	if len(a) != 1 || a[0].Found || len(b) != 0 || b == nil {
		t.Fatalf("a %+v b %#v", a, b)
	}
}

func TestExtractNames_should_list_named_extractions_in_order_and_nil_for_none(t *testing.T) {
	rows := []model.Extraction{{Name: "a"}, {Name: ""}, {Name: "b"}}

	got := vm.ExtractNames(rows)

	if len(got) != 2 || got[0] != "a" || got[1] != "b" || vm.ExtractNames(nil) != nil {
		t.Fatalf("got %v", got)
	}
}
