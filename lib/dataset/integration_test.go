// SPDX-License-Identifier: Apache-2.0

//go:build integration

// These tests talk to real PostgreSQL, MySQL and MariaDB servers: run `just test-integration`, which starts
// testdata/compose/docker-compose.yml. Addresses can be changed with FLOWI_TEST_POSTGRES, _MYSQL and _MARIADB.

package dataset_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/model"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type store struct {
	driver string
	server string
	// setup connects with the driver's own name and DSN to create the table; the test's source never writes.
	sqlDriver  string
	sqlDSN     string
	autoIncrem string
	// placeholder is the first placeholder of the store.
	placeholder string
}

func stores() []store {
	return []store{
		{driver: model.DriverPostgres, server: address("FLOWI_TEST_POSTGRES", "postgres://flowi:flowi@127.0.0.1:5433/flowi?sslmode=disable"),
			sqlDriver: "pgx", placeholder: "$1"},
		{driver: model.DriverMySQL, server: address("FLOWI_TEST_MYSQL", "mysql://flowi:flowi@127.0.0.1:3307/flowi"),
			sqlDriver: "mysql", placeholder: "?"},
		{driver: model.DriverMariaDB, server: address("FLOWI_TEST_MARIADB", "mariadb://flowi:flowi@127.0.0.1:3308/flowi"),
			sqlDriver: "mysql", placeholder: "?"},
	}
}

func address(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return fallback
}

// setupDSN is the server as its driver's own connection string.
func (s store) setupDSN(t *testing.T) string {
	t.Helper()
	if s.sqlDriver == "pgx" {
		return s.server
	}
	u, err := url.Parse(s.server)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	return fmt.Sprintf("%s:%s@tcp(%s)%s", u.User.Username(), pass, u.Host, u.Path)
}

// seed (re)creates the customers table and waits until the server accepts connections.
func (s store) seed(t *testing.T) {
	t.Helper()
	db, err := sql.Open(s.sqlDriver, s.setupDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deadline := time.Now().Add(90 * time.Second)
	for db.PingContext(t.Context()) != nil {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not come up: is the compose file running? (just test-integration)", s.driver)
		}
		time.Sleep(time.Second)
	}
	for _, statement := range []string{
		"DROP TABLE IF EXISTS customers",
		"CREATE TABLE customers (id INTEGER PRIMARY KEY, name VARCHAR(40) NOT NULL, balance DOUBLE PRECISION, price DECIMAL(10,2), note VARCHAR(40), born TIMESTAMP)",
		"INSERT INTO customers VALUES (1, 'Ada', 12.5, 19.90, NULL, '2024-03-05 10:20:30')",
		"INSERT INTO customers VALUES (2, 'Grace', 0, 5.00, 'vip', '2020-01-02 00:00:00')",
		"INSERT INTO customers VALUES (3, 'Linus', -3.25, 0.10, NULL, NULL)",
	} {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func (s store) run(t *testing.T, statement string, params ...string) (dataset.Result, error) {
	t.Helper()
	return dataset.Default().Run(context.Background(), dataset.Query{Driver: s.driver, Server: s.server, Statement: statement, Params: params, Timeout: 20 * time.Second})
}

func TestStores_should_return_the_same_typed_rows(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			s.seed(t)

			res, err := s.run(t, "SELECT id, name, balance, price, note FROM customers WHERE id = "+s.placeholder, "1")

			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rows) != 1 {
				t.Fatalf("rows %v", res.Rows)
			}
			row := res.Rows[0]
			if row[0] != int64(1) || row[1] != "Ada" || row[2] != 12.5 || row[3] != "19.90" && row[3] != "19.9" || row[4] != nil {
				t.Fatalf("row %#v", row)
			}
		})
	}
}

func TestStores_without_parameters_should_still_type_whole_numbers(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			s.seed(t)

			res, err := s.run(t, "SELECT id, balance FROM customers ORDER BY id")

			if err != nil {
				t.Fatal(err)
			}
			if res.Rows[0][0] != int64(1) || res.Rows[2][1] != -3.25 {
				t.Fatalf("rows %#v", res.Rows)
			}
		})
	}
}

func TestStores_should_write_times_as_rfc3339(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			s.seed(t)

			res, err := s.run(t, "SELECT born FROM customers WHERE id = "+s.placeholder, "1")

			if err != nil {
				t.Fatal(err)
			}
			got, _ := res.Rows[0][0].(string)
			if !strings.HasPrefix(strings.Replace(got, " ", "T", 1), "2024-03-05T10:20:30") {
				t.Fatalf("born %#v", res.Rows[0][0])
			}
		})
	}
}

func TestStores_should_not_write_even_when_the_guard_is_skipped(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			s.seed(t)
			source := dataset.Default()[s.driver]

			_, err := source.Query(context.Background(), dataset.Query{
				Driver: s.driver, Server: s.server, Statement: "DELETE FROM customers", Limit: 1, Timeout: 20 * time.Second,
			})
			after, countErr := s.run(t, "SELECT count(*) FROM customers")

			if err == nil {
				t.Fatal("the delete ran")
			}
			if countErr != nil || after.Rows[0][0] != int64(3) {
				t.Fatalf("rows left %v (%v)", after.Rows, countErr)
			}
		})
	}
}

func TestPostgres_should_refuse_a_cte_that_deletes(t *testing.T) {
	s := stores()[0]
	s.seed(t)

	_, err := s.run(t, "WITH gone AS (DELETE FROM customers RETURNING id) SELECT id FROM gone")
	after, _ := s.run(t, "SELECT count(*) FROM customers")

	if err == nil {
		t.Fatal("the delete ran")
	}
	if after.Rows[0][0] != int64(3) {
		t.Fatalf("rows left %v", after.Rows)
	}
}

func TestPostgres_should_not_let_a_query_switch_read_only_off(t *testing.T) {
	s := stores()[0]
	s.seed(t)

	_, err := s.run(t, "SELECT set_config('transaction_read_only', 'off', true)")
	after, _ := s.run(t, "SELECT count(*) FROM customers")

	if err == nil {
		t.Fatal("the transaction became writable")
	}
	if after.Rows[0][0] != int64(3) {
		t.Fatalf("rows left %v", after.Rows)
	}
}

func TestStores_should_stop_a_query_that_takes_too_long(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			s.seed(t)
			sleep := map[string]string{model.DriverPostgres: "SELECT pg_sleep(10)", model.DriverMySQL: "SELECT SLEEP(10)", model.DriverMariaDB: "SELECT SLEEP(10)"}[s.driver]
			start := time.Now()

			_, err := dataset.Default().Run(context.Background(), dataset.Query{Driver: s.driver, Server: s.server, Statement: sleep, Timeout: time.Second})

			if !errors.Is(err, dataset.ErrTimeout) || time.Since(start) > 5*time.Second {
				t.Fatalf("took %s, err %v", time.Since(start), err)
			}
		})
	}
}

func TestStores_should_not_leak_the_login_or_the_address_in_a_connection_error(t *testing.T) {
	for _, s := range stores() {
		t.Run(s.driver, func(t *testing.T) {
			u, _ := url.Parse(s.server)
			u.User = url.UserPassword("who-am-i-user", "wrong-password-xyz")
			u.Path = "/hidden-database-name"

			_, err := dataset.Default().Run(context.Background(), dataset.Query{Driver: s.driver, Server: u.String(), Statement: "SELECT 1", Timeout: 10 * time.Second})

			if err == nil {
				t.Fatal("a wrong login was accepted")
			}
			for _, secret := range []string{"wrong-password-xyz", "who-am-i-user", "hidden-database-name", u.Hostname(), u.Port()} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("%q is in the error: %v", secret, err)
				}
			}
		})
	}
}
