// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestMasker_should_mask_raw_json_url_and_base64_forms(t *testing.T) {
	const secret = `p"a&s/s w\rd`
	m := NewMasker(secret, "", "short")
	tests := map[string]string{
		"raw":         "pw=" + secret,
		"json":        `{"pw":"p\"a&s/s w\\rd"}`,
		"json html":   `{"pw":"p\"a&s/s w\\rd"}`,
		"query":       "?pw=" + url.QueryEscape(secret),
		"path":        "/" + url.PathEscape(secret),
		"base64":      "Basic " + base64.StdEncoding.EncodeToString([]byte(secret)),
		"other value": "a short story",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			got := m.String(in)

			if !strings.Contains(got, Mask) {
				t.Fatalf("not masked: %q", got)
			}
			if strings.Contains(got, "short") || strings.Contains(got, `a&s`) {
				t.Fatalf("leftover secret text: %q", got)
			}
		})
	}
}

func TestMasker_Given_secret_that_contains_another_Then_longest_masked_whole(t *testing.T) {
	m := NewMasker("abc", "abcdef")

	got := m.String("x abcdef y abc")

	if got != "x "+Mask+" y "+Mask {
		t.Fatalf("got %q", got)
	}
}

func TestMasker_With_should_not_change_original(t *testing.T) {
	a := NewMasker("one")
	b := a.With("two")

	if a.String("two") != "two" || b.String("two") != Mask || string(b.Bytes([]byte("one"))) != Mask {
		t.Fatal("With mutated the original or missed values")
	}
}

func TestMasker_zero_value_should_mask_nothing(t *testing.T) {
	var m Masker

	if m.String("anything") != "anything" {
		t.Fatal("zero masker changed text")
	}
}
