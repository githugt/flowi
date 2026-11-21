// SPDX-License-Identifier: Apache-2.0

package yamldoc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDecode_should_read_a_plain_document(t *testing.T) {
	got, err := Decode([]byte("openapi: 3.1.0\ninfo:\n  title: Shop\n"))

	if err != nil || got["openapi"] != "3.1.0" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// Before: a document of nested aliases was expanded without a limit: nine copies per level, so seven levels cost
// hundreds of milliseconds and a few more levels used all the memory of the machine.
func TestDecode_Given_nested_aliases_Then_refuses_the_document_quickly(t *testing.T) {
	var doc strings.Builder
	doc.WriteString("a0: &a0 [x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&doc, "a%d: &a%d [*a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d]\n", i, i, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1)
	}
	start := time.Now()

	_, err := Decode([]byte(doc.String()))

	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err %v, want ErrTooLarge", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("took %v", took)
	}
}
