# 0017: A flow is an Arazzo document; Flowi's own data rides along in it

- **Status:** Accepted
- **Date:** 2026-10-15
- **Supersedes in part:** 0013 (flows were Flowi's own YAML, converted to and from Arazzo on
  import and export; they are now stored as Arazzo)

## Context

The owner is moving Flowi from "a request tool with flows" to defining APIs and requests first
and stitching them together with Arazzo. A flow that other Arazzo tools can read, diff and run
(where they can) is worth more than a private format plus a lossy export. But a Flowi flow holds
more than Arazzo can say: parallel lanes, conditions, batches, `set`/`log`/`fetch`/`webhook`
nodes, concurrency and error policy, hook controls.

## Decision

- **A flow is one file, `flows/<path>.arazzo.yaml`**: an Arazzo 1.1.0 document with one
  workflow. Canvas positions stay in `<path>.layout.yaml`. Old `*.flow.yaml` files are not
  migrated or read; they are reported as unrecognised files (the owner had none to keep).
- **The flow's own data is kept whole in the workflow's `x-flowi-flow`** (the same YAML the old
  flow files held). Flowi reads that, so encode → decode → encode is byte-stable and nothing
  can be lost, whatever the node types. The standard Arazzo part (steps, `successCriteria`,
  `outputs`, `inputs`) is *derived* from it for other tools.
- **A node Arazzo cannot express becomes a custom step**: an ordinary step whose `operationId`
  is the built-in `flowiCustomStep`, with `x-flowi-node: <id>`. It resolves in any tool and cannot
  be run by one; Flowi runs the node. (A stub workflow was considered and rejected: every step
  of it would itself need an operation to call.) The GUI tells the person editing which steps
  these are (`FLW-ARAZZO-002`, from `FlowCompatibility`; `Validate` does not raise it, because
  the example flows are full of them).
- **Drift is detected, not merged.** The workflow carries `x-flowi-digest`, a hash of the
  standard steps as Flowi wrote them. If the steps no longer match, someone edited them in
  another tool: `FLW-ARAZZO-001`, and Flowi's own data is used. A hand-written file without a
  digest is not checked.
- **A foreign Arazzo document** (no `x-flowi-flow`) is read through the existing importer, which
  reports every loss (`FLW-IMPORT-001`); saving it writes the native form.
- **Requests get operation ids from a generated, committed description.**
  `descriptions/requests.openapi.yaml` has one operation per request version (stable ids: the
  request path as an identifier, `_vN` for non-default versions, `_2` on a clash), their learned
  schemas (ADR for schema inference), and the built-in operation. It is derived: rewritten with
  stable bytes after every change to a request, flow or learned schema and on mount, removed when
  the collection has neither. It is committed with the collection so a clone or another tool
  can resolve the flows. It carries parameter names, never values.

## Consequences

- Other Arazzo tools can read a Flowi flow, but not run custom steps; `ExportArazzo` still
  writes the standard-only form for them (steps Arazzo cannot express are left out and reported).
- The engine, canvas, validation and GUI are unchanged: `model.Flow` is still the in-memory model,
  and only the flow codec behind the store changed.
- Flow positions in diagnostics are mapped from `$.nodes[…]` to the place under `x-flowi-flow`.
- Renaming a request changes its operation id in the description and in the derived steps of
  flows that use it the next time those flows are saved; the flows themselves refer to requests
  by id, so they keep working.
- Imported OpenAPI specs are not yet kept as their own source descriptions; every operation is
  currently described by the generated file (a later step of the same direction).
