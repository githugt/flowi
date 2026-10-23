# Telemetry

<!-- Generated from telemetry.Catalogue; run `go test ./telemetry -run Reference -update`. -->

Telemetry is off unless you turn it on. It never contains URLs, request or response bodies, names of requests, flows or collections, ids, file paths, environment names, variable values or secrets. There is no install id, so reports cannot be tied together or to you.

- off: nothing is collected or sent.
- minimal: once per start, the app (cli or gui), its version, your operating system and CPU architecture, and the day.
- full: minimal, plus which command ran and how it ended, and for each run its kind, outcome, duration and node count as ranges.

Run `flowi telemetry show` to see exactly what would be sent. Nothing is sent unless the telemetry_endpoint setting is also set.

## Events

### `app_started` (minimal)

The application was started. It carries no fields; the version, system and architecture are in the payload's client block.

### `command_run` (full)

A command-line command finished.

| Field | Meaning | Values |
|---|---|---|
| `command` | Which command, never its arguments. | `send`, `run`, `ls`, `validate`, `fmt`, `serve`, `import`, `export`, `new`, `config`, `secret`, `env`, `workspace`, `init`, `schedule`, `news`, `changelog`, `telemetry`, `version`, `impact`, `cookies`, `blueprint`, `other` |
| `exit` | How it ended. | `ok`, `failed`, `usage`, `cancelled`, `error` |

### `run_finished` (full)

A send or flow run finished.

| Field | Meaning | Values |
|---|---|---|
| `kind` | A single request or a whole flow. | `send`, `flow` |
| `status` | How the run ended. | `passed`, `failed`, `error`, `cancelled`, `aborted` |
| `duration` | How long it took, as a range. | `<1s`, `<10s`, `<1m`, `<10m`, `>=10m` |
| `nodes` | How many nodes ran, as a range. | `0`, `1-5`, `6-20`, `21+` |

## Client block

| Field | Values |
|---|---|
| `app` | `cli`, `gui` |
| `version` | the release version, or `unknown` |
| `os` | `darwin`, `linux`, `windows`, `other` |
| `arch` | `amd64`, `arm64`, `other` |
