# Development guide

Flowi is three Go modules in one repository, each with its own licence:

| Folder | Module | Licence | Contents |
|---|---|---|---|
| `lib/` | `github.com/githugt/flowi/lib` | Apache-2.0 | All logic. No UI dependencies, no cgo. |
| `cli/` | `github.com/githugt/flowi/cli` | AGPL-3.0 | The `flowi` CLI. No cgo. |
| `app/` | `github.com/githugt/flowi/app` | AGPL-3.0 | The desktop app (Wails 3 + Svelte). Needs cgo and Node. |

## Working on all three at once

Clone the repository once. `go.work` at its root links the three modules, so a change in `lib/` is picked up by the
CLI and the app immediately, and each module's `go.mod` also has a `replace` for `../lib` so it builds on its own
too:

```bash
git clone https://github.com/githugt/flowi
```

The library is released with a `lib/vX.Y.Z` tag (Go needs the folder prefix for a module in a subfolder), the CLI with
`cli/vX.Y.Z` and the app with `app/vX.Y.Z`.

## Everyday commands

Run these in each folder (`lib`, `cli`, `app`):

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
  A test in each module checks every `.go` file.

## GUI prerequisites

- **macOS:** Xcode Command Line Tools (`xcode-select --install`).
- **Linux:** `gcc`, `libgl1-mesa-dev`, `xorg-dev`.
- **Windows:** MinGW-w64 `gcc` on `PATH`.

## Automated agents

Agents working in these repositories must not create commits or push unless a maintainer
explicitly asks for that specific commit.
