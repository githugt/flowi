// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"strings"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// ErrTemplatedServer is returned when a data file's server holds a {{template}}: its value is only known during a run.
var ErrTemplatedServer = errors.New("the server is written with a template, so its tables are known only when the request runs")

// DataFileTables lists the tables of the data files a request over files (CSV, Excel, Parquet) of collection would
// read: what the person writes after FROM. The server is a literal path, taken as a run takes it (relative to the
// collection, kept inside it unless datasets.allow_outside_collection allows more). A server with a template gives
// ErrTemplatedServer, and a database driver an error: only files can be listed without connecting anywhere.
func (w *Workspace) DataFileTables(ctx context.Context, collection model.ID, driver, server string) ([]dataset.TableInfo, error) {
	if strings.Contains(server, "{{") {
		return nil, ErrTemplatedServer
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	outside, _ := w.Settings(Scope{Collection: collection}).Get("datasets.allow_outside_collection")
	allow, _ := outside.Value.(bool)
	return dataset.Default().Tables(ctx, dataset.Query{Driver: driver, Server: server, Root: c.Root, AllowOutsideRoot: allow})
}
