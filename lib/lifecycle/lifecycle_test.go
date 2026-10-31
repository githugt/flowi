// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"net/http"
	"strings"
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

func TestRead_should_read_the_deprecation_and_sunset_headers(t *testing.T) {
	tests := []struct {
		name        string
		header      http.Header
		wantFound   bool
		wantSince   time.Time
		wantSunset  time.Time
		wantDocs    string
		wantSuccess string
	}{
		{name: "none", header: header("Content-Type", "application/json")},
		{name: "deprecation as a structured date (RFC 9745)", header: header("Deprecation", "@1688169599"), wantFound: true, wantSince: time.Unix(1688169599, 0).UTC()},
		{name: "deprecation true (the draft's form)", header: header("Deprecation", "true"), wantFound: true},
		{name: "deprecation as an HTTP date (an older draft)", header: header("Deprecation", "Fri, 30 Jun 2023 23:59:59 GMT"), wantFound: true, wantSince: time.Date(2023, 6, 30, 23, 59, 59, 0, time.UTC)},
		{name: "sunset alone (RFC 8594)", header: header("Sunset", "Sat, 31 Dec 2033 23:59:59 GMT"), wantFound: true, wantSunset: time.Date(2033, 12, 31, 23, 59, 59, 0, time.UTC)},
		{name: "both and the links", header: header("Deprecation", "@1688169599", "Sunset", "Sat, 31 Dec 2033 23:59:59 GMT",
			"Link", `<https://example.com/deprecation>; rel="deprecation"; type="text/html", <https://example.com/v2/pets>; rel="successor-version"`),
			wantFound: true, wantSince: time.Unix(1688169599, 0).UTC(), wantSunset: time.Date(2033, 12, 31, 23, 59, 59, 0, time.UTC),
			wantDocs: "https://example.com/deprecation", wantSuccess: "https://example.com/v2/pets"},
		{name: "a value that is not a date is still a deprecation", header: header("Deprecation", "yes please"), wantFound: true},
		{name: "a sunset that is not a date is left out", header: header("Sunset", "soon")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := Read(tt.header)

			if found != tt.wantFound || !got.Since.Equal(tt.wantSince) || !got.Sunset.Equal(tt.wantSunset) || got.DocsURL != tt.wantDocs || got.SuccessorURL != tt.wantSuccess {
				t.Fatalf("found %v notice %+v", found, got)
			}
		})
	}
}

func TestNotice_Describe_should_say_what_the_server_announced(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		notice Notice
		want   []string
	}{
		{name: "deprecated only", notice: Notice{Deprecated: true}, want: []string{"deprecated"}},
		{name: "deprecated since a date", notice: Notice{Deprecated: true, Since: time.Date(2023, 6, 30, 0, 0, 0, 0, time.UTC)}, want: []string{"since 2023-06-30"}},
		{name: "a sunset ahead", notice: Notice{Deprecated: true, Sunset: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)}, want: []string{"will be removed on 2027-01-15", "77 days"}},
		{name: "a sunset that has passed", notice: Notice{Sunset: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}, want: []string{"was to be removed on 2026-10-01", "28 days ago"}},
		{name: "with the links", notice: Notice{Deprecated: true, DocsURL: "https://e.com/d", SuccessorURL: "https://e.com/v2"}, want: []string{"https://e.com/d", "https://e.com/v2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.notice.Describe(now)

			for _, part := range tt.want {
				if !strings.Contains(got, part) {
					t.Fatalf("%q lacks %q", got, part)
				}
			}
		})
	}
}
