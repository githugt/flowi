// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// postgresSource uses pgx, the pure Go PostgreSQL driver. Besides the read-only transaction every query runs in, the
// session itself is started read-only, so no statement of it can write.
func postgresSource() Source {
	return sqlSource{open: openPostgres, readOnly: true}
}

func openPostgres(q Query) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(q.Server)
	if err != nil {
		return nil, fmt.Errorf("the postgres server is wrong: %w", errHidingPassword(err))
	}
	cfg.ConnectTimeout = q.Timeout
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	return stdlib.OpenDB(*cfg), nil
}

// errHidingPassword is the error without pgx's text, which quotes the connection string and so the password.
func errHidingPassword(error) error {
	return errors.New("it is not a valid URL like postgres://user:password@host:5432/database")
}
