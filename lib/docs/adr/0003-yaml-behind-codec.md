# 0003: YAML files, decoded behind a Codec interface

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

Collections live in users' git repositories, so files must be readable, commentable and
diff well. Flows are graphs and request bodies are often multi-line. TOML support is wanted
later.

## Decision

- All collection, workspace and settings files are **YAML**.
- The domain model never depends on a serialization format. Storage talks to a `Codec`
  interface; the YAML codec is the only implementation for now, and TOML can be added beside it.
- Decoding is strict: unknown keys, duplicate keys and type mismatches are errors with file,
  line and column.
- The manifest carries `format: <n>`, and older formats are migrated on load.
- Output is canonical and deterministic so `flowi fmt` and diffs stay stable.
- JSON Schemas are published for every file type.

## Consequences

- Editors get autocomplete and validation through the schemas.
- YAML's traps (indentation, implicit types) are handled by strict decoding and clear
  diagnostics rather than by avoiding YAML.
- The file format is a public contract: changes need a format bump and a migration.
