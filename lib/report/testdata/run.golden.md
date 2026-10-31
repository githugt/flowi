# User creation journey — ⊘ aborted

| | |
|---|---|
| Run | `01JRUN0000000000000000000A` |
| Flow | `user-creation/user-creation` |
| Environment | local |
| Seed | 42 |
| Started | 2026-10-07 09:00:00 UTC |
| Duration | 1m1.5s |

> **Aborted** (grace_period_exceeded after 1m0s); in flight: `both/b1/verify`

## Nodes

- ✓ **login** (request) — passed, 120ms
  - ✓ hook pre collection `hooks/sign` — 5ms
    - ✓ **sign** (set) — passed, 1ms
- ↷ **flaky** (assert) — failed (continued), 1ms
  - `FLW-RUN-001` assertion failed: vars.ready
- ✓ **users** (batch) — passed, 300ms
  - [0]
    - ✓ **create** (fetch) — passed, 100ms
  - [1]
    - ✓ **create** (fetch) — passed, 90ms
- ⊘ **both** (parallel) — aborted, 1m0s
  - b1
    - ⊘ **verify** (request) — aborted, 1m0s
  - b2
    - ✓ **note** (note) — passed, 0s

## Warnings

- `FLW-RUN-015` node "flaky" failed; continuing (on_error: continue)

## Notes

- `both/b2/note`: hello

## Request versions

| Node | Request | Version | Chosen by |
|---|---|---|---|
| `login` | `auth/login` | 2 | flow preference |
