# flowi-lib

The core library of **Flowi**: everything that is not a user interface. The
[CLI](https://github.com/githugt/flowi-cli) and the [desktop app](https://github.com/githugt/flowi)
are thin layers over it, and other Go programs can embed it.

> Status: **pre-alpha**. The file format and the API will change.

## Why Flowi

Most API tools stop at the single request. Real work is a journey: log in, take the token, verify,
fill in a form, check the result. Flowi keeps a library of **requests** and chains them into
**flows**, such as:

> Login → extract token → Verify → generate form values → Form page 1 → delay → Form page 2

- **Plain files.** Collections are YAML in your own git repository. Secrets and run history live in a
  local workspace that is never committed.
- **Reusable requests.** Requests have versions (`1`, `1.1`, `2`), pinned examples and pre/post hooks.
- **Flows you can run anywhere.** The same flow runs in the app, in CI (`flowi run`) or on a schedule.
- **Standards, not lock-in.** OpenAPI, AsyncAPI, Arazzo, `.proto`, HAR, curl and Mermaid come in;
  Arazzo and Mermaid go out.
- **Safe changes.** Impact analysis shows which flows and hooks break when a request changes.

## What is in this repository

| Area | Packages | What it does |
|---|---|---|
| Model and storage | `model`, `store`, `workspace`, `validate`, `schema` | Collections, requests, flows and environments as YAML; validation with positioned diagnostics (`FLW-…` codes); JSON Schemas for editors |
| Requests | `engine`, `template`, `transport`, `grpcx`, `snippet` | HTTP, gRPC and message requests (Kafka, MQTT, AMQP, NATS, Redis, SQS, SNS, Pub/Sub, Service Bus, WebSocket, SSE); `{{templates}}` and generators; cookies, retries, HTTP/3; code snippets (curl and more) |
| Flows | `engine`, `blueprint`, `analysis` | Flow runtime with 20+ node types (extract, assert, condition, batch, parallel, run_till, subflow, …); reports (JUnit, JSON, Markdown); templates; impact analysis |
| Secrets | `vault` | Encrypted vault with the key in the OS keychain; secrets are masked everywhere |
| Interop | `openapi`, `asyncapi`, `arazzo`, `importers/*`, `mermaid`, `specoverlay` | Import and export, with a per-item report; nothing is guessed |
| Serving | `serve/*` | The replayer (saved responses as a fake API, or record from a real one), HTTP flow triggers and cron schedules, with a clean shutdown |
| Operations | `otel`, `telemetry`, `announce`, `changelog` | Optional and off by default: OpenTelemetry traces, opt-in usage reporting, signed announcements |

The Go API is the `flowi` package itself (`Workspace` and its methods).

## Use it from Go

```bash
go get github.com/githugt/flowi-lib@latest
```

```go
paths, _ := settings.DiscoverPaths(os.Environ())
home, _ := os.UserHomeDir()
ws, problems, err := flowi.OpenWorkspace(ctx, flowi.WorkspaceOptions{
	Paths: paths, Environ: os.Environ(), Home: home, Keys: vault.OSKeyStore(),
})
// problems are what the files got wrong; err is only for system failures

flow, _ := ws.FindFlow("user-creation", "")
run, _ := ws.RunFlow(ctx, flow, flowi.RunOptions{Env: "local"})
fmt.Println(run.Wait().Status) // run.Events() streams progress while it runs
```

`ws.FindRequest` and `ws.SendRequest` do the same for a single request.

## Try it

`examples/user-creation` is a complete collection, and `examples/demoserver` is the small API it talks to.
With the [CLI](https://github.com/githugt/flowi-cli) installed:

```bash
go run ./examples/demoserver/cmd/flowi-demo-server &   # http://127.0.0.1:8080, password "demo"
export FLOWI_CONFIG_DIR=$PWD/.try FLOWI_SECRET_LOCAL_PASSWORD=demo
flowi workspace mount ./examples/user-creation
# environments belong to the workspace, so copy the example's there
mkdir -p .try/workspaces/default-workspace/environments && cp examples/user-creation/environments/*.yaml .try/workspaces/default-workspace/environments/
flowi run user-creation --env local                    # login → verify → onboarding
flowi run onboarding-check --env local                 # subflows, batch, parallel, run_till, digest
flowi impact auth/login                                # who uses this request, and does it still fit?
```

## More

- [Node reference](docs/reference/nodes.md): every node type and its parameters.
- [Development guide](docs/development.md): layout, rules, how to work on all three repositories.
- [Design decisions](docs/adr): one short record per decision.
- [Changelog](CHANGELOG.md).

## Development

```bash
just check     # tidy, lint and test with the race detector
```

Needs Go 1.27 or later and [just](https://github.com/casey/just). No cgo.

## Licence

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
