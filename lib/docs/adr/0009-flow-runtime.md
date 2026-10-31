# 0009: One runtime for flows, containers, subflows and hook flows

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

Flows can nest containers (`parallel`, `batch`, `run_till`, `condition`), call
subflows, and run requests whose API hooks are flows themselves. If each of these ran
its own loop, they would disagree on scheduling, scopes, limits and cancellation, and
every new node type would need to know about all of them.

## Decision

- **Nodes describe; the engine runs.** Each node type is one file in `internal/nodes`,
  registered with a parameter spec (`nodes.Specs()`). A node never starts goroutines,
  opens connections or reads the clock: it asks `nodes.Runtime` to run *lanes* (branches,
  iterations), send a request, run a subflow, make an HTTP call, sleep, or write a file.
  The specs are also the source of the validator's parameter checks and of the generated
  [node reference](../reference/nodes.md).
- **One scheduler.** A node list runs serially in list order. A node waits for its `needs`
  (the previous node by default) and, unless it is `concurrent: true`, for every earlier
  node. One coordinator per list owns the state; node goroutines live in an `errgroup`
  and report completions on a channel. `on_error` (`fail`, `continue`, `goto:<later
  sibling>`) and the flow's `on_branch_error` (`fail_fast`, `finish_running`,
  `continue_all`) are applied by the coordinator. Hook flows and inline hook steps run
  through the same scheduler, in a frame whose variables are the hooked request's.
- **Frames are scopes.** `batch` iterations get their own variables, `nodes.<id>` and
  cookie jar; `run_till` iterations their own `nodes.<id>`; `parallel` branches and
  `condition` branches write to the enclosing scope (validation rejects two concurrent
  writers of one variable); subflows have their own variables and nodes, and return
  their declared `outputs`.
- **Limits cannot deadlock.** The run-wide `max_concurrency` caps lanes in flight per
  container and HTTP exchanges in flight across the run; an HTTP slot is held for one
  attempt only, and lane slots are per container. `rate_limit` paces every attempt.
- **Events are the record.** Nodes, hooks and attempts emit events with instance paths
  (`users[2]/create`, `verify/pre:request:0/fetch`). The run's report — saved as JSON
  for every run and written as JUnit, Markdown or text — is built from the same events,
  so the live view, the report and digests agree.
- **Cancellation.** Cancelling the context stops new work and aborts HTTP; the run waits
  up to the grace period (`run.grace`, 60s) and then reports `RunAborted` with the nodes
  still running, drops later events, saves the report and returns.

## Consequences

- A new node type is a file, a spec and tests against a fake runtime; the scheduler,
  limits, scopes and reports apply to it unchanged.
- Hook flows gained every node type (M2 allowed only `set`, `extract`, `assert`, `log`)
  and report `NodeStarted`/`NodeFinished` inside their hook.
- Engine state touched from node goroutines must stay locked (variables, outputs, the
  secret tracker, generators, the request's overrides); engine tests run with `-race`.
