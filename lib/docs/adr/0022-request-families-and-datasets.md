# 0022: Requests belong to a family; a dataset query is the fourth

- **Status:** Accepted
- **Date:** 2026-10-29
- **Builds on:** 0018 (async requests), 0020 (gRPC requests)

## Context

Flowi had three kinds of request, each found by testing a pointer on the version (`Async != nil`, `GRPC != nil`) in a
dozen places. The owner asked for the reading of SQL stores (PostgreSQL, SQLite, MySQL and MariaDB), and for the
kinds to be named so that more can follow: HTTP, RPC, messaging and datasets. Local files such as CSV may come later;
they are not built.

## Decision

- **`model.Family`** names the four families: `http`, `rpc` (gRPC), `messaging` (every async protocol) and `dataset`.
  `RequestVersion.Family()` says which one a version is: the block that is set decides, and none set is HTTP.
  `RequestVersion.Exchange()` says which directions it has (`send`, `receive` or `send_receive`): only messaging
  varies, by whether it publishes, subscribes or does request and reply; a query sends and receives the rows.
- **The engine runs a family through one interface** (`engine.exchange`: `send`, `view`, `preview`), chosen once by
  `execution.exchange()`. It replaces the branches that were in `sendAndCheck`, `requestView` and `PreviewSend`. A new
  family is one more implementation and a `Family` value. Not every place moved: the YAML codec, the template-site
  lists and the validators still read each block by name, because each reads a different shape. The places that only
  asked "is this not HTTP?" (method validation, overlay fields, the seed kind, the Arazzo description) now ask `Family()`.
- **A `dataset:` block on a request version** (`model.Dataset`: `driver`, `server`, `query`, `params`, `limit`). The
  drivers are `postgres`, `mysql`, `mariadb` and `sqlite`. `dataset.Registry` maps a driver to a `dataset.Source`, so a
  store of another kind is one more `Source`.
- **Pure Go drivers only, no cgo** (the lib and the CLI build with `CGO_ENABLED=0`): `jackc/pgx` for PostgreSQL,
  `go-sql-driver/mysql` for MySQL and MariaDB (the same protocol), `modernc.org/sqlite` for SQLite. They are the
  drivers widely used in production Go for those stores (`lib/pq` is in maintenance mode and `mattn/go-sqlite3` needs
  cgo). `modernc.org/sqlite` was chosen over the WebAssembly-based `ncruces/go-sqlite3` because it has the longer track
  record. Neither the star counts nor the effect on the binary's size were measured.
- **A query only reads, guarded twice.** `dataset/readonly` checks the text before anything is sent: one statement
  that starts with `SELECT`, `WITH`, `VALUES`, `TABLE`, `SHOW`, `EXPLAIN` or `DESCRIBE`, without `INTO` (a list of what
  is allowed, not of what is refused). The store enforces the second guard: PostgreSQL, MySQL and MariaDB run the
  query in a read-only transaction (PostgreSQL's session is also started read-only), SQLite opens the file `mode=ro`
  with `query_only` on and never creates a missing file. A `WITH` that deletes passes the first guard and is stopped by
  the second (a PostgreSQL test checks that; the other stores are tested with a plain `DELETE` sent around the first
  guard). The strongest guard is still a database user that can only read, which the Query tab tells people to use.
- **The query is not templated; values are `params`.** They are sent apart from the statement, so a value cannot change
  what it means (`FLW-DATA-001` when the query contains `{{`). Params are text, templated, and the store converts them
  (a YAML `- 42` is the text `"42"`). The server and the params are templated and may use secrets; the record masks them.
- **The result is one JSON object**: `columns`, `rows` (objects by column name; a repeated name gets a suffix, `a_2`),
  `row_count` and `truncated`. At most `limit` rows are kept (default 1000). Values are the same in every store: whole
  numbers as integers, `DECIMAL` as text (a float would round it), times as RFC 3339, binary as base64. Extraction
  reads it as `$.rows[0].name`; assertions and hooks see `rows`, `columns`, `row_count`, `truncated`, `outcome` and
  `body`. `status` is not defined; the seeded default hook asserts `outcome == "ok"`.
- **Failures are diagnostics**: `FLW-DATA-001` (the request is wrong: driver, empty or refused query),
  `FLW-DATA-002` (the store said no) and `FLW-DATA-003` (the timeout). The record's kind is `dataset`, its method is
  `SQL`, and `Record.Rows()` reads the count back from a record read from a file as well as a fresh one.
- **No separate `sql` flow step.** A flow's request step runs any request, so a dataset query in a flow is a request
  step. In an exported Arazzo document it is a custom step (`Operation.Custom`, which replaces the gRPC-only flag).

## Not done

- No write mode, and no connection kept between steps (each run connects, queries and closes, as gRPC does).
- MySQL accepts only the `tls` option in its URL: any other would become a session variable.
- Arazzo 1.1 `channelPath` and `correlationId` are not generated. Every channel Flowi describes already has an
  operation, which the Arazzo examples address with `operationId`; `correlationId` names a message field and cannot
  express Flowi's `receive.match` expression. A third-party document that uses `channelPath` is imported as a note that
  says so (it was never dropped silently).
- Files (CSV and the like) as a fifth family or a `Source`: later.

## Verified against

PostgreSQL 17, MySQL 8.4 and MariaDB 11.4 containers (`just test-integration`, `dataset/integration_test.go`), SQLite
files in the unit tests, and the window with a SQLite file. Breaking the read-only transaction for any of the three
servers makes the write tests fail.
