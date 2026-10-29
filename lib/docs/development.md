# Development guide

Flowi is three Go modules in three repositories:

| Repository | Module | Contents |
|---|---|---|
| `flowi-lib` | `github.com/githugt/flowi-lib` | All logic. No UI dependencies, no cgo. |
| `flowi-cli` | `github.com/githugt/flowi-cli` | The `flowi` CLI. No cgo. |
| `flowi` | `github.com/githugt/flowi` | The desktop app (Wails 3 + Svelte). Needs cgo and Node. |

## Working on all three at once

Clone the repositories side by side and link them with a Go workspace in the **parent** folder:

```bash
mkdir flowi-workspace && cd flowi-workspace
git clone https://github.com/githugt/flowi-lib
git clone https://github.com/githugt/flowi-cli
git clone https://github.com/githugt/flowi
go work init ./flowi-lib ./flowi-cli ./flowi
```

`go.work` lives outside every repository and is never committed. With it, changes in
`flowi-lib` are picked up immediately by the CLI and GUI. Releases of the CLI and GUI
instead pin a tagged `flowi-lib` version in their `go.mod`. CI builds against `flowi-lib@main`
by creating a `go.work` on the fly.

## Everyday commands

Run these in each repository:

```bash
go test -race ./...          # tests (the race detector needs a C compiler)
golangci-lint run ./...      # lint (golangci-lint v2)
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

In `flowi-lib` and `flowi-cli`, also check that the code builds without cgo:

```bash
CGO_ENABLED=0 go build ./...
```

## Rules worth knowing up front

- **No cgo in `flowi-lib` or `flowi-cli`.** Any new dependency must build with
  `CGO_ENABLED=0` on macOS, Linux and Windows. Only the GUI uses cgo, through Wails.
- **No UI dependencies in `flowi-lib`.** No Wails, Fyne, Gio, Bubble Tea, cobra or `samber/do`.
- **Dependency injection** with `samber/do/v2` happens only in the CLI's composition
  root (the GUI wires its services by hand), always with an injector created explicitly (never the package-level default).
- **Environment variables** are read only by `flowi-lib`'s `settings` package; lint enforces it.
- **SPDX headers:** `Apache-2.0` in `flowi-lib`, `AGPL-3.0-or-later` in the CLI and GUI.
  A test in each repository checks every `.go` file.

## GUI prerequisites

- **macOS:** Xcode Command Line Tools (`xcode-select --install`).
- **Linux:** `gcc`, `libgl1-mesa-dev`, `xorg-dev`.
- **Windows:** MinGW-w64 `gcc` on `PATH`.

## Automated agents

Agents working in these repositories must not create commits or push unless a maintainer
explicitly asks for that specific commit.
