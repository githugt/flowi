// SPDX-License-Identifier: Apache-2.0

package readonly_test

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/lib/dataset/readonly"
)

func TestCheck_should_accept_statements_that_only_read(t *testing.T) {
	for _, statement := range []string{
		"SELECT 1",
		"select * from users where id = $1",
		"  \n\tSELECT 1;",
		"-- who is in\nSELECT name FROM users",
		"/* all of them */ SELECT * FROM users",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"VALUES (1), (2)",
		"SHOW TABLES",
		"EXPLAIN SELECT 1",
		"DESCRIBE users",
		"SELECT ';' AS semicolon",
		"SELECT 'it''s' AS quote",
		`SELECT "a;b" FROM t`,
		"SELECT 1 -- trailing; comment",
	} {
		t.Run(statement, func(t *testing.T) {
			err := readonly.Check(statement)

			if err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}

func TestCheck_should_refuse_anything_that_could_change_data(t *testing.T) {
	cases := []struct {
		statement string
		want      error
	}{
		{"INSERT INTO users VALUES (1)", readonly.ErrNotReadOnly},
		{"update users set name = 'x'", readonly.ErrNotReadOnly},
		{"DELETE FROM users", readonly.ErrNotReadOnly},
		{"DROP TABLE users", readonly.ErrNotReadOnly},
		{"CREATE TABLE t (a int)", readonly.ErrNotReadOnly},
		{"PRAGMA writable_schema = 1", readonly.ErrNotReadOnly},
		{"CALL cleanup()", readonly.ErrNotReadOnly},
		{"/* SELECT */ DELETE FROM users", readonly.ErrNotReadOnly},
		{"SELECT 1; DROP TABLE users", readonly.ErrManyStatements},
		{"SELECT 1;\nSELECT 2", readonly.ErrManyStatements},
		{"", readonly.ErrEmptyStatement},
		{"   -- nothing\n ", readonly.ErrEmptyStatement},
		{"SELECT 1 /* never closed", readonly.ErrNotReadOnly},
	}
	for _, c := range cases {
		t.Run(c.statement, func(t *testing.T) {
			err := readonly.Check(c.statement)

			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}
