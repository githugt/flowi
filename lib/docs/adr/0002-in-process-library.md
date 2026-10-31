# 0002: The CLI and GUI import the library in-process

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

The CLI and GUI must behave identically. Options were a shared library imported by both,
or a local engine server that both talk to over HTTP/gRPC.

## Decision

Both applications import `flowi-lib` directly and ship as single binaries. The library's
run API is shaped as an **event stream** (`NodeStarted`, `NodeFinished`, `RunFinished`, ...)
so it can later be wrapped by a long-running process (`flowi serve` / daemon) for cron
schedules, triggers and the replayer, without a rewrite.

## Consequences

- No daemon, ports or inter-process protocol to manage; fast startup; simple distribution.
- Every feature lands in the library first. The GUI never does anything the CLI cannot.
- A GUI run and a CLI run do not share a live process; scheduled and triggered runs need
  the later `serve` process.
