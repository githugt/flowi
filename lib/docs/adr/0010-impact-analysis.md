# 0010: Impact analysis reads templates statically and checks them against samples

- **Status:** Accepted
- **Date:** 2026-10-08

## Context

Changing an API should tell you what it breaks: which flows and hooks read a field
that is going away, and which callers do not send a new required input. Running every
flow to find out is slow, needs live services, and only covers the paths a run
happens to take. Request versions, cross-collection references and hooks at four
levels mean "who uses this" is not obvious from the files.

## Decision

- **Consumers are found statically**, per request version, across every mounted
  collection:
  - every `request` node (in flows, hook flows and inline hook steps) that resolves to
    the version without run-time preferences (pinned → flow `prefer_version` → default
    → highest; references to other collections go through `depends_on`, by name or id);
  - every post-hook step that applies to the version (collection, folders, request,
    version). Pre-hooks read no response, so they are not consumers;
  - the version's own `extract:` and `assert:`.
- **Reads come from templates.** For a request node, every template in the same root
  that reads `nodes.<id>.…` counts: `body.<path>`, `headers.<name>`, or an extract name,
  which maps through the version's extraction (JSONPath, header, cookie or regex). For
  hooks, `response.body…`, `response.headers…`, `response.cookies…` and single-form
  `extract` nodes count. `status`, `time` and `vars` rely on nothing a contract can
  lack. The template walkers live in `internal/sites`, shared with `validate`, so both
  agree on what a template reads and where it is written.
- **Inputs:** every bare name, `with.<name>` and `vars.<name>` a version's templates read
  must be filled the way the engine resolves them: a request runs with its own variables
  and no flow inputs, so a bare name is filled by the node's `with`, a variable the
  request's pre-hooks write, or a non-secret environment key; `with.<name>` only by
  `with`; `vars.<name>` only by pre-hooks. An unfilled input is broken.
- **Guarded reads rely on nothing.** A read under `??` or `?.` is nil when missing, not an
  error (design §6.2), so it is neither a field the contract must contain nor an input.
- **Contracts are samples.** Pinned examples for the version (and unversioned ones;
  `send --pin` now records the version), else the last saved response. OpenAPI schemas
  join as the best source in M5. Reads are checked against the 2xx samples when there
  are any: compatible if a sample has the field, unknown if none can tell (no samples, an
  empty array, `null`, or an example without headers), broken otherwise. A consumer is as
  bad as its worst read.
- **Two outputs.** `Impact` reports one request; `validate` adds broken reads as
  warnings (`FLW-CONTRACT-001/002`) at the reading template, so editors can mark them.
  Warnings never fail validation.

## Consequences

- Answers come from files alone: no network, no runs, deterministic in CI
  (`flowi impact` exits 1 when anything is broken).
- Samples show shape, not every case: an optional field present in the example is
  "compatible" even if the API sometimes omits it. Pin more examples (or add a schema in
  M5) to tighten this.
- Not traced yet: reads through a container's `results` (`nodes.users.results…`), values
  computed at run time (`nodes.login.body[vars.key]` stops at `body`), and cookies a
  request finds in the jar from an earlier response (judged against this response only).
- `validate` reads example files on every call; `send` and `run` check only structure, so
  they do not pay for the contract phase. One analysis builds its roots, callers and
  parsed templates once (500 requests and 8,000 nodes take about 0.15s).
