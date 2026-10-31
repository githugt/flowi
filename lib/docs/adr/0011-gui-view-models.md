# 0011: The GUI binds widgets to plain-Go view-models over a guarded backend

> **Update.** The view-models in `internal/vm` now serve the Wails GUI through `internal/bridge`; the Fyne widgets this ADR mentions are gone.

- **Status:** Accepted
- **Date:** 2026-10-08

## Context

The GUI must never freeze (design §10.5): every library call can touch the disk, the
network or the vault. Fyne widgets are hard to test without a window, and the GUI's
behaviour (saving, conflict handling, send options) needs tests that run in CI.

## Decision

- **Three layers.**
  - `internal/vm` holds view-models: plain Go, no Fyne import, one per screen or panel.
    They keep state, turn user intents into calls, and report changes through callbacks.
  - `internal/ui` holds screens that only bind view-model state to widgets.
  - `internal/app` wires both with `samber/do` once the workspace is open.
- **One way to the library.**
  - View-models see the library only through `vm.Backend`, an interface defined where it
    is consumed.
  - Every call runs through `vm.Worker`, which starts it on a goroutine and delivers the
    result on the UI goroutine through `fyne.Do`.
  - `internal/backend` adapts the library to `vm.Backend`, and every method first asks
    `uithread.Guard` whether it is on the UI goroutine. With `FLOWI_DEBUG_UI=1` (and in
    the guard's tests) that is a panic naming the call.
- **Saving is explicit and safe.**
  - The library gained `SaveRequest`, `SaveEnvironment` and `NewRequest`. They write the
    canonical encoding atomically and refuse to drop comments unless told to
    (`FLW-WRITE-001`). The GUI asks before confirming.
  - Edits stay in the view-model until saved. A file changed on disk while there are
    edits is flagged ("load from disk") and never silently overwritten.
- **Secrets never pass through files or memory longer than needed.**
  - The environment editor writes secret variables as declarations, then stores the
    typed values in the vault.
  - It clears them from the view-model after saving.

## Consequences

- Behaviour is tested headlessly: with a fake backend for each view-model, and against
  the real library and the demo server for sending and saving.
- Widgets stay thin; their tests use Fyne's test app.
- The first real run under the guard caught settings being read on the UI goroutine
  during startup. Preferences are now read on the worker while the workspace opens.
