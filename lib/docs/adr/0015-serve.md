# 0015: `flowi serve` is one library facade with four small parts, safe by default

- **Status:** Accepted
- **Date:** 2026-10-09

## Context

Roadmap M7 adds a long-running process: a replayer that answers HTTP calls with saved
responses, HTTP triggers that start flows, cron schedules, and a shutdown that does not
drop work. The CLI runs it as a daemon; the GUI runs the replayer inside the app.

Both need the same behaviour, and a process that listens on the network and runs flows
has to be safe when started with no options.

## Decision

- **One facade, small parts.** `Workspace.Serve(ctx, ServeOptions)` composes packages that
  know nothing about the workspace and are tested with fakes:
  - `serve/cron`: parses five-field expressions and finds the next minute. No clock.
  - `serve/replay`: the replayer, an `http.Handler`.
  - `serve/runs`: starts runs for triggers and schedules, enforces the limit
    (`serve.max_runs`) and one run per key, remembers recent results, and shuts down.
  - `serve/trigger` and `serve/scheduler`: the HTTP trigger and the cron loop.
  The CLI and GUI call only the facade (`ServeDefaults` fills options from settings).
- **Replayer.**
  - A call matches a request version by method and URL path. A segment containing `{{…}}`
    matches any one segment; the most literal match wins. A call behind extra leading
    segments also matches (a base URL may carry a path prefix) but ranks below an exact one.
  - Answers come from pinned examples first, then saved responses. `X-Flowi-Status`,
    `X-Flowi-Example`, `X-Flowi-Request`, `X-Flowi-Version` and `X-Flowi-Latency` steer it;
    `X-Flowi-Match`, `-Request`, `-Version`, `-Example`, `-Source` say what happened.
    A named pick is compared with file names, never joined to a path.
  - Headers from files are filtered: invalid names, control characters, hop-by-hop and
    framing headers, `Content-Encoding` (bodies are stored decoded) and `X-Flowi-*` are
    dropped.
  - `--sequence` walks a request's responses in order; `--record URL` forwards calls and
    saves matching responses in the workspace data dir (never in the collection), with
    credential-looking response headers redacted.
  - Control endpoints live under `/__flowi/` (traffic, routes, reset).
- **Triggers.** `POST /flows/<flow>/run` answers 202 with a run id (or 200 with the result
  when `?wait=` is given). Every route but `/healthz` needs a bearer token, checked before
  the path is read; the token is compared through SHA-256 digests in constant time and is
  only accepted from the header. The token is a vault entry under the workspace id (the
  reserved environment `serve`); `FLOWI_SECRET_SERVE_TRIGGER_TOKEN` overrides it. With no
  token, triggers are off.
- **Schedules** are in `workspace.yaml` (personal, not in a collection). Ticks slept
  through are not replayed. A schedule is never started on top of its own previous run.
  Problems are FLW-SERVE-001 (error) and FLW-SERVE-002 (a flow that cannot be found: a
  warning).
- **Safe by default.** All listeners bind to 127.0.0.1; a listener open to the network is
  allowed but noted at start.
- **Shutdown.** On cancellation: stop accepting and starting; let requests and runs finish
  for `serve.shutdown_timeout` (default 30s); cancel what is left (runs stop within
  `run.grace`); give up on runs that ignore cancellation after two more minutes.
  `Server.Wait` returns `ErrShutdownTimeout` if anything had to be cancelled. The CLI exits
  0 on a clean drain even though a signal cancelled it, 3 on a forced stop.

## Consequences

- Schedules and the token are read once at start; changing them needs a restart.
- The replayer has no authentication. That is fine on localhost; binding it to the
  network is the operator's choice.
- Failed-auth attempts are logged (`TriggerDenied`) but not rate limited.
- Bodies recorded by `--record` are kept as received in the data dir, as normal sends are.
