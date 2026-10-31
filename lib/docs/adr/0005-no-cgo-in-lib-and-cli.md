# 0005: No cgo in the library or the CLI

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

The project is Go-only by preference. cgo complicates cross-compilation and static
binaries. Every mature Go desktop toolkit still needs cgo for OS windowing and graphics,
so the GUI cannot avoid it.

## Decision

- `flowi-lib` and `flowi-cli` must build with `CGO_ENABLED=0` on macOS, Linux and Windows.
  CI enforces this.
- New dependencies for these two modules must be cgo-free on all three platforms.
- Only the GUI uses cgo, and only through its toolkit. GUI code itself is Go.
- The race detector needs cgo at test time; that is allowed.

## Consequences

- The CLI ships as a single static binary and cross-compiles freely.
- Some dependencies (for example keychain or compression libraries) must be chosen with
  this rule in mind.
