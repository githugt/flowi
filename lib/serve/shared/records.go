// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/githugt/flowi-lib/serve/runs"
)

// recordTTL is how long a published run stays readable by other instances.
const recordTTL = 24 * time.Hour

// Records publishes run records in a Store; it is a runs.SharedRecords.
type Records struct{ Store Store }

// SaveRecord implements runs.SharedRecords.
func (r Records) SaveRecord(ctx context.Context, rec runs.Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding run %s: %w", rec.ID, err)
	}
	return r.Store.Put(ctx, recordKey(rec.ID), data, recordTTL)
}

// LoadRecord implements runs.SharedRecords.
func (r Records) LoadRecord(ctx context.Context, id string) (runs.Record, bool, error) {
	data, ok, err := r.Store.Get(ctx, recordKey(id))
	if err != nil || !ok {
		return runs.Record{}, false, err
	}
	var rec runs.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return runs.Record{}, false, fmt.Errorf("decoding run %s: %w", id, err)
	}
	return rec, true, nil
}

func recordKey(id string) string { return "run:" + id }
