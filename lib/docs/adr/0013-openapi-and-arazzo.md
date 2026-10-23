# 0013: OpenAPI and Arazzo are pure transforms that report every loss

- **Status:** Accepted (flows are stored as Arazzo since 0017)
- **Date:** 2026-10-09

## Context

Flow-I has to meet the OpenAPI ecosystem: import operations as requests, use response
schemas as contracts for impact analysis, and move flows to and from Arazzo (design §9,
§11; roadmap M5). Arazzo and Flow-I overlap but do not match. Arazzo has AsyncAPI steps,
step actions and replacements; Flow-I has delays, branches, generators and its own
template language. The risk is a converter that guesses, and produces flows that look
right and run wrong.

## Decision

- **One library, used only to validate.** `pb33f/libopenapi` (MIT, cgo-free) checks
  that the input is an OpenAPI 3.x or Arazzo 1.x document. It is the only Go library with
  Arazzo models. The mappings work on the decoded YAML, with local `$ref` resolution;
  external references are reported, not followed.
- **Pure transforms.**
  - `openapi.Import` and `arazzo.Import`/`Export` take bytes and return model values
    plus diagnostics. They write nothing.
  - The facade plans files (`PreviewImport`, or `PreviewImportNew` for a dry run into a
    collection not created yet) and writes them (`ApplyImport`).
  - Existing files are refused (FLW-IMPORT-003) unless the caller chooses to overwrite
    them or to skip them. Replaced requests and flows keep their ids, so references
    survive a re-import.
- **Schemas are the best contract.**
  - Response schemas are stored, tool-owned, as
    `<request>.schemas/<version>-<status>.json`, with `$ref`s inlined. Cycles, nesting
    past 6 and expansion past 500 references per value are cut to `{}`, so densely
    referencing specs stay linear.
  - Only exact status codes become files: ranges, `default` and anything else are
    reported. Every planned file must lie inside the collection. Overwriting removes
    schema files the document no longer has.
  - They beat pinned examples, which beat the last saved response.
  - A property a schema does not list is ✗. Untyped schemas, objects listing no
    properties, `additionalProperties: true`, and headers, cookies and regexes are ⚠.
    `oneOf`/`anyOf` branches are judged together with the properties beside them.
- **Operations belong to a request's first version.**
  - `Request.operation` is the first version's `operationId`. Arazzo steps that call it
    are pinned to that version.
  - On export, a call that may run another version is reported. An unpinned call
    passes only when the request has a single version, since flow and run preferences
    also choose the version.
- **The expression mapping is a closed table** (`arazzo/expr.go`):

  | Arazzo | Flow-I |
  |---|---|
  | `$inputs.x` | `inputs.x` |
  | `$steps.s.outputs.o` | `vars.s_o`, written by the step's extract node |
  | `$statusCode` | `nodes.s.status` |
  | `$response.body#/a/0` | `nodes.s.body.a[0]` |
  | `$response.header.X` | `nodes.s.headers["x"]` |

  Anything else is unmappable. That includes `$message`, `$outputs`, a read of a step
  output that was not imported or comes later, literal `{{`, and on export any function,
  `env`, `secret` or another node's field.
- **Losses are reported, never guessed.**
  - On import, each element without an equivalent is FLW-IMPORT-001. A step that cannot
    be called becomes a `note` node, so the flow keeps its shape.
  - On export, nodes are FLW-EXPORT-001 and are listed in `x-flowi-skipped`; values are
    FLW-EXPORT-002. Nodes with `when`, `on_error` or `concurrent`, or whose id is not a
    valid stepId, are reported. Literals an Arazzo runner would read as expressions
    (`$…`, `{$…}`) are reported.
  - Conditions export only in Arazzo's own syntax: single-quoted strings, numbers,
    `true`/`false`/`null`, parentheses, `== != < <= > >= && || !` and references that
    map. Anything else is refused.
  - On import, step outputs whose variables would collide are reported. String
    literals in conditions are never rewritten.
- **Export writes Arazzo 1.0.1.**
  - Fields that 1.0.1 lacks (step `dependsOn` and `timeout`) go into `x-flowi-*`
    extensions, which import reads back.
  - Asserts and map-form extracts directly after a request become its
    `successCriteria` and `outputs`.
  - Goldens pin the export of the official 1.0 and 1.1 examples. Export, then import,
    then export again is byte-stable, and every export parses with libopenapi.

## Consequences

- A dependency tree (pb33f's YAML fork and JSONPath, small helpers) enters flowi-lib,
  but only the `openapi` and `arazzo` packages import it.
- `operationPath` steps, AsyncAPI steps, step actions, payload replacements and
  non-simple criteria import as notes, or are left out with a report. They need hand
  work.
- Flow inputs export as strings, because the model has no input types.
- Re-importing a changed spec with "skip existing" adds new operations but does not
  update edited ones. Updating in place is future work.
