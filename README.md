# Flowi

Flowi is a tool for building, running and testing sequences of API calls. This repository holds all of it, in three
parts that each have their own module, licence and releases:

| Folder | What it is | Licence |
|---|---|---|
| [`app/`](app) | The desktop app: build requests and flows, run them, see the results. | AGPL-3.0 |
| [`cli/`](cli) | The `flowi` command line: runs the same collections and flows in CI. | AGPL-3.0 |
| [`lib/`](lib) | The core library both are built on: no user interface, no cgo. | Apache-2.0 |

The licence in each folder's `LICENSE` file applies to that folder only. The library is Apache-2.0 so that it can be
used in other software; the app and the CLI are AGPL.

Start with the [app README](app/README.md) for what Flowi does and how to run it, or the
[development guide](lib/docs/development.md) for how the parts fit together.

## Working in the repository

`go.work` links the three modules, so a change in `lib/` shows up in `cli/` and `app/` straight away. From the root:

```bash
just run         # start the app
just check       # tidy, lint and test every module
```

## Releases

Each part is tagged on its own, with its folder as a prefix: `lib/v0.3.0`, `cli/v0.3.0`, `app/v0.3.0`. A `cli/` or
`app/` tag builds and publishes that part's release; the `lib/` tag is what `go get github.com/githugt/flowi/lib`
resolves.
