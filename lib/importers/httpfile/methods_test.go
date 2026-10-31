// SPDX-License-Identifier: Apache-2.0

package httpfile

import (
	"testing"
)

func TestRequestLine_Given_query_Then_it_is_a_method_not_a_url(t *testing.T) {
	// Act
	m := requestLine.FindStringSubmatch("QUERY https://api.example.com/search")

	// Assert
	if m == nil || m[1] != "QUERY" || m[2] != "https://api.example.com/search" {
		t.Fatalf("match %q", m)
	}
}
