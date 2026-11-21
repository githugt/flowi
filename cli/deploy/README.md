# Running `flowi serve` as a service

Templates for keeping `flowi serve` running under systemd, in a container, or in Kubernetes. They are starting
points: check the paths, the user and the ports against your own setup before relying on them.

## What `flowi serve` offers a supervisor

| Endpoint | Where | What it says |
| --- | --- | --- |
| `GET /healthz` | `--health-addr` | the process is alive (200 while it can answer) |
| `GET /readyz` | `--health-addr` | 200 when it takes new work; 503 with the reasons while it drains at shutdown or every run slot (`serve.max_runs`) is in use |
| `GET /metrics` | `--health-addr` with `--metrics` | Prometheus text: `flowi_up`, `flowi_ready`, `flowi_runs_in_flight`, `flowi_runs_max`, `flowi_schedules`, `flowi_replay_calls_total` |

None of them needs a token and none names a flow, a request or a secret: they are counts and states. Bind the
health address to loopback, or to the pod or container network, and not to a public interface. (`/healthz` is also
answered on the trigger address, but that endpoint is off without a token, so probes should use `--health-addr`.)

## Secrets in a service

A service has no keychain and no one to type a passphrase. Give it, through the environment (a systemd
`EnvironmentFile`, a Kubernetes Secret, a container secret):

- `FLOWI_VAULT_KEY`: the passphrase of the workspace vault.
- `FLOWI_SECRET_SERVE_TRIGGER_TOKEN`: the bearer token of the HTTP triggers, in place of `flowi serve token`. Named, limited tokens (`flowi serve tokens new`) live in the workspace directory (`trigger_tokens.json`, hashes only), so give a container that directory as a volume to keep them.
- `FLOWI_SECRET_<ENV>_<NAME>`: any environment secret a flow needs.
- `FLOWI_CONFIG_DIR`: where Flowi keeps its files, so the service does not depend on a home directory.

## Shutdown

`SIGTERM` stops accepting calls, lets running requests and flows finish for up to `--shutdown-timeout`
(`serve.shutdown_timeout`), then cancels what is left. While it drains, `/readyz` answers 503 so a load balancer
stops sending work. Set the supervisor's own stop timeout (`TimeoutStopSec`, `terminationGracePeriodSeconds`) a few
seconds longer than the shutdown timeout.

## Files

- [`flowi.service`](flowi.service): a systemd unit.
- [`Dockerfile`](Dockerfile) and [`compose.yaml`](compose.yaml): a container with a health check.
- [`kubernetes.yaml`](kubernetes.yaml): a Deployment with liveness and readiness probes and a Service.
