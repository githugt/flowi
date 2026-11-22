# flowi-cli

`flowi`, the command-line tool of **Flowi**: send saved API requests, run flows in CI, import and
export API descriptions, and serve a fake API from your saved responses.

> Status: **pre-alpha**. Commands and the file format will change.

## Why a command line

The [desktop app](https://github.com/githugt/flowi) is where you build requests and flows. The CLI runs
the same collections where the app cannot: in CI, on a server, in a script, or on a schedule. All logic
lives in [flowi-lib](https://github.com/githugt/flowi/tree/master/lib), so a flow behaves the same everywhere. The
CLI builds without cgo, so it is one static binary on every platform.

## Install

```bash
go install github.com/githugt/flowi/cli/cmd/flowi@latest
```

Needs Go 1.27 or later; the binary lands in `$(go env GOPATH)/bin`. Release builds for macOS, Linux and Windows are
attached to each [GitHub release](https://github.com/githugt/flowi/releases).

To build from a clone instead:

```bash
git clone https://github.com/githugt/flowi && cd flowi/cli
just build        # puts the binary in ./bin/flowi
```

Runs on macOS, Linux and Windows (Windows 10 or later: programs built with Go 1.21 or later do not run on Windows 7
or 8).

## Five-minute tour

```bash
flowi init ./my-api                          # create a collection and mount it
flowi workspace mount ../other-api           # or mount an existing one
flowi ls requests                            # what is in it
flowi secret set staging password            # prompts; the value goes to the encrypted vault
flowi secret get staging password --reveal   # prints it again (without --reveal: only whether it is set)
flowi send auth/login --env staging --with email=a@example.com
flowi run user-creation --env staging        # run a flow
flowi impact auth/login                      # who uses this request, and do they still fit it?
flowi diff user-creation --rev main          # what changed in this flow, step by step
```

`flowi send` prints the response body on standard output and everything else on standard error, so
`flowi send auth/login | jq .token` works.

## Commands

| Command | What it does |
|---|---|
| `init`, `workspace mount/ls`, `ls`, `new` | Create and mount collections; list and create requests and flows (`flowi new flow team/chain --from two-calls`); `ls --tag <tag>` or `--status review` lists only what carries the tag or has the [development status](../lib/docs/reference/status-and-tags.md) |
| `send <request>` | Send one saved request: HTTP, gRPC, a message (publish to or receive from Kafka, MQTT, NATS, SQS and more) or a database query. `--env`, `--with name=value`, `--prefer-version`, `--flowi-overlay`, `--pin` to keep the answer as an example, `--check` to only test the connection of a message or database request |
| `run <flow>` | Run a flow with a live progress tree. `--tag <tag>` or `--all` runs many flows (`--fail-fast` stops at the first that does not pass). `--input`, `--seed` to repeat generated values, `--matrix env=local,staging` to run every combination, `--data users.csv` to run once per row of a CSV or JSON file (see [data-driven runs](../lib/docs/data-driven-runs.md)), `--overlay`, `--bundle` to keep the run for offline replay, `--report junit=report.xml` (also `json`, `markdown`, `text`, `mermaid`, `har`) |
| `impact <request>` | List everything that uses a request, and whether each still fits its contract. Exits 1 if something is broken |
| `diff <flow>` | Show what changed in a flow since a git revision (`--rev main`) or an earlier file (`--against old.arazzo.yaml`): steps added, removed, moved, renamed, retyped or rewired, matched by step id. `--exit-code` exits 1 when anything changed |
| `migrate [--check]` | Show whether each collection is in the file format this release writes and whether this release satisfies its `min_version`. `--check` exits 1 if not, so CI fails early with a clear reason |
| `validate`, `fmt` | Check collections for errors; rewrite files in canonical form |
| `coverage`, `drift`, `access` | How much of an API description the flows use (`coverage --from-report run.json` takes the seen statuses from run reports, for a CI job with no history); whether live responses still match the schemas learned before (`drift --update` accepts what it saw as the new baseline: commit the changed `*.schemas` files); whether another user can reach what a flow made (`access --report junit=access.xml` for a CI report) |
| `import <kind> <file>` | Bring in OpenAPI, AsyncAPI, `.proto`, Arazzo, Postman, Bruno, HAR, `.http` or curl (a URL works; `-` reads standard input), and Mermaid sketches |
| `export arazzo\|mermaid <flow>` | Write a flow as Arazzo 1.0 or as a Mermaid flowchart for docs |
| `completion bash\|zsh\|fish\|powershell` | Print a shell completion script (for example `source <(flowi completion bash)`) |
| `serve` | One process for the replayer, HTTP flow triggers and cron schedules (below) |
| `auth tokens`, `auth forget <id>` | List the OAuth tokens flowi keeps (what each is for, when it expires, whether it renews; never the token) and drop one so the next send signs in again |
| `env ls/new/set/unset/import/share` | Environments: create one, set plain variables (`flowi env set staging retries 3 --type number`), copy the ones a collection ships into the workspace (`flowi env import`), or write one into the collection so it goes with it in git (`flowi env share staging`; secrets are written as names only). Where the workspace has no environment of that name, a send or run uses the collection's own file, with secrets from `FLOWI_SECRET_<ENV>_<NAME>` |
| `merge-driver install` | Let git merge flow files: two branches that changed different steps of a flow no longer conflict on the Arazzo steps and the digest line flowi derives. It adds `*.arazzo.yaml merge=flowi` to the collection's `.gitattributes` (commit it) and registers the driver in the repository's git config (each clone runs it once). Where both branches changed the same lines of the flow, git's usual markers stay, and `flowi validate` says so (`FLW-FILE-003`) instead of a YAML error |
| `schedule`, `secret`, `cookies`, `blueprint`, `overlay`, `config` | Cron schedules; the secret vault; cookie jars; flow templates; flowi overlays (request variants); settings (`flowi config explain timeout`) |

Every command takes `--output json` for NDJSON events followed by the result. Run `flowi <command> --help`
for the details.

In a GitHub Actions job (`GITHUB_ACTIONS=true`), `flowi run` and `flowi validate` also print each problem as a
workflow annotation (`::error file=…,line=…::…`), and `flowi run` appends a summary to `$GITHUB_STEP_SUMMARY`.
`--output github` prints the annotations on any machine.

### Import and export

Importers report each item with ✓, ⚠ or ✗ and show problems at their line and column. Postman test scripts are translated
where they use the common idioms (`pm.environment.set(…, pm.response.json().x)` becomes an extract; `pm.response.to.have.status`,
`pm.expect(…).to.eql` and the like become asserts); the lines that are not, are kept word for word in the request's docs. Credentials become
`{{secret.…}}` placeholders and are never written to a file. When items cannot be imported nothing is
written unless you pass `--partial`. Use `--dry-run` to list the files first.

### Serve: replayer, triggers, schedules

```bash
flowi serve                                   # replayer on 127.0.0.1:8525, schedules, triggers if a token exists
curl -X POST localhost:8525/v1/login          # answered from auth/login's pinned example
flowi serve --record https://api.example.com  # forward calls, save the real answers, and make requests of new calls
flowi schedule add nightly --flow user-creation --cron '0 3 * * *' --env staging
```

- **Replayer.** A call is matched to a request by method and path and answered from its pinned example or
  last saved response. Request headers such as `X-Flowi-Status: 401` or `X-Flowi-Latency: 800ms` steer it. `X-Flowi-Fault: error:418` or `drop` makes one call fail on purpose.
- **Triggers.** `POST /flows/<flow>/run` starts a flow (add `?wait=30s` to wait). Make a bearer token with
  `flowi serve tokens new <name>`, which can be limited to flows (`--flow 'smoke/*'`), environments (`--env staging`) and a
  time (`--expires 30d`); a limited token reads only the runs it started, `tokens ls` shows the limits (never the
  values, of which only a hash is kept), `tokens rotate` and `tokens rm` act at once even in a running `flowi serve`.
  `flowi serve token` still makes the single full-power token; without any token, triggers are off. The endpoint speaks plain HTTP unless you give
  `--tls-cert` and `--tls-key` (PEM files), and it refuses an address other machines can reach without TLS unless you
  pass `--insecure-listen` to say a proxy in front terminates it (the container image does); ten bad tokens from one address in a minute lock that
  address out (429) until the minute ends.
- **Schedules** are kept in `workspace.yaml`. Everything binds to localhost by default, and Ctrl-C finishes running work first.
  `--on-failure` and `--on-success` on `flowi schedule add` name where `flowi serve` tells when a run ends: a webhook address
  that takes a Slack-compatible `{"text": …}` POST, or `env:NAME` to read the address from an environment variable (a
  webhook address is a credential; `workspace.yaml` is not committed, but prefer `env:NAME`). A delivery that fails is logged
  with the host only and never changes the run.
- **Health.** `--health-addr 127.0.0.1:8527` serves `GET /healthz` (alive) and `GET /readyz` (503 while it drains or every run slot is in use) with no token, and `--metrics` adds `GET /metrics` in the Prometheus text format. [`deploy/`](deploy/) has a systemd unit, a Dockerfile, a Compose file and a Kubernetes manifest that use them.

## Environments and secrets

Environments belong to the workspace, not to a collection, so they are never committed. Secret values live
only in the workspace vault, whose key is kept in the OS keychain. On CI or a machine without a keychain, set
`FLOWI_VAULT_KEY` (any passphrase), or give each value as `FLOWI_SECRET_<ENV>_<NAME>`.
Any setting can be overridden with `FLOWI_<KEY>`; see [`.env.example`](.env.example).

A collection is someone else's code, so a send that would put a secret in a request stops (`FLW-SEC-005`) unless
the host is named by a variable of the selected environment (its base URL), is this computer, or is on your own list:
`flowi secret allow-host api.example.com` (also `host:port` and `*.example.com`), `flowi secret hosts`,
`flowi secret deny-host`. A collection file cannot change the list. `secrets.host_check` (`FLOWI_SECRETS_HOST_CHECK`)
is `block` by default, `warn` sends with a warning, `off` does not check. It covers HTTP and message requests; a
redirect is covered by `http.redirect_credentials`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (for `send`, an HTTP error status alone is not a failure) |
| 1 | A flow, hook or assertion failed, a check found differences, or `impact` found a broken consumer |
| 2 | Invalid usage, files or configuration (undefined variable, missing secret, …) |
| 3 | Cancelled or aborted |
| 4 | System error, including network failures and reports that could not be written |

## Development

```bash
just demo        # walk through the main commands in a throw-away sandbox, against the demo API
just cli ls requests   # run flowi inside that sandbox; your real settings are never touched
just check       # tidy, lint and test with the race detector
```

Optional extras (OpenTelemetry traces, usage reporting, signed announcements) are off until configured;
see [flowi-lib](https://github.com/githugt/flowi/tree/master/lib).

## Licence

AGPL-3.0-or-later. See [`LICENSE`](LICENSE).
