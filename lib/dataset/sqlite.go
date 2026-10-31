// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // the pure Go SQLite driver: no cgo
)

// sqliteSource opens the file read-only (mode=ro: a missing file is an error, never a new empty database) and sets
// query_only as well, so a statement that would write fails even if the file were opened writable.
func sqliteSource() Source {
	return sqlSource{open: openSQLite}
}

// sqliteBusyWaitMillis is how long a locked database is waited for before the query fails.
const sqliteBusyWaitMillis = 5000

func openSQLite(q Query) (*sql.DB, error) {
	path, err := sqlitePath(q)
	if err != nil {
		return nil, err
	}
	options := url.Values{}
	options.Set("mode", "ro")
	options.Add("_pragma", "query_only(1)")
	options.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", sqliteBusyWaitMillis))
	return sql.Open("sqlite", (&url.URL{Scheme: "file", Opaque: filepath.ToSlash(path), RawQuery: options.Encode()}).String())
}

// sqlitePath is the file to open: the server as written, or under the collection root when it is relative.
func sqlitePath(q Query) (string, error) {
	server := strings.TrimPrefix(q.Server, "sqlite://")
	if strings.TrimSpace(server) == "" {
		return "", errors.New("the server is empty: give the path of the SQLite file")
	}
	if filepath.IsAbs(server) || q.Root == "" {
		return server, nil
	}
	return filepath.Join(q.Root, server), nil
}
