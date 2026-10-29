// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	"github.com/githugt/flowi-lib/engine"

	"github.com/githugt/flowi/internal/vm"
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
