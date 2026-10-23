# AGENTS.md: flowi

Instructions for AI coding agents (and a useful checklist for people) working in this repository.
`flowi` is the Flow-I desktop app: Go and Wails 3 for the window, Svelte 5 and TypeScript for the screens, built on
`flowi-lib`. Read `README.md` for what the app does.

## Musts

- **Run `just check` before you call work done.** It tidies, lints, runs the Go tests with the race detector,
  type-checks the Svelte code and runs the front end's unit tests. It takes minutes: run it in the background and
  write the output to a file.
- **Never commit or push** unless the owner asks you to in that conversation.
- **Run `just bindings` after you change an exported method of a service** in `internal/bridge`, and do
  not edit `frontend/bindings` by hand: it is generated.
- **Keep logic in Go.** Screen logic belongs in `internal/vm` (plain Go, no UI toolkit), shared with the tests,
  and the bridge turns it into views. The front end renders views and sends user actions; it does not repeat rules.
- **Run every view-model on the one UI goroutine** (`bridge.Actor`). Calls into `flowi-lib` happen off it, through
  the worker, and report back on it. Tests use `NewInlineActor` and the fake backend in `internal/vm/vmtest`.
- **Use the design tokens** in `frontend/src/app.css` for every colour, size and radius. `src/lib/tokens.test.ts`
  fails on a hard-coded colour.
- **Check what you change on screen.** Build with `go build -tags server -o bin/flowi-server .`,
  start it with a throw-away `FLOWI_CONFIG_DIR` and `WAILS_SERVER_PORT=8731`, open that port in a browser, and look. Say what you could not check in the native
  window.
- **Put the SPDX header on every Go file** (`// SPDX-License-Identifier: AGPL-3.0-or-later`).
- **Wrap errors with context** and show users a clear message, never a Go error string or a stack.

## Must nots

- **No Fyne, and no second UI toolkit.** The old Fyne app is retired.
- **Do not push cgo into the libraries.** This app uses cgo (through Wails); `flowi-lib` and `flowi-cli` must not.
- **No global mutable state, no `init()`.** Pass dependencies explicitly.
- **No secrets** in files, logs, tests, fixtures or screenshots. Secret fields are password fields; values are
  never put in a view that is logged or published as an event.
- **No hard-coded paths, names or home directories** in code, tests or screenshots. Use temporary directories and
  fake data.
- **Do not commit build output** (`bin/`, `frontend/dist`, `frontend/node_modules`).
- **Do not let a Wails build task rewrite tracked files.** `wails3 task package`, `wails3 task build` and
  `wails3 task common:update:build-assets` regenerate `frontend/bindings` as TypeScript (deleting the `.js` files),
  the icons and the platform plists. Run them in a throw-away clone, or check `git status` afterwards and restore what
  you did not mean to change (`git checkout -- frontend/bindings build/...`). `frontend/bindings` comes from
  `just bindings` only.
- **No `console.log` or leftover debug code** in the front end; no `fmt.Print*` in Go (use `slog`).

## Dos

- Write code that reads like a story: orchestrators decide the flow and pass errors on; workers hold the logic.
  Name functions with a verb that says what they do and return.
- Use Svelte 5 runes (`$state`, `$derived`, `$props`). Keep components small, and share behaviour in `src/lib`.
- Give controls a label or title, and keep the app usable by keyboard. Respect all themes, including the
  high-contrast one.
- Keep text short and plain; say what the user can do next.
- Ask before you delete or overwrite something the user made, and make the question say what will happen.
- Write tests as Arrange, Act, Assert with blank lines between, named
  `TestFunc_Given_<context>_When_<action>_Then_<outcome>` (Go) or in plain words (vitest). Add a test for every
  behaviour change, and a `// Before: …` comment saying what was wrong.
- No real `time.Sleep` in tests: use the inline actor, channels or polling.
- Releases and the changelog come from Conventional Commits (`feat:`, `fix:`, `docs:`…).

## Layout

| Path | What it holds |
|---|---|
| `main.go`, `menu.go` | the Wails application and its menu |
| `internal/bridge` | one Go service per area (workspace, flows, requests, settings…); Svelte calls their methods |
| `internal/vm` | screen logic, tested without a window; `vmtest` is a fake backend |
| `internal/canvas` | the layout of the flow canvas |
| `internal/preflight` | the Linux start-up check: WebKit's helpers and sandbox tools, and the install command per distribution |
| `internal/backend`, `internal/uithread` | the adapter to `flowi-lib`, and the UI-goroutine guard |
| `frontend/src` | Svelte components; `lib/` holds shared TypeScript |
