// SPDX-License-Identifier: Apache-2.0

// Package dataset runs a read-only query against a data store and returns its rows. It is the dataset family of
// requests (see model.Family): a query goes out, rows come back.
//
// A Source runs the queries of one kind of store. Registry maps the driver names a request may use to Sources, so a
// new store (a CSV file, say) is one more Source and nothing that already works changes. The SQL sources use pure Go
// drivers only: no cgo, so the lib and the CLI still build everywhere with CGO_ENABLED=0.
package dataset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/githugt/flowi-lib/dataset/readonly"
	"github.com/githugt/flowi-lib/model"
)

// Query is one query to run. Statement is sent as written; Params are sent apart from it.
type Query struct {
	Driver    string
	Server    string
	Statement string
	Params    []string
	// Limit is the most rows kept; 0 means model.DefaultDatasetLimit.
	Limit int
	// Root is the directory a relative file path (sqlite) is resolved against.
	Root string
	// Timeout bounds connecting and running together; 0 means DefaultTimeout.
	Timeout time.Duration
}

// DefaultTimeout is how long a query may take when it sets no timeout.
const DefaultTimeout = 30 * time.Second

// Result is what a query returned. Every row has one value per column, in column order; a column that repeats a name
// gets a numbered suffix (a, a_2) so a row can also be read by name.
type Result struct {
	Columns []string
	Rows    [][]any
	// Truncated is true when the store had more rows than Limit.
	Truncated bool
	Duration  time.Duration
}

// Source runs the queries of one kind of store.
type Source interface {
	Query(ctx context.Context, q Query) (Result, error)
}

// Registry maps a driver name to the Source that runs its queries.
type Registry map[string]Source

// The errors a query can end in besides the store's own.
var (
	// ErrUnknownDriver is returned for a driver that is not in the registry.
	ErrUnknownDriver = errors.New("unknown dataset driver")
	// ErrTimeout is returned when the query did not finish in its timeout.
	ErrTimeout = errors.New("the query timed out")
)

// Default is the registry of the built-in stores.
func Default() Registry {
	return Registry{
		model.DriverPostgres: postgresSource(),
		model.DriverMySQL:    mysqlSource(),
		model.DriverMariaDB:  mysqlSource(), // MariaDB speaks the MySQL protocol
		model.DriverSQLite:   sqliteSource(),
	}
}

// Run checks that the statement only reads and runs it on the source of the query's driver.
func (r Registry) Run(ctx context.Context, q Query) (Result, error) {
	source, ok := r[q.Driver]
	if !ok {
		return Result{}, fmt.Errorf("%w %q", ErrUnknownDriver, q.Driver)
	}
	if err := readonly.Check(q.Statement); err != nil {
		return Result{}, err
	}
	if q.Timeout <= 0 {
		q.Timeout = DefaultTimeout
	}
	if q.Limit <= 0 {
		q.Limit = model.DefaultDatasetLimit
	}
	parent := ctx
	ctx, cancel := context.WithTimeoutCause(parent, q.Timeout, ErrTimeout)
	defer cancel()
	start := time.Now()
	res, err := source.Query(ctx, q)
	res.Duration = time.Since(start)
	if err != nil && parent.Err() == nil && errors.Is(context.Cause(ctx), ErrTimeout) {
		// Drivers report a deadline in their own words; the caller needs to tell it from a failed query.
		return res, fmt.Errorf("%w after %s", ErrTimeout, q.Timeout)
	}
	return res, err
}
