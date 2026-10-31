// SPDX-License-Identifier: Apache-2.0

package model

// Dataset is what a dataset request version does: it runs one read-only query against a data store and keeps the
// rows it returns.
type Dataset struct {
	// Driver is the store: postgres, mysql, mariadb or sqlite.
	Driver string
	// Server is the connection string, templated: a postgres:// or mysql:// URL, or for sqlite a file path
	// relative to the collection.
	Server string
	// Query is one SELECT (or WITH, VALUES, SHOW, EXPLAIN, DESCRIBE) statement. It is not templated: values go in
	// Params, which are sent to the store separately from the statement, so a value can never change what the
	// statement means.
	Query string
	// Params are the values of the query's placeholders, in order, templated. Placeholders are $1, $2 … for postgres
	// and sqlite, and ? for mysql, mariadb and sqlite.
	Params []string
	// Limit is the most rows kept; 0 means DefaultDatasetLimit. More rows than that are cut and the result says so.
	Limit int
}

// DefaultDatasetLimit is how many rows a dataset request keeps when it sets no limit.
const DefaultDatasetLimit = 1000

// The dataset drivers.
const (
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"
	DriverMariaDB  = "mariadb"
	DriverSQLite   = "sqlite"
)

// DatasetDrivers lists the drivers in the order a picker shows them.
func DatasetDrivers() []string {
	return []string{DriverPostgres, DriverMySQL, DriverMariaDB, DriverSQLite}
}
