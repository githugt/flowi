# 0004: Dependency injection container only in the applications

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

The applications wire many services (workspace, engine, logging, later the vault and
servers) and must shut them down in order. Embedders of the library should not be forced
to adopt a DI container.

## Decision

- `flowi-cli` and `flowi` use **`samber/do/v2`** in their composition roots.
- They always create their own injector (`do.New()`); the package-level default injector
  is never used, so there is no global state.
- Application shutdown goes through the injector, which releases services in reverse
  order within a bounded timeout.
- `flowi-lib` exposes plain constructors with explicit dependencies and does not import
  `samber/do`.

## Consequences

- Embedders use the library with ordinary Go code.
- Wiring and shutdown order live in one place per application.
