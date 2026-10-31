# 0012: The flow canvas is a pure layout plus undoable document commands

- **Status:** Accepted
- **Date:** 2026-10-09

## Context

The canvas must show any flow readably, let people rearrange it freely without
touching the flow file, make real changes explicit and undoable, and save YAML
identical to what a person would write (design §10.3; roadmap G2).

## Decision

- **Layout is a pure function** (`flowi/internal/canvas`). It turns a flow into units:
  - request blocks (requests, subflows);
  - action strips (consecutive plain actions);
  - single actions (fetch, webhook, nodes with `needs` or `concurrent`);
  - containers whose lanes are laid out recursively.

  Units are placed left to right on a shared port line. Golden files pin the result for
  the example flows.
- **Positions are not the flow.** Manual positions and collapsed strips live in
  `<flow>.layout.yaml`, which the tool owns:
  - it is canonical and has no comments;
  - unknown ids are dropped;
  - a malformed file starts fresh with a warning.

  Free drag is not undoable; Re-layout clears it.
- **Structural edits are document commands** (`vm.FlowDoc`): insert, delete, move,
  needs, rename, node and flow edits.
  - Each keeps a copy of the flow for undo. Typing into one field merges into one
    step. Reloads clear the history.
  - Saving writes canonical YAML through `SaveFlow`. Building the example flow from
    scratch with these commands saves byte-identical YAML (a test).
- **Gestures map to commands.**
  - Dropping a palette item or a tree request on an edge inserts before the edge's
    target.
  - Alt-dragging a unit onto an edge moves it there; a plain drag only moves it.
  - Dragging from a port adds `needs`.
- **Rendering follows G0** (ADR 0006): unit widgets, one raster per edge, labels hidden
  below 0.35×. Flows open fitted to the view, and refit until the user pans or zooms.
- **Runs** go through `RunFlow`. Events for the flow's own nodes (not hook steps, not
  subflow internals) are coalesced to about 30 updates a second. A node shows its worst
  instance's status.

## Consequences

- Layout changes are reviewed as golden diffs; the canvas widget only draws.
- Renaming a node updates `needs` but not template references; validate reports stale
  ones.
- Concurrent nodes stay on their row with a ∥ mark rather than a separate lane.
