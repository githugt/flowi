// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
)

func TestTookText_should_pick_the_unit_that_reads_best(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"nothing measured", 0, ""},
		{"microseconds", 820 * time.Microsecond, "820 µs"},
		{"milliseconds", 142 * time.Millisecond, "142 ms"},
		{"just under a second", 999 * time.Millisecond, "999 ms"},
		{"seconds", 1240 * time.Millisecond, "1.24 s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TookText(tc.in)

			if got != tc.want {
				t.Errorf("TookText(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSizeText_should_scale_bytes_and_show_an_empty_body(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"unknown", -1, ""},
		{"empty", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1234, "1.2 KB"},
		{"megabytes", 3_400_000, "3.4 MB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SizeText(tc.in)

			if got != tc.want {
				t.Errorf("SizeText(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResponseFacts_Given_a_compressed_redirected_http2_response_Then_the_line_says_all_three(t *testing.T) {
	rec := &engine.Record{Proto: "HTTP/2.0", Redirects: []engine.Hop{{Status: 301, URL: "/a", Location: "/b"}, {Status: 302, URL: "/b", Location: "/c"}}}
	rec.Body.Encoding, rec.Body.RawSize, rec.Body.Size = "gzip", 12000, 340000

	got := ResponseFacts(rec)

	// Before: nothing showed the version, the compression or the redirects.
	if want := "HTTP/2 · gzip 12.0 KB → 340.0 KB · 2 redirects"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResponseFacts_Given_a_plain_http11_response_Then_there_is_nothing_to_say(t *testing.T) {
	rec := &engine.Record{Proto: "HTTP/1.1"}
	rec.Body.Size = 10

	if got := ResponseFacts(rec); got != "" {
		t.Fatalf("got %q", got)
	}
	if ResponseFacts(nil) != "" || ResponseFacts(&engine.Record{Kind: engine.KindGRPC, Proto: "HTTP/2.0"}) != "" {
		t.Fatal("a missing record and a gRPC call have no facts line")
	}
}

func TestRedirectLines_Given_hops_Then_each_is_a_line_and_forwarded_credentials_are_flagged(t *testing.T) {
	rec := &engine.Record{Redirects: []engine.Hop{{Status: 301, URL: "/a", Location: "/b"}, {Status: 307, URL: "/b", Location: "https://other/c", Forwarded: []string{"Authorization"}}}}

	got := RedirectLines(rec)

	if len(got) != 2 || got[0] != "301 /a → /b" || got[1] != "307 /b → https://other/c (credentials forwarded: Authorization)" {
		t.Fatalf("got %q", got)
	}
}
