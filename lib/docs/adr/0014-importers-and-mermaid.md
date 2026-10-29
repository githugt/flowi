# 0014: Importers report per item, never write credentials, and Mermaid is a sketch format

- **Status:** Accepted
- **Date:** 2026-10-09

## Context

People move to Flowi from Postman, Bruno, browser recordings (HAR) and curl commands
they already have. They also sketch flows as Mermaid diagrams (design §11; roadmap M6).

Other tools' exports are messy:
- broken files;
- scripts in JavaScript;
- credentials pasted inline;
- invisible characters copied from documents.

An importer that stops at the first problem, guesses, or writes a token into a file that
is committed to git does more harm than good.

## Decision

- **One shape for every importer.**
  - `importers/<tool>` packages are pure transforms. Each returns an `importers.Result`
    holding requests, environments, flows and pinned examples, plus one `Item` per
    source item: imported or skipped, with its diagnostics.
  - Importers leave ids empty and point flows at requests by path. The facade
    (`planResult`) assigns ids, keeps the ids of files it replaces, and resolves paths.
- **Shared rules** (`importers`):
  - **Credentials.** `SensitiveHeader`/`Redact`/`RedactJSON`:
    - covered: Authorization, Cookie, auth blocks, any header or key naming a token,
      secret, key, session, password or auth;
    - written out literally, these become `{{secret.<name>}}`; in recorded bodies they
      become `(redacted)`;
    - each replacement is reported (FLW-IMPORT-006);
    - a reference such as `Bearer {{token}}` is translated, not redacted.
  - **Variables.** `Translator`:
    - `{{x}}` becomes `env.x`, or `secret.x` for credential-like names (the same rule
      declares those names secret when environments import);
    - variables the source extracts at run time are read bare (`{{token}}`, which looks
      in the flow's vars first);
    - dynamic variables map to generators, and unknown ones are reported;
    - `process.env.X` becomes `env.X`.
  - **Literal text** goes through `Literal`, so a `{{` in a pasted body is never read
    as a template.
- **Report and partial import.**
  - Positions are rune columns. `diag.Diagnostic.Excerpt` carries the source line and a
    caret, with invisible characters shown as `⟨U+200B⟩`.
  - JSON formats get their positions from `importers/jsonpos` (one decoder pass).
  - A document that cannot be read is FLW-IMPORT-002/004. An item that cannot be
    imported is skipped (FLW-IMPORT-005) and contributes nothing.
  - Partial import is a choice: `--partial` on the CLI, **Import the rest** in the GUI.
    Otherwise skipped items mean nothing is written.
- **Per tool:**
  - **curl:** bash, `$'…'` and Windows cmd quoting; harmless flags are ignored and
    unknown ones reported.
  - **HAR:** grouped by endpoint (ids in paths become `{{id}}`), a 2xx JSON response as
    each request's example, and a flow in recorded order. Static resources and non-HTTP
    traffic are left out as a summary.
  - **Postman v2.x:** folders, inherited auth, every body mode, saved responses as
    examples, collection variables and environment files as environments.
  - **Bruno:** a collection folder with inherited headers and auth. Asserts become
    request asserts and post-response vars become extracts.
  - **Scripts:** reported everywhere, never translated.
- **Mermaid is a sketch format.**
  - Import reads `flowchart`/`graph` and its label conventions: `req: path@v`,
    `subflow: path`, `delay: 2s`, `extract: a=$.x`, `set: a=b`, `assert: …`,
    `log: …`, `note: …`, or a plain request path.
  - Labels resolve against the target collection. Anything else becomes a placeholder
    note, with FLW-MERMAID-001 at the node.
  - Nodes are ordered topologically, and `needs` is written only where the drawing
    differs from the sequence. A cycle is FLW-MERMAID-002 and imports nothing.
  - Export writes the same conventions, so supported nodes round-trip; containers
    become subgraphs (export only).
- **PlantUML** is left for after M6.

## Consequences

- Imported collections validate. A credential never reaches a file: the user sets it
  with `flowi secret set`.
- Scripts, OAuth flows, Bruno's richer asserts and non-HTTP traffic need hand work.
  Each is listed in the report.
- The corpus under `importers/*/testdata` is hand-written in each tool's export format,
  with broken files and golden reports. Real exports may hold cases it misses.
