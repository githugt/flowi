# 0007: How a request is rendered, and how API hooks run around it

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

A request mixes templates (`{{env.base_url}}`, `{{email}}`, `{{$uuid-v4}}`), secrets and
API hooks that run before and after it. Pre-hooks must be able to set values the
request uses (an auth hook that fetches a token), and also to read the finished request
(a hook that signs the body). The template language is `expr-lang/expr`, which does not
know the notation the file format uses (`800ms`, `$first-name`, `x != empty`).

## Decision

- **Names.** A bare name (`{{email}}`) is looked up in the request's `with` values, then
  `vars`, `inputs` and `env`. Namespaced names (`env.x`, `vars.x`, `nodes.x`) always
  work. Secrets are only `secret.x`.
- **Strict.** A reference that does not resolve is an error naming it, unless guarded
  with `??` or `?.`. Expressions are compiled with each constant reference chain replaced
  by a synthetic variable resolved before evaluation, which also records whether a
  secret was read.
- **Notation.** Before compiling, `$name-with-hyphens(args)` becomes a generator call,
  number+unit (`2s`, `800ms`) becomes `duration("…")`, and comparisons with `empty`
  match null, `""`, and empty lists and maps. String literals are never rewritten.
- **JSON bodies** are rendered JSON-aware: a placeholder that is a whole string literal
  is replaced by the encoded value (`null` for nil), one inside a string is escaped,
  one outside a string is JSON-encoded. Values cannot break or inject JSON.
- **Order** for one request: pre-hooks (collection → folders → request → version), the
  strict render with pre-hook overrides applied on top, send, the version's `extract:`
  and `assert:`, post-hooks (exact reverse order), save. Post-hooks run even when an
  assertion failed; a pre-hook failure means nothing is sent.
- **Pre-hooks read a best-effort render** of the request (values not set yet read as
  null). Generators are memoised per field and call position across renders, so what a
  hook reads is byte-for-byte what is sent.
- **Hook flows** run serially and share the hooked request's variables. *(Since ADR 0009
  they run on the flow scheduler and support every node type; sharing the request's
  variables is unchanged.)* Requests inside
  them run without their own hooks unless `hooks.nested` is on, and never deeper than
  `hooks.max_depth`. Validation reports a hook-flow cycle (`FLW-CYC-003`) only when every
  request on it runs nested hooks, because only then can it recurse.

## Consequences

- Typos in templates fail loudly, before anything is sent.
- The common "collection pre-hook fetches a token from the same collection" pattern is
  valid without special cases.
- Until the M3 scheduler exists, hook flows support `request`, `set`, `extract`, `assert`
  and `log` nodes only (`FLW-RUN-003` otherwise).
