# 0020: gRPC is a request kind of its own, called without generated code

- **Status:** Accepted
- **Date:** 2026-10-16
- **Builds on:** 0017 (flows are Arazzo documents), 0018 (async requests)

## Context

Flow-I defines what requests are (HTTP, async) with hooks around each. The owner chose gRPC as the RPC kind to add
next: calling and testing existing services the way grpcurl does, from `.proto` files or from the server's
reflection service, with unary and server-streaming calls first.

## Decision

- **A `grpc:` block on a request version** (`model.GRPC`: server, method, JSON message, `proto` files, `import` roots,
  and `receive` options for a stream). It is its own block, not an async protocol: a call has a method, a typed message
  and a status, and no channel or binding. A version has at most one of `grpc` and `async` (`FLW-GRPC-001`). The
  version's headers are the call's metadata.
- **`flowi-lib/grpcx` makes the call with no generated code.** The schema is compiled from `.proto` files in pure Go
  (`protocompile`, import roots relative to the collection, which a validation rule keeps inside it) or fetched from
  the server (reflection v1, then the older v1alpha). Messages are JSON converted with `protojson` and `dynamicpb`.
  One call per run: connect, call, close. Client and bidirectional streaming are refused with a clear message.
- **A non-OK gRPC status is a result, not an error.** The record carries the code name (`NOT_FOUND`), message and
  trailers; `code` is an expression name next to `body`, `headers`, `trailers`, `messages`, `received` and `outcome`.
  An assertion can expect any code, and the seeded default hook asserts `code == "OK"`. `status` is not defined. Only
  what stops a call being made (schema, body, a stream wait that ran out) fails the run: `FLW-GRPC-002..004`.
- **The record is message-based like an async one** (`Kind: grpc`, `Outcome`, `Messages`, `HasMessages()`), so the
  messages list, live progress and the per-outcome saved file work unchanged. Responses, unary or streamed, learn
  `<version>-message.json`; the request message learns `<version>-request.json`.
- **A flow step for a gRPC request is a custom step.** Arazzo has no gRPC source type, so the call lives in
  `x-flowi-flow` and the standard step is the existing placeholder custom step; `FlowCompatibility` reports it as
  `FLW-ARAZZO-003`. No description is generated for gRPC requests, and the OpenAPI description leaves them out.
- **A bare `host:port` is plaintext for localhost and TLS for any other host**; a `grpc://` or `grpcs://` scheme
  always decides, and the request bar shows the scheme the call will use.

## Consequences

- New dependencies in `flowi-lib`: `google.golang.org/grpc` (already indirect), `bufbuild/protocompile`. Both stay
  behind `grpcx` (plus the tests, which also use grpc's reflection server).
- Everything is tested in the normal suite against an in-process gRPC server over a real socket (reflection v1 and
  v1alpha, TLS, streams, deadlines, metadata); there are no integration tests to run.
- Not done, by decision (see `docs/design/backlog.md`): client and bidirectional streaming, `-bin` metadata, client
  certificates beyond the existing TLS settings, gRPC-Web, a generated description and a standard Arazzo step.
