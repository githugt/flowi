// SPDX-License-Identifier: Apache-2.0

package dataset_test

import (
	"context"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/dataset"
	"github.com/githugt/flowi-lib/model"
)

func TestRegistry_Run_Given_a_bad_server_should_explain_without_connecting(t *testing.T) {
	cases := []struct {
		name   string
		driver string
		server string
		want   string
	}{
		{"mysql option that would set a session variable", model.DriverMySQL, "mysql://u:p@127.0.0.1:1/db?sql_mode=ANSI", "not supported"},
		{"mysql without a scheme", model.DriverMySQL, "u:p@tcp(127.0.0.1:3306)/db", "mysql://user:password@host"},
		{"mariadb with a postgres url", model.DriverMariaDB, "postgres://u:p@127.0.0.1/db", "mysql://user:password@host"},
		{"postgres that is not a url", model.DriverPostgres, "postgres://u:p@host:notaport/db", "postgres://user:password@host"},
		{"sqlite with no path", model.DriverSQLite, "  ", "path of the SQLite file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := dataset.Default().Run(context.Background(), dataset.Query{Driver: c.driver, Server: c.server, Statement: "SELECT 1"})

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want it to mention %q", err, c.want)
			}
		})
	}
}
