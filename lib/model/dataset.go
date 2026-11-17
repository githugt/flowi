// SPDX-License-Identifier: Apache-2.0

package model

import "slices"

// Dataset is what a dataset request version does: it runs one read-only query against a data store and keeps the
// rows it returns.
type Dataset struct {
	// Driver is the database: postgres, mysql, mariadb or sqlite. Data files are a DataFile, not a Dataset.
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
	// Writes lets this query change data: the read-only guards are off for it. Off is the default; the setting
	// datasets.allow_writes does the same for every query.
	Writes bool
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

// The formats of a data file query.
const (
	FormatCSV     = "csv"     // SQL over a CSV file, or a folder of them
	FormatXLSX    = "xlsx"    // SQL over an Excel workbook (a table for each sheet), or a folder of them
	FormatParquet = "parquet" // SQL over a Parquet file, or a folder of them
)

// DatasetDrivers lists the databases in the order a picker shows them.
func DatasetDrivers() []string {
	return []string{DriverPostgres, DriverMySQL, DriverMariaDB, DriverSQLite}
}

// DataFileFormats lists the formats of a data file query in the order a picker shows them.
func DataFileFormats() []string { return []string{FormatCSV, FormatXLSX, FormatParquet} }

// DataFile is what a data file request version does: it runs one read-only query over a file, or a folder of files,
// of the collection and keeps the rows it returns. It has no server and no login; a database is a Dataset.
type DataFile struct {
	// Format is csv, xlsx or parquet.
	Format string
	// Path is a file or a folder relative to the collection, templated. A folder holds one table for each file of the
	// format; a workbook has one table for each sheet.
	Path string
	// Query is one SELECT (or WITH, VALUES, DESCRIBE …) statement; like a database query it is not templated.
	Query string
	// Params are the values of the query's placeholders, in order, templated.
	Params []string
	// Limit is the most rows kept; 0 means DefaultDatasetLimit.
	Limit int
	// Writes lets this query change the data: a statement that does is run on the loaded copy and the changed tables
	// are written back to the file. Off is the default; the setting datasets.allow_writes does the same for every
	// query, and datasets.read_only forbids it for every query.
	Writes bool
}

// AsDataset is the query as the SQL runner takes it: the format is the driver and the path is the server.
func (f DataFile) AsDataset() Dataset {
	return Dataset{Driver: f.Format, Server: f.Path, Query: f.Query, Params: f.Params, Limit: f.Limit, Writes: f.Writes}
}

// DataFileOf is the data file a SQL query of a file format stands for.
func DataFileOf(d Dataset) DataFile {
	return DataFile{Format: d.Driver, Path: d.Server, Query: d.Query, Params: d.Params, Limit: d.Limit, Writes: d.Writes}
}

// IsDataFormat reports whether driver names a data file format.
func IsDataFormat(driver string) bool { return slices.Contains(DataFileFormats(), driver) }
