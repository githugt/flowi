# 0024: A GraphQL operation is the body of an HTTP request

- **Status:** Accepted
- **Date:** 2026-11-26
- **Builds on:** 0022 (request families and datasets)

## Context

GraphQL was half there. The importers for Postman, Bruno, Thunder Client and Insomnia turned a GraphQL body into a
JSON body with the query inside a string, the engine warned when an answer carried an `errors` list
(`requests.graphql_errors`, default `warn`), and the app's flow cards knew a `graphql` kind that nothing produced. A
GraphQL request could not be written as one, had no query or variables editor, and a 200 answer with errors passed.

## Decision

- **A body kind, not a block.** `model.Body.GraphQL` (`query`, `variables`, `operation`) sits beside `json`, `text`,
  `file`, `form` and `multipart`, and a version that has it is `model.FamilyGraphQL`. It is not a block like `grpc:`
  because GraphQL over HTTP *is* HTTP: the URL, method, headers, auth, cookies, redirects, retries, the response panel,
  extracts, asserts, hooks, flows, `serve` and every snippet work unchanged. `Family.OverHTTP()` is true for both
  families, and the places that asked "is this HTTP?" ask that.
- **On the wire.** POST sends `{"query", "operationName", "variables"}` as JSON (`Content-Type: application/json`).
  GET, for a query, sends the same three as URL parameters. A mutation with GET is refused: the GraphQL over HTTP
  specification forbids it. `Accept` is `application/graphql-response+json, application/json;q=0.9` unless the
  request names its own.
- **The query is not templated; the variables are.** A value in the document could change what it means (the same
  reason a SQL query takes params), so `{{` in a query is `FLW-GQL-001`. `variables` is a JSON object as text and is
  rendered like a JSON body, so `{"id": "{{env.user_id}}"}` works and secrets in it are masked.
- **Errors fail the step.** `requests.graphql_errors` is now `auto`: a request with a GraphQL body fails
  (`FLW-RUN-030`) when the answer has a non-empty `errors` list, even with status 200, and an HTTP request whose JSON
  merely looks like a query only warns, as before. `warn`, `fail` and `ignore` still force one choice. A request whose
  `assert:` lines mention `body.errors` (`response.body.errors`, or `body.errors` on an event) handles them itself, so a test that expects an error passes. Partial
  data with errors still fails.
- **`graphqlx` reads the document.** It lists the operations (kind and name) and picks the one a request runs, with a
  scanner that skips comments, strings and block strings. It does not validate; the server does. It decides the GET
  check, validation, the app's badge, and whether a flow step writes (`flow writes`: a query sent with POST is not a
  write, a mutation is).
- **Import.** The four importers make a GraphQL body. A query that holds `{{` (Postman and Bruno variables inside the
  query) stays a templated JSON body and the report says so. Insomnia's `operationName` is kept. Arazzo and the
  generated OpenAPI describe the request as a POST with the GraphQL-over-HTTP body schema; importing that back gives a
  plain JSON request, because the document does not say it is GraphQL.
- **Subscriptions are a stream over a WebSocket.** The same body, with an operation that is a subscription, does not
  go out as a POST: the engine sends it with the `graphqlws` transport (`transport/graphqlws`, the
  `graphql-transport-ws` protocol: `connection_init`, `connection_ack`, `subscribe`, then the payload of each `next`
  message is kept, and `complete` or a normal close ends the stream without failing it). The URL is the same address
  over `ws`/`wss`, the version's headers go with the handshake, and `body.graphql.receive` (`count`, `timeout`,
  `match`, the same three as a message request) says how much to wait for: the first event within 30 seconds by
  default. The result is an async record (the subscribe message and the events), so extracts, asserts, message hooks,
  the Messages tab and `flowi send` work as for any message request, and an event with an `errors` list fails the
  step like a query's answer. A server's `error` message or a close code (4401 and the rest) is `FLW-GQL-002` with
  the server's reason. The transport is registered under `graphql-ws`, which is not one of the async protocols a
  request can name: a subscription is a GraphQL body, not a message request.
- **The app.** GraphQL is a choice in the type menu (`TypeGraphQL`) and in the Body type picker. The Body tab shows a
  query editor, a variables editor, the kind of operation, and an operation picker when the query has several. The
  sidebar badge is `GQL`. The protected-environment confirmation treats a query as a read and a mutation as a write.

## Not done

- **The older `subscriptions-transport-ws` protocol** (`start`/`data`, `graphql-ws` as the subprotocol name), which
  Apollo Server 2 and some Hasura setups still speak. A server that selects another subprotocol is refused with a
  message naming `graphql-transport-ws`.
- **Sign-in inside the connection.** Credentials go in a header with the handshake; a `connection_init` payload (the
  usual place for a token in browsers) cannot be set, and an `auth:` block on a subscription is refused. Server-sent
  events and HTTP multipart (`@defer`, `@stream`, subscriptions over SSE) are not read either.
- **A subscription kept open between flow steps** (the `open`/`over` connections of message steps): the transport has
  no `Opener`, so every step opens, collects and closes.
- **Introspection and SDL import.** No schema is read, so there is no autocomplete, no explorer and no requests made
  from a schema.
- **A "New GraphQL request" menu entry.** A GraphQL request is made from the type menu or the body type of a new one.
- **Persisted queries, batching, file upload (multipart) and `@defer`/`@stream`.**
- **Query syntax colouring.** The query editor is plain text with line numbers.

## Verified against

The engine's test server and the demo server (`POST` and `GET /graphql`, and a `graphql-transport-ws` upgrade on the
same path), the CLI scripts (`send`, `run`), and the app's development server in a browser. Not a real GraphQL server
(Apollo, Hasura, GraphQL Yoga) and not the native window.
