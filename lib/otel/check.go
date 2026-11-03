// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/githugt/flowi/lib/engine"
)

const testTraceName = "flowi connection test"

// Check sends one small trace named "flowi connection test" to the
// collector cfg names and waits for its answer, so a person setting tracing
// up learns at once whether the address and headers work. It returns what
// the collector answered when it refused the trace.
func Check(ctx context.Context, cfg Config) error {
	e, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))) // the caller gets the error; a log line would repeat it
	if err != nil {
		return err
	}
	defer func() { _ = e.Close(ctx) }() // nothing is queued; Close only stops the worker
	const runID = "flowi-connection-test"
	t := newTrace(runID)
	now := e.cfg.Now()
	t.apply(engine.RunStarted{RunID: runID, Kind: "send", At: now}, now)
	t.root.name = testTraceName
	t.apply(engine.RunFinished{RunID: runID, Status: engine.StatusPassed}, now)
	body, err := encodeTrace(t, e.cfg.Service, e.cfg.Version)
	if err != nil {
		return fmt.Errorf("encoding the test trace: %w", err)
	}
	return e.post(body)
}
