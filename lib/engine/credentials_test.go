// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"testing"
)

func TestCredentialHeaders_should_name_the_headers_that_look_like_credentials_once(t *testing.T) {
	// Arrange
	rec := Record{Header: []Header{
		{Name: "Content-Type", Value: "application/json"},
		{Name: "Set-Cookie", Value: "a=1"},
		{Name: "set-cookie", Value: "b=2"},
		{Name: "X-Auth-Token", Value: "t"},
		{Name: "X-Request-Id", Value: "r"},
		{Name: "X-Api-Key", Value: "k"},
	}}

	// Act
	got := CredentialHeaders(rec)

	// Assert
	if want := []string{"Set-Cookie", "X-Auth-Token", "X-Api-Key"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCredentialHeaders_should_be_empty_for_an_ordinary_response(t *testing.T) {
	// Act
	got := CredentialHeaders(Record{Header: []Header{{Name: "Content-Type", Value: "text/plain"}, {Name: "ETag", Value: "x"}}})

	// Assert
	if len(got) != 0 {
		t.Errorf("got %v", got)
	}
}
