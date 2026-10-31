// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func header(pairs ...string) http.Header {
	h := http.Header{}
	for i := 0; i < len(pairs); i += 2 {
		h.Add(pairs[i], pairs[i+1])
	}
	return h
}

func TestRead_should_read_the_headers_a_server_may_send(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   Info
		found  bool
	}{
		{name: "none", header: header("Content-Type", "text/plain")},
		{name: "the draft's structured fields", header: header("RateLimit-Policy", `"burst";q=100;w=60`, "RateLimit", `"burst";r=50;t=30`),
			want: Info{Policy: "burst", Limit: 100, Remaining: 50, Reset: 30 * time.Second, Window: 60 * time.Second}, found: true},
		{name: "the draft without a policy header", header: header("RateLimit", `"default";r=0;t=12`),
			want: Info{Policy: "default", Limit: -1, Remaining: 0, Reset: 12 * time.Second}, found: true},
		{name: "several policies: the one with least left", header: header("RateLimit-Policy", `"sec";q=10;w=1, "day";q=1000;w=86400`, "RateLimit", `"sec";r=7;t=1, "day";r=3;t=40000`),
			want: Info{Policy: "day", Limit: 1000, Remaining: 3, Reset: 40000 * time.Second, Window: 86400 * time.Second}, found: true},
		{name: "the earlier drafts", header: header("RateLimit-Limit", "100", "RateLimit-Remaining", "99", "RateLimit-Reset", "30"),
			want: Info{Limit: 100, Remaining: 99, Reset: 30 * time.Second}, found: true},
		{name: "X-RateLimit with seconds to reset", header: header("X-RateLimit-Limit", "60", "X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "45"),
			want: Info{Limit: 60, Remaining: 0, Reset: 45 * time.Second}, found: true},
		{name: "X-RateLimit with an epoch reset", header: header("X-RateLimit-Limit", "60", "X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "1793290845"),
			want: Info{Limit: 60, Remaining: 0, Reset: 45 * time.Second}, found: true},
		{name: "a header that is not a number", header: header("RateLimit-Remaining", "lots")},
	}
	now := time.Unix(1793290800, 0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := Read(tt.header, now)

			if found != tt.found || got != tt.want {
				t.Fatalf("found %v got %+v, want %+v", found, got, tt.want)
			}
		})
	}
}

func TestInfo_Exhausted_should_be_true_only_when_nothing_is_left_and_it_resets(t *testing.T) {
	tests := []struct {
		info Info
		want bool
	}{
		{Info{Remaining: 0, Reset: 5 * time.Second}, true},
		{Info{Remaining: 3, Reset: 5 * time.Second}, false},
		{Info{Remaining: 0}, false}, // no reset time: nothing to wait for
	}
	for _, tt := range tests {
		if got := tt.info.Exhausted(); got != tt.want {
			t.Errorf("%+v: got %v", tt.info, got)
		}
	}
}
