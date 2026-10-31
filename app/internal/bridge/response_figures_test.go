// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
)

func exchangeWith(rec *engine.Record) vm.Exchange {
	return vm.Exchange{Title: "auth/login v1", At: time.Now(), Result: engine.SendResult{Status: engine.StatusPassed, Record: rec}}
}

func TestResponseOf_should_show_how_long_the_send_took_and_how_big_the_body_was(t *testing.T) {
	t.Parallel()
	rec := &engine.Record{Status: 200, Timings: engine.Timings{Total: 142 * time.Millisecond}}
	rec.Body.Size = 1234

	got := responseOf(exchangeWith(rec), vm.ViewJSON)

	if got.Took != "142 ms" || got.Size != "1.2 KB" {
		t.Fatalf("took %q size %q, want 142 ms and 1.2 KB", got.Took, got.Size)
	}
}

func TestResponseOf_should_show_no_size_for_a_message_exchange(t *testing.T) {
	t.Parallel()
	// Before: a received message list had no single body, yet the header could claim "0 B".
	rec := &engine.Record{Kind: engine.KindGRPC, GRPCCode: "OK", Timings: engine.Timings{Total: 5 * time.Millisecond}}

	got := responseOf(exchangeWith(rec), vm.ViewJSON)

	if got.Size != "" {
		t.Fatalf("size %q, want none", got.Size)
	}
}

func TestResponseOf_should_show_the_rows_of_a_dataset_record_as_a_table(t *testing.T) {
	t.Parallel()
	rec := &engine.Record{
		Kind: engine.KindDataset, Outcome: engine.OutcomeOK, Timings: engine.Timings{Total: 5 * time.Millisecond},
		Body: engine.Body{JSON: map[string]any{
			"columns":   []any{"id", "name", "note"},
			"rows":      []any{map[string]any{"id": 1, "name": "Ada", "note": nil}, map[string]any{"id": 2, "name": "Grace", "note": "vip"}},
			"row_count": 2, "truncated": true,
		}},
	}

	got := responseOf(exchangeWith(rec), vm.ViewJSON)

	table := got.Table
	if table == nil || len(table.Columns) != 3 || table.Columns[1] != "name" || len(table.Rows) != 2 || !table.Truncated || table.RowCount != 2 {
		t.Fatalf("table %+v", table)
	}
	if table.Rows[0][1] != "Ada" || table.Rows[0][2] != nil || table.Rows[1][0] != 2 {
		t.Fatalf("rows %v", table.Rows)
	}
	if got.Code != "ok" || got.Size != "" {
		t.Fatalf("code %q size %q", got.Code, got.Size)
	}
}

func TestResponseOf_should_show_no_table_for_a_response_that_is_not_a_dataset(t *testing.T) {
	t.Parallel()
	rec := &engine.Record{Status: 200}

	got := responseOf(exchangeWith(rec), vm.ViewJSON)

	if got.Table != nil {
		t.Fatalf("table %+v", got.Table)
	}
}
