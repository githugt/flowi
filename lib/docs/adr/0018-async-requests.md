# 0018: Async requests are a kind of request version; transports are pluggable and one-shot

- **Status:** Accepted
- **Date:** 2026-10-15
- **Builds on:** 0017 (flows are Arazzo documents)

## Context

Flow-I is moving from "an HTTP tool" to defining what requests are (HTTP, async, later gRPC)
with hooks around each, and stitching them with Arazzo. Arazzo 1.1 can call AsyncAPI operations
with `action: send|receive`. The owner asked for WebSocket, SSE and MQTT first "and any more you
need": the engine, record, hooks, schemas, descriptions and UI all had to learn a second kind of
request without disturbing the first.

## Decision

- **An async request is a request version with an `async:` block** (`model.Async`): protocol,
  server, channel, optional `send` and `receive`, protocol `binding` options. `Async == nil` is
  HTTP, so every existing file stays valid. Send only, receive only, or both (request and reply:
  subscribe first, then send, then collect).
- **`flowi-lib/transport` is one interface, `Run(ctx, Operation) (Result, error)`, with one pure-Go
  subpackage per protocol** (ws, sse, mqtt 5, kafka, amqp 0-9-1, nats core and JetStream). Each
  call connects, does its work and closes: nothing is kept between flow steps. A dependency is
  isolated to its package. A message the run published itself is recognised and skipped
  (MQTT no-local, NATS NoEcho, and the stream sequence, Kafka offset or AMQP message id elsewhere).
- **The engine branches in `sendAndCheck`.** An async run produces a `Record` with `Kind: async`,
  `Outcome` (ok, timeout, error) and `Messages`; `Status` stays 0. Hooks, `extract` and `assert`
  run unchanged through a synthesized response; expressions see `messages`, `all_messages`, `body`,
  `headers`, `received`, `outcome`. (`count` is a built-in function of the expression language,
  hence `received`.) A timeout keeps the partial messages and is `FLW-ASYNC-005`.
- **Schemas reuse the HTTP machinery**: the sent payload learns `<version>-request.json`, received
  messages `<version>-message.json` (`FLW-SCHEMA-004` for a mismatch).
- **A second generated, committed description**, `descriptions/requests.asyncapi.yaml`
  (AsyncAPI 3.0), exists while the collection has an async request: one operation per version
  (send, receive, or send with a reply). Credentials (`username`, `password`, `token`, anything
  that reads a secret) are never written to it.
- **Flows call async requests with Arazzo 1.1 steps**: `operationId` in the `flowi-async` source,
  `action`, and `timeout` (milliseconds) from the receive timeout. A reply and a match filter have
  no place in a step, so Flow-I keeps them (the flow's own data is whole in `x-flowi-flow`) and
  warns that other tools do not see them (`FLW-ARAZZO-003`, from `FlowCompatibility`).

## Consequences

- Five more dependencies in `flowi-lib` (coder/websocket, paho.golang, franz-go, amqp091-go,
  nats.go) and three test-only ones (mochi-mqtt, nats-server, franz-go kfake). Each is behind its
  transport package.
- WebSocket, SSE, MQTT, NATS (with JetStream) and Kafka are tested in the normal suite against
  in-process brokers. AMQP has no embeddable broker: it is covered by unit tests of its mapping
  and by `just test-integration` (podman compose, else docker compose) against RabbitMQ.
- Not done, by decision: MQTT 3.1.1, AMQP 1.0, connections kept open between steps (see
  `docs/design/backlog.md`).
