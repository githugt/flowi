# 0001: Apache-2.0 for the library, AGPL-3.0 for the applications

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

Flow-I is free and open source and aims at outside users. The core engine should be easy for
others to embed (CI tools, internal test harnesses), while the end-user applications should
stay copyleft. AGPL on a library forces every program that imports it to be AGPL too, which
would block embedding.

## Decision

- `flowi-lib` is licensed **Apache-2.0**.
- `flowi-cli` and `flowi` (GUI) are licensed **AGPL-3.0-or-later**.
- Every `.go` file carries an SPDX line; a test enforces it.
- No contributor licence agreement for now; revisit before accepting large outside
  contributions if dual licensing ever becomes relevant.

## Consequences

- Anyone may embed `flowi-lib`, including in closed-source software.
- Modified versions of the CLI or GUI offered to others, including over a network, must
  publish their source.
- Code must not move from the CLI or GUI into the library without the author's agreement,
  because that would relicense it.
