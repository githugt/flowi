// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"
	"time"

	"github.com/githugt/flowi-lib/engine"
)

func TestStatusLine_should_say_how_many_rows_a_dataset_request_returned(t *testing.T) {
	cases := []struct {
		name string
		rec  engine.Record
		want string
	}{
		{"rows", datasetRecord(2, false), "SQL postgres://app@db/shop → ok  2 rows  3ms  (version 1, default)"},
		{"one row", datasetRecord(1, false), "SQL postgres://app@db/shop → ok  1 row  3ms  (version 1, default)"},
		{"cut", datasetRecord(1000, true), "SQL postgres://app@db/shop → ok  1000 rows (more were cut)  3ms  (version 1, default)"},
		{"failed", engine.Record{Kind: engine.KindDataset, Outcome: engine.OutcomeError, Request: engine.SentRequest{Method: "SQL", URL: "postgres://app@db/shop"},
			Timings: engine.Timings{Total: 3 * time.Millisecond}, Version: "1", VersionSource: "default"}, "SQL postgres://app@db/shop → error  0 rows  3ms  (version 1, default)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := statusLine(&c.rec)

			if got != c.want {
				t.Fatalf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

func datasetRecord(rows int, truncated bool) engine.Record {
	return engine.Record{
		Kind: engine.KindDataset, Outcome: engine.OutcomeOK, Version: "1", VersionSource: "default",
		Request: engine.SentRequest{Method: "SQL", URL: "postgres://app@db/shop"},
		Timings: engine.Timings{Total: 3 * time.Millisecond},
		Body:    engine.Body{JSON: map[string]any{"row_count": rows, "truncated": truncated}},
	}
}
