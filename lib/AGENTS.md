# AGENTS.md: flowi-lib

Instructions for AI coding agents (and a useful checklist for people) working in this repository.
`flowi-lib` is the core library of Flow-I: all logic, no user interface. The CLI (`flowi-cli`) and the app
(`flowi`) are thin layers over it. Read `README.md` for what it does and `docs/development.md` for how the three
repositories fit together.

## Musts

- **Run `just check` before you call work done.** It tidies, lints and runs every test with the race detector.
  It takes minutes: run it in the background and write the output to a file.
- **Never commit or push** unless the owner asks you to in that conversation.
- **Keep it free of UI and cgo.** No Wails, Fyne, Gio, Bubble Tea or cobra imports. Every dependency must build
  with `CGO_ENABLED=0` on macOS, Linux and Windows. Prefer the standard library; a new dependency is a long-term cost.
- **Read environment variables only in the `settings` package.** Lint enforces it. Everything else receives
  values as arguments.
- **Give every user-facing problem a diagnostic code** (`FLW-…` in `diag/codes.go`) with an English message and a
  hint. Tests assert the number of codes and settings: update them when you add one.
- **Add a test for every behaviour change, and say why the old behaviour was wrong** in a comment
  (`// Before: …`). Tests come with the change, not after.
- **Put the SPDX header on every Go file** (`// SPDX-License-Identifier: Apache-2.0`).
- **Keep generated and hand-written schemas in step.** `schema/*.json` are maintained by hand against
  `settings/schema.go` and the model; `docs/reference/nodes.md` is regenerated with
  `go test ./internal/nodes -update`.
- **Wrap errors with context** (`fmt.Errorf("doing x: %w", err)`) and compare them with `errors.Is` / `errors.As`.

## Must nots

- **No global mutable state, no `init()`.** Pass dependencies explicitly. (Test flags such as `-update` are the
  only accepted package-level variables, with a `//nolint` that says why.)
- **No `fmt.Print*` and no `os.Getenv`** outside what lint allows. Use `slog` or an injected `io.Writer`.
- **No secrets in files, logs, tests or fixtures.** Credentials become `{{secret.…}}` references and are masked
  everywhere they could be printed. Test values must be obviously fake.
- **Do not guess when you import.** Anything with no equivalent is reported, never invented.
- **No silent error swallowing.** If you ignore an error, a comment says why.
- **No external call without a timeout or a context.**

## Dos

- Write code that reads like a story: orchestrators decide the flow and pass errors on; workers hold the logic.
  Aim for about 20 lines and two levels of nesting, but do not split a function only to satisfy the number.
- Name functions with a verb that says what they do and return (`validateInput`, `computeForecast`), never
  `processData` or `utils`.
- Encode rules as data plus small functions, not long `if` chains.
- Validate external input at the edge: collection files, imported documents, URLs, spec overlays, user settings.
- Comment the non-obvious: ordering requirements, invariants, edge cases. Do not restate the code.
- Use `time.Duration` constants and named constants for magic numbers.
- Use `context` for cancellation, and give every long-running goroutine a way to stop and an owner that stops it.
- Keep the public API small: new code goes under `internal/` unless another repository needs it. Prefer additive changes.

## Tests

- **Naming:** `TestFunc_Given_<context>_When_<action>_Then_<outcome>`, or `TestFunc_should_<behaviour>`.
- **Shape:** Arrange, Act, Assert, with blank lines between. Table-driven where there are several cases.
- **Time:** no real `time.Sleep`. Use the engine's fake clock, `testing/synctest`, channels or polling.
- **Goldens:** fixtures live in `testdata/`; rewrite them with `-update` and read the diff before you keep it.
- **Fuzz** the parsers (`FuzzParse`, codec fuzzers) when you add one.
- A flaky test is a bug: find the cause (a timeout that is too tight, shared state) instead of retrying.

## Layout and where things go

| You are adding… | It goes in… |
|---|---|
| A node type | `model/` (type), `internal/nodes/` (runtime), `schema/` and the validator; then regenerate `docs/reference/nodes.md` |
| A setting | `settings/schema.go`, the JSON schema, and the count in its test |
| A request protocol | `transport/<name>/`, the model's async protocol table and the engine |
| An importer | `importers/<name>/`, registered in `import_api.go`, with a per-item report |
| A facade method for the apps | the root `flowi` package (`*_api.go`) with a test |

Releases and the changelog come from Conventional Commits (`feat:`, `fix:`, `docs:`…) through `git cliff`.
